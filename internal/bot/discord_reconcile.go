package bot

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/community"
	"poraclego/internal/config"
	"poraclego/internal/dts"
	"poraclego/internal/logging"
	"poraclego/internal/render"
)

func (d *Discord) onChannelDelete(s *discordgo.Session, ev *discordgo.ChannelDelete) {
	if ev == nil || ev.Channel == nil || d.manager == nil || d.manager.query == nil {
		return
	}
	channelID := ev.Channel.ID
	if channelID == "" {
		return
	}
	count, err := d.manager.query.CountQuery("humans", map[string]any{"id": channelID})
	if err != nil || count == 0 {
		return
	}
	d.removeUserTracking(channelID)
	_, _ = d.manager.query.DeleteQuery("humans", map[string]any{"id": channelID})
}

func (d *Discord) onGuildMemberRemove(s *discordgo.Session, ev *discordgo.GuildMemberRemove) {
	if ev == nil || ev.User == nil || ev.User.Bot {
		return
	}
	removeInvalid, _ := d.manager.cfg.GetBool("reconciliation.discord.removeInvalidUsers")
	d.reconcileSingleUser(s, ev.User.ID, removeInvalid)
}

func (d *Discord) onGuildMemberUpdate(s *discordgo.Session, ev *discordgo.GuildMemberUpdate) {
	if ev == nil || ev.Member == nil || ev.User == nil || ev.User.Bot {
		return
	}
	removeInvalid, _ := d.manager.cfg.GetBool("reconciliation.discord.removeInvalidUsers")
	d.reconcileSingleUser(s, ev.User.ID, removeInvalid)
}

func (d *Discord) reconcileUser(s *discordgo.Session, userID string) {
	if userID == "" || d.manager == nil || d.manager.cfg == nil || d.manager.query == nil {
		return
	}
	if containsID(d.manager.cfg, "discord.admins", userID) {
		return
	}
	removeInvalid, _ := d.manager.cfg.GetBool("reconciliation.discord.removeInvalidUsers")
	info := d.loadDiscordUserInfo(s, userID)
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		return
	}
	if info == nil {
		d.reconcileDiscordUser(userID, human, nil, false, removeInvalid)
		return
	}
	d.reconcileDiscordUser(userID, human, info, false, removeInvalid)
}

func (d *Discord) loadDiscordUserInfo(s *discordgo.Session, userID string) *discordUserInfo {
	if s == nil || userID == "" || d.manager == nil || d.manager.cfg == nil {
		return nil
	}
	guilds, _ := d.manager.cfg.GetStringSlice("discord.guilds")
	info := discordUserInfo{}
	for _, guildID := range guilds {
		member, err := s.GuildMember(guildID, userID)
		if err != nil || member == nil || member.User == nil || member.User.Bot {
			continue
		}
		if info.name == "" {
			if member.Nick != "" {
				info.name = stripNonASCII(member.Nick)
			} else {
				info.name = stripNonASCII(member.User.Username)
			}
		}
		info.roles = append(info.roles, member.Roles...)
	}
	if info.name == "" && len(info.roles) == 0 {
		return nil
	}
	return &info
}

func (d *Discord) userHasAccess(s *discordgo.Session, userID string) bool {
	guilds, ok := d.manager.cfg.GetStringSlice("discord.guilds")
	if !ok || len(guilds) == 0 {
		return true
	}
	requiredRoles, _ := d.manager.cfg.GetStringSlice("discord.userRole")
	requireRole := len(requiredRoles) > 0
	for _, guildID := range guilds {
		member, err := s.GuildMember(guildID, userID)
		if err != nil || member == nil {
			continue
		}
		if !requireRole {
			return true
		}
		for _, roleID := range member.Roles {
			for _, required := range requiredRoles {
				if roleID == required {
					return true
				}
			}
		}
	}
	return false
}

