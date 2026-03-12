package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/aymerick/raymond"

	"poraclego/internal/i18n"
)

func pokemonHelper(id interface{}, form interface{}, options *raymond.Options) string {
	if options == nil || options.Fn() == "" {
		return ""
	}
	formID := toInt(form, 0)
	pokemonID := toInt(id, 0)
	if pokemonID == 0 {
		return ""
	}
	monster := monsterByIDForm(pokemonID, formID)
	if monster == nil {
		return ""
	}
	formName := ""
	if formMap, ok := monster["form"].(map[string]any); ok {
		formName = getString(formMap["name"])
	}
	formNormalisedEng := ""
	if formName != "" && formName != "Normal" {
		formNormalisedEng = formName
	}
	tr := userTranslator(options)
	nameEng := getString(monster["name"])
	name := nameEng
	if tr != nil {
		name = tr.Translate(nameEng, false)
	}
	formNormalised := formNormalisedEng
	if tr != nil && formNormalisedEng != "" {
		formNormalised = tr.Translate(formNormalisedEng, false)
	}
	typeNames := []string{}
	typeEmojis := []string{}
	if typesRaw, ok := monster["types"].([]any); ok {
		for _, typeItem := range typesRaw {
			if typeMap, ok := typeItem.(map[string]any); ok {
				typeName := getString(typeMap["name"])
				typeNames = append(typeNames, typeName)
				if emoji := typeEmoji(typeName, options); emoji != "" {
					typeEmojis = append(typeEmojis, emoji)
				}
			}
		}
	}
	fullNameEng := strings.TrimSpace(strings.Join([]string{nameEng, formNormalisedEng}, " "))
	fullName := strings.TrimSpace(strings.Join([]string{name, formNormalised}, " "))
	ctx := map[string]any{
		"name":              name,
		"nameEng":           nameEng,
		"formName":          formName,
		"formNameEng":       formName,
		"fullName":          fullName,
		"fullNameEng":       fullNameEng,
		"formNormalised":    formNormalised,
		"formNormalisedEng": formNormalisedEng,
		"emoji":             typeEmojis,
		"typeNameEng":       typeNames,
		"typeName":          translateTypeNames(typeNames, tr),
		"typeEmoji":         strings.Join(typeEmojis, ""),
		"hasEvolutions":     monster["evolutions"] != nil,
		"baseStats":         monster["stats"],
	}
	return options.FnWith(ctx)
}

func pokemonNameHelper(value interface{}, options *raymond.Options) string {
	pokemonID := toInt(value, 0)
	monster := monsterByID(pokemonID)
	if monster == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(getString(monster["name"]), false)
	}
	return getString(monster["name"])
}

func pokemonNameAltHelper(value interface{}) string {
	pokemonID := toInt(value, 0)
	monster := monsterByID(pokemonID)
	if monster == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		return tr.Translate(getString(monster["name"]), false)
	}
	return getString(monster["name"])
}

func pokemonNameEngHelper(value interface{}) string {
	pokemonID := toInt(value, 0)
	monster := monsterByID(pokemonID)
	if monster == nil {
		return ""
	}
	return getString(monster["name"])
}

func pokemonFormHelper(value interface{}, options *raymond.Options) string {
	formID := toInt(value, 0)
	monster := monsterByForm(formID)
	if monster == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		if formMap, ok := monster["form"].(map[string]any); ok {
			return tr.Translate(getString(formMap["name"]), false)
		}
		return ""
	}
	if formMap, ok := monster["form"].(map[string]any); ok {
		return getString(formMap["name"])
	}
	return ""
}

func pokemonFormAltHelper(value interface{}) string {
	formID := toInt(value, 0)
	monster := monsterByForm(formID)
	if monster == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		if formMap, ok := monster["form"].(map[string]any); ok {
			return tr.Translate(getString(formMap["name"]), false)
		}
		return ""
	}
	if formMap, ok := monster["form"].(map[string]any); ok {
		return getString(formMap["name"])
	}
	return ""
}

func pokemonFormEngHelper(value interface{}) string {
	formID := toInt(value, 0)
	monster := monsterByForm(formID)
	if monster == nil {
		return ""
	}
	if formMap, ok := monster["form"].(map[string]any); ok {
		return getString(formMap["name"])
	}
	return ""
}

func calculateCpHelper(baseStats interface{}, level interface{}, ivAttack interface{}, ivDefense interface{}, ivStamina interface{}) string {
	stats, ok := baseStats.(map[string]any)
	if !ok {
		return "0"
	}
	baseAtk := toFloat(stats["baseAttack"])
	baseDef := toFloat(stats["baseDefense"])
	baseSta := toFloat(stats["baseStamina"])
	lvl := toFloat(level)
	if lvl == 0 {
		lvl = 25
	}
	atk := toFloat(ivAttack)
	def := toFloat(ivDefense)
	sta := toFloat(ivStamina)
	cpMulti := cpMultiplier(lvl)
	cp := math.Max(10, math.Floor((baseAtk+atk)*math.Pow(baseDef+def, 0.5)*math.Pow(baseSta+sta, 0.5)*math.Pow(cpMulti, 2)/10))
	return fmt.Sprintf("%0.0f", cp)
}

