package tracking

import (
	"fmt"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/i18n"
)

// MonsterRowText mirrors PoracleJS monster tracking formatting.
func MonsterRowText(cfg *config.Config, tr *i18n.Translator, game *data.GameData, row map[string]any) string {
	monsterID := intFromAny(row["pokemon_id"])
	formID := intFromAny(row["form"])
	monsterName := ""
	formName := ""

	if monsterID == 0 {
		monsterName = tr.Translate("Everything", false)
	} else {
		mon := findMonster(game, monsterID, formID)
		if mon == nil {
			monsterName = fmt.Sprintf("%s %d", tr.Translate("Unknown monster", false), monsterID)
			formName = fmt.Sprintf("%d", formID)
		} else {
			monsterName = getString(mon["name"])
			form := getMap(mon["form"])
			formName = getString(form["name"])
			if formName == "" || (intFromAny(form["id"]) == 0 && formName == "Normal") {
				formName = ""
			}
		}
	}

	minIV := intFromAny(row["min_iv"])
	if minIV == -1 {
		minIV = -1
	}
	minRarity := intFromAny(row["rarity"])
	if minRarity == -1 {
		minRarity = 1
	}
	minSize := intFromAny(row["size"])
	if minSize < 1 {
		minSize = 1
	}

	pvpLeague := intFromAny(row["pvp_ranking_league"])
	pvpString := ""
	if pvpLeague != 0 {
		league := map[int]string{500: tr.Translate("littlepvp", false), 1500: tr.Translate("greatpvp", false), 2500: tr.Translate("ultrapvp", false)}
		pvpString = fmt.Sprintf("%s %s top%s%d (@%d+%s)",
			tr.Translate("pvp ranking:", false),
			league[pvpLeague],
			pvpBestSuffix(intFromAny(row["pvp_ranking_best"])),
			intFromAny(row["pvp_ranking_worst"]),
			intFromAny(row["pvp_ranking_min_cp"]),
			pvpCapSuffix(tr, intFromAny(row["pvp_ranking_cap"])),
		)
		pvpString = strings.TrimSpace(pvpString)
	}

	parts := []string{
		fmt.Sprintf("**%s**", tr.Translate(monsterName, false)),
		tr.Translate(formName, false),
	}

	distance := intFromAny(row["distance"])
	if distance > 0 {
		parts = append(parts, fmt.Sprintf("| %s: %dm", tr.Translate("distance", false), distance))
	}

	maxIV := intFromAny(row["max_iv"])
	parts = append(parts, fmt.Sprintf("| %s: %s%%-%d%%", tr.Translate("iv", false), formatMinValue(minIV), maxIV))
	parts = append(parts, fmt.Sprintf("| %s: %d-%d", tr.Translate("cp", false), intFromAny(row["min_cp"]), intFromAny(row["max_cp"])))
	parts = append(parts, fmt.Sprintf("| %s: %d-%d", tr.Translate("level", false), intFromAny(row["min_level"]), intFromAny(row["max_level"])))
	parts = append(parts, fmt.Sprintf("| %s: %d/%d/%d - %d/%d/%d",
		tr.Translate("stats", false),
		intFromAny(row["atk"]), intFromAny(row["def"]), intFromAny(row["sta"]),
		intFromAny(row["max_atk"]), intFromAny(row["max_def"]), intFromAny(row["max_sta"]),
	))
	if pvpString != "" {
		parts = append(parts, fmt.Sprintf("| %s", pvpString))
	}

	if intFromAny(row["size"]) > 0 || intFromAny(row["max_size"]) < 6 {
		parts = append(parts, fmt.Sprintf("| %s: %s-%s",
			tr.Translate("size", false),
			tr.Translate(utilLookup(game.UtilData, "size", minSize), false),
			tr.Translate(utilLookup(game.UtilData, "size", intFromAny(row["max_size"])), false),
		))
	}
	if intFromAny(row["rarity"]) > 0 || intFromAny(row["max_rarity"]) < 6 {
		parts = append(parts, fmt.Sprintf("| %s: %s-%s",
			tr.Translate("rarity", false),
			tr.Translate(utilLookup(game.UtilData, "rarity", minRarity), false),
			tr.Translate(utilLookup(game.UtilData, "rarity", intFromAny(row["max_rarity"])), false),
		))
	}
	gender := intFromAny(row["gender"])
	if gender != 0 {
		parts = append(parts, fmt.Sprintf("| %s: %s", tr.Translate("gender", false), genderEmoji(game.UtilData, gender)))
	}
	minTime := intFromAny(row["min_time"])
	if minTime > 0 {
		parts = append(parts, fmt.Sprintf("| %s %ds", tr.Translate("minimum time:", false), minTime))
	}
	parts = append(parts, standardText(cfg, tr, row))

	return strings.TrimSpace(strings.Join(cleanParts(parts), " "))
}

func standardText(cfg *config.Config, tr *i18n.Translator, row map[string]any) string {
	text := ""
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	if getString(row["template"]) != defaultTemplate {
		text += fmt.Sprintf(" %s: %s", tr.Translate("template", false), getString(row["template"]))
	}
	if intFromAny(row["clean"]) != 0 {
		text += fmt.Sprintf(" %s", tr.Translate("clean", false))
	}
	return text
}

func findMonster(game *data.GameData, pokemonID int, formID int) map[string]any {
	for _, raw := range game.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if intFromAny(mon["id"]) != pokemonID {
			continue
		}
		form := getMap(mon["form"])
		if intFromAny(form["id"]) == formID {
			return mon
		}
	}
	return nil
}

func utilLookup(util map[string]any, key string, index int) string {
	entry, ok := util[key]
	if !ok {
		return ""
	}
	switch v := entry.(type) {
	case []any:
		if index >= 0 && index < len(v) {
			return getString(v[index])
		}
	case map[string]any:
		if value, ok := v[fmt.Sprintf("%d", index)]; ok {
			if inner, ok := value.(map[string]any); ok {
				return getString(inner["emoji"])
			}
			return getString(value)
		}
	}
	return ""
}

func genderEmoji(util map[string]any, index int) string {
	entry, ok := util["genders"]
	if !ok {
		return ""
	}
	switch v := entry.(type) {
	case []any:
		if index >= 0 && index < len(v) {
			if m, ok := v[index].(map[string]any); ok {
				return getString(m["emoji"])
			}
			return getString(v[index])
		}
	}
	return ""
}

func pvpBestSuffix(best int) string {
	if best > 1 {
		return fmt.Sprintf("%d-", best)
	}
	return ""
}

func pvpCapSuffix(tr *i18n.Translator, cap int) string {
	if cap > 0 {
		return fmt.Sprintf(" %s%d", tr.Translate("level cap:", false), cap)
	}
	return ""
}

func formatMinValue(value int) string {
	if value == -1 {
		return "?"
	}
	return fmt.Sprintf("%d", value)
}

func getString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func getMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var out int
		_, _ = fmt.Sscanf(v, "%d", &out)
		return out
	default:
		return 0
	}
}

func cleanParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
