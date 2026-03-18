package bot

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"dexter/internal/util"
)

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

// getStringValue delegates to util.GetString for converting arbitrary values to string.
var getStringValue = util.GetString

// toIntValue converts an arbitrary value to int (zero fallback).
func toIntValue(value any) int {
	return util.ToInt(value, 0)
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

// questRewardMonsterChoices builds autocomplete choices by iterating all monsters,
// filtering to base forms (form ID 0) and applying an optional extra filter.
// prefix is used for the value (e.g. "energy", "candy", "xlcandy"),
// labelFmt is a fmt string with one %s placeholder for the monster name.
func (d *Discord) questRewardMonsterChoices(prefix, labelFmt string, filter func(mon map[string]any) bool) []questChoice {
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
		if filter != nil && !filter(mon) {
			continue
		}
		name := strings.TrimSpace(getStringValue(mon["name"]))
		if name == "" {
			continue
		}
		value := fmt.Sprintf("%s:%s", prefix, strings.ToLower(name))
		if seen[value] {
			continue
		}
		entries = append(entries, questChoice{
			label: fmt.Sprintf(labelFmt, titleCaseWords(name)),
			value: value,
		})
		seen[value] = true
	}
	return entries
}

func hasTempEvolutions(mon map[string]any) bool {
	temp, ok := mon["tempEvolutions"].([]any)
	return ok && len(temp) > 0
}

func (d *Discord) questMegaEnergyChoices() []questChoice {
	return d.questRewardMonsterChoices("energy", "Mega Energy %s", hasTempEvolutions)
}

func (d *Discord) questXLCandyMonsterChoices() []questChoice {
	return d.questRewardMonsterChoices("xlcandy", "%s XL Candy", nil)
}

func (d *Discord) questCandyMonsterChoices() []questChoice {
	return d.questRewardMonsterChoices("candy", "%s Candy", nil)
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

// gruntIDFromTypeName finds the first grunt ID matching a "<type> [gender]" string.
func (d *Discord) gruntIDFromTypeName(input string) int {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Grunts == nil {
		return 0
	}
	input = strings.ToLower(strings.TrimSpace(input))
	if input == "" || input == "everything" {
		return 0
	}
	for key, raw := range d.manager.data.Grunts {
		entry, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName := strings.ToLower(strings.TrimSpace(getStringValue(entry["type"])))
		if typeName == "" {
			continue
		}
		gender := toIntValue(entry["gender"])
		genderWord := invasionGenderWord(gender)
		candidate := typeName
		if genderWord != "" {
			candidate = typeName + " " + genderWord
		}
		if candidate == input || typeName == input {
			return toInt(key, 0)
		}
	}
	return 0
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

// sortQuestChoicesActiveFirst sorts quest choices with active items first, then alphabetical.
func sortQuestChoicesActiveFirst(entries []questChoice, activeValues map[string]bool) {
	sort.Slice(entries, func(i, j int) bool {
		iActive := activeValues[strings.ToLower(entries[i].value)]
		jActive := activeValues[strings.ToLower(entries[j].value)]
		if iActive != jActive {
			return iActive
		}
		return entries[i].label < entries[j].label
	})
}

// activeQuestItemNames returns a set of lowercased item value strings for currently-active quest items.
func (d *Discord) activeQuestItemNames() map[string]bool {
	if d == nil || d.manager == nil || d.manager.processor == nil {
		return nil
	}
	ra := d.manager.processor.RecentActivity()
	if ra == nil {
		return nil
	}
	ids := ra.ActiveQuestItems()
	if len(ids) == 0 {
		return nil
	}
	idSet := make(map[int]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	result := make(map[string]bool)
	if d.manager.data != nil && d.manager.data.Items != nil {
		for key, raw := range d.manager.data.Items {
			itemID := toInt(key, 0)
			if !idSet[itemID] {
				continue
			}
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name := strings.ToLower(strings.TrimSpace(getStringValue(item["name"])))
			if name != "" {
				result[name] = true
			}
		}
	}
	return result
}

// activeQuestPokemonNames returns a set of lowercased pokemon name strings for currently-active quest encounter rewards.
func (d *Discord) activeQuestPokemonNames() map[string]bool {
	if d == nil || d.manager == nil || d.manager.processor == nil {
		return nil
	}
	ra := d.manager.processor.RecentActivity()
	if ra == nil {
		return nil
	}
	ids := ra.ActiveQuestPokemon()
	if len(ids) == 0 {
		return nil
	}
	return d.monsterIDsToNameSet(ids, "")
}

// activeQuestCandyNames returns a set of lowercased value strings for currently-active quest candy rewards.
func (d *Discord) activeQuestCandyNames() map[string]bool {
	if d == nil || d.manager == nil || d.manager.processor == nil {
		return nil
	}
	ra := d.manager.processor.RecentActivity()
	if ra == nil {
		return nil
	}
	ids := ra.ActiveQuestCandy()
	if len(ids) == 0 {
		return nil
	}
	return d.monsterIDsToNameSet(ids, "candy")
}

// activeQuestMegaEnergyNames returns a set of lowercased value strings for currently-active mega energy quest rewards.
func (d *Discord) activeQuestMegaEnergyNames() map[string]bool {
	if d == nil || d.manager == nil || d.manager.processor == nil {
		return nil
	}
	ra := d.manager.processor.RecentActivity()
	if ra == nil {
		return nil
	}
	ids := ra.ActiveQuestMegaEnergy()
	if len(ids) == 0 {
		return nil
	}
	return d.monsterIDsToNameSet(ids, "energy")
}

// monsterIDsToNameSet converts a list of pokemon IDs to a set of lowercased value strings.
// If prefix is non-empty, values are formatted as "prefix:name"; otherwise just "name".
func (d *Discord) monsterIDsToNameSet(ids []int, prefix string) map[string]bool {
	if d == nil || d.manager == nil || d.manager.data == nil || d.manager.data.Monsters == nil || len(ids) == 0 {
		return nil
	}
	idSet := make(map[int]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	result := make(map[string]bool)
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := toInt(mon["id"], 0)
		if !idSet[id] {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toIntValue(form["id"]) != 0 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(getStringValue(mon["name"])))
		if name != "" {
			if prefix != "" {
				result[prefix+":"+name] = true
			} else {
				result[name] = true
			}
		}
	}
	return result
}
