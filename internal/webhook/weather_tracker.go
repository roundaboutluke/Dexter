package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"poraclego/internal/config"
	"poraclego/internal/geo"
)

// WeatherCell stores cached weather data for a cell.
type WeatherCell struct {
	Data                    map[int64]int `json:"data"`
	LastForecastLoad        int64         `json:"lastForecastLoad"`
	ForecastTimeout         int64         `json:"forecastTimeout"`
	LastCurrentWeatherCheck int64         `json:"lastCurrentWeatherCheck"`
	LocationKey             string        `json:"locationKey"`
}

// WeatherTracker stores recent weather state and optional forecasts.
type WeatherTracker struct {
	cfg      *config.Config
	client   *http.Client
	cacheDir string

	mu       sync.Mutex
	cells    map[string]*WeatherCell
	cares    map[string]*weatherCareCell
	boosts   map[string]*boostState
	keyUsage map[string]map[string]int
}

type boostState struct {
	WeatherFromBoost []int
	CurrentHour      int64
	MonsterWeather   int
}

type weatherCareCell struct {
	Cares map[string]*weatherCareEntry `json:"cares"`
}

type weatherCareEntry struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Type            string         `json:"type"`
	Clean           bool           `json:"clean"`
	Ping            string         `json:"ping"`
	Template        string         `json:"template"`
	Language        string         `json:"language"`
	CaresUntil      int64          `json:"caresUntil"`
	LastChangeAlert int64          `json:"lastChangeAlert"`
	CaredPokemons   []caredPokemon `json:"caredPokemons,omitempty"`
}

type caredPokemon struct {
	PokemonID        int     `json:"pokemon_id"`
	Form             int     `json:"form"`
	Name             string  `json:"name"`
	FormName         string  `json:"formName"`
	FullName         string  `json:"fullName"`
	IV               string  `json:"iv"`
	CP               int     `json:"cp"`
	Latitude         float64 `json:"latitude"`
	Longitude        float64 `json:"longitude"`
	DisappearTime    int64   `json:"disappear_time"`
	AlteringWeathers []int   `json:"alteringWeathers"`
}

// NewWeatherTracker creates a weather tracker and loads cache data.
func NewWeatherTracker(cfg *config.Config, root string) *WeatherTracker {
	tracker := &WeatherTracker{
		cfg:      cfg,
		client:   &http.Client{Timeout: 15 * time.Second},
		cacheDir: filepath.Join(root, ".cache"),
		cells:    map[string]*WeatherCell{},
		cares:    map[string]*weatherCareCell{},
		boosts:   map[string]*boostState{},
		keyUsage: map[string]map[string]int{},
	}
	tracker.loadCaches()
	return tracker
}

// UpdateFromHook updates weather data using a webhook payload.
func (w *WeatherTracker) UpdateFromHook(hook *Hook) {
	if w == nil || hook == nil {
		return
	}
	cellID := getString(hook.Message["s2_cell_id"])
	if cellID == "" {
		lat := getFloat(hook.Message["latitude"])
		lon := getFloat(hook.Message["longitude"])
		cellID = geo.WeatherCellID(lat, lon)
	}
	if cellID == "" {
		return
	}
	condition := weatherCondition(hook.Message)
	if condition == 0 {
		return
	}
	timestamp := getInt64(hook.Message["time_changed"])
	if timestamp == 0 {
		timestamp = getInt64(hook.Message["updated"])
	}
	if timestamp == 0 {
		timestamp = time.Now().Unix()
	}
	updateHour := timestamp - (timestamp % 3600)
	currentHour := time.Now().Unix()
	currentHour -= currentHour % 3600

	w.mu.Lock()
	cell := w.ensureCell(cellID)
	cell.Data[updateHour] = condition
	cell.LastCurrentWeatherCheck = updateHour
	w.expireCellLocked(cell, currentHour)
	w.mu.Unlock()
}

