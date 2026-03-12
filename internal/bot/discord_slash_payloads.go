package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/tileserver"
)

func (d *Discord) buildAreaShowPayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	if d.manager == nil || d.manager.fences == nil {
		return nil, nil, "No available areas found."
	}
	areas := selectableAreaNames(d.manager.fences.Fences)
	if len(areas) == 0 {
		return nil, nil, "No available areas found."
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager.query == nil {
		return nil, nil, "Target is not registered."
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, "Unable to load areas."
	}
	if human == nil {
		return nil, nil, "Target is not registered."
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

	provider, _ := d.manager.cfg.GetString("geocoding.staticProvider")
	var url string
	if strings.EqualFold(provider, "tileservercache") {
		client := tileserver.NewClient(d.manager.cfg)
		if staticMap, err := tileserver.GenerateGeofenceTile(d.manager.fences.Fences, client, d.manager.cfg, selected); err == nil {
			url = staticMap
		}
	}
	if url == "" {
		url = fallbackStaticMap(d.manager.cfg)
	}

	enabled := enabledSet[strings.ToLower(selected)]
	title := fmt.Sprintf("Area: %s", selected)
	if enabled {
		title += " ✅"
	}
	embed := &discordgo.MessageEmbed{
		Title: title,
	}
	if url != "" {
		embed.Image = &discordgo.MessageEmbedImage{URL: url}
	}

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
		Placeholder: "Select area",
		MaxValues:   1,
		MinValues:   &min,
	}
	buttonID := slashAreaShowAdd + selected
	buttonLabel := "Add Area"
	buttonStyle := discordgo.SuccessButton
	if enabled {
		buttonID = slashAreaShowRemove + selected
		buttonLabel = "Remove Area"
		buttonStyle = discordgo.DangerButton
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: buttonID, Label: buttonLabel, Style: buttonStyle},
			discordgo.Button{CustomID: slashProfileAreaBack, Label: "Back to Profiles", Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfilePayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
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
		embed, components := d.buildProfileEmptyPayload(human)
		return embed, components, ""
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
	selectedName := strings.TrimSpace(fmt.Sprintf("%v", selectedRow["name"]))
	if selectedName == "" {
		selectedName = fmt.Sprintf("Profile %d", selectedNo)
	}

	areas := parseProfileAreas(selectedRow["area"])
	areaText := "None"
	if len(areas) > 0 {
		areaText = strings.Join(areas, ", ")
	}
	lat := toFloat(selectedRow["latitude"])
	lon := toFloat(selectedRow["longitude"])
	locationText := "Not set"
	if lat != 0 || lon != 0 {
		locationText = fmt.Sprintf("%s, %s", formatFloat(lat), formatFloat(lon))
	}
	hoursText := profileScheduleText(selectedRow["active_hours"])
	if hoursText == "" {
		hoursText = "No schedules"
	}
	title := fmt.Sprintf("Profile: %s", selectedName)
	if selectedNo == currentProfile {
		title += " ✅"
	}
	description := "Schedules enable alerts only during the listed windows. Outside those windows, alerts are paused. If you have no schedules, alerts run all the time. End times are exclusive, so back-to-back periods can share the same minute. Times use your saved location timezone."
	if quietHoursEnabled {
		description = "**Quiet Hours Enabled**\nAlerts are currently paused outside your active schedule windows.\n\n" + description
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Location", Value: locationText, Inline: false},
			{Name: "Areas", Value: areaText, Inline: false},
			{Name: "Schedule", Value: hoursText, Inline: false},
		},
	}

	if d.manager != nil && d.manager.cfg != nil {
		if provider, _ := d.manager.cfg.GetString("geocoding.staticProvider"); strings.EqualFold(provider, "tileservercache") {
			client := tileserver.NewClient(d.manager.cfg)
			if lat != 0 || lon != 0 {
				if staticMap, err := tileserver.GenerateConfiguredLocationTile(client, d.manager.cfg, lat, lon); err == nil && staticMap != "" {
					embed.Image = &discordgo.MessageEmbedImage{URL: staticMap}
				}
			} else if len(areas) > 0 && d.manager.fences != nil {
				if staticMap, err := tileserver.GenerateGeofenceTile(d.manager.fences.Fences, client, d.manager.cfg, areas[0]); err == nil && staticMap != "" {
					embed.Image = &discordgo.MessageEmbedImage{URL: staticMap}
				}
			}
		}
	}
	if embed.Image == nil && d.manager != nil && d.manager.cfg != nil {
		if fallback := fallbackStaticMap(d.manager.cfg); fallback != "" {
			embed.Image = &discordgo.MessageEmbedImage{URL: fallback}
		}
	}

	options := make([]discordgo.SelectMenuOption, 0, len(profiles))
	for _, row := range profiles {
		number := toInt(row["profile_no"], 0)
		name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
		if name == "" {
			name = fmt.Sprintf("Profile %d", number)
		}
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
		Placeholder: "Select profile",
		MaxValues:   1,
		MinValues:   &min,
	}
	setDisabled := selectedNo == currentProfile
	setLabel := "Set Active"
	if quietHoursEnabled {
		setDisabled = selectedNo == preferredProfile
		setLabel = "Set for Active Hours"
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
			discordgo.Button{CustomID: slashProfileCreate, Label: "Create Profile", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileDelete + fmt.Sprintf("%d", selectedNo), Label: "Delete Profile", Style: discordgo.DangerButton, Disabled: len(profiles) <= 1},
		}},
	}
	clearDisabled := lat == 0 && lon == 0
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileLocation, Label: "Set Location", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileArea, Label: "Manage Areas", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileLocationClear, Label: "Clear Location", Style: discordgo.DangerButton, Disabled: clearDisabled},
	}})
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileScheduleOverview, Label: "Scheduler", Style: discordgo.PrimaryButton},
	}})
	return embed, components, ""
}

