package command

import (
	"fmt"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
)

func loadTargetRow(ctx *Context, t Target) (map[string]any, int, string, error) {
	var (
		row map[string]any
		err error
	)
	if t.Type == "webhook" {
		row, err = ctx.Query.SelectOneQuery("humans", map[string]any{
			"name": t.Name,
			"type": "webhook",
		})
	} else {
		row, err = ctx.Query.SelectOneQuery("humans", map[string]any{"id": t.ID})
	}
	if err != nil || row == nil {
		return row, 1, t.ID, err
	}
	currentProfileNo := toInt(row["current_profile_no"], 1)
	profileNo := resolveCommandProfileNo(ctx, row, currentProfileNo)
	targetID := t.ID
	if idVal, ok := row["id"]; ok {
		targetID = fmt.Sprintf("%v", idVal)
	}
	if t.Name == "" {
		if name, ok := row["name"].(string); ok {
			t.Name = name
		}
	}
	return row, profileNo, targetID, nil
}

func resolveCommandProfileNo(ctx *Context, row map[string]any, currentProfileNo int) int {
	if ctx != nil && ctx.ProfileOverride > 0 {
		return ctx.ProfileOverride
	}
	if currentProfileNo > 0 {
		return currentProfileNo
	}
	if preferred := toInt(row["preferred_profile_no"], 0); preferred > 0 {
		return preferred
	}
	return 1
}

func utilTypeKeys(ctx *Context) map[string]bool {
	out := map[string]bool{}
	raw, ok := ctx.Data.UtilData["types"]
	if !ok {
		return out
	}
	switch v := raw.(type) {
	case map[string]any:
		for key := range v {
			out[strings.ToLower(key)] = true
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				out[strings.ToLower(s)] = true
			}
		}
	}
	return out
}

type TargetResult struct {
	CanContinue     bool
	Target          Target
	Language        string
	ProfileNo       int
	UserHasLocation bool
	UserHasArea     bool
	TargetIsAdmin   bool
	IsRegistered    bool
	TargetID        string
	Message         string
}

