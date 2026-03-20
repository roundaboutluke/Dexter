package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dexter/internal/bot"
	"dexter/internal/circuitbreaker"
	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/db"
	"dexter/internal/digest"
	"dexter/internal/dispatch"
	"dexter/internal/dts"
	"dexter/internal/geofence"
	"dexter/internal/i18n"
	"dexter/internal/logging"
	"dexter/internal/metrics"
	"dexter/internal/profile"
	"dexter/internal/render"
	"dexter/internal/scanner"
	"dexter/internal/server"
	"dexter/internal/shiny"
	"dexter/internal/stats"
	"dexter/internal/tz"
	"dexter/internal/validate"
	"dexter/internal/webhook"
)

type alertStatePreloader interface {
	RefreshAlertCacheSync() error
}

// App wires Dexter components together.
type App struct {
	config          *config.Config
	db              *db.DB
	server          *server.Server
	queue           *webhook.Queue
	processor       *webhook.Processor
	discordQueue    *dispatch.Queue
	telegramQueue   *dispatch.Queue
	discordWorker   *dispatch.Worker
	telegramWorker  *dispatch.Worker
	statsWorker     *stats.Worker
	weatherTracker  *webhook.WeatherTracker
	pogoEvents      *webhook.PogoEventParser
	botManager      *bot.Manager
	fences          *geofence.Store
	query           *db.Query
	data            *data.GameData
	i18n            *i18n.Factory
	dts             []dts.Template
	shinyPossible   *shiny.Possible
	scannerClient   *scanner.Client
	profileSchedule *profile.Scheduler
	forceMigrations bool
}

// New returns a new App instance.
func New() *App {
	return &App{}
}

// SetForceMigrations controls whether startup should continue after migration errors.
func (a *App) SetForceMigrations(force bool) {
	if a == nil {
		return
	}
	a.forceMigrations = force
}

