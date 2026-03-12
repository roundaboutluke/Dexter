package webhook

import (
	"fmt"
	"strconv"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
)

type pvpBestSummary struct {
	rank int
	cp   int
}

func pvpCapsFromConfig(cfg *config.Config) []int {
	if cfg == nil {
		return []int{50}
	}
	if raw, ok := cfg.Get("pvp.levelCaps"); ok {
		if list := parseIntSlice(raw); len(list) > 0 {
			return list
		}
	}
	return []int{50}
}

func parseIntSlice(raw any) []int {
	switch v := raw.(type) {
	case []int:
		return v
	case []any:
		out := make([]int, 0, len(v))
		for _, item := range v {
			if num := getInt(item); num > 0 {
				out = append(out, num)
			}
		}
		return out
	case []string:
		out := make([]int, 0, len(v))
		for _, item := range v {
			if parsed, err := strconv.Atoi(strings.TrimSpace(item)); err == nil {
				out = append(out, parsed)
			}
		}
		return out
	default:
		return nil
	}
}

func pvpRankSummary(capsConsidered []int, league int, raw any, pokemonID int, evoEnabled bool, minCp int, maxRank int, pvpEvolution map[string]any) ([]map[string]any, pvpBestSummary) {
	best := map[int]pvpBestSummary{}
	for _, cap := range capsConsidered {
		best[cap] = pvpBestSummary{rank: 4096, cp: 0}
	}

	entries := pvpAnySlice(raw)
	if len(entries) == 0 {
		return summarizeBestRanks(best)
	}

	for _, item := range entries {
		stats, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rank := getInt(stats["rank"])
		if rank == 0 {
			continue
		}
		cp := getInt(stats["cp"])
		caps := pvpCapsForEntry(stats, capsConsidered)
		for _, cap := range caps {
			details := best[cap]
			if rank < details.rank {
				details.rank = rank
				details.cp = cp
				best[cap] = details
			} else if rank == details.rank && cp > details.cp {
				details.cp = cp
				best[cap] = details
			}
		}

		if !evoEnabled {
			continue
		}
		if getBool(stats["evolution"]) {
			continue
		}
		statsPokemon := getInt(stats["pokemon"])
		if statsPokemon == 0 || statsPokemon == pokemonID {
			continue
		}
		if rank > maxRank || cp < minCp {
			continue
		}
		entry := map[string]any{
			"rank":       rank,
			"percentage": getFloat(stats["percentage"]),
			"pokemon":    statsPokemon,
			"form":       getInt(stats["form"]),
			"level":      getFloat(stats["level"]),
			"cp":         cp,
			"caps":       caps,
		}
		addPvpEvolutionEntry(pvpEvolution, statsPokemon, league, entry)
	}

	return summarizeBestRanks(best)
}

func pvpCapsForEntry(stats map[string]any, capsConsidered []int) []int {
	capVal := getInt(stats["cap"])
	capped := getBool(stats["capped"])
	if capVal == 0 && !capped {
		return []int{50}
	}
	if capped {
		out := []int{}
		for _, cap := range capsConsidered {
			if cap >= capVal {
				out = append(out, cap)
			}
		}
		return out
	}
	if capVal > 0 && containsInt(capsConsidered, capVal) {
		return []int{capVal}
	}
	return nil
}

func summarizeBestRanks(best map[int]pvpBestSummary) ([]map[string]any, pvpBestSummary) {
	bestRanks := []map[string]any{}
	summary := pvpBestSummary{rank: 4096, cp: 0}
	for cap, details := range best {
		if details.rank < summary.rank {
			summary.rank = details.rank
		}
		if summary.cp == 0 || details.cp < summary.cp {
			summary.cp = details.cp
		}
		found := false
		for _, existing := range bestRanks {
			if getInt(existing["rank"]) == details.rank && getInt(existing["cp"]) == details.cp {
				if caps, ok := existing["caps"].([]int); ok {
					existing["caps"] = append(caps, cap)
				} else if caps, ok := existing["caps"].([]any); ok {
					existing["caps"] = append(caps, cap)
				} else {
					existing["caps"] = []int{cap}
				}
				found = true
				break
			}
		}
		if !found {
			bestRanks = append(bestRanks, map[string]any{
				"rank": details.rank,
				"cp":   details.cp,
				"caps": []int{cap},
			})
		}
	}
	return bestRanks, summary
}

func addPvpEvolutionEntry(pvpEvolution map[string]any, pokemonID int, league int, entry map[string]any) {
	key := strconv.Itoa(pokemonID)
	leagueKey := strconv.Itoa(league)
	raw, ok := pvpEvolution[key]
	if !ok {
		pvpEvolution[key] = map[string]any{leagueKey: []map[string]any{entry}}
		return
	}
	leagueMap, ok := raw.(map[string]any)
	if !ok {
		pvpEvolution[key] = map[string]any{leagueKey: []map[string]any{entry}}
		return
	}
	if list, ok := leagueMap[leagueKey].([]map[string]any); ok {
		leagueMap[leagueKey] = append(list, entry)
		return
	}
	if list, ok := leagueMap[leagueKey].([]any); ok {
		leagueMap[leagueKey] = append(list, entry)
		return
	}
	leagueMap[leagueKey] = []map[string]any{entry}
}