// CheckWeatherOnMonster updates weather data when boosted pokemon indicate a change.
// Returns true when a new weather change is detected for the current hour.
func (w *WeatherTracker) CheckWeatherOnMonster(cellID string, lat, lon float64, monsterWeather int) bool {
	if w == nil || monsterWeather == 0 {
		return false
	}
	if cellID == "" {
		cellID = geo.WeatherCellID(lat, lon)
	}
	if cellID == "" {
		return false
	}
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	previousHour := currentHour - 3600
	if now <= currentHour+30 {
		return false
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.ensureCell(cellID)
	state := w.boosts[cellID]
	if state == nil {
		state = &boostState{WeatherFromBoost: make([]int, 8)}
		w.boosts[cellID] = state
	}
	if cell.LastCurrentWeatherCheck == 0 {
		cell.LastCurrentWeatherCheck = previousHour
	}
	if cell.Data[currentHour] == monsterWeather && cell.LastCurrentWeatherCheck >= currentHour {
		resetBoostState(state)
		return false
	}
	for i := range state.WeatherFromBoost {
		if i == monsterWeather {
			state.WeatherFromBoost[i]++
		} else {
			state.WeatherFromBoost[i]--
		}
	}
	changed := false
	for _, count := range state.WeatherFromBoost {
		if count > 4 {
			resetBoostState(state)
			if state.CurrentHour != currentHour || state.MonsterWeather != monsterWeather {
				state.CurrentHour = currentHour
				state.MonsterWeather = monsterWeather
				cell.Data[currentHour] = monsterWeather
				cell.LastCurrentWeatherCheck = currentHour
				changed = true
			}
			break
		}
	}
	return changed
}

func resetBoostState(state *boostState) {
	if state == nil {
		return
	}
	for i := range state.WeatherFromBoost {
		state.WeatherFromBoost[i] = 0
	}
}

// WeatherInfo returns the stored weather info for a cell.
func (w *WeatherTracker) WeatherInfo(cellID string) *WeatherCell {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cells[cellID]
	if cell == nil {
		return nil
	}
	copy := &WeatherCell{
		Data:                    map[int64]int{},
		LastForecastLoad:        cell.LastForecastLoad,
		ForecastTimeout:         cell.ForecastTimeout,
		LastCurrentWeatherCheck: cell.LastCurrentWeatherCheck,
		LocationKey:             cell.LocationKey,
	}
	for key, value := range cell.Data {
		copy.Data[key] = value
	}
	return copy
}

func (w *WeatherTracker) TrackCare(cellID string, target alertTarget, caresUntil int64, clean bool, ping string, pokemon *caredPokemon) {
	if w == nil || cellID == "" || target.ID == "" {
		return
	}
	if caresUntil == 0 {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.ensureCareCell(cellID)
	entry := cell.Cares[target.ID]
	if entry == nil {
		entry = &weatherCareEntry{ID: target.ID}
		cell.Cares[target.ID] = entry
	}
	entry.Name = target.Name
	entry.Type = target.Type
	entry.Template = target.Template
	entry.Language = target.Language
	entry.Clean = clean
	entry.Ping = ping
	entry.CaresUntil = maxInt64(entry.CaresUntil, caresUntil)
	// PoracleJS only stores cared-pokemon details when altered-pokemon overlays are enabled.
	if !getBoolFromConfig(w.cfg, "weather.showAlteredPokemon", false) {
		entry.CaredPokemons = nil
		return
	}
	if pokemon != nil {
		if !hasCaredPokemon(entry.CaredPokemons, *pokemon) {
			entry.CaredPokemons = append(entry.CaredPokemons, *pokemon)
		}
	}
}

func (w *WeatherTracker) EligibleTargets(cellID string, weatherID int, showAltered bool) []string {
	if w == nil || cellID == "" {
		return nil
	}
	now := time.Now().Unix()
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return nil
	}
	targets := []string{}
	for id, entry := range cell.Cares {
		if entry.CaresUntil < now {
			delete(cell.Cares, id)
			continue
		}
		entry.CaredPokemons = filterActivePokemons(entry.CaredPokemons, now)
		if showAltered {
			if !hasAlteredPokemon(entry.CaredPokemons, weatherID) {
				continue
			}
		}
		targets = append(targets, id)
	}
	return targets
}

func (w *WeatherTracker) CareEntry(cellID, targetID string) *weatherCareEntry {
	if w == nil || cellID == "" || targetID == "" {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return nil
	}
	entry := cell.Cares[targetID]
	if entry == nil {
		return nil
	}
	copy := *entry
	copy.CaredPokemons = append([]caredPokemon(nil), entry.CaredPokemons...)
	return &copy
}

func (w *WeatherTracker) ShouldSendWeather(cellID, targetID string, hour int64) bool {
	if w == nil || cellID == "" || targetID == "" {
		return false
	}
	now := time.Now().Unix()
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return false
	}
	entry := cell.Cares[targetID]
	if entry == nil {
		return false
	}
	if entry.CaresUntil < now {
		delete(cell.Cares, targetID)
		return false
	}
	if entry.LastChangeAlert == hour {
		return false
	}
	entry.LastChangeAlert = hour
	return true
}

