package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func slashErrorEmbed(text string) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Description: "❌ " + text,
		Color:       0xED4245, // Discord red
	}
}

func (d *Discord) respondEphemeralError(s *discordgo.Session, i *discordgo.InteractionCreate, text string) {
	d.respondEphemeralComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{slashErrorEmbed(text)}, nil)
}

func (d *Discord) respondUpdateError(s *discordgo.Session, i *discordgo.InteractionCreate, text string) {
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{slashErrorEmbed(text)}, nil)
}

func (d *Discord) buildAreaShowPayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	embed, components, _, errText := d.buildAreaShowPayloadState(i, selected)
	return embed, components, errText
}

func (d *Discord) buildAreaShowPayloadState(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, *slashMapRequest, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.fences == nil {
		return nil, nil, nil, translateOrDefault(tr, "No available areas found.")
	}
	areas := selectableAreaNames(d.manager.fences.Fences)
	if len(areas) == 0 {
		return nil, nil, nil, translateOrDefault(tr, "No available areas found.")
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager.query == nil {
		return nil, nil, nil, translateOrDefault(tr, "Target is not registered.")
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, nil, translateOrDefault(tr, "Unable to load areas.")
	}
	if human == nil {
		return nil, nil, nil, translateOrDefault(tr, "Target is not registered.")
	}
	enabledAreas := parseAreaListFromHuman(human)
	enabledSet := map[string]bool{}
	for _, area := range enabledAreas {
		enabledSet[strings.ToLower(area)] = true
	}
	if strings.TrimSpace(selected) == "" {
		for _, area := range areas {
			if enabledSet[strings.ToLower(area)] {
				selected = area
				break
			}
		}
		if selected == "" {
			selected = areas[0]
		}
	}

	enabled := enabledSet[strings.ToLower(selected)]
	title := translateFormatOrDefault(tr, "Area: {0}", selected)
	if enabled {
		title += " ✅"
	}
	embed := &discordgo.MessageEmbed{
		Title: title,
		Color: 0x5865F2,
	}
	mapReq := d.areaMapRequest(selected)
	d.applySlashMapImage(embed, mapReq)

	min := 1
	options := make([]discordgo.SelectMenuOption, 0, len(areas))
	for _, area := range areas {
		label := area
		if enabledSet[strings.ToLower(area)] {
			label = area + " ✅"
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:   label,
			Value:   area,
			Default: strings.EqualFold(area, selected),
		})
	}
	menu := discordgo.SelectMenu{
		CustomID:    slashAreaShowSelect,
		Options:     options,
		Placeholder: translateOrDefault(tr, "Select area"),
		MaxValues:   1,
		MinValues:   &min,
	}
	buttonID := slashAreaShowAdd + selected
	buttonLabel := translateOrDefault(tr, "Add Area")
	buttonStyle := discordgo.SuccessButton
	if enabled {
		buttonID = slashAreaShowRemove + selected
		buttonLabel = translateOrDefault(tr, "Remove Area")
		buttonStyle = discordgo.DangerButton
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: buttonID, Label: buttonLabel, Style: buttonStyle},
			discordgo.Button{CustomID: slashProfileAreaBack, Label: translateOrDefault(tr, "Back to Profiles"), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, mapReq, ""
}

func (d *Discord) buildProfilePayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	embed, components, _, errText := d.buildProfilePayloadState(i, selected)
	return embed, components, errText
}

