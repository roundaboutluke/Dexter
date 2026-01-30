package bot

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/command"
	"poraclego/internal/i18n"
	"poraclego/internal/uicons"
)

func (d *Discord) handlePoracleClean(s *discordgo.Session, m *discordgo.MessageCreate, ctx *command.Context) {
	if !ctx.IsAdmin && !ctx.IsDM {
		if ch, err := s.UserChannelCreate(m.Author.ID); err == nil {
			_, _ = s.ChannelMessageSend(ch.ID, "Please run commands in Direct Messages")
		}
		return
	}
	botID := getBotID(s)
	if botID == "" {
		return
	}
	const limit = 100
	startMsg, _ := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Will start cleaning up to %d messages back - do not re-run until finished", limit))
	messages, err := s.ChannelMessages(m.ChannelID, limit, "", "", "")
	if err == nil {
		for _, msg := range messages {
			if msg.Author != nil && msg.Author.ID == botID {
				_ = s.ChannelMessageDelete(m.ChannelID, msg.ID)
			}
		}
	}
	if startMsg != nil {
		_ = s.ChannelMessageDelete(m.ChannelID, startMsg.ID)
	}
	finishMsg, _ := s.ChannelMessageSend(m.ChannelID, "Cleaning finished")
	if finishMsg != nil {
		time.AfterFunc(15*time.Second, func() {
			_ = s.ChannelMessageDelete(m.ChannelID, finishMsg.ID)
		})
	}
}

func (d *Discord) handlePoracleID(s *discordgo.Session, m *discordgo.MessageCreate, ctx *command.Context, args []string) {
	if !ctx.IsAdmin {
		return
	}
	guildID := resolveGuildID(m, args)
	if guildID == "" {
		_, _ = s.ChannelMessageSend(m.ChannelID, "No guild has been set, either execute inside a channel or specify guild<id>")
		return
	}
	guild, err := d.fetchGuild(s, guildID)
	if err != nil || guild == nil {
		_, _ = s.ChannelMessageSend(m.ChannelID, "I was not able to retrieve that guild")
		return
	}
	var buf strings.Builder
	for _, emoji := range guild.Emojis {
		buf.WriteString(fmt.Sprintf("  \"%s\":\"<%s:%s:%s>\"\n", emoji.Name, emojiPrefix(emoji), emoji.Name, emoji.ID))
	}
	buf.WriteString("\n\n")
	for _, role := range guild.Roles {
		buf.WriteString(fmt.Sprintf("  \"%s\":\"%s\"\n", role.Name, role.ID))
	}
	_, _ = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content: "Here's your guild ids!",
		Files: []*discordgo.File{
			{
				Name:   "id.txt",
				Reader: strings.NewReader(buf.String()),
			},
		},
	})
}

