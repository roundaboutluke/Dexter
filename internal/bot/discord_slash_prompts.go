package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
)

func (d *Discord) respondWithTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	options := []discordgo.SelectMenuOption{
		{Label: tr.Translate("Monster", false), Value: "monster"},
		{Label: tr.Translate("Raid", false), Value: "raid"},
		{Label: tr.Translate("Egg", false), Value: "egg"},
		{Label: tr.Translate("Quest", false), Value: "quest"},
		{Label: tr.Translate("Invasion", false), Value: "invasion"},
	}
	d.respondWithSelectMenu(s, i, tr.Translate("What do you want to track?", false), slashTrackTypeSelect, options)
}

func (d *Discord) respondWithMonsterOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithButtons(s, i, tr.Translate("Track a specific monster or everything?", false), []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseEverything, Label: tr.Translate("Everything", false), Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: slashChooseSearch, Label: tr.Translate("Search", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
	})
}

func (d *Discord) respondWithRaidOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithButtons(s, i, tr.Translate("Track raid boss, level, or everything?", false), []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseEverything, Label: tr.Translate("Everything", false), Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: slashChooseSearch, Label: tr.Translate("Boss/Level", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
	})
}

func (d *Discord) respondWithEggOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithButtons(s, i, tr.Translate("Track an egg level?", false), []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseSearch, Label: tr.Translate("Pick Level", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
	})
}

func (d *Discord) respondWithMaxbattleOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithButtons(s, i, tr.Translate("Track a max battle boss, level, or everything?", false), []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseEverything, Label: tr.Translate("Everything", false), Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: slashChooseSearch, Label: tr.Translate("Boss/Level", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
	})
}

func (d *Discord) respondWithQuestInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithModal(s, i, slashQuestInput, tr.Translate("Quest filters", false), tr.Translate("Filters", false), "reward:items d500 clean")
}

func (d *Discord) respondWithInvasionInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithModal(s, i, slashInvasionInput, tr.Translate("Invasion filters", false), tr.Translate("Filters", false), "grunt_type:fire d500 clean")
}

func (d *Discord) respondWithMonsterSearch(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithModal(s, i, slashMonsterSearch, tr.Translate("Search Pokemon", false), tr.Translate("Name or ID", false), "bulbasaur")
}

func (d *Discord) respondWithRaidInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithModal(s, i, slashRaidInput, tr.Translate("Raid boss or level", false), tr.Translate("Boss or level", false), "rayquaza or level5")
}

func (d *Discord) respondWithEggLevelSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	options := d.raidLevelOptions(tr)
	if len(options) == 0 {
		d.respondEphemeral(s, i, tr.Translate("No raid levels found.", false))
		return
	}
	d.respondWithSelectMenu(s, i, tr.Translate("Select egg level", false), slashEggLevelSelect, options)
}

func (d *Discord) respondWithFiltersInput(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	d.respondWithModal(s, i, slashFiltersModal, tr.Translate("Extra filters", false), tr.Translate("Args", false), "atk:15 def:15 sta:15 d500 clean")
}

func (d *Discord) respondWithGymTeamSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	options := []discordgo.SelectMenuOption{
		{Label: tr.Translate("Everything", false), Value: "everything"},
		{Label: tr.Translate("Mystic (Blue)", false), Value: "mystic"},
		{Label: tr.Translate("Valor (Red)", false), Value: "valor"},
		{Label: tr.Translate("Instinct (Yellow)", false), Value: "instinct"},
		{Label: tr.Translate("Uncontested", false), Value: "uncontested"},
		{Label: tr.Translate("Normal", false), Value: "normal"},
	}
	d.respondWithSelectMenu(s, i, tr.Translate("Select a gym team", false), slashGymTeamSelect, options)
}

func (d *Discord) respondWithFortTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	options := []discordgo.SelectMenuOption{
		{Label: tr.Translate("Everything", false), Value: "everything"},
		{Label: tr.Translate("Pokestop", false), Value: "pokestop"},
		{Label: tr.Translate("Gym", false), Value: "gym"},
	}
	d.respondWithSelectMenu(s, i, tr.Translate("Select a fort type", false), slashFortTypeSelect, options)
}

func (d *Discord) respondWithWeatherConditionSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	choices := d.autocompleteWeatherChoices("")
	if len(choices) == 0 {
		d.respondEphemeral(s, i, tr.Translate("No weather conditions found.", false))
		return
	}
	options := make([]discordgo.SelectMenuOption, 0, len(choices))
	for _, choice := range choices {
		if choice == nil {
			continue
		}
		options = append(options, discordgo.SelectMenuOption{
			Label: choice.Name,
			Value: fmt.Sprintf("%v", choice.Value),
		})
	}
	d.respondWithSelectMenu(s, i, tr.Translate("Select a weather condition", false), slashWeatherConditionSelect, options)
}

func (d *Discord) respondWithLureTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	options := []discordgo.SelectMenuOption{
		{Label: tr.Translate("Everything", false), Value: "everything"},
		{Label: tr.Translate("Basic", false), Value: "basic"},
		{Label: tr.Translate("Glacial", false), Value: "glacial"},
		{Label: tr.Translate("Mossy", false), Value: "mossy"},
		{Label: tr.Translate("Magnetic", false), Value: "magnetic"},
		{Label: tr.Translate("Rainy", false), Value: "rainy"},
		{Label: tr.Translate("Golden", false), Value: "sparkly"},
	}
	d.respondWithSelectMenu(s, i, tr.Translate("Select a lure type", false), slashLureTypeSelect, options)
}

func (d *Discord) respondWithInfoTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	tr := d.slashInteractionTranslator(i)
	options := []discordgo.SelectMenuOption{
		{Label: tr.Translate("Pokemon", false), Value: "pokemon"},
		{Label: tr.Translate("Moves", false), Value: "moves"},
		{Label: tr.Translate("Items", false), Value: "items"},
		{Label: tr.Translate("Weather", false), Value: "weather"},
		{Label: tr.Translate("Rarity", false), Value: "rarity"},
		{Label: tr.Translate("Shiny", false), Value: "shiny"},
		{Label: tr.Translate("Translate", false), Value: "translate"},
	}
	d.respondWithSelectMenu(s, i, tr.Translate("What do you want to look up?", false), slashInfoTypeSelect, options)
}

func (d *Discord) respondWithFiltersPrompt(s *discordgo.Session, i *discordgo.InteractionCreate, state *slashBuilderState) {
	tr := d.slashInteractionTranslator(i)
	commandLine := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
	content := tr.TranslateFormat("Ready to run `{0}`", commandLine)
	d.respondWithButtons(s, i, content, []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashFiltersModal, Label: tr.Translate("Add filters", false), Style: discordgo.SecondaryButton},
		discordgo.Button{CustomID: slashConfirmButton, Label: tr.Translate("Verify", false), Style: discordgo.SuccessButton},
		discordgo.Button{CustomID: slashCancelButton, Label: tr.Translate("Cancel", false), Style: discordgo.DangerButton},
	})
}

func (d *Discord) confirmTitle(i *discordgo.InteractionCreate, command string) string {
	tr := d.slashInteractionTranslator(i)
	switch strings.ToLower(command) {
	case "track":
		return tr.Translate("New Pokemon Alert:", false)
	case "raid":
		return tr.Translate("New Raid Alert:", false)
	case "egg":
		return tr.Translate("New Egg Alert:", false)
	case "maxbattle":
		return tr.Translate("New Max Battle Alert:", false)
	case "quest":
		return tr.Translate("New Quest Alert:", false)
	case "invasion":
		return tr.Translate("New Invasion Alert:", false)
	case "incident":
		return tr.Translate("New Pokestop Event Alert:", false)
	case "gym":
		return tr.Translate("New Gym Alert:", false)
	case "fort":
		return tr.Translate("New Fort Alert:", false)
	case "nest":
		return tr.Translate("New Nest Alert:", false)
	case "weather":
		return tr.Translate("New Weather Alert:", false)
	case "lure":
		return tr.Translate("New Lure Alert:", false)
	default:
		return tr.Translate("Confirm Command:", false)
	}
}

