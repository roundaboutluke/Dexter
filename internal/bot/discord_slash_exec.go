package bot

import (
	"fmt"
	"strings"
	"time"

	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"poraclego/internal/command"
	"poraclego/internal/logging"
)

func (d *Discord) respondDeferredEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord deferred interaction respond failed: %v", err)
		}
	}
}

func (d *Discord) respondDeferred(s *discordgo.Session, i *discordgo.InteractionCreate) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord deferred interaction respond failed: %v", err)
		}
	}
}

func (d *Discord) respondComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    text,
			Embeds:     embeds,
			Flags:      flags,
			Components: components,
		},
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction respond failed: %v", err)
		}
	}
}

func (d *Discord) respondEphemeralComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	d.respondComponentsEmbed(s, i, text, embeds, components, true)
}

func (d *Discord) respondEditMessage(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed) {
	content := text
	var embedPtr *[]*discordgo.MessageEmbed
	if embeds != nil {
		embedPtr = &embeds
	}
	edit := &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  embedPtr,
	}
	if _, err := s.InteractionResponseEdit(i.Interaction, edit); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction edit failed: %v", err)
		}
	}
}

func (d *Discord) followupEphemeralSlashReply(s *discordgo.Session, i *discordgo.InteractionCreate, reply string) {
	if i == nil || i.Interaction == nil {
		return
	}
	if d.sendSpecialSlashFollowup(s, i, reply) {
		return
	}
	const discordLimit = 1900
	for _, chunk := range splitDiscordMessage(reply, discordLimit) {
		if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: chunk}); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup failed: %v", err)
			}
			return
		}
	}
}

func (d *Discord) sendSpecialSlashFollowup(s *discordgo.Session, i *discordgo.InteractionCreate, reply string) bool {
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
		if payload.Message != "" {
			d.followupEphemeralSlashReply(s, i, payload.Message)
		}
		msg := &discordgo.WebhookParams{
			Files: []*discordgo.File{
				{Name: payload.Name, Reader: strings.NewReader(payload.Content)},
			},
		}
		if _, err := s.FollowupMessageCreate(i.Interaction, true, msg); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup (file) failed: %v", err)
			}
		}
		return true
	}
	if strings.HasPrefix(reply, command.DiscordEmbedPrefix) {
		raw := strings.TrimPrefix(reply, command.DiscordEmbedPrefix)
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		content := ""
		if value, ok := payload["content"].(string); ok {
			content = value
		}
		var embeds []*discordgo.MessageEmbed
		if embedRaw, ok := payload["embed"]; ok {
			if embed := decodeEmbed(embedRaw); embed != nil {
				embeds = []*discordgo.MessageEmbed{embed}
			}
		}
		if embedsRaw, ok := payload["embeds"]; ok {
			if parsed := decodeEmbeds(embedsRaw); len(parsed) > 0 {
				embeds = parsed
			}
		}
		if content == "" && len(embeds) == 0 {
			return false
		}
		msg := &discordgo.WebhookParams{
			Content: content,
			Embeds:  embeds,
		}
		if _, err := s.FollowupMessageCreate(i.Interaction, true, msg); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup (embed) failed: %v", err)
			}
		}
		return true
	}
	return false
}

func (d *Discord) respondEditComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	content := text
	var embedPtr *[]*discordgo.MessageEmbed
	if embeds != nil {
		embedPtr = &embeds
	}
	var componentPtr *[]discordgo.MessageComponent
	if components != nil {
		componentPtr = &components
	}
	edit := &discordgo.WebhookEdit{
		Content:    &content,
		Embeds:     embedPtr,
		Components: componentPtr,
	}
	_, _ = s.InteractionResponseEdit(i.Interaction, edit)
}

func (d *Discord) respondUpdateComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    text,
			Embeds:     embeds,
			Components: components,
		},
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction update failed: %v", err)
		}
	}
}