func (w *WeatherTracker) ActivePokemons(cellID, targetID string, weatherID int, maxCount int) []caredPokemon {
	if w == nil || cellID == "" || targetID == "" {
		return nil
	}
	now := time.Now().Unix()
	w.mu.Lock()
	defer w.mu.Unlock()
	cell := w.cares[cellID]
	if cell == nil {
		return nil
	}
	entry := cell.Cares[targetID]
	if entry == nil {
		return nil
	}
	entry.CaredPokemons = filterActivePokemons(entry.CaredPokemons, now)
	out := make([]caredPokemon, 0, len(entry.CaredPokemons))
	for _, mon := range entry.CaredPokemons {
		if weatherID > 0 && !containsInt(mon.AlteringWeathers, weatherID) {
			continue
		}
		out = append(out, mon)
		if maxCount > 0 && len(out) >= maxCount {
			break
		}
	}
	return out
}

// EnsureForecast refreshes forecast data if needed and returns a snapshot.
func (w *WeatherTracker) EnsureForecast(cellID string, lat, lon float64) *WeatherCell {
	if w == nil || cellID == "" {
		return w.WeatherInfo(cellID)
	}
	enabled, _ := w.cfg.GetBool("weather.enableWeatherForecast")
	if !enabled {
		return w.WeatherInfo(cellID)
	}
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	nextHour := currentHour + 3600

	w.mu.Lock()
	cell := w.ensureCell(cellID)
	needFetch := cell.Data[nextHour] == 0 || cell.ForecastTimeout <= currentHour
	smart, _ := w.cfg.GetBool("weather.smartForecast")
	if smart && cell.Data[nextHour] != 0 && cell.ForecastTimeout > currentHour {
		needFetch = false
	}
	if needFetch && cell.LastForecastLoad != currentHour {
		cell.LastForecastLoad = currentHour
		w.mu.Unlock()
		w.fetchForecast(cellID, lat, lon)
	} else {
		w.mu.Unlock()
	}
	return w.WeatherInfo(cellID)
}

// SaveCaches persists weather caches to disk.
func (w *WeatherTracker) SaveCaches() {
	if w == nil || w.cacheDir == "" {
		return
	}
	_ = os.MkdirAll(w.cacheDir, 0o755)
	w.mu.Lock()
	cellCopy := map[string]*WeatherCell{}
	for key, cell := range w.cells {
		c := &WeatherCell{
			Data:                    map[int64]int{},
			LastForecastLoad:        cell.LastForecastLoad,
			ForecastTimeout:         cell.ForecastTimeout,
			LastCurrentWeatherCheck: cell.LastCurrentWeatherCheck,
			LocationKey:             cell.LocationKey,
		}
		for ts, value := range cell.Data {
			c.Data[ts] = value
		}
		cellCopy[key] = c
	}
	keyCopy := map[string]map[string]int{}
	for day, usage := range w.keyUsage {
		u := map[string]int{}
		for key, count := range usage {
			u[key] = count
		}
		keyCopy[day] = u
	}
	careCopy := map[string]*weatherCareCell{}
	for cellID, cell := range w.cares {
		c := &weatherCareCell{Cares: map[string]*weatherCareEntry{}}
		for id, entry := range cell.Cares {
			c.Cares[id] = &weatherCareEntry{
				ID:              entry.ID,
				Name:            entry.Name,
				Type:            entry.Type,
				Clean:           entry.Clean,
				Ping:            entry.Ping,
				Template:        entry.Template,
				Language:        entry.Language,
				CaresUntil:      entry.CaresUntil,
				LastChangeAlert: entry.LastChangeAlert,
				CaredPokemons:   append([]caredPokemon(nil), entry.CaredPokemons...),
			}
		}
		careCopy[cellID] = c
	}
	w.mu.Unlock()
	_ = saveJSONFile(filepath.Join(w.cacheDir, "weatherCache.json"), cellCopy)
	_ = saveJSONFile(filepath.Join(w.cacheDir, "weatherKeyCache.json"), keyCopy)
	_ = saveJSONFile(filepath.Join(w.cacheDir, "weatherCares.json"), careCopy)
}