func slashSemanticCommandName(data discordgo.ApplicationCommandInteractionData) string {
	switch strings.ToLower(strings.TrimSpace(data.Name)) {
	case "pokemon":
		return "track"
	case "rocket":
		return "invasion"
	case "pokestop-event":
		return "incident"
	default:
		return strings.ToLower(strings.TrimSpace(data.Name))
	}
}

func (d *Discord) confirmFields(i *discordgo.InteractionCreate) []*discordgo.MessageEmbedField {
	if i == nil {
		return nil
	}
	tr := d.slashInteractionTranslator(i)
	data := i.ApplicationCommandData()
	commandName := slashSemanticCommandName(data)
	options := slashOptions(data)
	if len(options) == 0 {
		return nil
	}

	inline := strings.EqualFold(commandName, "track")
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
		if strings.EqualFold(commandName, "track") {
			if opt.Name == "pvp_ranks" {
				continue
			}
			if opt.Name == "pvp_league" {
				if ranks := findOption("pvp_ranks"); ranks != nil {
					fields = append(fields, &discordgo.MessageEmbedField{
						Name:   localizedOptionName(tr, opt.Name),
						Value:  d.formatConfirmValue(commandName, opt.Name, opt.Value, tr),
						Inline: inline,
					})
					fields = append(fields, &discordgo.MessageEmbedField{
						Name:   localizedOptionName(tr, ranks.Name),
						Value:  d.formatConfirmValue(commandName, ranks.Name, ranks.Value, tr),
						Inline: inline,
					})
					continue
				}
			}
		}
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   localizedOptionName(tr, opt.Name),
			Value:  d.formatConfirmValue(commandName, opt.Name, opt.Value, tr),
			Inline: inline,
		})
	}
	return fields
}

