package bot

import (
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
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
	slashGymTeamSelect               = "poracle:track:gym:team"
	slashFortTypeSelect              = "poracle:track:fort:type"
	slashWeatherConditionSelect      = "poracle:track:weather:condition"
	slashLureTypeSelect              = "poracle:track:lure:type"
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
	slashInfoTypeSelect              = "poracle:info:type"
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

func (d *Discord) slashCommandsDisabled() bool {
	if d == nil || d.manager == nil || d.manager.cfg == nil {
		return false
	}
	disabled, ok := d.manager.cfg.GetBool("discord.slash.disabled")
	return ok && disabled
}

func (d *Discord) onInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil {
		return
	}
	if d.slashCommandsDisabled() {
		// When disabled we intentionally do NOT respond to interactions, so another
		// /command provider using the same bot token can handle them.
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
			choices = d.autocompleteRaidTypeChoices(i, query)
		} else if focused.Name == "gym" {
			choices = d.autocompleteGymChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "raid")
		}
	case "maxbattle":
		if focused.Name == "type" {
			choices = d.autocompleteMaxbattleTypeChoices(i, query)
		} else if focused.Name == "station" {
			choices = d.autocompleteStationChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "maxbattle")
		}
	case "quest":
		if focused.Name == "type" {
			choices = d.autocompleteQuestTypeChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "quest")
		}
	case "egg":
		if focused.Name == "level" {
			choices = d.autocompleteRaidLevelChoices(i, query)
		} else if focused.Name == "gym" {
			choices = d.autocompleteGymChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "egg")
		}
	case "invasion":
		if focused.Name == "type" {
			choices = d.autocompleteIncidentTypeChoices(i, query)
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
		if focused.Name == "profile" {
			choices = d.autocompleteProfileChoices(i, query, true)
		} else if focused.Name == "tracking" {
			trackingType, _ := optionString(options, "type")
			profileToken, _ := optionString(options, "profile")
			choices = d.autocompleteRemoveTrackingChoices(query, trackingType, profileToken, i)
		}
	case "tracked":
		if focused.Name == "profile" {
			choices = d.autocompleteProfileChoices(i, query, true)
		}
	case "help":
		if focused.Name == "command" {
			choices = d.autocompleteHelpCommandChoices(query)
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
		d.respondEphemeral(s, i, d.slashText(i, "Canceled."))
		return
	}
	if data.CustomID == slashInfoTypeSelect {
		if len(data.Values) == 0 {
			return
		}
		d.handleSlashInfoTypeChoice(s, i, data.Values[0])
		return
	}
	if data.CustomID == slashInfoWeatherUseSaved {
		d.executeSlashLineDeferred(s, i, "info weather")
		return
	}
	if data.CustomID == slashInfoWeatherEnterCoordinates {
		tr := d.slashInteractionTranslator(i)
		d.respondWithModal(s, i, slashInfoWeatherModal, tr.Translate("Weather info", false), tr.Translate("Coordinates (lat,lon)", false), "51.5,-0.12")
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
		title, label, placeholder := d.profileLocationModalText(i)
		d.respondWithModal(s, i, slashProfileLocationMod, title, label, placeholder)
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
		title, label, placeholder := d.profileCreateModalText(i)
		d.respondWithModal(s, i, slashProfileCreateMod, title, label, placeholder)
		return
	}

	state := d.getSlashState(i.Member, i.User)
	if state == nil || state.ExpiresAt.Before(time.Now()) {
		d.clearSlashState(i.Member, i.User)
		d.respondEphemeral(s, i, d.slashExpiredText(i))
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
		case "maxbattle":
			d.respondWithRaidInput(s, i)
		case "nest", "info-pokemon":
			d.respondWithMonsterSearch(s, i)
		}
		return
	case slashMonsterSelect:
		if len(data.Values) == 0 {
			return
		}
		if state.Command == "info" && state.Step == "info-pokemon" {
			d.clearSlashState(i.Member, i.User)
			d.executeSlashLineDeferred(s, i, "info "+data.Values[0])
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
	case slashGymTeamSelect:
		if len(data.Values) == 0 {
			return
		}
		state.Args = []string{data.Values[0]}
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashFortTypeSelect:
		if len(data.Values) == 0 {
			return
		}
		state.Args = []string{data.Values[0]}
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashWeatherConditionSelect:
		if len(data.Values) == 0 {
			return
		}
		args, errText := d.guidedWeatherArgs(i, data.Values[0])
		if errText != "" {
			d.respondEphemeral(s, i, errText)
			return
		}
		state.Args = args
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashLureTypeSelect:
		if len(data.Values) == 0 {
			return
		}
		state.Args = []string{data.Values[0]}
		d.respondWithFiltersPrompt(s, i, state)
		return
	case slashConfirmButton:
		if state != nil && state.Command == "location" {
			d.clearSlashRenderMessage(i.Message)
			userID, _ := slashUser(i)
			if userID == "" || d.manager == nil || d.manager.query == nil {
				d.respondEphemeral(s, i, d.slashText(i, "Target is not registered."))
				return
			}
			line := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
			result := d.buildSlashExecutionResult(s, i, line)
			human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
			if err != nil {
				d.respondEphemeral(s, i, d.slashText(i, "Target is not registered."))
				return
			}
			target := ""
			if len(state.Args) > 0 {
				target = state.Args[0]
			}
			refreshProfile, message := profileLocationOutcome(human, target, result)
			if !refreshProfile {
				d.respondEphemeral(s, i, message)
				return
			}
			d.respondProfilePayload(s, i, "")
			d.clearSlashState(i.Member, i.User)
			return
		}
		// Clear the confirmation prompt buttons immediately, then send the command output as a follow-up.
		text, embeds, components := slashConfirmCloseoutPayload(i)
		d.respondUpdateComponentsEmbed(s, i, text, embeds, components)
		line := ""
		if state != nil {
			line = strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
		}
		result := d.buildSlashExecutionResult(s, i, line)
		d.followupEphemeralSlashReply(s, i, result.Reply)
		d.clearSlashState(i.Member, i.User)
		return
	case slashCancelButton:
		if state != nil && state.Command == "location" {
			d.clearSlashRenderMessage(i.Message)
			d.respondProfilePayload(s, i, "")
			d.clearSlashState(i.Member, i.User)
			return
		}
		// Clear the confirmation prompt buttons once canceled.
		d.respondUpdateComponentsEmbed(s, i, d.slashText(i, "Canceled."), nil, []discordgo.MessageComponent{})
		d.clearSlashState(i.Member, i.User)
		return
	case slashFiltersModal:
		d.respondWithFiltersInput(s, i)
		return
	}
}

