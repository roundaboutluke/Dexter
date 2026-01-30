package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/command"
	"poraclego/internal/config"
	"poraclego/internal/logging"
)

// Discord bot wrapper.
type Discord struct {
	manager *Manager
	token   string
	session *discordgo.Session
	slashMu sync.Mutex
	slash   map[string]*slashBuilderState

	greetMu             sync.Mutex
	lastGreetingMinute  int64
	greetingCountMinute int
}

// NewDiscord constructs a Discord bot.
func NewDiscord(manager *Manager, token string) *Discord {
	return &Discord{
		manager: manager,
		token:   token,
		slash:   map[string]*slashBuilderState{},
	}
}

// Start connects and listens for messages.
func (d *Discord) Start() error {
	session, err := discordgo.New("Bot " + d.token)
	if err != nil {
		return err
	}
	session.Identify.Intents = discordIntents()
	session.Identify.Presence = discordPresence(d.manager.cfg)
	session.AddHandler(d.onMessage)
	session.AddHandler(d.onInteractionCreate)
	session.AddHandler(d.onChannelDelete)
	session.AddHandler(d.onGuildMemberRemove)
	session.AddHandler(d.onGuildMemberUpdate)
	session.AddHandler(d.onReady)
	if err := session.Open(); err != nil {
		return err
	}
	d.session = session
	d.startReconciliation(session)
	return nil
}

func discordIntents() discordgo.Intent {
	return discordgo.IntentsGuilds |
		discordgo.IntentsGuildMembers |
		discordgo.IntentsGuildMessages |
		discordgo.IntentsGuildPresences |
		discordgo.IntentsDirectMessages |
		discordgo.IntentsMessageContent
}

func discordPresence(cfg *config.Config) discordgo.GatewayStatusUpdate {
	status := "online"
	if cfg != nil {
		if raw, ok := cfg.GetString("discord.workerStatus"); ok && raw != "" {
			status = strings.ToLower(raw)
		} else if raw, ok := cfg.GetString("discord.status"); ok && raw != "" {
			status = strings.ToLower(raw)
		}
	}
	switch status {
	case "available":
		status = "online"
	case "dnd", "idle", "invisible", "online":
	default:
		status = "online"
	}

	activity := ""
	if cfg != nil {
		if raw, ok := cfg.GetString("discord.workerActivity"); ok && raw != "" {
			activity = raw
		} else if raw, ok := cfg.GetString("discord.activity"); ok && raw != "" {
			activity = raw
		}
	}

	update := discordgo.GatewayStatusUpdate{
		Status: status,
		AFK:    false,
	}
	if activity != "" {
		update.Game = discordgo.Activity{
			Name: activity,
			Type: discordgo.ActivityTypeGame,
		}
	}
	return update
}

