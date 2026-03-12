package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/command"
	"poraclego/internal/config"
)

type channelTemplate struct {
	Name       string             `json:"name"`
	Definition channelTemplateDef `json:"definition"`
}

type channelTemplateDef struct {
	Category *templateCategory `json:"category"`
	Channels []templateChannel `json:"channels"`
}

type templateCategory struct {
	CategoryName string `json:"categoryName"`
	Roles        any    `json:"roles"`
}

type templateChannel struct {
	ChannelName string   `json:"channelName"`
	ChannelType string   `json:"channelType"`
	Topic       string   `json:"topic"`
	Roles       any      `json:"roles"`
	ControlType string   `json:"controlType"`
	WebhookName string   `json:"webhookName"`
	Commands    []string `json:"commands"`
}

func (d *Discord) handleAutocreate(s *discordgo.Session, m *discordgo.MessageCreate, ctx *command.Context, args []string) {
	if !ctx.IsAdmin {
		return
	}
	guildID := resolveGuildID(m, args)
	guild, err := d.fetchGuild(s, guildID)
	if err != nil || guild == nil {
		_, _ = s.ChannelMessageSend(m.ChannelID, "I was not able to retrieve that guild")
		return
	}
	if perms, err := s.UserChannelPermissions(getBotID(s), m.ChannelID); err == nil {
		if perms&discordgo.PermissionManageWebhooks == 0 {
			_, _ = s.ChannelMessageSend(m.ChannelID, "I have not been allowed to manage webhooks!")
			return
		}
		if perms&discordgo.PermissionManageChannels == 0 {
			_, _ = s.ChannelMessageSend(m.ChannelID, "I have not been allowed to manage channels!")
			return
		}
	}

	args = stripGuildArg(args)
	if len(args) == 0 {
		_, _ = s.ChannelMessageSend(m.ChannelID, "I can't find that channel template! (remember it has to be your first parameter)")
		return
	}
	templateName := args[0]
	templateDefs, err := loadChannelTemplates(ctx.Root)
	if err != nil {
		_, _ = s.ChannelMessageSend(m.ChannelID, "Cannot read channelTemplate definition")
		return
	}
	var selected *channelTemplate
	for _, tpl := range templateDefs {
		if tpl.Name == templateName {
			selected = &tpl
			break
		}
	}
	if selected == nil {
		_, _ = s.ChannelMessageSend(m.ChannelID, "I can't find that channel template! (remember it has to be your first parameter)")
		return
	}
	formatArgs := args[1:]
	subArgs := make([]string, len(formatArgs))
	for i, arg := range formatArgs {
		subArgs[i] = strings.ReplaceAll(arg, " ", "_")
	}

	categoryID := ""
	if selected.Definition.Category != nil {
		categoryName := formatTemplate(selected.Definition.Category.CategoryName, formatArgs)
		overwrites := buildRoleOverwrites(s, guildID, selected.Definition.Category.Roles, formatArgs)
		create := discordgo.GuildChannelCreateData{
			Name:                 categoryName,
			Type:                 discordgo.ChannelTypeGuildCategory,
			PermissionOverwrites: overwrites,
		}
		channel, err := s.GuildChannelCreateComplex(guildID, create)
		if err == nil && channel != nil {
			categoryID = channel.ID
			_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(">> Creating %s", categoryName))
		}
	}

	dirty := false
	for _, channelDef := range selected.Definition.Channels {
		channelName := formatTemplate(channelDef.ChannelName, formatArgs)
		create := discordgo.GuildChannelCreateData{
			Name: channelName,
		}
		switch strings.ToLower(channelDef.ChannelType) {
		case "voice":
			create.Type = discordgo.ChannelTypeGuildVoice
		default:
			create.Type = discordgo.ChannelTypeGuildText
		}
		if categoryID != "" {
			create.ParentID = categoryID
		}
		if channelDef.Topic != "" {
			create.Topic = formatTemplate(channelDef.Topic, formatArgs)
		}
		if overwrites := buildRoleOverwrites(s, guildID, channelDef.Roles, formatArgs); len(overwrites) > 0 {
			create.PermissionOverwrites = overwrites
		}

		channel, err := s.GuildChannelCreateComplex(guildID, create)
		if err != nil || channel == nil {
			continue
		}
		_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(">> Creating %s", channelName))

		if channelDef.ControlType == "" {
			continue
		}
		_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(">> Adding control type: %s", channelDef.ControlType))

		target := commandTarget{}
		if channelDef.ControlType == "bot" {
			target = commandTarget{
				ID:   channel.ID,
				Type: "discord:channel",
				Name: formatTemplate(channelDef.ChannelName, subArgs),
			}
		} else {
			webhook, err := s.WebhookCreate(channel.ID, "Poracle", "")
			if err != nil || webhook == nil {
				continue
			}
			name := channelName
			if channelDef.WebhookName != "" {
				name = formatTemplate(channelDef.WebhookName, subArgs)
			}
			target = commandTarget{
				ID:   buildWebhookURL(webhook),
				Type: "webhook",
				Name: name,
			}
		}

		if !d.insertAutocreateTarget(ctx, target) {
			continue
		}
		dirty = true

		for _, cmd := range channelDef.Commands {
			cmdLine := formatTemplate(cmd, subArgs)
			_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(">>> Executing %s", cmdLine))
			clone := *ctx
			clone.TargetOverride = &command.Target{
				ID:   target.ID,
				Type: target.Type,
				Name: target.Name,
			}
			_, _ = d.manager.Registry().Execute(&clone, cmdLine)
		}
	}
	if dirty {
		d.manager.RefreshAlertState()
	}
}

