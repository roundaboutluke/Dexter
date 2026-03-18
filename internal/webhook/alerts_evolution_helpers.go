package webhook

import (
	"fmt"
	"strings"

	"dexter/internal/i18n"
)

func applyPokemonEvolutions(p *Processor, data map[string]any, pokemonID, formID int, platform string, tr *i18n.Translator) {
	d := p.getData()
	if d == nil || data == nil || pokemonID <= 0 {
		return
	}
	monster := lookupMonster(d, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(d, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		return
	}

	hasEvolutions := false
	switch v := monster["evolutions"].(type) {
	case []any:
		hasEvolutions = len(v) > 0
	case []map[string]any:
		hasEvolutions = len(v) > 0
	}
	data["hasEvolutions"] = hasEvolutions

	count := 0
	evolutions := make([]map[string]any, 0)
	megaEvolutions := make([]map[string]any, 0)
	collectPokemonEvolutions(p, monster, &count, &evolutions, &megaEvolutions, platform, tr)

	data["evolutions"] = evolutions
	data["hasMegaEvolutions"] = len(megaEvolutions) > 0
	data["megaEvolutions"] = megaEvolutions
}

func collectPokemonEvolutions(p *Processor, monster map[string]any, totalCount *int, evolutions *[]map[string]any, megaEvolutions *[]map[string]any, platform string, tr *i18n.Translator) {
	if p == nil || monster == nil || totalCount == nil || evolutions == nil || megaEvolutions == nil {
		return
	}
	*totalCount++
	if *totalCount >= 10 {
		return
	}

	d := p.getData()
	switch raw := monster["evolutions"].(type) {
	case []any:
		for _, entry := range raw {
			evo, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			evoID := getInt(evo["evoId"])
			formID := getInt(evo["id"])
			if evoID <= 0 {
				continue
			}
			next := lookupMonster(d, fmt.Sprintf("%d_%d", evoID, formID))
			if next == nil && formID != 0 {
				next = lookupMonster(d, fmt.Sprintf("%d_0", evoID))
			}
			if next == nil {
				continue
			}

			nameEng := getString(next["name"])
			name := translateMaybe(tr, nameEng)
			formNameEng := ""
			if form, ok := next["form"].(map[string]any); ok {
				formNameEng = getString(form["name"])
			}
			formNormalisedEng := formNameEng
			if strings.EqualFold(formNormalisedEng, "Normal") {
				formNormalisedEng = ""
			}
			formNormalised := translateMaybe(tr, formNormalisedEng)
			fullNameEng := nameEng
			if formNormalisedEng != "" {
				fullNameEng = fmt.Sprintf("%s %s", nameEng, formNormalisedEng)
			}
			fullName := name
			if formNormalised != "" {
				fullName = fmt.Sprintf("%s %s", name, formNormalised)
			}

			typeNames := monsterTypeNames(p, evoID, formID)
			translatedTypes := make([]string, 0, len(typeNames))
			typeEmojis := make([]string, 0, len(typeNames))
			for _, typeName := range typeNames {
				translatedTypes = append(translatedTypes, translateMaybe(tr, typeName))
				if _, emojiKey := typeStyle(p, typeName); emojiKey != "" {
					if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
						typeEmojis = append(typeEmojis, translateMaybe(tr, emoji))
					}
				}
			}

			item := map[string]any{
				"id":                evoID,
				"form":              formID,
				"fullName":          fullName,
				"fullNameEng":       fullNameEng,
				"formNormalised":    formNormalised,
				"formNormalisedEng": formNormalisedEng,
				"name":              name,
				"nameEng":           nameEng,
				"formNameEng":       formNameEng,
				"typeName":          strings.Join(translatedTypes, ", "),
				"typeEmoji":         strings.Join(typeEmojis, ""),
			}
			if stats, ok := next["stats"].(map[string]any); ok {
				item["baseStats"] = stats
			}
			*evolutions = append(*evolutions, item)

			if *totalCount < 10 {
				collectPokemonEvolutions(p, next, totalCount, evolutions, megaEvolutions, platform, tr)
			}
		}
	}

	switch raw := monster["tempEvolutions"].(type) {
	case []any:
		for _, entry := range raw {
			evo, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			tempID := getInt(evo["tempEvoId"])
			if tempID == 0 {
				tempID = getInt(evo["tempEvoID"])
			}
			if tempID == 0 {
				continue
			}
			nameEng := getString(monster["name"])
			name := translateMaybe(tr, nameEng)
			fullNameEng := nameEng
			fullName := name
			if format := megaNameFormat(p, tempID); format != "" {
				fullNameEng = formatTemplate(format, nameEng)
				fullName = formatTemplate(format, name)
			}

			var typesPayload []any
			if types, ok := evo["types"].([]any); ok && len(types) > 0 {
				typesPayload = types
			} else if types, ok := monster["types"].([]any); ok && len(types) > 0 {
				typesPayload = types
			}

			typeNames := make([]string, 0, len(typesPayload))
			for _, entry := range typesPayload {
				if m, ok := entry.(map[string]any); ok {
					if name := getString(m["name"]); name != "" {
						typeNames = append(typeNames, name)
					}
				}
			}
			translatedTypes := make([]string, 0, len(typeNames))
			typeEmojis := make([]string, 0, len(typeNames))
			for _, typeName := range typeNames {
				translatedTypes = append(translatedTypes, translateMaybe(tr, typeName))
				if _, emojiKey := typeStyle(p, typeName); emojiKey != "" {
					if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
						typeEmojis = append(typeEmojis, translateMaybe(tr, emoji))
					}
				}
			}

			item := map[string]any{
				"fullName":    fullName,
				"fullNameEng": fullNameEng,
				"evolution":   tempID,
				"types":       typesPayload,
				"typeName":    strings.Join(translatedTypes, ", "),
				"typeEmoji":   strings.Join(typeEmojis, ""),
			}
			if stats, ok := evo["stats"].(map[string]any); ok {
				item["baseStats"] = stats
			}
			*megaEvolutions = append(*megaEvolutions, item)
		}
	}
}