func (d *Discord) removeUserTracking(id string) {
	_, _ = d.manager.query.DeleteQuery("egg", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("monsters", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("raid", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("quest", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("lures", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("gym", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("invasion", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("nests", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("forts", map[string]any{"id": id})
	_, _ = d.manager.query.DeleteQuery("weather", map[string]any{"id": id})
}

func (d *Discord) removeSubscribedRoles(s *discordgo.Session, userID string) {
	raw, ok := d.manager.cfg.Get("discord.userRoleSubscription")
	if !ok {
		return
	}
	rolesByGuild, ok := raw.(map[string]any)
	if !ok {
		return
	}
	for guildID, entry := range rolesByGuild {
		member, err := s.GuildMember(guildID, userID)
		if err != nil || member == nil {
			continue
		}
		roleSets := parseRoleSets(entry)
		for _, role := range roleSets.general {
			_ = s.GuildMemberRoleRemove(guildID, userID, role.id)
		}
		for _, set := range roleSets.exclusive {
			for _, role := range set {
				_ = s.GuildMemberRoleRemove(guildID, userID, role.id)
			}
		}
	}
}

func (d *Discord) sendLostRoleMessage(s *discordgo.Session, userID string) {
	msg, ok := d.manager.cfg.GetString("discord.lostRoleMessage")
	if !ok || msg == "" {
		return
	}
	ch, err := s.UserChannelCreate(userID)
	if err != nil {
		return
	}
	_, _ = s.ChannelMessageSend(ch.ID, msg)
}

func (d *Discord) startReconciliation(s *discordgo.Session) {
	enabled, _ := d.manager.cfg.GetBool("discord.checkRole")
	intervalHours, _ := d.manager.cfg.GetInt("discord.checkRoleInterval")
	if !enabled || intervalHours <= 0 {
		return
	}
	guilds, _ := d.manager.cfg.GetStringSlice("discord.guilds")
	if len(guilds) == 0 {
		return
	}
	time.AfterFunc(10*time.Second, func() {
		d.runReconciliation(s)
	})
	ticker := time.NewTicker(time.Duration(intervalHours) * time.Hour)
	go func() {
		for range ticker.C {
			d.runReconciliation(s)
		}
	}()
}

func (d *Discord) runReconciliation(s *discordgo.Session) {
	updateNames, _ := d.manager.cfg.GetBool("reconciliation.discord.updateUserNames")
	removeInvalid, _ := d.manager.cfg.GetBool("reconciliation.discord.removeInvalidUsers")
	registerNew, _ := d.manager.cfg.GetBool("reconciliation.discord.registerNewUsers")
	updateChannelNames, _ := d.manager.cfg.GetBool("reconciliation.discord.updateChannelNames")
	updateChannelNotes, _ := d.manager.cfg.GetBool("reconciliation.discord.updateChannelNotes")
	unregisterMissingChannels, _ := d.manager.cfg.GetBool("reconciliation.discord.unregisterMissingChannels")

	if updateChannelNames || updateChannelNotes || unregisterMissingChannels {
		d.syncDiscordChannels(s, updateChannelNames, updateChannelNotes, unregisterMissingChannels)
	}
	if updateNames || removeInvalid || registerNew {
		d.syncDiscordRole(s, registerNew, updateNames, removeInvalid)
	}
}

func (d *Discord) syncDiscordChannels(s *discordgo.Session, syncNames, syncNotes, removeInvalid bool) {
	rows, err := d.manager.query.SelectAllQuery("humans", map[string]any{
		"type":          "discord:channel",
		"admin_disable": 0,
	})
	if err != nil {
		return
	}
	for _, row := range rows {
		channelID := getString(row["id"])
		if channelID == "" {
			continue
		}
		channel, err := s.Channel(channelID)
		if err != nil || channel == nil {
			if removeInvalid {
				_, _ = d.manager.query.UpdateQuery("humans", map[string]any{
					"admin_disable": 1,
					"disabled_date": time.Now(),
				}, map[string]any{"id": channelID})
			}
			continue
		}
		updates := map[string]any{}
		if syncNames && getString(row["name"]) != channel.Name {
			updates["name"] = channel.Name
		}
		if syncNotes {
			notes := d.channelNotes(s, channel)
			if getString(row["notes"]) != notes {
				updates["notes"] = notes
			}
		}
		if row["area_restriction"] != nil && row["community_membership"] != nil {
			communities := parseStringList(row["community_membership"])
			restriction := community.CalculateLocationRestrictions(d.manager.cfg, communities)
			if !sameContents(parseStringList(row["area_restriction"]), restriction) {
				updates["area_restriction"] = toJSON(restriction)
			}
		}
		if len(updates) > 0 {
			_, _ = d.manager.query.UpdateQuery("humans", updates, map[string]any{"id": channelID})
		}
	}
}

func (d *Discord) channelNotes(s *discordgo.Session, channel *discordgo.Channel) string {
	if channel == nil {
		return ""
	}
	guild, err := s.Guild(channel.GuildID)
	if err != nil || guild == nil {
		return ""
	}
	notes := guild.Name
	if channel.ParentID != "" {
		if parent, err := s.Channel(channel.ParentID); err == nil && parent != nil {
			notes = notes + " / " + parent.Name
		}
	}
	return notes
}

type discordUserInfo struct {
	name  string
	roles []string
}

func (d *Discord) syncDiscordRole(s *discordgo.Session, registerNewUsers, syncNames, removeInvalidUsers bool) {
	users, err := d.manager.query.SelectAllQuery("humans", map[string]any{"type": "discord:user"})
	if err != nil {
		return
	}
	admins, _ := d.manager.cfg.GetStringSlice("discord.admins")
	usersToCheck := []map[string]any{}
	for _, row := range users {
		if containsString(admins, getString(row["id"])) {
			continue
		}
		usersToCheck = append(usersToCheck, row)
	}
	discordUsers := d.loadAllGuildUsers(s)
	checked := map[string]bool{}
	for _, row := range usersToCheck {
		id := getString(row["id"])
		checked[id] = true
		info, ok := discordUsers[id]
		if ok {
			d.reconcileDiscordUser(id, row, &info, syncNames, removeInvalidUsers)
		} else {
			d.reconcileDiscordUser(id, row, nil, syncNames, removeInvalidUsers)
		}
	}
	if registerNewUsers {
		for id, info := range discordUsers {
			if containsString(admins, id) || checked[id] {
				continue
			}
			d.reconcileDiscordUser(id, nil, &info, syncNames, removeInvalidUsers)
		}
	}
}

func (d *Discord) loadAllGuildUsers(s *discordgo.Session) map[string]discordUserInfo {
	userList := map[string]discordUserInfo{}
	guilds, _ := d.manager.cfg.GetStringSlice("discord.guilds")
	for _, guildID := range guilds {
		members, err := d.loadGuildMembers(s, guildID)
		if err != nil {
			continue
		}
		for _, member := range members {
			if member == nil || member.User == nil || member.User.Bot {
				continue
			}
			name := member.Nick
			if name == "" {
				name = member.User.Username
			}
			name = stripNonASCII(name)
			info := userList[member.User.ID]
			if info.name == "" {
				info.name = name
			}
			info.roles = append(info.roles, member.Roles...)
			userList[member.User.ID] = info
		}
	}
	return userList
}

func (d *Discord) loadGuildMembers(s *discordgo.Session, guildID string) ([]*discordgo.Member, error) {
	return d.loadGuildMembersREST(s, guildID)
}

func (d *Discord) loadGuildMembersREST(s *discordgo.Session, guildID string) ([]*discordgo.Member, error) {
	if s == nil {
		return nil, fmt.Errorf("discord session missing")
	}
	after := ""
	out := []*discordgo.Member{}
	for {
		members, err := s.GuildMembers(guildID, after, 1000)
		if err != nil {
			return out, err
		}
		if len(members) == 0 {
			break
		}
		lastID := ""
		for _, member := range members {
			if member == nil || member.User == nil {
				continue
			}
			lastID = member.User.ID
			out = append(out, member)
		}
		if lastID == "" {
			break
		}
		after = lastID
		if len(members) < 1000 {
			break
		}
	}
	return out, nil
}

func (d *Discord) reconcileSingleUser(s *discordgo.Session, userID string, removeInvalidUsers bool) {
	if userID == "" || d.manager == nil || d.manager.cfg == nil || d.manager.query == nil {
		return
	}
	if containsID(d.manager.cfg, "discord.admins", userID) {
		return
	}
	guilds, _ := d.manager.cfg.GetStringSlice("discord.guilds")
	if len(guilds) == 0 {
		return
	}
	info := discordUserInfo{}
	for _, guildID := range guilds {
		member, err := s.GuildMember(guildID, userID)
		if err != nil || member == nil || member.User == nil || member.User.Bot {
			continue
		}
		if info.name == "" {
			if member.Nick != "" {
				info.name = stripNonASCII(member.Nick)
			} else {
				info.name = stripNonASCII(member.User.Username)
			}
		}
		info.roles = append(info.roles, member.Roles...)
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		return
	}
	if info.name == "" && len(info.roles) == 0 {
		d.reconcileDiscordUser(userID, human, nil, false, removeInvalidUsers)
		return
	}
	d.reconcileDiscordUser(userID, human, &info, false, removeInvalidUsers)
}

func (d *Discord) reconcileDiscordUser(id string, user map[string]any, info *discordUserInfo, syncNames, removeInvalidUsers bool) {
	roleList := []string{}
	name := ""
	if info != nil {
		roleList = info.roles
		name = info.name
	}
	blocked := d.blockedAlerts(id, roleList)
	if enabled, _ := d.manager.cfg.GetBool("areaSecurity.enabled"); !enabled {
		requiredRoles, _ := d.manager.cfg.GetStringSlice("discord.userRole")
		if len(requiredRoles) == 0 {
			return
		}
		before := user != nil && toInt(user["admin_disable"], 0) == 0
		after := hasAnyRole(roleList, requiredRoles)
		if !before && after {
			if user == nil {
				_, err := d.manager.query.InsertOrUpdateQuery("humans", map[string]any{
					"id":                   id,
					"type":                 "discord:user",
					"name":                 name,
					"area":                 "[]",
					"community_membership": "[]",
					"blocked_alerts":       blocked,
				})
				if err != nil {
					return
				}
				d.sendGreetingsDiscord(id)
			} else if toInt(user["admin_disable"], 0) == 1 && hasDisabledDate(user) {
				_, err := d.manager.query.UpdateQuery("humans", map[string]any{
					"admin_disable":  0,
					"disabled_date":  nil,
					"blocked_alerts": blocked,
				}, map[string]any{"id": id})
				if err != nil {
					return
				}
				d.sendGreetingsDiscord(id)
			}
		}
		if before && !after && removeInvalidUsers {
			d.disableUser(user)
		}
		if before && after {
			updates := map[string]any{}
			if syncNames && getString(user["name"]) != name {
				updates["name"] = name
			}
			if getString(user["blocked_alerts"]) != blocked {
				updates["blocked_alerts"] = blocked
			}
			if len(updates) > 0 {
				_, _ = d.manager.query.UpdateQuery("humans", updates, map[string]any{"id": id})
			}
		}
		return
	}

	communityList := d.communitiesForRoles(roleList)
	before := user != nil && toInt(user["admin_disable"], 0) == 0
	after := len(communityList) > 0
	areaRestriction := community.CalculateLocationRestrictions(d.manager.cfg, communityList)
	if !before && after {
		if user == nil {
			_, err := d.manager.query.InsertOrUpdateQuery("humans", map[string]any{
				"id":                   id,
				"type":                 "discord:user",
				"name":                 name,
				"area":                 "[]",
				"area_restriction":     toJSON(areaRestriction),
				"community_membership": toJSON(communityList),
				"blocked_alerts":       blocked,
			})
			if err != nil {
				return
			}
			d.sendGreetingsDiscord(id)
		} else if toInt(user["admin_disable"], 0) == 1 && hasDisabledDate(user) {
			_, err := d.manager.query.UpdateQuery("humans", map[string]any{
				"admin_disable":        0,
				"disabled_date":        nil,
				"area_restriction":     toJSON(areaRestriction),
				"community_membership": toJSON(communityList),
				"blocked_alerts":       blocked,
			}, map[string]any{"id": id})
			if err != nil {
				return
			}
			d.sendGreetingsDiscord(id)
		}
	}
	if before && !after && removeInvalidUsers {
		d.disableUser(user)
	}
	if before && after {
		updates := map[string]any{}
		if syncNames && getString(user["name"]) != name {
			updates["name"] = name
		}
		if getString(user["blocked_alerts"]) != blocked {
			updates["blocked_alerts"] = blocked
		}
		if !sameContents(parseStringList(user["area_restriction"]), areaRestriction) {
			updates["area_restriction"] = toJSON(areaRestriction)
		}
		if !sameContents(parseStringList(user["community_membership"]), communityList) {
			updates["community_membership"] = toJSON(communityList)
		}
		if len(updates) > 0 {
			_, _ = d.manager.query.UpdateQuery("humans", updates, map[string]any{"id": id})
		}
	}
}

func (d *Discord) disableUser(user map[string]any) {
	mode, _ := d.manager.cfg.GetString("general.roleCheckMode")
	id := getString(user["id"])
	switch mode {
	case "disable-user":
		if toInt(user["admin_disable"], 0) == 0 {
			_, _ = d.manager.query.UpdateQuery("humans", map[string]any{
				"admin_disable": 1,
				"disabled_date": time.Now(),
			}, map[string]any{"id": id})
			d.removeSubscribedRoles(d.session, id)
			d.sendLostRoleMessage(d.session, id)
		}
	case "delete":
		d.removeUserTracking(id)
		_, _ = d.manager.query.DeleteQuery("profiles", map[string]any{"id": id})
		_, _ = d.manager.query.DeleteQuery("humans", map[string]any{"id": id})
		d.removeSubscribedRoles(d.session, id)
		d.sendLostRoleMessage(d.session, id)
	}
}

func (d *Discord) blockedAlerts(userID string, roles []string) string {
	raw, ok := d.manager.cfg.Get("discord.commandSecurity")
	if !ok {
		return ""
	}
	permissions, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	commands := []string{"raid", "monster", "gym", "specificgym", "lure", "nest", "egg", "invasion", "pvp"}
	blocked := []string{}
	for _, cmd := range commands {
		rawPerm, ok := permissions[cmd]
		if !ok {
			continue
		}
		allowed := containsAny(rawPerm, userID, roles)
		if !allowed {
			blocked = append(blocked, cmd)
		}
	}
	if len(blocked) == 0 {
		return ""
	}
	payload, _ := json.Marshal(blocked)
	return string(payload)
}

func containsAny(raw any, userID string, roles []string) bool {
	switch v := raw.(type) {
	case []any:
		for _, entry := range v {
			value := getString(entry)
			if value == userID {
				return true
			}
			for _, role := range roles {
				if value == role {
					return true
				}
			}
		}
	case []string:
		for _, entry := range v {
			if entry == userID {
				return true
			}
			for _, role := range roles {
				if entry == role {
					return true
				}
			}
		}
	}
	return false
}

func (d *Discord) communitiesForRoles(roles []string) []string {
	raw, ok := d.manager.cfg.Get("areaSecurity.communities")
	if !ok {
		return []string{}
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return []string{}
	}
	result := []string{}
	for name, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		discord, ok := entryMap["discord"].(map[string]any)
		if !ok {
			continue
		}
		userRoles, ok := discord["userRole"].([]any)
		if !ok {
			continue
		}
		for _, role := range userRoles {
			roleID := getString(role)
			if containsString(roles, roleID) {
				result = append(result, strings.ToLower(name))
				break
			}
		}
	}
	sort.Strings(result)
	return result
}

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

func getString(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprintf("%v", value)
	}
}

func hasDisabledDate(user map[string]any) bool {
	if user == nil {
		return false
	}
	value, ok := user["disabled_date"]
	if !ok || value == nil {
		return false
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value)) != ""
}

func getStringOr(cfg *config.Config, path, fallback string) string {
	if value, ok := cfg.GetString(path); ok && value != "" {
		return value
	}
	return fallback
}

func parseStringList(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return []string{}
		}
		var items []string
		if err := json.Unmarshal([]byte(v), &items); err == nil {
			return items
		}
	case []any:
		items := []string{}
		for _, item := range v {
			items = append(items, getString(item))
		}
		return items
	case []string:
		return v
	}
	return []string{}
}

func sameContents(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	counts := map[string]int{}
	for _, item := range a {
		counts[item]++
	}
	for _, item := range b {
		if counts[item] == 0 {
			return false
		}
		counts[item]--
	}
	return true
}

func hasAnyRole(userRoles, required []string) bool {
	for _, role := range userRoles {
		if containsString(required, role) {
			return true
		}
	}
	return false
}
