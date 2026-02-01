package bot

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/command"
	"poraclego/internal/config"
	"poraclego/internal/geofence"
	"poraclego/internal/logging"
	"poraclego/internal/scanner"
	"poraclego/internal/tileserver"
	"poraclego/internal/tracking"
	"poraclego/internal/webhook"
)

const (
	slashTrackTypeSelect             = "poracle:track:type"
	slashMonsterSearch               = "poracle:track:monster:search"
	slashMonsterSelect               = "poracle:track:monster:select"
	slashRaidInput                   = "poracle:track:raid:input"
	slashRaidLevelSelect             = "poracle:track:raid:level"
	slashEggLevelSelect              = "poracle:track:egg:level"
	slashQuestInput                  = "poracle:track:quest:input"
	slashInvasionInput               = "poracle:track:invasion:input"
	slashFiltersModal                = "poracle:track:filters"
	slashConfirmButton               = "poracle:track:confirm"
	slashCancelButton                = "poracle:track:cancel"
	slashChooseEverything            = "poracle:track:everything"
	slashChooseSearch                = "poracle:track:choose_search"
	slashAreaShowSelect              = "poracle:area:show:select"
	slashAreaShowAdd                 = "poracle:area:show:add:"
	slashAreaShowRemove              = "poracle:area:show:remove:"
	slashProfileSelect               = "poracle:profile:select"
	slashProfileSet                  = "poracle:profile:set:"
	slashProfileCreate               = "poracle:profile:create"
	slashProfileCreateMod            = "poracle:profile:create:modal"
	slashProfileScheduleAdd          = "poracle:profile:schedule:add:"
	slashProfileScheduleOverview     = "poracle:profile:schedule:overview"
	slashProfileScheduleAddGlobal    = "poracle:profile:schedule:add:all"
	slashProfileScheduleDay          = "poracle:profile:schedule:day:"
	slashProfileScheduleDayGlobal    = "poracle:profile:schedule:day:all"
	slashProfileScheduleTime         = "poracle:profile:schedule:time:"
	slashProfileScheduleAssign       = "poracle:profile:schedule:assign:"
	slashProfileScheduleEditGlobal   = "poracle:profile:schedule:edit:all"
	slashProfileScheduleEditDay      = "poracle:profile:schedule:edit:day:"
	slashProfileScheduleEditTime     = "poracle:profile:schedule:edit:time:"
	slashProfileScheduleEditAssign   = "poracle:profile:schedule:edit:assign:"
	slashProfileScheduleBack         = "poracle:profile:schedule:back:"
	slashProfileScheduleClear        = "poracle:profile:schedule:clear:"
	slashProfileScheduleRemove       = "poracle:profile:schedule:remove:"
	slashProfileScheduleRemoveGlobal = "poracle:profile:schedule:remove:all"
	slashProfileScheduleToggle       = "poracle:profile:schedule:toggle"
	slashProfileLocation             = "poracle:profile:location"
	slashProfileLocationClear        = "poracle:profile:location:clear"
	slashProfileLocationMod          = "poracle:profile:location:modal"
	slashProfileArea                 = "poracle:profile:area"
	slashProfileAreaBack             = "poracle:profile:area:back"
	slashProfileBack                 = "poracle:profile:back"
	slashProfileDelete               = "poracle:profile:delete:"
	slashProfileDeleteConfirm        = "poracle:profile:delete:confirm:"
	slashProfileDeleteCancel         = "poracle:profile:delete:cancel:"
	slashInfoCancelButton            = "poracle:info:cancel"
	slashInfoTranslateModal          = "poracle:info:translate"
	slashInfoWeatherModal            = "poracle:info:weather"
	slashInfoWeatherUseSaved         = "poracle:info:weather:saved"
	slashInfoWeatherEnterCoordinates = "poracle:info:weather:coords"
)

type slashBuilderState struct {
	Command         string
	Args            []string
	Step            string
	ExpiresAt       time.Time
	OriginMessageID string
	OriginChannelID string
}

func (d *Discord) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil {
		return
	}
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		d.handleSlashCommand(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		d.handleSlashAutocomplete(s, i)
	case discordgo.InteractionMessageComponent:
		d.handleSlashComponent(s, i)
	case discordgo.InteractionModalSubmit:
		d.handleSlashModal(s, i)
	}
}

func (d *Discord) handleSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name == "" {
		return
	}
	switch data.Name {
	case "track":
		d.handleSlashTrack(s, i)
	case "raid":
		d.handleSlashRaid(s, i)
	case "egg":
		d.handleSlashEgg(s, i)
	case "maxbattle":
		d.handleSlashMaxbattle(s, i)
	case "quest":
		d.handleSlashQuest(s, i)
	case "invasion":
		d.handleSlashIncident(s, i)
	case "gym":
		d.handleSlashGym(s, i)
	case "fort":
		d.handleSlashFort(s, i)
	case "nest":
		d.handleSlashNest(s, i)
	case "weather":
		d.handleSlashWeather(s, i)
	case "lure":
		d.handleSlashLure(s, i)
	case "profile":
		d.handleSlashProfile(s, i)
	case "tracked":
		d.handleSlashTracked(s, i)
	case "remove":
		d.handleSlashRemove(s, i)
	case "language":
		d.handleSlashLanguage(s, i)
	case "start":
		d.handleSlashStart(s, i)
	case "stop":
		d.handleSlashStop(s, i)
	case "help":
		d.handleSlashHelp(s, i)
	case "info":
		d.handleSlashInfo(s, i)
	}
}

func (d *Discord) handleSlashAutocomplete(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	if data.Name == "" {
		return
	}
	focused := focusedOption(data.Options)
	if focused == nil {
		return
	}

	options := slashOptions(data)
	query := ""
	if value, ok := focused.Value.(string); ok {
		query = value
	} else if value, ok := optionString(options, focused.Name); ok {
		query = value
	}

	var choices []*discordgo.ApplicationCommandOptionChoice
	switch data.Name {
	case "track":
		if focused.Name == "pokemon" {
			choices = d.autocompletePokemonChoices(query)
		} else if focused.Name == "form" {
			pokemon, _ := optionString(options, "pokemon")
			choices = d.autocompletePokemonFormChoices(query, pokemon)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "monster")
		}
	case "raid":
		if focused.Name == "type" {
			choices = d.autocompleteRaidTypeChoices(query)
		} else if focused.Name == "gym" {
			choices = d.autocompleteGymChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "raid")
		}
	case "maxbattle":
		if focused.Name == "type" {
			choices = d.autocompleteMaxbattleTypeChoices(query)
		} else if focused.Name == "station" {
			choices = d.autocompleteStationChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "maxbattle")
		}
	case "quest":
		if focused.Name == "type" {
			choices = d.autocompleteQuestTypeChoices(query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "quest")
		}
	case "egg":
		if focused.Name == "level" {
			choices = d.autocompleteRaidLevelChoices(query)
		} else if focused.Name == "gym" {
			choices = d.autocompleteGymChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "egg")
		}
	case "invasion":
		if focused.Name == "type" {
			choices = d.autocompleteIncidentTypeChoices(query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "invasion")
		}
	case "gym":
		if focused.Name == "gym" {
			choices = d.autocompleteGymChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "gym")
		}
	case "fort":
		if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "forts")
		}
	case "nest":
		if focused.Name == "pokemon" {
			choices = d.autocompletePokemonChoices(query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "nests")
		}
	case "weather":
		if focused.Name == "condition" {
			choices = d.autocompleteWeatherChoices(query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "weather")
		}
	case "lure":
		if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "lure")
		}
	case "language":
		if focused.Name == "to" {
			choices = d.autocompleteLanguageChoices(query)
		}
	case "info":
		if focused.Name == "pokemon" {
			choices = d.autocompleteInfoPokemonChoices(query)
		}
	case "remove":
		if focused.Name == "tracking" {
			trackingType, _ := optionString(options, "type")
			choices = d.autocompleteRemoveTrackingChoices(query, trackingType, i)
		}
	}

	if choices == nil {
		choices = []*discordgo.ApplicationCommandOptionChoice{}
	}
	_ = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionApplicationCommandAutocompleteResult,
		Data: &discordgo.InteractionResponseData{Choices: choices},
	})
}

func (d *Discord) handleSlashComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if data.CustomID == slashInfoCancelButton {
		d.respondEphemeral(s, i, "Canceled.")
		return
	}
	if data.CustomID == slashInfoWeatherUseSaved {
		d.executeSlashLineDeferred(s, i, "info weather")
		return
	}
	if data.CustomID == slashInfoWeatherEnterCoordinates {
		d.respondWithModal(s, i, slashInfoWeatherModal, "Weather info", "Coordinates (lat,lon)", "51.5,-0.12")
		return
	}
	if data.CustomID == slashAreaShowSelect {
		if len(data.Values) == 0 {
			return
		}
		d.handleAreaShowSelect(s, i, data.Values[0])
		return
	}
	if data.CustomID == slashProfileSelect {
		if len(data.Values) == 0 {
			return
		}
		d.handleProfileSelect(s, i, data.Values[0])
		return
	}
	if strings.HasPrefix(data.CustomID, slashAreaShowAdd) {
		area := strings.TrimPrefix(data.CustomID, slashAreaShowAdd)
		if strings.TrimSpace(area) != "" {
			d.handleAreaShowToggle(s, i, area, true)
		}
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileSet) {
		value := strings.TrimPrefix(data.CustomID, slashProfileSet)
		if strings.TrimSpace(value) != "" {
			d.handleProfileSet(s, i, value)
		}
		return
	}
	if data.CustomID == slashProfileScheduleAddGlobal {
		d.handleProfileScheduleAddGlobal(s, i)
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleAdd) {
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleAdd)
		if strings.TrimSpace(value) != "" {
			d.handleProfileScheduleAdd(s, i, value)
		}
		return
	}
	if data.CustomID == slashProfileScheduleOverview {
		d.handleProfileScheduleOverview(s, i)
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleBack) {
		d.handleProfileShow(s, i)
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleClear) {
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleClear)
		if strings.TrimSpace(value) != "" {
			d.handleProfileScheduleClear(s, i, value)
		}
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleRemoveGlobal) {
		if len(data.Values) == 0 {
			return
		}
		d.handleProfileScheduleRemoveGlobal(s, i, data.Values[0])
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleRemove) {
		if len(data.Values) == 0 {
			return
		}
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleRemove)
		if strings.TrimSpace(value) != "" {
			d.handleProfileScheduleRemove(s, i, value, data.Values[0])
		}
		return
	}
	if data.CustomID == slashProfileScheduleToggle {
		d.handleProfileScheduleToggle(s, i)
		return
	}
	if data.CustomID == slashProfileLocation {
		d.setSlashState(i.Member, i.User, &slashBuilderState{
			Step:            "profile-location",
			ExpiresAt:       time.Now().Add(5 * time.Minute),
			OriginMessageID: i.Message.ID,
			OriginChannelID: i.ChannelID,
		})
		d.respondWithModal(s, i, slashProfileLocationMod, "Set location", "Address or coordinates", "51.5,-0.12")
		return
	}
	if data.CustomID == slashProfileLocationClear {
		d.handleProfileLocationClear(s, i)
		return
	}
	if data.CustomID == slashProfileArea {
		d.handleProfileAreaShow(s, i)
		return
	}
	if data.CustomID == slashProfileAreaBack {
		d.handleProfileShow(s, i)
		return
	}
	if data.CustomID == slashProfileBack {
		d.handleProfileShow(s, i)
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileDeleteConfirm) {
		value := strings.TrimPrefix(data.CustomID, slashProfileDeleteConfirm)
		if strings.TrimSpace(value) != "" {
			d.handleProfileDeleteConfirm(s, i, value)
		}
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileDeleteCancel) {
		value := strings.TrimPrefix(data.CustomID, slashProfileDeleteCancel)
		d.handleProfileDeleteCancel(s, i, value)
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileDelete) {
		value := strings.TrimPrefix(data.CustomID, slashProfileDelete)
		if strings.TrimSpace(value) != "" {
			d.handleProfileDeletePrompt(s, i, value)
		}
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleEditGlobal) {
		if len(data.Values) == 0 {
			return
		}
		d.handleProfileScheduleEditSelect(s, i, data.Values[0])
		return
	}
	if data.CustomID == slashProfileScheduleDayGlobal {
		if len(data.Values) == 0 {
			return
		}
		d.handleProfileScheduleDayGlobal(s, i, data.Values)
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleDay) {
		if len(data.Values) == 0 {
			return
		}
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleDay)
		if strings.TrimSpace(value) != "" {
			d.handleProfileScheduleDay(s, i, value, data.Values[0])
		}
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleAssign) {
		if len(data.Values) == 0 {
			return
		}
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleAssign)
		d.handleProfileScheduleAssign(s, i, value, data.Values[0])
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleEditDay) {
		if len(data.Values) == 0 {
			return
		}
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleEditDay)
		d.handleProfileScheduleEditDay(s, i, value, data.Values[0])
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleEditAssign) {
		if len(data.Values) == 0 {
			return
		}
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleEditAssign)
		d.handleProfileScheduleEditAssign(s, i, value, data.Values[0])
		return
	}
	if strings.HasPrefix(data.CustomID, slashAreaShowRemove) {
		area := strings.TrimPrefix(data.CustomID, slashAreaShowRemove)
		if strings.TrimSpace(area) != "" {
			d.handleAreaShowToggle(s, i, area, false)
		}
		return
	}
	if data.CustomID == slashProfileCreate {
		d.respondWithModal(s, i, slashProfileCreateMod, "New profile", "Profile name", "home")
		return
	}

	state := d.getSlashState(i.Member, i.User)
	if state == nil || state.ExpiresAt.Before(time.Now()) {
		d.clearSlashState(i.Member, i.User)
		d.respondEphemeral(s, i, "Slash command expired. Please run the command again.")
		return
	}

	switch data.CustomID {
	case slashTrackTypeSelect:
		if len(data.Values) == 0 {
			return
		}
		switch data.Values[0] {
		case "monster":
			state.Command = "track"
			state.Step = "monster"
			d.respondWithMonsterOptions(s, i)
		case "raid":
			state.Command = "raid"
			state.Step = "raid"
			d.respondWithRaidOptions(s, i)
		case "egg":
			state.Command = "egg"
			state.Step = "egg"
			d.respondWithEggOptions(s, i)
		case "quest":
			state.Command = "quest"
			state.Step = "quest"
			d.respondWithQuestInput(s, i)
		case "invasion":
			state.Command = "invasion"
			state.Step = "invasion"
			d.respondWithInvasionInput(s, i)
		}
		return
	case slashChooseEverything:
		state.Args = []string{"everything"}
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashChooseSearch:
		switch state.Step {
		case "monster":
			d.respondWithMonsterSearch(s, i)
		case "raid":
			d.respondWithRaidInput(s, i)
		case "egg":
			d.respondWithEggLevelSelect(s, i)
		}
		return
	case slashMonsterSelect:
		if len(data.Values) == 0 {
			return
		}
		state.Args = []string{data.Values[0]}
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashRaidLevelSelect:
		if len(data.Values) == 0 {
			return
		}
		state.Args = []string{data.Values[0]}
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashEggLevelSelect:
		if len(data.Values) == 0 {
			return
		}
		state.Args = []string{data.Values[0]}
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashConfirmButton:
		if state != nil && state.Command == "location" {
			line := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
			_ = d.buildSlashReply(s, i, line)
			embed, components, errText := d.buildProfilePayload(i, "")
			if errText != "" {
				d.respondEphemeral(s, i, errText)
			} else {
				d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
			}
			d.clearSlashState(i.Member, i.User)
			return
		}
		// Clear the confirmation prompt buttons immediately, then send the command output as a follow-up.
		d.respondUpdateComponentsEmbed(s, i, "Confirmed ✅", nil, []discordgo.MessageComponent{})
		line := ""
		if state != nil {
			line = strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
		}
		reply := d.buildSlashReply(s, i, line)
		if reply == "" {
			reply = "Done."
		}
		d.followupEphemeralSlashReply(s, i, reply)
		d.clearSlashState(i.Member, i.User)
		return
	case slashCancelButton:
		if state != nil && state.Command == "location" {
			embed, components, errText := d.buildProfilePayload(i, "")
			if errText != "" {
				d.respondEphemeral(s, i, errText)
			} else {
				d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
			}
			d.clearSlashState(i.Member, i.User)
			return
		}
		// Clear the confirmation prompt buttons once canceled.
		d.respondUpdateComponentsEmbed(s, i, "Canceled.", nil, []discordgo.MessageComponent{})
		d.clearSlashState(i.Member, i.User)
		return
	case slashFiltersModal:
		d.respondWithFiltersInput(s, i)
		return
	}
}