func (d *Discord) sendSpecialSlashReply(s *discordgo.Session, i *discordgo.InteractionCreate, reply string) bool {
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
		if payload.Message != "" {
			d.respondEditMessage(s, i, payload.Message, nil)
		} else {
			d.respondEditMessage(s, i, "", nil)
		}
		msg := &discordgo.WebhookParams{
			Files: []*discordgo.File{
				{
					Name:   payload.Name,
					Reader: strings.NewReader(payload.Content),
				},
			},
		}
		if _, err := s.FollowupMessageCreate(i.Interaction, true, msg); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup (file) failed: %v", err)
			}
		}
		return true
	}
	if strings.HasPrefix(reply, command.DiscordEmbedPrefix) {
		raw := strings.TrimPrefix(reply, command.DiscordEmbedPrefix)
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		content := ""
		if value, ok := payload["content"].(string); ok {
			content = value
		}
		var embeds []*discordgo.MessageEmbed
		if embedRaw, ok := payload["embed"]; ok {
			if embed := decodeEmbed(embedRaw); embed != nil {
				embeds = []*discordgo.MessageEmbed{embed}
			}
		}
		if embedsRaw, ok := payload["embeds"]; ok {
			if parsed := decodeEmbeds(embedsRaw); len(parsed) > 0 {
				embeds = parsed
			}
		}
		if content == "" && len(embeds) == 0 {
			return false
		}
		d.respondEditMessage(s, i, content, embeds)
		return true
	}
	return false
}

func (d *Discord) respondWithModal(s *discordgo.Session, i *discordgo.InteractionCreate, customID, title, label, placeholder string) {
	component := discordgo.TextInput{
		CustomID:    "query",
		Label:       label,
		Style:       discordgo.TextInputShort,
		Placeholder: placeholder,
		Required:    true,
	}
	modal := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{component}},
			},
		},
	}
	if err := s.InteractionRespond(i.Interaction, modal); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction modal respond failed: %v", err)
		}
	}
}

func (d *Discord) respondWithScheduleModal(s *discordgo.Session, i *discordgo.InteractionCreate, customID, startPlaceholder, endPlaceholder, startValue, endValue string) {
	startInput := discordgo.TextInput{
		CustomID:    "start",
		Label:       "Start time (HH:MM)",
		Style:       discordgo.TextInputShort,
		Placeholder: startPlaceholder,
		Value:       startValue,
		Required:    true,
	}
	endInput := discordgo.TextInput{
		CustomID:    "end",
		Label:       "End time (HH:MM)",
		Style:       discordgo.TextInputShort,
		Placeholder: endPlaceholder,
		Value:       endValue,
		Required:    true,
	}
	modal := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    "Add schedule",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{startInput}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{endInput}},
			},
		},
	}
	_ = s.InteractionRespond(i.Interaction, modal)
}

func modalTextValue(data discordgo.ModalSubmitInteractionData, customID string) string {
	fallback := ""
	for _, row := range data.Components {
		var components []discordgo.MessageComponent
		switch actions := row.(type) {
		case discordgo.ActionsRow:
			components = actions.Components
		case *discordgo.ActionsRow:
			components = actions.Components
		default:
			continue
		}
		for _, comp := range components {
			switch input := comp.(type) {
			case discordgo.TextInput:
				if fallback == "" {
					fallback = input.Value
				}
				if input.CustomID == customID {
					return input.Value
				}
			case *discordgo.TextInput:
				if fallback == "" {
					fallback = input.Value
				}
				if input.CustomID == customID {
					return input.Value
				}
			}
		}
	}
	return fallback
}

func slashOptions(data discordgo.ApplicationCommandInteractionData) []*discordgo.ApplicationCommandInteractionDataOption {
	if len(data.Options) == 0 {
		return nil
	}
	if data.Options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return data.Options[0].Options
	}
	if data.Options[0].Type == discordgo.ApplicationCommandOptionSubCommandGroup && len(data.Options[0].Options) > 0 {
		return data.Options[0].Options[0].Options
	}
	return data.Options
}

func slashSubcommand(data discordgo.ApplicationCommandInteractionData) string {
	if len(data.Options) == 0 {
		return ""
	}
	if data.Options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return data.Options[0].Name
	}
	return ""
}

func focusedOption(options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	for _, opt := range options {
		if opt.Focused {
			return opt
		}
		if len(opt.Options) > 0 {
			if child := focusedOption(opt.Options); child != nil {
				return child
			}
		}
	}
	return nil
}

