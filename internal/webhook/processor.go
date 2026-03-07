package webhook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/geo/s2"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/db"
	"poraclego/internal/digest"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/geo"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
	"poraclego/internal/pvp"
	"poraclego/internal/ratelimit"
	"poraclego/internal/scanner"
	"poraclego/internal/shiny"
	"poraclego/internal/stats"
	"poraclego/internal/tz"
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
	fences        *geofence.Store
	data          *data.GameData
	i18n          *i18n.Factory
	templates     []dts.Template
	geocoder      *Geocoder
	weather       *WeatherClient
	weatherData   *WeatherTracker
	rateChecker   *ratelimit.Checker
	stats         *stats.Tracker
	shinyPossible *shiny.Possible
	tzLocator     *tz.Locator
	eventParser   *PogoEventParser
	pvpCalc       *pvp.Calculator
	discordQueue  *dispatch.Queue
	telegramQueue *dispatch.Queue
	scanner       *scanner.Client
	questDigests  *digest.Store
	cacheDir      string
	root          string
	customEmoji   map[string]map[string]string

	processing bool
	workCh     chan any
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
		fences:        fences,
		data:          gameData,
		i18n:          i18nFactory,
		templates:     templates,
		geocoder:      NewGeocoder(cfg),
		weather:       NewWeatherClient(cfg),
		weatherData:   weatherData,
		rateChecker:   ratelimit.NewChecker(cfg),
		stats:         statsTracker,
		shinyPossible: shinyPossible,
		tzLocator:     tzLocator,
		eventParser:   eventParser,
		pvpCalc:       pvp.NewCalculator(cfg, gameData),
		discordQueue:  discordQueue,
		telegramQueue: telegramQueue,
		scanner:       scannerClient,
		questDigests:  digestStore,
		cacheDir:      cacheDir(root),
		root:          root,
		customEmoji:   loadCustomEmoji(root),
	}
	p.loadCaches()
	return p
}

type raidSeenEntry struct {
	Signature string
	Expires   time.Time
}

// Start runs the processor loop in a goroutine.
func (p *Processor) Start() {
	if p == nil || p.processing {
		return
	}
	p.processing = true
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
}

// UpdateData replaces the game data set used for webhook processing.
func (p *Processor) UpdateData(game *data.GameData) {
	if p == nil || game == nil {
		return
	}
	p.data = game
	p.pvpCalc = pvp.NewCalculator(p.cfg, game)
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

func (p *Processor) startCachePruner() {
	if p == nil {
		return
	}
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		if p.cache != nil {
			p.cache.PruneExpired(now)
		}
		p.pruneRaidSeen(now)
	}
}

func (p *Processor) pruneRaidSeen(now time.Time) {
	if p == nil {
		return
	}
	p.raidSeenMu.Lock()
	defer p.raidSeenMu.Unlock()
	for key, entry := range p.raidSeen {
		if entry.Expires.IsZero() {
			continue
		}
		if now.After(entry.Expires) {
			delete(p.raidSeen, key)
		}
	}
}

// UpdateTemplates replaces the DTS template list.
func (p *Processor) UpdateTemplates(templates []dts.Template) {
	if p == nil {
		return
	}
	p.templates = templates
}

// SaveCaches persists cache data to disk for warm restarts.
func (p *Processor) SaveCaches() {
	if p == nil || p.cacheDir == "" {
		return
	}
	_ = os.MkdirAll(p.cacheDir, 0o755)
	if p.gymCache != nil {
		_ = saveJSONFile(filepath.Join(p.cacheDir, "gymCache.json"), p.gymCache.Snapshot())
	}
	if p.geocoder != nil {
		_ = p.geocoder.SaveCache(filepath.Join(p.cacheDir, "geocoderCache.json"))
	}
	if p.weather != nil {
		_ = p.weather.SaveCache(filepath.Join(p.cacheDir, "weatherCache.json"))
	}
	if p.weatherData != nil {
		p.weatherData.SaveCaches()
	}
	if p.monsterChange != nil {
		p.monsterChange.SaveCache()
	}
}

func (p *Processor) loadCaches() {
	if p == nil || p.cacheDir == "" {
		return
	}
	_ = os.MkdirAll(p.cacheDir, 0o755)
	if p.gymCache != nil {
		var payload map[string]GymState
		if err := loadJSONFile(filepath.Join(p.cacheDir, "gymCache.json"), &payload); err == nil {
			p.gymCache.Load(payload)
		}
	}
	if p.geocoder != nil {
		p.geocoder.LoadCache(filepath.Join(p.cacheDir, "geocoderCache.json"))
	}
	if p.weather != nil {
		p.weather.LoadCache(filepath.Join(p.cacheDir, "weatherCache.json"))
	}
	if p.monsterChange != nil {
		p.monsterChange.LoadCache()
	}
}

func cacheDir(root string) string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, ".cache")
}