func (d *Discord) handleSlashModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	if data.CustomID == slashInfoTranslateModal {
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, "Please enter text to translate.")
			return
		}
		d.executeSlashLineDeferred(s, i, "info translate "+query)
		return
	}
	if data.CustomID == slashInfoWeatherModal {
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, "Please enter coordinates (lat,lon).")
			return
		}
		d.executeSlashLineDeferred(s, i, "info weather "+query)
		return
	}
	if data.CustomID == slashProfileCreateMod {
		d.handleProfileCreate(s, i, strings.TrimSpace(modalTextValue(data, "query")))
		return
	}
	if data.CustomID == slashProfileLocationMod {
		d.handleLocationInput(s, i, strings.TrimSpace(modalTextValue(data, "query")))
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleTime) {
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleTime)
		d.handleProfileScheduleTime(s, i, value, data)
		return
	}
	if strings.HasPrefix(data.CustomID, slashProfileScheduleEditTime) {
		value := strings.TrimPrefix(data.CustomID, slashProfileScheduleEditTime)
		d.handleProfileScheduleEditTime(s, i, value, data)
		return
	}
	state := d.getSlashState(i.Member, i.User)
	if state == nil || state.ExpiresAt.Before(time.Now()) {
		d.clearSlashState(i.Member, i.User)
		d.respondEphemeral(s, i, "Slash command expired. Please run the command again.")
		return
	}
	switch data.CustomID {
	case slashMonsterSearch:
		query := modalTextValue(data, "query")
		if strings.TrimSpace(query) == "" {
			d.respondEphemeral(s, i, "Please enter a Pokemon name or ID.")
			return
		}
		if strings.EqualFold(strings.TrimSpace(query), "everything") {
			state.Args = []string{"everything"}
			d.respondWithFiltersPrompt(s, i, state)
			return
		}
		options := d.monsterSearchOptions(query)
		if len(options) == 0 {
			d.respondEphemeral(s, i, "No Pokemon matched that search.")
			return
		}
		d.respondWithSelectMenu(s, i, "Select a Pokemon", slashMonsterSelect, options)
	case slashRaidInput:
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, "Please enter a raid boss name or level.")
			return
		}
		if strings.EqualFold(query, "everything") {
			state.Args = []string{"everything"}
			d.respondWithFiltersPrompt(s, i, state)
			return
		}
		state.Args = []string{query}
		d.respondWithFiltersPrompt(s, i, state)
	case slashQuestInput:
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, "Please enter quest filters (e.g. reward:items).")
			return
		}
		state.Args = splitQuotedArgs(query)
		d.respondWithFiltersPrompt(s, i, state)
	case slashInvasionInput:
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, "Please enter invasion filters (e.g. grunt type).")
			return
		}
		state.Args = splitQuotedArgs(query)
		d.respondWithFiltersPrompt(s, i, state)
	case slashFiltersModal:
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query != "" {
			state.Args = append(state.Args, splitQuotedArgs(query)...)
		}
		d.respondWithFiltersPrompt(s, i, state)
	}
}

func (d *Discord) respondWithTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := []discordgo.SelectMenuOption{
		{Label: "Monster", Value: "monster"},
		{Label: "Raid", Value: "raid"},
		{Label: "Egg", Value: "egg"},
		{Label: "Quest", Value: "quest"},
		{Label: "Invasion", Value: "invasion"},
	}
	d.respondWithSelectMenu(s, i, "What do you want to track?", slashTrackTypeSelect, options)
}

func (d *Discord) respondWithMonsterOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithButtons(s, i, "Track a specific monster or everything?", []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseEverything, Label: "Everything", Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: slashChooseSearch, Label: "Search", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashCancelButton, Label: "Cancel", Style: discordgo.DangerButton},
	})
}

func (d *Discord) respondWithRaidOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithButtons(s, i, "Track raid boss, level, or everything?", []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseEverything, Label: "Everything", Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: slashChooseSearch, Label: "Boss/Level", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashCancelButton, Label: "Cancel", Style: discordgo.DangerButton},
	})
}

func (d *Discord) respondWithEggOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithButtons(s, i, "Track an egg level?", []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseSearch, Label: "Pick Level", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashCancelButton, Label: "Cancel", Style: discordgo.DangerButton},
	})
}

func (d *Discord) respondWithQuestInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithModal(s, i, slashQuestInput, "Quest filters", "Filters", "reward:items d500 clean")
}

func (d *Discord) respondWithInvasionInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithModal(s, i, slashInvasionInput, "Invasion filters", "Filters", "grunt_type:fire d500 clean")
}

func (d *Discord) respondWithMonsterSearch(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithModal(s, i, slashMonsterSearch, "Search Pokemon", "Name or ID", "bulbasaur")
}

func (d *Discord) respondWithRaidInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithModal(s, i, slashRaidInput, "Raid boss or level", "Boss or level", "rayquaza or level5")
}

func (d *Discord) respondWithEggLevelSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := d.raidLevelOptions()
	if len(options) == 0 {
		d.respondEphemeral(s, i, "No raid levels found.")
		return
	}
	d.respondWithSelectMenu(s, i, "Select egg level", slashEggLevelSelect, options)
}

func (d *Discord) respondWithFiltersInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithModal(s, i, slashFiltersModal, "Extra filters", "Args", "atk:15 def:15 sta:15 d500 clean")
}

func (d *Discord) respondWithFiltersPrompt(s *discordgo.Session, i *discordgo.InteractionCreate, state *slashBuilderState) {
	commandLine := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
	content := fmt.Sprintf("Ready to run `%s`", commandLine)
	d.respondWithButtons(s, i, content, []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashFiltersModal, Label: "Add filters", Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashConfirmButton, Label: "Verify", Style: discordgo.SuccessButton},
		discordgo.Button{CustomID: slashCancelButton, Label: "Cancel", Style: discordgo.DangerButton},
	})
}

func (d *Discord) confirmTitle(command string) string {
	switch strings.ToLower(command) {
	case "track":
		return "New Pokemon Alert:"
	case "raid":
		return "New Raid Alert:"
	case "egg":
		return "New Egg Alert:"
	case "maxbattle":
		return "New Max Battle Alert:"
	case "quest":
		return "New Quest Alert:"
	case "invasion":
		return "New Invasion Alert:"
	case "lure":
		return "New Lure Alert:"
	default:
		return "Confirm Command:"
	}
}

func (d *Discord) confirmFields(i *discordgo.InteractionCreate) []*discordgo.MessageEmbedField {
	if i == nil {
		return nil
	}
	data := i.ApplicationCommandData()
	options := slashOptions(data)
	if len(options) == 0 {
		return nil
	}

	inline := strings.EqualFold(data.Name, "track")
	fields := []*discordgo.MessageEmbedField{}

	findOption := func(name string) *discordgo.ApplicationCommandInteractionDataOption {
		for _, opt := range options {
			if opt.Name == name {
				return opt
			}
		}
		return nil
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		if strings.EqualFold(data.Name, "track") {
			if opt.Name == "pvp_ranks" {
				continue
			}
			if opt.Name == "pvp_league" {
				if ranks := findOption("pvp_ranks"); ranks != nil {
					fields = append(fields, &discordgo.MessageEmbedField{
						Name:   humanizeOptionName(opt.Name),
						Value:  d.formatConfirmValue(data.Name, opt.Name, opt.Value),
						Inline: inline,
					})
					fields = append(fields, &discordgo.MessageEmbedField{
						Name:   humanizeOptionName(ranks.Name),
						Value:  d.formatConfirmValue(data.Name, ranks.Name, ranks.Value),
						Inline: inline,
					})
					continue
				}
			}
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   humanizeOptionName(opt.Name),
			Value:  d.formatConfirmValue(data.Name, opt.Name, opt.Value),
			Inline: inline,
		})
	}
	return fields
}

func (d *Discord) formatConfirmValue(command, name string, value any) string {
	switch v := value.(type) {
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return ""
		}
		lower := strings.ToLower(text)
		if lower == "everything" {
			return "Everything"
		}
		if name == "form" {
			if lower == "all" {
				return "All forms"
			}
			return d.titleCase(text)
		}
		if name == "gym" {
			if label := d.gymLabel(text); label != "" {
				return label
			}
		}
		if command == "maxbattle" && name == "station" {
			if label := d.stationLabel(text); label != "" {
				return label
			}
		}
		if command == "invasion" && name == "type" {
			return d.invasionTypeLabel(text)
		}
		if command == "quest" && name == "type" {
			return d.questTypeLabel(text)
		}
		if name == "pokemon" || (command == "raid" && name == "type") || (command == "quest" && name == "type") {
			if id, ok := parseIntString(lower); ok {
				return d.pokemonLabel(id)
			}
		}
		if (command == "egg" && name == "level") || (command == "raid" && name == "type") {
			if level, ok := parseLevelString(lower); ok {
				return d.raidLevelLabel(level)
			}
		}
		if command == "maxbattle" && name == "type" {
			if level, ok := parseLevelString(lower); ok {
				return d.maxbattleLevelLabel(level)
			}
		}
		return text
	case bool:
		if v {
			return "Yes"
		}
		return "No"
	case float64:
		return d.formatConfirmValue(command, name, strconv.Itoa(int(v)))
	case int:
		return d.formatConfirmValue(command, name, strconv.Itoa(v))
	case int64:
		return d.formatConfirmValue(command, name, strconv.FormatInt(v, 10))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func humanizeOptionName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	parts := strings.Fields(strings.ReplaceAll(name, "_", " "))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		switch strings.ToLower(part) {
		case "iv":
			out = append(out, "IV")
		case "cp":
			out = append(out, "CP")
		case "pvp":
			out = append(out, "PVP")
		case "gmax":
			out = append(out, "GMax")
		default:
			out = append(out, titleCaseWords(part))
		}
	}
	return strings.Join(out, " ")
}

func (d *Discord) pokemonLabel(id int) string {
	name := d.pokemonName(id)
	if name == "" {
		return fmt.Sprintf("%d", id)
	}
	return fmt.Sprintf("%s (#%d)", name, id)
}

func (d *Discord) invasionTypeLabel(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	if lower == "everything" {
		return "Everything"
	}
	if d.manager != nil && d.manager.data != nil {
		for _, raw := range d.manager.data.Grunts {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if grunt, ok := entry["grunt"].(string); ok {
				if strings.EqualFold(grunt, text) {
					return titleCaseWords(grunt)
				}
			}
			if typ, ok := entry["type"].(string); ok {
				if strings.EqualFold(typ, text) {
					return titleCaseWords(typ)
				}
			}
		}
		if d.manager.data.UtilData != nil {
			if raw, ok := d.manager.data.UtilData["pokestopEvent"].(map[string]any); ok {
				for _, value := range raw {
					if entry, ok := value.(map[string]any); ok {
						if name, ok := entry["name"].(string); ok {
							if strings.EqualFold(name, text) {
								return titleCaseWords(name)
							}
						}
					}
				}
			}
		}
	}
	return titleCaseWords(text)
}

func (d *Discord) questTypeLabel(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	if lower == "everything" {
		return "Everything"
	}
	if lower == "candy" {
		return "Rare Candy"
	}
	if strings.HasPrefix(lower, "candy:") {
		mon := strings.TrimSpace(text[len("candy:"):])
		if name := d.questMonsterLabel(mon); name != "" {
			return fmt.Sprintf("%s Candy", name)
		}
		return fmt.Sprintf("%s Candy", titleCaseWords(mon))
	}
	if lower == "xl candy" || lower == "xlcandy" {
		return "Rare Candy XL"
	}
	if lower == "stardust" {
		return "Stardust"
	}
	if lower == "experience" {
		return "Experience"
	}
	if lower == "energy" {
		return "Mega Energy"
	}
	if strings.HasPrefix(lower, "energy:") {
		mon := strings.TrimSpace(text[len("energy:"):])
		if name := d.questMonsterLabel(mon); name != "" {
			return fmt.Sprintf("Mega Energy %s", name)
		}
		return fmt.Sprintf("Mega Energy %s", titleCaseWords(mon))
	}
	if strings.HasPrefix(lower, "xlcandy:") {
		mon := strings.TrimSpace(text[len("xlcandy:"):])
		if name := d.questMonsterLabel(mon); name != "" {
			return fmt.Sprintf("%s XL Candy", name)
		}
		return fmt.Sprintf("%s XL Candy", titleCaseWords(mon))
	}
	if strings.HasPrefix(lower, "form:") {
		form := strings.TrimSpace(text[len("form:"):])
		return fmt.Sprintf("Form %s", titleCaseWords(form))
	}
	if strings.Contains(lower, " form:") {
		parts := strings.SplitN(text, "form:", 2)
		mon := strings.TrimSpace(parts[0])
		form := strings.TrimSpace(parts[1])
		monLabel := d.questMonsterLabel(mon)
		if monLabel == "" {
			monLabel = titleCaseWords(mon)
		}
		if form != "" {
			return fmt.Sprintf("%s (%s)", monLabel, titleCaseWords(form))
		}
		return monLabel
	}
	if name := d.questMonsterLabel(text); name != "" {
		return name
	}
	return titleCaseWords(text)
}

func (d *Discord) pokemonName(id int) string {
	if d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil || id <= 0 {
		return ""
	}
	if raw, ok := d.manager.data.Monsters[fmt.Sprintf("%d_0", id)]; ok {
		if mon, ok := raw.(map[string]any); ok {
			name := strings.TrimSpace(fmt.Sprintf("%v", mon["name"]))
			if name != "" {
				return name
			}
		}
	}
	if raw, ok := d.manager.data.Monsters[strconv.Itoa(id)]; ok {
		if mon, ok := raw.(map[string]any); ok {
			name := strings.TrimSpace(fmt.Sprintf("%v", mon["name"]))
			if name != "" {
				return name
			}
		}
	}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if toInt(mon["id"], 0) != id {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", mon["name"]))
		if name != "" {
			return name
		}
	}
	return ""
}

func (d *Discord) pokemonIDFromValue(value string) int {
	value = strings.TrimSpace(value)
	if value == "" || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return 0
	}
	if id, ok := parseIntString(value); ok {
		return id
	}
	query := strings.ToLower(value)
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", mon["name"])))
		if name != "" && name == query {
			return toInt(mon["id"], 0)
		}
	}
	return 0
}

func (d *Discord) pokemonFormNames(id int) []string {
	if id <= 0 || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return nil
	}
	baseForm := false
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if toInt(mon["id"], 0) != id {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) == 0 {
			baseForm = true
			break
		}
	}
	seen := map[string]bool{}
	names := []string{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if toInt(mon["id"], 0) != id {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		name := strings.TrimSpace(fmt.Sprintf("%v", form["name"]))
		if name == "" {
			continue
		}
		if baseForm && strings.EqualFold(name, "normal") {
			continue
		}
		lower := strings.ToLower(name)
		if seen[lower] {
			continue
		}
		seen[lower] = true
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

func (d *Discord) gymLabel(gymID string) string {
	if d.manager != nil && d.manager.scanner != nil {
		if name, err := d.manager.scanner.GetGymName(gymID); err == nil && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return strings.TrimSpace(gymID)
}

func (d *Discord) stationLabel(stationID string) string {
	if d.manager != nil && d.manager.scanner != nil {
		if name, err := d.manager.scanner.GetStationName(stationID); err == nil && strings.TrimSpace(name) != "" {
			return strings.TrimSpace(name)
		}
	}
	return strings.TrimSpace(stationID)
}

func parseIntString(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	id, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return id, true
}

func parseLevelString(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if strings.HasPrefix(value, "level") {
		value = strings.TrimSpace(strings.TrimPrefix(value, "level"))
	}
	level, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return level, true
}

func (d *Discord) promptSlashConfirmation(s *discordgo.Session, i *discordgo.InteractionCreate, command string, args []string, title string, fields []*discordgo.MessageEmbedField) {
	state := &slashBuilderState{
		Command:   command,
		Args:      args,
		Step:      "confirm",
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
	d.setSlashState(i.Member, i.User, state)
	if len(fields) == 0 {
		commandLine := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
		fields = []*discordgo.MessageEmbedField{
			{Name: "command", Value: commandLine, Inline: false},
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:  title,
		Fields: fields,
	}
	ephemeral := strings.EqualFold(command, "track")
	d.respondComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashConfirmButton, Label: "Verify", Style: discordgo.SuccessButton},
			discordgo.Button{CustomID: slashCancelButton, Label: "Cancel", Style: discordgo.DangerButton},
		}},
	}, ephemeral)
}

func (d *Discord) respondWithSelectMenu(s *discordgo.Session, i *discordgo.InteractionCreate, text, customID string, options []discordgo.SelectMenuOption) {
	min := 1
	menu := discordgo.SelectMenu{
		CustomID:    customID,
		Options:     options,
		Placeholder: text,
		MaxValues:   1,
		MinValues:   &min,
	}
	d.respondEphemeralComponents(s, i, text, []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{menu}},
	})
}

func (d *Discord) respondWithButtons(s *discordgo.Session, i *discordgo.InteractionCreate, text string, buttons []discordgo.MessageComponent) {
	d.respondEphemeralComponents(s, i, text, []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: buttons},
	})
}

func (d *Discord) respondEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, text string) {
	d.respondEphemeralComponents(s, i, text, nil)
}

func (d *Discord) respondEphemeralComponents(s *discordgo.Session, i *discordgo.InteractionCreate, text string, components []discordgo.MessageComponent) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    text,
			Flags:      discordgo.MessageFlagsEphemeral,
			Components: components,
		},
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction respond failed: %v", err)
		}
	}
}

func (d *Discord) respondDeferredEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord deferred interaction respond failed: %v", err)
		}
	}
}

func (d *Discord) respondDeferred(s *discordgo.Session, i *discordgo.InteractionCreate) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord deferred interaction respond failed: %v", err)
		}
	}
}

func (d *Discord) respondComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent, ephemeral bool) {
	flags := discordgo.MessageFlags(0)
	if ephemeral {
		flags = discordgo.MessageFlagsEphemeral
	}
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content:    text,
			Embeds:     embeds,
			Flags:      flags,
			Components: components,
		},
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction respond failed: %v", err)
		}
	}
}

func (d *Discord) respondEphemeralComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	d.respondComponentsEmbed(s, i, text, embeds, components, true)
}