func optionString(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (string, bool) {
	for _, opt := range options {
		if opt.Name != name {
			continue
		}
		if value, ok := opt.Value.(string); ok {
			return value, true
		}
	}
	return "", false
}

func optionInt(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (int, bool) {
	for _, opt := range options {
		if opt.Name != name {
			continue
		}
		switch value := opt.Value.(type) {
		case float64:
			return int(value), true
		case int:
			return value, true
		case int64:
			return int(value), true
		}
	}
	return 0, false
}

func optionBool(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (bool, bool) {
	for _, opt := range options {
		if opt.Name != name {
			continue
		}
		if value, ok := opt.Value.(bool); ok {
			return value, true
		}
	}
	return false, false
}

func optionEnabled(options []*discordgo.ApplicationCommandInteractionDataOption, name string) bool {
	for _, opt := range options {
		if opt.Name != name {
			continue
		}
		switch value := opt.Value.(type) {
		case bool:
			return value
		case string:
			flag := strings.ToLower(strings.TrimSpace(value))
			if flag == "" {
				return false
			}
			switch flag {
			case "0", "false", "no", "off", "none":
				return false
			default:
				return true
			}
		case int:
			return value != 0
		case int64:
			return value != 0
		case float64:
			return value != 0
		default:
			// Unknown payload types are treated as enabled when present.
			return true
		}
	}
	return false
}

func appendRangeArg(args []string, prefix, maxPrefix string, minVal, maxVal *int) []string {
	if minVal == nil && maxVal == nil {
		return args
	}
	if minVal != nil && maxVal != nil {
		if *minVal == *maxVal {
			return append(args, fmt.Sprintf("%s%d", prefix, *minVal))
		}
		return append(args, fmt.Sprintf("%s%d-%d", prefix, *minVal, *maxVal))
	}
	if minVal != nil {
		return append(args, fmt.Sprintf("%s%d", prefix, *minVal))
	}
	return append(args, fmt.Sprintf("%s%d", maxPrefix, *maxVal))
}

func (d *Discord) startSlashGuide(i *discordgo.InteractionCreate, commandName, step string) {
	state := &slashBuilderState{
		Command:   commandName,
		Step:      step,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	d.setSlashState(i.Member, i.User, state)
}

func (d *Discord) logSlashUX(i *discordgo.InteractionCreate, commandName, action, detail string) {
	logger := logging.Get().Discord
	if logger == nil || i == nil {
		return
	}
	userID, _ := slashUser(i)
	if detail != "" {
		logger.Infof("slash ux: command=%s action=%s user=%s detail=%s", commandName, action, userID, detail)
		return
	}
	logger.Infof("slash ux: command=%s action=%s user=%s", commandName, action, userID)
}

func (d *Discord) effectiveProfileInfo(i *discordgo.InteractionCreate) (int, string) {
	if d == nil || d.manager == nil || d.manager.query == nil {
		return 1, "Profile 1"
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return 1, "Profile 1"
	}
	profileNo := d.userProfileNo(userID)
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return profileNo, fmt.Sprintf("Profile %d", profileNo)
	}
	if row := profileRowByNo(profiles, profileNo); row != nil {
		return profileNo, profileDisplayName(row)
	}
	return profileNo, fmt.Sprintf("Profile %d", profileNo)
}

func (d *Discord) guidedWeatherArgs(i *discordgo.InteractionCreate, condition string) ([]string, string) {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return nil, "Please pick a weather condition."
	}
	userID, _ := slashUser(i)
	location := ""
	if d.manager != nil && d.manager.query != nil && userID != "" {
		if row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID}); err == nil && row != nil {
			lat := toFloat(row["latitude"])
			lon := toFloat(row["longitude"])
			if lat != 0 || lon != 0 {
				location = fmt.Sprintf("%s,%s", formatFloat(lat), formatFloat(lon))
			} else if d.manager.fences != nil {
				areas := parseAreaListFromHuman(row)
				if len(areas) > 0 {
					target := strings.ToLower(strings.TrimSpace(areas[0]))
					for _, fence := range d.manager.fences.Fences {
						if strings.EqualFold(strings.TrimSpace(fence.Name), target) {
							if centerLat, centerLon, ok := fenceCentroid(fence); ok {
								location = fmt.Sprintf("%s,%s", formatFloat(centerLat), formatFloat(centerLon))
							}
							break
						}
					}
				}
			}
		}
	}
	if location == "" {
		return nil, "Set a saved location in `/profile`, or provide a location with `/weather condition:<condition> location:<place>`."
	}
	args := append(strings.Fields(location), "|", condition)
	return args, ""
}

