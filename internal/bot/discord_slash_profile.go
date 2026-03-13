package bot

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/tileserver"
	"poraclego/internal/webhook"
)

func profileDeleteConfirmFollowup(result slashExecutionResult) string {
	if result.Success() {
		return ""
	}
	return result.Reply
}

func profileLocationActionOutcome(result slashExecutionResult) (bool, string) {
	if result.Success() {
		return true, ""
	}
	return false, result.Reply
}

func (d *Discord) profileLocationModalText(i *discordgo.InteractionCreate) (string, string, string) {
	tr := d.slashInteractionTranslator(i)
	return tr.Translate("Set location", false), tr.Translate("Address or coordinates", false), "51.5,-0.12"
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
	embed, components, errText := d.buildAreaShowPayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileLocationClear(s *discordgo.Session, i *discordgo.InteractionCreate) {
	result := d.buildSlashExecutionResult(s, i, "location remove")
	refreshProfile, message := profileLocationActionOutcome(result)
	if !refreshProfile {
		d.respondEphemeral(s, i, message)
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
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
	result := d.buildSlashExecutionResult(s, i, fmt.Sprintf("profile remove %s", profileValue))
	if followup := profileDeleteConfirmFollowup(result); followup != "" {
		d.respondEphemeral(s, i, followup)
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileDeleteCancel(s *discordgo.Session, i *discordgo.InteractionCreate, profileValue string) {
	embed, components, errText := d.buildProfilePayload(i, profileValue)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) updateMessageEmbed(s *discordgo.Session, channelID, messageID string, embed *discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	if s == nil || channelID == "" || messageID == "" {
		return
	}
	edit := &discordgo.MessageEdit{
		ID:         messageID,
		Channel:    channelID,
		Embeds:     &[]*discordgo.MessageEmbed{embed},
		Components: &components,
	}
	_, _ = s.ChannelMessageEditComplex(edit)
}

func (d *Discord) handleAreaShowSelect(s *discordgo.Session, i *discordgo.InteractionCreate, area string) {
	embed, components, errText := d.buildAreaShowPayload(i, area)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleAreaShowToggle(s *discordgo.Session, i *discordgo.InteractionCreate, area string, add bool) {
	verb := "remove"
	if add {
		verb = "add"
	}
	_ = d.buildSlashReply(s, i, strings.TrimSpace(fmt.Sprintf("area %s %q", verb, area)))
	embed, components, errText := d.buildAreaShowPayload(i, area)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileShow(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	if i.Type == discordgo.InteractionApplicationCommand {
		d.respondComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components, false)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileSelect(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
	embed, components, errText := d.buildProfilePayload(i, value)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
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
	embed, components, errText := d.buildProfilePayload(i, fmt.Sprintf("%d", profileNo))
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
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
	_ = d.buildSlashReply(s, i, fmt.Sprintf("profile add %q", name))
	embed, components, errText := d.buildProfilePayload(i, name)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleLocationInput(s *discordgo.Session, i *discordgo.InteractionCreate, input string) {
	tr := d.slashInteractionTranslator(i)
	input = strings.TrimSpace(input)
	if input == "" {
		d.respondEphemeral(s, i, tr.Translate("Please provide an address or coordinates.", false))
		return
	}
	if strings.EqualFold(input, "remove") {
		d.executeSlashLineDeferred(s, i, "location remove")
		return
	}

	prev := d.getSlashState(i.Member, i.User)
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

	mapLink := fmt.Sprintf("https://maps.google.com/maps?q=%s,%s", formatFloat(lat), formatFloat(lon))
	description := tr.TranslateFormat("I set your location to the following coordinates in{0}:\n{1}", placeConfirmation, mapLink)
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate("Confirm location", false),
		Description: description,
	}
	if d.manager != nil && d.manager.cfg != nil {
		if provider, _ := d.manager.cfg.GetString("geocoding.staticProvider"); strings.EqualFold(provider, "tileservercache") {
			opts := tileserver.GetOptions(d.manager.cfg, "location")
			if !strings.EqualFold(opts.Type, "none") {
				client := tileserver.NewClient(d.manager.cfg)
				if staticMap, err := tileserver.GenerateConfiguredLocationTile(client, d.manager.cfg, lat, lon); err == nil && staticMap != "" {
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
	components := []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashConfirmButton, Label: tr.Translate("Verify", false), Style: discordgo.SuccessButton},
			discordgo.Button{CustomID: slashCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
		}},
	}
	if state.OriginMessageID != "" && state.OriginChannelID != "" {
		d.updateMessageEmbed(s, state.OriginChannelID, state.OriginMessageID, embed, components)
		_ = s.InteractionResponseDelete(i.Interaction)
		return
	}
	d.respondEditComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleSlashProfile(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.handleProfileShow(s, i)
}
