package bot

import (
	"fmt"
	"sort"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) buildProfileScheduleDayPayload(i *discordgo.InteractionCreate, profileToken string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, tr.Translate("Unable to load profiles.", false)
	}
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		return nil, nil, tr.Translate("Profile not found.", false)
	}
	name := localizedProfileName(tr, selected)
	embed := &discordgo.MessageEmbed{
		Title:       tr.TranslateFormat("Add schedule for {0}", name),
		Description: tr.Translate("Select a day for this schedule slot.", false),
	}
	options := []discordgo.SelectMenuOption{
		{Label: localizedDayLabel(tr, 1), Value: "mon"},
		{Label: localizedDayLabel(tr, 2), Value: "tue"},
		{Label: localizedDayLabel(tr, 3), Value: "wed"},
		{Label: localizedDayLabel(tr, 4), Value: "thu"},
		{Label: localizedDayLabel(tr, 5), Value: "fri"},
		{Label: localizedDayLabel(tr, 6), Value: "sat"},
		{Label: localizedDayLabel(tr, 7), Value: "sun"},
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    slashProfileScheduleDay + fmt.Sprintf("%d", toInt(selected["profile_no"], 0)),
		Options:     options,
		Placeholder: tr.Translate("Select day", false),
		MaxValues:   1,
		MinValues:   &min,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleBack + fmt.Sprintf("%d", toInt(selected["profile_no"], 0)), Label: tr.Translate("Back", false), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleDayPayloadGlobal(i *discordgo.InteractionCreate) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Add schedule", false),
		Description: tr.Translate("Select a day for this schedule slot.", false),
	}
	options := []discordgo.SelectMenuOption{
		{Label: localizedDayLabel(tr, 1), Value: "mon"},
		{Label: localizedDayLabel(tr, 2), Value: "tue"},
		{Label: localizedDayLabel(tr, 3), Value: "wed"},
		{Label: localizedDayLabel(tr, 4), Value: "thu"},
		{Label: localizedDayLabel(tr, 5), Value: "fri"},
		{Label: localizedDayLabel(tr, 6), Value: "sat"},
		{Label: localizedDayLabel(tr, 7), Value: "sun"},
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    slashProfileScheduleDayGlobal,
		Options:     options,
		Placeholder: tr.Translate("Select day(s)", false),
		MaxValues:   7,
		MinValues:   &min,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: tr.Translate("Back", false), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleAssignPayload(i *discordgo.InteractionCreate, days []int, startMin, endMin int) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	userID, _ := slashUser(i)
	if userID == "" {
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
	options := []discordgo.SelectMenuOption{}
	for _, row := range profiles {
		number := toInt(row["profile_no"], 0)
		name := localizedProfileName(tr, row)
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
		Placeholder: tr.Translate("Select profile", false),
		MaxValues:   1,
		MinValues:   &min,
	}
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Choose profile", false),
		Description: tr.TranslateFormat("Schedule {0} {1}-{2}", labelDayListLocalized(tr, days), fmt.Sprintf("%02d:%02d", startMin/60, startMin%60), fmt.Sprintf("%02d:%02d", endMin/60, endMin%60)),
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: tr.Translate("Back", false), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleEditDayPayload(i *discordgo.InteractionCreate, entry scheduleEntry) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Edit schedule", false),
		Description: tr.Translate("Select a new day for this schedule slot.", false),
	}
	options := []discordgo.SelectMenuOption{
		{Label: localizedDayLabel(tr, 1), Value: "mon", Default: entry.Day == 1},
		{Label: localizedDayLabel(tr, 2), Value: "tue", Default: entry.Day == 2},
		{Label: localizedDayLabel(tr, 3), Value: "wed", Default: entry.Day == 3},
		{Label: localizedDayLabel(tr, 4), Value: "thu", Default: entry.Day == 4},
		{Label: localizedDayLabel(tr, 5), Value: "fri", Default: entry.Day == 5},
		{Label: localizedDayLabel(tr, 6), Value: "sat", Default: entry.Day == 6},
		{Label: localizedDayLabel(tr, 7), Value: "sun", Default: entry.Day == 7},
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    slashProfileScheduleEditDay + fmt.Sprintf("%d|%s", entry.ProfileNo, scheduleEntryValue(entry)),
		Options:     options,
		Placeholder: tr.Translate("Select day", false),
		MaxValues:   1,
		MinValues:   &min,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: tr.Translate("Back", false), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfileScheduleEditAssignPayload(i *discordgo.InteractionCreate, entry scheduleEntry, day, startMin, endMin int) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	userID, _ := slashUser(i)
	if userID == "" {
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
	options := []discordgo.SelectMenuOption{}
	for _, row := range profiles {
		number := toInt(row["profile_no"], 0)
		name := localizedProfileName(tr, row)
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
		Placeholder: tr.Translate("Select profile", false),
		MaxValues:   1,
		MinValues:   &min,
	}
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Edit schedule", false),
		Description: tr.TranslateFormat("Schedule {0} {1}-{2}", localizedDayLabel(tr, day), fmt.Sprintf("%02d:%02d", startMin/60, startMin%60), fmt.Sprintf("%02d:%02d", endMin/60, endMin%60)),
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileScheduleOverview, Label: tr.Translate("Back", false), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}
