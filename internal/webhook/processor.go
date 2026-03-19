package webhook

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dexter/internal/alertstate"
	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/db"
	"dexter/internal/digest"
	"dexter/internal/dispatch"
	"dexter/internal/dts"
	"dexter/internal/geofence"
	"dexter/internal/i18n"
	"dexter/internal/logging"
	"dexter/internal/pvp"
	"dexter/internal/ratelimit"
	"dexter/internal/scanner"
	"dexter/internal/shiny"
	"dexter/internal/stats"
	"dexter/internal/tz"
)

// Hook represents a single webhook payload.
type Hook struct {
	Type    string
	Message map[string]any
}

// Processor drains webhook queue items and routes them to handlers.
type Processor struct {
	queue         *Queue
	interval      time.Duration
	cfg           *config.Config
	cache         *TTLCache
	gymCache      *GymCache
	monsterCache  *MonsterAlarmCache
	monsterChange *MonsterChangeTracker
	raidSeen      map[string]raidSeenEntry
	raidSeenMu    sync.Mutex
	query         *db.Query
	fences        atomic.Pointer[geofence.Store]
	data          atomic.Pointer[data.GameData]
	i18n          *i18n.Factory
	templates     atomic.Pointer[[]dts.Template]
	geocoder      *Geocoder
	weather       *WeatherClient
	weatherData   *WeatherTracker
	rateChecker   *ratelimit.Checker
	stats         *stats.Tracker
	shinyPossible *shiny.Possible
	tzLocator     *tz.Locator
	eventParser   *PogoEventParser
	pvpCalc       atomic.Pointer[pvp.Calculator]
	discordQueue  *dispatch.Queue
	telegramQueue *dispatch.Queue
	scanner        *scanner.Client
	questDigests   *digest.Store
	recentActivity *RecentActivity
	cacheDir       string
	root          string
	customEmoji   map[string]map[string]string

	alertState           *alertstate.Manager
	alertStateRefreshMu  sync.Mutex
	alertStateRefreshing bool
	alertStatePending    bool
	alertStateLoader     func() (*alertstate.Snapshot, error)

	startOnce sync.Once
	workCh    chan any
	stopCh    chan struct{}
}

// NewProcessor returns a processor that drains the queue on the given interval.
func NewProcessor(queue *Queue, cfg *config.Config, query *db.Query, fences *geofence.Store, gameData *data.GameData, i18nFactory *i18n.Factory, templates []dts.Template, discordQueue *dispatch.Queue, telegramQueue *dispatch.Queue, statsTracker *stats.Tracker, weatherData *WeatherTracker, shinyPossible *shiny.Possible, tzLocator *tz.Locator, eventParser *PogoEventParser, scannerClient *scanner.Client, digestStore *digest.Store, root string, interval time.Duration) *Processor {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	p := &Processor{
		queue:         queue,
		interval:      interval,
		cfg:           cfg,
		cache:         NewTTLCache(),
		gymCache:      NewGymCache(),
		monsterCache:  NewMonsterAlarmCache(),
		monsterChange: NewMonsterChangeTracker(cfg, root),
		raidSeen:      map[string]raidSeenEntry{},
		query:         query,
		i18n:          i18nFactory,
		geocoder:      NewGeocoder(cfg),
		weather:       NewWeatherClient(cfg),
		weatherData:   weatherData,
		rateChecker:   ratelimit.NewChecker(cfg),
		stats:         statsTracker,
		shinyPossible: shinyPossible,
		tzLocator:     tzLocator,
		eventParser:   eventParser,
		discordQueue:  discordQueue,
		telegramQueue: telegramQueue,
		scanner:       scannerClient,
		questDigests:   digestStore,
		recentActivity: NewRecentActivity(),
		cacheDir:       cacheDir(root),
		root:          root,
		customEmoji:   loadCustomEmoji(root),
		alertState:    alertstate.NewManager(),
		stopCh:        make(chan struct{}),
	}
	p.fences.Store(fences)
	p.data.Store(gameData)
	p.templates.Store(&templates)
	p.pvpCalc.Store(pvp.NewCalculator(cfg, gameData))
	p.loadCaches()
	return p
}

type raidSeenEntry struct {
	Signature string
	Expires   time.Time
}

func (p *Processor) generalLogger() *logging.Logger {
	return logging.Get().General
}

