package render

import (
	"fmt"

	"github.com/aymerick/raymond"

	"poraclego/internal/i18n"
)

func userTranslator(options *raymond.Options) *i18n.Translator {
	if translator == nil {
		return nil
	}
	language := options.DataStr("language")
	if language == "" && cfg != nil {
		if val, ok := cfg.GetString("general.locale"); ok {
			language = val
		}
	}
	return translator.Translator(language)
}

func altTranslator() *i18n.Translator {
	if translator == nil {
		return nil
	}
	if altLanguage == "" && cfg != nil {
		if val, ok := cfg.GetString("general.locale"); ok {
			altLanguage = val
		}
	}
	return translator.Translator(altLanguage)
}

func emojiLookup(name string, platform string) string {
	if customEmoji != nil && platform != "" {
		if platformMap, ok := customEmoji[platform]; ok {
			if val, ok := platformMap[name]; ok {
				return val
			}
		}
	}
	if gameData == nil || gameData.UtilData == nil {
		return ""
	}
	emojis, ok := gameData.UtilData["emojis"].(map[string]any)
	if !ok {
		return ""
	}
	if val, ok := emojis[name]; ok {
		return fmt.Sprintf("%v", val)
	}
	return ""
}

func moveNameHelper(value interface{}, options *raymond.Options) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(getString(move["name"]), false)
	}
	return getString(move["name"])
}

func moveNameAltHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		return tr.Translate(getString(move["name"]), false)
	}
	return getString(move["name"])
}

func moveNameEngHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	return getString(move["name"])
}

func moveTypeHelper(value interface{}, options *raymond.Options) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := userTranslator(options); tr != nil {
		return tr.Translate(getString(move["type"]), false)
	}
	return getString(move["type"])
}

func moveTypeAltHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	if tr := altTranslator(); tr != nil {
		return tr.Translate(getString(move["type"]), false)
	}
	return getString(move["type"])
}

func moveTypeEngHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	return getString(move["type"])
}

func moveEmojiHelper(value interface{}, options *raymond.Options) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	types := utilTypes()
	moveType := getString(move["type"])
	if entry, ok := types[moveType].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		emoji := emojiLookup(emojiName, options.DataStr("platform"))
		if tr := userTranslator(options); tr != nil {
			return tr.Translate(emoji, false)
		}
		return emoji
	}
	return ""
}

func moveEmojiAltHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	types := utilTypes()
	moveType := getString(move["type"])
	if entry, ok := types[moveType].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		emoji := emojiLookup(emojiName, "")
		if tr := altTranslator(); tr != nil {
			return tr.Translate(emoji, false)
		}
		return emoji
	}
	return ""
}

func moveEmojiEngHelper(value interface{}) string {
	move := moveByID(toInt(value, 0))
	if move == nil {
		return ""
	}
	types := utilTypes()
	moveType := getString(move["type"])
	if entry, ok := types[moveType].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		return emojiLookup(emojiName, "")
	}
	return ""
}

func typeEmoji(typeName string, options *raymond.Options) string {
	types := utilTypes()
	if entry, ok := types[typeName].(map[string]any); ok {
		emojiName := getString(entry["emoji"])
		emoji := emojiLookup(emojiName, options.DataStr("platform"))
		if tr := userTranslator(options); tr != nil {
			return tr.Translate(emoji, false)
		}
		return emoji
	}
	return ""
}

func translateTypeNames(names []string, tr *i18n.Translator) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, translateMaybe(tr, name))
	}
	return out
}

func translateAltHelper(value interface{}) string {
	if tr := altTranslator(); tr != nil {
		return tr.Translate(fmt.Sprintf("%v", value), false)
	}
	return fmt.Sprintf("%v", value)
}