func (d *Discord) onReady(s *discordgo.Session, r *discordgo.Ready) {
	logger := logging.Get().Discord
	checkRole, _ := d.manager.cfg.GetBool("discord.checkRole")
	interval, _ := d.manager.cfg.GetInt("discord.checkRoleInterval")
	readyMsg := fmt.Sprintf("Discord ready: %s#%s (%s)", r.User.Username, r.User.Discriminator, r.User.ID)
	if logger != nil {
		logger.Infof("%s", readyMsg)
	} else if general := logging.Get().General; general != nil {
		general.Infof("%s", readyMsg)
	}
	if checkRole && interval > 0 {
		d.runReconciliation(s)
	}
	presence := discordPresence(d.manager.cfg)
	statusData := discordgo.UpdateStatusData{
		Status: presence.Status,
		AFK:    presence.AFK,
	}
	if presence.Game.Name != "" {
		statusData.Activities = []*discordgo.Activity{
			{
				Name: presence.Game.Name,
				Type: presence.Game.Type,
			},
		}
	}
	if err := s.UpdateStatusComplex(statusData); err != nil {
		if logger != nil {
			logger.Warnf("Discord presence update failed: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "Discord presence update failed: %v\n", err)
		}
	}
	d.registerSlashCommands(s)
}

func (d *Discord) onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author == nil {
		return
	}
	if m.WebhookID != "" {
		return
	}
	prefix, _ := d.manager.cfg.GetString("discord.prefix")
	if prefix == "" {
		prefix = "!"
	}
	isAdmin := containsID(d.manager.cfg, "discord.admins", m.Author.ID)
	if m.Author.Bot && !isAdmin {
		return
	}
	if !strings.HasPrefix(m.Content, prefix) {
		return
	}

	channelName := ""
	isDM := m.GuildID == ""
	channel, err := s.Channel(m.ChannelID)
	if err == nil && channel != nil {
		channelName = channel.Name
		if channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM {
			isDM = true
		}
	}
	if isDM {
		d.logDM(s, m)
	}
	roles := []string{}
	if isDM {
		roles = d.rolesForDM(s, m.Author.ID)
	} else if channel != nil && channel.GuildID != "" {
		if member, err := s.GuildMember(channel.GuildID, m.Author.ID); err == nil && member != nil {
			roles = append(roles, member.Roles...)
		}
	}
	ctx := d.manager.Context("discord", "", prefix, m.Author.ID, m.Author.Username, m.ChannelID, channelName, isDM, isAdmin, roles, ".")
	ctx.Ping = buildDiscordPings(m)
	lines := parseCommandLines(m.Content, prefix, d.manager.i18n)
	recognized := false
	for _, line := range lines {
		if line == "" {
			continue
		}
		tokens := splitQuotedArgs(line)
		if len(tokens) == 0 {
			continue
		}
		if disabled, ok := ctx.Config.GetStringSlice("general.disabledCommands"); ok {
			if containsString(disabled, tokens[0]) {
				continue
			}
		}
		recognized = true
		if d.handleSpecialCommand(s, m, ctx, tokens) {
			continue
		}
		reply, err := d.manager.Registry().Execute(ctx, line)
		if err != nil {
			_, _ = s.ChannelMessageSend(m.ChannelID, err.Error())
			continue
		}
		if reply == "" {
			continue
		}
		if handled := d.sendSpecialReply(s, m.ChannelID, reply); handled {
			continue
		}
		sendText := func(channelID string, text string) bool {
			const discordLimit = 1900
			for _, chunk := range splitDiscordMessage(text, discordLimit) {
				if _, err := s.ChannelMessageSend(channelID, chunk); err != nil {
					if logger := logging.Get().Discord; logger != nil {
						logger.Warnf("Discord message send failed (channel=%q): %v", channelID, err)
					}
					return false
				}
			}
			return true
		}
		if !isDM && d.shouldReplyByDM(ctx, reply) {
			if ch, err := s.UserChannelCreate(m.Author.ID); err == nil && ch != nil {
				if sendText(ch.ID, reply) {
					continue
				}
			}
		}
		_ = sendText(m.ChannelID, reply)
	}
	if isDM && !recognized {
		if msg, ok := d.manager.cfg.GetString("discord.unrecognisedCommandMessage"); ok && msg != "" {
			_, _ = s.ChannelMessageSend(m.ChannelID, msg)
		}
	}
}

