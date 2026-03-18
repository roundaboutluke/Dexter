package tracking

import (
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/i18n"
	"dexter/internal/scanner"
)

// EggRowText mirrors PoracleJS egg tracking formatting, using scanner lookup when available.
func EggRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any, lookup scanner.GymNameResolver) string {
	team := intFromAny(row["team"])
	teamName := tr.Translate(teamNameFromUtil(game.UtilData, team), false)
	distance := intFromAny(row["distance"])
	level := intFromAny(row["level"])
	exclusive := intFromAny(row["exclusive"]) != 0
	gymID := getString(row["gym_id"])
	gymName := gymID
	if lookup != nil && gymID != "" {
		if name, err := lookup.GetGymName(gymID); err == nil && name != "" {
			gymName = name
		}
	}
	rsvp := intFromAny(row["rsvp_changes"])

	levelText := tr.Translate("level", false)
	if level == 90 {
		levelText = tr.Translate("All level", false)
	} else {
		levelText = titleCase(levelText) + fmt.Sprintf(" %d", level)
	}
	parts := []string{
		fmt.Sprintf("**%s %s**", levelText, tr.Translate("eggs", false)),
	}
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	if team != 4 {
		parts = append(parts, fmt.Sprintf("| %s %s", tr.Translate("controlled by", false), teamName))
	}
	if exclusive {
		parts = append(parts, fmt.Sprintf("| %s", tr.Translate("must be an EX Gym", false)))
	}
	if gymName != "" {
		parts = append(parts, fmt.Sprintf("%s %s", tr.Translate("at gym ", false), gymName))
	}
	rsvpText := rsvpText(tr, rsvp)
	if rsvpText != "" {
		parts = append(parts, rsvpText)
	}
	parts = append(parts, standardText(cfg, tr, row))

	return strings.TrimSpace(strings.Join(parts, " "))
}