func (p *Processor) handle(item any) {
	hook, ok := normalizeHook(item)
	if !ok {
		logger := logging.Get().Webhooks
		if logger != nil {
			logger.Warnf("webhook processor skipping unsupported payload: %T", item)
		} else {
			fmt.Fprintf(os.Stderr, "webhook processor skipping unsupported payload: %T\n", item)
		}
		return
	}
	if hook.Type == "raid" {
		pokemonID := getInt(hook.Message["pokemon_id"])
		if pokemonID == 0 {
			pokemonID = getInt(hook.Message["pokemonId"])
			if pokemonID > 0 {
				hook.Message["pokemon_id"] = pokemonID
			}
		}
		if getInt(hook.Message["level"]) == 0 {
			if level := getInt(hook.Message["raid_level"]); level > 0 {
				hook.Message["level"] = level
			}
		}
		if pokemonID == 0 {
			hook.Type = "egg"
		}
	}
	if hook.Type == "raid" || hook.Type == "egg" {
		if getString(hook.Message["gym_id"]) == "" {
			if id := getString(hook.Message["id"]); id != "" {
				hook.Message["gym_id"] = id
			}
		}
		if getInt(hook.Message["level"]) == 0 {
			if level := getInt(hook.Message["raid_level"]); level > 0 {
				hook.Message["level"] = level
			}
		}
	}
	if hook.Type == "max_battle" {
		stationID := getString(hook.Message["id"])
		if stationID != "" && getString(hook.Message["stationId"]) == "" {
			hook.Message["stationId"] = stationID
		}
		if stationName := getString(hook.Message["name"]); stationName != "" && getString(hook.Message["stationName"]) == "" {
			hook.Message["stationName"] = stationName
		}
		if pokemonID := getInt(hook.Message["battle_pokemon_id"]); pokemonID > 0 && getInt(hook.Message["pokemon_id"]) == 0 {
			hook.Message["pokemon_id"] = pokemonID
		}
		if pokemonID := getInt(hook.Message["pokemon_id"]); pokemonID > 0 && getInt(hook.Message["battle_pokemon_id"]) == 0 {
			hook.Message["battle_pokemon_id"] = pokemonID
		}
		if form := getInt(hook.Message["battle_pokemon_form"]); form > 0 && getInt(hook.Message["form"]) == 0 {
			hook.Message["form"] = form
		}
		if form := getInt(hook.Message["form"]); form > 0 && getInt(hook.Message["battle_pokemon_form"]) == 0 {
			hook.Message["battle_pokemon_form"] = form
		}
		if level := getInt(hook.Message["battle_level"]); level > 0 && getInt(hook.Message["level"]) == 0 {
			hook.Message["level"] = level
		}
		if level := getInt(hook.Message["level"]); level > 0 && getInt(hook.Message["battle_level"]) == 0 {
			hook.Message["battle_level"] = level
		}
		if move1 := getInt(hook.Message["battle_pokemon_move_1"]); move1 > 0 && getInt(hook.Message["move_1"]) == 0 {
			hook.Message["move_1"] = move1
		}
		if move2 := getInt(hook.Message["battle_pokemon_move_2"]); move2 > 0 && getInt(hook.Message["move_2"]) == 0 {
			hook.Message["move_2"] = move2
		}
		if gender := getInt(hook.Message["battle_pokemon_gender"]); gender > 0 && getInt(hook.Message["gender"]) == 0 {
			hook.Message["gender"] = gender
		}
		if costume := getInt(hook.Message["battle_pokemon_costume"]); costume > 0 && getInt(hook.Message["costume"]) == 0 {
			hook.Message["costume"] = costume
		}
		if alignment := getInt(hook.Message["battle_pokemon_alignment"]); alignment > 0 && getInt(hook.Message["alignment"]) == 0 {
			hook.Message["alignment"] = alignment
		}
		if bread := getInt(hook.Message["battle_pokemon_bread_mode"]); bread > 0 && getInt(hook.Message["bread"]) == 0 {
			hook.Message["bread"] = bread
		}
		if _, ok := hook.Message["evolution"]; !ok {
			// Match PoracleJS maxbattle controller which currently sets evolution to 0.
			hook.Message["evolution"] = 0
		}
		if level := getInt(hook.Message["level"]); level > 0 && hook.Message["gmax"] == nil {
			if level > 6 {
				hook.Message["gmax"] = 1
			} else {
				hook.Message["gmax"] = 0
			}
		}
		if getString(hook.Message["color"]) == "" {
			hook.Message["color"] = "D000C0"
		}
	}
	normalizeHookCoordinates(hook)
	p.normalizeHookExpiry(hook)
	if shouldSkipExpiredHook(hook) {
		return
	}
	if p.shouldSkipMinimumTime(hook) {
		return
	}
	if p.shouldSkipLongRaid(hook) {
		return
	}

	switch hook.Type {
	case "pokemon":
		if p.disabled("general.disablePokemon") {
			return
		}
		if !getBoolFromConfig(p.cfg, "tuning.disablePokemonCache", false) {
			if !p.dedupePokemon(hook) {
				return
			}
		}
		p.applyPvp(hook)
		p.updateStats(hook)
		if p.weatherData != nil {
			weatherID := weatherCondition(hook.Message)
			if weatherID > 0 {
				cellID := getString(hook.Message["s2_cell_id"])
				if p.weatherData.CheckWeatherOnMonster(cellID, getFloat(hook.Message["latitude"]), getFloat(hook.Message["longitude"]), weatherID) {
					enabled := true
					if p.cfg != nil {
						enabled, _ = p.cfg.GetBool("weather.weatherChangeAlert")
					}
					if enabled {
						_ = p.dispatchWeatherChange(hook)
					}
				}
			}
		}
		p.dispatchMonsterChange(hook)
		if p.monsterChange != nil {
			encounterID := strings.TrimSpace(getString(hook.Message["encounter_id"]))
			if encounterID != "" {
				expires := hookExpiryUnix(hook)
				if p.monsterChange.ShouldSuppressStandardAlert(encounterID, hook, expires) {
					return
				}
			}
		}
		p.dispatch(hook)
	case "raid", "egg":
		if p.disabled("general.disableRaid") {
			return
		}
		if !p.dedupeRaid(hook) {
			return
		}
		p.dispatch(hook)
	case "max_battle":
		if p.disabled("general.disableMaxBattle") {
			return
		}
		if !p.dedupeMaxBattle(hook) {
			return
		}
		p.dispatch(hook)
	case "invasion", "pokestop":
		if p.disabled("general.disablePokestop") {
			return
		}
		p.handlePokestop(hook)
	case "fort_update":
		if p.disabled("general.disableFortUpdate") {
			return
		}
		p.dispatch(hook)
	case "quest":
		if p.disabled("general.disableQuest") {
			return
		}
		if !p.dedupeQuest(hook) {
			return
		}
		p.dispatch(hook)
	case "gym", "gym_details":
		if p.disabled("general.disableGym") {
			return
		}
		if !p.dedupeGym(hook) {
			return
		}
		p.dispatch(hook)
	case "nest":
		if p.disabled("general.disableNest") {
			return
		}
		if !p.dedupeNest(hook) {
			return
		}
		p.dispatch(hook)
	case "weather":
		if p.disabled("general.disableWeather") {
			return
		}
		if getString(hook.Message["s2_cell_id"]) == "" {
			lat := getFloat(hook.Message["latitude"])
			lon := getFloat(hook.Message["longitude"])
			if cell := geo.WeatherCellID(lat, lon); cell != "" {
				hook.Message["s2_cell_id"] = cell
			}
		}
		if !p.dedupeWeather(hook) {
			return
		}
		if p.weatherData != nil {
			p.weatherData.UpdateFromHook(hook)
		}
		if p.cfg != nil {
			enabled, _ := p.cfg.GetBool("weather.weatherChangeAlert")
			if !enabled {
				return
			}
		}
		if !p.dispatchWeatherChange(hook) {
			return
		}
		p.dispatch(hook)
	default:
		logger := logging.Get().Webhooks
		if logger != nil {
			logger.Warnf("webhook processor unknown hook type %s", hook.Type)
		} else {
			fmt.Fprintf(os.Stderr, "webhook processor unknown hook type %s\n", hook.Type)
		}
	}
}