func (d *Discord) insertAutocreateTarget(ctx *command.Context, target commandTarget) bool {
	if ctx == nil || ctx.Query == nil {
		return false
	}
	_, err := ctx.Query.InsertQuery("humans", map[string]any{
		"id":                   target.ID,
		"type":                 target.Type,
		"name":                 target.Name,
		"area":                 "[]",
		"community_membership": "[]",
	})
	return err == nil
}

type commandTarget struct {
	ID   string
	Type string
	Name string
}

func stripGuildArg(args []string) []string {
	out := []string{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "guild") {
			continue
		}
		out = append(out, arg)
	}
	return out
}

func formatTemplate(input string, args []string) string {
	if input == "" {
		return ""
	}
	out := input
	for i := len(args) - 1; i >= 0; i-- {
		out = strings.ReplaceAll(out, fmt.Sprintf("{%d}", i), args[i])
	}
	return out
}

func loadChannelTemplates(root string) ([]channelTemplate, error) {
	paths := []string{
		filepath.Join(root, "config", "channelTemplate.json"),
		filepath.Join(root, "config", "defaults", "channelTemplate.json"),
	}
	var payload []byte
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		payload = data
		break
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("missing channelTemplate.json")
	}
	payload = config.StripJSONComments(payload)
	var templates []channelTemplate
	if err := json.Unmarshal(payload, &templates); err != nil {
		return nil, err
	}
	return templates, nil
}

func buildRoleOverwrites(s *discordgo.Session, guildID string, rawRoles any, args []string) []*discordgo.PermissionOverwrite {
	roleDefs := parseRoleDefs(rawRoles)
	if len(roleDefs) == 0 {
		return nil
	}
	overwrites := []*discordgo.PermissionOverwrite{}
	guild, _ := s.State.Guild(guildID)
	for _, role := range roleDefs {
		roleName := formatTemplate(role.Name, args)
		roleID := resolveRoleID(s, guild, guildID, roleName)
		if roleID == "" {
			continue
		}
		allow, deny := rolePermissions(role)
		overwrites = append(overwrites, &discordgo.PermissionOverwrite{
			ID:    roleID,
			Type:  discordgo.PermissionOverwriteTypeRole,
			Allow: allow,
			Deny:  deny,
		})
	}
	return overwrites
}

type roleDefinition struct {
	Name                 string `json:"name"`
	View                 *bool  `json:"view"`
	ViewHistory          *bool  `json:"viewHistory"`
	Send                 *bool  `json:"send"`
	React                *bool  `json:"react"`
	PingEveryone         *bool  `json:"pingEveryone"`
	EmbedLinks           *bool  `json:"embedLinks"`
	AttachFiles          *bool  `json:"attachFiles"`
	SendTTS              *bool  `json:"sendTTS"`
	ExternalEmoji        *bool  `json:"externalEmoji"`
	ExternalStickers     *bool  `json:"externalStickers"`
	CreatePublicThreads  *bool  `json:"createPublicThreads"`
	CreatePrivateThreads *bool  `json:"createPrivateThreads"`
	SendThreads          *bool  `json:"sendThreads"`
	SlashCommands        *bool  `json:"slashCommands"`
	Connect              *bool  `json:"connect"`
	Speak                *bool  `json:"speak"`
	AutoMic              *bool  `json:"autoMic"`
	Stream               *bool  `json:"stream"`
	VcActivities         *bool  `json:"vcActivities"`
	PrioritySpeaker      *bool  `json:"prioritySpeaker"`
	CreateInvite         *bool  `json:"createInvite"`
	Channels             *bool  `json:"channels"`
	Messages             *bool  `json:"messages"`
	Roles                *bool  `json:"roles"`
	Webhooks             *bool  `json:"webhooks"`
	Threads              *bool  `json:"threads"`
	Events               *bool  `json:"events"`
	Mute                 *bool  `json:"mute"`
	Deafen               *bool  `json:"deafen"`
	Move                 *bool  `json:"move"`
}

