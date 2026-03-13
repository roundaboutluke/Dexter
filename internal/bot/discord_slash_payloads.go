package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/tileserver"
)

func (d *Discord) buildAreaShowPayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
	tr := d.slashInteractionTranslator(i)
	if d.manager == nil || d.manager.fences == nil {
		return nil, nil, tr.Translate("No available areas found.", false)
	}
	areas := selectableAreaNames(d.manager.fences.Fences)
	if len(areas) == 0 {
		return nil, nil, tr.Translate("No available areas found.", false)
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager.query == nil {
		return nil, nil, tr.Translate("Target is not registered.", false)
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		return nil, nil, tr.Translate("Unable to load areas.", false)
	}
	if human == nil {
		return nil, nil, tr.Translate("Target is not registered.", false)
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
	title := tr.TranslateFormat("Area: {0}", selected)
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
		Placeholder: tr.Translate("Select area", false),
		MaxValues:   1,
		MinValues:   &min,
	}
	buttonID := slashAreaShowAdd + selected
	buttonLabel := tr.Translate("Add Area", false)
	buttonStyle := discordgo.SuccessButton
	if enabled {
		buttonID = slashAreaShowRemove + selected
		buttonLabel = tr.Translate("Remove Area", false)
		buttonStyle = discordgo.DangerButton
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: buttonID, Label: buttonLabel, Style: buttonStyle},
			discordgo.Button{CustomID: slashProfileAreaBack, Label: tr.Translate("Back to Profiles", false), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}

func (d *Discord) buildProfilePayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
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
	selectedName := localizedProfileName(tr, selectedRow)

	areas := parseProfileAreas(selectedRow["area"])
	areaText := tr.Translate("None", false)
	if len(areas) > 0 {
		areaText = strings.Join(areas, ", ")
	}
	lat := toFloat(selectedRow["latitude"])
	lon := toFloat(selectedRow["longitude"])
	locationText := tr.Translate("Not set", false)
	if lat != 0 || lon != 0 {
		locationText = fmt.Sprintf("%s, %s", formatFloat(lat), formatFloat(lon))
	}
	hoursText := profileScheduleTextLocalized(tr, selectedRow["active_hours"])
	if hoursText == "" {
		hoursText = tr.Translate("No schedules", false)
	}
	title := tr.TranslateFormat("Profile: {0}", selectedName)
	if selectedNo == currentProfile {
		title += " ✅"
	}
	description := tr.Translate("Schedules enable alerts only during the listed windows. Outside those windows, alerts are paused. If you have no schedules, alerts run all the time. End times are exclusive, so back-to-back periods can share the same minute. Times use your saved location timezone.", false)
	if quietHoursEnabled {
		description = tr.Translate("**Quiet Hours Enabled**\nAlerts are currently paused outside your active schedule windows.", false) + "\n\n" + description
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: description,
		Fields: []*discordgo.MessageEmbedField{
			{Name: tr.Translate("Location", false), Value: locationText, Inline: false},
			{Name: tr.Translate("Areas", false), Value: areaText, Inline: false},
			{Name: tr.Translate("Schedule", false), Value: hoursText, Inline: false},
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
		Placeholder: tr.Translate("Select profile", false),
		MaxValues:   1,
		MinValues:   &min,
	}
	setDisabled := selectedNo == currentProfile
	setLabel := tr.Translate("Set Active", false)
	if quietHoursEnabled {
		setDisabled = selectedNo == preferredProfile
		setLabel = tr.Translate("Set for Active Hours", false)
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
			discordgo.Button{CustomID: slashProfileCreate, Label: tr.Translate("Create Profile", false), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileDelete + fmt.Sprintf("%d", selectedNo), Label: tr.Translate("Delete Profile", false), Style: discordgo.DangerButton, Disabled: len(profiles) <= 1},
		}},
	}
	clearDisabled := lat == 0 && lon == 0
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileLocation, Label: tr.Translate("Set Location", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileArea, Label: tr.Translate("Manage Areas", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashProfileLocationClear, Label: tr.Translate("Clear Location", false), Style: discordgo.DangerButton, Disabled: clearDisabled},
	}})
	components = append(components, discordgo.ActionsRow{Components: []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashProfileScheduleOverview, Label: tr.Translate("Scheduler", false), Style: discordgo.PrimaryButton},
	}})
	return embed, components, ""
}

func (d *Discord) buildProfileEmptyPayload(human map[string]any) (*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	tr := d.slashTranslator(d.resolvedHumanLanguage(human))
	areas := parseAreaListFromHuman(human)
	areaText := tr.Translate("None", false)
	if len(areas) > 0 {
		areaText = strings.Join(areas, ", ")
	}
	lat := toFloat(human["latitude"])
	lon := toFloat(human["longitude"])
	locationText := tr.Translate("Not set", false)
	if lat != 0 || lon != 0 {
		locationText = fmt.Sprintf("%s, %s", formatFloat(lat), formatFloat(lon))
	}

	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("No profiles yet", false),
		Description: tr.Translate("Create your first profile to manage alerts. You can still set location and areas now.", false),
		Fields: []*discordgo.MessageEmbedField{
			{Name: tr.Translate("Location", false), Value: locationText, Inline: false},
			{Name: tr.Translate("Areas", false), Value: areaText, Inline: false},
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
			discordgo.Button{CustomID: slashProfileCreate, Label: tr.Translate("Create Profile", false), Style: discordgo.PrimaryButton},
		}},
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileLocation, Label: tr.Translate("Set Location", false), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileArea, Label: tr.Translate("Manage Areas", false), Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashProfileLocationClear, Label: tr.Translate("Clear Location", false), Style: discordgo.DangerButton, Disabled: clearDisabled},
		}},
	}
	return embed, components
}

func (d *Discord) buildProfileDeletePayload(i *discordgo.InteractionCreate, selected string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, string) {
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
	if len(profiles) <= 1 {
		return nil, nil, tr.Translate("You must keep at least one profile.", false)
	}
	selectedRow := profileRowByToken(profiles, selected)
	if selectedRow == nil {
		return nil, nil, tr.Translate("Profile not found.", false)
	}
	profileNo := toInt(selectedRow["profile_no"], 0)
	name := localizedProfileName(tr, selectedRow)
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Delete Profile", false),
		Description: tr.TranslateFormat("Delete **{0}**? This cannot be undone.", name),
	}
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashProfileDeleteConfirm + fmt.Sprintf("%d", profileNo), Label: tr.Translate("Delete", false), Style: discordgo.DangerButton},
			discordgo.Button{CustomID: slashProfileDeleteCancel + fmt.Sprintf("%d", profileNo), Label: tr.Translate("Cancel", false), Style: discordgo.SecondaryButton},
		}},
	}
	return embed, components, ""
}
