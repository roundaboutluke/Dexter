package webhook

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"dexter/internal/logging"
)

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
		p.logControllerf(logging.LevelDebug, hook, "pokestop received but no invasion or lure information, ignoring")
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
		} else {
			p.logControllerf(logging.LevelDebug, hook, "lure was sent again too soon, ignoring")
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
			} else {
				p.logControllerf(logging.LevelDebug, hook, "invasion was sent again too soon, ignoring")
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
		} else {
			p.logControllerf(logging.LevelDebug, hook, "confirmed invasion was sent again too soon, ignoring")
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
	tooSoon := p.cache.Get(cacheKey)
	if inBattle {
		p.cache.Set(cacheKey, 5*time.Minute)
	}
	cached := p.gymCache.Get(id)
	if cached != nil {
		if cached.TeamID == team && cached.SlotsAvailable == getInt(hook.Message["slots_available"]) && tooSoon {
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
