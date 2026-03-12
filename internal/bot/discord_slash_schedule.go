package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) buildProfileScheduleDayPayload(i *discordgo.InteractionCreate, profileToken string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, "Target is not registered."
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return nil, nil, "Target is not registered."
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, "Unable to load profiles."
	}
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		return nil, nil, "Profile not found."
	}
	name := strings.TrimSpace(fmt.Sprintf("%v", selected["name"]))
	if name == "" {
		name = fmt.Sprintf("Profile %d", toInt(selected["profile_no"], 0))
	}
	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("Add schedule for %s", name),
		Description: "Select a day for this schedule slot.",
	}
	options := []discordgo.SelectMenuOption{
		{Label: "Monday", Value: "mon"},
		{Label: "Tuesday", Value: "tue"},
		{Label: "Wednesday", Value: "wed"},
		{Label: "Thursday", Value: "thu"},
		{Label: "Friday", Value: "fri"},
		{Label: "Saturday", Value: "sat"},
		{Label: "Sunday", Value: "sun"},
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    slashProfileScheduleDay + fmt.Sprintf("%d", toInt(selected["profile_no"], 0)),
		Options:     options,
		Placeholder: "Select day",
		MaxValues:   1,
		MinValues:   &min,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleBack + fmt.Sprintf("%d", toInt(selected["profile_no"], 0)), Label: "Back", Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleDayPayloadGlobal(i *discordgo.InteractionCreate) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	embed := &discordgo.MessageEmbed{
		Title:       "Add schedule",
		Description: "Select a day for this schedule slot.",
	}
	options := []discordgo.SelectMenuOption{
		{Label: "Monday", Value: "mon"},
		{Label: "Tuesday", Value: "tue"},
		{Label: "Wednesday", Value: "wed"},
		{Label: "Thursday", Value: "thu"},
		{Label: "Friday", Value: "fri"},
		{Label: "Saturday", Value: "sat"},
		{Label: "Sunday", Value: "sun"},
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    slashProfileScheduleDayGlobal,
		Options:     options,
		Placeholder: "Select day(s)",
		MaxValues:   7,
		MinValues:   &min,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: "Back", Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleAssignPayload(i *discordgo.InteractionCreate, days []int, startMin, endMin int) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, "Target is not registered."
	}
	userID, _ := slashUser(i)
	if userID == "" {
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
	options := []discordgo.SelectMenuOption{}
	for _, row := range profiles {
		number := toInt(row["profile_no"], 0)
		name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
		if name == "" {
			name = fmt.Sprintf("Profile %d", number)
		}
		options = append(options, discordgo.SelectMenuOption{
			Label: fmt.Sprintf("%d. %s", number, name),
			Value: fmt.Sprintf("%d", number),
		})
		if len(options) >= 25 {
			break
		}
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    fmt.Sprintf("%s%s-%d-%d", slashProfileScheduleAssign, joinDayList(days), startMin, endMin),
		Options:     options,
		Placeholder: "Select profile",
		MaxValues:   1,
		MinValues:   &min,
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Choose profile",
		Description: fmt.Sprintf("Schedule %s %02d:%02d-%02d:%02d", labelDayList(days), startMin/60, startMin%60, endMin/60, endMin%60),
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: "Back", Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleEditDayPayload(entry scheduleEntry) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	embed := &discordgo.MessageEmbed{
		Title:       "Edit schedule",
		Description: "Select a new day for this schedule slot.",
	}
	options := []discordgo.SelectMenuOption{
		{Label: "Monday", Value: "mon", Default: entry.Day == 1},
		{Label: "Tuesday", Value: "tue", Default: entry.Day == 2},
		{Label: "Wednesday", Value: "wed", Default: entry.Day == 3},
		{Label: "Thursday", Value: "thu", Default: entry.Day == 4},
		{Label: "Friday", Value: "fri", Default: entry.Day == 5},
		{Label: "Saturday", Value: "sat", Default: entry.Day == 6},
		{Label: "Sunday", Value: "sun", Default: entry.Day == 7},
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    slashProfileScheduleEditDay + fmt.Sprintf("%d|%s", entry.ProfileNo, scheduleEntryValue(entry)),
		Options:     options,
		Placeholder: "Select day",
		MaxValues:   1,
		MinValues:   &min,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: "Back", Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleEditAssignPayload(i *discordgo.InteractionCreate, entry scheduleEntry, day, startMin, endMin int) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, "Target is not registered."
	}
	userID, _ := slashUser(i)
	if userID == "" {
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
	options := []discordgo.SelectMenuOption{}
	for _, row := range profiles {
		number := toInt(row["profile_no"], 0)
		name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
		if name == "" {
			name = fmt.Sprintf("Profile %d", number)
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:   fmt.Sprintf("%d. %s", number, name),
			Value:   fmt.Sprintf("%d", number),
			Default: number == entry.ProfileNo,
		})
		if len(options) >= 25 {
			break
		}
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    fmt.Sprintf("%s%d|%s:%d-%d-%d", slashProfileScheduleEditAssign, entry.ProfileNo, scheduleEntryValue(entry), day, startMin, endMin),
		Options:     options,
		Placeholder: "Select profile",
		MaxValues:   1,
		MinValues:   &min,
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Edit schedule",
		Description: fmt.Sprintf("Schedule %s %02d:%02d-%02d:%02d", []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}[day-1], startMin/60, startMin%60, endMin/60, endMin%60),
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: "Back", Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}