// Run starts the application lifecycle.
func (a *App) Run(ctx context.Context) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve workdir: %w", err)
	}
	if err := config.EnsureDefaultFiles(root); err != nil {
		return fmt.Errorf("init config: %w", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		return err
	}
	a.config = cfg
	if err := logging.Init(cfg, root); err != nil {
		return err
	}
	logf("Dexter startup - initialising from %s", root)
	validate.CheckConfig(cfg, logf)

	// Initialize Prometheus metrics.
	metricsEnabled, _ := cfg.GetBool("server.metrics.enabled")
	if metricsEnabled {
		metrics.Init()
		logf("Prometheus metrics enabled")
	}

	if data.HasData(root) {
		logf("Refreshing game data")
		data.GenerateBestEffort(root, true, logf)
	} else {
		logf("First run detected: downloading game data (requires internet)...")
		if err := data.Generate(root, true, logf); err != nil {
			return fmt.Errorf("first-run data download failed: %w\n\n"+
				"Dexter needs game data files in util/. "+
				"Ensure you have internet access and try again, or run:\n"+
				"  go run ./cmd/dexter-generate", err)
		}
	}
	logf("Refreshing geofence cache")
	geofence.FetchKojiFences(cfg, root, logf)

	a.queue = webhook.NewQueue()
	a.discordQueue = dispatch.NewQueue("discord")
	a.telegramQueue = dispatch.NewQueue("telegram")

	// Create circuit breakers for external services.
	breakers := initCircuitBreakers(cfg)
	discordBreakers := map[string]*circuitbreaker.Breaker{}
	telegramBreakers := map[string]*circuitbreaker.Breaker{}
	if b, ok := breakers["discord"]; ok {
		discordBreakers["discord"] = b
	}
	if b, ok := breakers["telegram"]; ok {
		telegramBreakers["telegram"] = b
	}

	a.discordWorker = dispatch.NewWorker(a.discordQueue, cfg, "discord", 750*time.Millisecond, root, discordBreakers)
	a.telegramWorker = dispatch.NewWorker(a.telegramQueue, cfg, "telegram", 750*time.Millisecond, root, telegramBreakers)
	a.discordWorker.Start()
	a.telegramWorker.Start()
	logf("Started dispatch workers")

	fences, err := geofence.Load(cfg, root)
	if err != nil {
		return err
	}
	validate.CheckGeofence(fences.Fences, logf)
	a.fences = fences
	logf("Geofence loaded with %d fences", len(fences.Fences))

	gameData, err := data.Load(root)
	if err != nil {
		return err
	}
	a.data = gameData
	a.i18n = i18n.NewFactory(root, cfg)
	render.Init(root, cfg, a.data, a.i18n)
	logf("Game data and translators loaded")

	a.weatherTracker = webhook.NewWeatherTracker(cfg, root)
	a.shinyPossible = shiny.NewPossible("")
	a.shinyPossible.Start(ctx, filepath.Join(root, ".cache", "shinyPossible.json"))
	a.pogoEvents = webhook.NewPogoEventParser("")
	a.pogoEvents.Start(ctx, filepath.Join(root, ".cache", "pogoEvents.json"))
	logf("Started shiny and event refresh workers")

	scannerClient, err := scanner.New(cfg)
	if err != nil {
		return err
	}
	if b, ok := breakers["scanner_db"]; ok {
		scannerClient.SetBreaker(b)
	}
	a.scannerClient = scannerClient

	database, err := db.New(cfg)
	if err != nil {
		return err
	}
	a.db = database
	a.query = db.NewQuery(database.Conn)

	forcedMigrationFailure := false
	migrationsDir := filepath.Join(root, "migrations")
	if err := db.EnsureMigrationsDir(migrationsDir); err != nil {
		return fmt.Errorf("ensure migrations dir: %w", err)
	}
	if err := db.RunMigrations(ctx, database, migrationsDir); err != nil {
		if a.forceMigrations {
			logf("migrations failed (continuing due to --force): %v", err)
			forcedMigrationFailure = true
		} else {
			return fmt.Errorf("migrations failed: %w", err)
		}
	} else {
		logf("Database migrations complete")
	}

	templates, err := dts.Load(root)
	if err != nil {
		return err
	}
	validate.CheckDTS(templates, cfg, logf)
	a.dts = templates
	logf("Loaded %d DTS templates", len(templates))

	statsTracker := stats.NewTracker()
	a.statsWorker = stats.NewWorker(statsTracker, cfg)
	a.statsWorker.Start()
	tzLocator := tz.NewLocator(cfg, root)
	digestStore := digest.NewStore()
	a.processor = webhook.NewProcessor(a.queue, cfg, a.query, a.fences, a.data, a.i18n, a.dts, a.discordQueue, a.telegramQueue, statsTracker, a.weatherTracker, a.shinyPossible, tzLocator, a.pogoEvents, a.scannerClient, digestStore, root, 250*time.Millisecond)
	if err := preloadAlertState(a.processor, forcedMigrationFailure); err != nil {
		return fmt.Errorf("load alert state: %w", err)
	}
	a.processor.Start()
	logf("Webhook processor started")

	srv, err := server.New(cfg, a.queue, a.processor, a.discordQueue, a.telegramQueue, a.query, a.fences, root, a.data, a.i18n, a.dts, a.scannerClient, a.db.Conn)
	if err != nil {
		return err
	}
	a.server = srv
	a.server.Start()
	host, _ := cfg.GetString("server.host")
	port, ok := cfg.GetInt("server.port")
	if !ok {
		port = 3030
	}
	logf("Service started on %s:%d", host, port)

	// Start metrics background collector.
	if metrics.Get() != nil {
		metrics.StartCollector(ctx, a.queue, a.discordQueue, a.telegramQueue, a.db.Conn)
		logf("Metrics collector started")
	}

	botManager := bot.NewManager(cfg, a.query, a.data, a.i18n, a.dts, a.fences, a.processor, a.discordQueue, a.telegramQueue, a.queue, statsTracker, a.weatherTracker, tzLocator, a.scannerClient)
	botManager.Start()
	a.botManager = botManager
	a.server.SetBotManager(botManager)
	logf("Bot manager started")

	a.profileSchedule = profile.NewScheduler(cfg, a.query, a.i18n, tzLocator, a.discordQueue, a.telegramQueue, digestStore, a.dts)
	a.profileSchedule.SetRefreshAlertState(a.processor.RefreshAlertCacheAsync)
	a.profileSchedule.Start(ctx)
	logf("Profile scheduler started")

	a.startWatchers(ctx, root)
	logf("File watchers started")

	return nil
}