func profileDisplayName(row map[string]any) string {
	if row == nil {
		return "Profile"
	}
	profileNo := toInt(row["profile_no"], 0)
	name := ""
	if raw, ok := row["name"]; ok && raw != nil {
		name = strings.TrimSpace(fmt.Sprintf("%v", raw))
	}
	if name == "" {
		return fmt.Sprintf("Profile %d", profileNo)
	}
	if profileNo > 0 {
		return fmt.Sprintf("%s (P%d)", name, profileNo)
	}
	return name
}

func (d *Discord) executeSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, state *slashBuilderState) {
	if state == nil {
		d.respondEphemeral(s, i, "No command to run.")
		return
	}
	line := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
	if line == "" {
		d.respondEphemeral(s, i, "No command to run.")
		return
	}
	d.executeSlashLineDeferred(s, i, line)
}

func (d *Discord) executeSlashLine(s *discordgo.Session, i *discordgo.InteractionCreate, line string) {
	reply := d.buildSlashReply(s, i, line)
	if reply == "" {
		reply = "Done."
	}
	if d.sendSpecialSlashReply(s, i, reply) {
		return
	}
	const discordLimit = 1900
	chunks := splitDiscordMessage(reply, discordLimit)
	d.respondEphemeral(s, i, chunks[0])
	for _, chunk := range chunks[1:] {
		content := chunk
		if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: content}); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup failed: %v", err)
			}
			return
		}
	}
}

func (d *Discord) executeSlashLineDeferred(s *discordgo.Session, i *discordgo.InteractionCreate, line string) {
	d.respondDeferredEphemeral(s, i)
	reply := d.buildSlashReply(s, i, line)
	if reply == "" {
		reply = "Done."
	}
	if d.sendSpecialSlashReply(s, i, reply) {
		return
	}
	const discordLimit = 1900
	chunks := splitDiscordMessage(reply, discordLimit)
	d.respondEditMessage(s, i, chunks[0], nil)
	for _, chunk := range chunks[1:] {
		content := chunk
		if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: content}); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup failed: %v", err)
			}
			return
		}
	}
}

func (d *Discord) buildSlashContext(s *discordgo.Session, i *discordgo.InteractionCreate) *command.Context {
	if d == nil || d.manager == nil || i == nil {
		return nil
	}
	channelName := ""
	isDM := i.GuildID == ""
	if s != nil {
		if channel, err := s.Channel(i.ChannelID); err == nil && channel != nil {
			channelName = channel.Name
			if channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM {
				isDM = true
			}
		}
	}
	roles := []string{}
	userID, userName := slashUser(i)
	isAdmin := false
	if d.manager.cfg != nil {
		isAdmin = containsID(d.manager.cfg, "discord.admins", userID)
	}
	if s != nil {
		if isDM {
			roles = d.rolesForDM(s, userID)
		} else if channelName != "" {
			if member, err := s.GuildMember(i.GuildID, userID); err == nil && member != nil {
				roles = append(roles, member.Roles...)
			}
		}
	}
	return d.manager.Context("discord", "", "/", userID, userName, i.ChannelID, channelName, isDM, isAdmin, roles, ".")
}

func (d *Discord) buildSlashReply(s *discordgo.Session, i *discordgo.InteractionCreate, line string) string {
	if line == "" {
		return "No command to run."
	}
	ctx := d.buildSlashContext(s, i)
	if ctx == nil {
		return "No command to run."
	}
	tokens := splitQuotedArgs(line)
	if len(tokens) > 0 {
		if disabled, ok := ctx.Config.GetStringSlice("general.disabledCommands"); ok {
			if containsString(disabled, tokens[0]) {
				return "That command is disabled."
			}
		}
	}
	reply, err := d.manager.Registry().Execute(ctx, line)
	if err != nil {
		return err.Error()
	}
	if reply == "" {
		return "Done."
	}
	return reply
}