func (d *Discord) respondEditMessage(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed) {
	content := text
	var embedPtr *[]*discordgo.MessageEmbed
	if embeds != nil {
		embedPtr = &embeds
	}
	edit := &discordgo.WebhookEdit{
		Content: &content,
		Embeds:  embedPtr,
	}
	if _, err := s.InteractionResponseEdit(i.Interaction, edit); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction edit failed: %v", err)
		}
	}
}

func (d *Discord) followupEphemeralSlashReply(s *discordgo.Session, i *discordgo.InteractionCreate, reply string) {
	if i == nil || i.Interaction == nil {
		return
	}
	if d.sendSpecialSlashFollowup(s, i, reply) {
		return
	}
	const discordLimit = 1900
	for _, chunk := range splitDiscordMessage(reply, discordLimit) {
		if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: chunk}); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup failed: %v", err)
			}
			return
		}
	}
}

func (d *Discord) sendSpecialSlashFollowup(s *discordgo.Session, i *discordgo.InteractionCreate, reply string) bool {
	if strings.HasPrefix(reply, command.FileReplyPrefix) {
		raw := strings.TrimPrefix(reply, command.FileReplyPrefix)
		var payload struct {
			Name    string `json:"name"`
			Message string `json:"message"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		if payload.Name == "" || payload.Content == "" {
			return false
		}
		if payload.Message != "" {
			d.followupEphemeralSlashReply(s, i, payload.Message)
		}
		msg := &discordgo.WebhookParams{
			Files: []*discordgo.File{
				{Name: payload.Name, Reader: strings.NewReader(payload.Content)},
			},
		}
		if _, err := s.FollowupMessageCreate(i.Interaction, true, msg); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup (file) failed: %v", err)
			}
		}
		return true
	}
	if strings.HasPrefix(reply, command.DiscordEmbedPrefix) {
		raw := strings.TrimPrefix(reply, command.DiscordEmbedPrefix)
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		content := ""
		if value, ok := payload["content"].(string); ok {
			content = value
		}
		var embeds []*discordgo.MessageEmbed
		if embedRaw, ok := payload["embed"]; ok {
			if embed := decodeEmbed(embedRaw); embed != nil {
				embeds = []*discordgo.MessageEmbed{embed}
			}
		}
		if embedsRaw, ok := payload["embeds"]; ok {
			if parsed := decodeEmbeds(embedsRaw); len(parsed) > 0 {
				embeds = parsed
			}
		}
		if content == "" && len(embeds) == 0 {
			return false
		}
		msg := &discordgo.WebhookParams{
			Content: content,
			Embeds:  embeds,
		}
		if _, err := s.FollowupMessageCreate(i.Interaction, true, msg); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup (embed) failed: %v", err)
			}
		}
		return true
	}
	return false
}

func (d *Discord) respondEditComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	content := text
	var embedPtr *[]*discordgo.MessageEmbed
	if embeds != nil {
		embedPtr = &embeds
	}
	var componentPtr *[]discordgo.MessageComponent
	if components != nil {
		componentPtr = &components
	}
	edit := &discordgo.WebhookEdit{
		Content:    &content,
		Embeds:     embedPtr,
		Components: componentPtr,
	}
	_, _ = s.InteractionResponseEdit(i.Interaction, edit)
}

func (d *Discord) respondUpdateComponentsEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, text string, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	response := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Content:    text,
			Embeds:     embeds,
			Components: components,
		},
	}
	if err := s.InteractionRespond(i.Interaction, response); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction update failed: %v", err)
		}
	}
}

func (d *Discord) sendSpecialSlashReply(s *discordgo.Session, i *discordgo.InteractionCreate, reply string) bool {
	if strings.HasPrefix(reply, command.FileReplyPrefix) {
		raw := strings.TrimPrefix(reply, command.FileReplyPrefix)
		var payload struct {
			Name    string `json:"name"`
			Message string `json:"message"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		if payload.Name == "" || payload.Content == "" {
			return false
		}
		if payload.Message != "" {
			d.respondEditMessage(s, i, payload.Message, nil)
		} else {
			d.respondEditMessage(s, i, "", nil)
		}
		msg := &discordgo.WebhookParams{
			Files: []*discordgo.File{
				{
					Name:   payload.Name,
					Reader: strings.NewReader(payload.Content),
				},
			},
		}
		if _, err := s.FollowupMessageCreate(i.Interaction, true, msg); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup (file) failed: %v", err)
			}
		}
		return true
	}
	if strings.HasPrefix(reply, command.DiscordEmbedPrefix) {
		raw := strings.TrimPrefix(reply, command.DiscordEmbedPrefix)
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			return false
		}
		content := ""
		if value, ok := payload["content"].(string); ok {
			content = value
		}
		var embeds []*discordgo.MessageEmbed
		if embedRaw, ok := payload["embed"]; ok {
			if embed := decodeEmbed(embedRaw); embed != nil {
				embeds = []*discordgo.MessageEmbed{embed}
			}
		}
		if embedsRaw, ok := payload["embeds"]; ok {
			if parsed := decodeEmbeds(embedsRaw); len(parsed) > 0 {
				embeds = parsed
			}
		}
		if content == "" && len(embeds) == 0 {
			return false
		}
		d.respondEditMessage(s, i, content, embeds)
		return true
	}
	return false
}

func (d *Discord) respondWithModal(s *discordgo.Session, i *discordgo.InteractionCreate, customID, title, label, placeholder string) {
	component := discordgo.TextInput{
		CustomID:    "query",
		Label:       label,
		Style:       discordgo.TextInputShort,
		Placeholder: placeholder,
		Required:    true,
	}
	modal := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{component}},
			},
		},
	}
	if err := s.InteractionRespond(i.Interaction, modal); err != nil {
		if logger := logging.Get().Discord; logger != nil {
			logger.Warnf("Discord interaction modal respond failed: %v", err)
		}
	}
}

func (d *Discord) respondWithScheduleModal(s *discordgo.Session, i *discordgo.InteractionCreate, customID, startPlaceholder, endPlaceholder, startValue, endValue string) {
	startInput := discordgo.TextInput{
		CustomID:    "start",
		Label:       "Start time (HH:MM)",
		Style:       discordgo.TextInputShort,
		Placeholder: startPlaceholder,
		Value:       startValue,
		Required:    true,
	}
	endInput := discordgo.TextInput{
		CustomID:    "end",
		Label:       "End time (HH:MM)",
		Style:       discordgo.TextInputShort,
		Placeholder: endPlaceholder,
		Value:       endValue,
		Required:    true,
	}
	modal := &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: customID,
			Title:    "Add schedule",
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{startInput}},
				discordgo.ActionsRow{Components: []discordgo.MessageComponent{endInput}},
			},
		},
	}
	_ = s.InteractionRespond(i.Interaction, modal)
}

func modalTextValue(data discordgo.ModalSubmitInteractionData, customID string) string {
	fallback := ""
	for _, row := range data.Components {
		var components []discordgo.MessageComponent
		switch actions := row.(type) {
		case discordgo.ActionsRow:
			components = actions.Components
		case *discordgo.ActionsRow:
			components = actions.Components
		default:
			continue
		}
		for _, comp := range components {
			switch input := comp.(type) {
			case discordgo.TextInput:
				if fallback == "" {
					fallback = input.Value
				}
				if input.CustomID == customID {
					return input.Value
				}
			case *discordgo.TextInput:
				if fallback == "" {
					fallback = input.Value
				}
				if input.CustomID == customID {
					return input.Value
				}
			}
		}
	}
	return fallback
}

func slashOptions(data discordgo.ApplicationCommandInteractionData) []*discordgo.ApplicationCommandInteractionDataOption {
	if len(data.Options) == 0 {
		return nil
	}
	if data.Options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return data.Options[0].Options
	}
	if data.Options[0].Type == discordgo.ApplicationCommandOptionSubCommandGroup && len(data.Options[0].Options) > 0 {
		return data.Options[0].Options[0].Options
	}
	return data.Options
}

func slashSubcommand(data discordgo.ApplicationCommandInteractionData) string {
	if len(data.Options) == 0 {
		return ""
	}
	if data.Options[0].Type == discordgo.ApplicationCommandOptionSubCommand {
		return data.Options[0].Name
	}
	return ""
}

func focusedOption(options []*discordgo.ApplicationCommandInteractionDataOption) *discordgo.ApplicationCommandInteractionDataOption {
	for _, opt := range options {
		if opt.Focused {
			return opt
		}
		if len(opt.Options) > 0 {
			if child := focusedOption(opt.Options); child != nil {
				return child
			}
		}
	}
	return nil
}

func optionString(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (string, bool) {
	for _, opt := range options {
		if opt.Name != name {
			continue
		}
		if value, ok := opt.Value.(string); ok {
			return value, true
		}
	}
	return "", false
}

func optionInt(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (int, bool) {
	for _, opt := range options {
		if opt.Name != name {
			continue
		}
		switch value := opt.Value.(type) {
		case float64:
			return int(value), true
		case int:
			return value, true
		case int64:
			return int(value), true
		}
	}
	return 0, false
}

func optionBool(options []*discordgo.ApplicationCommandInteractionDataOption, name string) (bool, bool) {
	for _, opt := range options {
		if opt.Name != name {
			continue
		}
		if value, ok := opt.Value.(bool); ok {
			return value, true
		}
	}
	return false, false
}

func appendRangeArg(args []string, prefix, maxPrefix string, minVal, maxVal *int) []string {
	if minVal == nil && maxVal == nil {
		return args
	}
	if minVal != nil && maxVal != nil {
		if *minVal == *maxVal {
			return append(args, fmt.Sprintf("%s%d", prefix, *minVal))
		}
		return append(args, fmt.Sprintf("%s%d-%d", prefix, *minVal, *maxVal))
	}
	if minVal != nil {
		return append(args, fmt.Sprintf("%s%d", prefix, *minVal))
	}
	return append(args, fmt.Sprintf("%s%d", maxPrefix, *maxVal))
}

func (d *Discord) executeSlashCommand(s *discordgo.Session, i *discordgo.InteractionCreate, state *slashBuilderState) {
	if state == nil {
		d.respondEphemeral(s, i, "No command to run.")
		return
	}
	line := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
	if line == "" {
		d.respondEphemeral(s, i, "No command to run.")
		return
	}
	d.executeSlashLineDeferred(s, i, line)
}

func (d *Discord) executeSlashLine(s *discordgo.Session, i *discordgo.InteractionCreate, line string) {
	reply := d.buildSlashReply(s, i, line)
	if reply == "" {
		reply = "Done."
	}
	if d.sendSpecialSlashReply(s, i, reply) {
		return
	}
	const discordLimit = 1900
	chunks := splitDiscordMessage(reply, discordLimit)
	d.respondEphemeral(s, i, chunks[0])
	for _, chunk := range chunks[1:] {
		content := chunk
		if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: content}); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup failed: %v", err)
			}
			return
		}
	}
}

func (d *Discord) executeSlashLineDeferred(s *discordgo.Session, i *discordgo.InteractionCreate, line string) {
	d.respondDeferredEphemeral(s, i)
	reply := d.buildSlashReply(s, i, line)
	if reply == "" {
		reply = "Done."
	}
	if d.sendSpecialSlashReply(s, i, reply) {
		return
	}
	const discordLimit = 1900
	chunks := splitDiscordMessage(reply, discordLimit)
	d.respondEditMessage(s, i, chunks[0], nil)
	for _, chunk := range chunks[1:] {
		content := chunk
		if _, err := s.FollowupMessageCreate(i.Interaction, true, &discordgo.WebhookParams{Content: content}); err != nil {
			if logger := logging.Get().Discord; logger != nil {
				logger.Warnf("Discord followup failed: %v", err)
			}
			return
		}
	}
}

func (d *Discord) buildSlashReply(s *discordgo.Session, i *discordgo.InteractionCreate, line string) string {
	if line == "" {
		return "No command to run."
	}
	channelName := ""
	isDM := i.GuildID == ""
	if channel, err := s.Channel(i.ChannelID); err == nil && channel != nil {
		channelName = channel.Name
		if channel.Type == discordgo.ChannelTypeDM || channel.Type == discordgo.ChannelTypeGroupDM {
			isDM = true
		}
	}
	roles := []string{}
	userID, userName := slashUser(i)
	isAdmin := containsID(d.manager.cfg, "discord.admins", userID)
	if isDM {
		roles = d.rolesForDM(s, userID)
	} else if channelName != "" {
		if member, err := s.GuildMember(i.GuildID, userID); err == nil && member != nil {
			roles = append(roles, member.Roles...)
		}
	}
	ctx := d.manager.Context("discord", "", "/", userID, userName, i.ChannelID, channelName, isDM, isAdmin, roles, ".")
	tokens := splitQuotedArgs(line)
	if len(tokens) > 0 {
		if disabled, ok := ctx.Config.GetStringSlice("general.disabledCommands"); ok {
			if containsString(disabled, tokens[0]) {
				return "That command is disabled."
			}
		}
	}
	reply, err := d.manager.Registry().Execute(ctx, line)
	if err != nil {
		return err.Error()
	}
	if reply == "" {
		return "Done."
	}
	return reply
}

func (d *Discord) handleSlashTrack(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	pokemon, ok := optionString(options, "pokemon")
	if !ok || strings.TrimSpace(pokemon) == "" {
		d.respondEphemeral(s, i, "Please pick a Pokemon.")
		return
	}
	pokemon = strings.TrimSpace(pokemon)
	args := []string{pokemon}
	if strings.EqualFold(pokemon, "everything") {
		args = []string{"everything"}
	}

	if value, ok := optionString(options, "gender"); ok {
		switch strings.ToLower(value) {
		case "male", "female":
			args = append(args, strings.ToLower(value))
		}
	}
	if value, ok := optionString(options, "size"); ok && !strings.EqualFold(value, "all") {
		args = append(args, "size"+strings.ToLower(value))
	}

	minIV := optionalInt(options, "min_iv")
	maxIV := optionalInt(options, "max_iv")
	args = appendRangeArg(args, "iv", "maxiv", minIV, maxIV)

	minAtk := optionalInt(options, "min_atk")
	maxAtk := optionalInt(options, "max_atk")
	args = appendRangeArg(args, "atk", "maxatk", minAtk, maxAtk)

	minDef := optionalInt(options, "min_def")
	maxDef := optionalInt(options, "max_def")
	args = appendRangeArg(args, "def", "maxdef", minDef, maxDef)

	minSta := optionalInt(options, "min_sta")
	maxSta := optionalInt(options, "max_sta")
	args = appendRangeArg(args, "sta", "maxsta", minSta, maxSta)

	minCP := optionalInt(options, "min_cp")
	maxCP := optionalInt(options, "max_cp")
	args = appendRangeArg(args, "cp", "maxcp", minCP, maxCP)

	minLevel := optionalInt(options, "min_level")
	maxLevel := optionalInt(options, "max_level")
	args = appendRangeArg(args, "level", "maxlevel", minLevel, maxLevel)

	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionInt(options, "min_time"); ok && value > 0 {
		args = append(args, fmt.Sprintf("t%d", value))
	}

	if value, ok := optionString(options, "pvp_league"); ok {
		if ranks, ok := optionInt(options, "pvp_ranks"); ok && ranks > 0 {
			league := strings.ToLower(value)
			if league == "great" || league == "ultra" || league == "little" {
				args = append(args, fmt.Sprintf("%s1-%d", league, ranks))
			}
		}
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	if value, ok := optionString(options, "form"); ok && strings.TrimSpace(value) != "" {
		formValue := strings.TrimSpace(value)
		if strings.EqualFold(formValue, "all") {
			args = append(args, "form:all")
		} else {
			args = append(args, "form:"+formValue)
		}
	} else if !strings.EqualFold(pokemon, "everything") {
		args = append(args, "form:all")
	}

	d.promptSlashConfirmation(s, i, "track", args, d.confirmTitle("track"), d.confirmFields(i))
}

func (d *Discord) handleSlashRaid(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	raidType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(raidType) == "" {
		d.respondEphemeral(s, i, "Please pick a raid type.")
		return
	}
	args := []string{normalizeRaidType(strings.TrimSpace(raidType))}
	if value, ok := optionString(options, "team"); ok {
		switch strings.ToLower(value) {
		case "blue", "red", "yellow", "white":
			args = append(args, strings.ToLower(value))
		}
	}
	if value, ok := optionString(options, "rsvp"); ok {
		switch strings.ToLower(value) {
		case "on":
			args = append(args, "rsvp")
		case "only":
			args = append(args, "rsvp", "only")
		case "off":
			args = append(args, "no", "rsvp")
		}
	}
	if value, ok := optionString(options, "gym"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "gym:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "raid", args, d.confirmTitle("raid"), d.confirmFields(i))
}

func (d *Discord) handleSlashMaxbattle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	mbType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(mbType) == "" {
		d.respondEphemeral(s, i, "Please pick a max battle type.")
		return
	}
	args := []string{normalizeRaidType(strings.TrimSpace(mbType))}
	if value, ok := optionBool(options, "gmax_only"); ok && value {
		args = append(args, "gmax")
	}
	if value, ok := optionString(options, "station"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "station:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "maxbattle", args, d.confirmTitle("maxbattle"), d.confirmFields(i))
}

func (d *Discord) handleSlashEgg(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	level, ok := optionString(options, "level")
	if !ok || strings.TrimSpace(level) == "" {
		d.respondEphemeral(s, i, "Please pick an egg level.")
		return
	}
	args := []string{}
	if ok && strings.TrimSpace(level) != "" {
		args = append(args, normalizeRaidType(strings.TrimSpace(level)))
	}
	if value, ok := optionString(options, "team"); ok {
		switch strings.ToLower(value) {
		case "blue", "red", "yellow", "white":
			args = append(args, strings.ToLower(value))
		}
	}
	if value, ok := optionString(options, "rsvp"); ok {
		switch strings.ToLower(value) {
		case "on":
			args = append(args, "rsvp")
		case "only":
			args = append(args, "rsvp", "only")
		case "off":
			args = append(args, "no", "rsvp")
		}
	}
	if value, ok := optionString(options, "gym"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "gym:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "egg", args, d.confirmTitle("egg"), d.confirmFields(i))
}

func (d *Discord) handleSlashQuest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	questType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(questType) == "" {
		d.respondEphemeral(s, i, "Please pick a quest reward.")
		return
	}
	questType = strings.TrimSpace(questType)
	if minAmount, ok := optionInt(options, "min_amount"); ok && minAmount > 0 {
		if strings.HasPrefix(strings.ToLower(questType), "stardust") {
			questType = fmt.Sprintf("stardust%d", minAmount)
		}
	}
	args := []string{formatQuestArg(questType)}
	if value, ok := optionString(options, "ar"); ok {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "ar":
			args = append(args, "ar")
		case "noar":
			args = append(args, "noar")
		}
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "quest", args, d.confirmTitle("quest"), d.confirmFields(i))
}

func (d *Discord) handleSlashIncident(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	incidentType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(incidentType) == "" {
		d.respondEphemeral(s, i, "Please pick an invasion type.")
		return
	}
	args := []string{formatInvasionArg(strings.TrimSpace(incidentType))}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "invasion", args, d.confirmTitle("invasion"), d.confirmFields(i))
}