func pokemonBaseStatsHelper(pokemonID interface{}, formID interface{}) map[string]any {
	monster := monsterByIDForm(toInt(pokemonID, 0), toInt(formID, 0))
	if monster == nil {
		return map[string]any{"baseAttack": 0, "baseDefense": 0, "baseStamina": 0}
	}
	if stats, ok := monster["stats"].(map[string]any); ok {
		return stats
	}
	return map[string]any{"baseAttack": 0, "baseDefense": 0, "baseStamina": 0}
}

func getEmojiHelper(name interface{}, options *raymond.Options) string {
	emoji := emojiLookup(fmt.Sprintf("%v", name), options.DataStr("platform"))
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(emoji, false)
	}
	return emoji
}

func getPowerUpCostHelper(levelStart interface{}, levelEnd interface{}, options *raymond.Options) string {
	start := toInt(levelStart, 0)
	end := toInt(levelEnd, 0)
	if start == 0 || end == 0 {
		return ""
	}
	stardust := 0
	candy := 0
	xl := 0
	costs := utilPowerUpCost()
	for level, raw := range costs {
		levelInt := toInt(level, 0)
		if levelInt >= start && levelInt < end {
			if m, ok := raw.(map[string]any); ok {
				stardust += toInt(m["stardust"], 0)
				candy += toInt(m["candy"], 0)
				xl += toInt(m["xlCandy"], 0)
			}
		}
	}
	if isCurrentBlockHelper(options, "getPowerUpCost") {
		return options.FnWith(map[string]any{"stardust": stardust, "candy": candy, "xl": xl})
	}
	tr := userTranslator(options)
	stardustLabel := "Stardust"
	candyLabel := "Candies"
	xlLabel := "XL Candies"
	if tr != nil {
		stardustLabel = tr.Translate(stardustLabel, false)
		candyLabel = tr.Translate(candyLabel, false)
		xlLabel = tr.Translate(xlLabel, false)
	}
	parts := []string{}
	if stardust > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", formatWithCommas(int64(stardust)), stardustLabel))
	}
	if candy > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", formatWithCommas(int64(candy)), candyLabel))
	}
	if xl > 0 {
		parts = append(parts, fmt.Sprintf("%s %s", formatWithCommas(int64(xl)), xlLabel))
	}
	return strings.Join(parts, " and ")
}

func moveByID(id int) map[string]any {
	if gameData == nil || gameData.Moves == nil || id == 0 {
		return nil
	}
	raw, ok := gameData.Moves[fmt.Sprintf("%d", id)]
	if !ok {
		return nil
	}
	if m, ok := raw.(map[string]any); ok {
		return m
	}
	return nil
}

func monsterByID(id int) map[string]any {
	if gameData == nil || gameData.Monsters == nil || id == 0 {
		return nil
	}
	key := fmt.Sprintf("%d_0", id)
	if raw, ok := gameData.Monsters[key]; ok {
		if m, ok := raw.(map[string]any); ok {
			return m
		}
	}
	for _, raw := range gameData.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if toInt(m["id"], 0) == id {
				return m
			}
		}
	}
	return nil
}

func monsterByIDForm(id, form int) map[string]any {
	if gameData == nil || gameData.Monsters == nil || id == 0 {
		return nil
	}
	key := fmt.Sprintf("%d_%d", id, form)
	if raw, ok := gameData.Monsters[key]; ok {
		if m, ok := raw.(map[string]any); ok {
			return m
		}
	}
	for _, raw := range gameData.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if toInt(m["id"], 0) == id {
				if formVal, ok := m["form"].(map[string]any); ok && toInt(formVal["id"], 0) == form {
					return m
				}
			}
		}
	}
	return nil
}

func monsterByForm(form int) map[string]any {
	if gameData == nil || gameData.Monsters == nil || form == 0 {
		return nil
	}
	for _, raw := range gameData.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if formVal, ok := m["form"].(map[string]any); ok && toInt(formVal["id"], 0) == form {
				return m
			}
		}
	}
	return nil
}

func utilTypes() map[string]any {
	if gameData == nil || gameData.UtilData == nil {
		return map[string]any{}
	}
	if types, ok := gameData.UtilData["types"].(map[string]any); ok {
		return types
	}
	return map[string]any{}
}

func utilPowerUpCost() map[string]any {
	if gameData == nil || gameData.UtilData == nil {
		return map[string]any{}
	}
	if costs, ok := gameData.UtilData["powerUpCost"].(map[string]any); ok {
		return costs
	}
	return map[string]any{}
}

func cpMultiplier(level float64) float64 {
	if gameData == nil || gameData.UtilData == nil {
		return 1
	}
	if mults, ok := gameData.UtilData["cpMultipliers"].(map[string]any); ok {
		if val, ok := mults[fmt.Sprintf("%v", level)]; ok {
			return toFloat(val)
		}
	}
	return 1
}

func getString(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case map[string]any:
		if name, ok := v["name"].(string); ok {
			return name
		}
	}
	return fmt.Sprintf("%v", value)
}

func translateMaybe(tr *i18n.Translator, value string) string {
	if tr == nil || value == "" {
		return value
	}
	return tr.Translate(value, false)
}