func (p *Processor) controllerLogger() *logging.Logger {
	return logging.Get().Controller
}

func (p *Processor) webhooksLogger() *logging.Logger {
	return logging.Get().Webhooks
}

// RecentActivity returns the tracker for recently-seen game entities.
func (p *Processor) RecentActivity() *RecentActivity {
	if p == nil {
		return nil
	}
	return p.recentActivity
}

func (p *Processor) logInboundPayload(hook *Hook) {
	logger := p.webhooksLogger()
	if logger == nil || hook == nil || hook.Message == nil || !logger.Enabled(logging.LevelInfo) {
		return
	}
	raw, err := json.Marshal(hook.Message)
	if err != nil {
		logger.Infof("%s %v", hook.Type, hook.Message)
		return
	}
	logger.Infof("%s %s", hook.Type, string(raw))
}

func (p *Processor) logControllerf(level logging.Level, hook *Hook, format string, args ...any) {
	logger := p.controllerLogger()
	if logger == nil || !logger.Enabled(level) {
		return
	}
	ref := hookLogReference(hook)
	if ref != "" {
		args = append([]any{ref}, args...)
		logger.Logf(level, "%s: "+format, args...)
		return
	}
	logger.Logf(level, format, args...)
}

func hookLogReference(hook *Hook) string {
	if hook == nil || hook.Message == nil {
		return ""
	}
	for _, key := range []string{"encounter_id", "gym_id", "pokestop_id", "nest_id", "id", "s2_cell_id"} {
		if value := strings.TrimSpace(getString(hook.Message[key])); value != "" {
			return value
		}
	}
	return strings.TrimSpace(hook.Type)
}

// Start runs the processor loop in a goroutine.
func (p *Processor) Start() {
	if p == nil {
		return
	}
	p.startOnce.Do(func() {
		go p.startCachePruner()
		workerCount := 1
		workerConcurrency := 1
		if p.cfg != nil {
			if value, ok := p.cfg.GetInt("tuning.webhookProcessingWorkers"); ok && value > 0 {
				workerCount = value
			}
			if value, ok := p.cfg.GetInt("tuning.concurrentWebhookProcessorsPerWorker"); ok && value > 0 {
				workerConcurrency = value
			}
		}
		if p.workCh == nil {
			buffer := workerCount * workerConcurrency * 4
			if buffer < 64 {
				buffer = 64
			}
			p.workCh = make(chan any, buffer)
		}
		for i := 0; i < workerCount; i++ {
			go p.workerLoop(workerConcurrency)
		}
		if p.monsterCache != nil {
			p.monsterCache.RefreshAsync(p.cfg, p.query)
		}
		go func() {
			ticker := time.NewTicker(p.interval)
			defer ticker.Stop()
			for range ticker.C {
				items := p.queue.Drain()
				if len(items) == 0 {
					continue
				}
				for _, item := range items {
					p.workCh <- item
				}
			}
		}()
	})
}

// getFences returns the current geofence store. Safe for nil receiver.
func (p *Processor) getFences() *geofence.Store {
	if p == nil {
		return nil
	}
	return p.fences.Load()
}

// getData returns the current game data. Safe for nil receiver.
func (p *Processor) getData() *data.GameData {
	if p == nil {
		return nil
	}
	return p.data.Load()
}

// getTemplates returns the current DTS templates. Safe for nil receiver.
func (p *Processor) getTemplates() []dts.Template {
	if p == nil {
		return nil
	}
	ptr := p.templates.Load()
	if ptr == nil {
		return nil
	}
	return *ptr
}

// getPvpCalc returns the current PVP calculator. Safe for nil receiver.
func (p *Processor) getPvpCalc() *pvp.Calculator {
	if p == nil {
		return nil
	}
	return p.pvpCalc.Load()
}

func (p *Processor) workerLoop(concurrency int) {
	if concurrency < 1 {
		concurrency = 1
	}
	sem := make(chan struct{}, concurrency)
	for item := range p.workCh {
		sem <- struct{}{}
		go func(it any) {
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					if logger := logging.Get().Webhooks; logger != nil {
						logger.Errorf("panic in webhook worker: %v\n%s", r, string(debug.Stack()))
					} else {
						fmt.Fprintf(os.Stderr, "panic in webhook worker: %v\n%s\n", r, string(debug.Stack()))
					}
				}
			}()
			p.handle(it)
		}(item)
	}
}
