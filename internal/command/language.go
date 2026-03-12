package command

import (
	"fmt"
	"strings"
)

// LanguageCommand updates user language.
type LanguageCommand struct{}

func (c *LanguageCommand) Name() string { return "language" }

func (c *LanguageCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "language") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}

	raw, ok := ctx.Config.Get("general.availableLanguages")
	if !ok {
		return "🙅", nil
	}
	available, ok := raw.(map[string]any)
	if !ok || len(available) == 0 {
		return "🙅", nil
	}

	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	languageNames := map[string]string{}
	if rawNames, ok := ctx.Data.UtilData["languageNames"].(map[string]any); ok {
		for key, value := range rawNames {
			languageNames[key] = fmt.Sprintf("%v", value)
		}
	}

	if len(args) == 0 {
		currentName := languageNames[result.Language]
		if currentName == "" {
			currentName = result.Language
		}
		keys := []string{}
		for key := range available {
			keys = append(keys, key)
		}
		return fmt.Sprintf("%s: %s\n%s", tr.Translate("Current language is set to", false), currentName, tr.TranslateFormat("Use `{0}language` to set to one of {1}", ctx.Prefix, strings.Join(keys, ", "))), nil
	}

	newLanguage := strings.ToLower(args[0])
	if _, ok := available[newLanguage]; !ok {
		for key, name := range languageNames {
			if strings.EqualFold(name, args[0]) {
				newLanguage = key
				break
			}
		}
	}
	if _, ok := available[newLanguage]; !ok {
		keys := []string{}
		for key := range available {
			keys = append(keys, key)
		}
		return fmt.Sprintf("%s: %s", tr.Translate("I only recognise the following languages", false), strings.Join(keys, ", ")), nil
	}

	if _, err := ctx.Query.UpdateQuery("humans", map[string]any{"language": newLanguage}, map[string]any{"id": result.TargetID}); err != nil {
		return "", err
	}
	ctx.MarkAlertStateDirty()
	name := languageNames[newLanguage]
	if name == "" {
		name = newLanguage
	}
	newTranslator := ctx.I18n.Translator(newLanguage)
	return fmt.Sprintf("%s: %s", newTranslator.Translate("I have changed your language setting to", false), newTranslator.Translate(name, false)), nil
}
