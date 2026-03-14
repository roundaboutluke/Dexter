package command

import (
	"fmt"
	"regexp"
	"strings"

	"poraclego/internal/community"
)

// ChannelCommand registers/removes channels/webhooks (admin only).
type ChannelCommand struct{}

func (c *ChannelCommand) Name() string { return "channel" }

func channelLanguageLabel(languageNames map[string]string, language string) string {
	label := strings.TrimSpace(languageNames[language])
	if label == "" {
		return language
	}
	return label
}

func (c *ChannelCommand) Handle(ctx *Context, args []string) (string, error) {
	tr := ctx.I18n.Translator(ctx.Language)
	if ctx.Platform != "telegram" && !ctx.IsAdmin {
		return "🙅", nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		return tr.TranslateFormat("Valid commands are `{0}channel add`, `{0}channel remove`, `{0}channel add name:<webhook> <url>`", ctx.Prefix), nil
	}

	command := strings.ToLower(args[0])
	args = args[1:]
	webhookName := ""
	webhookURL := ""
	channelName := ""
	channelID := ""
	areaName := ""
	language := ""

	communityList := []string{}
	var areaRestriction []string
	fullAdmin := ctx.IsAdmin
	areaNames := map[string]string{}
	if ctx.Fences != nil {
		for _, fence := range ctx.Fences.Fences {
			name := strings.ToLower(strings.ReplaceAll(fence.Name, "_", " "))
			areaNames[name] = fence.Name
		}
	}
	languageNames := map[string]string{}
	if rawNames, ok := ctx.Data.UtilData["languageNames"].(map[string]any); ok {
		for key, value := range rawNames {
			languageNames[key] = fmt.Sprintf("%v", value)
		}
	}
	availableLanguages := []string{}
	if ctx.I18n != nil {
		availableLanguages = ctx.I18n.EffectiveLanguages()
	}

	for _, arg := range args {
		if re.Area.MatchString(arg) {
			match := re.Area.FindStringSubmatch(arg)
			if len(match) > 2 {
				areaCandidate := strings.ToLower(strings.ReplaceAll(match[2], "_", " "))
				if valid, ok := areaNames[areaCandidate]; ok {
					areaName = valid
				}
			}
			continue
		}
		if re.Language.MatchString(arg) {
			match := re.Language.FindStringSubmatch(arg)
			if len(match) > 2 {
				name := ctx.I18n.ReverseTranslateCommand(match[2], true)
				langKey := name
				for key, label := range languageNames {
					if strings.EqualFold(label, name) {
						langKey = key
						break
					}
				}
				langKey = strings.ToLower(strings.TrimSpace(langKey))
				if contains(availableLanguages, langKey) {
					language = langKey
				}
			}
			continue
		}
		if re.Name.MatchString(arg) {
			match := re.Name.FindStringSubmatch(arg)
			if len(match) > 2 {
				webhookName = match[2]
				channelName = match[2]
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(arg), "http") {
			webhookURL = arg
			continue
		}
		if channelID == "" && isNumericID(arg) {
			channelID = arg
		}
	}

	if command == "add" {
		if ctx.Platform == "telegram" {
			if (channelName != "" && channelID == "") || (channelName == "" && channelID != "") {
				return "To add a channel, provide both a name and an channel id", nil
			}
		}
		if ctx.Platform == "discord" {
			if webhookName != "" && webhookURL == "" || webhookName == "" && webhookURL != "" {
				return "To add webhooks, provide both a name using the `name` parameter and an url", nil
			}
			if webhookName != "" && webhookURL != "" {
				count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": webhookURL})
				if count > 0 {
					return "👌", nil
				}
				areaPayload := toJSON([]string{strings.ToLower(areaName)})
				if _, err := ctx.Query.InsertQuery("humans", map[string]any{
					"id":                   webhookURL,
					"type":                 "webhook",
					"name":                 webhookName,
					"area":                 areaPayload,
					"language":             nullableString(language),
					"community_membership": "[]",
				}); err != nil {
					return "", err
				}
				ctx.MarkAlertStateDirty()
				reply := tr.Translate("Webhook added", false)
				if areaName != "" {
					reply = fmt.Sprintf("%s %s %s %s", reply, tr.Translate("with", false), tr.Translate("area", false), areaName)
				}
				if language != "" {
					join := tr.Translate("with", false)
					if areaName != "" {
						join = tr.Translate("and", false)
					}
					reply = fmt.Sprintf("%s %s %s %s", reply, join, tr.Translate("language", false), tr.Translate(channelLanguageLabel(languageNames, language), false))
				}
				return reply, nil
			}
			if ctx.IsDM {
				return "Adding a bot controlled channel cannot be done from DM. To add webhooks, provide both a name using the `name` parameter and an url", nil
			}
			count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": ctx.ChannelID})
			if count > 0 {
				return "👌", nil
			}
			areaPayload := toJSON([]string{strings.ToLower(areaName)})
			if _, err := ctx.Query.InsertQuery("humans", map[string]any{
				"id":                   ctx.ChannelID,
				"type":                 "discord:channel",
				"name":                 ctx.ChannelName,
				"area":                 areaPayload,
				"language":             nullableString(language),
				"community_membership": "[]",
			}); err != nil {
				return "", err
			}
			ctx.MarkAlertStateDirty()
			reply := tr.Translate("Channel added", false)
			if areaName != "" {
				reply = fmt.Sprintf("%s %s %s %s", reply, tr.Translate("with", false), tr.Translate("area", false), areaName)
			}
			if language != "" {
				join := tr.Translate("with", false)
				if areaName != "" {
					join = tr.Translate("and", false)
				}
				reply = fmt.Sprintf("%s %s %s %s", reply, join, tr.Translate("language", false), tr.Translate(channelLanguageLabel(languageNames, language), false))
			}
			return reply, nil
		}
		if ctx.Platform == "telegram" {
			communityList = nil
			if enabled, _ := ctx.Config.GetBool("areaSecurity.enabled"); enabled {
				if list := community.IsTelegramCommunityAdmin(ctx.Config, ctx.UserID); len(list) > 0 {
					communityList = list
					areaRestriction = community.CalculateLocationRestrictions(ctx.Config, list)
				}
			}
			if ctx.IsAdmin {
				communityList = []string{}
				areaRestriction = nil
				fullAdmin = true
			}
			if communityList == nil && !fullAdmin {
				return "", nil
			}
		}
		if ctx.Platform == "telegram" {
			if channelName != "" && channelID != "" {
				if !fullAdmin {
					return "You are not a full poracle administrator", nil
				}
				count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": channelID})
				if count > 0 {
					return "👌", nil
				}
				targetType := "telegram:channel"
				if _, err := ctx.Query.InsertQuery("humans", map[string]any{
					"id":                   channelID,
					"type":                 targetType,
					"name":                 channelName,
					"area":                 "[]",
					"community_membership": toJSON(communityList),
					"area_restriction":     nullableString(toJSON(areaRestriction)),
				}); err != nil {
					return "", err
				}
				ctx.MarkAlertStateDirty()
				return "✅", nil
			}
			if ctx.IsDM {
				return "To add a group, please send /channel add in the group", nil
			}
			channelID = ctx.ChannelID
			channelName = strings.ToLower(ctx.ChannelName)
			count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": channelID})
			if count > 0 {
				return "👌", nil
			}
		}
		if ctx.IsDM {
			return "Adding a bot controlled channel cannot be done from DM.", nil
		}
		count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": ctx.ChannelID})
		if count > 0 {
			return "👌", nil
		}
		if _, err := ctx.Query.InsertQuery("humans", map[string]any{
			"id":                   ctx.ChannelID,
			"type":                 ctx.Platform + ":channel",
			"name":                 ctx.ChannelName,
			"area":                 "[]",
			"community_membership": "[]",
		}); err != nil {
			return "", err
		}
		ctx.MarkAlertStateDirty()
		return "Channel added.", nil
	}

	if command == "remove" {
		if ctx.Platform == "discord" {
			if webhookName != "" {
				count, _ := ctx.Query.CountQuery("humans", map[string]any{"name": webhookName, "type": "webhook"})
				if count == 0 {
					return "Webhook with that name does not appeared to be registered", nil
				}
				if _, err := ctx.Query.DeleteQuery("humans", map[string]any{"name": webhookName, "type": "webhook"}); err != nil {
					return "", err
				}
				ctx.MarkAlertStateDirty()
				return "✅", nil
			}
			if ctx.IsDM {
				return "Removing a bot controlled channel cannot be done from DM", nil
			}
			count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": ctx.ChannelID})
			if count == 0 {
				return fmt.Sprintf("%s %s %s%s %s", ctx.ChannelName, tr.Translate("does not seem to be registered. add it with", false), ctx.Prefix, tr.Translate("channel", true), tr.Translate("add", true)), nil
			}
			if _, err := ctx.Query.DeleteQuery("humans", map[string]any{"id": ctx.ChannelID}); err != nil {
				return "", err
			}
			ctx.MarkAlertStateDirty()
			return "✅", nil
		}
		if webhookName != "" {
			// Webhook removal is handled in the discord branch only.
			webhookName = ""
		}
		if ctx.Platform == "telegram" {
			if channelName != "" && channelID != "" {
				if !fullAdmin {
					return "You are not a full poracle administrator", nil
				}
				count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": channelID})
				if count == 0 {
					return fmt.Sprintf("%s %s %s%s %s", channelID, tr.Translate("does not seem to be registered. add it with", false), ctx.Prefix, tr.Translate("channel", true), tr.Translate("add", true)), nil
				}
				if _, err := ctx.Query.DeleteQuery("humans", map[string]any{"id": channelID}); err != nil {
					return "", err
				}
				ctx.MarkAlertStateDirty()
				return "✅", nil
			}
			if ctx.IsDM {
				return fmt.Sprintf("%s %s %s%s %s", ctx.ChannelID, tr.Translate("does not seem to be registered. add it with", false), ctx.Prefix, tr.Translate("channel", true), tr.Translate("add", true)), nil
			}
			channelID = ctx.ChannelID
			count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": channelID})
			if count == 0 {
				return fmt.Sprintf("%s %s %s%s %s", channelID, tr.Translate("does not seem to be registered. add it with", false), ctx.Prefix, tr.Translate("channel", true), tr.Translate("add", true)), nil
			}
			if _, err := ctx.Query.DeleteQuery("humans", map[string]any{"id": channelID}); err != nil {
				return "", err
			}
			ctx.MarkAlertStateDirty()
			return "✅", nil
		}
		if ctx.IsDM {
			return "Removing a bot controlled channel cannot be done from DM.", nil
		}
		if _, err := ctx.Query.DeleteQuery("humans", map[string]any{"id": ctx.ChannelID}); err != nil {
			return "", err
		}
		ctx.MarkAlertStateDirty()
		return "Channel removed.", nil
	}

	return "", nil
}

var numericIDRe = regexp.MustCompile(`^-?\d{1,20}$`)

func isNumericID(value string) bool {
	return numericIDRe.MatchString(value)
}