func buildTarget(ctx *Context, args []string) TargetResult {
	if ctx.TargetOverride != nil {
		tgt := *ctx.TargetOverride
		row, profileNo, targetID, err := loadTargetRow(ctx, tgt)
		if err != nil {
			return TargetResult{CanContinue: false, Message: err.Error()}
		}
		tr := ctx.I18n.Translator(ctx.Language)
		if row != nil {
			lang := ctx.Language
			if l, ok := row["language"].(string); ok && l != "" {
				lang = l
			}
			area := ""
			if a, ok := row["area"].(string); ok {
				area = a
			}
			hasLocation := toFloat(row["latitude"]) != 0 && toFloat(row["longitude"]) != 0
			hasArea := len(area) > 2
			return TargetResult{
				CanContinue:     true,
				Target:          tgt,
				Language:        lang,
				ProfileNo:       profileNo,
				UserHasLocation: hasLocation,
				UserHasArea:     hasArea,
				TargetIsAdmin:   strings.Contains(tgt.Type, "discord") && containsStringSlice(ctx.Config, "discord.admins", targetID),
				IsRegistered:    true,
				TargetID:        targetID,
				Message:         tr.Translate("OK", false),
			}
		}
		return TargetResult{
			CanContinue:  false,
			Target:       tgt,
			Language:     ctx.Language,
			ProfileNo:    1,
			IsRegistered: false,
			TargetID:     targetID,
			Message:      unregisteredMessage(ctx, tr),
		}
	}

	re := NewRegexSet(ctx.I18n)
	tgt := Target{ID: ctx.UserID, Name: ctx.UserName, Type: ctx.Platform + ":user"}

	if !ctx.IsDM && !ctx.IsAdmin {
		if !delegatedChannelAllowed(ctx) {
			return TargetResult{CanContinue: false, Message: "Please run commands in Direct Messages"}
		}
	}

	if !ctx.IsDM && ctx.IsAdmin {
		if ctx.Platform == "discord" {
			tgt = Target{
				ID:   ctx.ChannelID,
				Name: ctx.ChannelName,
				Type: "discord:channel",
			}
		} else if ctx.Platform == "telegram" {
			tgt = Target{
				ID:   ctx.ChannelID,
				Name: ctx.ChannelName,
				Type: "telegram:group",
			}
		}
	}

	if ctx.IsAdmin {
		for _, arg := range args {
			if re.User.MatchString(arg) {
				match := re.User.FindStringSubmatch(arg)
				if len(match) > 2 {
					tgt.ID = match[2]
					tgt.Type = ctx.Platform + ":user"
					tgt.Name = match[2]
				}
			}
			if re.Name.MatchString(arg) {
				match := re.Name.FindStringSubmatch(arg)
				if len(match) > 2 {
					if ctx.Platform == "discord" {
						tgt.Type = "webhook"
					} else if ctx.Platform == "telegram" {
						tgt.Type = "telegram:channel"
					}
					tgt.Name = match[2]
				}
			}
		}
	} else if delegatedWebhookAllowed(ctx) {
		for _, arg := range args {
			if re.Name.MatchString(arg) {
				match := re.Name.FindStringSubmatch(arg)
				if len(match) > 2 {
					if ctx.Platform == "discord" {
						tgt.Type = "webhook"
					} else if ctx.Platform == "telegram" {
						tgt.Type = "telegram:channel"
					}
					tgt.Name = match[2]
				}
			}
		}
	}

	row, profileNo, targetID, err := loadTargetRow(ctx, tgt)
	if err != nil {
		return TargetResult{CanContinue: false, Message: err.Error()}
	}
	tr := ctx.I18n.Translator(ctx.Language)
	if row == nil {
		if tgt.Type == "webhook" {
			return TargetResult{CanContinue: false, Message: fmt.Sprintf("Webhook %s does not seem to be registered. add it with %swebhook add <Your-Webhook-url>", tgt.Name, ctx.Prefix)}
		}
		if tgt.Type == "telegram:channel" {
			return TargetResult{CanContinue: false, Message: fmt.Sprintf("Channel %s does not seem to be registered. add it with %schannel add name <Your-Channel-id>", tgt.Name, ctx.Prefix)}
		}
		if strings.Contains(tgt.Type, ":channel") || tgt.Type == "telegram:group" {
			return TargetResult{CanContinue: false, Message: fmt.Sprintf("%s does not seem to be registered. add it with %schannel add", tgt.Name, ctx.Prefix)}
		}
		if ctx.IsDM {
			return TargetResult{CanContinue: false, Message: unregisteredMessage(ctx, tr)}
		}
		return TargetResult{
			CanContinue:  true,
			Target:       tgt,
			Language:     ctx.Language,
			ProfileNo:    1,
			IsRegistered: false,
			TargetID:     targetID,
			Message:      tr.Translate("OK", false),
		}
	}

	if row != nil {
		lang := ctx.Language
		if l, ok := row["language"].(string); ok && l != "" {
			lang = l
		}
		area := ""
		if a, ok := row["area"].(string); ok {
			area = a
		}
		hasLocation := toFloat(row["latitude"]) != 0 && toFloat(row["longitude"]) != 0
		hasArea := len(area) > 2
		return TargetResult{
			CanContinue:     true,
			Target:          tgt,
			Language:        lang,
			ProfileNo:       profileNo,
			UserHasLocation: hasLocation,
			UserHasArea:     hasArea,
			TargetIsAdmin:   strings.Contains(tgt.Type, "discord") && containsStringSlice(ctx.Config, "discord.admins", targetID),
			IsRegistered:    true,
			TargetID:        targetID,
			Message:         tr.Translate("OK", false),
		}
	}

	return TargetResult{
		CanContinue:  false,
		Target:       tgt,
		Language:     ctx.Language,
		ProfileNo:    1,
		IsRegistered: false,
		TargetID:     targetID,
		Message:      unregisteredMessage(ctx, tr),
	}
}

func unregisteredMessage(ctx *Context, tr *i18n.Translator) string {
	if ctx == nil || ctx.Config == nil {
		return "You are not registered."
	}
	path := ""
	switch ctx.Platform {
	case "discord":
		path = "discord.unregisteredUserMessage"
	case "telegram":
		path = "telegram.unregisteredUserMessage"
	}
	if path != "" {
		if msg, ok := ctx.Config.GetString(path); ok && msg != "" {
			return msg
		}
	}
	return "¯\\_(ツ)_/¯"
}

func containsStringSlice(cfg *config.Config, path string, target string) bool {
	values, _ := cfg.GetStringSlice(path)
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func delegatedChannelAllowed(ctx *Context) bool {
	raw, ok := ctx.Config.Get(ctx.Platform + ".delegatedAdministration.channelTracking")
	if !ok {
		return false
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	for key, value := range entries {
		if key != ctx.ChannelID {
			continue
		}
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				if fmt.Sprintf("%v", item) == ctx.UserID {
					return true
				}
			}
		case []string:
			for _, item := range v {
				if item == ctx.UserID {
					return true
				}
			}
		}
	}
	return false
}

func delegatedWebhookAllowed(ctx *Context) bool {
	raw, ok := ctx.Config.Get(ctx.Platform + ".delegatedAdministration.webhookTracking")
	if !ok {
		return false
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	for _, value := range entries {
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				if fmt.Sprintf("%v", item) == ctx.UserID {
					return true
				}
			}
		case []string:
			for _, item := range v {
				if item == ctx.UserID {
					return true
				}
			}
		}
	}
	return false
}
