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
	ProfileNo       int
	ProfileLabel    string
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
	case "pokemon":
		d.handleSlashTrack(s, i)
	case "track":
		d.handleSlashTrack(s, i)
	case "raid":
		switch slashSubcommand(data) {
		case "boss":
			d.handleSlashRaidBoss(s, i)
		case "level":
			d.handleSlashRaidLevel(s, i)
		case "egg":
			d.handleSlashRaidEgg(s, i)
		default:
			d.handleSlashRaid(s, i)
		}
	case "egg":
		d.handleSlashEgg(s, i)
	case "maxbattle":
		switch slashSubcommand(data) {
		case "boss":
			d.handleSlashMaxbattleBoss(s, i)
		case "level":
			d.handleSlashMaxbattleLevel(s, i)
		default:
			d.handleSlashMaxbattle(s, i)
		}
	case "quest":
		switch slashSubcommand(data) {
		case "pokemon":
			d.handleSlashQuestPokemon(s, i)
		case "item":
			d.handleSlashQuestItem(s, i)
		case "stardust":
			d.handleSlashQuestStardust(s, i)
		case "candy":
			d.handleSlashQuestCandy(s, i)
		case "mega-energy":
			d.handleSlashQuestMegaEnergy(s, i)
		default:
			d.handleSlashQuest(s, i)
		}
	case "rocket":
		d.handleSlashRocket(s, i)
	case "pokestop-event":
		d.handleSlashPokestopEvent(s, i)
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
	case "filters":
		switch slashSubcommand(data) {
		case "show":
			d.handleSlashTracked(s, i)
		case "remove":
			d.handleSlashRemove(s, i)
		}
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

