package webhook

import (
	"os"
	"path/filepath"
	"time"
)

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