func pvpRankingList(p *Processor, hook *Hook, key string, tr *i18n.Translator) []map[string]any {
	raw := hook.Message[key]
	items := pvpAnySlice(raw)
	if len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pokemonID := getInt(m["pokemon"])
		if pokemonID == 0 {
			pokemonID = getInt(m["pokemon_id"])
		}
		out = append(out, map[string]any{
			"fullName":     translateMaybe(tr, monsterName(p, pokemonID)),
			"rank":         getInt(m["rank"]),
			"cp":           getInt(m["cp"]),
			"level":        getFloat(m["level"]),
			"levelWithCap": getFloat(m["level"]),
		})
	}
	return out
}

type pvpFilter struct {
	League int
	Worst  int
	Cap    int
}

func pvpFiltersFromRow(row map[string]any) []pvpFilter {
	if row == nil {
		return nil
	}
	worst := getInt(row["pvp_ranking_worst"])
	if worst <= 0 || worst >= 4096 {
		return nil
	}
	return []pvpFilter{{
		League: getInt(row["pvp_ranking_league"]),
		Worst:  worst,
		Cap:    getInt(row["pvp_ranking_cap"]),
	}}
}

func pvpDisplayList(p *Processor, raw any, leagueCap int, maxRank int, minCp int, filters []pvpFilter, filterByTrack bool, tr *i18n.Translator) []map[string]any {
	items := pvpAnySlice(raw)
	if len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rank := getInt(m["rank"])
		cp := getInt(m["cp"])
		if rank == 0 || rank > maxRank || cp < minCp {
			continue
		}
		pokemonID := getInt(m["pokemon"])
		if pokemonID == 0 {
			pokemonID = getInt(m["pokemon_id"])
		}
		if pokemonID == 0 {
			continue
		}
		formID := getInt(m["form"])
		if formID == 0 {
			formID = getInt(m["form_id"])
		}
		evolutionID := getInt(m["evolution"])
		level := getFloat(m["level"])
		cap := getInt(m["cap"])
		capped := getBool(m["capped"])
		levelWithCap := any(level)
		if cap > 0 && !capped {
			levelWithCap = fmt.Sprintf("%.0f/%d", level, cap)
		}
		percentage := getFloat(m["percentage"])
		if percentage <= 1 {
			percentage = percentage * 100
		}
		stats := map[string]any{
			"baseAttack":  0,
			"baseDefense": 0,
			"baseStamina": 0,
		}
		if found, ok := lookupMonsterStats(p, pokemonID, formID); ok {
			stats = found
		}
		nameEng := monsterName(p, pokemonID)
		formEng := monsterFormName(p, pokemonID, formID)
		if nameEng == "" || strings.HasPrefix(nameEng, "Pokemon ") {
			nameEng = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		if strings.EqualFold(formEng, "Normal") {
			formEng = ""
		}
		name := translateMaybe(tr, nameEng)
		form := translateMaybe(tr, formEng)
		fullNameEng := joinNonEmptyWithSep([]string{nameEng, formEng}, " ")
		fullName := joinNonEmptyWithSep([]string{name, form}, " ")
		if evolutionID != 0 {
			if format := megaNameFormat(p, evolutionID); format != "" {
				fullNameEng = formatTemplate(format, fullNameEng)
				fullName = formatTemplate(format, fullName)
			}
		}
		passesFilter := true
		matchesUserTrack := false
		if len(filters) > 0 {
			passesFilter = false
			for _, filter := range filters {
				leagueMatch := filter.League == 0 || filter.League == leagueCap
				capMatch := filter.Cap == 0 || filter.Cap == cap || capped
				if leagueMatch && capMatch && filter.Worst >= rank {
					passesFilter = true
					matchesUserTrack = true
					break
				}
			}
		}
		displayRank := map[string]any{
			"rank":             rank,
			"formId":           formID,
			"evolution":        evolutionID,
			"level":            level,
			"cap":              cap,
			"capped":           capped,
			"levelWithCap":     levelWithCap,
			"cp":               cp,
			"pokemonId":        pokemonID,
			"percentage":       fmt.Sprintf("%.2f", percentage),
			"baseStats":        stats,
			"nameEng":          nameEng,
			"formEng":          formEng,
			"name":             name,
			"form":             form,
			"fullNameEng":      fullNameEng,
			"fullName":         fullName,
			"passesFilter":     passesFilter,
			"matchesUserTrack": matchesUserTrack,
		}
		if !filterByTrack || passesFilter {
			out = append(out, displayRank)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func pvpBestInfo(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	list, ok := raw.([]map[string]any)
	if !ok {
		if items, ok := raw.([]any); ok {
			parsed := make([]map[string]any, 0, len(items))
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					parsed = append(parsed, m)
				}
			}
			list = parsed
		}
	}
	if len(list) == 0 {
		return nil
	}
	bestRank := 4096
	bestList := []map[string]any{}
	for _, entry := range list {
		rank := getInt(entry["rank"])
		if rank == bestRank {
			bestList = append(bestList, entry)
		} else if rank < bestRank {
			bestRank = rank
			bestList = []map[string]any{entry}
		}
	}
	if len(bestList) == 0 {
		return nil
	}
	nameSet := map[string]bool{}
	nameEngSet := map[string]bool{}
	names := []string{}
	namesEng := []string{}
	for _, entry := range bestList {
		if name := getString(entry["fullName"]); name != "" && !nameSet[name] {
			nameSet[name] = true
			names = append(names, name)
		}
		if name := getString(entry["fullNameEng"]); name != "" && !nameEngSet[name] {
			nameEngSet[name] = true
			namesEng = append(namesEng, name)
		}
	}
	return map[string]any{
		"rank":    bestRank,
		"list":    bestList,
		"name":    strings.Join(names, ", "),
		"nameEng": strings.Join(namesEng, ", "),
	}
}
