package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"poraclego/internal/alertstate"
	"poraclego/internal/community"
	"poraclego/internal/db"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/render"
)

// StartCommand enables alerts.
type StartCommand struct{}

func (c *StartCommand) Name() string { return "start" }

func (c *StartCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "start") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	_, err := ctx.Query.UpdateQuery("humans", map[string]any{"enabled": 1}, map[string]any{"id": result.TargetID})
	if err != nil {
		return "", err
	}
	ctx.MarkAlertStateDirty()
	return tr.TranslateFormat("Your tracking is now activated, send {0}{1} for more information about available commands", ctx.Prefix, tr.Translate("help", true)), nil
}

// StopCommand disables alerts.
type StopCommand struct{}

func (c *StopCommand) Name() string { return "stop" }

func (c *StopCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "stop") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	if len(args) > 0 {
		target := result.Target
		if target.Type == "" {
			target = Target{ID: ctx.UserID, Name: ctx.UserName, Type: ctx.Platform + ":user"}
		}
		warn := tr.TranslateFormat("The {0}{1} command is used to stop all alert messages, is that what you want?", ctx.Prefix, tr.Translate("stop", true))
		helpLine := singleLineHelpText(ctx, "stop", result.Language, target)
		if helpLine != "" {
			return warn + "\n" + helpLine, nil
		}
		return warn, nil
	}
	_, err := ctx.Query.UpdateQuery("humans", map[string]any{"enabled": 0}, map[string]any{"id": result.TargetID})
	if err != nil {
		return "", err
	}
	ctx.MarkAlertStateDirty()
	return tr.TranslateFormat("All alert messages have been stopped, you can resume them with {0}{1}", ctx.Prefix, tr.Translate("start", true)), nil
}

// PoracleCommand registers a user.
type PoracleCommand struct{}

func (c *PoracleCommand) Name() string { return "poracle" }

func (c *PoracleCommand) Handle(ctx *Context, _ []string) (string, error) {
	if ctx == nil {
		return "", nil
	}
	if ctx.Platform == "telegram" && ctx.IsDM {
		return "", nil
	}

	communityToAdd := ""
	areaSecurity, _ := ctx.Config.GetBool("areaSecurity.enabled")
	if areaSecurity {
		raw, ok := ctx.Config.Get("areaSecurity.communities")
		if ok {
			if communities, ok := raw.(map[string]any); ok {
				for key, entry := range communities {
					entryMap, ok := entry.(map[string]any)
					if !ok {
						continue
					}
					switch ctx.Platform {
					case "discord":
						if discordEntry, ok := entryMap["discord"].(map[string]any); ok {
							if channels, ok := discordEntry["channels"].([]any); ok {
								for _, c := range channels {
									if fmt.Sprintf("%v", c) == ctx.ChannelID {
										communityToAdd = key
										break
									}
								}
							}
						}
					case "telegram":
						if telegramEntry, ok := entryMap["telegram"].(map[string]any); ok {
							if channels, ok := telegramEntry["channels"].([]any); ok {
								for _, c := range channels {
									if fmt.Sprintf("%v", c) == ctx.ChannelID {
										communityToAdd = key
										break
									}
								}
							}
						}
					}
					if communityToAdd != "" {
						break
					}
				}
			}
		}
		if communityToAdd == "" {
			return "", nil
		}
	} else {
		var allowed []string
		switch ctx.Platform {
		case "discord":
			allowed, _ = ctx.Config.GetStringSlice("discord.channels")
		case "telegram":
			allowed, _ = ctx.Config.GetStringSlice("telegram.channels")
		}
		if len(allowed) > 0 && !containsString(allowed, ctx.ChannelID) {
			return "", nil
		}
	}

	user, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": ctx.UserID})
	if err != nil {
		return "", err
	}

	if user != nil {
		if toInt(user["admin_disable"], 0) == 1 && user["disabled_date"] == nil {
			return "🙅", nil
		}
		update := map[string]any{}
		updateRequired := false
		if toInt(user["enabled"], 1) == 0 {
			update["enabled"] = 1
			updateRequired = true
		}
		roleCheckMode, _ := ctx.Config.GetString("general.roleCheckMode")
		if roleCheckMode == "disable-user" {
			if toInt(user["admin_disable"], 0) == 1 && user["disabled_date"] != nil {
				update["admin_disable"] = 0
				update["disabled_date"] = nil
				updateRequired = true
			}
		}
		if communityToAdd != "" {
			existing := []string{}
			if raw := stringValue(user["community_membership"]); raw != "" {
				_ = json.Unmarshal([]byte(raw), &existing)
			}
			communities := community.AddCommunity(ctx.Config, existing, communityToAdd)
			update["community_membership"] = toJSON(communities)
			update["area_restriction"] = nullableString(toJSON(community.CalculateLocationRestrictions(ctx.Config, communities)))
			updateRequired = true
		}
		if updateRequired {
			if _, err := ctx.Query.UpdateQuery("humans", update, map[string]any{"id": ctx.UserID}); err != nil {
				return "", err
			}
			ctx.MarkAlertStateDirty()
			c.enqueueGreetings(ctx, ctx.UserID, ctx.Language)
			if ctx.Platform == "telegram" {
				c.enqueueTelegramWelcome(ctx)
			}
			return "✅", nil
		}
		return "👌", nil
	}

	communityMembership := "[]"
	areaRestriction := ""
	if communityToAdd != "" {
		communities := community.AddCommunity(ctx.Config, []string{}, communityToAdd)
		communityMembership = toJSON(communities)
		areaRestriction = toJSON(community.CalculateLocationRestrictions(ctx.Config, communities))
	}
	values := map[string]any{
		"id":                   ctx.UserID,
		"type":                 ctx.Platform + ":user",
		"name":                 ctx.UserName,
		"enabled":              1,
		"language":             ctx.Language,
		"area":                 "[]",
		"community_membership": communityMembership,
		"area_restriction":     nullableString(areaRestriction),
	}
	if _, err := ctx.Query.InsertQuery("humans", values); err != nil {
		return "", err
	}
	ctx.MarkAlertStateDirty()
	c.enqueueGreetings(ctx, ctx.UserID, ctx.Language)
	if ctx.Platform == "telegram" {
		c.enqueueTelegramWelcome(ctx)
	}
	return "✅", nil
}