func (d *Discord) handleSlashGym(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	team, ok := optionString(options, "team")
	if !ok || strings.TrimSpace(team) == "" {
		d.respondEphemeral(s, i, "Please pick a team.")
		return
	}
	args := []string{strings.ToLower(strings.TrimSpace(team))}
	if value, ok := optionBool(options, "slot_changes"); ok && value {
		args = append(args, "slot_changes")
	}
	if value, ok := optionBool(options, "battle_changes"); ok && value {
		args = append(args, "battle_changes")
	}
	if value, ok := optionString(options, "gym"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "gym:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "gym", args, d.confirmTitle("gym"), d.confirmFields(i))
}

func (d *Discord) handleSlashFort(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	args := []string{}
	if value, ok := optionString(options, "type"); ok && strings.TrimSpace(value) != "" {
		args = append(args, strings.ToLower(strings.TrimSpace(value)))
	}
	if value, ok := optionBool(options, "include_empty"); ok && value {
		args = append(args, "include_empty")
	}
	if value, ok := optionBool(options, "location"); ok && value {
		args = append(args, "location")
	}
	if value, ok := optionBool(options, "name"); ok && value {
		args = append(args, "name")
	}
	if value, ok := optionBool(options, "photo"); ok && value {
		args = append(args, "photo")
	}
	if value, ok := optionBool(options, "removal"); ok && value {
		args = append(args, "removal")
	}
	if value, ok := optionBool(options, "new"); ok && value {
		args = append(args, "new")
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "fort", args, d.confirmTitle("fort"), d.confirmFields(i))
}

func (d *Discord) handleSlashNest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	pokemon, ok := optionString(options, "pokemon")
	if !ok || strings.TrimSpace(pokemon) == "" {
		d.respondEphemeral(s, i, "Please pick a Pokemon.")
		return
	}
	args := []string{strings.TrimSpace(pokemon)}
	if value, ok := optionInt(options, "min_spawn"); ok && value > 0 {
		args = append(args, fmt.Sprintf("minspawn%d", value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "nest", args, d.confirmTitle("nest"), d.confirmFields(i))
}

func (d *Discord) handleSlashWeather(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	condition, ok := optionString(options, "condition")
	if !ok || strings.TrimSpace(condition) == "" {
		d.respondEphemeral(s, i, "Please pick a weather condition.")
		return
	}
	location, _ := optionString(options, "location")
	location = strings.TrimSpace(location)
	if location == "" {
		userID, _ := slashUser(i)
		if d.manager != nil && d.manager.query != nil && userID != "" {
			if row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID}); err == nil && row != nil {
				lat := toFloat(row["latitude"])
				lon := toFloat(row["longitude"])
				if lat != 0 || lon != 0 {
					location = fmt.Sprintf("%s,%s", formatFloat(lat), formatFloat(lon))
				} else if d.manager.fences != nil {
					areas := parseAreaListFromHuman(row)
					if len(areas) > 0 {
						target := strings.ToLower(strings.TrimSpace(areas[0]))
						for _, fence := range d.manager.fences.Fences {
							if strings.EqualFold(strings.TrimSpace(fence.Name), target) {
								if centerLat, centerLon, ok := fenceCentroid(fence); ok {
									location = fmt.Sprintf("%s,%s", formatFloat(centerLat), formatFloat(centerLon))
								}
								break
							}
						}
					}
				}
			}
		}
	}
	if location == "" {
		d.respondEphemeral(s, i, "Please set your location in /profile, or provide a location.")
		return
	}
	args := []string{}
	args = append(args, strings.Fields(location)...)
	args = append(args, "|", strings.TrimSpace(condition))
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "weather", args, d.confirmTitle("weather"), d.confirmFields(i))
}

func (d *Discord) handleSlashLure(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	lureType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(lureType) == "" {
		d.respondEphemeral(s, i, "Please pick a lure type.")
		return
	}
	args := []string{strings.TrimSpace(lureType)}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionBool(options, "clean"); ok && value {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "lure", args, d.confirmTitle("lure"), d.confirmFields(i))
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
	update := map[string]any{"current_profile_no": profileNo}
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

func (d *Discord) handleProfileScheduleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken string) {
	if strings.EqualFold(strings.TrimSpace(profileToken), "all") {
		d.handleProfileScheduleAddGlobal(s, i)
		return
	}
	embed, components, errText := d.buildProfileScheduleDayPayload(i, profileToken)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleDay(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken, dayValue string) {
	day := parseDayValue(dayValue)
	if day == 0 {
		d.respondEphemeral(s, i, "Please select a day.")
		return
	}
	customID := fmt.Sprintf("%s%s:%d", slashProfileScheduleTime, strings.TrimSpace(profileToken), day)
	d.respondWithScheduleModal(s, i, customID, "Start time", "End time", "", "")
}

func (d *Discord) handleProfileScheduleTime(s *discordgo.Session, i *discordgo.InteractionCreate, payload string, data discordgo.ModalSubmitInteractionData) {
	parts := strings.Split(payload, ":")
	if len(parts) < 2 {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	profileToken := strings.TrimSpace(parts[0])
	dayPart := strings.TrimSpace(parts[1])
	day := toInt(dayPart, 0)
	if day < 1 || day > 7 {
		day = 0
	}
	startText := strings.TrimSpace(modalTextValue(data, "start"))
	endText := strings.TrimSpace(modalTextValue(data, "end"))
	startMin, ok := parseClockMinutes(startText)
	if !ok {
		d.respondEphemeral(s, i, "Start time must be in HH:MM.")
		return
	}
	endMin, ok := parseClockMinutes(endText)
	if !ok {
		d.respondEphemeral(s, i, "End time must be in HH:MM.")
		return
	}
	if endMin <= startMin {
		d.respondEphemeral(s, i, "End time must be after start time.")
		return
	}
	if strings.EqualFold(profileToken, "all") {
		days := parseDayList(dayPart)
		if len(days) == 0 {
			d.respondEphemeral(s, i, "Invalid day selected.")
			return
		}
		embed, components, errText := d.buildProfileScheduleAssignPayload(i, days, startMin, endMin)
		if errText != "" {
			d.respondEphemeral(s, i, errText)
			return
		}
		d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
		return
	}
	if day == 0 {
		d.respondEphemeral(s, i, "Invalid day selected.")
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
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	entries, errText := addScheduleEntry(profiles, selected, day, startMin, endMin)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	if _, err := d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": encodeScheduleEntries(entries)}, map[string]any{"id": userID, "profile_no": toInt(selected["profile_no"], 0)}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken, value string) {
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
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	updated := removeScheduleEntry(entries, value)
	if len(updated) == len(entries) {
		d.respondEphemeral(s, i, "Schedule entry not found.")
		return
	}
	if _, err := d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": encodeScheduleEntries(updated)}, map[string]any{"id": userID, "profile_no": toInt(selected["profile_no"], 0)}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleRemoveGlobal(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
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
	profileNo, entry, ok := parseGlobalScheduleValue(value)
	if !ok {
		d.respondEphemeral(s, i, "Schedule entry not found.")
		return
	}
	selected := profileRowByNo(profiles, profileNo)
	removed := false
	if selected != nil {
		entries := scheduleEntriesFromRaw(selected["active_hours"])
		updated := removeScheduleEntry(entries, entry)
		if len(updated) != len(entries) {
			if _, err := d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": encodeScheduleEntries(updated)}, map[string]any{"id": userID, "profile_no": toInt(selected["profile_no"], 0)}); err != nil {
				d.respondEphemeral(s, i, "Unable to save schedule.")
				return
			}
			removed = true
		}
	}
	if !removed {
		for _, row := range profiles {
			entries := scheduleEntriesFromRaw(row["active_hours"])
			updated := removeScheduleEntry(entries, entry)
			if len(updated) == len(entries) {
				continue
			}
			if _, err := d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": encodeScheduleEntries(updated)}, map[string]any{"id": userID, "profile_no": toInt(row["profile_no"], 0)}); err != nil {
				d.respondEphemeral(s, i, "Unable to save schedule.")
				return
			}
			removed = true
			break
		}
	}
	if !removed {
		d.respondEphemeral(s, i, "Schedule entry not found.")
		return
	}
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleClear(s *discordgo.Session, i *discordgo.InteractionCreate, profileToken string) {
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
	selected := profileRowByToken(profiles, profileToken)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	if _, err := d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": "[]"}, map[string]any{"id": userID, "profile_no": toInt(selected["profile_no"], 0)}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfilePayload(i, "")
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleAddGlobal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed, components, errText := d.buildProfileScheduleDayPayloadGlobal(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleDayGlobal(s *discordgo.Session, i *discordgo.InteractionCreate, dayValues []string) {
	days := parseDayValues(dayValues)
	if len(days) == 0 {
		d.respondEphemeral(s, i, "Please select at least one day.")
		return
	}
	customID := fmt.Sprintf("%sall:%s", slashProfileScheduleTime, joinDayList(days))
	d.respondWithScheduleModal(s, i, customID, "Start time", "End time", "", "")
}

func (d *Discord) handleProfileScheduleOverview(s *discordgo.Session, i *discordgo.InteractionCreate) {
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleAssign(s *discordgo.Session, i *discordgo.InteractionCreate, payload, profileValue string) {
	days, startMin, endMin, ok := parseAssignPayloadDays(payload)
	if !ok || len(days) == 0 {
		d.respondEphemeral(s, i, "Unable to read schedule.")
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
	selected := profileRowByToken(profiles, profileValue)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	entries, errText := addScheduleEntriesForDays(profiles, selected, days, startMin, endMin)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	if _, err := d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": encodeScheduleEntries(entries)}, map[string]any{"id": userID, "profile_no": toInt(selected["profile_no"], 0)}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleEditSelect(s *discordgo.Session, i *discordgo.InteractionCreate, value string) {
	entry, ok := parseScheduleValue(value)
	if !ok || entry.Legacy {
		d.respondEphemeral(s, i, "That entry cannot be edited.")
		return
	}
	embed, components, errText := d.buildProfileScheduleEditDayPayload(entry)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleEditDay(s *discordgo.Session, i *discordgo.InteractionCreate, payload, dayValue string) {
	entry, ok := parseScheduleValue(payload)
	if !ok {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	day := parseDayValue(dayValue)
	if day == 0 {
		d.respondEphemeral(s, i, "Please select a day.")
		return
	}
	startValue := fmt.Sprintf("%02d:%02d", entry.StartMin/60, entry.StartMin%60)
	endValue := fmt.Sprintf("%02d:%02d", entry.EndMin/60, entry.EndMin%60)
	customID := fmt.Sprintf("%s%d|%s:%d", slashProfileScheduleEditTime, entry.ProfileNo, scheduleEntryValue(entry), day)
	d.respondWithScheduleModal(s, i, customID, "Start time", "End time", startValue, endValue)
}

func (d *Discord) handleProfileScheduleEditTime(s *discordgo.Session, i *discordgo.InteractionCreate, payload string, data discordgo.ModalSubmitInteractionData) {
	parts := strings.Split(payload, ":")
	if len(parts) < 2 {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	original, ok := parseScheduleValue(parts[0])
	if !ok {
		d.respondEphemeral(s, i, "Unable to read schedule.")
		return
	}
	day := toInt(parts[1], 0)
	if day < 1 || day > 7 {
		d.respondEphemeral(s, i, "Invalid day selected.")
		return
	}
	startText := strings.TrimSpace(modalTextValue(data, "start"))
	endText := strings.TrimSpace(modalTextValue(data, "end"))
	startMin, ok := parseClockMinutes(startText)
	if !ok {
		d.respondEphemeral(s, i, "Start time must be in HH:MM.")
		return
	}
	endMin, ok := parseClockMinutes(endText)
	if !ok {
		d.respondEphemeral(s, i, "End time must be in HH:MM.")
		return
	}
	if endMin <= startMin {
		d.respondEphemeral(s, i, "End time must be after start time.")
		return
	}
	embed, components, errText := d.buildProfileScheduleEditAssignPayload(i, original, day, startMin, endMin)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, components)
}

func (d *Discord) handleProfileScheduleEditAssign(s *discordgo.Session, i *discordgo.InteractionCreate, payload, profileValue string) {
	original, day, startMin, endMin, ok := parseEditAssignPayload(payload)
	if !ok {
		d.respondEphemeral(s, i, "Unable to read schedule.")
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
	selected := profileRowByToken(profiles, profileValue)
	if selected == nil {
		d.respondEphemeral(s, i, "Profile not found.")
		return
	}
	entries, errText := addScheduleEntryWithIgnore(profiles, selected, day, startMin, endMin, original.ProfileNo, original)
	if errText != "" {
		d.respondEphemeral(s, i, errText)
		return
	}
	if original.ProfileNo != 0 {
		if old := profileRowByNo(profiles, original.ProfileNo); old != nil {
			oldEntries := scheduleEntriesFromRaw(old["active_hours"])
			oldEntries = removeScheduleEntry(oldEntries, scheduleEntryValue(original))
			_, _ = d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": encodeScheduleEntries(oldEntries)}, map[string]any{"id": userID, "profile_no": original.ProfileNo})
		}
	}
	if _, err := d.manager.query.UpdateQuery("profiles", map[string]any{"active_hours": encodeScheduleEntries(entries)}, map[string]any{"id": userID, "profile_no": toInt(selected["profile_no"], 0)}); err != nil {
		d.respondEphemeral(s, i, "Unable to save schedule.")
		return
	}
	embed, components, errText := d.buildProfileScheduleOverviewPayload(i)
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

func (d *Discord) handleProfileScheduleToggle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	userID, _ := slashUser(i)
	if userID == "" {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || human == nil {
		d.respondEphemeral(s, i, "Target is not registered.")
		return
	}
	disabled := toInt(human["schedule_disabled"], 0) == 1
	update := map[string]any{"schedule_disabled": 0}
	if !disabled {
		update["schedule_disabled"] = 1
		if current := toInt(human["current_profile_no"], 0); current == 0 {
			profiles, err := d.manager.query.SelectAllQuery("profiles", map[string]any{"id": userID})
			if err == nil && len(profiles) > 0 {
				sort.Slice(profiles, func(i, j int) bool {
					return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0)
				})
				update["current_profile_no"] = toInt(profiles[0]["profile_no"], 1)
			} else {
				update["current_profile_no"] = 1
			}
		}
	}
	if _, err := d.manager.query.UpdateQuery("humans", update, map[string]any{"id": userID}); err != nil {
		d.respondEphemeral(s, i, "Unable to update scheduler.")
		return
	}
	d.handleProfileShow(s, i)
}

func (d *Discord) handleSlashRemove(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	trackType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(trackType) == "" {
		d.respondEphemeral(s, i, "Please pick a tracking type.")
		return
	}
	value, ok := optionString(options, "tracking")
	if !ok || strings.TrimSpace(value) == "" {
		d.respondEphemeral(s, i, "Please pick a tracking entry.")
		return
	}

	trackingType, uid := parseRemoveSelection(trackType, value)
	if trackingType == "" || uid == "" {
		d.respondEphemeral(s, i, "That tracking entry could not be parsed.")
		return
	}
	if strings.Contains(value, "|") {
		expected := strings.ToLower(strings.TrimSpace(trackType))
		if expected == "incident" {
			expected = "invasion"
		}
		if expected != "" && trackingType != expected {
			d.respondEphemeral(s, i, "Tracking type changed; please clear the tracking selection and pick again.")
			return
		}
	}
	table := removeTrackingTable(trackingType)
	if table == "" || d.manager == nil || d.manager.query == nil {
		d.respondEphemeral(s, i, "That tracking entry could not be removed.")
		return
	}

	userID, _ := slashUser(i)
	// Do not scope removals to current_profile_no: scheduler-driven quiet hours can set
	// current_profile_no=0, which would otherwise prevent users from removing their trackings.
	where := map[string]any{"id": userID}
	if !strings.EqualFold(uid, "all") && !strings.EqualFold(uid, "everything") {
		where["uid"] = parseUID(uid)
	}
	removed, err := d.manager.query.DeleteQuery(table, where)
	if err != nil {
		d.respondEphemeral(s, i, err.Error())
		return
	}
	if removed == 0 {
		if strings.EqualFold(uid, "all") || strings.EqualFold(uid, "everything") {
			d.respondEphemeral(s, i, "No tracking entries found.")
			return
		}
		d.respondEphemeral(s, i, "Tracking not found.")
		return
	}
	if strings.EqualFold(uid, "all") || strings.EqualFold(uid, "everything") {
		d.respondEphemeral(s, i, fmt.Sprintf("Removed %d tracking entries.", removed))
		return
	}
	d.respondEphemeral(s, i, "Tracking removed.")
}

func (d *Discord) handleSlashTracked(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.executeSlashLineDeferred(s, i, "tracked")
}

func (d *Discord) handleSlashStart(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.executeSlashLineDeferred(s, i, "start")
}

func (d *Discord) handleSlashStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.executeSlashLineDeferred(s, i, "stop")
}

func (d *Discord) handleSlashHelp(s *discordgo.Session, i *discordgo.InteractionCreate) {
	// Use a dedicated DTS help template for slash users, so legacy `!help` can stay focused on legacy commands.
	// Falls back to default `help` behaviour if the template does not exist.
	d.respondDeferredEphemeral(s, i)
	reply := d.buildSlashReply(s, i, "help slash")
	if strings.TrimSpace(reply) == "🙅" {
		reply = d.buildSlashReply(s, i, "help")
	}
	if reply == "" {
		reply = "Done."
	}
	if d.sendSpecialSlashReply(s, i, reply) {
		return
	}
	d.respondEditMessage(s, i, reply, nil)
}

func (d *Discord) handleSlashInfo(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()
	options := slashOptions(data)

	infoType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(infoType) == "" {
		d.respondEphemeral(s, i, "Please pick an info type.")
		return
	}
	infoType = strings.ToLower(strings.TrimSpace(infoType))

	switch infoType {
	case "pokemon":
		pokemon, _ := optionString(options, "pokemon")
		pokemon = strings.TrimSpace(pokemon)
		if pokemon == "" {
			d.respondEphemeral(s, i, "Please pick a Pokemon.")
			return
		}
		if strings.EqualFold(pokemon, "everything") {
			d.respondEphemeral(s, i, "Please pick a specific Pokemon (\"Everything\" is not supported here).")
			return
		}
		d.executeSlashLineDeferred(s, i, "info "+pokemon)
	case "moves", "items", "rarity", "shiny":
		d.executeSlashLineDeferred(s, i, "info "+infoType)
	case "weather":
		d.respondWithButtons(s, i, "Weather info: use your saved location or enter coordinates?", []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashInfoWeatherUseSaved, Label: "Use saved location", Style: discordgo.PrimaryButton},
			discordgo.Button{CustomID: slashInfoWeatherEnterCoordinates, Label: "Enter coordinates", Style: discordgo.SecondaryButton},
			discordgo.Button{CustomID: slashInfoCancelButton, Label: "Cancel", Style: discordgo.DangerButton},
		})
	case "translate":
		d.respondWithModal(s, i, slashInfoTranslateModal, "Translate", "Text", "Bonjour")
	default:
		d.executeSlashLineDeferred(s, i, "info")
	}
}

func (d *Discord) handleSlashLanguage(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	value, _ := optionString(options, "to")
	value = strings.TrimSpace(value)
	if value == "" {
		d.executeSlashLineDeferred(s, i, "language")
		return
	}
	d.executeSlashLineDeferred(s, i, "language "+value)
}

func normalizeRaidType(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "level") {
		return "level" + strings.TrimPrefix(lower, "level")
	}
	if _, err := strconv.Atoi(lower); err == nil {
		return "level" + lower
	}
	return trimmed
}

func optionalInt(options []*discordgo.ApplicationCommandInteractionDataOption, name string) *int {
	if value, ok := optionInt(options, name); ok {
		return &value
	}
	return nil
}

func parseRemoveSelection(optionType, value string) (string, string) {
	if strings.Contains(value, "|") {
		parts := strings.SplitN(value, "|", 2)
		return strings.ToLower(parts[0]), parts[1]
	}
	return strings.ToLower(optionType), value
}

func removeTrackingTable(trackingType string) string {
	switch strings.ToLower(trackingType) {
	case "pokemon":
		return "monsters"
	case "raid":
		return "raid"
	case "gym":
		return "gym"
	case "maxbattle":
		return "maxbattle"
	case "incident", "invasion":
		return "invasion"
	case "quest":
		return "quest"
	case "weather":
		return "weather"
	case "lure":
		return "lures"
	case "nest":
		return "nests"
	case "fort":
		return "forts"
	default:
		return ""
	}
}

func parseUID(value string) any {
	if value == "" {
		return value
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return value
}

func (d *Discord) userProfileNo(userID string) int {
	if d.manager == nil || d.manager.query == nil || userID == "" {
		return 1
	}
	row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err != nil || row == nil {
		return 1
	}
	return toInt(row["current_profile_no"], 1)
}

func (d *Discord) userLanguage(userID string) string {
	if d.manager == nil || d.manager.query == nil || userID == "" {
		return "en"
	}
	row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
	if err == nil && row != nil {
		if lang, ok := row["language"].(string); ok && lang != "" {
			return lang
		}
	}
	return "en"
}

func floatPtr(value float64) *float64 {
	return &value
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return parsed
		}
	}
	return 0
}

func parseLatLonString(value string) (float64, float64, bool) {
	re := regexp.MustCompile(`^([-+]?(?:[1-8]?\d(?:\.\d+)?|90(?:\.0+)?)),\s*([-+]?(?:180(\.0+)?|(?:(?:1[0-7]\d)|(?:[1-9]?\d))(?:\.\d+)?))$`)
	match := re.FindStringSubmatch(strings.TrimSpace(value))
	if len(match) < 3 {
		return 0, 0, false
	}
	lat, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, 0, false
	}
	lon, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return 0, 0, false
	}
	return lat, lon, true
}

func formatFloat(value float64) string {
	if value == 0 {
		return "0"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.7f", value), "0"), ".")
}

func parseAreaListFromHuman(human map[string]any) []string {
	areas := []string{}
	if raw, ok := human["area"].(string); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &areas)
	}
	return areas
}

func fenceCentroid(fence geofence.Fence) (float64, float64, bool) {
	path := fence.Path
	if len(path) == 0 && len(fence.MultiPath) > 0 {
		longest := fence.MultiPath[0]
		for _, candidate := range fence.MultiPath[1:] {
			if len(candidate) > len(longest) {
				longest = candidate
			}
		}
		path = longest
	}
	if len(path) == 0 {
		return 0, 0, false
	}
	var sumLat, sumLon float64
	for _, point := range path {
		if len(point) < 2 {
			continue
		}
		sumLat += point[0]
		sumLon += point[1]
	}
	count := float64(len(path))
	if count == 0 {
		return 0, 0, false
	}
	return sumLat / count, sumLon / count, true
}

func selectableAreaNames(fences []geofence.Fence) []string {
	names := []string{}
	for _, fence := range fences {
		selectable := true
		if fence.UserSelectable != nil {
			selectable = *fence.UserSelectable
		}
		if !selectable {
			continue
		}
		name := strings.TrimSpace(fence.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

func (d *Discord) autocompletePokemonChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	type candidate struct {
		ID   int
		Name string
	}
	candidates := []candidate{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := toInt(mon["id"], 0)
		if name == "" || id == 0 {
			continue
		}
		if query == "" || name == query || fmt.Sprintf("%d", id) == query || strings.HasPrefix(name, query) || strings.Contains(name, query) {
			candidates = append(candidates, candidate{ID: id, Name: name})
		}
	}
	if len(candidates) == 0 && query != "" {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(candidates)+1)
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
		if len(candidates) > 24 {
			candidates = candidates[:24]
		}
	} else if len(candidates) > 25 {
		candidates = candidates[:25]
	}
	for _, mon := range candidates {
		label := fmt.Sprintf("%s (#%d)", d.titleCase(mon.Name), mon.ID)
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: fmt.Sprintf("%d", mon.ID),
		})
	}
	return choices
}

func (d *Discord) autocompleteInfoPokemonChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	type candidate struct {
		ID   int
		Name string
	}
	candidates := []candidate{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := toInt(mon["id"], 0)
		if name == "" || id == 0 {
			continue
		}
		if query == "" || name == query || fmt.Sprintf("%d", id) == query || strings.HasPrefix(name, query) || strings.Contains(name, query) {
			candidates = append(candidates, candidate{ID: id, Name: name})
		}
	}
	if len(candidates) == 0 && query != "" {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	if len(candidates) > 25 {
		candidates = candidates[:25]
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(candidates))
	for _, mon := range candidates {
		label := fmt.Sprintf("%s (#%d)", d.titleCase(mon.Name), mon.ID)
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: fmt.Sprintf("%d", mon.ID),
		})
	}
	return choices
}

func (d *Discord) autocompletePokemonFormChoices(query, pokemon string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	id := d.pokemonIDFromValue(pokemon)
	if id == 0 {
		return nil
	}
	forms := d.pokemonFormNames(id)
	if len(forms) == 0 {
		return nil
	}
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "All forms",
			Value: "all",
		})
	}
	for _, form := range forms {
		if len(choices) >= 25 {
			break
		}
		lower := strings.ToLower(form)
		if query != "" && !strings.Contains(lower, query) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  d.titleCase(form),
			Value: form,
		})
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteLanguageChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.cfg == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))

	raw, ok := d.manager.cfg.Get("general.availableLanguages")
	if !ok {
		return nil
	}
	available, ok := raw.(map[string]any)
	if !ok || len(available) == 0 {
		return nil
	}

	languageNames := map[string]string{}
	if d.manager.data != nil && d.manager.data.UtilData != nil {
		if rawNames, ok := d.manager.data.UtilData["languageNames"].(map[string]any); ok {
			for key, value := range rawNames {
				languageNames[strings.ToLower(key)] = strings.TrimSpace(fmt.Sprintf("%v", value))
			}
		}
	}

	type entry struct {
		key   string
		label string
	}
	entries := make([]entry, 0, len(available))
	for key := range available {
		k := strings.ToLower(strings.TrimSpace(key))
		if k == "" {
			continue
		}
		name := languageNames[k]
		label := k
		if name != "" {
			label = fmt.Sprintf("%s (%s)", name, k)
		}
		entries = append(entries, entry{key: k, label: label})
	}
	sort.Slice(entries, func(i, j int) bool { return strings.ToLower(entries[i].label) < strings.ToLower(entries[j].label) })

	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(entries))
	for _, e := range entries {
		if len(choices) >= 25 {
			break
		}
		if query != "" {
			if !strings.Contains(strings.ToLower(e.key), query) && !strings.Contains(strings.ToLower(e.label), query) {
				continue
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  e.label,
			Value: e.key,
		})
	}
	return choices
}

func (d *Discord) autocompleteWeatherChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	raw, ok := d.manager.data.UtilData["weather"].(map[string]any)
	if !ok || len(raw) == 0 {
		return nil
	}

	type entry struct {
		id   int
		name string
	}
	entries := make([]entry, 0, len(raw))
	for key, value := range raw {
		weatherID := toInt(key, 0)
		if weatherID <= 0 {
			continue
		}
		m, ok := value.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", m["name"]))
		if name == "" {
			continue
		}
		entries = append(entries, entry{id: weatherID, name: name})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })

	choices := []*discordgo.ApplicationCommandOptionChoice{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
	}
	for _, e := range entries {
		if len(choices) >= 25 {
			break
		}
		label := fmt.Sprintf("%s (#%d)", e.name, e.id)
		value := fmt.Sprintf("%d", e.id)
		if query != "" && !strings.Contains(strings.ToLower(label), query) && !strings.Contains(value, query) {
			continue
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: value,
		})
	}
	return choices
}

func (d *Discord) autocompleteRaidTypeChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	seen := map[string]bool{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
		seen["everything"] = true
	}

	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["raidLevels"].(map[string]any); ok {
			levels := []int{}
			for key := range raw {
				if value := toInt(key, 0); value > 0 {
					levels = append(levels, value)
				}
			}
			sort.Ints(levels)
			for _, level := range levels {
				value := fmt.Sprintf("level%d", level)
				if query == "" || strings.Contains(value, query) {
					seen[value] = true
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  d.raidLevelLabel(level),
						Value: value,
					})
				}
			}
		}
	}

	for _, choice := range d.autocompletePokemonChoices(query) {
		if len(choices) >= 25 {
			break
		}
		value := fmt.Sprintf("%v", choice.Value)
		if seen[value] {
			continue
		}
		seen[value] = true
		choices = append(choices, choice)
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteMaxbattleTypeChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	seen := map[string]bool{}
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
		seen["everything"] = true
	}

	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["maxbattleLevels"].(map[string]any); ok {
			levels := []int{}
			for key := range raw {
				if value := toInt(key, 0); value > 0 {
					levels = append(levels, value)
				}
			}
			sort.Ints(levels)
			for _, level := range levels {
				value := fmt.Sprintf("level%d", level)
				if query == "" || strings.Contains(value, query) {
					seen[value] = true
					choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
						Name:  d.maxbattleLevelLabel(level),
						Value: value,
					})
				}
				if len(choices) >= 25 {
					break
				}
			}
		}
	}

	for _, choice := range d.autocompletePokemonChoices(query) {
		if len(choices) >= 25 {
			break
		}
		value := fmt.Sprintf("%v", choice.Value)
		if seen[value] {
			continue
		}
		seen[value] = true
		choices = append(choices, choice)
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteRaidLevelChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	raw, ok := d.manager.data.UtilData["raidLevels"].(map[string]any)
	if !ok {
		return nil
	}
	levels := []int{}
	for key := range raw {
		if value := toInt(key, 0); value > 0 {
			levels = append(levels, value)
		}
	}
	if len(levels) == 0 {
		return nil
	}
	sort.Ints(levels)
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(levels)+1)
	if query == "" {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
	}
	for _, level := range levels {
		value := fmt.Sprintf("level%d", level)
		label := d.raidLevelLabel(level)
		if query == "" || strings.Contains(strings.ToLower(value), query) || strings.Contains(strings.ToLower(label), query) {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  label,
				Value: value,
			})
		}
		if len(choices) >= 25 {
			break
		}
	}
	return choices
}

func (d *Discord) autocompleteGymChoices(i *discordgo.InteractionCreate, query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.scanner == nil {
		return nil
	}
	query = strings.TrimSpace(query)
	var entries []scanner.GymEntry
	var err error
	if query == "" {
		userID, _ := slashUser(i)
		if d.manager.query != nil && userID != "" {
			if row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID}); err == nil && row != nil {
				lat := toFloat(row["latitude"])
				lon := toFloat(row["longitude"])
				if lat != 0 || lon != 0 {
					entries, err = d.manager.scanner.SearchGymsNearby(lat, lon, 25)
				} else if d.manager.fences != nil {
					areas := parseAreaListFromHuman(row)
					if len(areas) > 0 {
						target := strings.ToLower(strings.TrimSpace(areas[0]))
						for _, fence := range d.manager.fences.Fences {
							if strings.EqualFold(strings.TrimSpace(fence.Name), target) {
								if centerLat, centerLon, ok := fenceCentroid(fence); ok {
									entries, err = d.manager.scanner.SearchGymsNearby(centerLat, centerLon, 25)
								}
								break
							}
						}
					}
				}
			}
		}
	}
	if entries == nil || len(entries) == 0 {
		entries, err = d.manager.scanner.SearchGyms(query, 25)
	}
	if err != nil {
		return nil
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" || entry.ID == "" {
			continue
		}
		if entry.HasCoords && d.manager != nil && d.manager.fences != nil {
			areas := d.manager.fences.MatchedAreas([]float64{entry.Latitude, entry.Longitude})
			if len(areas) > 0 && areas[0].Name != "" {
				name = fmt.Sprintf("%s (%s)", name, areas[0].Name)
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: entry.ID,
		})
	}
	return choices
}

