package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"poraclego/internal/dts"
	"poraclego/internal/render"
)

// HelpCommand renders help or greeting DTS templates.
type HelpCommand struct{}

func (c *HelpCommand) Name() string {
	return "help"
}

func (c *HelpCommand) Handle(ctx *Context, args []string) (string, error) {
	if ctx == nil {
		return "", nil
	}
	result := buildTarget(ctx, args)
	if !result.CanContinue {
		return result.Message, nil
	}
	language := result.Language
	if ctx.Query != nil && result.TargetID != "" {
		if row, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": result.TargetID}); err == nil && row != nil {
			if lang, ok := row["language"].(string); ok && lang == "" {
				_, _ = ctx.Query.UpdateQuery("humans", map[string]any{"language": language}, map[string]any{"id": result.TargetID})
			}
		}
	}

	platform := resolveHelpPlatform(result.Target, ctx.Platform)
	templateID := ""
	if len(args) > 0 {
		templateID = args[0]
	}
	var tpl *dts.Template
	if templateID != "" {
		tpl = findHelpTemplate(ctx.Templates, platform, language, templateID)
	} else {
		tpl = findGreetingTemplate(ctx.Templates, platform, language)
	}
	if tpl == nil {
		return "🙅", nil
	}
	payload := normalizeHelpTemplate(tpl.Template)
	if payload == nil {
		return "🙅", nil
	}
	view := map[string]any{
		"prefix": ctx.Prefix,
	}
	meta := map[string]any{
		"language": language,
		"platform": platform,
	}
	if platform == "telegram" {
		messages := renderTelegramHelpMessages(payload, view, meta)
		if len(messages) == 0 {
			return "🙅", nil
		}
		if len(messages) == 1 {
			return messages[0], nil
		}
		raw, _ := json.Marshal(messages)
		return TelegramMultiPrefix + string(raw), nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "🙅", nil
	}
	rendered, err := render.RenderHandlebars(string(raw), view, meta)
	if err != nil || strings.TrimSpace(rendered) == "" {
		return "🙅", nil
	}
	return DiscordEmbedPrefix + rendered, nil
}

func resolveHelpPlatform(target Target, fallback string) string {
	t := strings.ToLower(target.Type)
	switch {
	case t == "webhook" || strings.HasPrefix(t, "discord"):
		return "discord"
	case strings.HasPrefix(t, "telegram"):
		return "telegram"
	}
	if fallback != "" {
		return fallback
	}
	return "discord"
}

func findHelpTemplate(templates []dts.Template, platform, language, templateID string) *dts.Template {
	if templateID == "" {
		return nil
	}
	if tpl := matchTemplate(templates, "help", platform, language, templateID, false); tpl != nil {
		return tpl
	}
	if tpl := matchTemplate(templates, "help", "", language, templateID, false); tpl != nil {
		return tpl
	}
	if tpl := matchTemplate(templates, "help", platform, "", templateID, true); tpl != nil {
		return tpl
	}
	return matchTemplate(templates, "help", "", "", templateID, true)
}

func findGreetingTemplate(templates []dts.Template, platform, language string) *dts.Template {
	if tpl := matchTemplate(templates, "greeting", platform, language, "", false); tpl != nil {
		return tpl
	}
	if tpl := matchTemplate(templates, "greeting", platform, "", "", true); tpl != nil {
		return tpl
	}
	if tpl := matchTemplate(templates, "greeting", "", language, "", false); tpl != nil {
		return tpl
	}
	return matchTemplate(templates, "greeting", "", "", "", true)
}

func matchTemplate(templates []dts.Template, templateType, platform, language, templateID string, requireDefault bool) *dts.Template {
	for _, tpl := range templates {
		if tpl.Type != templateType {
			continue
		}
		if platform != "" && tpl.Platform != platform {
			continue
		}
		if requireDefault && !tpl.Default {
			continue
		}
		if templateID != "" && fmt.Sprintf("%v", tpl.ID) != templateID {
			continue
		}
		if language != "" && !languageMatches(tpl.Language, language) {
			continue
		}
		return &tpl
	}
	return nil
}

func languageMatches(lang *string, expected string) bool {
	if lang == nil {
		return false
	}
	return *lang == expected
}

func normalizeHelpTemplate(template any) map[string]any {
	raw, err := json.Marshal(template)
	if err != nil {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	if embed, ok := payload["embed"].(map[string]any); ok {
		if _, ok := embed["title"]; ok {
			embed["title"] = ""
		}
		if _, ok := embed["description"]; ok {
			embed["description"] = ""
		}
	}
	return payload
}

func renderTelegramHelpMessages(template map[string]any, view, meta map[string]any) []string {
	raw, err := json.Marshal(template)
	if err != nil {
		return nil
	}
	rendered, err := render.RenderHandlebars(string(raw), view, meta)
	if err != nil {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(rendered), &payload); err != nil {
		return nil
	}
	embed, ok := payload["embed"].(map[string]any)
	if !ok {
		return nil
	}
	fields, ok := embed["fields"].([]any)
	if !ok {
		return nil
	}
	var out []string
	var buf strings.Builder
	for _, field := range fields {
		entry, ok := field.(map[string]any)
		if !ok {
			continue
		}
		name := fmt.Sprintf("%v", entry["name"])
		value := fmt.Sprintf("%v", entry["value"])
		chunk := fmt.Sprintf("\n\n%s\n\n%s", name, value)
		if buf.Len()+len(chunk) > 1024 {
			out = append(out, buf.String())
			buf.Reset()
		}
		buf.WriteString(chunk)
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

func singleLineHelpText(ctx *Context, commandName, language string, target Target) string {
	if ctx == nil || ctx.I18n == nil {
		return ""
	}
	tr := ctx.I18n.Translator(language)
	platform := resolveHelpPlatform(target, ctx.Platform)
	if findHelpTemplate(ctx.Templates, platform, language, commandName) != nil {
		return tr.TranslateFormat("Use `{0}{1} {2}` for more details on this command", ctx.Prefix, tr.Translate("help", true), tr.Translate(commandName, true))
	}
	return tr.TranslateFormat("Use `{0}{1}` for more help", ctx.Prefix, tr.Translate("help", true))
}
