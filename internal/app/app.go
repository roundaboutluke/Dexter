package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"poraclego/internal/bot"
	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/db"
	"poraclego/internal/digest"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
	"poraclego/internal/profile"
	"poraclego/internal/render"
	"poraclego/internal/scanner"
	"poraclego/internal/server"
	"poraclego/internal/shiny"
	"poraclego/internal/stats"
	"poraclego/internal/tz"
	"poraclego/internal/validate"
	"poraclego/internal/webhook"
)

// App wires PoracleGo components together.
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
	logf("PoracleGo startup - initialising from %s", root)
	validate.CheckConfig(cfg, logf)

	logf("Generating supporting data files")
	data.GenerateBestEffort(root, true, logf)
	logf("Refreshing geofence cache")
	geofence.FetchKojiFences(cfg, root, logf)

	a.queue = webhook.NewQueue()
	a.discordQueue = dispatch.NewQueue("discord")
	a.telegramQueue = dispatch.NewQueue("telegram")
	a.discordWorker = dispatch.NewWorker(a.discordQueue, cfg, "discord", 750*time.Millisecond, root)
	a.telegramWorker = dispatch.NewWorker(a.telegramQueue, cfg, "telegram", 750*time.Millisecond, root)
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
	a.scannerClient = scannerClient

	database, err := db.New(cfg)
	if err != nil {
		return err
	}
	a.db = database
	a.query = db.NewQuery(database.Conn)

	migrationsDir := filepath.Join(root, "migrations")
	if err := db.EnsureMigrationsDir(migrationsDir); err != nil {
		return fmt.Errorf("ensure migrations dir: %w", err)
	}
	if err := db.RunMigrations(ctx, database, migrationsDir); err != nil {
		if a.forceMigrations {
			logf("migrations failed (continuing due to --force): %v", err)
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
	a.processor.Start()
	logf("Webhook processor started")

	srv, err := server.New(cfg, a.queue, a.processor, a.discordQueue, a.telegramQueue, a.query, a.fences, root, a.data, a.i18n, a.dts, a.scannerClient)
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

	botManager := bot.NewManager(cfg, a.query, a.data, a.i18n, a.dts, a.fences, a.processor, a.discordQueue, a.telegramQueue, a.queue, statsTracker, a.weatherTracker, tzLocator, a.scannerClient)
	botManager.Start()
	a.botManager = botManager
	a.server.SetBotManager(botManager)
	logf("Bot manager started")

	a.profileSchedule = profile.NewScheduler(cfg, a.query, a.i18n, tzLocator, a.discordQueue, a.telegramQueue, digestStore, a.dts)
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

// Shutdown performs a graceful shutdown.
func (a *App) Shutdown(ctx context.Context) error {
	logf("Poracle shutdown - starting save of cache")
	if a.processor != nil {
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
	logf("Poracle shutdown - complete")
	return logging.Close()
}