func (w *WeatherTracker) loadCaches() {
	if w == nil || w.cacheDir == "" {
		return
	}
	var cells map[string]*WeatherCell
	if err := loadJSONFile(filepath.Join(w.cacheDir, "weatherCache.json"), &cells); err == nil && cells != nil {
		w.cells = cells
	}
	var usage map[string]map[string]int
	if err := loadJSONFile(filepath.Join(w.cacheDir, "weatherKeyCache.json"), &usage); err == nil && usage != nil {
		w.keyUsage = usage
	}
	var cares map[string]*weatherCareCell
	if err := loadJSONFile(filepath.Join(w.cacheDir, "weatherCares.json"), &cares); err == nil && cares != nil {
		w.cares = cares
	}
}

func (w *WeatherTracker) ensureCell(cellID string) *WeatherCell {
	cell := w.cells[cellID]
	if cell == nil {
		cell = &WeatherCell{Data: map[int64]int{}}
		w.cells[cellID] = cell
	}
	if cell.Data == nil {
		cell.Data = map[int64]int{}
	}
	return cell
}

func (w *WeatherTracker) ensureCareCell(cellID string) *weatherCareCell {
	cell := w.cares[cellID]
	if cell == nil {
		cell = &weatherCareCell{Cares: map[string]*weatherCareEntry{}}
		w.cares[cellID] = cell
	}
	if cell.Cares == nil {
		cell.Cares = map[string]*weatherCareEntry{}
	}
	return cell
}

