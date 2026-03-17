package bot

import (
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/alertstate"
	"poraclego/internal/db"
	"poraclego/internal/logging"
)

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
	info, complete := d.loadDiscordUserInfoState(s, userID)
	if !complete {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Reconciliation (Discord) Skipping single-user reconcile for %s due to incomplete guild/member load", userID)
		}
		return
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		return
	}
	if info == nil {
		if d.reconcileDiscordUser(userID, human, nil, false, removeInvalid) {
			d.manager.RefreshAlertState()
		}
		return
	}
	if d.reconcileDiscordUser(userID, human, info, false, removeInvalid) {
		d.manager.RefreshAlertState()
	}
}

func (d *Discord) removeUserTracking(query *db.Query, id string) (bool, error) {
	changed := false
	for _, table := range alertstate.TrackedTables() {
		if removed, err := query.DeleteQuery(table, map[string]any{"id": id}); err == nil && removed > 0 {
			changed = true
		} else if err != nil {
			return false, err
		}
	}
	return changed, nil
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
		member, err := d.fetchGuildMemberByID(s, guildID, userID)
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
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				d.runReconciliation(s)
			case <-d.stopCh:
				return
			}
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

	changed := false
	if updateChannelNames || updateChannelNotes || unregisterMissingChannels {
		changed = d.syncDiscordChannels(s, updateChannelNames, updateChannelNotes, unregisterMissingChannels) || changed
	}
	if updateNames || removeInvalid || registerNew {
		changed = d.syncDiscordRole(s, registerNew, updateNames, removeInvalid) || changed
	}
	if changed {
		d.manager.RefreshAlertState()
	}
}