func (d *Discord) autocompleteStationChoices(i *discordgo.InteractionCreate, query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.scanner == nil {
		return nil
	}
	query = strings.TrimSpace(query)
	var entries []scanner.StationEntry
	var err error
	if query == "" {
		userID, _ := slashUser(i)
		if d.manager.query != nil && userID != "" {
			if row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID}); err == nil && row != nil {
				lat := toFloat(row["latitude"])
				lon := toFloat(row["longitude"])
				if lat != 0 || lon != 0 {
					entries, err = d.manager.scanner.SearchStationsNearby(lat, lon, 25)
				} else if d.manager.fences != nil {
					areas := parseAreaListFromHuman(row)
					if len(areas) > 0 {
						target := strings.ToLower(strings.TrimSpace(areas[0]))
						for _, fence := range d.manager.fences.Fences {
							if strings.EqualFold(strings.TrimSpace(fence.Name), target) {
								if centerLat, centerLon, ok := fenceCentroid(fence); ok {
									entries, err = d.manager.scanner.SearchStationsNearby(centerLat, centerLon, 25)
								}
								break
							}
						}
					}
				}
			}
		}
	}
	if entries == nil || len(entries) == 0 {
		entries, err = d.manager.scanner.SearchStations(query, 25)
	}
	if err != nil {
		return nil
	}
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(entries))
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		if name == "" || entry.ID == "" {
			continue
		}
		if entry.HasCoords && d.manager != nil && d.manager.fences != nil {
			areas := d.manager.fences.MatchedAreas([]float64{entry.Latitude, entry.Longitude})
			if len(areas) > 0 && areas[0].Name != "" {
				name = fmt.Sprintf("%s (%s)", name, areas[0].Name)
			}
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  name,
			Value: entry.ID,
		})
	}
	return choices
}

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
		return nil, nil, "You do not have any profiles."
	}
	sort.Slice(profiles, func(i, j int) bool {
		return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0)
	})
	currentProfile := toInt(human["current_profile_no"], 1)
	selectedRow := profileRowByToken(profiles, selected)
	if selectedRow == nil {
		selectedRow = profileRowByToken(profiles, fmt.Sprintf("%d", currentProfile))
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
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: "Schedules enable alerts only during the listed windows. Outside those windows, alerts are paused. If you have no schedules, alerts run all the time. End times are exclusive, so back-to-back periods can share the same minute. Times use your saved location timezone.",
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
	setButton := discordgo.Button{
		CustomID: slashProfileSet + fmt.Sprintf("%d", selectedNo),
		Label:    "Set Active",
		Style:    discordgo.SuccessButton,
		Disabled: selectedNo == currentProfile,
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

func profileRowByToken(rows []map[string]any, token string) map[string]any {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if num, err := strconv.Atoi(token); err == nil && num > 0 {
		for _, row := range rows {
			if toInt(row["profile_no"], 0) == num {
				return row
			}
		}
	}
	for _, row := range rows {
		if strings.EqualFold(fmt.Sprintf("%v", row["name"]), token) {
			return row
		}
	}
	return nil
}

func profileRowByNo(rows []map[string]any, number int) map[string]any {
	if number == 0 {
		return nil
	}
	for _, row := range rows {
		if toInt(row["profile_no"], 0) == number {
			return row
		}
	}
	return nil
}

func profileNameExistsRows(rows []map[string]any, name string) bool {
	for _, row := range rows {
		if strings.EqualFold(fmt.Sprintf("%v", row["name"]), name) {
			return true
		}
	}
	return false
}

func fallbackStaticMap(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	value, _ := cfg.GetString("fallbacks.staticMap")
	return strings.TrimSpace(value)
}

func parseProfileAreas(raw any) []string {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if text == "" || text == "[]" {
		return nil
	}
	areas := []string{}
	if err := json.Unmarshal([]byte(text), &areas); err != nil {
		return nil
	}
	out := []string{}
	for _, area := range areas {
		trimmed := strings.TrimSpace(area)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func profileHoursText(raw any) string {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if len(text) <= 2 {
		return ""
	}
	var times []map[string]any
	if err := json.Unmarshal([]byte(text), &times); err != nil || len(times) == 0 {
		return ""
	}
	dayNames := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	parts := []string{}
	for _, entry := range times {
		day := toInt(entry["day"], 0)
		if day < 1 || day > 7 {
			continue
		}
		hours := toInt(entry["hours"], 0)
		mins := toInt(entry["mins"], 0)
		parts = append(parts, fmt.Sprintf("%s %d:%02d", dayNames[day-1], hours, mins))
	}
	return strings.Join(parts, ", ")
}

func profileScheduleText(raw any) string {
	entries := scheduleEntriesFromRaw(raw)
	if len(entries) == 0 {
		return ""
	}
	lines := []string{}
	for _, entry := range entries {
		lines = append(lines, scheduleEntryLabel(entry))
	}
	return strings.Join(lines, "\n")
}

type scheduleEntry struct {
	ProfileNo int
	Day       int
	StartMin  int
	EndMin    int
	Legacy    bool
}

func scheduleEntriesFromRaw(raw any) []scheduleEntry {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if len(text) <= 2 {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	out := []scheduleEntry{}
	for _, row := range rows {
		day := toInt(row["day"], 0)
		if day < 1 || day > 7 {
			continue
		}
		if startHours, ok := row["start_hours"]; ok {
			startMins := toInt(row["start_mins"], 0)
			endHours := toInt(row["end_hours"], 0)
			endMins := toInt(row["end_mins"], 0)
			out = append(out, scheduleEntry{
				Day:      day,
				StartMin: toInt(startHours, 0)*60 + startMins,
				EndMin:   endHours*60 + endMins,
			})
			continue
		}
		if hours, ok := row["hours"]; ok {
			mins := toInt(row["mins"], 0)
			out = append(out, scheduleEntry{
				Day:      day,
				StartMin: toInt(hours, 0)*60 + mins,
				Legacy:   true,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].StartMin < out[j].StartMin
	})
	return out
}

func scheduleEntryLabel(entry scheduleEntry) string {
	day := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}[entry.Day-1]
	start := fmt.Sprintf("%02d:%02d", entry.StartMin/60, entry.StartMin%60)
	if entry.Legacy || entry.EndMin <= 0 {
		return fmt.Sprintf("%s %s (switch)", day, start)
	}
	end := fmt.Sprintf("%02d:%02d", entry.EndMin/60, entry.EndMin%60)
	return fmt.Sprintf("%s %s-%s", day, start, end)
}

func scheduleEntryValue(entry scheduleEntry) string {
	return fmt.Sprintf("%d|%d|%d|%t", entry.Day, entry.StartMin, entry.EndMin, entry.Legacy)
}

func scheduleRemoveOptions(raw any) []discordgo.SelectMenuOption {
	entries := scheduleEntriesFromRaw(raw)
	if len(entries) == 0 {
		return nil
	}
	options := make([]discordgo.SelectMenuOption, 0, len(entries))
	for _, entry := range entries {
		value := scheduleEntryValue(entry)
		options = append(options, discordgo.SelectMenuOption{
			Label: scheduleEntryLabel(entry),
			Value: value,
		})
		if len(options) >= 25 {
			break
		}
	}
	return options
}

func removeScheduleEntry(entries []scheduleEntry, value string) []scheduleEntry {
	parts := strings.Split(value, "|")
	if len(parts) < 4 {
		return entries
	}
	day := toInt(parts[0], 0)
	start := toInt(parts[1], 0)
	end := toInt(parts[2], 0)
	legacy := strings.EqualFold(parts[3], "true")
	out := []scheduleEntry{}
	removed := false
	for _, entry := range entries {
		if !removed && entry.Day == day && entry.StartMin == start && entry.EndMin == end && entry.Legacy == legacy {
			removed = true
			continue
		}
		out = append(out, entry)
	}
	return out
}

func encodeScheduleEntries(entries []scheduleEntry) string {
	rows := []map[string]any{}
	for _, entry := range entries {
		if entry.Legacy {
			rows = append(rows, map[string]any{
				"day":   entry.Day,
				"hours": entry.StartMin / 60,
				"mins":  entry.StartMin % 60,
			})
			continue
		}
		rows = append(rows, map[string]any{
			"day":         entry.Day,
			"start_hours": entry.StartMin / 60,
			"start_mins":  entry.StartMin % 60,
			"end_hours":   entry.EndMin / 60,
			"end_mins":    entry.EndMin % 60,
		})
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func scheduleRemoveOptionsGlobal(profiles []map[string]any) []discordgo.SelectMenuOption {
	options := []discordgo.SelectMenuOption{}
	for _, profile := range profiles {
		profileNo := toInt(profile["profile_no"], 0)
		if profileNo == 0 {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", profile["name"]))
		if name == "" {
			name = fmt.Sprintf("Profile %d", profileNo)
		}
		for _, entry := range scheduleEntriesFromRaw(profile["active_hours"]) {
			label := fmt.Sprintf("%s — %s", scheduleEntryLabel(entry), name)
			value := fmt.Sprintf("%d|%s", profileNo, scheduleEntryValue(entry))
			options = append(options, discordgo.SelectMenuOption{Label: label, Value: value})
			if len(options) >= 25 {
				return options
			}
		}
	}
	return options
}

func scheduleEditOptionsGlobal(profiles []map[string]any) []discordgo.SelectMenuOption {
	options := []discordgo.SelectMenuOption{}
	for _, profile := range profiles {
		profileNo := toInt(profile["profile_no"], 0)
		if profileNo == 0 {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", profile["name"]))
		if name == "" {
			name = fmt.Sprintf("Profile %d", profileNo)
		}
		for _, entry := range scheduleEntriesFromRaw(profile["active_hours"]) {
			if entry.Legacy {
				continue
			}
			value := fmt.Sprintf("%d|%s", profileNo, scheduleEntryValue(entry))
			label := fmt.Sprintf("%s — %s", scheduleEntryLabel(entry), name)
			options = append(options, discordgo.SelectMenuOption{Label: label, Value: value})
			if len(options) >= 25 {
				return options
			}
		}
	}
	return options
}

func addScheduleEntry(allProfiles []map[string]any, selected map[string]any, day, startMin, endMin int) ([]scheduleEntry, string) {
	if selected == nil {
		return nil, "Profile not found."
	}
	selectedNo := toInt(selected["profile_no"], 0)
	if selectedNo == 0 {
		return nil, "Profile not found."
	}
	if conflicts := scheduleConflicts(allProfiles, day, startMin, endMin, 0, scheduleEntry{}); len(conflicts) > 0 {
		return nil, fmt.Sprintf("That overlaps with existing schedules: %s", strings.Join(conflicts, ", "))
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	entries = append(entries, scheduleEntry{Day: day, StartMin: startMin, EndMin: endMin})
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		return entries[i].StartMin < entries[j].StartMin
	})
	return entries, ""
}

func addScheduleEntriesForDays(allProfiles []map[string]any, selected map[string]any, days []int, startMin, endMin int) ([]scheduleEntry, string) {
	if selected == nil {
		return nil, "Profile not found."
	}
	if len(days) == 0 {
		return nil, "Please select at least one day."
	}
	conflicts := []string{}
	for _, day := range days {
		conflicts = append(conflicts, scheduleConflicts(allProfiles, day, startMin, endMin, 0, scheduleEntry{})...)
	}
	if len(conflicts) > 0 {
		return nil, fmt.Sprintf("That overlaps with existing schedules: %s", strings.Join(uniqueStrings(conflicts), ", "))
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	for _, day := range days {
		entries = append(entries, scheduleEntry{Day: day, StartMin: startMin, EndMin: endMin})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		return entries[i].StartMin < entries[j].StartMin
	})
	return entries, ""
}

func addScheduleEntryWithIgnore(allProfiles []map[string]any, selected map[string]any, day, startMin, endMin int, ignoreProfileNo int, ignoreEntry scheduleEntry) ([]scheduleEntry, string) {
	if selected == nil {
		return nil, "Profile not found."
	}
	selectedNo := toInt(selected["profile_no"], 0)
	if selectedNo == 0 {
		return nil, "Profile not found."
	}
	if conflicts := scheduleConflicts(allProfiles, day, startMin, endMin, ignoreProfileNo, ignoreEntry); len(conflicts) > 0 {
		return nil, fmt.Sprintf("That overlaps with existing schedules: %s", strings.Join(conflicts, ", "))
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	entries = append(entries, scheduleEntry{Day: day, StartMin: startMin, EndMin: endMin})
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		return entries[i].StartMin < entries[j].StartMin
	})
	return entries, ""
}

func scheduleConflicts(allProfiles []map[string]any, day, startMin, endMin int, ignoreProfileNo int, ignoreEntry scheduleEntry) []string {
	conflicts := []string{}
	for _, row := range allProfiles {
		profileNo := toInt(row["profile_no"], 0)
		name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
		if name == "" {
			name = fmt.Sprintf("Profile %d", profileNo)
		}
		for _, entry := range scheduleEntriesFromRaw(row["active_hours"]) {
			if entry.Legacy || entry.Day != day {
				continue
			}
			if profileNo == ignoreProfileNo && entry.Day == ignoreEntry.Day && entry.StartMin == ignoreEntry.StartMin && entry.EndMin == ignoreEntry.EndMin && entry.Legacy == ignoreEntry.Legacy {
				continue
			}
			if startMin < entry.EndMin && entry.StartMin < endMin {
				conflicts = append(conflicts, fmt.Sprintf("%s %s", name, scheduleEntryLabel(entry)))
			}
		}
	}
	return conflicts
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func parseClockMinutes(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour := toInt(parts[0], -1)
	min := toInt(parts[1], -1)
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, false
	}
	return hour*60 + min, true
}

func parseDayValue(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mon":
		return 1
	case "monday":
		return 1
	case "tue":
		return 2
	case "tuesday":
		return 2
	case "wed":
		return 3
	case "wednesday":
		return 3
	case "thu":
		return 4
	case "thursday":
		return 4
	case "fri":
		return 5
	case "friday":
		return 5
	case "sat":
		return 6
	case "saturday":
		return 6
	case "sun":
		return 7
	case "sunday":
		return 7
	}
	return toInt(value, 0)
}

func parseDayValues(values []string) []int {
	if len(values) == 0 {
		return nil
	}
	out := []int{}
	seen := map[int]bool{}
	for _, value := range values {
		parts := strings.Split(strings.TrimSpace(value), ",")
		for _, part := range parts {
			day := parseDayValue(part)
			if day >= 1 && day <= 7 && !seen[day] {
				seen[day] = true
				out = append(out, day)
			}
		}
	}
	sort.Ints(out)
	return out
}

func parseDayList(value string) []int {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		return r == ',' || r == '.'
	})
	return parseDayValues(parts)
}

func joinDayList(days []int) string {
	if len(days) == 0 {
		return ""
	}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		parts = append(parts, fmt.Sprintf("%d", day))
	}
	return strings.Join(parts, ".")
}

func labelDayList(days []int) string {
	if len(days) == 0 {
		return ""
	}
	labels := []string{}
	for _, day := range days {
		if day < 1 || day > 7 {
			continue
		}
		labels = append(labels, []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}[day-1])
	}
	return strings.Join(labels, ", ")
}

func parseAssignPayloadDays(value string) ([]int, int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 3 {
		return nil, 0, 0, false
	}
	start := toInt(parts[1], 0)
	end := toInt(parts[2], 0)
	days := parseDayList(parts[0])
	if len(days) == 0 || start < 0 || end <= start {
		return nil, 0, 0, false
	}
	return days, start, end, true
}

func parseGlobalScheduleValue(value string) (int, string, bool) {
	parts := strings.Split(strings.TrimSpace(value), "|")
	if len(parts) != 5 {
		return 0, "", false
	}
	profileNo := toInt(parts[0], 0)
	if profileNo == 0 {
		return 0, "", false
	}
	entry := strings.Join(parts[1:], "|")
	return profileNo, entry, true
}

func parseScheduleValue(value string) (scheduleEntry, bool) {
	parts := strings.Split(strings.TrimSpace(value), "|")
	if len(parts) != 5 {
		return scheduleEntry{}, false
	}
	profileNo := toInt(parts[0], 0)
	day := toInt(parts[1], 0)
	start := toInt(parts[2], 0)
	end := toInt(parts[3], 0)
	legacy := strings.EqualFold(parts[4], "true")
	if profileNo == 0 || day < 1 || day > 7 || start < 0 || end < 0 {
		return scheduleEntry{}, false
	}
	return scheduleEntry{ProfileNo: profileNo, Day: day, StartMin: start, EndMin: end, Legacy: legacy}, true
}

func parseEditAssignPayload(value string) (scheduleEntry, int, int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return scheduleEntry{}, 0, 0, 0, false
	}
	entry, ok := parseScheduleValue(parts[0])
	if !ok {
		return scheduleEntry{}, 0, 0, 0, false
	}
	newParts := strings.Split(parts[1], "-")
	if len(newParts) != 3 {
		return scheduleEntry{}, 0, 0, 0, false
	}
	day := toInt(newParts[0], 0)
	start := toInt(newParts[1], 0)
	end := toInt(newParts[2], 0)
	if day < 1 || day > 7 || end <= start {
		return scheduleEntry{}, 0, 0, 0, false
	}
	return entry, day, start, end, true
}

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

func (d *Discord) autocompleteQuestTypeChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	if query == "" || strings.Contains("everything", query) {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
	}

	addChoice := func(entries *[]questChoice, label, value string) {
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		if label == "" || value == "" {
			return
		}
		lowerLabel := strings.ToLower(label)
		lowerValue := strings.ToLower(value)
		if query == "" || strings.Contains(lowerLabel, query) || strings.Contains(lowerValue, query) {
			*entries = append(*entries, questChoice{label: label, value: value})
		}
	}

	entries := []questChoice{}
	addChoice(&entries, "Stardust", "stardust")
	addChoice(&entries, "Rare Candy", "candy")
	addChoice(&entries, "Rare Candy XL", "xl candy")
	addChoice(&entries, "Mega Energy", "energy")
	addChoice(&entries, "Experience", "experience")

	itemEntries := d.questItemChoices()
	sort.Slice(itemEntries, func(i, j int) bool { return itemEntries[i].label < itemEntries[j].label })
	for _, entry := range itemEntries {
		addChoice(&entries, entry.label, entry.value)
	}

	energyEntries := d.questMegaEnergyChoices()
	sort.Slice(energyEntries, func(i, j int) bool { return energyEntries[i].label < energyEntries[j].label })
	for _, entry := range energyEntries {
		addChoice(&entries, entry.label, entry.value)
	}

	monsterEntries := d.questMonsterChoices()
	sort.Slice(monsterEntries, func(i, j int) bool { return monsterEntries[i].label < monsterEntries[j].label })
	for _, entry := range monsterEntries {
		addChoice(&entries, entry.label, entry.value)
	}

	candyEntries := d.questCandyMonsterChoices()
	sort.Slice(candyEntries, func(i, j int) bool { return candyEntries[i].label < candyEntries[j].label })
	for _, entry := range candyEntries {
		addChoice(&entries, entry.label, entry.value)
	}

	xlEntries := d.questXLCandyMonsterChoices()
	sort.Slice(xlEntries, func(i, j int) bool { return xlEntries[i].label < xlEntries[j].label })
	for _, entry := range xlEntries {
		addChoice(&entries, entry.label, entry.value)
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		if len(choices) >= 25 {
			break
		}
		if seen[entry.value] {
			continue
		}
		seen[entry.value] = true
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  truncateChoiceLabel(entry.label),
			Value: entry.value,
		})
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteIncidentTypeChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	if query == "" || strings.Contains("everything", query) {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  "Everything",
			Value: "everything",
		})
	}

	type invasionChoice struct {
		label string
		value string
	}

	addChoice := func(entries *[]invasionChoice, label, value string) {
		label = strings.TrimSpace(label)
		value = strings.TrimSpace(value)
		if label == "" || value == "" {
			return
		}
		lowerLabel := strings.ToLower(label)
		lowerValue := strings.ToLower(value)
		if query == "" || strings.Contains(lowerLabel, query) || strings.Contains(lowerValue, query) {
			*entries = append(*entries, invasionChoice{
				label: label,
				value: value,
			})
		}
	}

	entries := []invasionChoice{}
	if d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["pokestopEvent"].(map[string]any); ok {
			for _, value := range raw {
				if entry, ok := value.(map[string]any); ok {
					if name, ok := entry["name"].(string); ok {
						label := invasionEventLabel(name)
						addChoice(&entries, label, strings.ToLower(strings.TrimSpace(name)))
					}
				}
			}
		}
	}

	type gruntChoice struct {
		labelType string
		valueType string
		gender    int
		names     []string
		seen      map[string]bool
	}
	gruntChoices := map[string]*gruntChoice{}
	for _, raw := range d.manager.data.Grunts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName := strings.TrimSpace(getStringValue(entry["type"]))
		if typeName == "" {
			continue
		}
		names := d.invasionEncounterNames(entry)
		if len(names) == 0 {
			continue
		}
		gender := toIntValue(entry["gender"])
		valueType := strings.ToLower(typeName)
		labelType := typeName
		if strings.EqualFold(labelType, "Metal") {
			labelType = "Steel"
		}
		gruntLabel := strings.TrimSpace(getStringValue(entry["grunt"]))
		if strings.EqualFold(labelType, "Mixed") && strings.EqualFold(gruntLabel, "Grunt") {
			labelType = "Grunt"
		}
		key := fmt.Sprintf("%s|%d", valueType, gender)
		choice := gruntChoices[key]
		if choice == nil {
			choice = &gruntChoice{
				labelType: labelType,
				valueType: valueType,
				gender:    gender,
				seen:      map[string]bool{},
			}
			gruntChoices[key] = choice
		}
		for _, name := range names {
			if name == "" || choice.seen[name] {
				continue
			}
			choice.seen[name] = true
			choice.names = append(choice.names, name)
		}
	}

	gruntEntries := make([]invasionChoice, 0, len(gruntChoices))
	for _, entry := range gruntChoices {
		label := titleCaseWords(entry.labelType)
		if symbol := invasionGenderSymbol(entry.gender); symbol != "" {
			label = label + symbol
		}
		if len(entry.names) > 0 {
			label = fmt.Sprintf("%s (%s)", label, strings.Join(entry.names, ", "))
		}
		value := entry.valueType
		if genderWord := invasionGenderWord(entry.gender); genderWord != "" {
			value = fmt.Sprintf("%s %s", value, genderWord)
		}
		addChoice(&gruntEntries, label, value)
	}

	sort.Slice(gruntEntries, func(i, j int) bool {
		return gruntEntries[i].label < gruntEntries[j].label
	})
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].label < entries[j].label
	})
	all := append(entries, gruntEntries...)
	seen := map[string]bool{}
	for _, entry := range all {
		if len(choices) >= 25 {
			break
		}
		if seen[entry.value] {
			continue
		}
		seen[entry.value] = true
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  truncateChoiceLabel(entry.label),
			Value: entry.value,
		})
	}
	if len(choices) > 25 {
		choices = choices[:25]
	}
	return choices
}

