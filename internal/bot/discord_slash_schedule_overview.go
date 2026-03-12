package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) buildProfileScheduleOverviewPayload(i *discordgo.InteractionCreate) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, "Target is not registered."
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return nil, nil, "Target is not registered."
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		return nil, nil, "Target is not registered."
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, "Unable to load profiles."
	}
	if len(profiles) == 0 {
		return nil, nil, "You do not have any profiles."
	}
	sort.Slice(profiles, func(i, j int) bool { return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0) })
	type rowEntry struct {
		ProfileName string
		Entry       scheduleEntry
	}
	entries := []rowEntry{}
	for _, profile := range profiles {
		name := strings.TrimSpace(fmt.Sprintf("%v", profile["name"]))
		if name == "" {
			name = fmt.Sprintf("Profile %d", toInt(profile["profile_no"], 0))
		}
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
		lines = append(lines, fmt.Sprintf("%s — %s", scheduleEntryLabel(entry.Entry), entry.ProfileName))
	}
	content := "No schedules set."
	if len(lines) > 0 {
		content = strings.Join(lines, "\n")
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Scheduler",
		Description: "Day | Start-End — Profile",
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Schedules", Value: content, Inline: false},
		},
	}
	scheduleDisabled := toInt(human["schedule_disabled"], 0) == 1
	schedulerText := "Enabled"
	if scheduleDisabled {
		schedulerText = "Disabled"
	}
	embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
		Name:   "Scheduler",
		Value:  schedulerText,
		Inline: false,
	})
	components := []discordgo.MessageComponent{}
	if options := scheduleEditOptionsGlobal(profiles); len(options) > 0 {
		min := 1
		editMenu := discordgo.SelectMenu{
			CustomID:    slashProfileScheduleEditGlobal,
			Options:     options,
			Placeholder: "Edit schedule entry",
			MaxValues:   1,
			MinValues:   &min,
		}
		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{editMenu}})
	}
	if options := scheduleRemoveOptionsGlobal(profiles); len(options) > 0 {
		min := 1
		removeMenu := discordgo.SelectMenu{
			CustomID:    slashProfileScheduleRemoveGlobal,
			Options:     options,
			Placeholder: "Remove schedule entry",
			MaxValues:   1,
			MinValues:   &min,
		}
		components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{removeMenu}})
	}
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileScheduleAddGlobal, Label: "Add Period", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileScheduleBack + "all", Label: "Back to Profiles", Style: discordgo.PrimaryButton},
	}})
	scheduleLabel := "Disable Scheduler"
	scheduleStyle := discordgo.SecondaryButton
	if scheduleDisabled {
		scheduleLabel = "Enable Scheduler"
		scheduleStyle = discordgo.SuccessButton
	}
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileScheduleToggle, Label: scheduleLabel, Style: scheduleStyle},
	}})
	return embed, components, ""
}
