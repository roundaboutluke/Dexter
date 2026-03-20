package webhook

import (
	"fmt"
	"strconv"
	"strings"

	"dexter/internal/i18n"
)

func caredPokemonFromHook(p *Processor, hook *Hook) *caredPokemon {
	if p == nil || hook == nil {
		return nil
	}
	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		return nil
	}
	formID := hookFormID(hook.Message)
	nameEng := monsterName(p, pokemonID)
	formName := monsterFormName(p, pokemonID, formID)
	fullName := nameEng
	if formName != "" && !strings.EqualFold(formName, "Normal") {
		fullName = fmt.Sprintf("%s %s", nameEng, formName)
	}
	weatherID := getInt(hook.Message["weather"])
	if boosted := getInt(hook.Message["boosted_weather"]); boosted > 0 {
		weatherID = boosted
	}
	if weatherID == 0 {
		weatherID = weatherCondition(hook.Message)
	}
	types := monsterTypes(p, pokemonID, formID)
	altering := alteringWeathers(p, types, weatherID)
	ivString := "-1"
	if hasNumeric(hook.Message["individual_attack"]) && hasNumeric(hook.Message["individual_defense"]) && hasNumeric(hook.Message["individual_stamina"]) {
		if ivValue := computeIV(hook); ivValue >= 0 {
			ivString = fmt.Sprintf("%.2f", ivValue)
		}
	}
	return &caredPokemon{
		PokemonID:        pokemonID,
		Form:             formID,
		Name:             nameEng,
		FormName:         formName,
		FullName:         fullName,
		IV:               ivString,
		CP:               getInt(hook.Message["cp"]),
		Latitude:         getFloat(hook.Message["latitude"]),
		Longitude:        getFloat(hook.Message["longitude"]),
		DisappearTime:    getInt64(hook.Message["disappear_time"]),
		AlteringWeathers: altering,
	}
}

func alteringWeathers(p *Processor, types []int, weather int) []int {
	d := p.getData()
	if d == nil || d.UtilData == nil {
		return nil
	}
	raw, ok := d.UtilData["weatherTypeBoost"].(map[string]any)
	if !ok || len(types) == 0 {
		return nil
	}
	boosting := []int{}
	for key, value := range raw {
		weatherID, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			typeID := getInt(item)
			if containsInt(types, typeID) {
				boosting = append(boosting, weatherID)
				break
			}
		}
	}
	if weather > 0 {
		nonBoosting := []int{}
		for id := 1; id <= 7; id++ {
			if !containsInt(boosting, id) {
				nonBoosting = append(nonBoosting, id)
			}
		}
		return nonBoosting
	}
	return boosting
}

func weatherBoostsTypes(p *Processor, weatherID int, types []int) bool {
	d := p.getData()
	if d == nil || d.UtilData == nil || weatherID == 0 || len(types) == 0 {
		return false
	}
	raw, ok := d.UtilData["weatherTypeBoost"].(map[string]any)
	if !ok {
		return false
	}
	items, ok := raw[strconv.Itoa(weatherID)].([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if containsInt(types, getInt(item)) {
			return true
		}
	}
	return false
}

func boostingWeathersForTypes(p *Processor, types []int) []int {
	d := p.getData()
	if d == nil || d.UtilData == nil || len(types) == 0 {
		return nil
	}
	raw, ok := d.UtilData["weatherTypeBoost"].(map[string]any)
	if !ok {
		return nil
	}
	out := []int{}
	for _, typeID := range types {
		weatherID := 0
		for key, value := range raw {
			items, ok := value.([]any)
			if !ok {
				continue
			}
			for _, item := range items {
				if getInt(item) == typeID {
					if parsed, err := strconv.Atoi(key); err == nil {
						weatherID = parsed
					}
					break
				}
			}
			if weatherID > 0 {
				break
			}
		}
		if weatherID > 0 {
			out = append(out, weatherID)
		}
	}
	return out
}

func weaknessListForTypes(p *Processor, typeNames []string, platform string, tr *i18n.Translator) ([]map[string]any, string) {
	d := p.getData()
	if d == nil || len(typeNames) == 0 {
		return nil, ""
	}
	rawTypes := d.Types
	if rawTypes == nil {
		return nil, ""
	}
	utilTypes, ok := d.UtilData["types"].(map[string]any)
	if !ok {
		return nil, ""
	}
	weaknesses := map[string]float64{}
	for _, typeName := range typeNames {
		entry, ok := rawTypes[typeName].(map[string]any)
		if !ok {
			continue
		}
		for _, item := range []struct {
			key    string
			factor float64
		}{
			{"weaknesses", 2},
			{"resistances", 0.5},
			{"immunes", 0.25},
		} {
			rawList, ok := entry[item.key].([]any)
			if !ok {
				continue
			}
			for _, raw := range rawList {
				m, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				target := getString(m["typeName"])
				if target == "" {
					continue
				}
				if _, ok := weaknesses[target]; !ok {
					weaknesses[target] = 1
				}
				weaknesses[target] *= item.factor
			}
		}
	}
	typeObj := []struct {
		key   string
		value float64
		text  string
	}{
		{"extraWeak", 4, "Very vulnerable to"},
		{"weak", 2, "Vulnerable to"},
		{"resist", 0.5, "Resistant to"},
		{"immune", 0.25, "Very resistant to"},
		{"extraImmune", 0.125, "Extremely resistant to"},
	}
	list := []map[string]any{}
	weaknessEmoji := ""
	for _, entry := range typeObj {
		group := map[string]any{
			"value": entry.value,
			"text":  translateMaybe(tr, entry.text),
			"types": []map[string]any{},
		}
		types := group["types"].([]map[string]any)
		for name, value := range weaknesses {
			if value != entry.value {
				continue
			}
			emojiKey := ""
			if utilEntry, ok := utilTypes[name].(map[string]any); ok {
				emojiKey = getString(utilEntry["emoji"])
			}
			emoji := ""
			if emojiKey != "" {
				emoji = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, platform))
			}
			types = append(types, map[string]any{
				"nameEng": name,
				"name":    translateMaybe(tr, name),
				"emoji":   emoji,
			})
		}
		if len(types) == 0 {
			continue
		}
		group["types"] = types
		typeEmoji := ""
		for _, entry := range types {
			typeEmoji += getString(entry["emoji"])
		}
		group["typeEmoji"] = typeEmoji
		weaknessEmoji = weaknessEmoji + fmt.Sprintf("%gx%s ", entry.value, typeEmoji)
		list = append(list, group)
	}
	return list, strings.TrimSpace(weaknessEmoji)
}