func (d *Discord) handlePoracleEmoji(s *discordgo.Session, m *discordgo.MessageCreate, ctx *command.Context, args []string) {
	if !ctx.IsAdmin {
		return
	}
	guildID := resolveGuildID(m, args)
	if guildID == "" {
		_, _ = s.ChannelMessageSend(m.ChannelID, "No guild has been set, either execute inside a channel or specify guild<id>")
		return
	}
	guild, err := d.fetchGuild(s, guildID)
	if err != nil || guild == nil {
		_, _ = s.ChannelMessageSend(m.ChannelID, "I was not able to retrieve that guild")
		return
	}
	botID := getBotID(s)
	if botID == "" {
		return
	}
	perms, err := s.State.UserChannelPermissions(botID, m.ChannelID)
	if err == nil && perms&discordgo.PermissionManageEmojis == 0 {
		_, _ = s.ChannelMessageSend(m.ChannelID, "I have not been allowed to manage emojis and stickers!")
		return
	}
	upload := containsString(args, "upload")
	overwrite := containsString(args, "overwrite")
	if upload {
		_, _ = s.ChannelMessageSend(m.ChannelID, "Beginning upload of emojis, this may take a little while. Don't run a second time unless you are told it is finished!")
	}

	imgURL, _ := ctx.Config.GetString("general.imgUrl")
	if imgURL == "" {
		_, _ = s.ChannelMessageSend(m.ChannelID, "Currently configured imgUrl is not a uicons repository")
		return
	}
	ui := uicons.NewClient(imgURL, "png")
	if ok, _ := ui.IsUiconsRepository(); !ok {
		_, _ = s.ChannelMessageSend(m.ChannelID, "Currently configured imgUrl is not a uicons repository")
		return
	}

	existing := map[string]*discordgo.Emoji{}
	for _, emoji := range guild.Emojis {
		existing[emoji.Name] = emoji
	}
	poracleEmoji := map[string]*discordgo.Emoji{}

	setEmoji := func(url, name string) {
		if name == "" {
			return
		}
		discordName := strings.ReplaceAll("poracle-"+name, "-", "_")
		current := existing[discordName]
		if upload {
			if url == "" || strings.HasSuffix(url, "/0.png") {
				_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Repository does not have a suitable emoji for %s", discordName))
			} else {
				if current != nil && overwrite {
					_ = s.GuildEmojiDelete(guildID, current.ID)
					current = nil
				}
				if current == nil || overwrite {
					if data, err := fetchEmojiData(url); err == nil {
						_, _ = s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Uploading %s from %s", discordName, url))
						created, err := s.GuildEmojiCreate(guildID, &discordgo.EmojiParams{
							Name:  discordName,
							Image: data,
						})
						if err == nil {
							current = created
						}
					}
				}
			}
		}
		if current != nil {
			poracleEmoji[name] = current
		}
	}

	if raw, ok := ctx.Data.UtilData["types"].(map[string]any); ok {
		for _, value := range raw {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			emojiName := fmt.Sprintf("%v", entry["emoji"])
			if emojiName == "" {
				continue
			}
			typeID := toInt(entry["id"], 0)
			url := ""
			if upload {
				url, _ = ui.TypeIcon(typeID)
			}
			setEmoji(url, emojiName)
		}
	}
	if raw, ok := ctx.Data.UtilData["weather"].(map[string]any); ok {
		for id, value := range raw {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			emojiName := fmt.Sprintf("%v", entry["emoji"])
			if emojiName == "" {
				continue
			}
			weatherID := toInt(id, 0)
			url := ""
			if upload {
				url, _ = ui.WeatherIcon(weatherID)
			}
			setEmoji(url, emojiName)
		}
	}
	if raw, ok := ctx.Data.UtilData["lures"].(map[string]any); ok {
		for id, value := range raw {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			emojiName := fmt.Sprintf("%v", entry["emoji"])
			if emojiName == "" {
				continue
			}
			lureID := toInt(id, 0)
			url := ""
			if upload {
				url, _ = ui.RewardItemIcon(lureID)
			}
			setEmoji(url, emojiName)
		}
	}
	if raw, ok := ctx.Data.UtilData["teams"].(map[string]any); ok {
		for id, value := range raw {
			entry, ok := value.(map[string]any)
			if !ok {
				continue
			}
			emojiName := fmt.Sprintf("%v", entry["emoji"])
			if emojiName == "" {
				continue
			}
			teamID := toInt(id, 0)
			url := ""
			if upload {
				url, _ = ui.TeamIcon(teamID)
			}
			setEmoji(url, emojiName)
		}
	}

	type pair struct {
		name  string
		emoji *discordgo.Emoji
	}
	pairs := make([]pair, 0, len(poracleEmoji))
	for name, emoji := range poracleEmoji {
		pairs = append(pairs, pair{name: name, emoji: emoji})
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].name < pairs[j].name })

	var buf strings.Builder
	buf.WriteString("{\n  \"discord\": {")
	for i, entry := range pairs {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString(fmt.Sprintf("\n    \"%s\":\"<:%s:%s>\"", entry.name, entry.emoji.Name, entry.emoji.ID))
	}
	buf.WriteString("\n  }\n}\n")

	_, _ = s.ChannelMessageSendComplex(m.ChannelID, &discordgo.MessageSend{
		Content: "Here's a nice new emoji.json for you!",
		Files: []*discordgo.File{
			{
				Name:   "emoji.json",
				Reader: strings.NewReader(buf.String()),
			},
		},
	})
}

