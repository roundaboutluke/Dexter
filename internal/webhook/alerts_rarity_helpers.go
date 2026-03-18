package webhook

import (
	"fmt"
	"strconv"

	"dexter/internal/stats"
)

func rarityGroupForPokemon(tracker *stats.Tracker, pokemonID int) int {
	if tracker == nil || pokemonID <= 0 {
		return -1
	}
	report, ok := tracker.LatestReport()
	if !ok {
		return -1
	}
	for group, ids := range report.Rarity {
		for _, id := range ids {
			if id == pokemonID {
				return group
			}
		}
	}
	return -1
}

func rarityNameEng(p *Processor, group int) string {
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return ""
	}
	raw, ok := d.UtilData["rarity"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := raw[strconv.Itoa(group)].(string)
	return name
}

func sizeNameEng(p *Processor, size int) string {
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return ""
	}
	raw, ok := d.UtilData["size"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := raw[strconv.Itoa(size)].(string)
	return name
}

func shinyStatsForPokemon(tracker *stats.Tracker, pokemonID int) any {
	if tracker == nil || pokemonID <= 0 {
		return nil
	}
	report, ok := tracker.LatestReport()
	if !ok {
		return nil
	}
	stat, ok := report.Shiny[pokemonID]
	if !ok || stat.Ratio <= 0 {
		return nil
	}
	return fmt.Sprintf("%.0f", stat.Ratio)
}
