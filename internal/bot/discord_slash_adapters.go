package bot

import (
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// slashComponentExactHandlers dispatches component interactions by exact customID match.
var slashComponentExactHandlers = map[string]func(*Discord, *discordgo.Session, *discordgo.InteractionCreate){
	slashInfoCancelButton:            (*Discord).handleComponentInfoCancel,
	slashInfoWeatherUseSaved:         (*Discord).handleComponentWeatherUseSaved,
	slashInfoWeatherEnterCoordinates: (*Discord).handleComponentWeatherEnterCoords,
	slashProfileScheduleAddGlobal:    (*Discord).handleProfileScheduleAddGlobal,
	slashProfileScheduleOverview:     (*Discord).handleProfileScheduleOverview,
	slashProfileScheduleToggle:       (*Discord).handleProfileScheduleToggle,
	slashProfileLocation:             (*Discord).handleComponentProfileLocation,
	slashProfileLocationClear:        (*Discord).handleProfileLocationClear,
	slashProfileArea:                 (*Discord).handleProfileAreaShow,
	slashProfileAreaBack:             (*Discord).handleProfileShow,
	slashProfileBack:                 (*Discord).handleProfileShow,
	slashProfileCreate:               (*Discord).handleComponentProfileCreate,
	slashInfoTypeSelect:              (*Discord).handleComponentInfoTypeSelect,
	slashAreaShowSelect:              (*Discord).handleComponentAreaShowSelect,
	slashProfileSelect:               (*Discord).handleComponentProfileSelect,
	slashProfileScheduleDayGlobal:    (*Discord).handleComponentScheduleDayGlobal,
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

// --- Adapter methods for exact-match component handlers ---

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

// --- Adapter methods for prefix-match component handlers ---

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
