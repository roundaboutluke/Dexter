package webhook

import (
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"dexter/internal/config"
	"dexter/internal/geo"
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
