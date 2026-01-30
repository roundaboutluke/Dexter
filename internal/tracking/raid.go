package tracking

import (
	"fmt"
	"strconv"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
	"poraclego/internal/scanner"
)

// RaidRowText mirrors PoracleJS raid tracking formatting, using scanner lookup when available.
func RaidRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any, lookup scanner.GymNameResolver) string {
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

	team := intFromAny(row["team"])
	teamName := tr.Translate(teamNameFromUtil(game.UtilData, team), false)
	distance := intFromAny(row["distance"])
	level := intFromAny(row["level"])
	clean := intFromAny(row["clean"]) != 0
	exclusive := intFromAny(row["exclusive"]) != 0
	gymID := getString(row["gym_id"])
	gymName := gymID
	if lookup != nil && gymID != "" {
		if name, err := lookup.GetGymName(gymID); err == nil && name != "" {
			gymName = name
		}
	}
	move := intFromAny(row["move"])
	evolution := intFromAny(row["evolution"])
	rsvp := intFromAny(row["rsvp_changes"])

	parts := []string{}
	if monsterID == 9000 {
		levelText := tr.Translate("level", false)
		if level == 90 {
			levelText = tr.Translate("All level", false)
		} else {
			levelText = titleCase(levelText) + fmt.Sprintf(" %d", level)
		}
		parts = append(parts, fmt.Sprintf("**%s %s**", levelText, tr.Translate("raids", false)))
	} else {
		parts = append(parts, fmt.Sprintf("**%s**", monsterName))
		if formName != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", tr.Translate("form", false), formName))
		}
	}
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
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
	if team != 4 {
		parts = append(parts, fmt.Sprintf("| %s %s", tr.Translate("controlled by", false), teamName))
	}
	if exclusive {
		parts = append(parts, fmt.Sprintf("| %s", tr.Translate("must be an EX Gym", false)))
	}
	if clean {
		parts = append(parts, fmt.Sprintf("| %s", tr.Translate("clean", false)))
	}
	rsvpText := rsvpText(tr, rsvp)
	if rsvpText != "" {
		parts = append(parts, rsvpText)
	}
	parts = append(parts, standardText(cfg, tr, row))
	if gymName != "" {
		parts = append(parts, fmt.Sprintf("%s %s", tr.Translate("at gym ", false), gymName))
	}

	return strings.TrimSpace(strings.Join(parts, " "))
}

func moveDisplay(tr *i18n.Translator, game *data.GameData, moveID int) string {
	if game == nil || tr == nil || moveID == 0 {
		return ""
	}
	raw, ok := game.Moves[fmt.Sprintf("%d", moveID)]
	if !ok {
		return ""
	}
	entry, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	name := getString(entry["name"])
	moveType := getString(entry["type"])
	if name == "" {
		return ""
	}
	if moveType == "" {
		return tr.Translate(name, false)
	}
	return fmt.Sprintf("%s/%s", tr.Translate(name, false), tr.Translate(moveType, false))
}

func teamNameFromUtil(util map[string]any, team int) string {
	entry, ok := util["teams"]
	if !ok {
		return ""
	}
	switch v := entry.(type) {
	case []any:
		if team >= 0 && team < len(v) {
			if m, ok := v[team].(map[string]any); ok {
				return getString(m["name"])
			}
		}
	case map[string]any:
		if m, ok := v[strconv.Itoa(team)].(map[string]any); ok {
			return getString(m["name"])
		}
	}
	return ""
}

func titleCase(input string) string {
	if input == "" {
		return input
	}
	return strings.ToUpper(input[:1]) + input[1:]
}

func rsvpText(tr *i18n.Translator, rsvp int) string {
	switch rsvp {
	case 0:
		return tr.Translate("without rsvp updates", false)
	case 1:
		return tr.Translate("including rsvp updates", false)
	case 2:
		return tr.Translate("rsvp only", false)
	default:
		return ""
	}
}
