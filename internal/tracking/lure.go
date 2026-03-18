package tracking

import (
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/i18n"
)

// LureRowText mirrors PoracleJS lure tracking formatting.
func LureRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any) string {
	lureID := intFromAny(row["lure_id"])
	typeText := "any"
	if lureID != 0 {
		if name := lureName(game.UtilData, lureID); name != "" {
			typeText = name
		}
	}
	parts := []string{
		fmt.Sprintf("%s: **%s**", tr.Translate("Lure type", false), tr.Translate(typeText, true)),
	}
	distance := intFromAny(row["distance"])
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	parts = append(parts, standardText(cfg, tr, row))
	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}

func lureName(util map[string]any, id int) string {
	entry, ok := util["lures"]
	if !ok {
		return ""
	}
	switch v := entry.(type) {
	case []any:
		if id >= 0 && id < len(v) {
			if m, ok := v[id].(map[string]any); ok {
				return getString(m["name"])
			}
		}
	case map[string]any:
		if value, ok := v[fmt.Sprintf("%d", id)]; ok {
			if m, ok := value.(map[string]any); ok {
				return getString(m["name"])
			}
			return getString(value)
		}
	}
	return ""
}
