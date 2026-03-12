package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) handleSlashRemove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	trackType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(trackType) == "" {
		d.respondEphemeral(s, i, "Please pick a tracking type.")
		return
	}
	value, ok := optionString(options, "tracking")
	if !ok || strings.TrimSpace(value) == "" {
		d.respondEphemeral(s, i, "Please pick a tracking entry.")
		return
	}

	trackingType, uid := parseRemoveSelection(trackType, value)
	if trackingType == "" || uid == "" {
		d.respondEphemeral(s, i, "That tracking entry could not be parsed.")
		return
	}
	if strings.Contains(value, "|") {
		expected := strings.ToLower(strings.TrimSpace(trackType))
		if expected == "incident" {
			expected = "invasion"
		}
		if expected != "" && trackingType != expected {
			d.respondEphemeral(s, i, "Tracking type changed; please clear the tracking selection and pick again.")
			return
		}
	}
	table := removeTrackingTable(trackingType)
	if table == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "That tracking entry could not be removed.")
		return
	}

	profileToken, _ := optionString(options, "profile")
	selection, errText := d.resolveSlashProfileSelection(i, profileToken)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	where := map[string]any{"id": selection.UserID}
	if selection.Mode != slashProfileScopeAll && selection.ProfileNo > 0 {
		where["profile_no"] = selection.ProfileNo
	}
	if !strings.EqualFold(uid, "all") && !strings.EqualFold(uid, "everything") {
		where["uid"] = parseUID(uid)
	}
	removed, err := d.manager.query.DeleteQuery(table, where)
	if err != nil {
		d.respondEphemeral(s, i, err.Error())
		return
	}
	if removed == 0 {
		target := selection.TargetLabel()
		if strings.EqualFold(uid, "all") || strings.EqualFold(uid, "everything") {
			d.respondEphemeral(s, i, fmt.Sprintf("No tracking entries found in %s.", target))
			return
		}
		d.respondEphemeral(s, i, fmt.Sprintf("Tracking not found in %s.", target))
		return
	}
	// Keep slash removals in parity with text commands and legacy API deletes:
	// monster alerts may still match from the fastMonsters cache until it is refreshed.
	if d.manager != nil && d.manager.processor != nil {
		d.manager.processor.RefreshAlertCacheAsync()
	}
	d.logSlashUX(i, "remove", "scope", selection.LogValue())
	if strings.EqualFold(uid, "all") || strings.EqualFold(uid, "everything") {
		target := selection.TargetLabel()
		d.respondEphemeral(s, i, fmt.Sprintf("Removed %d tracking entries from %s. Next: use `/tracked` to review your alerts.", removed, target))
		return
	}
	d.respondEphemeral(s, i, fmt.Sprintf("Tracking removed from %s. Next: use `/tracked` to review your alerts.", selection.TargetLabel()))
}
