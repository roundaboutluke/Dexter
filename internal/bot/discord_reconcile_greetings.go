package bot

import (
	"encoding/json"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/config"
	"poraclego/internal/dts"
	"poraclego/internal/logging"
	"poraclego/internal/render"
)

func (d *Discord) sendGreetingsDiscord(userID string) {
	disableGreetings, _ := d.manager.cfg.GetBool("discord.disableAutoGreetings")
	if disableGreetings || d.session == nil {
		return
	}

	// Match PoracleJS behavior: throttle greetings to avoid bans (10+ messages/minute).
	currentMinute := time.Now().Unix() / 60
	d.greetMu.Lock()
	if d.lastGreetingMinute == currentMinute {
		d.greetingCountMinute++
		if d.greetingCountMinute > 10 {
			d.greetMu.Unlock()
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Reconciliation (Discord) Did not send greeting to %s - attempting to avoid ban (%d messages in this minute)", userID, d.greetingCountMinute)
			}
			return
		}
	} else {
		d.greetingCountMinute = 0
		d.lastGreetingMinute = currentMinute
	}
	d.greetMu.Unlock()

	tpl := findGreetingTemplate(d.manager.templates, "discord")
	if tpl == nil {
		return
	}
	view := map[string]any{
		"prefix": getStringOr(d.manager.cfg, "discord.prefix", "!"),
	}
	rendered := renderTemplatePayload(tpl.Template, view)
	if rendered.Content == "" && len(rendered.Embeds) == 0 {
		return
	}
	ch, err := d.session.UserChannelCreate(userID)
	if err != nil {
		return
	}
	_, _ = d.session.ChannelMessageSendComplex(ch.ID, &rendered)
}

func renderTemplatePayload(template any, view map[string]any) discordgo.MessageSend {
	payload := map[string]any{}
	raw, err := json.Marshal(template)
	if err != nil {
		return discordgo.MessageSend{}
	}
	text, err := render.RenderHandlebars(string(raw), view, nil)
	if err != nil {
		return discordgo.MessageSend{}
	}
	_ = json.Unmarshal([]byte(text), &payload)

	msg := discordgo.MessageSend{}
	if content, ok := payload["content"].(string); ok {
		msg.Content = content
	}
	if embedRaw, ok := payload["embed"]; ok {
		if embed := decodeEmbed(embedRaw); embed != nil {
			msg.Embeds = []*discordgo.MessageEmbed{embed}
		}
	}
	if embedsRaw, ok := payload["embeds"]; ok {
		if embeds := decodeEmbeds(embedsRaw); len(embeds) > 0 {
			msg.Embeds = embeds
		}
	}
	return msg
}

func decodeEmbed(raw any) *discordgo.MessageEmbed {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var embed discordgo.MessageEmbed
	if err := json.Unmarshal(data, &embed); err != nil {
		return nil
	}
	return &embed
}

func decodeEmbeds(raw any) []*discordgo.MessageEmbed {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var embeds []*discordgo.MessageEmbed
	if err := json.Unmarshal(data, &embeds); err != nil {
		return nil
	}
	return embeds
}

func findGreetingTemplate(templates []dts.Template, platform string) *dts.Template {
	for _, tpl := range templates {
		if tpl.Type == "greeting" && tpl.Platform == platform && tpl.Default {
			return &tpl
		}
	}
	return nil
}

func getStringOr(cfg *config.Config, path, fallback string) string {
	if value, ok := cfg.GetString(path); ok && value != "" {
		return value
	}
	return fallback
}
