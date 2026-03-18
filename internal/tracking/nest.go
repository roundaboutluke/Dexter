package tracking

import (
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/i18n"
)

// NestRowText mirrors PoracleJS nest tracking formatting.
func NestRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any) string {
	monsterID := intFromAny(row["pokemon_id"])
	formID := intFromAny(row["form"])
	monsterName := ""
	formName := ""

	if monsterID == 0 {
		monsterName = tr.Translate("Everything", false)
	} else {
		mon := findMonster(game, monsterID, formID)
		if mon != nil {
			monsterName = tr.Translate(getString(mon["name"]), false)
			formName = tr.Translate(getString(getMap(mon["form"])["name"]), false)
			if formName == "" || (intFromAny(getMap(mon["form"])["id"]) == 0 && formName == "Normal") {
				formName = ""
			}
		} else {
			monsterName = fmt.Sprintf("%s %d", tr.Translate("Unknown monster", false), monsterID)
		}
	}

	parts := []string{fmt.Sprintf("**%s**", monsterName)}
	if formName != "" {
		parts = append(parts, fmt.Sprintf("%s: %s", tr.Translate("form", false), formName))
	}
	distance := intFromAny(row["distance"])
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	minSpawn := intFromAny(row["min_spawn_avg"])
	if minSpawn > 0 {
		parts = append(parts, tr.TranslateFormat("Min avg. spawn {0}/hour", minSpawn))
	}
	parts = append(parts, standardText(cfg, tr, row))
	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}