func (w *WeatherTracker) expireCellLocked(cell *WeatherCell, currentHour int64) {
	if cell == nil {
		return
	}
	for ts := range cell.Data {
		if ts < currentHour-3600 {
			delete(cell.Data, ts)
		}
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func hasCaredPokemon(list []caredPokemon, candidate caredPokemon) bool {
	for _, item := range list {
		if item.PokemonID == candidate.PokemonID && item.Form == candidate.Form && item.DisappearTime == candidate.DisappearTime {
			return true
		}
	}
	return false
}

func filterActivePokemons(list []caredPokemon, now int64) []caredPokemon {
	if len(list) == 0 {
		return list
	}
	out := make([]caredPokemon, 0, len(list))
	for _, item := range list {
		if item.DisappearTime <= now {
			continue
		}
		out = append(out, item)
	}
	return out
}

func hasAlteredPokemon(list []caredPokemon, weatherID int) bool {
	if weatherID == 0 {
		return len(list) > 0
	}
	for _, item := range list {
		if containsInt(item.AlteringWeathers, weatherID) {
			return true
		}
	}
	return false
}

func (w *WeatherTracker) fetchForecast(cellID string, lat, lon float64) {
	if w == nil {
		return
	}
	key := w.nextWeatherKey()
	if key == "" {
		return
	}
	locationKey := w.locationKey(cellID, key, lat, lon)
	if locationKey == "" {
		return
	}
	entries := w.hourlyForecast(key, locationKey)
	if len(entries) == 0 {
		return
	}
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	refreshHours, _ := w.cfg.GetInt("weather.forecastRefreshInterval")
	forecastTimeout := currentHour + int64(refreshHours*3600)

	w.mu.Lock()
	cell := w.ensureCell(cellID)
	for _, entry := range entries {
		hour := entry.Timestamp - (entry.Timestamp % 3600)
		cell.Data[hour] = entry.WeatherID
	}
	cell.ForecastTimeout = forecastTimeout
	w.mu.Unlock()
}

func (w *WeatherTracker) locationKey(cellID, apiKey string, lat, lon float64) string {
	w.mu.Lock()
	cell := w.ensureCell(cellID)
	if cell.LocationKey != "" {
		key := cell.LocationKey
		w.mu.Unlock()
		return key
	}
	w.mu.Unlock()
	endpoint := fmt.Sprintf("https://dataservice.accuweather.com/locations/v1/cities/geoposition/search?apikey=%s&q=%f,%f", apiKey, lat, lon)
	resp, err := w.client.Get(endpoint)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var payload struct {
		Key string `json:"Key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	if payload.Key == "" {
		return ""
	}
	w.mu.Lock()
	cell = w.ensureCell(cellID)
	cell.LocationKey = payload.Key
	w.mu.Unlock()
	return payload.Key
}

type forecastEntry struct {
	Timestamp int64
	WeatherID int
}

func (w *WeatherTracker) hourlyForecast(apiKey, locationKey string) []forecastEntry {
	endpoint := fmt.Sprintf("https://dataservice.accuweather.com/forecasts/v1/hourly/12hour/%s?apikey=%s&details=true&metric=true", locationKey, apiKey)
	resp, err := w.client.Get(endpoint)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var payload []struct {
		DateTime    string `json:"DateTime"`
		WeatherIcon int    `json:"WeatherIcon"`
		Wind        struct {
			Speed struct {
				Value float64 `json:"Value"`
			} `json:"Speed"`
		} `json:"Wind"`
		WindGust struct {
			Speed struct {
				Value float64 `json:"Value"`
			} `json:"Speed"`
		} `json:"WindGust"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	out := []forecastEntry{}
	for _, entry := range payload {
		ts, err := time.Parse(time.RFC3339, entry.DateTime)
		if err != nil {
			continue
		}
		weatherID := mapPoGoWeather(entry.WeatherIcon, entry.Wind.Speed.Value, entry.WindGust.Speed.Value)
		out = append(out, forecastEntry{
			Timestamp: ts.Unix(),
			WeatherID: weatherID,
		})
	}
	return out
}

func (w *WeatherTracker) nextWeatherKey() string {
	keys, _ := w.cfg.GetStringSlice("weather.apiKeyAccuWeather")
	if len(keys) == 0 {
		return ""
	}
	quota, _ := w.cfg.GetInt("weather.apiKeyDayQuota")
	if quota <= 0 {
		quota = 1
	}
	dayKey := time.Now().Format("2006-1-2")
	w.mu.Lock()
	usage := w.keyUsage[dayKey]
	if usage == nil {
		usage = map[string]int{}
		w.keyUsage[dayKey] = usage
	}
	var selected string
	minUsed := int(^uint(0) >> 1)
	for _, key := range keys {
		count := usage[key]
		if count < minUsed {
			minUsed = count
			selected = key
		}
	}
	if selected == "" || usage[selected] >= quota {
		w.mu.Unlock()
		return ""
	}
	usage[selected]++
	w.mu.Unlock()
	return selected
}

func weatherCondition(msg map[string]any) int {
	for _, key := range []string{"gameplay_weather", "gameplay_condition", "condition", "weather"} {
		if value, ok := msg[key]; ok {
			if id := getInt(value); id != 0 {
				return id
			}
		}
	}
	return 0
}

func mapPoGoWeather(icon int, windSpeed, windGust float64) int {
	switch icon {
	case 1, 2, 33, 34:
		if windSpeed > 20 || windGust > 30 {
			return 5
		}
		return 1
	case 3, 4, 35, 36:
		return 3
	case 5, 6, 7, 8, 37, 38:
		return 4
	case 11:
		return 7
	case 12, 15, 18, 26, 29:
		return 2
	case 13, 16, 20, 23, 40, 42:
		return 4
	case 14, 17, 21, 39, 41:
		return 3
	case 19, 22, 24, 25, 43, 44:
		return 6
	case 32:
		return 5
	default:
		return 0
	}
}
