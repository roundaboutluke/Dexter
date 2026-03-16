package bot

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) handleSlashRemove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	options := slashOptions(i.ApplicationCommandData())
	trackType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(trackType) == "" {
		d.respondEphemeral(s, i, translateOrDefault(tr, "Please pick a tracking type."))
		return
	}
	value, ok := optionString(options, "tracking")
	if !ok || strings.TrimSpace(value) == "" {
		d.respondEphemeral(s, i, translateOrDefault(tr, "Please pick a tracking entry."))
		return
	}

	trackingType, uid := parseRemoveSelection(trackType, value)
	if trackingType == "" || uid == "" {
		d.respondEphemeral(s, i, translateOrDefault(tr, "That tracking entry could not be parsed."))
		return
	}
	if strings.Contains(value, "|") {
		expected := strings.ToLower(strings.TrimSpace(trackType))
		if expected != "" && trackingType != expected {
			d.respondEphemeral(s, i, translateOrDefault(tr, "Tracking type changed; please clear the tracking selection and pick again."))
			return
		}
	}
	if d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, translateOrDefault(tr, "That tracking entry could not be removed."))
		return
	}

	profileToken, _ := optionString(options, "profile")
	selection, errText := d.resolveSlashProfileSelection(i, profileToken)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	rows, table := d.slashTrackingRowsForSelection(selection, trackingType)
	if table == "" {
		d.respondEphemeral(s, i, translateOrDefault(tr, "That tracking entry could not be removed."))
		return
	}
	targetRows := rows
	if !strings.EqualFold(uid, "all") && !strings.EqualFold(uid, "everything") {
		targetRows = slashFilterRowsByUID(rows, uid)
	}
	removed, err := d.deleteSlashTrackingRows(table, targetRows)
	if err != nil {
		d.respondEphemeral(s, i, err.Error())
		return
	}
	if removed == 0 {
		target := selection.TargetLabelLocalized(tr)
		if strings.EqualFold(uid, "all") || strings.EqualFold(uid, "everything") {
			d.respondEphemeral(s, i, translateFormatOrDefault(tr, "No filters found in {0}.", target))
			return
		}
		d.respondEphemeral(s, i, translateFormatOrDefault(tr, "Filter not found in {0}.", target))
		return
	}
	d.logSlashUX(i, "remove", "scope", selection.LogValue())
	embeds, components, ok := d.slashFilterMutationResponse(i, "removed", trackingType, table, selection.TargetLabelLocalized(tr), selection.UserID, targetRows)
	if !ok {
		d.respondEphemeral(s, i, translateOrDefault(tr, "No filters were changed."))
		return
	}
	d.respondEphemeralComponentsEmbed(s, i, "", embeds, components)
}
