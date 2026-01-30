package tracking

import (
	"fmt"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

// FortUpdateRowText mirrors PoracleJS fort tracking formatting.
func FortUpdateRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any) string {
	fortType := getString(row["fort_type"])
	distance := intFromAny(row["distance"])
	changeTypes := strings.TrimSpace(getString(row["change_types"]))
	includeEmpty := intFromAny(row["include_empty"]) != 0

	parts := []string{
		fmt.Sprintf("%s: **%s**", tr.Translate("Fort updates", false), tr.Translate(fortType, false)),
	}
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	if changeTypes != "" {
		parts = append(parts, changeTypes)
	}
	if includeEmpty {
		parts = append(parts, "including empty changes")
	}
	parts = append(parts, standardText(cfg, tr, row))

	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}
