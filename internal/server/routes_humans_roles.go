package server

import (
	"net/http"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"

	"dexter/internal/config"
	"dexter/internal/logging"
)

type roleEntry struct {
	Description string `json:"description"`
	ID          string `json:"id"`
	Set         bool   `json:"set"`
}

type roleGroup struct {
	Exclusive [][]roleEntry `json:"exclusive"`
	General   []roleEntry   `json:"general"`
}

type guildRoleList struct {
	Name  string    `json:"name"`
	Roles roleGroup `json:"roles"`
}

type roleConfig struct {
	general   map[string]string
	exclusive []map[string]string
}

type channelInfo struct {
	id         string
	categoryID string
}

func handleHumanRoles(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	if getString(human["type"]) != "discord:user" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	session := discordSession(s)
	if session == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	configs := parseDiscordRoleConfig(s.cfg)
	if len(configs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"guilds": []any{},
		})
		return
	}

	out := []guildRoleList{}
	for guildID, entry := range configs {
		guild, err := session.Guild(guildID)
		if err != nil || guild == nil {
			continue
		}
		member, err := session.GuildMember(guildID, id)
		if err != nil || member == nil {
			continue
		}

		roleGroup := roleGroup{
			Exclusive: [][]roleEntry{},
			General:   []roleEntry{},
		}

		for _, set := range entry.exclusive {
			row := []roleEntry{}
			for desc, roleID := range set {
				row = append(row, roleEntry{
					Description: desc,
					ID:          roleID,
					Set:         memberHasRole(member, roleID),
				})
			}
			roleGroup.Exclusive = append(roleGroup.Exclusive, row)
		}
		for desc, roleID := range entry.general {
			roleGroup.General = append(roleGroup.General, roleEntry{
				Description: desc,
				ID:          roleID,
				Set:         memberHasRole(member, roleID),
			})
		}
		out = append(out, guildRoleList{
			Name:  guild.Name,
			Roles: roleGroup,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"guilds": out,
	})
}

func handleHumanRoleUpdate(w http.ResponseWriter, s *Server, id, roleID string, set bool) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	if getString(human["type"]) != "discord:user" {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	session := discordSession(s)
	if session == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	configs := parseDiscordRoleConfig(s.cfg)
	if len(configs) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"result": []any{},
		})
		return
	}

	changes := []roleEntry{}
	for guildID, entry := range configs {
		member, err := session.GuildMember(guildID, id)
		if err != nil || member == nil {
			continue
		}
		for desc, idValue := range entry.general {
			if idValue != roleID {
				continue
			}
			if set {
				_ = session.GuildMemberRoleAdd(guildID, id, roleID)
				changes = append(changes, roleEntry{Description: desc, ID: roleID, Set: true})
			} else {
				_ = session.GuildMemberRoleRemove(guildID, id, roleID)
				changes = append(changes, roleEntry{Description: desc, ID: roleID, Set: false})
			}
		}
		for _, setRoles := range entry.exclusive {
			desc, matched := roleDescByID(setRoles, roleID)
			if !matched {
				continue
			}
			if set {
				for roleDesc, rid := range setRoles {
					if rid == roleID {
						_ = session.GuildMemberRoleAdd(guildID, id, rid)
						changes = append(changes, roleEntry{Description: roleDesc, ID: rid, Set: true})
						continue
					}
					if memberHasRole(member, rid) {
						_ = session.GuildMemberRoleRemove(guildID, id, rid)
						changes = append(changes, roleEntry{Description: roleDesc, ID: rid, Set: false})
					}
				}
			} else {
				_ = session.GuildMemberRoleRemove(guildID, id, roleID)
				changes = append(changes, roleEntry{Description: desc, ID: roleID, Set: false})
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"result": changes,
	})
}

