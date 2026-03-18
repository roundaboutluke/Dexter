package tracking

import (
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/data"
	"dexter/internal/i18n"
)

// InvasionRowText mirrors PoracleJS invasion tracking formatting.
func InvasionRowText(cfg *config.Config, tr *i18n.Translator, _ *data.GameData, row map[string]any) string {
	gender := intFromAny(row["gender"])
	grunt := getString(row["grunt_type"])
	genderText := tr.Translate("any", false)
	if gender == 1 {
		genderText = tr.Translate("male", false)
	} else if gender == 2 {
		genderText = tr.Translate("female", false)
	}
	if grunt == "" {
		grunt = "any"
	}
	parts := []string{
		fmt.Sprintf("%s: **%s**", titleCase(tr.Translate("grunt type", false)), tr.Translate(grunt, true)),
	}
	distance := intFromAny(row["distance"])
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}
	parts = append(parts, fmt.Sprintf("| %s: %s", tr.Translate("gender", false), genderText))
	parts = append(parts, standardText(cfg, tr, row))

	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}
