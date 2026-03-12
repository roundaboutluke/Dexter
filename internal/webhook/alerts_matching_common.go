package webhook

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"poraclego/internal/i18n"
)

func defaultTranslator(p *Processor, hook *Hook) *i18nTranslator {
	language := getString(hook.Message["language"])
	if language == "" && p.cfg != nil {
		if v, ok := p.cfg.GetString("general.locale"); ok {
			language = v
		}
	}
	return &i18nTranslator{factory: p.i18n, language: language}
}

type i18nTranslator struct {
	factory  *i18n.Factory
	language string
}

func (t *i18nTranslator) Translate(key string, fallback bool) string {
	if t.factory == nil {
		return key
	}
	tr := t.factory.Translator(t.language)
	return tr.Translate(key, fallback)
}

func rowMatchesHook(p *Processor, hook *Hook, row map[string]any) bool {
	switch hook.Type {
	case "pokemon":
		return matchPokemon(p, hook, row)
	case "raid":
		return matchRaid(hook, row)
	case "egg":
		return matchEgg(hook, row)
	case "max_battle":
		return matchMaxBattle(hook, row)
	case "quest":
		return matchQuest(hook, row)
	case "invasion":
		return matchInvasion(hook, row)
	case "lure":
		return matchLure(hook, row)
	case "nest":
		return matchNest(hook, row)
	case "gym", "gym_details":
		return matchGym(hook, row)
	case "weather":
		return matchWeather(hook, row)
	case "fort_update":
		return matchFort(hook, row)
	default:
		return false
	}
}

func matchPokemon(p *Processor, hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["pokemon_id"])
	trackedID := getInt(row["pokemon_id"])
	league := getInt(row["pvp_ranking_league"])
	trackedForm := getInt(row["form"])
	spawnForm := getInt(hook.Message["form"])
	evoEnabled := false
	if p != nil && p.cfg != nil {
		evoEnabled, _ = p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")
	}
	if trackedID != 0 && trackedID != pokemonID {
		if league == 0 || !evoEnabled {
			return false
		}
	} else {
		if trackedForm > 0 && spawnForm != trackedForm {
			return false
		}
	}
	if league > 0 {
		if !matchPvpLeague(p, hook, row, league, pokemonID, spawnForm, trackedID, trackedForm) {
			return false
		}
	}
	iv := computeIV(hook)
	minIV := getInt(row["min_iv"])
	maxIV := getInt(row["max_iv"])
	if iv < float64(minIV) {
		return false
	}
	if iv > float64(maxIV) {
		return false
	}
	cp := getInt(hook.Message["cp"])
	if min := getInt(row["min_cp"]); cp < min {
		return false
	}
	if max := getInt(row["max_cp"]); cp > max {
		return false
	}
	level := getInt(hook.Message["pokemon_level"])
	if min := getInt(row["min_level"]); level < min {
		return false
	}
	if max := getInt(row["max_level"]); level > max {
		return false
	}
	atk := ivStatValue(hook.Message["individual_attack"])
	def := ivStatValue(hook.Message["individual_defense"])
	sta := ivStatValue(hook.Message["individual_stamina"])
	if minAtk := getInt(row["atk"]); atk < minAtk {
		return false
	}
	if minDef := getInt(row["def"]); def < minDef {
		return false
	}
	if minSta := getInt(row["sta"]); sta < minSta {
		return false
	}
	if maxAtk := getInt(row["max_atk"]); atk > maxAtk {
		return false
	}
	if maxDef := getInt(row["max_def"]); def > maxDef {
		return false
	}
	if maxSta := getInt(row["max_sta"]); sta > maxSta {
		return false
	}
	if minTime := getInt(row["min_time"]); minTime > 0 {
		expire := hookExpiryUnix(hook)
		if expire == 0 {
			return false
		}
		remaining := int(time.Until(time.Unix(expire, 0)).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		if remaining < minTime {
			return false
		}
	}
	weight := int(getFloat(hook.Message["weight"]) * 1000)
	if minWeight := getInt(row["min_weight"]); weight < minWeight {
		return false
	}
	if maxWeight := getInt(row["max_weight"]); weight > maxWeight {
		return false
	}
	size := getInt(hook.Message["size"])
	if minSize := getInt(row["size"]); minSize >= 0 && size < minSize {
		return false
	}
	if maxSize := getInt(row["max_size"]); size > maxSize {
		return false
	}
	if gender := getInt(row["gender"]); gender > 0 && getInt(hook.Message["gender"]) != gender {
		return false
	}
	if p != nil {
		rarityGroup := getInt(hook.Message["rarityGroup"])
		if rarityGroup == 0 {
			rarityGroup = getInt(hook.Message["rarity_group"])
		}
		if rarityGroup == 0 {
			rarityGroup = rarityGroupForPokemon(p.stats, pokemonID)
		}
		minRarity := getInt(row["rarity"])
		maxRarity := getInt(row["max_rarity"])
		if rarityGroup < minRarity {
			return false
		}
		if rarityGroup > maxRarity {
			return false
		}
	}
	return true
}

