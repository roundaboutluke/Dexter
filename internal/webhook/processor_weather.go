package webhook

import (
	"strconv"
	"time"

	"github.com/golang/geo/s2"

	"dexter/internal/dispatch"
	"dexter/internal/geo"
	"dexter/internal/logging"
)

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
		p.logControllerf(logging.LevelDebug, hook, "weather change ignored because no weather cell could be resolved")
		return false
	}
	weatherID := weatherCondition(hook.Message)
	if weatherID == 0 {
		p.logControllerf(logging.LevelDebug, hook, "weather change ignored because no weather condition was present")
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
		p.logControllerf(logging.LevelVerbose, weatherHook, "weather has not changed, nobody cares")
		return false
	}
	showAltered := false
	if p.cfg != nil {
		showAltered = getBoolFromConfig(p.cfg, "weather.showAlteredPokemon", false)
	}
	targetIDs := p.weatherData.EligibleTargets(cellID, weatherID, showAltered)
	if len(targetIDs) == 0 {
		p.logControllerf(logging.LevelVerbose, weatherHook, "weather change has no eligible targets")
		return false
	}
	queuedJobs := 0
	for _, id := range targetIDs {
		entry := p.weatherData.CareEntry(cellID, id)
		if entry == nil {
			continue
		}
		if !p.weatherData.ShouldSendWeather(cellID, id, currentHour) {
			p.logControllerf(logging.LevelDebug, weatherHook, "user already alerted for this weather change")
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
		p.logControllerf(logging.LevelDebug, weatherHook, "creating weather alert for %s %s %s %s", target.Type, target.ID, target.Name, target.Template)
		if p.rateChecker == nil || job.AlwaysSend {
			p.enqueue(job)
			queuedJobs++
			continue
		}
		additional, sendOriginal := p.applyRateLimit(job)
		for _, extra := range additional {
			p.enqueue(extra)
			queuedJobs++
		}
		if sendOriginal {
			p.enqueue(job)
			queuedJobs++
		}
	}
	if queuedJobs > 0 {
		p.logControllerf(logging.LevelInfo, weatherHook, "weather alert generated and %d humans cared", len(targetIDs))
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
