package webhook

import (
	"strings"
	"time"

	"dexter/internal/logging"
	"dexter/internal/pvp"
)

func (p *Processor) updateStats(hook *Hook) {
	if p == nil || p.stats == nil || hook == nil {
		return
	}
	pokemonID := getInt(hook.Message["pokemon_id"])
	isShiny := getBool(hook.Message["shiny"])
	if pokemonID > 0 {
		ivScanned := hook.Message["individual_defense"] != nil ||
			hook.Message["individual_attack"] != nil ||
			hook.Message["individual_stamina"] != nil
		p.stats.Update(pokemonID, ivScanned, isShiny)
	}
}

func (p *Processor) applyPvp(hook *Hook) {
	calc := p.getPvpCalc()
	if p == nil || hook == nil || p.cfg == nil || calc == nil {
		return
	}
	start := time.Now()
	source, _ := p.cfg.GetString("pvp.dataSource")
	if source == "" {
		source = "webhook"
	}
	source = strings.ToLower(source)
	if source != "internal" && source != "compare" {
		return
	}

	atk, okAtk := getIntFromKeys(hook.Message, "individual_attack", "atk")
	def, okDef := getIntFromKeys(hook.Message, "individual_defense", "def")
	sta, okSta := getIntFromKeys(hook.Message, "individual_stamina", "sta")
	if !okAtk || !okDef || !okSta {
		return
	}

	pokemonID := getInt(hook.Message["pokemon_id"])
	formID := getInt(hook.Message["form"])
	if pokemonID == 0 {
		return
	}

	includeEvolution, _ := p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")
	includeMega, _ := p.cfg.GetBool("pvp.includeMegaEvolution")
	littleLeagueCanEvolve, _ := p.cfg.GetBool("pvp.littleLeagueCanEvolve")

	ranks := calc.Rankings(pokemonID, formID, atk, def, sta, includeEvolution)
	filtered := map[int][]pvp.Entry{}
	for league, entries := range ranks {
		allowed := []pvp.Entry{}
		for _, entry := range entries {
			if !includeMega && entry.Evolution {
				if isMegaForm(calc, entry.PokemonID, entry.FormID) {
					continue
				}
			}
			if league == 500 && !littleLeagueCanEvolve && entry.Evolution {
				continue
			}
			allowed = append(allowed, entry)
		}
		if len(allowed) > 0 {
			filtered[league] = allowed
		}
	}

	if source == "compare" {
		hook.Message["ohbem_pvp"] = pvpEntriesToPayload(filtered)
		if logging.PvpEnabled(p.cfg) {
			p.logControllerf(logging.LevelVerbose, hook, "PVP From internal compare: %v", hook.Message["ohbem_pvp"])
		}
		p.logControllerf(logging.TimingLevel(p.cfg), hook, "PVP time: %dms", time.Since(start).Milliseconds())
		return
	}
	for league, entries := range filtered {
		key := pvpLeagueKey(league)
		if key == "" {
			continue
		}
		hook.Message[key] = pvpEntriesToPayload(map[int][]pvp.Entry{league: entries})[league]
	}
	if logging.PvpEnabled(p.cfg) {
		p.logControllerf(logging.LevelVerbose, hook, "PVP From internal: great=%v ultra=%v little=%v", hook.Message["pvp_rankings_great_league"], hook.Message["pvp_rankings_ultra_league"], hook.Message["pvp_rankings_little_league"])
	}
	p.logControllerf(logging.TimingLevel(p.cfg), hook, "PVP time: %dms", time.Since(start).Milliseconds())
}

func pvpLeagueKey(league int) string {
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

func pvpEntriesToPayload(entries map[int][]pvp.Entry) map[int][]map[string]any {
	out := map[int][]map[string]any{}
	for league, list := range entries {
		payload := []map[string]any{}
		for _, entry := range list {
			item := map[string]any{
				"pokemon":    entry.PokemonID,
				"form":       entry.FormID,
				"rank":       entry.Rank,
				"cp":         entry.CP,
				"level":      entry.Level,
				"percentage": entry.Percentage,
				"cap":        entry.Cap,
				"caps":       entry.Caps,
				"evolution":  entry.Evolution,
			}
			payload = append(payload, item)
		}
		out[league] = payload
	}
	return out
}

func getIntFromKeys(values map[string]any, keys ...string) (int, bool) {
	for _, key := range keys {
		if value, ok := values[key]; ok {
			return getInt(value), true
		}
	}
	return 0, false
}

func isMegaForm(calc *pvp.Calculator, pokemonID, formID int) bool {
	if calc == nil {
		return false
	}
	entry := calc.Lookup(pokemonID, formID)
	if entry == nil {
		return false
	}
	form, ok := entry["form"].(map[string]any)
	if !ok {
		return false
	}
	name, _ := form["name"].(string)
	name = strings.ToLower(name)
	return strings.Contains(name, "mega") || strings.Contains(name, "primal")
}
