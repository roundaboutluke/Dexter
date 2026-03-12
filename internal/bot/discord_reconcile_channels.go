package bot

import (
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/community"
	"poraclego/internal/db"
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
	changed := false
	if err := d.manager.withQueryTx(func(query *db.Query) error {
		trackingChanged, err := d.removeUserTracking(query, channelID)
		if err != nil {
			return err
		}
		changed = trackingChanged
		if removed, err := query.DeleteQuery("humans", map[string]any{"id": channelID}); err == nil && removed > 0 {
			changed = true
		} else if err != nil {
			return err
		}
		return nil
	}); err != nil {
		return
	}
	if changed {
		d.manager.RefreshAlertState()
	}
}

func (d *Discord) syncDiscordChannels(s *discordgo.Session, syncNames, syncNotes, removeInvalid bool) bool {
	rows, err := d.manager.query.SelectAllQuery("humans", map[string]any{
		"type":          "discord:channel",
		"admin_disable": 0,
	})
	if err != nil {
		return false
	}
	changed := false
	for _, row := range rows {
		channelID := getString(row["id"])
		if channelID == "" {
			continue
		}
		channel, err := s.Channel(channelID)
		if err != nil || channel == nil {
			if removeInvalid {
				if updated, err := d.manager.query.UpdateQuery("humans", map[string]any{
					"admin_disable": 1,
					"disabled_date": time.Now(),
				}, map[string]any{"id": channelID}); err == nil && updated > 0 {
					changed = true
				}
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
			if updated, err := d.manager.query.UpdateQuery("humans", updates, map[string]any{"id": channelID}); err == nil && updated > 0 {
				changed = true
			}
		}
	}
	return changed
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