func (c *PoracleCommand) enqueueGreetings(ctx *Context, userID string, language string) {
	if ctx == nil {
		return
	}
	switch ctx.Platform {
	case "discord":
		template := greetingTemplate(ctx.Templates, "discord", language)
		if template == nil || ctx.DiscordQueue == nil {
			return
		}
		prefix, _ := ctx.Config.GetString("discord.prefix")
		payload := renderTemplatePayload(template.Template, map[string]any{"prefix": prefix})
		if payload == nil {
			return
		}
		ctx.DiscordQueue.Push(dispatch.MessageJob{
			Type:    "discord:user",
			Target:  userID,
			Message: "",
			Payload: payload,
		})
	case "telegram":
		template := greetingTemplate(ctx.Templates, "telegram", language)
		if template == nil || ctx.TelegramQueue == nil {
			return
		}
		messages := renderTelegramGreeting(template.Template, map[string]any{"prefix": "/"})
		for _, message := range messages {
			ctx.TelegramQueue.Push(dispatch.MessageJob{
				Type:    "telegram:user",
				Target:  userID,
				Message: message,
			})
		}
	}
}

func (c *PoracleCommand) enqueueTelegramWelcome(ctx *Context) {
	if ctx == nil || ctx.TelegramQueue == nil {
		return
	}
	if msg, ok := ctx.Config.GetString("telegram.botWelcomeText"); ok && msg != "" {
		ctx.TelegramQueue.Push(dispatch.MessageJob{
			Type:    "telegram:user",
			Target:  ctx.UserID,
			Message: msg,
		})
	}
	if !ctx.IsDM {
		if msg, ok := ctx.Config.GetString("telegram.groupWelcomeText"); ok && msg != "" {
			msg = strings.ReplaceAll(msg, "{user}", ctx.UserName)
			ctx.TelegramQueue.Push(dispatch.MessageJob{
				Type:    "telegram:group",
				Target:  ctx.ChannelID,
				Message: msg,
			})
		}
	}
}

func greetingTemplate(templates []dts.Template, platform, language string) *dts.Template {
	for _, tpl := range templates {
		if tpl.Type != "greeting" || tpl.Platform != platform {
			continue
		}
		if language != "" && tpl.Language != nil && *tpl.Language == language {
			return &tpl
		}
	}
	for _, tpl := range templates {
		if tpl.Type == "greeting" && tpl.Platform == platform && tpl.Default {
			return &tpl
		}
	}
	return nil
}

func renderTemplatePayload(template any, view map[string]any) map[string]any {
	raw, err := json.Marshal(template)
	if err != nil {
		return nil
	}
	text, err := render.RenderHandlebars(string(raw), view, nil)
	if err != nil {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		return nil
	}
	if embed, ok := payload["embed"]; ok {
		if _, hasEmbeds := payload["embeds"]; !hasEmbeds {
			payload["embeds"] = []any{embed}
			delete(payload, "embed")
		}
	}
	return payload
}

func renderTelegramGreeting(template any, view map[string]any) []string {
	raw, err := json.Marshal(template)
	if err != nil {
		return nil
	}
	text, err := render.RenderHandlebars(string(raw), view, nil)
	if err != nil {
		return nil
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
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
		name := stringValue(entry["name"])
		value := stringValue(entry["value"])
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

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

// UnregisterCommand removes a user.
type UnregisterCommand struct{}

func (c *UnregisterCommand) Name() string { return "unregister" }

func (c *UnregisterCommand) Handle(ctx *Context, args []string) (string, error) {
	targets := []string{ctx.UserID}
	if ctx.IsAdmin {
		targets = parseTargetIDs(ctx, args, false)
		if len(targets) == 0 {
			return "No-one to unregister: as an admin I won't let you unregister yourself", nil
		}
	}
	removed := false
	if err := withAlertStateTx(ctx, func(query *db.Query) error {
		for _, t := range targets {
			if count, err := query.CountQuery("humans", map[string]any{"id": t}); err == nil && count > 0 {
				removed = true
			}
			for _, table := range alertstate.TrackedTables() {
				if _, err := query.DeleteQuery(table, map[string]any{"id": t}); err != nil {
					return err
				}
			}
			if _, err := query.DeleteQuery("humans", map[string]any{"id": t}); err != nil {
				return err
			}
			if _, err := query.DeleteQuery("profiles", map[string]any{"id": t}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return "", err
	}
	if removed {
		ctx.MarkAlertStateDirty()
	}
	if removed {
		return "✅", nil
	}
	return "👌", nil
}
