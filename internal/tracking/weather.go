package tracking

import (
	"fmt"
	"strings"

	"dexter/internal/data"
	"dexter/internal/i18n"
)

// WeatherRowText formats weather tracking output.
func WeatherRowText(tr *i18n.Translator, game *data.GameData, row map[string]any) string {
	condition := fmt.Sprintf("%v", row["condition"])
	cell := fmt.Sprintf("%v", row["cell"])

	name := ""
	if raw, ok := game.UtilData["weather"].(map[string]any); ok {
		if entry, ok := raw[condition].(map[string]any); ok {
			name = fmt.Sprintf("%v", entry["name"])
		}
	}
	if name == "" && condition == "0" {
		name = "everything"
	}
	if name != "" {
		name = tr.Translate(name, false)
	}

	parts := []string{}
	if name != "" {
		parts = append(parts, name)
	} else {
		parts = append(parts, "weather")
	}
	if cell != "" {
		parts = append(parts, fmt.Sprintf("cell:%s", cell))
	}
	return strings.Join(parts, " ")
}