func parseRoleDefs(raw any) []roleDefinition {
	switch v := raw.(type) {
	case []any:
		data, _ := json.Marshal(v)
		roles := []roleDefinition{}
		_ = json.Unmarshal(data, &roles)
		return roles
	case []roleDefinition:
		return v
	default:
		return nil
	}
}

func resolveRoleID(s *discordgo.Session, guild *discordgo.Guild, guildID, roleName string) string {
	if roleName == "@everyone" {
		return guildID
	}
	if guild != nil {
		for _, role := range guild.Roles {
			if role.Name == roleName {
				return role.ID
			}
		}
	}
	created, err := s.GuildRoleCreate(guildID, &discordgo.RoleParams{Name: roleName})
	if err != nil || created == nil {
		return ""
	}
	return created.ID
}

func rolePermissions(role roleDefinition) (int64, int64) {
	var allow int64
	var deny int64
	setPerm := func(flag int64, value *bool) {
		if value == nil {
			return
		}
		if *value {
			allow |= flag
		} else {
			deny |= flag
		}
	}
	setPerm(discordgo.PermissionViewChannel, role.View)
	setPerm(discordgo.PermissionReadMessageHistory, role.ViewHistory)
	setPerm(discordgo.PermissionSendMessages, role.Send)
	setPerm(discordgo.PermissionAddReactions, role.React)
	setPerm(discordgo.PermissionMentionEveryone, role.PingEveryone)
	setPerm(discordgo.PermissionEmbedLinks, role.EmbedLinks)
	setPerm(discordgo.PermissionAttachFiles, role.AttachFiles)
	setPerm(discordgo.PermissionSendTTSMessages, role.SendTTS)
	setPerm(discordgo.PermissionUseExternalEmojis, role.ExternalEmoji)
	setPerm(discordgo.PermissionUseExternalStickers, role.ExternalStickers)
	setPerm(discordgo.PermissionCreatePublicThreads, role.CreatePublicThreads)
	setPerm(discordgo.PermissionCreatePrivateThreads, role.CreatePrivateThreads)
	setPerm(discordgo.PermissionSendMessagesInThreads, role.SendThreads)
	setPerm(discordgo.PermissionUseSlashCommands, role.SlashCommands)
	setPerm(discordgo.PermissionVoiceConnect, role.Connect)
	setPerm(discordgo.PermissionVoiceSpeak, role.Speak)
	setPerm(discordgo.PermissionVoiceUseVAD, role.AutoMic)
	setPerm(discordgo.PermissionVoiceStreamVideo, role.Stream)
	setPerm(discordgo.PermissionUseActivities, role.VcActivities)
	setPerm(discordgo.PermissionVoicePrioritySpeaker, role.PrioritySpeaker)
	setPerm(discordgo.PermissionCreateInstantInvite, role.CreateInvite)
	setPerm(discordgo.PermissionManageChannels, role.Channels)
	setPerm(discordgo.PermissionManageMessages, role.Messages)
	setPerm(discordgo.PermissionManageRoles, role.Roles)
	setPerm(discordgo.PermissionManageWebhooks, role.Webhooks)
	setPerm(discordgo.PermissionManageThreads, role.Threads)
	setPerm(discordgo.PermissionManageEvents, role.Events)
	setPerm(discordgo.PermissionVoiceMuteMembers, role.Mute)
	setPerm(discordgo.PermissionVoiceDeafenMembers, role.Deafen)
	setPerm(discordgo.PermissionVoiceMoveMembers, role.Move)
	return allow, deny
}

func buildWebhookURL(webhook *discordgo.Webhook) string {
	if webhook == nil || webhook.ID == "" || webhook.Token == "" {
		return ""
	}
	return fmt.Sprintf("https://discord.com/api/webhooks/%s/%s", webhook.ID, webhook.Token)
}