func (d *Discord) buildProfileEmptyPayload(human map[string]any) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	areas := parseAreaListFromHuman(human)
	areaText := "None"
	if len(areas) > 0 {
		areaText = strings.Join(areas, ", ")
	}
	lat := toFloat(human["latitude"])
	lon := toFloat(human["longitude"])
	locationText := "Not set"
	if lat != 0 || lon != 0 {
		locationText = fmt.Sprintf("%s, %s", formatFloat(lat), formatFloat(lon))
	}

	embed := &discordgo.MessageEmbed{
		Title:       "No profiles yet",
		Description: "Create your first profile to manage alerts. You can still set location and areas now.",
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Location", Value: locationText, Inline: false},
			{Name: "Areas", Value: areaText, Inline: false},
		},
	}

	if d.manager != nil && d.manager.cfg != nil {
		if provider, _ := d.manager.cfg.GetString("geocoding.staticProvider"); strings.EqualFold(provider, "tileservercache") {
			client := tileserver.NewClient(d.manager.cfg)
			if lat != 0 || lon != 0 {
				if staticMap, err := tileserver.GenerateConfiguredLocationTile(client, d.manager.cfg, lat, lon); err == nil && staticMap != "" {
					embed.Image = &discordgo.MessageEmbedImage{URL: staticMap}
				}
			} else if len(areas) > 0 && d.manager.fences != nil {
				if staticMap, err := tileserver.GenerateGeofenceTile(d.manager.fences.Fences, client, d.manager.cfg, areas[0]); err == nil && staticMap != "" {
					embed.Image = &discordgo.MessageEmbedImage{URL: staticMap}
				}
			}
		}
	}
	if embed.Image == nil && d.manager != nil && d.manager.cfg != nil {
		if fallback := fallbackStaticMap(d.manager.cfg); fallback != "" {
			embed.Image = &discordgo.MessageEmbedImage{URL: fallback}
		}
	}

	clearDisabled := lat == 0 && lon == 0
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileCreate, Label: "Create Profile", Style: discordgo.PrimaryButton},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileLocation, Label: "Set Location", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileArea, Label: "Manage Areas", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileLocationClear, Label: "Clear Location", Style: discordgo.DangerButton, Disabled: clearDisabled},
		}},
	}
	return embed, components
}

func (d *Discord) buildProfileDeletePayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
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
	if len(profiles) <= 1 {
		return nil, nil, "You must keep at least one profile."
	}
	selectedRow := profileRowByToken(profiles, selected)
	if selectedRow == nil {
		return nil, nil, "Profile not found."
	}
	profileNo := toInt(selectedRow["profile_no"], 0)
	name := strings.TrimSpace(fmt.Sprintf("%v", selectedRow["name"]))
	if name == "" {
		name = fmt.Sprintf("Profile %d", profileNo)
	}
	embed := &discordgo.MessageEmbed{
		Title:       "Delete Profile",
		Description: fmt.Sprintf("Delete **%s**? This cannot be undone.", name),
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileDeleteConfirm + fmt.Sprintf("%d", profileNo), Label: "Delete", Style: discordgo.DangerButton},
			discordgo.Button{CustomID: slashProfileDeleteCancel + fmt.Sprintf("%d", profileNo), Label: "Cancel", Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}
