package bot

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/logging"
	"poraclego/internal/webhook"
)

func profileDeleteOutcome(profiles []map[string]any, profileValue string, result slashExecutionResult) (bool, string) {
	if profileRowByToken(profiles, profileValue) == nil {
		return true, ""
	}
	return false, result.Reply
}

func profileLocationOutcome(human map[string]any, target string, result slashExecutionResult) (bool, string) {
	if human != nil {
		lat := formatFloat(toFloat(human["latitude"]))
		lon := formatFloat(toFloat(human["longitude"]))
		switch strings.ToLower(strings.TrimSpace(target)) {
		case "", "remove":
			if lat == "0" && lon == "0" {
				return true, ""
			}
		default:
			if targetLat, targetLon, ok := parseLatLonString(target); ok {
				if lat == formatFloat(targetLat) && lon == formatFloat(targetLon) {
					return true, ""
				}
			}
		}
	}
	return false, result.Reply
}

func profileCreateOutcome(profiles []map[string]any, name string, result slashExecutionResult) (bool, string) {
	if profileNameExistsRows(profiles, name) {
		return true, ""
	}
	return false, result.Reply
}

func areaToggleOutcome(areas []string, area string, add bool, result slashExecutionResult) (bool, string) {
	enabled := false
	for _, current := range areas {
		if strings.EqualFold(strings.TrimSpace(current), strings.TrimSpace(area)) {
			enabled = true
			break
		}
	}
	if enabled == add {
		return true, ""
	}
	return false, result.Reply
}

func profileLocationModalRemoveOutcome(human map[string]any, result slashExecutionResult) (bool, string, bool) {
	refreshProfile, message := profileLocationOutcome(human, "remove", result)
	return refreshProfile, message, true
}

func (d *Discord) profileLocationModalText(i *discordgo.InteractionCreate) (string, string, string) {
	tr := d.slashInteractionTranslator(i)
	return tr.Translate("Set location", false), tr.Translate("Address or coordinates", false), "51.5,-0.12"
}

func (d *Discord) profileCreateModalText(i *discordgo.InteractionCreate) (string, string, string) {
	tr := d.slashInteractionTranslator(i)
	return tr.Translate("New profile", false), tr.Translate("Profile name", false), "home"
}

func (d *Discord) buildLocationConfirmPayloadState(i *discordgo.InteractionCreate, lat, lon float64, placeConfirmation string) (*discordgo.MessageEmbed, []discordgo.MessageComponent, *slashMapRequest) {
	tr := d.slashInteractionTranslator(i)
	mapLink := fmt.Sprintf("https://maps.google.com/maps?q=%s,%s", formatFloat(lat), formatFloat(lon))
	description := tr.TranslateFormat("I set your location to the following coordinates in{0}:\n{1}", placeConfirmation, mapLink)
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Confirm location", false),
		Description: description,
	}
	mapReq := d.locationMapRequest(lat, lon)
	if !d.locationMapsEnabled() {
		mapReq = nil
	}
	d.applySlashMapImage(embed, mapReq)
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashConfirmButton, Label: tr.Translate("Verify", false), Style: discordgo.SuccessButton},
			discordgo.Button{CustomID: slashCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
		}},
	}
	return embed, components, mapReq
}

func (d *Discord) handleAreaShow(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed, components, errText := d.buildAreaShowPayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondEphemeralComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileAreaShow(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondAreaPayload(s, i, "")
}

func (d *Discord) handleProfileLocationClear(s *discordgo.Session, i *discordgo.InteractionCreate) {
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, d.slashText(i, "Target is not registered."))
		return
	}
	result := d.buildSlashExecutionResult(s, i, "location remove")
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, d.slashText(i, "Target is not registered."))
		return
	}
	refreshProfile, message := profileLocationOutcome(human, "remove", result)
	if !refreshProfile {
		d.respondEphemeral(s, i, message)
		return
	}
	d.respondProfilePayload(s, i, "")
}