func (d *Discord) formatConfirmValue(command, name string, value any, tr *i18n.Translator) string {
	switch v := value.(type) {
	case string:
		text := strings.TrimSpace(v)
		if text == "" {
			return ""
		}
		lower := strings.ToLower(text)
		if lower == "everything" {
			return translateOrDefault(tr, "Everything")
		}
		if name == "form" {
			if lower == "all" {
				return translateOrDefault(tr, "All forms")
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
		if (command == "invasion" || command == "incident") && name == "type" {
			return d.invasionTypeLabel(text, tr)
		}
		if command == "quest" && name == "type" {
			return d.questTypeLabel(text, tr)
		}
		if name == "pokemon" || (command == "raid" && name == "type") || (command == "quest" && name == "type") {
			if id, ok := parseIntString(lower); ok {
				return d.pokemonLabel(id)
			}
		}
		if ((command == "egg" || command == "raid") && name == "level") || (command == "raid" && name == "type") {
			if level, ok := parseLevelString(lower); ok {
				return d.raidLevelLabel(level, tr)
			}
		}
		if command == "maxbattle" && (name == "type" || name == "level") {
			if level, ok := parseLevelString(lower); ok {
				return d.maxbattleLevelLabel(level, tr)
			}
		}
		return text
	case bool:
		if v {
			return translateOrDefault(tr, "Yes")
		}
		return translateOrDefault(tr, "No")
	case float64:
		return d.formatConfirmValue(command, name, strconv.Itoa(int(v)), tr)
	case int:
		return d.formatConfirmValue(command, name, strconv.Itoa(v), tr)
	case int64:
		return d.formatConfirmValue(command, name, strconv.FormatInt(v, 10), tr)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func localizedOptionName(tr *i18n.Translator, name string) string {
	if tr != nil {
		if translated := translateSlashName(tr, "option", name); translated != name {
			return humanizeOptionName(translated)
		}
	}
	return humanizeOptionName(name)
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

func (d *Discord) invasionTypeLabel(value string, tr *i18n.Translator) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	if lower == "everything" {
		return translateOrDefault(tr, "Everything")
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

func (d *Discord) questTypeLabel(value string, tr *i18n.Translator) string {
	text := strings.TrimSpace(value)
	if text == "" {
		return ""
	}
	lower := strings.ToLower(text)
	if lower == "everything" {
		return translateOrDefault(tr, "Everything")
	}
	if lower == "candy" {
		return translateOrDefault(tr, "Rare Candy")
	}
	if strings.HasPrefix(lower, "candy:") {
		mon := strings.TrimSpace(text[len("candy:"):])
		if name := d.questMonsterLabel(mon); name != "" {
			if tr != nil {
				return tr.TranslateFormat("{0} Candy", name)
			}
			return fmt.Sprintf("%s Candy", name)
		}
		if tr != nil {
			return tr.TranslateFormat("{0} Candy", titleCaseWords(mon))
		}
		return fmt.Sprintf("%s Candy", titleCaseWords(mon))
	}
	if lower == "xl candy" || lower == "xlcandy" {
		return translateOrDefault(tr, "Rare Candy XL")
	}
	if lower == "stardust" {
		return translateOrDefault(tr, "Stardust")
	}
	if lower == "experience" {
		return translateOrDefault(tr, "Experience")
	}
	if lower == "energy" {
		return translateOrDefault(tr, "Mega Energy")
	}
	if strings.HasPrefix(lower, "energy:") {
		mon := strings.TrimSpace(text[len("energy:"):])
		if name := d.questMonsterLabel(mon); name != "" {
			if tr != nil {
				return tr.TranslateFormat("Mega Energy {0}", name)
			}
			return fmt.Sprintf("Mega Energy %s", name)
		}
		if tr != nil {
			return tr.TranslateFormat("Mega Energy {0}", titleCaseWords(mon))
		}
		return fmt.Sprintf("Mega Energy %s", titleCaseWords(mon))
	}
	if strings.HasPrefix(lower, "xlcandy:") {
		mon := strings.TrimSpace(text[len("xlcandy:"):])
		if name := d.questMonsterLabel(mon); name != "" {
			if tr != nil {
				return tr.TranslateFormat("{0} XL Candy", name)
			}
			return fmt.Sprintf("%s XL Candy", name)
		}
		if tr != nil {
			return tr.TranslateFormat("{0} XL Candy", titleCaseWords(mon))
		}
		return fmt.Sprintf("%s XL Candy", titleCaseWords(mon))
	}
	if strings.HasPrefix(lower, "form:") {
		form := strings.TrimSpace(text[len("form:"):])
		if tr != nil {
			return tr.TranslateFormat("Form {0}", titleCaseWords(form))
		}
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
	commandLine := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
	profileNo, profileLabel := d.effectiveProfileInfo(i)
	tr := d.slashInteractionTranslator(i)
	profileFieldName := translateOrDefault(tr, "Profile")
	commandFieldName := translateOrDefault(tr, "Command")
	contextFields := []*discordgo.MessageEmbedField{
		{Name: profileFieldName, Value: profileLabel, Inline: true},
		{Name: commandFieldName, Value: commandLine, Inline: false},
	}
	if len(fields) == 0 {
		fields = contextFields
	} else {
		fields = append(fields, contextFields...)
	}
	if profileNo > 0 && len(fields) > 0 {
		for _, field := range fields {
			if field == nil {
				continue
			}
			if field.Name == profileFieldName {
				field.Value = profileLabel
			}
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:  title,
		Fields: fields,
	}
	d.respondComponentsEmbed(s, i, "", []*discordgo.MessageEmbed{embed}, []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: slashConfirmButton, Label: translateOrDefault(tr, "Verify"), Style: discordgo.SuccessButton},
			discordgo.Button{CustomID: slashCancelButton, Label: translateOrDefault(tr, "Cancel"), Style: discordgo.DangerButton},
		}},
	}, true)
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