func (p *Processor) normalizeHookExpiry(hook *Hook) {
	if p == nil || hook == nil || hook.Message == nil {
		return
	}
	if hook.Type != "quest" {
		return
	}
	if getInt64(hook.Message["expiration"]) > 0 || getInt64(hook.Message["disappear_time"]) > 0 {
		return
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 && lon == 0 {
		return
	}
	loc := time.Local
	if p.tzLocator != nil {
		if found, ok := p.tzLocator.Location(lat, lon); ok {
			loc = found
		}
	}
	now := time.Now().In(loc)
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
	hook.Message["expiration"] = end.Unix()
}

func (p *Processor) shouldSkipMinimumTime(hook *Hook) bool {
	if p == nil || p.cfg == nil || hook == nil {
		return false
	}
	minTTH, ok := p.cfg.GetInt("general.alertMinimumTime")
	if !ok || minTTH <= 0 {
		return false
	}
	switch hook.Type {
	case "pokemon", "raid", "egg", "quest", "invasion", "lure", "gym", "gym_details", "max_battle":
	default:
		return false
	}
	expire := hookExpiryUnix(hook)
	if hook.Type == "egg" {
		expire = getInt64(hook.Message["start"])
		if expire == 0 {
			expire = getInt64(hook.Message["hatch_time"])
		}
	}
	if expire <= 0 {
		// PoracleJS treats gym alerts as having a fixed 1-hour TTH (and applies alertMinimumTime against that).
		if hook.Type == "gym" || hook.Type == "gym_details" {
			return 3600 < minTTH
		}
		return false
	}
	remaining := time.Until(time.Unix(expire, 0))
	if remaining <= 0 {
		return true
	}
	return int(remaining.Seconds()) < minTTH
}

func (p *Processor) shouldSkipLongRaid(hook *Hook) bool {
	if p == nil || p.cfg == nil || hook == nil {
		return false
	}
	if hook.Type != "raid" && hook.Type != "egg" {
		return false
	}
	ignore, _ := p.cfg.GetBool("general.ignoreLongRaids")
	if !ignore {
		return false
	}
	start := getInt64(hook.Message["start"])
	if start == 0 {
		start = getInt64(hook.Message["hatch_time"])
	}
	end := getInt64(hook.Message["end"])
	if start == 0 || end == 0 {
		return false
	}
	return (end - start) > 47*60
}

func normalizeHookCoordinates(hook *Hook) {
	if hook == nil || hook.Message == nil {
		return
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 {
		lat = getFloat(hook.Message["lat"])
	}
	if lon == 0 {
		lon = getFloat(hook.Message["lon"])
		if lon == 0 {
			lon = getFloat(hook.Message["lng"])
		}
	}
	if getFloat(hook.Message["latitude"]) == 0 && lat != 0 {
		hook.Message["latitude"] = lat
	}
	if getFloat(hook.Message["longitude"]) == 0 && lon != 0 {
		hook.Message["longitude"] = lon
	}
	if hook.Type != "fort_update" {
		return
	}
	if getFloat(hook.Message["latitude"]) == 0 && getFloat(hook.Message["longitude"]) == 0 {
		if lat, lon, ok := extractLocation(hook.Message["new"]); ok {
			hook.Message["latitude"] = lat
			hook.Message["longitude"] = lon
		} else if lat, lon, ok := extractLocation(hook.Message["old"]); ok {
			hook.Message["latitude"] = lat
			hook.Message["longitude"] = lon
		}
	}
	if getString(hook.Message["fort_type"]) == "" && getString(hook.Message["fortType"]) == "" {
		if entry, ok := hook.Message["new"].(map[string]any); ok {
			if value, ok := entry["type"].(string); ok && value != "" {
				hook.Message["fort_type"] = value
			}
		}
		if entry, ok := hook.Message["old"].(map[string]any); ok {
			if value, ok := entry["type"].(string); ok && value != "" {
				hook.Message["fort_type"] = value
			}
		}
	}
	if getString(hook.Message["id"]) == "" {
		if entry, ok := hook.Message["new"].(map[string]any); ok {
			if value := getString(entry["id"]); value != "" {
				hook.Message["id"] = value
				return
			}
		}
		if entry, ok := hook.Message["old"].(map[string]any); ok {
			if value := getString(entry["id"]); value != "" {
				hook.Message["id"] = value
			}
		}
	}
}

func (p *Processor) dispatch(hook *Hook) {
	if p.query == nil {
		logger := logging.Get().Webhooks
		if logger != nil {
			logger.Warnf("webhook processor missing query for %s", hook.Type)
		} else {
			fmt.Fprintf(os.Stderr, "webhook processor missing query for %s\n", hook.Type)
		}
		return
	}
	if hook != nil && hook.Type == "pokemon" {
		normalizePvpRankings(hook)
	}
	targets, err := p.matchTargets(hook)
	if err != nil {
		logger := logging.Get().Webhooks
		if logger != nil {
			logger.Errorf("webhook processor match error: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "webhook processor match error: %v\n", err)
		}
		return
	}
	if len(targets) == 0 {
		return
	}
	trackWeather := false
	if hook.Type == "pokemon" && p.cfg != nil && p.weatherData != nil {
		trackWeather, _ = p.cfg.GetBool("weather.weatherChangeAlert")
	}
	var cared *caredPokemon
	trackWeatherCare := func(match alertMatch) {
		if !trackWeather || p.weatherData == nil {
			return
		}
		cellID := getString(hook.Message["s2_cell_id"])
		if cellID == "" {
			lat := getFloat(hook.Message["latitude"])
			lon := getFloat(hook.Message["longitude"])
			cellID = geo.WeatherCellID(lat, lon)
		}
		caresUntil := hookExpiryUnix(hook)
		if cellID == "" || caresUntil <= 0 {
			return
		}
		if cared == nil {
			if hasNumeric(hook.Message["individual_attack"]) && hasNumeric(hook.Message["individual_defense"]) && hasNumeric(hook.Message["individual_stamina"]) {
				cared = caredPokemonFromHook(p, hook)
			}
		}
		clean := getBool(match.Row["clean"])
		ping := getString(match.Row["ping"])
		p.weatherData.TrackCare(cellID, match.Target, caresUntil, clean, ping, cared)
	}
	for _, match := range targets {
		if hook.Type == "raid" || hook.Type == "egg" {
			rsvpChanges := getInt(match.Row["rsvp_changes"])
			if rsvpChanges == 0 && !getBool(hook.Message["firstNotification"]) {
				continue
			}
			if rsvpChanges == 2 {
				if rsvps, ok := hook.Message["rsvps"].([]any); !ok || len(rsvps) == 0 {
					continue
				}
			}
		}
		payload, message := p.formatPayload(hook, match)
		target := match.Target
		clean := getBool(match.Row["clean"])
		ping := getString(match.Row["ping"])
		tth := buildCleanTTH(hook)
		updateKey := ""
		updateExisting := false
		if hook.Type == "raid" || hook.Type == "egg" {
			updateKey = updateKeyForRaid(hook)
			if updateKey != "" {
				updateExisting = !getBool(hook.Message["firstNotification"])
			}
		} else if hook.Type == "pokemon" {
			updateKey = updateKeyForPokemon(hook, match.Row)
		}
		job := dispatch.MessageJob{
			Lat:            getFloat(hook.Message["latitude"]),
			Lon:            getFloat(hook.Message["longitude"]),
			Message:        message,
			Payload:        payload,
			Target:         target.ID,
			Type:           target.Type,
			Name:           target.Name,
			TTH:            tth,
			Clean:          clean,
			Emoji:          "",
			LogReference:   "Webhook",
			Language:       target.Language,
			UpdateKey:      updateKey,
			UpdateExisting: updateExisting,
		}
		if p.rateChecker == nil || job.AlwaysSend {
			trackWeatherCare(match)
			if hook.Type == "pokemon" && p.monsterChange != nil && updateKey != "" {
				encounterID := getString(hook.Message["encounter_id"])
				caresUntil := hookExpiryUnix(hook)
				if encounterID != "" && caresUntil > 0 {
					p.monsterChange.TrackCare(encounterID, target, caresUntil, clean, ping, updateKey, hook)
				}
			}
			p.enqueue(job)
			continue
		}
		additional, sendOriginal := p.applyRateLimit(job)
		for _, extra := range additional {
			p.enqueue(extra)
		}
		if sendOriginal {
			trackWeatherCare(match)
			if hook.Type == "pokemon" && p.monsterChange != nil && updateKey != "" {
				encounterID := getString(hook.Message["encounter_id"])
				caresUntil := hookExpiryUnix(hook)
				if encounterID != "" && caresUntil > 0 {
					p.monsterChange.TrackCare(encounterID, target, caresUntil, clean, ping, updateKey, hook)
				}
			}
			p.enqueue(job)
		}
	}
}

func shouldSkipExpiredHook(hook *Hook) bool {
	if hook == nil || hook.Message == nil {
		return false
	}
	switch hook.Type {
	case "pokemon", "raid", "egg", "quest", "invasion", "lure", "max_battle":
	default:
		return false
	}
	expire := hookExpiryUnix(hook)
	if hook.Type == "egg" {
		expire = getInt64(hook.Message["start"])
		if expire == 0 {
			expire = getInt64(hook.Message["hatch_time"])
		}
	}
	if expire <= 0 {
		return false
	}
	return time.Now().After(time.Unix(expire, 0))
}

func buildCleanTTH(hook *Hook) dispatch.TimeToHide {
	if hook == nil {
		return dispatch.TimeToHide{Hours: 1}
	}
	expire := int64(0)
	if hook.Type == "egg" {
		expire = getInt64(hook.Message["start"])
		if expire == 0 {
			expire = getInt64(hook.Message["hatch_time"])
		}
	} else {
		expire = hookExpiryUnix(hook)
	}
	if expire <= 0 {
		return dispatch.TimeToHide{Hours: 1}
	}
	remaining := time.Until(time.Unix(expire, 0))
	if remaining < 0 {
		remaining = 0
	}
	total := int(remaining.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return dispatch.TimeToHide{Hours: h, Minutes: m, Seconds: s}
}

func updateKeyForRaid(hook *Hook) string {
	if hook == nil {
		return ""
	}
	gymID := getString(hook.Message["gym_id"])
	if gymID == "" {
		gymID = getString(hook.Message["id"])
	}
	if gymID == "" {
		return ""
	}
	if hook.Type == "egg" {
		start := getInt64(hook.Message["start"])
		if start == 0 {
			start = getInt64(hook.Message["hatch_time"])
		}
		level := getInt(hook.Message["level"])
		if level == 0 {
			level = getInt(hook.Message["raid_level"])
		}
		return fmt.Sprintf("egg:%s:%d:%d", gymID, start, level)
	}
	end := getInt64(hook.Message["end"])
	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		pokemonID = getInt(hook.Message["pokemonId"])
	}
	return fmt.Sprintf("raid:%s:%d:%d", gymID, end, pokemonID)
}

func updateKeyForPokemon(hook *Hook, row map[string]any) string {
	if hook == nil || hook.Message == nil {
		return ""
	}
	encounterID := strings.TrimSpace(getString(hook.Message["encounter_id"]))
	if encounterID == "" {
		return ""
	}
	uid := ""
	if row != nil {
		uid = strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
	}
	if uid == "" {
		return "pokemon:" + encounterID
	}
	return fmt.Sprintf("pokemon:%s:%s", encounterID, uid)
}

func monsterChangeUpdateKey(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	return "monsterchange:" + base
}

func (p *Processor) dispatchMonsterChange(hook *Hook) {
	if p == nil || hook == nil || hook.Type != "pokemon" || p.monsterChange == nil {
		return
	}
	encounterID := strings.TrimSpace(getString(hook.Message["encounter_id"]))
	if encounterID == "" {
		return
	}
	expires := hookExpiryUnix(hook)
	old, cares, changed, flapping := p.monsterChange.DetectChange(encounterID, hook, expires)
	if !changed || len(cares) == 0 || old.PokemonID == 0 {
		return
	}

	changeHook := &Hook{Type: "pokemon", Message: map[string]any{}}
	for key, value := range hook.Message {
		changeHook.Message[key] = value
	}
	changeHook.Message["_monsterChange"] = true
	changeHook.Message["oldPokemonId"] = old.PokemonID
	changeHook.Message["oldFormId"] = old.Form
	changeHook.Message["oldCostume"] = old.Costume
	changeHook.Message["oldGender"] = old.Gender
	changeHook.Message["oldCp"] = old.CP
	changeHook.Message["oldIv"] = old.IV
	changeHook.Message["oldIvKnown"] = old.IV >= 0
	changeHook.Message["abSpawn"] = flapping

	expireUnix := expires
	if expireUnix <= 0 {
		expireUnix = old.Expires
	}
	tth := dispatch.TimeToHide{Hours: 1, Minutes: 0, Seconds: 0}
	if expireUnix > 0 {
		tth = buildTTHFromUnix(expireUnix)
	}

	for _, care := range cares {
		if care.TargetID == "" || care.TargetType == "" || care.UpdateKey == "" {
			continue
		}
		target := alertTarget{
			ID:       care.TargetID,
			Type:     care.TargetType,
			Name:     care.TargetName,
			Language: care.Language,
			Template: care.Template,
			Platform: platformFromType(care.TargetType),
		}
		match := alertMatch{
			Target: target,
			Row: map[string]any{
				"ping":  care.Ping,
				"clean": care.Clean,
			},
		}
		payload, message := p.formatPayload(changeHook, match)
		changeUpdateKey := monsterChangeUpdateKey(care.UpdateKey)
		job := dispatch.MessageJob{
			Lat:          getFloat(changeHook.Message["latitude"]),
			Lon:          getFloat(changeHook.Message["longitude"]),
			Message:      message,
			Payload:      payload,
			Target:       target.ID,
			Type:         target.Type,
			Name:         target.Name,
			TTH:          tth,
			Clean:        care.Clean,
			Emoji:        "",
			LogReference: "MonsterChange",
			Language:     target.Language,
			UpdateKey:    changeUpdateKey,
			// First monster change should notify as a new message.
			// On flap/revert, edit the existing monster-change alert to avoid a third post.
			UpdateExisting: flapping && changeUpdateKey != "",
		}
		if p.rateChecker == nil || job.AlwaysSend {
			p.enqueue(job)
			continue
		}
		additional, sendOriginal := p.applyRateLimit(job)
		for _, extra := range additional {
			p.enqueue(extra)
		}
		if sendOriginal {
			p.enqueue(job)
		}
	}
}

func (p *Processor) dispatchWeatherChange(hook *Hook) bool {
	if p == nil || hook == nil || p.weatherData == nil {
		return true
	}
	cellID := getString(hook.Message["s2_cell_id"])
	if cellID == "" {
		lat := getFloat(hook.Message["latitude"])
		lon := getFloat(hook.Message["longitude"])
		cellID = geo.WeatherCellID(lat, lon)
	}
	if cellID == "" {
		return false
	}
	weatherID := weatherCondition(hook.Message)
	if weatherID == 0 {
		return false
	}
	weatherHook := hook
	timestamp := getInt64(hook.Message["time_changed"])
	if timestamp == 0 {
		timestamp = getInt64(hook.Message["updated"])
	}
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	if hook.Type != "weather" {
		lat := getFloat(hook.Message["latitude"])
		lon := getFloat(hook.Message["longitude"])
		if cellInt, err := strconv.ParseUint(cellID, 10, 64); err == nil {
			cell := s2.CellFromCellID(s2.CellID(cellInt))
			center := s2.LatLngFromPoint(cell.Center())
			lat = center.Lat.Degrees()
			lon = center.Lng.Degrees()
		}
		weatherHook = &Hook{
			Type: "weather",
			Message: map[string]any{
				"latitude":     lat,
				"longitude":    lon,
				"s2_cell_id":   cellID,
				"condition":    weatherID,
				"time_changed": timestamp,
				"source":       "fromMonster",
			},
		}
	} else {
		weatherHook.Message["condition"] = weatherID
		weatherHook.Message["time_changed"] = timestamp
		if weatherHook.Message["s2_cell_id"] == "" {
			weatherHook.Message["s2_cell_id"] = cellID
		}
	}
	weatherHook.Message["_weatherChange"] = true
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	updateHour := timestamp - (timestamp % 3600)
	prevHour := updateHour - 3600
	prevWeather := 0
	if cell := p.weatherData.WeatherInfo(cellID); cell != nil {
		prevWeather = cell.Data[prevHour]
	}
	if prevWeather == weatherID {
		return false
	}
	showAltered := false
	if p.cfg != nil {
		showAltered = getBoolFromConfig(p.cfg, "weather.showAlteredPokemon", false)
	}
	targetIDs := p.weatherData.EligibleTargets(cellID, weatherID, showAltered)
	if len(targetIDs) == 0 {
		return false
	}
	for _, id := range targetIDs {
		entry := p.weatherData.CareEntry(cellID, id)
		if entry == nil {
			continue
		}
		if !p.weatherData.ShouldSendWeather(cellID, id, currentHour) {
			continue
		}
		target := alertTarget{
			ID:       entry.ID,
			Type:     entry.Type,
			Name:     entry.Name,
			Language: entry.Language,
			Template: entry.Template,
			Platform: platformFromType(entry.Type),
		}
		match := alertMatch{
			Target: target,
			Row: map[string]any{
				"ping":  entry.Ping,
				"clean": entry.Clean,
			},
		}
		payload, message := p.formatPayload(weatherHook, match)
		tth := dispatch.TimeToHide{Hours: 1, Minutes: 0, Seconds: 0}
		caresUntil := entry.CaresUntil
		// When altered-pokemon overlays are enabled, align the weather-change alert lifetime with
		// the last pokemon that is actually affected by the weather change (and therefore can be shown).
		if p.cfg != nil {
			if showAltered, _ := p.cfg.GetBool("weather.showAlteredPokemon"); showAltered && p.weatherData != nil {
				if active := p.weatherData.ActivePokemons(cellID, id, weatherID, 0); len(active) > 0 {
					latest := int64(0)
					for _, mon := range active {
						if mon.DisappearTime > latest {
							latest = mon.DisappearTime
						}
					}
					if latest > 0 {
						caresUntil = latest
					}
				}
			}
		}
		if caresUntil > 0 {
			tth = buildTTHFromUnix(caresUntil)
		}
		job := dispatch.MessageJob{
			Lat:          getFloat(weatherHook.Message["latitude"]),
			Lon:          getFloat(weatherHook.Message["longitude"]),
			Message:      message,
			Payload:      payload,
			Target:       target.ID,
			Type:         target.Type,
			Name:         target.Name,
			TTH:          tth,
			Clean:        entry.Clean,
			Emoji:        "",
			LogReference: "Webhook",
			Language:     target.Language,
		}
		if p.rateChecker == nil || job.AlwaysSend {
			p.enqueue(job)
			continue
		}
		additional, sendOriginal := p.applyRateLimit(job)
		for _, extra := range additional {
			p.enqueue(extra)
		}
		if sendOriginal {
			p.enqueue(job)
		}
	}
	return false
}

func buildTTHFromUnix(expireUnix int64) dispatch.TimeToHide {
	if expireUnix <= 0 {
		return dispatch.TimeToHide{Hours: 1}
	}
	remaining := time.Until(time.Unix(expireUnix, 0))
	if remaining < 0 {
		remaining = 0
	}
	total := int(remaining.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return dispatch.TimeToHide{Hours: h, Minutes: m, Seconds: s}
}

func (p *Processor) applyRateLimit(job dispatch.MessageJob) ([]dispatch.MessageJob, bool) {
	if p.rateChecker == nil || p.cfg == nil {
		return nil, true
	}
	destinationID := job.Target
	if job.Type == "webhook" && job.Name != "" {
		destinationID = job.Name
	}
	rate := p.rateChecker.ValidateMessage(destinationID, job.Type)
	if rate.PassMessage {
		return nil, true
	}
	if !rate.JustBreached {
		return nil, false
	}

	tr := p.i18n.Translator(job.Language)
	warning := tr.TranslateFormat("You have reached the limit of {0} messages over {1} seconds", rate.MessageLimit, rate.MessageTTL)
	prefix := "/"
	if strings.HasPrefix(job.Type, "discord") || job.Type == "webhook" {
		prefix, _ = p.cfg.GetString("discord.prefix")
		if prefix == "" {
			prefix = "!"
		}
	}

	logMessage := ""
	shameMessage := ""
	disableOnStop, _ := p.cfg.GetBool("alertLimits.disableOnStop")
	maxLimits, _ := p.cfg.GetInt("alertLimits.maxLimitsBeforeStop")
	if maxLimits > 0 {
		stopRes := p.rateChecker.UserIsBanned(destinationID)
		if !stopRes.PassMessage {
			stopTemplate := "You have breached the rate limit too many times in the last 24 hours. Your messages are now stopped, use {0}start to resume"
			if disableOnStop {
				stopTemplate = "You have breached the rate limit too many times in the last 24 hours. Your messages are now stopped, contact an administrator to resume"
			}
			warning = tr.TranslateFormat(stopTemplate, prefix)

			if shouldDisable(job.Type) && p.query != nil {
				if disableOnStop {
					_, _ = p.query.UpdateQuery("humans", map[string]any{
						"admin_disable": 1,
						"disabled_date": nil,
					}, map[string]any{"id": job.Target})
				} else {
					_, _ = p.query.UpdateQuery("humans", map[string]any{
						"enabled": 0,
					}, map[string]any{"id": job.Target})
				}
			}

			logMessage = fmt.Sprintf("Stopped alerts (rate-limit exceeded too many times) for target %s %s %s", job.Type, destinationID, job.Name)
			if job.Type == "discord:user" {
				shameMessage = tr.TranslateFormat("<@{0}> has had their Poracle tracking disabled for exceeding the rate limit too many times!", destinationID)
			}
		}
	}

	warningJob := job
	warningJob.Message = warning
	warningJob.Payload = map[string]any{"content": warning}
	warningJob.AlwaysSend = true

	jobs := []dispatch.MessageJob{warningJob}
	if logMessage != "" {
		if logChannel, ok := p.cfg.GetString("discord.dmLogChannelID"); ok && logChannel != "" {
			jobs = append(jobs, dispatch.MessageJob{
				Message:      logMessage,
				Payload:      map[string]any{"content": logMessage},
				Target:       logChannel,
				Type:         "discord:channel",
				Name:         "Log channel",
				TTH:          dispatch.TimeToHide{Hours: 0, Minutes: getIntConfig(p.cfg, "discord.dmLogChannelDeletionTime", 0), Seconds: 0},
				Clean:        getIntConfig(p.cfg, "discord.dmLogChannelDeletionTime", 0) > 0,
				LogReference: job.LogReference,
				Language:     job.Language,
				AlwaysSend:   true,
			})
		}
	}
	if shameMessage != "" {
		if shameChannel, ok := p.cfg.GetString("alertLimits.shameChannel"); ok && shameChannel != "" {
			jobs = append(jobs, dispatch.MessageJob{
				Message:      shameMessage,
				Payload:      map[string]any{"content": shameMessage},
				Target:       shameChannel,
				Type:         "discord:channel",
				Name:         "Shame channel",
				TTH:          dispatch.TimeToHide{Hours: 0, Minutes: 0, Seconds: 0},
				Clean:        false,
				LogReference: job.LogReference,
				Language:     job.Language,
				AlwaysSend:   true,
			})
		}
	}
	return jobs, false
}

func shouldDisable(targetType string) bool {
	return strings.Contains(targetType, "user") || strings.Contains(targetType, "channel")
}

func getIntConfig(cfg *config.Config, path string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	value, ok := cfg.GetInt(path)
	if !ok {
		return fallback
	}
	return value
}

func (p *Processor) updateStats(hook *Hook) {
	if p == nil || p.stats == nil || hook == nil {
		return
	}
	pokemonID := getInt(hook.Message["pokemon_id"])
	isShiny := getBool(hook.Message["shiny"])
	if pokemonID > 0 {
		ivScanned := hook.Message["individual_defense"] != nil ||
			hook.Message["individual_attack"] != nil ||
			hook.Message["individual_stamina"] != nil
		p.stats.Update(pokemonID, ivScanned, isShiny)
	}
}

func (p *Processor) applyPvp(hook *Hook) {
	if p == nil || hook == nil || p.cfg == nil || p.pvpCalc == nil {
		return
	}
	source, _ := p.cfg.GetString("pvp.dataSource")
	if source == "" {
		source = "webhook"
	}
	source = strings.ToLower(source)
	if source != "internal" && source != "compare" {
		return
	}

	atk, okAtk := getIntFromKeys(hook.Message, "individual_attack", "atk")
	def, okDef := getIntFromKeys(hook.Message, "individual_defense", "def")
	sta, okSta := getIntFromKeys(hook.Message, "individual_stamina", "sta")
	if !okAtk || !okDef || !okSta {
		return
	}

	pokemonID := getInt(hook.Message["pokemon_id"])
	formID := getInt(hook.Message["form"])
	if pokemonID == 0 {
		return
	}

	includeEvolution, _ := p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")
	includeMega, _ := p.cfg.GetBool("pvp.includeMegaEvolution")
	littleLeagueCanEvolve, _ := p.cfg.GetBool("pvp.littleLeagueCanEvolve")

	ranks := p.pvpCalc.Rankings(pokemonID, formID, atk, def, sta, includeEvolution)
	filtered := map[int][]pvp.Entry{}
	for league, entries := range ranks {
		allowed := []pvp.Entry{}
		for _, entry := range entries {
			if !includeMega && entry.Evolution {
				if isMegaForm(p.pvpCalc, entry.PokemonID, entry.FormID) {
					continue
				}
			}
			if league == 500 && !littleLeagueCanEvolve && entry.Evolution {
				continue
			}
			allowed = append(allowed, entry)
		}
		if len(allowed) > 0 {
			filtered[league] = allowed
		}
	}

	if source == "compare" {
		hook.Message["ohbem_pvp"] = pvpEntriesToPayload(filtered)
		return
	}
	for league, entries := range filtered {
		key := pvpLeagueKey(league)
		if key == "" {
			continue
		}
		hook.Message[key] = pvpEntriesToPayload(map[int][]pvp.Entry{league: entries})[league]
	}
}

func pvpLeagueKey(league int) string {
	switch league {
	case 1500:
		return "pvp_rankings_great_league"
	case 2500:
		return "pvp_rankings_ultra_league"
	case 500:
		return "pvp_rankings_little_league"
	default:
		return ""
	}
}

func pvpEntriesToPayload(entries map[int][]pvp.Entry) map[int][]map[string]any {
	out := map[int][]map[string]any{}
	for league, list := range entries {
		payload := []map[string]any{}
		for _, entry := range list {
			item := map[string]any{
				"pokemon":    entry.PokemonID,
				"form":       entry.FormID,
				"rank":       entry.Rank,
				"cp":         entry.CP,
				"level":      entry.Level,
				"percentage": entry.Percentage,
				"cap":        entry.Cap,
				"caps":       entry.Caps,
				"evolution":  entry.Evolution,
			}
			payload = append(payload, item)
		}
		out[league] = payload
	}
	return out
}

func getIntFromKeys(values map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return getInt(value), true
		}
	}
	return 0, false
}

func isMegaForm(calc *pvp.Calculator, pokemonID, formID int) bool {
	if calc == nil {
		return false
	}
	entry := calc.Lookup(pokemonID, formID)
	if entry == nil {
		return false
	}
	form, ok := entry["form"].(map[string]any)
	if !ok {
		return false
	}
	name, _ := form["name"].(string)
	name = strings.ToLower(name)
	return strings.Contains(name, "mega") || strings.Contains(name, "primal")
}

func (p *Processor) disabled(path string) bool {
	if p.cfg == nil {
		return false
	}
	value, _ := p.cfg.GetBool(path)
	return value
}

func (p *Processor) dedupePokemon(hook *Hook) bool {
	encounter := getString(hook.Message["encounter_id"])
	if encounter == "" {
		return true
	}
	verified := getBool(hook.Message["verified"]) || getBool(hook.Message["disappear_time_verified"])
	cp := getString(hook.Message["cp"])
	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		pokemonID = getInt(hook.Message["pokemonId"])
	}
	formID := getInt(hook.Message["form"])
	costume := getInt(hook.Message["costume"])
	gender := getInt(hook.Message["gender"])
	key := fmt.Sprintf("%s%t%s%d:%d:%d:%d", encounter, verified, cp, pokemonID, formID, costume, gender)
	if p.cache.Get(key) {
		return false
	}
	var ttl time.Duration
	if !verified {
		ttl = time.Hour
	} else {
		disappear := getInt64(hook.Message["disappear_time"])
		if disappear > 0 {
			ttl = expiryTTL(disappear, 5*time.Minute)
		} else {
			// PoracleJS computes `max((disappear_time-now), 0) + 300` seconds; treat missing/zero as 5 minutes.
			ttl = 5 * time.Minute
		}
	}
	p.cache.Set(key, ttl)
	return true
}

func (p *Processor) dedupeRaid(hook *Hook) bool {
	gymID := getString(hook.Message["gym_id"])
	if gymID == "" {
		gymID = getString(hook.Message["id"])
	}
	end := getString(hook.Message["end"])
	pokemonID := getString(hook.Message["pokemon_id"])
	key := fmt.Sprintf("%s%s%s", gymID, end, pokemonID)
	if key == "" {
		return true
	}
	p.raidSeenMu.Lock()
	defer p.raidSeenMu.Unlock()
	now := time.Now()
	newEntries := raidSignatureEntries(hook.Message["rsvps"])
	signature := raidEncodeEntries(newEntries)
	oldEntry, seen := p.raidSeen[key]
	if seen && !oldEntry.Expires.IsZero() && now.After(oldEntry.Expires) {
		delete(p.raidSeen, key)
		seen = false
	}
	if seen {
		oldEntries := raidDecodeEntries(oldEntry.Signature)
		if !raidHasRsvpDifference(oldEntries, newEntries) {
			return false
		}
	}
	if signature == "" {
		signature = "none"
	}
	// PoracleJS stores raid dedupe keys in a NodeCache with stdTTL=5400s (90m).
	p.raidSeen[key] = raidSeenEntry{Signature: signature, Expires: now.Add(90 * time.Minute)}
	hook.Message["firstNotification"] = !seen
	return true
}

func (p *Processor) dedupeMaxBattle(hook *Hook) bool {
	if p == nil || p.cache == nil || hook == nil || hook.Message == nil {
		return true
	}
	stationID := getString(hook.Message["id"])
	if stationID == "" {
		stationID = getString(hook.Message["stationId"])
	}
	if stationID == "" {
		return true
	}
	end := getInt64(hook.Message["battle_end"])
	if end == 0 {
		end = hookExpiryUnix(hook)
	}
	pokemonID := getInt(hook.Message["battle_pokemon_id"])
	if pokemonID == 0 {
		pokemonID = getInt(hook.Message["pokemon_id"])
	}
	form := getInt(hook.Message["battle_pokemon_form"])
	if form == 0 {
		form = getInt(hook.Message["form"])
	}
	level := getInt(hook.Message["battle_level"])
	if level == 0 {
		level = getInt(hook.Message["level"])
	}

	// PoracleJS uses a NodeCache (stdTTL=5400s). The PR-929 cache key is `${station_id}${battle_end}${battle_pokemon_id}`.
	// For Go, keep this maxbattle-specific and align expiry to battle_end when present, so it remains useful for long battles.
	key := fmt.Sprintf("maxbattle:%s:%d:%d:%d:%d", stationID, end, pokemonID, form, level)
	if p.cache.Get(key) {
		return false
	}
	ttl := 90 * time.Minute
	if end > 0 {
		ttl = expiryTTL(end, 5*time.Minute)
	}
	p.cache.Set(key, ttl)
	return true
}

type raidRsvpEntry struct {
	Time  string `json:"time"`
	Going int    `json:"going"`
	Maybe int    `json:"maybe"`
}

func raidSignatureEntries(raw any) []raidRsvpEntry {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	entries := make([]raidRsvpEntry, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		entries = append(entries, raidRsvpEntry{
			Time:  getString(m["timeslot"]),
			Going: getInt(m["going_count"]),
			Maybe: getInt(m["maybe_count"]),
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Time < entries[j].Time })
	return entries
}

func raidEncodeEntries(entries []raidRsvpEntry) string {
	if len(entries) == 0 {
		return ""
	}
	encoded, _ := json.Marshal(entries)
	return string(encoded)
}

func raidDecodeEntries(signature string) []raidRsvpEntry {
	if signature == "" || signature == "none" {
		return nil
	}
	var entries []raidRsvpEntry
	if err := json.Unmarshal([]byte(signature), &entries); err != nil {
		return nil
	}
	return entries
}

func raidHasRsvpDifference(oldEntries, newEntries []raidRsvpEntry) bool {
	if len(oldEntries) == 0 && len(newEntries) == 0 {
		return false
	}
	oldMap := map[string]raidRsvpEntry{}
	for _, entry := range oldEntries {
		oldMap[entry.Time] = entry
	}
	newMap := map[string]raidRsvpEntry{}
	for _, entry := range newEntries {
		newMap[entry.Time] = entry
	}
	if len(oldMap) != len(newMap) {
		return true
	}
	for _, entry := range newEntries {
		prev, ok := oldMap[entry.Time]
		if !ok {
			return true
		}
		if prev.Going != entry.Going || prev.Maybe != entry.Maybe {
			return true
		}
	}
	return false
}

func (p *Processor) handlePokestop(hook *Hook) {
	pokestopID := getString(hook.Message["pokestop_id"])
	if pokestopID == "" {
		pokestopID = getString(hook.Message["id"])
	}
	if pokestopID != "" && getString(hook.Message["pokestop_id"]) == "" {
		// Match PoracleJS expectations: templates/logging assume `pokestop_id` is present.
		hook.Message["pokestop_id"] = pokestopID
	}
	incidentExpiration := getInt64(hook.Message["incident_expiration"])
	if incidentExpiration == 0 {
		incidentExpiration = getInt64(hook.Message["incident_expire_timestamp"])
	}
	lureExpiration := getInt64(hook.Message["lure_expiration"])
	if lureExpiration == 0 && incidentExpiration == 0 {
		return
	}

	if lureExpiration > 0 && !p.disabled("general.disableLure") {
		key := fmt.Sprintf("%sL%d", pokestopID, lureExpiration)
		if !p.cache.Get(key) {
			ttl := expiryTTL(lureExpiration, 5*time.Minute)
			p.cache.Set(key, ttl)
			hook.Type = "lure"
			if !p.shouldSkipMinimumTime(hook) {
				p.dispatch(hook)
			}
		}
	}

	displayType := getInt(hook.Message["display_type"])
	if incidentExpiration > 0 && !p.disabled("general.disableInvasion") {
		if !p.disabled("general.disableUnconfirmedInvasion") || displayType > 6 {
			key := fmt.Sprintf("%sI%d", pokestopID, incidentExpiration)
			if !p.cache.Get(key) {
				ttl := expiryTTL(incidentExpiration, 5*time.Minute)
				p.cache.Set(key, ttl)
				hook.Type = "invasion"
				if !p.shouldSkipMinimumTime(hook) {
					p.dispatch(hook)
				}
			}
		}
	}

	confirmed := getBool(hook.Message["confirmed"])
	if confirmed && !p.disabled("general.disableInvasion") && p.enabled("general.processConfirmedInvasionLineups") {
		key := fmt.Sprintf("%sI%dH02", pokestopID, incidentExpiration)
		if !p.cache.Get(key) {
			ttl := expiryTTL(incidentExpiration, 5*time.Minute)
			if ttl <= 0 {
				// Some providers don't include an expiration timestamp for confirmed lineups.
				// Avoid storing a non-expiring dedupe key in that case.
				ttl = 90 * time.Minute
			}
			p.cache.Set(key, ttl)
			hook.Type = "invasion"
			if !p.shouldSkipMinimumTime(hook) {
				p.dispatch(hook)
			}
		}
	}
}

func (p *Processor) dedupeQuest(hook *Hook) bool {
	pokestopID := getString(hook.Message["pokestop_id"])
	rewardsBytes, _ := json.Marshal(hook.Message["rewards"])
	withAR := getBool(hook.Message["with_ar"])
	key := fmt.Sprintf("%s_%s_%t", pokestopID, string(rewardsBytes), withAR)
	if p.cache.Get(key) {
		return false
	}
	// PoracleJS uses NodeCache with stdTTL=5400s (90m) for quest dedupe keys.
	p.cache.Set(key, 90*time.Minute)
	return true
}

func (p *Processor) dedupeGym(hook *Hook) bool {
	id := getString(hook.Message["id"])
	if id == "" {
		id = getString(hook.Message["gym_id"])
	}
	team := teamFromHookMessage(hook.Message)
	inBattle := gymInBattle(hook.Message)
	cacheKey := fmt.Sprintf("%s_battle", id)
	if inBattle {
		p.cache.Set(cacheKey, 5*time.Minute)
	}
	cached := p.gymCache.Get(id)
	if cached != nil {
		if cached.TeamID == team && cached.SlotsAvailable == getInt(hook.Message["slots_available"]) && p.cache.Get(cacheKey) {
			return false
		}
		hook.Message["old_team_id"] = cached.TeamID
		hook.Message["old_slots_available"] = cached.SlotsAvailable
		hook.Message["old_in_battle"] = cached.InBattle
		hook.Message["last_owner_id"] = cached.LastOwnerID
	} else {
		// Match PoracleJS: populate "old_*" fields with -1 when there is no cached state.
		hook.Message["old_team_id"] = -1
		hook.Message["old_slots_available"] = -1
		hook.Message["old_in_battle"] = -1
		hook.Message["last_owner_id"] = -1
	}
	lastOwner := getInt(hook.Message["last_owner_id"])
	if team != 0 {
		lastOwner = team
	}
	p.gymCache.Set(id, GymState{
		TeamID:         team,
		SlotsAvailable: getInt(hook.Message["slots_available"]),
		LastOwnerID:    lastOwner,
		InBattle:       inBattle,
	})
	return true
}

func (p *Processor) dedupeNest(hook *Hook) bool {
	nestID := getString(hook.Message["nest_id"])
	pokemonID := getString(hook.Message["pokemon_id"])
	reset := getInt64(hook.Message["reset_time"])
	key := fmt.Sprintf("%s_%s_%d", nestID, pokemonID, reset)
	if p.cache.Get(key) {
		return false
	}
	ttl := expiryTTL(reset+14*24*60*60, 0)
	if ttl <= 0 {
		// If reset_time is missing/zero, avoid creating a non-expiring dedupe key.
		// Use a long TTL so nests won't re-alert "early" if resent.
		ttl = 14 * 24 * time.Hour
	}
	p.cache.Set(key, ttl)
	return true
}

func (p *Processor) dedupeWeather(hook *Hook) bool {
	cell := getString(hook.Message["s2_cell_id"])
	if cell == "" {
		cell = fmt.Sprintf("%v,%v", hook.Message["latitude"], hook.Message["longitude"])
	}
	updated := getInt64(hook.Message["time_changed"])
	if updated == 0 {
		updated = getInt64(hook.Message["updated"])
	}
	if updated == 0 {
		return true
	}
	hourStamp := updated - (updated % 3600)
	key := fmt.Sprintf("%s_%d", cell, hourStamp)
	if p.cache.Get(key) {
		return false
	}
	// PoracleJS uses NodeCache with stdTTL=5400s (90m) for weather dedupe keys.
	p.cache.Set(key, 90*time.Minute)
	return true
}

func (p *Processor) enabled(path string) bool {
	if p.cfg == nil {
		return false
	}
	value, _ := p.cfg.GetBool(path)
	return value
}

func normalizeHook(item any) (*Hook, bool) {
	raw, ok := item.(map[string]any)
	if !ok {
		return nil, false
	}
	hookType, ok := raw["type"].(string)
	if !ok || hookType == "" {
		return nil, false
	}
	message := map[string]any{}
	switch v := raw["message"].(type) {
	case map[string]any:
		message = v
	default:
		for key, value := range raw {
			if key == "type" {
				continue
			}
			message[key] = value
		}
	}
	return &Hook{Type: hookType, Message: message}, true
}

func expiryTTL(expireUnix int64, buffer time.Duration) time.Duration {
	if expireUnix == 0 {
		return 0
	}
	remaining := time.Until(time.Unix(expireUnix, 0))
	if remaining < 0 {
		remaining = 0
	}
	return remaining + buffer
}

func getString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	}
}

func getInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return int(parsed)
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return int(parsed)
		}
	}
	return 0
}

func getInt64(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case float64:
		return int64(v)
	case float32:
		return int64(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.ParseInt(strings.TrimSpace(string(v)), 10, 64); err == nil {
			return parsed
		}
	}
	return 0
}

func getBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v == "true" || v == "1"
	case []byte:
		s := strings.TrimSpace(string(v))
		return s == "true" || s == "1"
	}
	return false
}

// gymInBattle mirrors PoracleJS gym handling:
// `const inBattle = hook.message.is_in_battle ?? hook.message.in_battle ?? 0`
// followed by normal JS truthiness checks.
func gymInBattle(message map[string]any) bool {
	if message == nil {
		return false
	}
	if raw, ok := message["is_in_battle"]; ok && raw != nil {
		return jsTruthy(raw)
	}
	if raw, ok := message["in_battle"]; ok && raw != nil {
		return jsTruthy(raw)
	}
	return false
}

func jsTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case float32:
		return v != 0
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed != 0
		}
		return false
	case string:
		return v != ""
	case []byte:
		return len(v) > 0
	default:
		// Non-null objects/arrays are truthy in JS.
		return true
	}
}

func teamFromHookMessage(message map[string]any) int {
	if message == nil {
		return 0
	}
	// Match PoracleJS nullish coalescing behavior: `team_id ?? team` (0 is a real value).
	if raw, ok := message["team_id"]; ok && raw != nil {
		return getInt(raw)
	}
	if raw, ok := message["team"]; ok && raw != nil {
		return getInt(raw)
	}
	return 0
}

func getFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return parsed
		}
	}
	return 0
}