func (d *Discord) handleProfileDeletePrompt(s *discordgo.Session, i *discordgo.InteractionCreate, profileValue string) {
	embed, components, errText := d.buildProfileDeletePayload(i, profileValue)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileDeleteConfirm(s *discordgo.Session, i *discordgo.InteractionCreate, profileValue string) {
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, d.slashText(i, "Target is not registered."))
		return
	}
	result := d.buildSlashExecutionResult(s, i, fmt.Sprintf("profile remove %s", profileValue))
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, d.slashText(i, "Unable to load profiles."))
		return
	}
	refreshProfile, message := profileDeleteOutcome(profiles, profileValue, result)
	if !refreshProfile {
		d.respondEphemeral(s, i, message)
		return
	}
	d.respondProfilePayload(s, i, "")
}

func (d *Discord) handleProfileDeleteCancel(s *discordgo.Session, i *discordgo.InteractionCreate, profileValue string) {
	d.respondProfilePayload(s, i, profileValue)
}

func (d *Discord) updateMessageEmbed(s *discordgo.Session, channelID, messageID string, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) (*discordgo.Message, error) {
	if s == nil || channelID == "" || messageID == "" {
		return nil, fmt.Errorf("missing message target")
	}
	edit := &discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	}
	msg, err := s.ChannelMessageEditComplex(edit)
	if err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord message edit failed (channel=%q message=%q): %v", channelID, messageID, err)
		}
		return nil, err
	}
	return msg, nil
}

func (d *Discord) handleAreaShowSelect(s *discordgo.Session, i *discordgo.InteractionCreate, area string) {
	d.respondAreaPayload(s, i, area)
}

func (d *Discord) handleAreaShowToggle(s *discordgo.Session, i *discordgo.InteractionCreate, area string, add bool) {
	verb := "remove"
	if add {
		verb = "add"
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, d.slashText(i, "Target is not registered."))
		return
	}
	result := d.buildSlashExecutionResult(s, i, strings.TrimSpace(fmt.Sprintf("area %s %q", verb, area)))
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, d.slashText(i, "Unable to load areas."))
		return
	}
	if human == nil {
		d.respondEphemeral(s, i, d.slashText(i, "Target is not registered."))
		return
	}
	refreshArea, message := areaToggleOutcome(parseAreaListFromHuman(human), area, add, result)
	if !refreshArea {
		d.respondEphemeral(s, i, message)
		return
	}
	d.respondAreaPayload(s, i, area)
}

func (d *Discord) handleProfileShow(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondProfilePayload(s, i, "")
}

func (d *Discord) respondProfilePayload(s *discordgo.Session, i *discordgo.InteractionCreate, selected string) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		d.respondDeferred(s, i)
	case discordgo.InteractionMessageComponent:
		d.respondDeferredUpdate(s, i)
	case discordgo.InteractionModalSubmit:
		d.respondDeferredEphemeral(s, i)
	}
	embed, components, mapReq, errText := d.buildProfilePayloadState(i, selected)
	if errText != "" {
		if i.Type == discordgo.InteractionApplicationCommand || i.Type == discordgo.InteractionMessageComponent || i.Type == discordgo.InteractionModalSubmit {
			d.respondEditMessage(s, i, errText, nil)
		} else {
			d.respondEphemeral(s, i, errText)
		}
		return
	}
	if i.Type == discordgo.InteractionModalSubmit && i.Message != nil {
		msg, err := d.updateMessageEmbed(s, i.Message.ChannelID, i.Message.ID, embed, components)
		d.registerSuccessfulSlashRender(s, msg, err, mapReq, embed)
		_ = s.InteractionResponseDelete(i.Interaction)
		return
	}
	msg, err := d.respondEditComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
	d.registerSuccessfulSlashRender(s, msg, err, mapReq, embed)
}

func (d *Discord) handleProfileSelect(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
	d.respondProfilePayload(s, i, value)
}

