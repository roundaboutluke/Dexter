package webhook

import (
	"fmt"
	"os"
	"time"

	"dexter/internal/geo"
	"dexter/internal/logging"
	"dexter/internal/metrics"
)

func (p *Processor) handle(item any) {
	start := time.Now()
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
	p.logInboundPayload(hook)
	normalizeRaidOrEggHook(hook)
	normalizeMaxBattleHook(hook)
	normalizeHookCoordinates(hook)
	p.normalizeHookExpiry(hook)
	if shouldSkipExpiredHook(hook) {
		p.logControllerf(logging.LevelDebug, hook, "%s already expired, ignoring", hook.Type)
		return
	}
	if p.shouldSkipMinimumTime(hook) {
		p.logControllerf(logging.LevelDebug, hook, "%s did not meet alertMinimumTime, ignoring", hook.Type)
		return
	}
	if p.shouldSkipLongRaid(hook) {
		p.logControllerf(logging.LevelDebug, hook, "%s is longer than 47 minutes and ignoreLongRaids is enabled", hook.Type)
		return
	}
	p.routeHook(hook)
	if m := metrics.Get(); m != nil {
		m.WebhookReceivedTotal.WithLabelValues(hook.Type).Inc()
		m.WebhookProcessDuration.WithLabelValues(hook.Type).Observe(time.Since(start).Seconds())
	}
}

func normalizeRaidOrEggHook(hook *Hook) {
	if hook == nil || hook.Message == nil {
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
	if hook.Type != "raid" && hook.Type != "egg" {
		return
	}
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

func normalizeMaxBattleHook(hook *Hook) {
	if hook == nil || hook.Type != "max_battle" || hook.Message == nil {
		return
	}
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

func (p *Processor) routeHook(hook *Hook) {
	switch hook.Type {
	case "pokemon":
		p.handlePokemonHook(hook)
	case "raid", "egg":
		p.handleRaidHook(hook)
	case "max_battle":
		p.handleMaxBattleHook(hook)
	case "invasion", "pokestop":
		p.handlePokestopHook(hook)
	case "fort_update":
		p.handleFortUpdateHook(hook)
	case "quest":
		p.handleQuestHook(hook)
	case "gym", "gym_details":
		p.handleGymHook(hook)
	case "nest":
		p.handleNestHook(hook)
	case "weather":
		p.handleWeatherHook(hook)
	default:
		logger := logging.Get().Webhooks
		if logger != nil {
			logger.Warnf("webhook processor unknown hook type %s", hook.Type)
		} else {
			fmt.Fprintf(os.Stderr, "webhook processor unknown hook type %s\n", hook.Type)
		}
	}
}

func (p *Processor) handlePokemonHook(hook *Hook) {
	if p.disabled("general.disablePokemon") {
		p.logControllerf(logging.LevelDebug, hook, "wild encounter was received but set to be ignored in config")
		return
	}
	if !getBoolFromConfig(p.cfg, "tuning.disablePokemonCache", false) {
		if !p.dedupePokemon(hook) {
			p.logControllerf(logging.LevelDebug, hook, "wild encounter was sent again too soon, ignoring")
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
		encounterID := getString(hook.Message["encounter_id"])
		if encounterID != "" {
			expires := hookExpiryUnix(hook)
			if p.monsterChange.ShouldSuppressStandardAlert(encounterID, hook, expires) {
				p.logControllerf(logging.LevelDebug, hook, "standard monster alert suppressed after monster change alert")
				return
			}
		}
	}
	p.dispatch(hook)
}

func (p *Processor) handleRaidHook(hook *Hook) {
	if p.disabled("general.disableRaid") {
		p.logControllerf(logging.LevelDebug, hook, "%s was received but set to be ignored in config", hook.Type)
		return
	}
	if !p.dedupeRaid(hook) {
		p.logControllerf(logging.LevelDebug, hook, "%s was sent again too soon, ignoring", hook.Type)
		return
	}
	p.dispatch(hook)
}

func (p *Processor) handleMaxBattleHook(hook *Hook) {
	if p.disabled("general.disableMaxBattle") {
		p.logControllerf(logging.LevelDebug, hook, "max battle was received but set to be ignored in config")
		return
	}
	if !p.dedupeMaxBattle(hook) {
		p.logControllerf(logging.LevelDebug, hook, "max battle was sent again too soon, ignoring")
		return
	}
	p.dispatch(hook)
}

func (p *Processor) handlePokestopHook(hook *Hook) {
	if p.disabled("general.disablePokestop") {
		p.logControllerf(logging.LevelDebug, hook, "pokestop was received but set to be ignored in config")
		return
	}
	p.handlePokestop(hook)
}

func (p *Processor) handleFortUpdateHook(hook *Hook) {
	if p.disabled("general.disableFortUpdate") {
		p.logControllerf(logging.LevelDebug, hook, "fort update was received but set to be ignored in config")
		return
	}
	p.dispatch(hook)
}

func (p *Processor) handleQuestHook(hook *Hook) {
	if p.disabled("general.disableQuest") {
		p.logControllerf(logging.LevelDebug, hook, "quest was received but set to be ignored in config")
		return
	}
	if !p.dedupeQuest(hook) {
		p.logControllerf(logging.LevelDebug, hook, "quest was sent again too soon, ignoring")
		return
	}
	p.dispatch(hook)
}

func (p *Processor) handleGymHook(hook *Hook) {
	if p.disabled("general.disableGym") {
		p.logControllerf(logging.LevelDebug, hook, "gym was received but set to be ignored in config")
		return
	}
	if !p.dedupeGym(hook) {
		p.logControllerf(logging.LevelDebug, hook, "gym battle cooldown time hasn't ended, ignoring")
		return
	}
	p.dispatch(hook)
}

func (p *Processor) handleNestHook(hook *Hook) {
	if p.disabled("general.disableNest") {
		p.logControllerf(logging.LevelDebug, hook, "nest was received but set to be ignored in config")
		return
	}
	if !p.dedupeNest(hook) {
		p.logControllerf(logging.LevelDebug, hook, "nest was sent again too soon, ignoring")
		return
	}
	p.dispatch(hook)
}

func (p *Processor) handleWeatherHook(hook *Hook) {
	if p.disabled("general.disableWeather") {
		p.logControllerf(logging.LevelDebug, hook, "weather was received but set to be ignored in config")
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
		p.logControllerf(logging.LevelDebug, hook, "weather for this cell was sent again too soon, ignoring")
		return
	}
	if p.weatherData != nil {
		p.weatherData.UpdateFromHook(hook)
	}
	if p.cfg != nil {
		enabled, _ := p.cfg.GetBool("weather.weatherChangeAlert")
		if !enabled {
			p.logControllerf(logging.LevelDebug, hook, "weather change alerts are disabled, ignoring")
			return
		}
	}
	if !p.dispatchWeatherChange(hook) {
		return
	}
	p.dispatch(hook)
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
	start := hookEggStart(hook.Message)
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
