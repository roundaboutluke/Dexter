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
	_ = d.buildSlashReply(s, i, "location remove")
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
	reply := d.buildSlashReply(s, i, fmt.Sprintf("profile remove %s", profileValue))
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
	if reply != "" && !strings.EqualFold(reply, "done.") && !strings.EqualFold(reply, "profile removed.") {
		d.respondEphemeral(s, i, reply)
	}
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
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	selected := profileRowByToken(profiles, value)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
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
	if _, err := d.manager.query.UpdateQuery("humans", update, map[string]any{"id": userID}); err != nil {
		d.respondEphemeral(s, i, "Failed to set profile.")
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
	name = strings.TrimSpace(name)
	if name == "" || strings.EqualFold(name, "all") {
		d.respondEphemeral(s, i, "That is not a valid profile name.")
		return
	}
	userID, _ := slashUser(i)
	if userID == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
	if err != nil {
		d.respondEphemeral(s, i, "Unable to load profiles.")
		return
	}
	if profileNameExistsRows(profiles, name) {
		d.respondEphemeral(s, i, "That profile name already exists.")
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
	input = strings.TrimSpace(input)
	if input == "" {
		d.respondEphemeral(s, i, "Please provide an address or coordinates.")
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
			d.respondEditMessage(s, i, "Oops, you need to specify more than just a city name to locate accurately your position", nil)
			return
		}
		if d.manager == nil || d.manager.cfg == nil {
			d.respondEditMessage(s, i, "Geocoding is not configured.", nil)
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
	description := fmt.Sprintf("I set your location to the following coordinates in%s:\n%s", placeConfirmation, mapLink)
	embed := &discordgo.MessageEmbed{
		Title:       "Confirm location",
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
			discordgo.Button{CustomID: slashConfirmButton, Label: "Verify", Style: discordgo.SuccessButton},
			discordgo.Button{CustomID: slashCancelButton, Label: "Cancel", Style: discordgo.DangerButton},
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