func (d *Discord) handleProfileSet(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
	tr := d.slashInteractionTranslator(i)
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, tr.Translate("Target is not registered.", false))
		return
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		d.respondEphemeral(s, i, tr.Translate("Target is not registered.", false))
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, tr.Translate("Unable to load profiles.", false))
		return
	}
	selected := profileRowByToken(profiles, value)
	if selected == nil {
		d.respondEphemeral(s, i, tr.Translate("Profile not found.", false))
		return
	}
	profileNo := toInt(selected["profile_no"], 0)
	quietHoursEnabled := toInt(human["schedule_disabled"], 0) == 0 && toInt(human["current_profile_no"], 1) == 0
	update := map[string]any{"preferred_profile_no": profileNo}
	if !quietHoursEnabled {
		update["current_profile_no"] = profileNo
	}
	update["area"] = selected["area"]
	update["latitude"] = selected["latitude"]
	update["longitude"] = selected["longitude"]
	if err := d.persistSlashHumanUpdate(userID, update); err != nil {
		d.respondEphemeral(s, i, tr.Translate("Failed to set profile.", false))
		return
	}
	d.respondProfilePayload(s, i, fmt.Sprintf("%d", profileNo))
}

func (d *Discord) handleProfileCreate(s *discordgo.Session, i *discordgo.InteractionCreate, name string) {
	tr := d.slashInteractionTranslator(i)
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "all") {
		d.respondEphemeral(s, i, tr.Translate("That is not a valid profile name.", false))
		return
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, tr.Translate("Target is not registered.", false))
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, tr.Translate("Unable to load profiles.", false))
		return
	}
	if profileNameExistsRows(profiles, name) {
		d.respondEphemeral(s, i, tr.Translate("That profile name already exists.", false))
		return
	}
	result := d.buildSlashExecutionResult(s, i, fmt.Sprintf("profile add %q", name))
	profiles, err = d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, tr.Translate("Unable to load profiles.", false))
		return
	}
	refreshProfile, message := profileCreateOutcome(profiles, name, result)
	if !refreshProfile {
		d.respondEphemeral(s, i, message)
		return
	}
	d.respondProfilePayload(s, i, name)
}