type pvpEntry struct {
	PokemonID int
	FormID    int
	Rank      int
	CP        int
	Caps      []int
	Evolution bool
}

func matchPvpLeague(p *Processor, hook *Hook, row map[string]any, league int, spawnPokemonID int, spawnFormID int, trackedPokemonID int, trackedFormID int) bool {
	capsConsidered := pvpCapsFromConfig(nil)
	if p != nil {
		capsConsidered = pvpCapsFromConfig(p.cfg)
	}
	entries := pvpEntriesFromHookWithCaps(hook, league, capsConsidered)
	if len(entries) == 0 {
		return false
	}
	pvpQueryLimit := 100
	evoEnabled := false
	evoMinCP := 0
	if p != nil && p.cfg != nil {
		if limit, ok := p.cfg.GetInt("pvp.pvpQueryMaxRank"); ok && limit > 0 {
			pvpQueryLimit = limit
		} else if limit, ok := p.cfg.GetInt("pvp.pvpFilterMaxRank"); ok && limit > 0 {
			pvpQueryLimit = limit
		}
		evoEnabled, _ = p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")
		switch league {
		case 1500:
			evoMinCP = getIntFromConfig(p.cfg, "pvp.pvpFilterGreatMinCP", 0)
		case 2500:
			evoMinCP = getIntFromConfig(p.cfg, "pvp.pvpFilterUltraMinCP", 0)
		case 500:
			evoMinCP = getIntFromConfig(p.cfg, "pvp.pvpFilterLittleMinCP", 0)
		}
	}
	rowCap := getInt(row["pvp_ranking_cap"])
	minCp := getInt(row["pvp_ranking_min_cp"])
	best := getInt(row["pvp_ranking_best"])
	worst := getInt(row["pvp_ranking_worst"])

	for _, entry := range entries {
		if entry.Rank > pvpQueryLimit {
			continue
		}
		switch {
		case trackedPokemonID == 0:
			if entry.PokemonID != spawnPokemonID || entry.FormID != spawnFormID {
				continue
			}
		case trackedPokemonID == spawnPokemonID:
			if entry.PokemonID != spawnPokemonID || entry.FormID != spawnFormID {
				continue
			}
		default:
			if !evoEnabled {
				continue
			}
			if entry.Evolution {
				continue
			}
			if entry.PokemonID != trackedPokemonID {
				continue
			}
			if trackedFormID > 0 && entry.FormID != trackedFormID {
				continue
			}
			if evoMinCP > 0 && entry.CP < evoMinCP {
				continue
			}
		}
		if entry.Rank > worst || entry.Rank < best {
			continue
		}
		if entry.CP < minCp {
			continue
		}
		if rowCap > 0 && len(entry.Caps) > 0 && !containsInt(entry.Caps, rowCap) {
			continue
		}
		return true
	}
	return false
}