func handleHumanAdministrationRoles(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}

	result := map[string]any{}

	if enabled, _ := s.cfg.GetBool("discord.enabled"); enabled {
		admin := map[string]any{
			"channels": []string{},
			"webhooks": []string{},
			"users":    false,
		}
		session := discordSession(s)
		if session == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}

		roles := discordUserRoles(session, s.cfg, id)
		rolesAndID := append(append([]string{}, roles...), id)

		channelTracking, _ := s.cfg.Get("discord.delegatedAdministration.channelTracking")
		if entries, ok := channelTracking.(map[string]any); ok && len(entries) > 0 {
			channels := discordChannels(session, s.cfg)
			allowedChannels := map[string]bool{}
			for key, raw := range entries {
				if !containsAny(raw, rolesAndID) {
					continue
				}
				if guildChannels, ok := channels[key]; ok {
					for _, ch := range guildChannels {
						allowedChannels[ch.id] = true
					}
					continue
				}
				for _, guildChannels := range channels {
					for _, ch := range guildChannels {
						if ch.categoryID == key || ch.id == key {
							allowedChannels[ch.id] = true
						}
					}
				}
			}
			admin["channels"] = sortedKeys(allowedChannels)
		}

		webhookTracking, _ := s.cfg.Get("discord.delegatedAdministration.webhookTracking")
		if entries, ok := webhookTracking.(map[string]any); ok && len(entries) > 0 {
			allowedWebhooks := map[string]bool{}
			for key, raw := range entries {
				if containsAny(raw, rolesAndID) {
					allowedWebhooks[key] = true
				}
			}
			admin["webhooks"] = sortedKeys(allowedWebhooks)
		}

		userTracking, _ := s.cfg.Get("discord.delegatedAdministration.userTracking")
		if containsAny(userTracking, rolesAndID) {
			admin["users"] = true
		}

		result["discord"] = admin
	}

	if enabled, _ := s.cfg.GetBool("telegram.enabled"); enabled {
		admin := map[string]any{
			"channels": []string{},
			"users":    false,
		}

		channelTracking, _ := s.cfg.Get("telegram.delegatedAdministration.channelTracking")
		if entries, ok := channelTracking.(map[string]any); ok && len(entries) > 0 {
			allowedChannels := map[string]bool{}
			for key, raw := range entries {
				if containsAny(raw, []string{id}) {
					allowedChannels[key] = true
				}
			}
			admin["channels"] = sortedKeys(allowedChannels)
		}

		userTracking, _ := s.cfg.Get("telegram.delegatedAdministration.userTracking")
		if containsAny(userTracking, []string{id}) {
			admin["users"] = true
		}

		result["telegram"] = admin
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"admin":  result,
	})
}

func discordSession(s *Server) *discordgo.Session {
	if s == nil || s.botManager == nil {
		return nil
	}
	return s.botManager.DiscordSession()
}

func parseDiscordRoleConfig(cfg *config.Config) map[string]roleConfig {
	raw, ok := cfg.Get("discord.userRoleSubscription")
	if !ok {
		return map[string]roleConfig{}
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return map[string]roleConfig{}
	}
	out := map[string]roleConfig{}
	for guildID, entry := range entries {
		values, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		general := parseRoleMap(values["roles"])
		exclusive := []map[string]string{}
		switch rawExclusive := values["exclusiveRoles"].(type) {
		case []any:
			for _, item := range rawExclusive {
				if roleMap, ok := item.(map[string]any); ok {
					exclusive = append(exclusive, parseRoleMap(roleMap))
				}
			}
		case map[string]any:
			exclusive = append(exclusive, parseRoleMap(rawExclusive))
		}
		out[guildID] = roleConfig{general: general, exclusive: exclusive}
	}
	return out
}

func parseRoleMap(raw any) map[string]string {
	out := map[string]string{}
	values, ok := raw.(map[string]any)
	if !ok {
		return out
	}
	for desc, id := range values {
		out[desc] = getString(id)
	}
	return out
}

func roleDescByID(roles map[string]string, roleID string) (string, bool) {
	for desc, id := range roles {
		if id == roleID {
			return desc, true
		}
	}
	return "", false
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

func discordUserRoles(session *discordgo.Session, cfg *config.Config, userID string) []string {
	if session == nil || cfg == nil || userID == "" {
		return []string{}
	}
	guilds, _ := cfg.GetStringSlice("discord.guilds")
	roleSet := map[string]bool{}
	for _, guildID := range guilds {
		member, err := session.GuildMember(guildID, userID)
		if err != nil || member == nil {
			continue
		}
		for _, roleID := range member.Roles {
			roleSet[roleID] = true
		}
	}
	return sortedKeys(roleSet)
}

func discordChannels(session *discordgo.Session, cfg *config.Config) map[string][]channelInfo {
	out := map[string][]channelInfo{}
	if session == nil || cfg == nil {
		return out
	}
	guilds, _ := cfg.GetStringSlice("discord.guilds")
	for _, guildID := range guilds {
		channels, err := session.GuildChannels(guildID)
		if err != nil {
			logger := logging.Get().Discord
			if logger != nil {
				logger.Errorf("Get channels (Discord) error: %v", err)
			}
			continue
		}
		list := []channelInfo{}
		for _, ch := range channels {
			if ch == nil {
				continue
			}
			if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews {
				continue
			}
			list = append(list, channelInfo{id: ch.ID, categoryID: ch.ParentID})
		}
		out[guildID] = list
	}
	return out
}

func containsAny(raw any, targets []string) bool {
	values := toStringSlice(raw)
	if len(values) == 0 || len(targets) == 0 {
		return false
	}
	for _, target := range targets {
		value := strings.ToLower(target)
		if containsString(values, value) {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]bool) []string {
	if len(values) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