func (d *Discord) autocompleteTemplateChoices(query, templateType string) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	seen := map[string]bool{}
	for _, tpl := range d.manager.templates {
		if tpl.Hidden {
			continue
		}
		if tpl.Platform != "" && !strings.EqualFold(tpl.Platform, "discord") {
			continue
		}
		if templateType == "monster" {
			if tpl.Type != "monster" && tpl.Type != "monsterNoIv" {
				continue
			}
		} else if tpl.Type != templateType {
			continue
		}
		id := strings.TrimSpace(fmt.Sprintf("%v", tpl.ID))
		if id == "" || seen[id] {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(id), query) {
			continue
		}
		seen[id] = true
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  id,
			Value: id,
		})
		if len(choices) >= 25 {
			break
		}
	}
	return choices
}

func (d *Discord) autocompleteRemoveTrackingChoices(query, trackingType string, i *discordgo.InteractionCreate) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.query == nil {
		return nil
	}
	userID, _ := slashUser(i)
	if userID == "" {
		return nil
	}
	profileNo := d.userProfileNo(userID)
	tr := d.manager.i18n.Translator(d.userLanguage(userID))

	// Discord may send the previously-selected choice value back as the focused query.
	// Those values look like "type|uid" and should not be used to filter results,
	// especially when the user switches the "type" option.
	query = strings.TrimSpace(query)
	if strings.Contains(query, "|") {
		query = ""
	}
	query = strings.ToLower(query)
	choices := []*discordgo.ApplicationCommandOptionChoice{}

	fetchLimit := 200
	if query != "" {
		fetchLimit = 5000
	}

	appendChoice := func(label, value string) {
		if label == "" || value == "" {
			return
		}
		if query != "" && !strings.Contains(strings.ToLower(label), query) {
			return
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  label,
			Value: value,
		})
	}

	labelWithProfile := func(row map[string]any, label string) string {
		rowProfile := toInt(row["profile_no"], 0)
		if rowProfile <= 0 {
			return label
		}
		if profileNo == 0 || (profileNo > 0 && rowProfile != profileNo) {
			return fmt.Sprintf("P%d: %s", rowProfile, label)
		}
		return label
	}

	whereByUser := func() map[string]any {
		// Do not scope to profile_no here: if the scheduler set current_profile_no=0 for
		// quiet hours, we'd return no rows and autocomplete would look broken.
		return map[string]any{"id": userID}
	}

	switch strings.ToLower(trackingType) {
	case "pokemon":
		if query == "" {
			appendChoice("Everything (remove all)", "pokemon|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("monsters", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.MonsterRowText(d.manager.cfg, tr, d.manager.data, row))
			appendChoice(label, "pokemon|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "raid":
		if query == "" {
			appendChoice("Everything (remove all)", "raid|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("raid", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.RaidRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner))
			appendChoice(label, "raid|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "maxbattle":
		if query == "" {
			appendChoice("Everything (remove all)", "maxbattle|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("maxbattle", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.MaxbattleRowText(d.manager.cfg, tr, d.manager.data, row))
			appendChoice(label, "maxbattle|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "quest":
		if query == "" {
			appendChoice("Everything (remove all)", "quest|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("quest", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.QuestRowText(d.manager.cfg, tr, d.manager.data, row))
			appendChoice(label, "quest|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "incident", "invasion":
		if query == "" {
			appendChoice("Everything (remove all)", "invasion|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("invasion", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.InvasionRowText(d.manager.cfg, tr, d.manager.data, row))
			appendChoice(label, "invasion|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "lure":
		if query == "" {
			appendChoice("Everything (remove all)", "lure|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("lures", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.LureRowText(d.manager.cfg, tr, d.manager.data, row))
			appendChoice(label, "lure|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "weather":
		if query == "" {
			appendChoice("Everything (remove all)", "weather|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("weather", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.WeatherRowText(tr, d.manager.data, row))
			appendChoice(label, "weather|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "gym":
		if query == "" {
			appendChoice("Everything (remove all)", "gym|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("gym", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.GymRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner))
			appendChoice(label, "gym|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "nest":
		if query == "" {
			appendChoice("Everything (remove all)", "nest|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("nests", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.NestRowText(d.manager.cfg, tr, d.manager.data, row))
			appendChoice(label, "nest|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "fort":
		if query == "" {
			appendChoice("Everything (remove all)", "fort|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("forts", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.FortUpdateRowText(d.manager.cfg, tr, d.manager.data, row))
			appendChoice(label, "fort|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	}

	return choices
}

func slashUser(i *discordgo.InteractionCreate) (string, string) {
	if i == nil {
		return "", ""
	}
	if i.Member != nil && i.Member.User != nil {
		return i.Member.User.ID, i.Member.User.Username
	}
	if i.User != nil {
		return i.User.ID, i.User.Username
	}
	return "", ""
}

func (d *Discord) monsterSearchOptions(query string) []discordgo.SelectMenuOption {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil
	}
	type candidate struct {
		ID   int
		Name string
	}
	candidates := []candidate{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := toInt(mon["id"], 0)
		if name == "" || id == 0 {
			continue
		}
		if name == query || fmt.Sprintf("%d", id) == query {
			candidates = append(candidates, candidate{ID: id, Name: name})
			continue
		}
		if strings.HasPrefix(name, query) || strings.Contains(name, query) {
			candidates = append(candidates, candidate{ID: id, Name: name})
		}
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Name < candidates[j].Name })
	if len(candidates) > 25 {
		candidates = candidates[:25]
	}
	options := make([]discordgo.SelectMenuOption, 0, len(candidates))
	for _, mon := range candidates {
		label := fmt.Sprintf("%s (#%d)", d.titleCase(mon.Name), mon.ID)
		options = append(options, discordgo.SelectMenuOption{
			Label: label,
			Value: fmt.Sprintf("%d", mon.ID),
		})
	}
	return options
}

func (d *Discord) raidLevelOptions() []discordgo.SelectMenuOption {
	if d.manager == nil || d.manager.data == nil {
		return nil
	}
	raw, ok := d.manager.data.UtilData["raidLevels"].(map[string]any)
	if !ok {
		return nil
	}
	levels := []int{}
	for key := range raw {
		if value := toInt(key, 0); value > 0 {
			levels = append(levels, value)
		}
	}
	sort.Ints(levels)
	if len(levels) == 0 {
		return nil
	}
	options := make([]discordgo.SelectMenuOption, 0, len(levels))
	for _, level := range levels {
		options = append(options, discordgo.SelectMenuOption{
			Label: d.raidLevelLabel(level),
			Value: fmt.Sprintf("level%d", level),
		})
	}
	if len(options) > 25 {
		options = options[:25]
	}
	return options
}

func (d *Discord) raidLevelLabel(level int) string {
	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["raidLevels"].(map[string]any); ok {
			if label, ok := raw[fmt.Sprintf("%d", level)]; ok {
				text := strings.TrimSpace(fmt.Sprintf("%v", label))
				lower := strings.ToLower(text)
				if text != "" && !strings.Contains(lower, "level") && !strings.Contains(lower, "tier") && !strings.Contains(lower, "raid") {
					text += " raid"
				}
				if text != "" {
					return d.titleCase(text)
				}
			}
		}
	}
	return fmt.Sprintf("Level %d", level)
}

func (d *Discord) maxbattleLevelLabel(level int) string {
	if level == 90 {
		return "All max battle levels"
	}
	if d.manager != nil && d.manager.data != nil && d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["maxbattleLevels"].(map[string]any); ok {
			if entry, ok := raw[fmt.Sprintf("%d", level)]; ok {
				label := strings.TrimSpace(fmt.Sprintf("%v", entry))
				if label != "" {
					return label
				}
			}
		}
	}
	return fmt.Sprintf("Level %d max battle", level)
}

func (d *Discord) setSlashState(member *discordgo.Member, user *discordgo.User, state *slashBuilderState) {
	userID := slashUserID(member, user)
	if userID == "" {
		return
	}
	d.slashMu.Lock()
	d.slash[userID] = state
	d.slashMu.Unlock()
}

func (d *Discord) getSlashState(member *discordgo.Member, user *discordgo.User) *slashBuilderState {
	userID := slashUserID(member, user)
	if userID == "" {
		return nil
	}
	d.slashMu.Lock()
	state := d.slash[userID]
	d.slashMu.Unlock()
	return state
}

func (d *Discord) clearSlashState(member *discordgo.Member, user *discordgo.User) {
	userID := slashUserID(member, user)
	if userID == "" {
		return
	}
	d.slashMu.Lock()
	delete(d.slash, userID)
	d.slashMu.Unlock()
}

func (d *Discord) registerSlashCommands(s *discordgo.Session) {
	if s == nil || s.State == nil || s.State.User == nil {
		return
	}
	appID := s.State.User.ID
	deregisterSlashCommands := false
	maxDistance := 0
	pvpMaxRank := 0
	gymBattleEnabled := false
	hideTemplateOptions := false
	if d.manager != nil && d.manager.cfg != nil {
		if value, ok := d.manager.cfg.GetInt("tracking.maxDistance"); ok {
			maxDistance = value
		}
		if value, ok := d.manager.cfg.GetInt("general.pvpFilterMaxRank"); ok {
			pvpMaxRank = value
		}
		if value, ok := d.manager.cfg.GetBool("tracking.enableGymBattle"); ok {
			gymBattleEnabled = value
		}
		if value, ok := d.manager.cfg.GetBool("discord.hideTemplateOptions"); ok {
			hideTemplateOptions = value
		}
		// Optional namespaced override.
		if value, ok := d.manager.cfg.GetBool("discord.slash.hideTemplateOptions"); ok {
			hideTemplateOptions = value
		}
		if value, ok := d.manager.cfg.GetBool("discord.slash.deregisterOnStart"); ok {
			deregisterSlashCommands = value
		}
	}
	if pvpMaxRank <= 0 {
		pvpMaxRank = 4096
	}

	if deregisterSlashCommands {
		// Remove global slash commands so the next run can re-register from scratch.
		logger := logging.Get().Discord
		deleteFor := func(guildID string) {
			commands, err := s.ApplicationCommands(appID, guildID)
			if err != nil {
				if logger != nil {
					logger.Warnf("Slash command deregistration failed (guild=%q): %v", guildID, err)
				}
				return
			}
			for _, cmd := range commands {
				if cmd == nil || cmd.ID == "" {
					continue
				}
				_ = s.ApplicationCommandDelete(appID, guildID, cmd.ID)
			}
			if logger != nil {
				scope := "global"
				if guildID != "" {
					scope = guildID
				}
				logger.Infof("Slash commands deregistered (%s): %d", scope, len(commands))
			}
		}

		deleteFor("")
		return
	}

	distanceOption := func() *discordgo.ApplicationCommandOption {
		opt := &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "distance",
			Description: "Distance away in meters",
			MinValue:    floatPtr(0),
		}
		if maxDistance > 0 {
			opt.MaxValue = float64(maxDistance)
		}
		return opt
	}

	templateOptions := func() []*discordgo.ApplicationCommandOption {
		if hideTemplateOptions {
			return nil
		}
		return []*discordgo.ApplicationCommandOption{
			{
				Type:         discordgo.ApplicationCommandOptionString,
				Name:         "template",
				Description:  "Optional template name",
				Autocomplete: true,
			},
		}
	}

	trackOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "pokemon",
			Description:  "Enter Pokemon name",
			Required:     true,
			Autocomplete: true,
		},
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "form",
			Description:  "Select form (optional)",
			Autocomplete: true,
		},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_iv", Description: "Set minimum IV", MinValue: floatPtr(0), MaxValue: 100},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_iv", Description: "Set maximum IV", MinValue: floatPtr(0), MaxValue: 100},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_atk", Description: "Minimum attack", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_atk", Description: "Maximum attack", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_def", Description: "Minimum defense", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_def", Description: "Maximum defense", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_sta", Description: "Minimum stamina", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_sta", Description: "Maximum stamina", MinValue: floatPtr(0), MaxValue: 15},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_cp", Description: "Minimum CP", MinValue: floatPtr(0)},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_cp", Description: "Maximum CP", MinValue: floatPtr(0)},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_level", Description: "Minimum level", MinValue: floatPtr(1), MaxValue: 50},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "max_level", Description: "Maximum level", MinValue: floatPtr(1), MaxValue: 50},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "size",
			Description: "Select size",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "all", Value: "all"},
				{Name: "xxs", Value: "xxs"},
				{Name: "xs", Value: "xs"},
				{Name: "m", Value: "m"},
				{Name: "xl", Value: "xl"},
				{Name: "xxl", Value: "xxl"},
			},
		},
		distanceOption(),
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "gender",
			Description: "Select gender",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "All", Value: "all"},
				{Name: "Male", Value: "male"},
				{Name: "Female", Value: "female"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "pvp_league",
			Description: "Select pvp league (pvp_ranks required)",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "little", Value: "little"},
				{Name: "great", Value: "great"},
				{Name: "ultra", Value: "ultra"},
			},
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        "pvp_ranks",
			Description: "Set number of pvp ranks (pvp_league required)",
			MinValue:    floatPtr(0),
			MaxValue:    float64(pvpMaxRank),
		},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_time", Description: "Minimum time left in seconds", MinValue: floatPtr(0)},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after despawn"},
	}
	trackOptions = append(trackOptions, templateOptions()...)

	gymOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "team",
			Description: "Select team",
			Required:    true,
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Everything", Value: "everything"},
				{Name: "Mystic (Blue)", Value: "mystic"},
				{Name: "Valor (Red)", Value: "valor"},
				{Name: "Instinct (Yellow)", Value: "instinct"},
				{Name: "Uncontested", Value: "uncontested"},
				{Name: "Normal", Value: "normal"},
			},
		},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "slot_changes", Description: "Alert on slot changes"},
		{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
		distanceOption(),
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
	}
	gymOptions = append(gymOptions, templateOptions()...)
	if gymBattleEnabled {
		gymOptions = append(gymOptions, &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        "battle_changes",
			Description: "Alert on battle changes",
		})
	}

	fortOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "type",
			Description: "Select fort type",
			Choices: []*discordgo.ApplicationCommandOptionChoice{
				{Name: "Everything", Value: "everything"},
				{Name: "Pokestop", Value: "pokestop"},
				{Name: "Gym", Value: "gym"},
			},
		},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "include_empty", Description: "Include empty POIs"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "location", Description: "Track location changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "name", Description: "Track name changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "photo", Description: "Track photo changes"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "removal", Description: "Track removals"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "new", Description: "Track new POIs"},
		distanceOption(),
	}
	fortOptions = append(fortOptions, templateOptions()...)

	nestOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "pokemon",
			Description:  "Select Pokemon",
			Required:     true,
			Autocomplete: true,
		},
		{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_spawn", Description: "Optional minimum average spawns", MinValue: floatPtr(0)},
		distanceOption(),
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after despawn"},
	}
	nestOptions = append(nestOptions, templateOptions()...)

	weatherOptions := []*discordgo.ApplicationCommandOption{
		{
			Type:         discordgo.ApplicationCommandOptionString,
			Name:         "condition",
			Description:  "Select weather condition",
			Required:     true,
			Autocomplete: true,
		},
		{Type: discordgo.ApplicationCommandOptionString, Name: "location", Description: "Optional location (address or lat,lon)"},
		{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
	}
	weatherOptions = append(weatherOptions, templateOptions()...)

	commands := []*discordgo.ApplicationCommand{
		{
			Name:        "track",
			Description: "Set Pokemon spawn filters",
			Options:     trackOptions,
		},
		{
			Name:        "raid",
			Description: "Add raid alert",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Enter level or Pokemon", Required: true, Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "team",
					Description: "Select controlling team",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "All", Value: "all"},
						{Name: "Valor (Red)", Value: "red"},
						{Name: "Mystic (Blue)", Value: "blue"},
						{Name: "Instinct (Yellow)", Value: "yellow"},
						{Name: "Uncontested (White)", Value: "white"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "rsvp", Description: "RSVP matching", Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "off", Value: "off"},
					{Name: "on", Value: "on"},
					{Name: "only", Value: "only"},
				}},
				distanceOption(),
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
			}, templateOptions()...),
		},
		{
			Name:        "egg",
			Description: "Add egg alert",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "level", Description: "Select egg level", Required: true, Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "gym", Description: "Optional gym", Autocomplete: true},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "team",
					Description: "Select controlling team",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "All", Value: "all"},
						{Name: "Valor (Red)", Value: "red"},
						{Name: "Mystic (Blue)", Value: "blue"},
						{Name: "Instinct (Yellow)", Value: "yellow"},
						{Name: "Uncontested (White)", Value: "white"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "rsvp", Description: "RSVP matching", Choices: []*discordgo.ApplicationCommandOptionChoice{
					{Name: "off", Value: "off"},
					{Name: "on", Value: "on"},
					{Name: "only", Value: "only"},
				}},
				distanceOption(),
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
			}, templateOptions()...),
		},
		{
			Name:        "maxbattle",
			Description: "Add max battle alert",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Enter level or Pokemon", Required: true, Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionString, Name: "station", Description: "Optional station", Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "gmax_only", Description: "Only Gigantamax battles"},
				distanceOption(),
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
			}, templateOptions()...),
		},
		{
			Name:        "quest",
			Description: "Add quest alert",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Enter Pokemon or item", Required: true, Autocomplete: true},
				{Type: discordgo.ApplicationCommandOptionInteger, Name: "min_amount", Description: "Optional minimum value for items only", MinValue: floatPtr(0)},
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "ar",
					Description: "Optional AR mapping filter",
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Any", Value: "any"},
						{Name: "No AR", Value: "noar"},
						{Name: "With AR", Value: "ar"},
					},
				},
				distanceOption(),
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
			}, templateOptions()...),
		},
		{
			Name:        "invasion",
			Description: "Add invasion alert",
			Options: append([]*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "type", Description: "Enter leader or grunt type", Required: true, Autocomplete: true},
				distanceOption(),
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
			}, templateOptions()...),
		},
		{
			Name:        "gym",
			Description: "Add gym alert",
			Options:     gymOptions,
		},
		{
			Name:        "fort",
			Description: "Add fort update alert",
			Options:     fortOptions,
		},
		{
			Name:        "nest",
			Description: "Add nest alert",
			Options:     nestOptions,
		},
		{
			Name:        "weather",
			Description: "Add weather alert",
			Options:     weatherOptions,
		},
		{
			Name:        "lure",
			Description: "Add lure alert",
			Options: append([]*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Select lure type",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Everything", Value: "everything"},
						{Name: "Basic", Value: "basic"},
						{Name: "Glacial", Value: "glacial"},
						{Name: "Mossy", Value: "mossy"},
						{Name: "Magnetic", Value: "magnetic"},
						{Name: "Rainy", Value: "rainy"},
						{Name: "Golden", Value: "sparkly"},
					},
				},
				distanceOption(),
				{Type: discordgo.ApplicationCommandOptionBoolean, Name: "clean", Description: "Auto delete after expiration"},
			}, templateOptions()...),
		},
		{
			Name:        "language",
			Description: "Change language",
			Options: []*discordgo.ApplicationCommandOption{
				{Type: discordgo.ApplicationCommandOptionString, Name: "to", Description: "Select language", Autocomplete: true},
			},
		},
		{
			Name:        "profile",
			Description: "Profile & Scheduler Settings",
		},
		{
			Name:        "tracked",
			Description: "Show current tracking",
		},
		{
			Name:        "remove",
			Description: "Delete custom trackings",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Select tracking type",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "pokemon", Value: "pokemon"},
						{Name: "raid", Value: "raid"},
						{Name: "maxbattle", Value: "maxbattle"},
						{Name: "invasion", Value: "invasion"},
						{Name: "quest", Value: "quest"},
						{Name: "gym", Value: "gym"},
						{Name: "weather", Value: "weather"},
						{Name: "lure", Value: "lure"},
						{Name: "nest", Value: "nest"},
						{Name: "fort", Value: "fort"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "tracking", Description: "Select tracking to remove", Required: true, Autocomplete: true},
			},
		},
		{
			Name:        "start",
			Description: "Enable alerts",
		},
		{
			Name:        "stop",
			Description: "Disable alerts",
		},
		{
			Name:        "help",
			Description: "Show command help",
		},
		{
			Name:        "info",
			Description: "Look up Poracle info",
			Options: []*discordgo.ApplicationCommandOption{
				{
					Type:        discordgo.ApplicationCommandOptionString,
					Name:        "type",
					Description: "Select info type",
					Required:    true,
					Choices: []*discordgo.ApplicationCommandOptionChoice{
						{Name: "Pokemon", Value: "pokemon"},
						{Name: "Moves", Value: "moves"},
						{Name: "Items", Value: "items"},
						{Name: "Weather", Value: "weather"},
						{Name: "Rarity", Value: "rarity"},
						{Name: "Shiny", Value: "shiny"},
						{Name: "Translate", Value: "translate"},
					},
				},
				{Type: discordgo.ApplicationCommandOptionString, Name: "pokemon", Description: "Pokemon (required when type=pokemon)", Autocomplete: true},
			},
		},
	}
	for _, cmd := range commands {
		_, _ = s.ApplicationCommandCreate(appID, "", cmd)
	}
}