func (d *Discord) handleRoleCommand(s *discordgo.Session, m *discordgo.MessageCreate, ctx *command.Context, args []string) {
	if !ctx.IsAdmin && !ctx.IsDM {
		if ch, err := s.UserChannelCreate(m.Author.ID); err == nil {
			_, _ = s.ChannelMessageSend(ch.ID, "Please run commands in Direct Messages")
		}
		return
	}
	rawConfig, ok := ctx.Config.Get("discord.userRoleSubscription")
	if !ok {
		_ = s.MessageReactionAdd(m.ChannelID, m.ID, "🙅")
		return
	}
	roleConfig, ok := rawConfig.(map[string]any)
	if !ok || len(roleConfig) == 0 {
		_ = s.MessageReactionAdd(m.ChannelID, m.ID, "🙅")
		return
	}

	targetID := m.Author.ID
	if ctx.IsAdmin {
		re := command.NewRegexSet(ctx.I18n)
		for _, arg := range args {
			if re.User.MatchString(arg) {
				match := re.User.FindStringSubmatch(arg)
				if len(match) > 2 {
					targetID = match[2]
				}
			}
		}
	}
	human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": targetID, "admin_disable": 0})
	if err != nil || human == nil {
		_ = s.MessageReactionAdd(m.ChannelID, m.ID, "🙅")
		return
	}
	lang := ctx.Language
	if l, ok := human["language"].(string); ok && l != "" {
		lang = l
	}
	tr := ctx.I18n.Translator(lang)

	if len(args) == 0 || (args[0] != "add" && args[0] != "remove" && args[0] != "list" && args[0] != "membership") {
		reply := fmt.Sprintf("Valid commands are `%srole list`, `%srole add <areaname>`, `%srole remove <areaname>`", ctx.Prefix, ctx.Prefix, ctx.Prefix)
		_, _ = s.ChannelMessageSend(m.ChannelID, reply)
		return
	}

	switch args[0] {
	case "list":
		roleList := d.roleList(s, roleConfig, targetID, tr, false)
		d.sendRoleList(s, m.ChannelID, roleList)
	case "membership":
		roleList := d.roleList(s, roleConfig, targetID, tr, true)
		d.sendRoleList(s, m.ChannelID, roleList)
	default:
		set := args[0] == "add"
		results := d.setRoles(s, roleConfig, targetID, args[1:], set)
		for _, result := range results {
			if result.id == "" {
				_, _ = s.ChannelMessageSend(m.ChannelID, tr.TranslateFormat("Unknown role -- {0}", result.description))
				continue
			}
			if result.set {
				_, _ = s.ChannelMessageSend(m.ChannelID, tr.TranslateFormat("You have been granted the role {0}", result.description))
			} else {
				_, _ = s.ChannelMessageSend(m.ChannelID, tr.TranslateFormat("I have removed the role {0}", result.description))
			}
		}
	}
}

type roleChange struct {
	description string
	id          string
	set         bool
}

func (d *Discord) roleList(s *discordgo.Session, roleConfig map[string]any, userID string, tr *i18n.Translator, membershipOnly bool) string {
	var buf strings.Builder
	if membershipOnly {
		buf.WriteString(tr.Translate("You have the following roles", false) + ":\n")
	} else {
		buf.WriteString(tr.Translate("Roles available", false) + ":\n")
	}

	for guildID, raw := range roleConfig {
		guild, err := d.fetchGuild(s, guildID)
		if err != nil || guild == nil {
			continue
		}
		member, err := s.GuildMember(guildID, userID)
		if err != nil || member == nil {
			continue
		}
		buf.WriteString(fmt.Sprintf("**%s**\n", guild.Name))

		roleSets := parseRoleSets(raw)
		for _, set := range roleSets.exclusive {
			for _, role := range set {
				role.set = memberHasRole(member, role.id)
				if membershipOnly && !role.set {
					continue
				}
				check := ""
				if role.set {
					check = " ☑️"
				}
				buf.WriteString(fmt.Sprintf("   %s%s\n", strings.ReplaceAll(role.description, " ", "_"), check))
			}
			buf.WriteString("\n")
		}
		for _, role := range roleSets.general {
			role.set = memberHasRole(member, role.id)
			if membershipOnly && !role.set {
				continue
			}
			check := ""
			if role.set {
				check = " ☑️"
			}
			buf.WriteString(fmt.Sprintf("   %s%s\n", strings.ReplaceAll(role.description, " ", "_"), check))
		}
		if len(roleSets.general) > 0 {
			buf.WriteString("\n")
		}
	}

	return buf.String()
}

func (d *Discord) sendRoleList(s *discordgo.Session, channelID string, payload string) {
	if len(payload) > 2000 {
		_, _ = s.ChannelMessageSendComplex(channelID, &discordgo.MessageSend{
			Content: "Role List",
			Files: []*discordgo.File{
				{
					Name:   "rolelist.txt",
					Reader: strings.NewReader(payload),
				},
			},
		})
		return
	}
	_, _ = s.ChannelMessageSend(channelID, payload)
}

func (d *Discord) setRoles(s *discordgo.Session, roleConfig map[string]any, userID string, args []string, set bool) []roleChange {
	results := []roleChange{}
	for _, arg := range args {
		if strings.HasPrefix(arg, "user") {
			continue
		}
		roleArg := normalizeRoleName(arg)
		found := false

		for guildID, raw := range roleConfig {
			guild, err := d.fetchGuild(s, guildID)
			if err != nil || guild == nil {
				continue
			}
			member, err := s.GuildMember(guildID, userID)
			if err != nil || member == nil {
				continue
			}
			roleSets := parseRoleSets(raw)

			for _, role := range roleSets.general {
				if normalizeRoleName(role.description) == roleArg {
					found = true
					change := d.setRoleForMember(s, guildID, member, role.description, role.id, set)
					results = append(results, change...)
				}
			}

			for _, setGroup := range roleSets.exclusive {
				for _, role := range setGroup {
					if normalizeRoleName(role.description) == roleArg {
						found = true
						changes := d.setExclusiveRole(s, guildID, member, setGroup, role.id, set)
						results = append(results, changes...)
					}
				}
			}
		}
		if !found {
			results = append(results, roleChange{description: arg, id: "", set: false})
		}
	}
	return results
}