func pvpEntriesFromHookWithCaps(hook *Hook, league int, capsConsidered []int) []pvpEntry {
	key := pvpLeagueKeyForMatch(league)
	if key == "" {
		return nil
	}
	raw := hook.Message[key]
	items := pvpAnySlice(raw)
	if len(items) == 0 {
		return nil
	}
	out := []pvpEntry{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pokemonID := getInt(m["pokemon"])
		if pokemonID == 0 {
			pokemonID = getInt(m["pokemon_id"])
		}
		entry := pvpEntry{
			PokemonID: pokemonID,
			FormID:    getInt(m["form"]),
			Rank:      getInt(m["rank"]),
			CP:        getInt(m["cp"]),
			Evolution: getBool(m["evolution"]),
		}
		if caps, ok := m["caps"].([]any); ok {
			for _, cap := range caps {
				entry.Caps = append(entry.Caps, getInt(cap))
			}
		} else if cap := getInt(m["cap"]); cap > 0 || getBool(m["capped"]) {
			entry.Caps = pvpCapsForEntry(m, capsConsidered)
		}
		out = append(out, entry)
	}
	return out
}

func pvpLeagueKeyForMatch(league int) string {
	switch league {
	case 1500:
		return "pvp_rankings_great_league"
	case 2500:
		return "pvp_rankings_ultra_league"
	case 500:
		return "pvp_rankings_little_league"
	default:
		return ""
	}
}

func normalizePvpRankings(hook *Hook) {
	if hook == nil || hook.Message == nil {
		return
	}
	pvpRaw := hook.Message["pvp"]
	ohbemRaw := hook.Message["ohbem_pvp"]
	for _, league := range []int{500, 1500, 2500} {
		key := pvpLeagueKeyForMatch(league)
		if key == "" || hook.Message[key] != nil {
			continue
		}
		name := pvpLeagueName(league)
		if name == "" {
			continue
		}
		if list := pvpListFromMap(pvpRaw, name); len(list) > 0 {
			hook.Message[key] = list
			continue
		}
		if list := pvpListFromMap(ohbemRaw, name); len(list) > 0 {
			hook.Message[key] = list
			continue
		}
	}
}

func pvpLeagueName(league int) string {
	switch league {
	case 1500:
		return "great"
	case 2500:
		return "ultra"
	case 500:
		return "little"
	default:
		return ""
	}
}

func pvpListFromMap(raw any, leagueName string) []any {
	if leagueName == "" {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	if list := pvpAnySlice(m[leagueName]); len(list) > 0 {
		return list
	}
	if list := pvpAnySlice(m[leagueName+"_league"]); len(list) > 0 {
		return list
	}
	return nil
}

func pvpAnySlice(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = v[i]
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var decoded []any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return decoded
		}
		var decodedMap []map[string]any
		if err := json.Unmarshal([]byte(v), &decodedMap); err == nil {
			out := make([]any, len(decodedMap))
			for i := range decodedMap {
				out[i] = decodedMap[i]
			}
			return out
		}
	case []byte:
		if len(v) == 0 {
			return nil
		}
		var decoded []any
		if err := json.Unmarshal(v, &decoded); err == nil {
			return decoded
		}
		var decodedMap []map[string]any
		if err := json.Unmarshal(v, &decodedMap); err == nil {
			out := make([]any, len(decodedMap))
			for i := range decodedMap {
				out[i] = decodedMap[i]
			}
			return out
		}
	}
	return nil
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func computeIV(hook *Hook) float64 {
	rawAtk, okAtk := hook.Message["individual_attack"]
	rawDef, okDef := hook.Message["individual_defense"]
	rawSta, okSta := hook.Message["individual_stamina"]
	if !okAtk || !okDef || !okSta {
		return -1
	}
	if !hasNumeric(rawAtk) || !hasNumeric(rawDef) || !hasNumeric(rawSta) {
		return -1
	}
	atk := int(toFloat(rawAtk))
	def := int(toFloat(rawDef))
	sta := int(toFloat(rawSta))
	return float64(atk+def+sta) / 45.0 * 100.0
}

func hasNumeric(value any) bool {
	switch v := value.(type) {
	case int, int64, float64, float32, json.Number:
		return toFloat(v) >= 0
	case string:
		if strings.TrimSpace(v) == "" {
			return false
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return err == nil && parsed >= 0
	default:
		return false
	}
}

func ivStatValue(value any) int {
	if !hasNumeric(value) {
		return 0
	}
	if v := int(toFloat(value)); v > 0 {
		return v
	}
	return 0
}
