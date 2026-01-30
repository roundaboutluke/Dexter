package tracking

import (
	"fmt"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

// MaxbattleRowText mirrors PoracleJS maxbattle tracking formatting.
func MaxbattleRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any) string {
	monsterID := intFromAny(row["pokemon_id"])
	formID := intFromAny(row["form"])
	monsterName := "levelMon"
	formName := "levelMonForm"
	if monsterID != 9000 {
		if mon := findMonster(game, monsterID, formID); mon != nil {
			monsterName = tr.Translate(getString(mon["name"]), false)
			form := getMap(mon["form"])
			formName = tr.Translate(getString(form["name"]), false)
			if formName == "" || (intFromAny(form["id"]) == 0 && formName == "Normal") {
				formName = ""
			}
		}
	}

	level := intFromAny(row["level"])
	distance := intFromAny(row["distance"])
	gmax := intFromAny(row["gmax"]) != 0
	move := intFromAny(row["move"])
	evolution := intFromAny(row["evolution"])
	stationID := getString(row["station_id"])

	parts := []string{}
	if monsterID == 9000 {
		levelText := tr.Translate("level", false)
		if level == 90 {
			levelText = tr.Translate("All level", false)
		} else if label := utilLookup(game.UtilData, "maxbattleLevels", level); label != "" {
			levelText = tr.Translate(label, false)
		} else {
			levelText = titleCase(levelText) + fmt.Sprintf(" %d", level)
		}
		parts = append(parts, fmt.Sprintf("**%s**", levelText))
	} else {
		parts = append(parts, fmt.Sprintf("**%s**", monsterName))
		if formName != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", tr.Translate("form", false), formName))
		}
		if label := utilLookup(game.UtilData, "maxbattleLevels", level); label != "" {
			parts = append(parts, fmt.Sprintf("| %s", tr.Translate(label, false)))
		} else if level > 0 {
			parts = append(parts, fmt.Sprintf("| %s %d", tr.Translate("level", false), level))
		}
	}

	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	if gmax {
		parts = append(parts, fmt.Sprintf("| %s", tr.Translate("gmax", false)))
	}
	if move != 9000 {
		moveText := moveDisplay(tr, game, move)
		if moveText == "" {
			moveText = fmt.Sprintf("%d", move)
		}
		parts = append(parts, fmt.Sprintf("| %s %s", tr.Translate("with move", false), moveText))
	}
	if evolution != 9000 {
		parts = append(parts, fmt.Sprintf("| %s %d", tr.Translate("evolution", false), evolution))
	}
	parts = append(parts, standardText(cfg, tr, row))
	if stationID != "" {
		parts = append(parts, fmt.Sprintf("| %s %s", tr.Translate("at station", false), stationID))
	}

	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}
