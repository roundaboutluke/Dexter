package command

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// WebhookCommand registers webhook names/urls (admin only).
type WebhookCommand struct{}

func (c *WebhookCommand) Name() string { return "webhook" }

func (c *WebhookCommand) Handle(ctx *Context, args []string) (string, error) {
	tr := ctx.I18n.Translator(ctx.Language)
	if !ctx.IsAdmin {
		return "🙅", nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		return tr.TranslateFormat("Valid commands are `{0}webhook add name:<webhook> <url>`, `{0}webhook remove name:<webhook>`", ctx.Prefix), nil
	}

	command := strings.ToLower(args[0])
	args = args[1:]
	webhookName := ""
	webhookURL := ""

	for _, arg := range args {
		if re.Name.MatchString(arg) {
			match := re.Name.FindStringSubmatch(arg)
			if len(match) > 2 {
				webhookName = match[2]
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(arg), "http") {
			webhookURL = arg
		}
	}

	if ctx.Platform == "discord" {
		return handleDiscordWebhook(ctx, command, webhookName)
	}

	switch command {
	case "add":
		if webhookName == "" || webhookURL == "" {
			return tr.Translate("To add webhooks, provide both a name using the `name` parameter and an url", false), nil
		}
		if _, err := ctx.Query.InsertQuery("humans", map[string]any{
			"id":                   webhookURL,
			"type":                 "webhook",
			"name":                 webhookName,
			"area":                 "[]",
			"community_membership": "[]",
		}); err != nil {
			return "", err
		}
		ctx.MarkAlertStateDirty()
		return tr.Translate("Webhook added.", false), nil
	case "remove":
		if webhookName == "" {
			return tr.Translate("Provide the webhook name to remove.", false), nil
		}
		if _, err := ctx.Query.DeleteQuery("humans", map[string]any{"name": webhookName, "type": "webhook"}); err != nil {
			return "", err
		}
		ctx.MarkAlertStateDirty()
		return tr.Translate("Webhook removed.", false), nil
	default:
		return "", nil
	}
}

func handleDiscordWebhook(ctx *Context, command string, webhookName string) (string, error) {
	tr := ctx.I18n.Translator(ctx.Language)
	if ctx.IsDM {
		return "This needs to be run from within a channel on the appropriate guild", nil
	}
	token := discordTokenForTarget(ctx, ctx.ChannelID)
	if token == "" {
		return "Discord token missing", nil
	}
	if command == "list" {
		hooks, err := listDiscordWebhooks(ctx.ChannelID, token)
		if err != nil {
			return "", err
		}
		lines := []string{}
		for _, hook := range hooks {
			lines = append(lines, fmt.Sprintf("%s | %s", hook.Name, hook.URL))
		}
		return strings.Join(lines, "\n"), nil
	}
	if webhookName == "" {
		webhookName = ctx.ChannelName
	}
	switch command {
	case "remove":
		count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": ctx.ChannelID})
		if count > 0 {
			return "This channel is already registered under bot control - `channel remove` first", nil
		}
		count, _ = ctx.Query.CountQuery("humans", map[string]any{"name": webhookName})
		if count > 0 {
			if _, err := ctx.Query.DeleteQuery("humans", map[string]any{"name": webhookName, "type": "webhook"}); err != nil {
				return "", err
			}
			ctx.MarkAlertStateDirty()
			return "✅", nil
		}
		return fmt.Sprintf("A webhook or channel with the name %s cannot be found", webhookName), nil
	case "create", "add":
		count, _ := ctx.Query.CountQuery("humans", map[string]any{"id": ctx.ChannelID})
		if count > 0 {
			return "This channel is already registered under bot control - `channel remove` first", nil
		}
		count, _ = ctx.Query.CountQuery("humans", map[string]any{"name": webhookName})
		if count > 0 {
			return fmt.Sprintf("A webhook or channel with the name %s already exists", webhookName), nil
		}
		if strings.Contains(webhookName, "_") {
			return "A poracle webhook name cannot contain an underscore (_) - use name parameter to override", nil
		}
		url, err := ensureDiscordWebhook(ctx.ChannelID, token)
		if err != nil {
			return "", err
		}
		if _, err := ctx.Query.InsertQuery("humans", map[string]any{
			"id":                   url,
			"type":                 "webhook",
			"name":                 webhookName,
			"area":                 "[]",
			"community_membership": "[]",
		}); err != nil {
			return "", err
		}
		ctx.MarkAlertStateDirty()
		return "✅", nil
	default:
		return tr.Translate("Unknown webhook command.", false), nil
	}
}

type discordWebhook struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

func listDiscordWebhooks(channelID, token string) ([]discordWebhook, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("https://discord.com/api/v10/channels/%s/webhooks", channelID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bot "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("discord http %d", resp.StatusCode)
	}
	var hooks []discordWebhook
	if err := json.NewDecoder(resp.Body).Decode(&hooks); err != nil {
		return nil, err
	}
	return hooks, nil
}

func ensureDiscordWebhook(channelID, token string) (string, error) {
	hooks, err := listDiscordWebhooks(channelID, token)
	if err != nil {
		return "", err
	}
	for _, hook := range hooks {
		if hook.Name == "Poracle" && hook.URL != "" {
			return hook.URL, nil
		}
	}
	payload, _ := json.Marshal(map[string]any{"name": "Poracle"})
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("https://discord.com/api/v10/channels/%s/webhooks", channelID), bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bot "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("discord http %d", resp.StatusCode)
	}
	var hook discordWebhook
	if err := json.NewDecoder(resp.Body).Decode(&hook); err != nil {
		return "", err
	}
	if hook.URL == "" {
		return "", fmt.Errorf("discord webhook url missing")
	}
	return hook.URL, nil
}

func discordTokenForTarget(ctx *Context, target string) string {
	if ctx == nil || ctx.Config == nil {
		return ""
	}
	values, ok := ctx.Config.GetStringSlice("discord.token")
	if ok && len(values) > 0 {
		if len(values) == 1 || target == "" {
			return strings.TrimSpace(values[0])
		}
		index := int(hashString(target)) % len(values)
		return strings.TrimSpace(values[index])
	}
	if value, ok := ctx.Config.GetString("discord.token"); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func hashString(value string) uint32 {
	const fnvOffset = 2166136261
	const fnvPrime = 16777619
	hash := uint32(fnvOffset)
	for i := 0; i < len(value); i++ {
		hash ^= uint32(value[i])
		hash *= fnvPrime
	}
	return hash
}