func slashCommandAddsTracking(data discordgo.ApplicationCommandInteractionData) bool {
	switch data.Name {
	case "pokemon", "track", "rocket", "pokestop-event", "invasion", "gym", "fort", "nest", "weather", "lure", "egg":
		return true
	case "raid", "maxbattle":
		return true
	case "quest":
		return true
	default:
		return false
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
	if focused.Name == "profile" && slashCommandAddsTracking(data) {
		choices = d.autocompleteProfileChoices(i, query, false)
	}
	switch data.Name {
	case "pokemon":
		if choices != nil {
			break
		}
		if focused.Name == "pokemon" {
			choices = d.autocompletePokemonChoices(query)
		} else if focused.Name == "form" {
			pokemon, _ := optionString(options, "pokemon")
			choices = d.autocompletePokemonFormChoices(query, pokemon)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "monster")
		}
	case "track":
		if choices != nil {
			break
		}
		if focused.Name == "pokemon" {
			choices = d.autocompletePokemonChoices(query)
		} else if focused.Name == "form" {
			pokemon, _ := optionString(options, "pokemon")
			choices = d.autocompletePokemonFormChoices(query, pokemon)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "monster")
		}
	case "raid":
		if choices != nil {
			break
		}
		switch slashSubcommand(data) {
		case "boss":
			if focused.Name == "pokemon" {
				choices = d.autocompletePokemonChoices(query)
			} else if focused.Name == "gym" {
				choices = d.autocompleteGymChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "raid")
			}
		case "level":
			if focused.Name == "level" {
				choices = d.autocompleteRaidLevelChoices(i, query)
			} else if focused.Name == "gym" {
				choices = d.autocompleteGymChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "raid")
			}
		case "egg":
			if focused.Name == "level" {
				choices = d.autocompleteRaidLevelChoices(i, query)
			} else if focused.Name == "gym" {
				choices = d.autocompleteGymChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "egg")
			}
		default:
			if focused.Name == "type" {
				choices = d.autocompleteRaidTypeChoices(i, query)
			} else if focused.Name == "gym" {
				choices = d.autocompleteGymChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "raid")
			}
		}
	case "maxbattle":
		if choices != nil {
			break
		}
		switch slashSubcommand(data) {
		case "boss":
			if focused.Name == "pokemon" {
				choices = d.autocompletePokemonChoices(query)
			} else if focused.Name == "station" {
				choices = d.autocompleteStationChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "maxbattle")
			}
		case "level":
			if focused.Name == "level" {
				choices = d.autocompleteMaxbattleLevelChoices(i, query)
			} else if focused.Name == "station" {
				choices = d.autocompleteStationChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "maxbattle")
			}
		default:
			if focused.Name == "type" {
				choices = d.autocompleteMaxbattleTypeChoices(i, query)
			} else if focused.Name == "station" {
				choices = d.autocompleteStationChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "maxbattle")
			}
		}
	case "quest":
		if choices != nil {
			break
		}
		switch slashSubcommand(data) {
		case "pokemon":
			if focused.Name == "pokemon" {
				choices = d.autocompleteQuestPokemonChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "quest")
			}
		case "item":
			if focused.Name == "item" {
				choices = d.autocompleteQuestItemRewardChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "quest")
			}
		case "candy":
			if focused.Name == "pokemon" {
				choices = d.autocompleteQuestCandyRewardChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "quest")
			}
		case "mega-energy":
			if focused.Name == "pokemon" {
				choices = d.autocompleteQuestMegaEnergyRewardChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "quest")
			}
		case "stardust":
			if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "quest")
			}
		default:
			if focused.Name == "type" {
				choices = d.autocompleteQuestTypeChoices(i, query)
			} else if focused.Name == "template" {
				choices = d.autocompleteTemplateChoices(query, "quest")
			}
		}
	case "egg":
		if choices != nil {
			break
		}
		if focused.Name == "level" {
			choices = d.autocompleteRaidLevelChoices(i, query)
		} else if focused.Name == "gym" {
			choices = d.autocompleteGymChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "egg")
		}
	case "invasion":
		if choices != nil {
			break
		}
		if focused.Name == "type" {
			choices = d.autocompleteIncidentTypeChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "invasion")
		}
	case "rocket":
		if choices != nil {
			break
		}
		if focused.Name == "type" {
			choices = d.autocompleteRocketTypeChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "invasion")
		}
	case "pokestop-event":
		if choices != nil {
			break
		}
		if focused.Name == "type" {
			choices = d.autocompletePokestopEventChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "invasion")
		}
	case "gym":
		if choices != nil {
			break
		}
		if focused.Name == "gym" {
			choices = d.autocompleteGymChoices(i, query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "gym")
		}
	case "fort":
		if choices != nil {
			break
		}
		if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "forts")
		}
	case "nest":
		if choices != nil {
			break
		}
		if focused.Name == "pokemon" {
			choices = d.autocompletePokemonChoices(query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "nests")
		}
	case "weather":
		if choices != nil {
			break
		}
		if focused.Name == "condition" {
			choices = d.autocompleteWeatherChoices(query)
		} else if focused.Name == "template" {
			choices = d.autocompleteTemplateChoices(query, "weather")
		}
	case "lure":
		if choices != nil {
			break
		}
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
	case "filters":
		switch slashSubcommand(data) {
		case "remove":
			if focused.Name == "profile" {
				choices = d.autocompleteProfileChoices(i, query, true)
			} else if focused.Name == "tracking" {
				trackingType, _ := optionString(options, "type")
				profileToken, _ := optionString(options, "profile")
				choices = d.autocompleteRemoveTrackingChoices(query, trackingType, profileToken, i)
			}
		case "show":
			if focused.Name == "profile" {
				choices = d.autocompleteProfileChoices(i, query, true)
			}
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

// slashComponentExactHandlers dispatches component interactions by exact customID match.
var slashComponentExactHandlers = map[string]func(*Discord, *discordgo.Session, *discordgo.InteractionCreate){
	slashInfoCancelButton:          (*Discord).handleComponentInfoCancel,
	slashInfoWeatherUseSaved:       (*Discord).handleComponentWeatherUseSaved,
	slashInfoWeatherEnterCoordinates: (*Discord).handleComponentWeatherEnterCoords,
	slashProfileScheduleAddGlobal:  (*Discord).handleProfileScheduleAddGlobal,
	slashProfileScheduleOverview:   (*Discord).handleProfileScheduleOverview,
	slashProfileScheduleToggle:     (*Discord).handleProfileScheduleToggle,
	slashProfileLocation:           (*Discord).handleComponentProfileLocation,
	slashProfileLocationClear:      (*Discord).handleProfileLocationClear,
	slashProfileArea:               (*Discord).handleProfileAreaShow,
	slashProfileAreaBack:           (*Discord).handleProfileShow,
	slashProfileBack:               (*Discord).handleProfileShow,
	slashProfileCreate:             (*Discord).handleComponentProfileCreate,
	slashInfoTypeSelect:            (*Discord).handleComponentInfoTypeSelect,
	slashAreaShowSelect:            (*Discord).handleComponentAreaShowSelect,
	slashProfileSelect:             (*Discord).handleComponentProfileSelect,
	slashProfileScheduleDayGlobal:  (*Discord).handleComponentScheduleDayGlobal,
}

// slashComponentPrefixHandlers dispatches component interactions by customID prefix.
// Longer prefixes that are substrings of shorter ones must appear first.
var slashComponentPrefixHandlers = []struct {
	prefix  string
	handler func(*Discord, *discordgo.Session, *discordgo.InteractionCreate, string)
}{
	{slashAreaShowAdd, (*Discord).handleComponentAreaShowAdd},
	{slashProfileSet, (*Discord).handleComponentProfileSet},
	{slashProfileScheduleAdd, (*Discord).handleComponentScheduleAdd},
	{slashProfileScheduleBack, (*Discord).handleComponentScheduleBack},
	{slashProfileScheduleClear, (*Discord).handleComponentScheduleClear},
	{slashProfileScheduleRemoveGlobal, (*Discord).handleComponentScheduleRemoveGlobal},
	{slashProfileScheduleRemove, (*Discord).handleComponentScheduleRemove},
	{slashProfileDeleteConfirm, (*Discord).handleComponentDeleteConfirm},
	{slashProfileDeleteCancel, (*Discord).handleComponentDeleteCancel},
	{slashProfileDelete, (*Discord).handleComponentDeletePrompt},
	{slashProfileScheduleEditGlobal, (*Discord).handleComponentScheduleEditGlobal},
	{slashProfileScheduleDay, (*Discord).handleComponentScheduleDay},
	{slashProfileScheduleAssign, (*Discord).handleComponentScheduleAssign},
	{slashProfileScheduleEditDay, (*Discord).handleComponentScheduleEditDay},
	{slashProfileScheduleEditAssign, (*Discord).handleComponentScheduleEditAssign},
	{slashFilterRemoveButtonPrefix, (*Discord).handleComponentFilterRemove},
	{slashFilterRestoreButtonPrefix, (*Discord).handleComponentFilterRestore},
	{slashAreaShowRemove, (*Discord).handleComponentAreaShowRemove},
}

// --- Adapter methods for exact-match handlers that need inline logic ---

func (d *Discord) handleComponentInfoCancel(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondEphemeral(s, i, d.slashText(i, "Canceled."))
}

func (d *Discord) handleComponentWeatherUseSaved(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.executeSlashLineDeferred(s, i, "info weather")
}

func (d *Discord) handleComponentWeatherEnterCoords(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithModal(s, i, slashInfoWeatherModal, tr.Translate("Weather info", false), tr.Translate("Coordinates (lat,lon)", false), "51.5,-0.12")
}

func (d *Discord) handleComponentProfileLocation(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.setSlashState(i.Member, i.User, &slashBuilderState{
		Step:            "profile-location",
		ExpiresAt:       time.Now().Add(5 * time.Minute),
		OriginMessageID: i.Message.ID,
		OriginChannelID: i.ChannelID,
	})
	title, label, placeholder := d.profileLocationModalText(i)
	d.respondWithModal(s, i, slashProfileLocationMod, title, label, placeholder)
}

func (d *Discord) handleComponentProfileCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	title, label, placeholder := d.profileCreateModalText(i)
	d.respondWithModal(s, i, slashProfileCreateMod, title, label, placeholder)
}