func (d *Discord) titleCase(input string) string {
	if input == "" {
		return input
	}
	return strings.ToUpper(input[:1]) + input[1:]
}

func titleCaseWords(input string) string {
	parts := strings.Fields(strings.TrimSpace(input))
	if len(parts) == 0 {
		return ""
	}
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, " ")
}

type questChoice struct {
	label string
	value string
}

func invasionEventLabel(name string) string {
	if strings.EqualFold(strings.TrimSpace(name), "Gold-Stop") {
		return "Gold Coins"
	}
	return titleCaseWords(name)
}

func invasionGenderSymbol(gender int) string {
	switch gender {
	case 1:
		return "♂"
	case 2:
		return "♀"
	case 3:
		return "⚲"
	default:
		return ""
	}
}

func invasionGenderWord(gender int) string {
	switch gender {
	case 1:
		return "male"
	case 2:
		return "female"
	case 3:
		return "genderless"
	default:
		return ""
	}
}

func formatInvasionArg(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	for _, gender := range []string{"female", "male", "genderless"} {
		if strings.HasSuffix(lower, " "+gender) {
			typePart := strings.TrimSpace(text[:len(text)-len(gender)])
			if strings.ContainsAny(typePart, " \t") {
				typePart = strconv.Quote(typePart)
			}
			return strings.TrimSpace(typePart + " " + gender)
		}
	}
	if strings.ContainsAny(text, " \t") {
		return strconv.Quote(text)
	}
	return text
}

func formatQuestArg(value string) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	formIndex := strings.Index(lower, " form:")
	if formIndex > 0 {
		monster := strings.TrimSpace(text[:formIndex])
		formPart := strings.TrimSpace(text[formIndex+1:])
		if strings.ContainsAny(monster, " \t") {
			monster = strconv.Quote(monster)
		}
		if strings.Contains(formPart, ":") {
			keyValue := strings.SplitN(formPart, ":", 2)
			if len(keyValue) == 2 && strings.ContainsAny(strings.TrimSpace(keyValue[1]), " \t") {
				formPart = keyValue[0] + ":" + strconv.Quote(strings.TrimSpace(keyValue[1]))
			}
		}
		return strings.TrimSpace(monster + " " + formPart)
	}
	if strings.ContainsAny(text, " \t") {
		return strconv.Quote(text)
	}
	return text
}

func truncateChoiceLabel(label string) string {
	const maxRunes = 100
	runes := []rune(label)
	if len(runes) <= maxRunes {
		return label
	}
	if maxRunes <= 3 {
		return string(runes[:maxRunes])
	}
	return string(runes[:maxRunes-3]) + "..."
}

func getStringValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toIntValue(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return 0
}

func (d *Discord) invasionEncounterNames(grunt map[string]any) []string {
	if d == nil || d.manager == nil || d.manager.data == nil || grunt == nil {
		return nil
	}
	raw, ok := grunt["encounters"].(map[string]any)
	if !ok {
		return nil
	}
	out := []string{}
	seen := map[string]bool{}
	for _, key := range []string{"first", "second", "third"} {
		list, ok := raw[key].([]any)
		if !ok {
			continue
		}
		for _, item := range list {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			id := toIntValue(entry["id"])
			formID := toIntValue(entry["form"])
			if id == 0 {
				continue
			}
			name := d.monsterNameWithForm(id, formID)
			if name == "" {
				name = fmt.Sprintf("Pokemon %d", id)
			}
			if seen[name] {
				continue
			}
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

func (d *Discord) questMonsterChoices() []questChoice {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return nil
	}
	entries := []questChoice{}
	seen := map[string]bool{}

	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(getStringValue(mon["name"]))
		if name == "" {
			continue
		}
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		formID := toIntValue(form["id"])
		formName := strings.TrimSpace(getStringValue(form["name"]))
		lowerName := strings.ToLower(name)
		if formID == 0 {
			if !seen[lowerName] {
				entries = append(entries, questChoice{
					label: titleCaseWords(name),
					value: lowerName,
				})
				seen[lowerName] = true
			}
			continue
		}
		if formName == "" || strings.EqualFold(formName, "Normal") {
			continue
		}
		value := fmt.Sprintf("%s form:%s", lowerName, strings.ToLower(formName))
		label := fmt.Sprintf("%s (%s)", titleCaseWords(name), titleCaseWords(formName))
		if !seen[value] {
			entries = append(entries, questChoice{
				label: label,
				value: value,
			})
			seen[value] = true
		}
	}
	return entries
}

func (d *Discord) questItemChoices() []questChoice {
	if d == nil || d.manager == nil || d.manager.data == nil {
		return nil
	}
	entries := []questChoice{}
	seen := map[string]bool{}
	if d.manager.data.UtilData != nil {
		if raw, ok := d.manager.data.UtilData["questItems"].(map[string]any); ok {
			for name := range raw {
				label := titleCaseWords(name)
				value := strings.ToLower(name)
				if value == "" || seen[value] {
					continue
				}
				entries = append(entries, questChoice{label: label, value: value})
				seen[value] = true
			}
		}
	}
	if len(entries) == 0 && d.manager.data.Items != nil {
		for _, raw := range d.manager.data.Items {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name := strings.TrimSpace(getStringValue(item["name"]))
			if name == "" {
				continue
			}
			label := titleCaseWords(name)
			value := strings.ToLower(name)
			if seen[value] {
				continue
			}
			entries = append(entries, questChoice{label: label, value: value})
			seen[value] = true
		}
	}
	return entries
}

func (d *Discord) questMegaEnergyChoices() []questChoice {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return nil
	}
	entries := []questChoice{}
	seen := map[string]bool{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		if toIntValue(form["id"]) != 0 {
			continue
		}
		if temp, ok := mon["tempEvolutions"].([]any); !ok || len(temp) == 0 {
			continue
		}
		name := strings.TrimSpace(getStringValue(mon["name"]))
		if name == "" {
			continue
		}
		value := fmt.Sprintf("energy:%s", strings.ToLower(name))
		if seen[value] {
			continue
		}
		entries = append(entries, questChoice{
			label: fmt.Sprintf("Mega Energy %s", titleCaseWords(name)),
			value: value,
		})
		seen[value] = true
	}
	return entries
}

func (d *Discord) questXLCandyMonsterChoices() []questChoice {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return nil
	}
	entries := []questChoice{}
	seen := map[string]bool{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		if toIntValue(form["id"]) != 0 {
			continue
		}
		name := strings.TrimSpace(getStringValue(mon["name"]))
		if name == "" {
			continue
		}
		value := fmt.Sprintf("xlcandy:%s", strings.ToLower(name))
		if seen[value] {
			continue
		}
		entries = append(entries, questChoice{
			label: fmt.Sprintf("%s XL Candy", titleCaseWords(name)),
			value: value,
		})
		seen[value] = true
	}
	return entries
}

func (d *Discord) questCandyMonsterChoices() []questChoice {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return nil
	}
	entries := []questChoice{}
	seen := map[string]bool{}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		if toIntValue(form["id"]) != 0 {
			continue
		}
		name := strings.TrimSpace(getStringValue(mon["name"]))
		if name == "" {
			continue
		}
		value := fmt.Sprintf("candy:%s", strings.ToLower(name))
		if seen[value] {
			continue
		}
		entries = append(entries, questChoice{
			label: fmt.Sprintf("%s Candy", titleCaseWords(name)),
			value: value,
		})
		seen[value] = true
	}
	return entries
}

func (d *Discord) itemNameByID(id int) string {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Items == nil {
		return ""
	}
	key := fmt.Sprintf("%d", id)
	raw, ok := d.manager.data.Items[key]
	if !ok {
		return ""
	}
	item, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	return getStringValue(item["name"])
}

func (d *Discord) questMonsterLabel(value string) string {
	query := strings.ToLower(strings.TrimSpace(value))
	if query == "" || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return ""
	}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(getStringValue(mon["name"]))
		if name == "" || name != query {
			continue
		}
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		formID := toIntValue(form["id"])
		formName := strings.TrimSpace(getStringValue(form["name"]))
		label := titleCaseWords(name)
		if formID != 0 && formName != "" && !strings.EqualFold(formName, "Normal") {
			return fmt.Sprintf("%s (%s)", label, titleCaseWords(formName))
		}
		return label
	}
	return ""
}

func (d *Discord) monsterNameWithForm(pokemonID, formID int) string {
	name, formName := d.monsterInfo(pokemonID, formID)
	if name == "" {
		return ""
	}
	if formName != "" && !strings.EqualFold(formName, "Normal") {
		return fmt.Sprintf("%s %s", formName, name)
	}
	return name
}

func (d *Discord) monsterInfo(pokemonID, formID int) (string, string) {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return "", ""
	}
	monster := d.lookupMonster(fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = d.lookupMonster(fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = d.lookupMonster(fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return "", ""
	}
	name := getStringValue(monster["name"])
	formName := ""
	if form, ok := monster["form"].(map[string]any); ok {
		formName = getStringValue(form["name"])
	}
	return name, formName
}

func (d *Discord) lookupMonster(key string) map[string]any {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil {
		return nil
	}
	raw, ok := d.manager.data.Monsters[key]
	if !ok {
		return nil
	}
	monster, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return monster
}

func slashUserID(member *discordgo.Member, user *discordgo.User) string {
	if member != nil && member.User != nil {
		return member.User.ID
	}
	if user != nil {
		return user.ID
	}
	return ""
}
