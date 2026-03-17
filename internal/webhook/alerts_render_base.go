package webhook

import (
	"fmt"
	"strings"
	"time"

	"poraclego/internal/geo"
)

type renderBaseContext struct {
	lat       float64
	lon       float64
	weatherID int
}

func applyBaseRenderData(p *Processor, hook *Hook, match alertMatch, data map[string]any) renderBaseContext {
	if hook != nil {
		for key, value := range hook.Message {
			data[key] = value
		}
		if hook.Type == "nest" {
			if name := getString(hook.Message["name"]); name != "" {
				data["nestName"] = name
			}
		}
	}
	if p != nil && p.cfg != nil {
		if raw, ok := p.cfg.Get("general.dtsDictionary"); ok {
			if dict, ok := raw.(map[string]any); ok {
				for key, value := range dict {
					data[key] = value
				}
			}
		}
	}
	prepareMapPosition(p, hook, data)
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	data["lat"] = fmt.Sprintf("%.6f", lat)
	data["lon"] = fmt.Sprintf("%.6f", lon)
	data["latitude"] = data["lat"]
	data["longitude"] = data["lon"]
	matchedAreas := []map[string]any{}
	matched := []string{}
	areasDisplay := []string{}
	if fences := p.getFences(); fences != nil {
		for _, fence := range fences.MatchedAreas([]float64{lat, lon}) {
			display := true
			if fence.DisplayInMatch != nil {
				display = *fence.DisplayInMatch
			}
			matchedAreas = append(matchedAreas, map[string]any{
				"name":             fence.Name,
				"description":      fence.Description,
				"displayInMatches": display,
				"group":            fence.Group,
			})
			matched = append(matched, strings.ToLower(fence.Name))
			if display {
				areasDisplay = append(areasDisplay, fence.Name)
			}
		}
	}
	data["matchedAreas"] = matchedAreas
	data["matched"] = matched
	data["areas"] = strings.Join(areasDisplay, ", ")
	if mapLat := getFloat(hook.Message["map_latitude"]); mapLat != 0 {
		data["map_latitude"] = fmt.Sprintf("%.6f", mapLat)
	}
	if mapLon := getFloat(hook.Message["map_longitude"]); mapLon != 0 {
		data["map_longitude"] = fmt.Sprintf("%.6f", mapLon)
	}
	if mapZoom := getFloat(hook.Message["zoom"]); mapZoom != 0 {
		data["zoom"] = mapZoom
	}
	gymID := getString(hook.Message["gym_id"])
	if gymID == "" && hook.Type != "max_battle" {
		gymID = getString(hook.Message["id"])
	}
	data["gym_id"] = gymID
	data["pokestop_id"] = getString(hook.Message["pokestop_id"])
	data["teamId"] = teamFromHookMessage(hook.Message)
	data["encounterId"] = getString(hook.Message["encounter_id"])
	data["googleMap"] = googleMapURL(hook)
	data["shortUrl"] = shortenURL(p, fmt.Sprintf("%v", data["googleMap"]))
	data["ex"] = getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
	data["pokemonId"] = getInt(hook.Message["pokemon_id"])
	if hook.Type == "max_battle" {
		stationID := getString(hook.Message["stationId"])
		if stationID == "" {
			stationID = getString(hook.Message["id"])
		}
		if stationID != "" {
			data["stationId"] = stationID
		}
		stationName := getString(hook.Message["stationName"])
		if stationName == "" {
			stationName = getString(hook.Message["name"])
		}
		if stationName != "" {
			data["stationName"] = stationName
		}
	}
	switch hook.Type {
	case "pokemon", "raid", "egg":
		data["id"] = data["pokemonId"]
	case "weather":
		data["id"] = match.Target.ID
	}
	if p != nil && p.shinyPossible != nil && data["pokemonId"].(int) > 0 {
		formID := getInt(hook.Message["form"])
		if formID == 0 {
			formID = getInt(hook.Message["form_id"])
		}
		if formID == 0 {
			formID = getInt(hook.Message["pokemon_form"])
		}
		data["shinyPossible"] = p.shinyPossible.IsPossible(data["pokemonId"].(int), formID)
	}
	weatherID := getInt(hook.Message["weather"])
	if weatherID == 0 {
		weatherID = weatherCondition(hook.Message)
	}
	if boosted := getInt(hook.Message["boosted_weather"]); boosted > 0 {
		weatherID = boosted
	}
	if (hook.Type == "raid" || hook.Type == "egg") && weatherID == 0 && p != nil && p.weatherData != nil {
		weatherCellID := geo.WeatherCellID(lat, lon)
		if weatherCellID != "" {
			if cell := p.weatherData.WeatherInfo(weatherCellID); cell != nil {
				now := time.Now().Unix()
				currentHour := now - (now % 3600)
				if current := cell.Data[currentHour]; current > 0 {
					weatherID = current
				}
			}
		}
	}
	data["weather"] = weatherID
	return renderBaseContext{lat: lat, lon: lon, weatherID: weatherID}
}