func (d *Discord) sendSpecialReply(s *discordgo.Session, channelID, reply string) bool {
	if strings.HasPrefix(reply, command.FileReplyPrefix) {
		raw := strings.TrimPrefix(reply, command.FileReplyPrefix)
		var payload struct {
			Name    string `json:"name"`
			Message string `json:"message"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		if payload.Name == "" || payload.Content == "" {
			return false
		}
		msg := discordgo.MessageSend{Content: payload.Message}
		msg.Files = []*discordgo.File{
			{
				Name:   payload.Name,
				Reader: strings.NewReader(payload.Content),
			},
		}
		_, _ = s.ChannelMessageSendComplex(channelID, &msg)
		return true
	}
	if strings.HasPrefix(reply, command.DiscordEmbedPrefix) {
		raw := strings.TrimPrefix(reply, command.DiscordEmbedPrefix)
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
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
		if msg.Content == "" && len(msg.Embeds) == 0 {
			return false
		}
		_, _ = s.ChannelMessageSendComplex(channelID, &msg)
		return true
	}
	return false
}

func (d *Discord) rolesForDM(s *discordgo.Session, userID string) []string {
	guilds, _ := d.manager.cfg.GetStringSlice("discord.guilds")
	if len(guilds) == 0 {
		return nil
	}
	roleSet := map[string]struct{}{}
	for _, guildID := range guilds {
		if guildID == "" {
			continue
		}
		member, err := s.GuildMember(guildID, userID)
		if err != nil || member == nil {
			continue
		}
		for _, role := range member.Roles {
			if role != "" {
				roleSet[role] = struct{}{}
			}
		}
	}
	if len(roleSet) == 0 {
		return nil
	}
	roles := make([]string, 0, len(roleSet))
	for role := range roleSet {
		roles = append(roles, role)
	}
	return roles
}

func (d *Discord) logDM(s *discordgo.Session, m *discordgo.MessageCreate) {
	channelID, ok := d.manager.cfg.GetString("discord.dmLogChannelID")
	if !ok || channelID == "" {
		return
	}
	msg := fmt.Sprintf("%s > %s", m.Author.Mention(), m.Content)
	if guilds, ok := d.manager.cfg.GetStringSlice("discord.guilds"); ok && len(guilds) > 1 {
		msg = fmt.Sprintf("%s %s > %s", m.Author.Username, m.Author.Mention(), m.Content)
	}
	if containsID(d.manager.cfg, "discord.admins", m.Author.ID) {
		msg = fmt.Sprintf("**%s** > %s", m.Author.Username, m.Content)
	}
	logMsg, err := s.ChannelMessageSend(channelID, msg)
	if err != nil || logMsg == nil {
		return
	}
	if minutes, ok := d.manager.cfg.GetInt("discord.dmLogChannelDeletionTime"); ok && minutes > 0 {
		time.AfterFunc(time.Duration(minutes)*time.Minute, func() {
			_ = s.ChannelMessageDelete(channelID, logMsg.ID)
		})
	}
}

func (d *Discord) shouldReplyByDM(ctx *command.Context, reply string) bool {
	if ctx == nil || ctx.I18n == nil {
		return false
	}
	tr := ctx.I18n.Translator(ctx.Language)
	if tr == nil {
		return false
	}
	return reply == tr.Translate("Please run commands in Direct Messages", false)
}

func buildDiscordPings(m *discordgo.MessageCreate) string {
	if m == nil {
		return ""
	}
	parts := []string{}
	for _, user := range m.Mentions {
		if user != nil {
			parts = append(parts, fmt.Sprintf("<@!%s>", user.ID))
		}
	}
	for _, roleID := range m.MentionRoles {
		if roleID != "" {
			parts = append(parts, fmt.Sprintf("<@&%s>", roleID))
		}
	}
	return strings.Join(parts, "")
}

func (d *Discord) handleSpecialCommand(s *discordgo.Session, m *discordgo.MessageCreate, ctx *command.Context, tokens []string) bool {
	switch tokens[0] {
	case "poracle-clean":
		d.handlePoracleClean(s, m, ctx)
		return true
	case "poracle-id":
		d.handlePoracleID(s, m, ctx, tokens[1:])
		return true
	case "poracle-emoji":
		d.handlePoracleEmoji(s, m, ctx, tokens[1:])
		return true
	case "role":
		d.handleRoleCommand(s, m, ctx, tokens[1:])
		return true
	case "autocreate":
		d.handleAutocreate(s, m, ctx, tokens[1:])
		return true
	default:
		return false
	}
}