func (d *Discord) handleComponentInfoTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleSlashInfoTypeChoice(s, i, data.Values[0])
}

func (d *Discord) handleComponentAreaShowSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleAreaShowSelect(s, i, data.Values[0])
}

func (d *Discord) handleComponentProfileSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleProfileSelect(s, i, data.Values[0])
}

func (d *Discord) handleComponentScheduleDayGlobal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleProfileScheduleDayGlobal(s, i, data.Values)
}

// --- Adapter methods for prefix-match handlers ---

func (d *Discord) handleComponentAreaShowAdd(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	if strings.TrimSpace(suffix) != "" {
		d.handleAreaShowToggle(s, i, suffix, true)
	}
}

func (d *Discord) handleComponentProfileSet(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	if strings.TrimSpace(suffix) != "" {
		d.handleProfileSet(s, i, suffix)
	}
}

func (d *Discord) handleComponentScheduleAdd(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	if strings.TrimSpace(suffix) != "" {
		d.handleProfileScheduleAdd(s, i, suffix)
	}
}

func (d *Discord) handleComponentScheduleBack(s *discordgo.Session, i *discordgo.InteractionCreate, _ string) {
	d.handleProfileShow(s, i)
}

func (d *Discord) handleComponentScheduleClear(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	if strings.TrimSpace(suffix) != "" {
		d.handleProfileScheduleClear(s, i, suffix)
	}
}