func slashConfirmCloseoutPayload(i *discordgo.InteractionCreate) (string, []*discordgo.MessageEmbed, []discordgo.MessageComponent) {
	if i == nil || i.Message == nil {
		return "", nil, []discordgo.MessageComponent{}
	}
	return i.Message.Content, i.Message.Embeds, []discordgo.MessageComponent{}
}

func (d *Discord) handleSlashModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	if data.CustomID == slashInfoTranslateModal {
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, d.slashText(i, "Please enter text to translate."))
			return
		}
		d.executeSlashLineDeferred(s, i, "info translate "+query)
		return
	}
	if data.CustomID == slashInfoWeatherModal {
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, d.slashText(i, "Please enter coordinates (lat,lon)."))
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
		d.respondEphemeral(s, i, d.slashExpiredText(i))
		return
	}
	switch data.CustomID {
	case slashMonsterSearch:
		query := modalTextValue(data, "query")
		if strings.TrimSpace(query) == "" {
			d.respondEphemeral(s, i, d.slashText(i, "Please enter a Pokemon name or ID."))
			return
		}
		if state.Command == "info" && state.Step == "info-pokemon" && strings.EqualFold(strings.TrimSpace(query), "everything") {
			d.respondEphemeral(s, i, d.slashText(i, "Please pick a specific Pokemon."))
			return
		}
		if strings.EqualFold(strings.TrimSpace(query), "everything") {
			state.Args = []string{"everything"}
			d.respondWithFiltersPrompt(s, i, state)
			return
		}
		options := d.monsterSearchOptions(query)
		if len(options) == 0 {
			d.respondEphemeral(s, i, d.slashText(i, "No Pokemon matched that search."))
			return
		}
		d.respondWithSelectMenu(s, i, d.slashText(i, "Select a Pokemon"), slashMonsterSelect, options)
	case slashRaidInput:
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, d.slashText(i, "Please enter a raid boss name or level."))
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
			d.respondEphemeral(s, i, d.slashText(i, "Please enter quest filters (e.g. reward:items)."))
			return
		}
		state.Args = splitQuotedArgs(query)
		d.respondWithFiltersPrompt(s, i, state)
	case slashInvasionInput:
		query := strings.TrimSpace(modalTextValue(data, "query"))
		if query == "" {
			d.respondEphemeral(s, i, d.slashText(i, "Please enter invasion filters (e.g. grunt type)."))
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
