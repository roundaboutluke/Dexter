package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dexter/internal/config"
	"dexter/internal/logging"
)

// WeatherClient fetches external weather data when configured.
type WeatherClient struct {
	cfg    *config.Config
	client *http.Client
	cache  map[string]string
}

// NewWeatherClient constructs a weather client.
func NewWeatherClient(cfg *config.Config) *WeatherClient {
	return &WeatherClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 10 * time.Second},
		cache:  map[string]string{},
	}
}

// LoadCache populates the weather cache from disk.
func (w *WeatherClient) LoadCache(path string) {
	if w == nil || path == "" {
		return
	}
	var payload map[string]string
	if err := loadJSONFile(path, &payload); err != nil {
		return
	}
	w.cache = payload
}

// SaveCache writes the weather cache to disk.
func (w *WeatherClient) SaveCache(path string) error {
	if w == nil || path == "" {
		return nil
	}
	payload := make(map[string]string, len(w.cache))
	for key, value := range w.cache {
		payload[key] = value
	}
	return saveJSONFile(path, payload)
}

// Summary returns a short weather summary for lat/lon or empty if unavailable.
func (w *WeatherClient) Summary(lat, lon float64) string {
	if w == nil || w.cfg == nil {
		return ""
	}
	keys, _ := w.cfg.GetStringSlice("weather.apiKeyAccuWeather")
	if len(keys) == 0 || strings.TrimSpace(keys[0]) == "" {
		return ""
	}
	cacheKey := fmt.Sprintf("%.3f,%.3f", lat, lon)
	if summary, ok := w.cache[cacheKey]; ok {
		return summary
	}
	locationKey := w.locationKey(keys[0], lat, lon)
	if locationKey == "" {
		return ""
	}
	summary := w.forecastSummary(keys[0], locationKey)
	if summary != "" {
		w.cache[cacheKey] = summary
	}
	return summary
}

func (w *WeatherClient) locationKey(apiKey string, lat, lon float64) string {
	if logger := logging.Get().General; logger != nil {
		logger.Debugf("Requesting weather location for %f,%f", lat, lon)
	}
	endpoint := fmt.Sprintf("https://dataservice.accuweather.com/locations/v1/cities/geoposition/search?apikey=%s&q=%f,%f", apiKey, lat, lon)
	resp, err := w.client.Get(endpoint)
	if err != nil {
		if logger := logging.Get().General; logger != nil {
			logger.Warnf("weather location fetch failed for %f,%f: %v", lat, lon, err)
		}
		return ""
	}
	defer resp.Body.Close()
	var payload struct {
		Key string `json:"Key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	return payload.Key
}

func (w *WeatherClient) forecastSummary(apiKey, locationKey string) string {
	if logger := logging.Get().General; logger != nil {
		logger.Debugf("Requesting weather forecast for %s", locationKey)
	}
	endpoint := fmt.Sprintf("https://dataservice.accuweather.com/forecasts/v1/hourly/12hour/%s?apikey=%s&details=true&metric=true", locationKey, apiKey)
	resp, err := w.client.Get(endpoint)
	if err != nil {
		if logger := logging.Get().General; logger != nil {
			logger.Warnf("weather forecast fetch failed for %s: %v", locationKey, err)
		}
		return ""
	}
	defer resp.Body.Close()
	var payload []struct {
		Temperature struct {
			Value float64 `json:"Value"`
		} `json:"Temperature"`
		IconPhrase string `json:"IconPhrase"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	if len(payload) == 0 {
		return ""
	}
	return fmt.Sprintf("%.1fC %s", payload[0].Temperature.Value, payload[0].IconPhrase)
}
