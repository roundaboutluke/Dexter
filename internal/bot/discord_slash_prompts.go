package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/logging"
)

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

func (d *Discord) respondWithMaxbattleOptions(s *discordgo.Session, i *discordgo.InteractionCreate) {
	d.respondWithButtons(s, i, "Track a max battle boss, level, or everything?", []discordgo.MessageComponent{
		discordgo.Button{CustomID: slashChooseEverything, Label: "Everything", Style: discordgo.PrimaryButton},
		discordgo.Button{CustomID: slashChooseSearch, Label: "Boss/Level", Style: discordgo.SecondaryButton},
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

func (d *Discord) respondWithGymTeamSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := []discordgo.SelectMenuOption{
		{Label: "Everything", Value: "everything"},
		{Label: "Mystic (Blue)", Value: "mystic"},
		{Label: "Valor (Red)", Value: "valor"},
		{Label: "Instinct (Yellow)", Value: "instinct"},
		{Label: "Uncontested", Value: "uncontested"},
		{Label: "Normal", Value: "normal"},
	}
	d.respondWithSelectMenu(s, i, "Select a gym team", slashGymTeamSelect, options)
}

func (d *Discord) respondWithFortTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := []discordgo.SelectMenuOption{
		{Label: "Everything", Value: "everything"},
		{Label: "Pokestop", Value: "pokestop"},
		{Label: "Gym", Value: "gym"},
	}
	d.respondWithSelectMenu(s, i, "Select a fort type", slashFortTypeSelect, options)
}

func (d *Discord) respondWithWeatherConditionSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	choices := d.autocompleteWeatherChoices("")
	if len(choices) == 0 {
		d.respondEphemeral(s, i, "No weather conditions found.")
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
	d.respondWithSelectMenu(s, i, "Select a weather condition", slashWeatherConditionSelect, options)
}

func (d *Discord) respondWithLureTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := []discordgo.SelectMenuOption{
		{Label: "Everything", Value: "everything"},
		{Label: "Basic", Value: "basic"},
		{Label: "Glacial", Value: "glacial"},
		{Label: "Mossy", Value: "mossy"},
		{Label: "Magnetic", Value: "magnetic"},
		{Label: "Rainy", Value: "rainy"},
		{Label: "Golden", Value: "sparkly"},
	}
	d.respondWithSelectMenu(s, i, "Select a lure type", slashLureTypeSelect, options)
}

func (d *Discord) respondWithInfoTypeSelect(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := []discordgo.SelectMenuOption{
		{Label: "Pokemon", Value: "pokemon"},
		{Label: "Moves", Value: "moves"},
		{Label: "Items", Value: "items"},
		{Label: "Weather", Value: "weather"},
		{Label: "Rarity", Value: "rarity"},
		{Label: "Shiny", Value: "shiny"},
		{Label: "Translate", Value: "translate"},
	}
	d.respondWithSelectMenu(s, i, "What do you want to look up?", slashInfoTypeSelect, options)
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
	commandLine := strings.TrimSpace(state.Command + " " + strings.Join(state.Args, " "))
	profileNo, profileLabel := d.effectiveProfileInfo(i)
	contextFields := []*discordgo.MessageEmbedField{
		{Name: "Profile", Value: profileLabel, Inline: true},
		{Name: "Command", Value: commandLine, Inline: false},
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
			if field.Name == "Profile" {
				field.Value = profileLabel
			}
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