func (d *Discord) handleComponentScheduleRemoveGlobal(s *discordgo.Session, i *discordgo.InteractionCreate, _ string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleProfileScheduleRemoveGlobal(s, i, data.Values[0])
}

func (d *Discord) handleComponentScheduleRemove(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	if strings.TrimSpace(suffix) != "" {
		d.handleProfileScheduleRemove(s, i, suffix, data.Values[0])
	}
}

func (d *Discord) handleComponentDeleteConfirm(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	if strings.TrimSpace(suffix) != "" {
		d.handleProfileDeleteConfirm(s, i, suffix)
	}
}

func (d *Discord) handleComponentDeleteCancel(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	d.handleProfileDeleteCancel(s, i, suffix)
}

func (d *Discord) handleComponentDeletePrompt(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	if strings.TrimSpace(suffix) != "" {
		d.handleProfileDeletePrompt(s, i, suffix)
	}
}

func (d *Discord) handleComponentScheduleEditGlobal(s *discordgo.Session, i *discordgo.InteractionCreate, _ string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleProfileScheduleEditSelect(s, i, data.Values[0])
}

func (d *Discord) handleComponentScheduleDay(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	if strings.TrimSpace(suffix) != "" {
		d.handleProfileScheduleDay(s, i, suffix, data.Values[0])
	}
}

func (d *Discord) handleComponentScheduleAssign(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleProfileScheduleAssign(s, i, suffix, data.Values[0])
}

func (d *Discord) handleComponentScheduleEditDay(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleProfileScheduleEditDay(s, i, suffix, data.Values[0])
}

func (d *Discord) handleComponentScheduleEditAssign(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	data := i.MessageComponentData()
	if len(data.Values) == 0 {
		return
	}
	d.handleProfileScheduleEditAssign(s, i, suffix, data.Values[0])
}

func (d *Discord) handleComponentFilterRemove(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	d.handleSlashFilterRemoveAction(s, i, suffix)
}

func (d *Discord) handleComponentFilterRestore(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	d.handleSlashFilterRestoreAction(s, i, suffix)
}

func (d *Discord) handleComponentAreaShowRemove(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	if strings.TrimSpace(suffix) != "" {
		d.handleAreaShowToggle(s, i, suffix, false)
	}
}

