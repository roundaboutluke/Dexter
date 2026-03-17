package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) buildProfileScheduleOverviewPayload(i *discordgo.InteractionCreate) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, tr.Translate("Unable to load profiles.", false)
	}
	if len(profiles) == 0 {
		return nil, nil, tr.Translate("You do not have any profiles.", false)
	}
	sort.Slice(profiles, func(i, j int) bool { return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0) })
	type rowEntry struct {
		ProfileName string
		Entry       scheduleEntry
	}
	entries := []rowEntry{}
	for _, profile := range profiles {
		name := localizedProfileName(tr, profile)
		for _, entry := range scheduleEntriesFromRaw(profile["active_hours"]) {
			if entry.Legacy {
				continue
			}
			entry.ProfileNo = toInt(profile["profile_no"], 0)
			entries = append(entries, rowEntry{ProfileName: name, Entry: entry})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Entry.Day != entries[j].Entry.Day {
			return entries[i].Entry.Day < entries[j].Entry.Day
		}
		return entries[i].Entry.StartMin < entries[j].Entry.StartMin
	})
	lines := []string{}
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("%s — %s", scheduleEntryLabelLocalized(tr, entry.Entry), entry.ProfileName))
	}
	content := tr.Translate("No schedules set.", false)
	if len(lines) > 0 {
		content = strings.Join(lines, "\n")
	}
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Scheduler", false),
		Description: tr.Translate("Day | Start-End — Profile", false),
		Fields: []*discordgo.MessageEmbedField{
			{Name: tr.Translate("Schedules", false), Value: content, Inline: false},
		},
	}
	scheduleDisabled := toInt(human["schedule_disabled"], 0) == 1
	schedulerText := tr.Translate("Enabled", false)
	if scheduleDisabled {
		schedulerText = tr.Translate("Disabled", false)
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   tr.Translate("Scheduler", false),
		Value:  schedulerText,
		Inline: false,
	})
	components := []discordgo.MessageComponent{}
	if options := scheduleEditOptionsGlobalLocalized(tr, profiles); len(options) > 0 {
		min := 1
		editMenu := discordgo.SelectMenu{
			CustomID:    slashProfileScheduleEditGlobal,
			Options:     options,
			Placeholder: tr.Translate("Edit schedule entry", false),
			MaxValues:   1,
			MinValues:   &min,
		}
		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{editMenu}})
	}
	if options := scheduleRemoveOptionsGlobalLocalized(tr, profiles); len(options) > 0 {
		min := 1
		removeMenu := discordgo.SelectMenu{
			CustomID:    slashProfileScheduleRemoveGlobal,
			Options:     options,
			Placeholder: tr.Translate("Remove schedule entry", false),
			MaxValues:   1,
			MinValues:   &min,
		}
		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{removeMenu}})
	}
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileScheduleAddGlobal, Label: tr.Translate("Add Period", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileScheduleBack + "all", Label: tr.Translate("Back to Profiles", false), Style: discordgo.PrimaryButton},
	}})
	scheduleLabel := tr.Translate("Disable Scheduler", false)
	scheduleStyle := discordgo.SecondaryButton
	if scheduleDisabled {
		scheduleLabel = tr.Translate("Enable Scheduler", false)
		scheduleStyle = discordgo.SuccessButton
	}
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileScheduleToggle, Label: scheduleLabel, Style: scheduleStyle},
	}})
	return embed, components, ""
}
