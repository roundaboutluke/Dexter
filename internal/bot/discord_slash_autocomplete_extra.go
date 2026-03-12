package bot

import (
	"fmt"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/tracking"
)

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

func (d *Discord) autocompleteProfileChoices(i *discordgo.InteractionCreate, query string, includeAll bool) []*discordgo.ApplicationCommandOptionChoice {
	if d == nil || d.manager == nil || d.manager.query == nil {
		return nil
	}
	_, human, profiles, errText := d.loadSlashProfileContext(i)
	if errText != "" {
		return nil
	}
	query = strings.ToLower(strings.TrimSpace(query))
	effectiveNo := effectiveProfileNoFromHuman(human)
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	addChoice := func(label, value string, aliases ...string) {
		if len(choices) >= 25 || label == "" || value == "" {
			return
		}
		search := strings.ToLower(label + " " + value + " " + strings.Join(aliases, " "))
		if query != "" && !strings.Contains(search, query) {
			return
		}
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  truncateChoiceLabel(label),
			Value: value,
		})
	}
	if row := profileRowByNo(profiles, effectiveNo); row != nil {
		addChoice("Current profile: "+profileDisplayName(row), "effective", "current", "effective", fmt.Sprintf("p%d", effectiveNo))
	} else {
		addChoice(fmt.Sprintf("Current profile: Profile %d", effectiveNo), "effective", "current", "effective", fmt.Sprintf("p%d", effectiveNo))
	}
	for _, row := range profiles {
		profileNo := toInt(row["profile_no"], 0)
		if profileNo <= 0 {
			continue
		}
		addChoice(profileDisplayName(row), fmt.Sprintf("%d", profileNo), fmt.Sprintf("profile %d", profileNo), fmt.Sprintf("p%d", profileNo), fmt.Sprintf("%v", row["name"]))
	}
	if includeAll {
		addChoice("All profiles", "all", "all", "every", "summary")
	}
	return choices
}

func (d *Discord) autocompleteHelpCommandChoices(query string) []*discordgo.ApplicationCommandOptionChoice {
	query = strings.ToLower(strings.TrimSpace(query))
	ids := []string{}
	seen := map[string]bool{}
	if d != nil && d.manager != nil {
		for _, tpl := range d.manager.templates {
			if tpl.Type != "help" {
				continue
			}
			if tpl.Platform != "" && !strings.EqualFold(tpl.Platform, "discord") {
				continue
			}
			id := strings.TrimSpace(fmt.Sprintf("%v", tpl.ID))
			if id == "" || strings.EqualFold(id, "slash") || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		ids = []string{"track", "raid", "quest", "invasion", "tracked", "remove", "profile", "info"}
	}
	sort.Strings(ids)
	choices := []*discordgo.ApplicationCommandOptionChoice{}
	for _, id := range ids {
		if query != "" && !strings.Contains(strings.ToLower(id), query) {
			continue
		}
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

func (d *Discord) autocompleteRemoveTrackingChoices(query, trackingType, profileToken string, i *discordgo.InteractionCreate) []*discordgo.ApplicationCommandOptionChoice {
	if d.manager == nil || d.manager.query == nil {
		return nil
	}
	selection, errText := d.resolveSlashProfileSelection(i, profileToken)
	if errText != "" {
		return nil
	}
	tr := d.slashTranslator(d.userLanguage(selection.UserID))

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

	truncateChoiceLabel := func(value string) string {
		const max = 100 // Discord limit for choice name
		value = strings.TrimSpace(value)
		if value == "" {
			return value
		}
		r := []rune(value)
		if len(r) <= max {
			return value
		}
		if max <= 3 {
			return string(r[:max])
		}
		return string(r[:max-3]) + "..."
	}

	appendChoice := func(label, value string) {
		if label == "" || value == "" {
			return
		}
		label = truncateChoiceLabel(label)
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
		if selection.Mode == slashProfileScopeAll {
			return fmt.Sprintf("P%d: %s", rowProfile, label)
		}
		return label
	}

	whereByUser := func() map[string]any {
		where := map[string]any{"id": selection.UserID}
		if selection.Mode != slashProfileScopeAll && selection.ProfileNo > 0 {
			where["profile_no"] = selection.ProfileNo
		}
		return where
	}

	removeAllLabel := func(typeName string) string {
		if selection.Mode == slashProfileScopeAll {
			return fmt.Sprintf("Everything in all profiles (%s)", typeName)
		}
		return fmt.Sprintf("Everything in %s (%s)", selection.TargetLabel(), typeName)
	}

	switch strings.ToLower(trackingType) {
	case "pokemon":
		if query == "" {
			appendChoice(removeAllLabel("remove all"), "pokemon|all")
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
			appendChoice(removeAllLabel("remove all"), "raid|all")
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
	case "egg":
		if query == "" {
			appendChoice(removeAllLabel("remove all"), "egg|all")
		}
		rows, err := d.manager.query.SelectAllQueryLimit("egg", whereByUser(), fetchLimit)
		if err != nil {
			return nil
		}
		for _, row := range rows {
			uid := strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
			if uid == "" {
				continue
			}
			label := labelWithProfile(row, tracking.EggRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner))
			appendChoice(label, "egg|"+uid)
			if len(choices) >= 25 {
				break
			}
		}
	case "maxbattle":
		if query == "" {
			appendChoice(removeAllLabel("remove all"), "maxbattle|all")
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
			appendChoice(removeAllLabel("remove all"), "quest|all")
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
			appendChoice(removeAllLabel("remove all"), "invasion|all")
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
			appendChoice(removeAllLabel("remove all"), "lure|all")
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
			appendChoice(removeAllLabel("remove all"), "weather|all")
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
			appendChoice(removeAllLabel("remove all"), "gym|all")
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
			appendChoice(removeAllLabel("remove all"), "nest|all")
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
			appendChoice(removeAllLabel("remove all"), "fort|all")
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