func (d *Discord) handleSlashComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	customID := data.CustomID

	if handler, ok := slashComponentExactHandlers[customID]; ok {
		handler(d, s, i)
		return
	}
	for _, entry := range slashComponentPrefixHandlers {
		if strings.HasPrefix(customID, entry.prefix) {
			suffix := strings.TrimPrefix(customID, entry.prefix)
			entry.handler(d, s, i, suffix)
			return
		}
	}

	state := d.getSlashState(i.Member, i.User)
	if state == nil || state.ExpiresAt.Before(time.Now()) {
		d.clearSlashState(i.Member, i.User)
		d.respondEphemeralError(s, i, d.slashExpiredText(i))
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
		if state != nil && state.Step != "confirm" && slashTrackingTypeFromCommand(state.Command) != "" {
			d.promptSlashConfirmationState(s, i, state, d.confirmTitle(i, state.Command), nil)
			return
		}
		if state != nil && state.Command == "location" {
			d.clearSlashRenderMessage(i.Message)
			userID, _ := slashUser(i)
			if userID == "" || d.manager == nil || d.manager.query == nil {
				d.respondEphemeralError(s, i, d.slashText(i, "Target is not registered."))
				return
			}
			line := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
			result := d.buildSlashExecutionResult(s, i, line)
			human, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID})
			if err != nil {
				d.respondEphemeralError(s, i, d.slashText(i, "Target is not registered."))
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
		line := ""
		if state != nil {
			line = strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
		}
		trackingType := ""
		table := ""
		profileLabel := ""
		beforeRows := []map[string]any(nil)
		selection := slashProfileSelection{}
		if state != nil {
			trackingType = slashTrackingTypeFromCommand(state.Command)
			if trackingType != "" {
				if resolved, errText := d.slashProfileSelectionForState(i, state); errText == "" {
					selection = resolved
					beforeRows, table = d.slashTrackingRowsForSelection(selection, trackingType)
					profileLabel = selection.TargetLabelLocalized(d.slashInteractionTranslator(i))
					state.ProfileNo = selection.ProfileNo
					state.ProfileLabel = profileLabel
				}
			}
		}
		profileNo := 0
		if state != nil {
			profileNo = state.ProfileNo
		}
		result := d.buildSlashExecutionResultForProfile(s, i, line, profileNo)
		if result.Success() && table != "" && selection.UserID != "" {
			afterRows, _ := d.slashTrackingRowsForSelection(selection, trackingType)
			changedRows := slashChangedRows(beforeRows, afterRows)
			if embeds, components, ok := d.slashFilterMutationResponse(i, "added", trackingType, table, profileLabel, selection.UserID, changedRows); ok {
				d.respondUpdateComponentsEmbed(s, i, "", embeds, components)
				d.clearSlashState(i.Member, i.User)
				return
			}
		}
		// Clear the confirmation prompt buttons immediately, then send the command output as a follow-up.
		text, embeds, components := slashConfirmCloseoutPayload(i)
		d.respondUpdateComponentsEmbed(s, i, text, embeds, components)
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

// slashModalExactHandlers dispatches modal submissions by exact customID match.
var slashModalExactHandlers = map[string]func(*Discord, *discordgo.Session, *discordgo.InteractionCreate){
	slashInfoTranslateModal: (*Discord).handleModalTranslate,
	slashInfoWeatherModal:   (*Discord).handleModalWeather,
	slashProfileCreateMod:   (*Discord).handleModalProfileCreate,
	slashProfileLocationMod: (*Discord).handleModalProfileLocation,
}

// slashModalPrefixHandlers dispatches modal submissions by customID prefix.
var slashModalPrefixHandlers = []struct {
	prefix  string
	handler func(*Discord, *discordgo.Session, *discordgo.InteractionCreate, string)
}{
	{slashProfileScheduleTime, (*Discord).handleModalScheduleTime},
	{slashProfileScheduleEditTime, (*Discord).handleModalScheduleEditTime},
}

// --- Adapter methods for modal exact-match handlers ---

func (d *Discord) handleModalTranslate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	query := strings.TrimSpace(modalTextValue(data, "query"))
	if query == "" {
		d.respondEphemeral(s, i, d.slashText(i, "Please enter text to translate."))
		return
	}
	d.executeSlashLineDeferred(s, i, "info translate "+query)
}

func (d *Discord) handleModalWeather(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	query := strings.TrimSpace(modalTextValue(data, "query"))
	if query == "" {
		d.respondEphemeral(s, i, d.slashText(i, "Please enter coordinates (lat,lon)."))
		return
	}
	d.executeSlashLineDeferred(s, i, "info weather "+query)
}

func (d *Discord) handleModalProfileCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	d.handleProfileCreate(s, i, strings.TrimSpace(modalTextValue(data, "query")))
}

func (d *Discord) handleModalProfileLocation(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	d.handleLocationInput(s, i, strings.TrimSpace(modalTextValue(data, "query")))
}

// --- Adapter methods for modal prefix-match handlers ---

func (d *Discord) handleModalScheduleTime(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	data := i.ModalSubmitData()
	d.handleProfileScheduleTime(s, i, suffix, data)
}

func (d *Discord) handleModalScheduleEditTime(s *discordgo.Session, i *discordgo.InteractionCreate, suffix string) {
	data := i.ModalSubmitData()
	d.handleProfileScheduleEditTime(s, i, suffix, data)
}

func (d *Discord) handleSlashModal(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	customID := data.CustomID

	if handler, ok := slashModalExactHandlers[customID]; ok {
		handler(d, s, i)
		return
	}
	for _, entry := range slashModalPrefixHandlers {
		if strings.HasPrefix(customID, entry.prefix) {
			suffix := strings.TrimPrefix(customID, entry.prefix)
			entry.handler(d, s, i, suffix)
			return
		}
	}

	state := d.getSlashState(i.Member, i.User)
	if state == nil || state.ExpiresAt.Before(time.Now()) {
		d.clearSlashState(i.Member, i.User)
		d.respondEphemeralError(s, i, d.slashExpiredText(i))
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
			d.respondEphemeralError(s, i, d.slashText(i, "No Pokemon matched that search."))
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