type roleEntry struct {
	description string
	id          string
	set         bool
}

type roleSets struct {
	general   []roleEntry
	exclusive [][]roleEntry
}

func parseRoleSets(raw any) roleSets {
	sets := roleSets{general: []roleEntry{}, exclusive: [][]roleEntry{}}
	config, ok := raw.(map[string]any)
	if !ok {
		return sets
	}
	if roles, ok := config["roles"].(map[string]any); ok {
		for desc, rid := range roles {
			sets.general = append(sets.general, roleEntry{description: desc, id: fmt.Sprintf("%v", rid)})
		}
	}
	if exclusive, ok := config["exclusiveRoles"]; ok {
		switch v := exclusive.(type) {
		case []any:
			for _, entry := range v {
				sets.exclusive = append(sets.exclusive, parseExclusiveEntry(entry))
			}
		case map[string]any:
			sets.exclusive = append(sets.exclusive, parseExclusiveEntry(v))
		}
	}
	return sets
}

func parseExclusiveEntry(raw any) []roleEntry {
	out := []roleEntry{}
	entries, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for desc, rid := range entries {
		out = append(out, roleEntry{description: desc, id: fmt.Sprintf("%v", rid)})
	}
	return out
}

func (d *Discord) setRoleForMember(s *discordgo.Session, guildID string, member *discordgo.Member, description, roleID string, set bool) []roleChange {
	results := []roleChange{}
	if set {
		_ = s.GuildMemberRoleAdd(guildID, member.User.ID, roleID)
		results = append(results, roleChange{description: description, id: roleID, set: true})
	} else {
		_ = s.GuildMemberRoleRemove(guildID, member.User.ID, roleID)
		results = append(results, roleChange{description: description, id: roleID, set: false})
	}
	return results
}

func (d *Discord) setExclusiveRole(s *discordgo.Session, guildID string, member *discordgo.Member, group []roleEntry, roleID string, set bool) []roleChange {
	results := []roleChange{}
	if set {
		for _, entry := range group {
			if entry.id == roleID {
				_ = s.GuildMemberRoleAdd(guildID, member.User.ID, entry.id)
				results = append(results, roleChange{description: entry.description, id: entry.id, set: true})
			} else {
				_ = s.GuildMemberRoleRemove(guildID, member.User.ID, entry.id)
				results = append(results, roleChange{description: entry.description, id: entry.id, set: false})
			}
		}
	} else {
		description := roleID
		for _, entry := range group {
			if entry.id == roleID {
				description = entry.description
				break
			}
		}
		_ = s.GuildMemberRoleRemove(guildID, member.User.ID, roleID)
		results = append(results, roleChange{description: description, id: roleID, set: false})
	}
	return results
}

func normalizeRoleName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	return strings.ToLower(strings.TrimSpace(name))
}

func memberHasRole(member *discordgo.Member, roleID string) bool {
	if member == nil {
		return false
	}
	for _, id := range member.Roles {
		if id == roleID {
			return true
		}
	}
	return false
}

func resolveGuildID(m *discordgo.MessageCreate, args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, "guild") {
			raw := strings.TrimPrefix(arg, "guild")
			raw = strings.Trim(raw, "<>#:=")
			raw = strings.TrimSpace(raw)
			if raw != "" {
				return raw
			}
		}
	}
	if m.GuildID != "" {
		return m.GuildID
	}
	return ""
}

func (d *Discord) fetchGuild(s *discordgo.Session, guildID string) (*discordgo.Guild, error) {
	if g, err := s.State.Guild(guildID); err == nil && g != nil {
		return g, nil
	}
	return s.Guild(guildID)
}

func emojiPrefix(emoji *discordgo.Emoji) string {
	if emoji != nil && emoji.Animated {
		return "a"
	}
	return ""
}

func getBotID(s *discordgo.Session) string {
	if s.State != nil && s.State.User != nil {
		return s.State.User.ID
	}
	user, err := s.User("@me")
	if err != nil || user == nil {
		return ""
	}
	return user.ID
}

func fetchEmojiData(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	mime := "image/png"
	if strings.HasSuffix(url, ".gif") {
		mime = "image/gif"
	}
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

func toInt(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.Atoi(strings.TrimSpace(string(v))); err == nil {
			return parsed
		}
	}
	return fallback
}
