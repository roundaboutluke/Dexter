package bot

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/community"
	"poraclego/internal/db"
	"poraclego/internal/logging"
	"poraclego/internal/util"
)

type discordUserInfo struct {
	name  string
	roles []string
}

func (d *Discord) loadDiscordUserInfo(s *discordgo.Session, userID string) *discordUserInfo {
	info, complete := d.loadDiscordUserInfoState(s, userID)
	if !complete {
		return nil
	}
	return info
}

func (d *Discord) loadDiscordUserInfoState(s *discordgo.Session, userID string) (*discordUserInfo, bool) {
	if s == nil || userID == "" || d.manager == nil || d.manager.cfg == nil {
		return nil, false
	}
	logger := logging.Get().Discord
	guilds, _ := d.manager.cfg.GetStringSlice("discord.guilds")
	info := discordUserInfo{}
	for _, guildID := range guilds {
		member, err := d.fetchGuildMemberByID(s, guildID, userID)
		if err != nil {
			fetch := classifyDiscordFetchError(err)
			if fetch.permanentNotFound {
				continue
			}
			if logger != nil {
				logger.Warnf("Reconciliation (Discord) Problem loading user %s from guild %s (%s): %v", userID, guildID, fetch.summary(), err)
			}
			return nil, false
		}
		if member == nil {
			if logger != nil {
				logger.Warnf("Reconciliation (Discord) Problem loading user %s from guild %s: nil member result", userID, guildID)
			}
			return nil, false
		}
		if member.User == nil || member.User.Bot {
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
		return nil, true
	}
	return &info, true
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

func (d *Discord) syncDiscordRole(s *discordgo.Session, registerNewUsers, syncNames, removeInvalidUsers bool) bool {
	users, err := d.manager.query.SelectAllQuery("humans", map[string]any{"type": "discord:user"})
	if err != nil {
		return false
	}
	logger := logging.Get().Discord
	admins, _ := d.manager.cfg.GetStringSlice("discord.admins")
	usersToCheck := []map[string]any{}
	for _, row := range users {
		if containsString(admins, getString(row["id"])) {
			continue
		}
		usersToCheck = append(usersToCheck, row)
	}
	discordUsers, complete := d.loadAllGuildUsers(s)
	if !complete {
		if logger != nil {
			logger.Warnf("Reconciliation (Discord) Skipping full user role sync due to incomplete guild/member load")
		}
		return false
	}
	checked := map[string]bool{}
	changed := false
	for _, row := range usersToCheck {
		id := getString(row["id"])
		checked[id] = true
		info, ok := discordUsers[id]
		if ok {
			changed = d.reconcileDiscordUser(id, row, &info, syncNames, removeInvalidUsers) || changed
		} else {
			changed = d.reconcileDiscordUser(id, row, nil, syncNames, removeInvalidUsers) || changed
		}
	}
	if registerNewUsers {
		for id, info := range discordUsers {
			if containsString(admins, id) || checked[id] {
				continue
			}
			changed = d.reconcileDiscordUser(id, nil, &info, syncNames, removeInvalidUsers) || changed
		}
	}
	return changed
}

func (d *Discord) loadAllGuildUsers(s *discordgo.Session) (map[string]discordUserInfo, bool) {
	userList := map[string]discordUserInfo{}
	logger := logging.Get().Discord
	guilds, _ := d.manager.cfg.GetStringSlice("discord.guilds")
	for _, guildID := range guilds {
		members, err := d.loadGuildMembers(s, guildID)
		if err != nil {
			if logger != nil {
				info := classifyDiscordFetchError(err)
				logger.Warnf("Reconciliation (Discord) Problem loading guild members for %s (%s): %v", guildID, info.summary(), err)
			}
			return nil, false
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
	return userList, true
}

func (d *Discord) loadGuildMembers(s *discordgo.Session, guildID string) ([]*discordgo.Member, error) {
	return d.fetchGuildMembersForGuild(s, guildID)
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
	info, complete := d.loadDiscordUserInfoState(s, userID)
	if !complete {
		return
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		return
	}
	changed := false
	if info == nil {
		changed = d.reconcileDiscordUser(userID, human, nil, false, removeInvalidUsers)
	} else {
		changed = d.reconcileDiscordUser(userID, human, info, false, removeInvalidUsers)
	}
	if changed {
		d.manager.RefreshAlertState()
	}
}

func (d *Discord) reconcileDiscordUser(id string, user map[string]any, info *discordUserInfo, syncNames, removeInvalidUsers bool) bool {
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
			return false
		}
		before := user != nil && toInt(user["admin_disable"], 0) == 0
		after := hasAnyRole(roleList, requiredRoles)
		if !before && after {
			if user == nil {
				changedRows, err := d.manager.query.InsertOrUpdateQuery("humans", map[string]any{
					"id":                   id,
					"type":                 "discord:user",
					"name":                 name,
					"area":                 "[]",
					"community_membership": "[]",
					"blocked_alerts":       blocked,
				})
				if err != nil {
					return false
				}
				d.sendGreetingsDiscord(id)
				return changedRows > 0
			}
			if toInt(user["admin_disable"], 0) == 1 && hasDisabledDate(user) {
				changedRows, err := d.manager.query.UpdateQuery("humans", map[string]any{
					"admin_disable":  0,
					"disabled_date":  nil,
					"blocked_alerts": blocked,
				}, map[string]any{"id": id})
				if err != nil {
					return false
				}
				d.sendGreetingsDiscord(id)
				return changedRows > 0
			}
		}
		if before && !after && removeInvalidUsers {
			return d.disableUser(user)
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
				if changedRows, err := d.manager.query.UpdateQuery("humans", updates, map[string]any{"id": id}); err == nil {
					return changedRows > 0
				}
			}
		}
		return false
	}

	communityList := d.communitiesForRoles(roleList)
	before := user != nil && toInt(user["admin_disable"], 0) == 0
	after := len(communityList) > 0
	areaRestriction := community.CalculateLocationRestrictions(d.manager.cfg, communityList)
	if !before && after {
		if user == nil {
			changedRows, err := d.manager.query.InsertOrUpdateQuery("humans", map[string]any{
				"id":                   id,
				"type":                 "discord:user",
				"name":                 name,
				"area":                 "[]",
				"area_restriction":     toJSON(areaRestriction),
				"community_membership": toJSON(communityList),
				"blocked_alerts":       blocked,
			})
			if err != nil {
				return false
			}
			d.sendGreetingsDiscord(id)
			return changedRows > 0
		}
		if toInt(user["admin_disable"], 0) == 1 && hasDisabledDate(user) {
			changedRows, err := d.manager.query.UpdateQuery("humans", map[string]any{
				"admin_disable":        0,
				"disabled_date":        nil,
				"area_restriction":     toJSON(areaRestriction),
				"community_membership": toJSON(communityList),
				"blocked_alerts":       blocked,
			}, map[string]any{"id": id})
			if err != nil {
				return false
			}
			d.sendGreetingsDiscord(id)
			return changedRows > 0
		}
	}
	if before && !after && removeInvalidUsers {
		return d.disableUser(user)
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
			if changedRows, err := d.manager.query.UpdateQuery("humans", updates, map[string]any{"id": id}); err == nil {
				return changedRows > 0
			}
		}
	}
	return false
}

