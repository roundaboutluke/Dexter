package webhook

import (
	"fmt"
	"os"
	"strings"
	"time"

	"dexter/internal/dispatch"
	"dexter/internal/geo"
	"dexter/internal/logging"
	"dexter/internal/metrics"
)

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
	p.recordRecentActivity(hook)
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
		p.logControllerf(logging.LevelVerbose, hook, "no humans cared for %s", hook.Type)
		return
	}
	if m := metrics.Get(); m != nil {
		m.WebhookMatchedTotal.WithLabelValues(hook.Type).Inc()
	}
	queuedJobs := 0
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
		p.logControllerf(logging.LevelDebug, hook, "creating %s alert for %s %s %s %s", hook.Type, target.Type, target.ID, target.Name, target.Template)
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
			queuedJobs++
			continue
		}
		additional, sendOriginal := p.applyRateLimit(job)
		for _, extra := range additional {
			p.enqueue(extra)
			queuedJobs++
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
			queuedJobs++
		}
	}
	p.logControllerf(logging.LevelInfo, hook, "%s matched %d humans and queued %d messages", hook.Type, len(targets), queuedJobs)
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
	return buildTTHFromUnix(expire)
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
		start := hookEggStart(hook.Message)
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
	p.logControllerf(logging.LevelInfo, hook, "monster change detected for %d cared targets", len(cares))

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
			Lat:            getFloat(changeHook.Message["latitude"]),
			Lon:            getFloat(changeHook.Message["longitude"]),
			Message:        message,
			Payload:        payload,
			Target:         target.ID,
			Type:           target.Type,
			Name:           target.Name,
			TTH:            tth,
			Clean:          care.Clean,
			Emoji:          "",
			LogReference:   "MonsterChange",
			Language:       target.Language,
			UpdateKey:      changeUpdateKey,
			UpdateExisting: flapping && changeUpdateKey != "",
		}
		p.logControllerf(logging.LevelDebug, changeHook, "creating monster change alert for %s %s %s %s", target.Type, target.ID, target.Name, target.Template)
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