func (d *Discord) buildProfilePayloadState(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, *slashMapRequest, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, nil, translateOrDefault(tr, "Target is not registered.")
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return nil, nil, nil, translateOrDefault(tr, "Target is not registered.")
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		return nil, nil, nil, translateOrDefault(tr, "Target is not registered.")
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, nil, translateOrDefault(tr, "Unable to load profiles.")
	}
	if len(profiles) == 0 {
		embed, components, mapReq := d.buildProfileEmptyPayloadState(human)
		return embed, components, mapReq, ""
	}
	sort.Slice(profiles, func(i, j int) bool {
		return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0)
	})
	currentProfile := toInt(human["current_profile_no"], 1)
	preferredProfile := toInt(human["preferred_profile_no"], 0)
	if preferredProfile <= 0 {
		preferredProfile = 1
	}
	quietHoursEnabled := toInt(human["schedule_disabled"], 0) == 0 && currentProfile == 0
	selectedRow := profileRowByToken(profiles, selected)
	if selectedRow == nil {
		fallbackProfile := currentProfile
		if quietHoursEnabled {
			fallbackProfile = preferredProfile
		}
		if fallbackProfile > 0 {
			selectedRow = profileRowByToken(profiles, fmt.Sprintf("%d", fallbackProfile))
		}
	}
	if selectedRow == nil {
		selectedRow = profiles[0]
	}
	selectedNo := toInt(selectedRow["profile_no"], 0)
	selectedName := localizedProfileName(tr, selectedRow)

	areas := parseProfileAreas(selectedRow["area"])
	areaText := translateOrDefault(tr, "None")
	if len(areas) > 0 {
		areaText = strings.Join(areas, ", ")
	}
	lat := toFloat(selectedRow["latitude"])
	lon := toFloat(selectedRow["longitude"])
	locationText := translateOrDefault(tr, "Not set")
	if lat != 0 || lon != 0 {
		locationText = fmt.Sprintf("%s, %s", formatFloat(lat), formatFloat(lon))
	}
	hoursText := profileScheduleText(tr, selectedRow["active_hours"])
	if hoursText == "" {
		hoursText = translateOrDefault(tr, "No schedules")
	}
	title := translateFormatOrDefault(tr, "Profile: {0}", selectedName)
	if selectedNo == currentProfile {
		title += " ✅"
	}
	description := translateOrDefault(tr, "Schedules enable alerts only during the listed windows. Outside those windows, alerts are paused. If you have no schedules, alerts run all the time. End times are exclusive, so back-to-back periods can share the same minute. Times use your saved location timezone.")
	if quietHoursEnabled {
		description = translateOrDefault(tr, "**Quiet Hours Enabled**\nAlerts are currently paused outside your active schedule windows.") + "\n\n" + description
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Color:       0x5865F2,
		Fields: []*discordgo.MessageEmbedField{
			{Name: translateOrDefault(tr, "Location"), Value: locationText, Inline: false},
			{Name: translateOrDefault(tr, "Areas"), Value: areaText, Inline: false},
			{Name: translateOrDefault(tr, "Schedule"), Value: hoursText, Inline: false},
		},
	}
	mapReq := d.profileMapRequest(lat, lon, areas)
	d.applySlashMapImage(embed, mapReq)

	options := make([]discordgo.SelectMenuOption, 0, len(profiles))
	for _, row := range profiles {
		number := toInt(row["profile_no"], 0)
		name := localizedProfileName(tr, row)
		label := fmt.Sprintf("%d. %s", number, name)
		if number == currentProfile {
			label += " ✅"
		}
		options = append(options, discordgo.SelectMenuOption{
			Label:   label,
			Value:   fmt.Sprintf("%d", number),
			Default: number == selectedNo,
		})
	}
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    slashProfileSelect,
		Options:     options,
		Placeholder: translateOrDefault(tr, "Select profile"),
		MaxValues:   1,
		MinValues:   &min,
	}
	setDisabled := selectedNo == currentProfile
	setLabel := translateOrDefault(tr, "Set Active")
	if quietHoursEnabled {
		setDisabled = selectedNo == preferredProfile
		setLabel = translateOrDefault(tr, "Set for Active Hours")
	}
	setButton := discordgo.Button{
		CustomID: slashProfileSet + fmt.Sprintf("%d", selectedNo),
		Label:    setLabel,
		Style:    discordgo.SuccessButton,
		Disabled: setDisabled,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			setButton,
			discordgo.Button{CustomID: slashProfileCreate, Label: translateOrDefault(tr, "Create Profile"), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileDelete + fmt.Sprintf("%d", selectedNo), Label: translateOrDefault(tr, "Delete Profile"), Style: discordgo.DangerButton, Disabled: len(profiles) <= 1},
		}},
	}
	clearDisabled := lat == 0 && lon == 0
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileLocation, Label: translateOrDefault(tr, "Set Location"), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileArea, Label: translateOrDefault(tr, "Manage Areas"), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileLocationClear, Label: translateOrDefault(tr, "Clear Location"), Style: discordgo.DangerButton, Disabled: clearDisabled},
	}})
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileScheduleOverview, Label: translateOrDefault(tr, "Scheduler"), Style: discordgo.PrimaryButton},
	}})
	return embed, components, mapReq, ""
}