func (d *Discord) disableUser(user map[string]any) bool {
	mode, _ := d.manager.cfg.GetString("general.roleCheckMode")
	id := getString(user["id"])
	switch mode {
	case "disable-user":
		if toInt(user["admin_disable"], 0) == 0 {
			changedRows, _ := d.manager.query.UpdateQuery("humans", map[string]any{
				"admin_disable": 1,
				"disabled_date": time.Now(),
			}, map[string]any{"id": id})
			d.removeSubscribedRoles(d.session, id)
			d.sendLostRoleMessage(d.session, id)
			return changedRows > 0
		}
	case "delete":
		changed := false
		if err := d.manager.withQueryTx(func(query *db.Query) error {
			trackingChanged, err := d.removeUserTracking(query, id)
			if err != nil {
				return err
			}
			changed = trackingChanged
			if removed, err := query.DeleteQuery("profiles", map[string]any{"id": id}); err == nil && removed > 0 {
				changed = true
			} else if err != nil {
				return err
			}
			if removed, err := query.DeleteQuery("humans", map[string]any{"id": id}); err == nil && removed > 0 {
				changed = true
			} else if err != nil {
				return err
			}
			return nil
		}); err != nil {
			return false
		}
		d.removeSubscribedRoles(d.session, id)
		d.sendLostRoleMessage(d.session, id)
		return changed
	}
	return false
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

var getString = util.GetString

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
