package command

import (
	"fmt"
	"sort"
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

	if ctx.I18n == nil {
		return "🙅", nil
	}
	available := ctx.I18n.EffectiveLanguages()
	if len(available) == 0 {
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
		keys := append([]string(nil), available...)
		sort.Strings(keys)
		return fmt.Sprintf("%s: %s\n%s", tr.Translate("Current language is set to", false), currentName, tr.TranslateFormat("Use `{0}language` to set to one of {1}", ctx.Prefix, strings.Join(keys, ", "))), nil
	}

	newLanguage := strings.ToLower(args[0])
	if !contains(available, newLanguage) {
		for key, name := range languageNames {
			if strings.EqualFold(name, args[0]) {
				newLanguage = key
				break
			}
		}
	}
	if !contains(available, newLanguage) {
		keys := append([]string(nil), available...)
		sort.Strings(keys)
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

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