func (d *Discord) buildProfileEmptyPayload(human map[string]any) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	embed, components, _ := d.buildProfileEmptyPayloadState(human)
	return embed, components
}

func (d *Discord) buildProfileEmptyPayloadState(human map[string]any) (*discordgo.MessageEmbed, []discordgo.MessageComponent, *slashMapRequest) {
	tr := d.slashTranslator(d.resolvedHumanLanguage(human))
	areas := parseAreaListFromHuman(human)
	areaText := translateOrDefault(tr, "None")
	if len(areas) > 0 {
		areaText = strings.Join(areas, ", ")
	}
	lat := toFloat(human["latitude"])
	lon := toFloat(human["longitude"])
	locationText := translateOrDefault(tr, "Not set")
	if lat != 0 || lon != 0 {
		locationText = fmt.Sprintf("%s, %s", formatFloat(lat), formatFloat(lon))
	}

	embed := &discordgo.MessageEmbed{
		Title:       translateOrDefault(tr, "No profiles yet"),
		Description: translateOrDefault(tr, "Create your first profile to manage alerts. You can still set location and areas now."),
		Color:       0x5865F2,
		Fields: []*discordgo.MessageEmbedField{
			{Name: translateOrDefault(tr, "Location"), Value: locationText, Inline: false},
			{Name: translateOrDefault(tr, "Areas"), Value: areaText, Inline: false},
		},
	}
	mapReq := d.profileMapRequest(lat, lon, areas)
	d.applySlashMapImage(embed, mapReq)

	clearDisabled := lat == 0 && lon == 0
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileCreate, Label: translateOrDefault(tr, "Create Profile"), Style: discordgo.PrimaryButton},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileLocation, Label: translateOrDefault(tr, "Set Location"), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileArea, Label: translateOrDefault(tr, "Manage Areas"), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileLocationClear, Label: translateOrDefault(tr, "Clear Location"), Style: discordgo.DangerButton, Disabled: clearDisabled},
		}},
	}
	return embed, components, mapReq
}

func (d *Discord) buildProfileDeletePayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.query == nil {
		return nil, nil, translateOrDefault(tr, "Target is not registered.")
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return nil, nil, translateOrDefault(tr, "Target is not registered.")
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, translateOrDefault(tr, "Unable to load profiles.")
	}
	if len(profiles) <= 1 {
		return nil, nil, translateOrDefault(tr, "You must keep at least one profile.")
	}
	selectedRow := profileRowByToken(profiles, selected)
	if selectedRow == nil {
		return nil, nil, translateOrDefault(tr, "Profile not found.")
	}
	profileNo := toInt(selectedRow["profile_no"], 0)
	name := localizedProfileName(tr, selectedRow)
	embed := &discordgo.MessageEmbed{
		Title:       translateOrDefault(tr, "Delete Profile"),
		Description: translateFormatOrDefault(tr, "Delete **{0}**? This cannot be undone.", name),
		Color:       0xED4245,
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileDeleteConfirm + fmt.Sprintf("%d", profileNo), Label: translateOrDefault(tr, "Delete"), Style: discordgo.DangerButton},
			discordgo.Button{CustomID: slashProfileDeleteCancel + fmt.Sprintf("%d", profileNo), Label: translateOrDefault(tr, "Cancel"), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}
