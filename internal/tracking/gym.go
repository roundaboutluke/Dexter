package tracking

import (
	"fmt"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
	"poraclego/internal/scanner"
)

// GymRowText mirrors PoracleJS gym tracking formatting, using scanner lookup when available.
func GymRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any, lookup scanner.GymNameResolver) string {
	team := intFromAny(row["team"])
	teamName := tr.Translate(teamNameFromUtil(game.UtilData, team), false)
	if team == 4 {
		teamName = tr.Translate("All team's", false)
	}
	distance := intFromAny(row["distance"])
	slotChanges := intFromAny(row["slot_changes"]) != 0
	battleChanges := intFromAny(row["battle_changes"]) != 0
	gymID := getString(row["gym_id"])
	gymName := gymID
	if lookup != nil && gymID != "" {
		if name, err := lookup.GetGymName(gymID); err == nil && name != "" {
			gymName = name
		}
	}

	parts := []string{fmt.Sprintf("**%s %s**", teamName, tr.Translate("gyms", false))}
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	if slotChanges {
		parts = append(parts, fmt.Sprintf("| %s", tr.Translate("including slot changes", false)))
	}
	if battleChanges {
		parts = append(parts, fmt.Sprintf("| %s", tr.Translate("including battle changes", false)))
	}
	parts = append(parts, standardText(cfg, tr, row))
	if gymName != "" {
		parts = append(parts, fmt.Sprintf("%s %s", tr.Translate("at gym ", false), gymName))
	}
	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}
