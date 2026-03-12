package webhook

import (
	"encoding/json"
	"fmt"
	"time"

	"poraclego/internal/logging"
)

func (w *WeatherTracker) fetchForecast(cellID string, lat, lon float64) {
	if w == nil {
		return
	}
	if logger := logging.Get().General; logger != nil {
		logger.Debugf("%s: Requesting weather forecast", cellID)
	}
	key := w.nextWeatherKey()
	if key == "" {
		if logger := logging.Get().General; logger != nil {
			logger.Infof("%s: Couldn't fetch weather forecast - no API key available", cellID)
		}
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
		if logger := logging.Get().General; logger != nil {
			logger.Warnf("%s: Fetching weather location errored: %v", cellID, err)
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
		if logger := logging.Get().General; logger != nil {
			logger.Warnf("%s: Fetching weather forecast errored: %v", locationKey, err)
		}
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