func logf(format string, args ...any) {
	logger := logging.Get().General
	if logger == nil {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
		return
	}
	logger.Infof(format, args...)
}

func preloadAlertState(preloader alertStatePreloader, forcedMigrationFailure bool) error {
	if preloader == nil {
		return nil
	}
	if err := preloader.RefreshAlertCacheSync(); err != nil {
		if !forcedMigrationFailure {
			return err
		}
		logger := logging.Get().General
		message := fmt.Sprintf("alert state preload failed after force-tolerated migration error; continuing without preloaded snapshot: %v", err)
		if logger != nil {
			logger.Warnf("%s", message)
			logger.Warnf("webhook matching will use the DB fallback until a later alert-state refresh succeeds")
		} else {
			fmt.Fprintln(os.Stderr, message)
			fmt.Fprintln(os.Stderr, "webhook matching will use the DB fallback until a later alert-state refresh succeeds")
		}
		return nil
	}
	return nil
}

func initCircuitBreakers(cfg *config.Config) map[string]*circuitbreaker.Breaker {
	enabled, ok := cfg.GetBool("circuitBreaker.enabled")
	if ok && !enabled {
		return nil
	}
	breakers := map[string]*circuitbreaker.Breaker{}
	m := metrics.Get()

	type cbConfig struct {
		name          string
		thresholdPath string
		cooldownPath  string
		defaultThresh int
		defaultCool   int
	}
	configs := []cbConfig{
		{"discord", "circuitBreaker.discord.failThreshold", "circuitBreaker.discord.cooldownSeconds", 5, 30},
		{"telegram", "circuitBreaker.telegram.failThreshold", "circuitBreaker.telegram.cooldownSeconds", 5, 30},
		{"scanner_db", "circuitBreaker.scannerDb.failThreshold", "circuitBreaker.scannerDb.cooldownSeconds", 3, 60},
	}
	for _, c := range configs {
		thresh := c.defaultThresh
		if v, ok := cfg.GetInt(c.thresholdPath); ok && v > 0 {
			thresh = v
		}
		cool := c.defaultCool
		if v, ok := cfg.GetInt(c.cooldownPath); ok && v > 0 {
			cool = v
		}
		b := circuitbreaker.New(c.name, thresh, time.Duration(cool)*time.Second)
		if m != nil {
			b.SetOnStateChange(func(name string, _, to circuitbreaker.State) {
				m.CircuitBreakerState.WithLabelValues(name).Set(float64(to))
				if to == circuitbreaker.Open {
					m.CircuitBreakerTrips.WithLabelValues(name).Inc()
				}
			})
		}
		breakers[c.name] = b
	}
	return breakers
}

// Shutdown performs a graceful shutdown.
func (a *App) Shutdown(ctx context.Context) error {
	logf("Dexter shutdown - saving caches")
	if a.processor != nil {
		a.processor.Stop()
		a.processor.SaveCaches()
	}
	if a.weatherTracker != nil {
		a.weatherTracker.SaveCaches()
	}
	if a.discordWorker != nil {
		a.discordWorker.SaveCaches()
	}
	if a.telegramWorker != nil {
		a.telegramWorker.SaveCaches()
	}
	if a.botManager != nil {
		a.botManager.Stop()
	}
	if a.statsWorker != nil {
		a.statsWorker.Stop()
	}
	if a.server != nil {
		_ = a.server.Shutdown(ctx)
	}
	if a.db != nil {
		_ = a.db.Close()
	}
	if a.scannerClient != nil {
		_ = a.scannerClient.Close()
	}
	logf("Dexter shutdown - complete")
	return logging.Close()
}