func (d *Discord) respondAreaPayload(s *discordgo.Session, i *discordgo.InteractionCreate, selected string) {
	switch i.Type {
	case discordgo.InteractionMessageComponent:
		d.respondDeferredUpdate(s, i)
	case discordgo.InteractionModalSubmit:
		d.respondDeferredEphemeral(s, i)
	}
	embed, components, mapReq, errText := d.buildAreaShowPayloadState(i, selected)
	if errText != "" {
		if i.Type == discordgo.InteractionMessageComponent || i.Type == discordgo.InteractionModalSubmit {
			d.respondEditMessage(s, i, errText, nil)
		} else {
			d.respondEphemeral(s, i, errText)
		}
		return
	}
	if i.Type == discordgo.InteractionModalSubmit && i.Message != nil {
		msg, err := d.updateMessageEmbed(s, i.Message.ChannelID, i.Message.ID, embed, components)
		d.registerSuccessfulSlashRender(s, msg, err, mapReq, embed)
		_ = s.InteractionResponseDelete(i.Interaction)
		return
	}
	if i.Type == discordgo.InteractionMessageComponent || i.Type == discordgo.InteractionModalSubmit {
		msg, err := d.respondEditComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
		d.registerSuccessfulSlashRender(s, msg, err, mapReq, embed)
		return
	}
	d.respondEphemeralComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleLocationInput(s *discordgo.Session, i *discordgo.InteractionCreate, input string) {
	tr := d.slashInteractionTranslator(i)
	input = strings.TrimSpace(input)
	prev := d.getSlashState(i.Member, i.User)
	if input == "" {
		d.respondEphemeral(s, i, tr.Translate("Please provide an address or coordinates.", false))
		return
	}
	if strings.EqualFold(input, "remove") {
		d.respondDeferredEphemeral(s, i)
		userID, _ := slashUser(i)
		if userID == "" || d.manager == nil || d.manager.query == nil {
			d.respondEditMessage(s, i, tr.Translate("Target is not registered.", false), nil)
			d.clearSlashState(i.Member, i.User)
			return
		}
		result := d.buildSlashExecutionResult(s, i, "location remove")
		human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
		if err != nil {
			d.respondEditMessage(s, i, tr.Translate("Target is not registered.", false), nil)
			d.clearSlashState(i.Member, i.User)
			return
		}
		refreshProfile, message, clearState := profileLocationModalRemoveOutcome(human, result)
		if clearState {
			defer d.clearSlashState(i.Member, i.User)
		}
		if !refreshProfile {
			d.respondEditMessage(s, i, message, nil)
			return
		}
		embed, components, mapReq, errText := d.buildProfilePayloadState(i, "")
		if errText != "" {
			d.respondEditMessage(s, i, errText, nil)
			return
		}
		if prev != nil && prev.OriginMessageID != "" && prev.OriginChannelID != "" {
			msg, err := d.updateMessageEmbed(s, prev.OriginChannelID, prev.OriginMessageID, embed, components)
			d.registerSuccessfulSlashRender(s, msg, err, mapReq, embed)
			_ = s.InteractionResponseDelete(i.Interaction)
			return
		}
		msg, err := d.respondEditComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
		d.registerSuccessfulSlashRender(s, msg, err, mapReq, embed)
		return
	}

	d.respondDeferredEphemeral(s, i)

	lat, lon, ok := parseLatLonString(input)
	placeConfirmation := ""
	if !ok {
		parts := strings.Fields(input)
		if len(parts) == 1 && !regexp.MustCompile(`^\d{1,5}$`).MatchString(parts[0]) {
			d.respondEditMessage(s, i, tr.Translate("Oops, you need to specify more than just a city name to locate accurately your position", false), nil)
			return
		}
		if d.manager == nil || d.manager.cfg == nil {
			d.respondEditMessage(s, i, tr.Translate("Geocoding is not configured.", false), nil)
			return
		}
		geo := webhook.NewGeocoder(d.manager.cfg)
		results := geo.Forward(input)
		if len(results) == 0 {
			d.respondEditMessage(s, i, "🙅", nil)
			return
		}
		lat = results[0].Latitude
		lon = results[0].Longitude
		if results[0].City != "" && results[0].Country != "" {
			placeConfirmation = fmt.Sprintf(" **%s - %s** ", results[0].City, results[0].Country)
		} else if results[0].Country != "" {
			placeConfirmation = fmt.Sprintf(" **%s** ", results[0].Country)
		}
	}
	if lat == 0 && lon == 0 {
		d.respondEditMessage(s, i, "🙅", nil)
		return
	}

	embed, components, mapReq := d.buildLocationConfirmPayloadState(i, lat, lon, placeConfirmation)
	state := &slashBuilderState{
		Command:   "location",
		Args:      []string{fmt.Sprintf("%s,%s", formatFloat(lat), formatFloat(lon))},
		Step:      "confirm",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	if prev != nil {
		state.OriginMessageID = prev.OriginMessageID
		state.OriginChannelID = prev.OriginChannelID
	}
	d.setSlashState(i.Member, i.User, state)
	if state.OriginMessageID != "" && state.OriginChannelID != "" {
		msg, err := d.updateMessageEmbed(s, state.OriginChannelID, state.OriginMessageID, embed, components)
		d.registerSuccessfulSlashRender(s, msg, err, mapReq, embed)
		_ = s.InteractionResponseDelete(i.Interaction)
		return
	}
	msg, err := d.respondEditComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
	d.registerSuccessfulSlashInteractionRender(s, i, msg, err, mapReq, embed)
}

func (d *Discord) handleSlashProfile(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.handleProfileShow(s, i)
}
