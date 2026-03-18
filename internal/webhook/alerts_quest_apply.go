package webhook

import (
	"fmt"
	"strings"

	"dexter/internal/i18n"
)

func matchQuestWithVariants(hook *Hook, row map[string]any, rewardDataNoAR, rewardDataAR map[string]any) (bool, bool, bool) {
	if hook == nil || row == nil {
		return false, false, false
	}
	arMode := getInt(row["ar"])
	noARMatch := false
	arMatch := false
	if arMode == 0 || arMode == 1 {
		noARMatch = matchQuestWithData(hook, row, rewardDataNoAR)
	}
	if arMode == 0 || arMode == 2 {
		arMatch = matchQuestWithData(hook, row, rewardDataAR)
	}
	switch arMode {
	case 1:
		return noARMatch, noARMatch, false
	case 2:
		return arMatch, false, arMatch
	default:
		return noARMatch || arMatch, noARMatch, arMatch
	}
}

func applyQuestRewardDetails(p *Processor, data map[string]any, rewardData map[string]any, platform string, tr *i18n.Translator) {
	if data == nil || rewardData == nil {
		return
	}
	monsters, _ := rewardData["monsters"].([]map[string]any)
	items, _ := rewardData["items"].([]map[string]any)
	energyMonsters, _ := rewardData["energyMonsters"].([]map[string]any)
	candy, _ := rewardData["candy"].([]map[string]any)
	dustAmount := getInt(rewardData["dustAmount"])
	data["dustAmount"] = dustAmount
	data["itemAmount"] = getInt(rewardData["itemAmount"])
	if len(items) == 0 {
		data["itemAmount"] = 0
	}
	if len(energyMonsters) > 0 {
		data["energyAmount"] = getInt(energyMonsters[0]["amount"])
	} else {
		data["energyAmount"] = 0
	}
	if len(candy) > 0 {
		data["candyAmount"] = getInt(candy[0]["amount"])
	} else {
		data["candyAmount"] = 0
	}
	if len(monsters) > 0 {
		data["isShiny"] = getBool(monsters[0]["shiny"])
		pokemonID := getInt(monsters[0]["pokemonId"])
		formID := getInt(monsters[0]["formId"])
		if p != nil && p.shinyPossible != nil && pokemonID > 0 {
			data["shinyPossible"] = p.shinyPossible.IsPossible(pokemonID, formID)
		} else {
			data["shinyPossible"] = false
		}
		if p != nil {
			if stats, ok := lookupMonsterStats(p, pokemonID, formID); ok {
				data["baseStats"] = stats
			}
		}
	} else {
		data["isShiny"] = false
		data["shinyPossible"] = false
	}
	for _, monster := range monsters {
		pokemonID := getInt(monster["pokemonId"])
		formID := getInt(monster["formId"])
		name, formName := monsterInfo(p, pokemonID, formID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		if strings.EqualFold(formName, "Normal") {
			formName = ""
		}
		monster["nameEng"] = name
		monster["formEng"] = formName
		monster["name"] = translateMaybe(tr, name)
		monster["form"] = translateMaybe(tr, formName)
		fullNameEng := name
		if formName != "" {
			fullNameEng = fmt.Sprintf("%s %s", name, formName)
		}
		fullName := translateMaybe(tr, name)
		if translatedForm := getString(monster["form"]); translatedForm != "" {
			fullName = fmt.Sprintf("%s %s", fullName, translatedForm)
		}
		monster["fullNameEng"] = fullNameEng
		monster["fullName"] = fullName
	}
	data["monsterNames"] = joinQuestMonsterNames(monsters, "fullName")
	data["monsterNamesEng"] = joinQuestMonsterNames(monsters, "fullNameEng")
	for _, item := range items {
		itemID := getInt(item["id"])
		name := itemName(p, itemID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown item"), itemID)
		}
		item["nameEng"] = name
		item["name"] = translateMaybe(tr, name)
	}
	data["itemNames"] = joinQuestItemNames(items, true)
	data["itemNamesEng"] = joinQuestItemNames(items, false)
	for _, monster := range energyMonsters {
		pokemonID := getInt(monster["pokemonId"])
		name := monsterName(p, pokemonID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		monster["nameEng"] = name
		monster["name"] = translateMaybe(tr, name)
	}
	for _, monster := range candy {
		pokemonID := getInt(monster["pokemonId"])
		name := monsterName(p, pokemonID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		monster["nameEng"] = name
		monster["name"] = translateMaybe(tr, name)
	}
	data["energyMonstersNames"] = joinQuestEnergyNames(energyMonsters, tr, "Mega Energy")
	data["energyMonstersNamesEng"] = joinQuestEnergyNames(energyMonsters, nil, "Mega Energy")
	data["candyMonstersNames"] = joinQuestEnergyNames(candy, tr, "Candy")
	data["candyMonstersNamesEng"] = joinQuestEnergyNames(candy, nil, "Candy")
	rewardString := []string{
		data["monsterNames"].(string),
		"",
		data["itemNames"].(string),
		data["energyMonstersNames"].(string),
		data["candyMonstersNames"].(string),
	}
	if dustAmount > 0 {
		rewardString[1] = fmt.Sprintf("%d %s", dustAmount, translateMaybe(tr, "Stardust"))
	}
	rewardStringEng := []string{
		data["monsterNamesEng"].(string),
		"",
		data["itemNamesEng"].(string),
		data["energyMonstersNamesEng"].(string),
		data["candyMonstersNamesEng"].(string),
	}
	if dustAmount > 0 {
		rewardStringEng[1] = fmt.Sprintf("%d Stardust", dustAmount)
	}
	data["rewardString"] = joinNonEmpty(rewardString)
	data["rewardStringEng"] = joinNonEmpty(rewardStringEng)
	if shinyPossible, ok := data["shinyPossible"].(bool); ok && shinyPossible {
		data["shinyPossibleEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, "shiny", platform))
	} else {
		data["shinyPossibleEmoji"] = ""
	}
}

func applyQuestRewardImages(p *Processor, data map[string]any, rewardData map[string]any) {
	if p == nil || p.cfg == nil || data == nil || rewardData == nil {
		return
	}
	shiny := getBool(data["isShiny"])
	if !shiny {
		if getBoolFromConfig(p.cfg, "general.requestShinyImages", false) && getBool(data["shinyPossible"]) {
			shiny = true
		}
	}
	if base := imageBaseURL(p.cfg, "quest", "general.images", "general.imgUrl"); base != "" {
		if url := questRewardIconURL(base, "png", rewardData, shiny); url != "" {
			data["imgUrl"] = url
		}
	}
	if base := imageBaseURL(p.cfg, "quest", "general.images", "general.imgUrlAlt"); base != "" {
		if url := questRewardIconURL(base, "png", rewardData, shiny); url != "" {
			data["imgUrlAlt"] = url
		}
	}
	if base := imageBaseURL(p.cfg, "quest", "general.stickers", "general.stickerUrl"); base != "" {
		if url := questRewardIconURL(base, "webp", rewardData, shiny); url != "" {
			data["stickerUrl"] = url
		}
	}
}

func questRewardIconURL(baseURL, imageType string, rewardData map[string]any, shiny bool) string {
	if baseURL == "" || rewardData == nil {
		return ""
	}
	if !isUiconsRepo(baseURL, imageType) {
		return ""
	}
	client := uiconsClient(baseURL, imageType)
	monsters, _ := rewardData["monsters"].([]map[string]any)
	if len(monsters) > 0 {
		pokemonID := getInt(monsters[0]["pokemonId"])
		formID := getInt(monsters[0]["formId"])
		if url, ok := client.PokemonIcon(pokemonID, formID, 0, 0, 0, 0, shiny, 0); ok {
			return url
		}
	}
	items, _ := rewardData["items"].([]map[string]any)
	if len(items) > 0 {
		itemID := getInt(items[0]["id"])
		if url, ok := client.RewardItemIcon(itemID); ok {
			return url
		}
	}
	if dustAmount := getInt(rewardData["dustAmount"]); dustAmount > 0 {
		if url, ok := client.RewardStardustIcon(dustAmount); ok {
			return url
		}
	}
	energyMonsters, _ := rewardData["energyMonsters"].([]map[string]any)
	if len(energyMonsters) > 0 {
		pokemonID := getInt(energyMonsters[0]["pokemonId"])
		amount := getInt(energyMonsters[0]["amount"])
		if url, ok := client.RewardMegaEnergyIcon(pokemonID, amount); ok {
			return url
		}
	}
	candy, _ := rewardData["candy"].([]map[string]any)
	if len(candy) > 0 {
		pokemonID := getInt(candy[0]["pokemonId"])
		amount := getInt(candy[0]["amount"])
		if url, ok := client.RewardCandyIcon(pokemonID, amount); ok {
			return url
		}
	}
	return ""
}

func joinQuestMonsterNames(monsters []map[string]any, key string) string {
	names := make([]string, 0, len(monsters))
	for _, monster := range monsters {
		if name := getString(monster[key]); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func joinQuestItemNames(items []map[string]any, translated bool) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		nameKey := "nameEng"
		if translated {
			nameKey = "name"
		}
		name := getString(item[nameKey])
		amount := getInt(item["amount"])
		if name == "" {
			continue
		}
		if amount > 0 {
			names = append(names, fmt.Sprintf("%d %s", amount, name))
		} else {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func joinQuestEnergyNames(items []map[string]any, tr *i18n.Translator, suffix string) string {
	names := make([]string, 0, len(items))
	suffixText := suffix
	if tr != nil && suffix != "" {
		suffixText = translateMaybe(tr, suffix)
	}
	for _, item := range items {
		nameKey := "nameEng"
		if tr != nil {
			nameKey = "name"
		}
		name := getString(item[nameKey])
		amount := getInt(item["amount"])
		if name == "" {
			continue
		}
		if suffixText != "" {
			names = append(names, fmt.Sprintf("%d %s %s", amount, name, suffixText))
		} else {
			names = append(names, fmt.Sprintf("%d %s", amount, name))
		}
	}
	return strings.Join(names, ", ")
}

func joinNonEmpty(values []string) string {
	return joinNonEmptyWithSep(values, ", ")
}

func joinNonEmptyWithSep(values []string, sep string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, sep)
}

func formatTemplate(input string, args ...any) string {
	result := input
	for i := len(args) - 1; i >= 0; i-- {
		needle := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, needle, fmt.Sprintf("%v", args[i]))
	}
	return result
}

func getIntFromMap(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	return getInt(m[key])
}

func getBoolFromMap(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	return getBool(m[key])
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func lookupMonsterStats(p *Processor, pokemonID, formID int) (map[string]any, bool) {
	if p == nil {
		return nil, false
	}
	d := p.getData()
	if d == nil {
		return nil, false
	}
	monster := lookupMonster(d, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(d, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(d, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return nil, false
	}
	stats, ok := monster["stats"].(map[string]any)
	return stats, ok
}

func itemName(p *Processor, itemID int) string {
	if itemID == 0 || p == nil {
		return ""
	}
	d := p.getData()
	if d == nil {
		return ""
	}
	raw, ok := d.Items[fmt.Sprintf("%d", itemID)]
	if !ok {
		return ""
	}
	if m, ok := raw.(map[string]any); ok {
		if name, ok := m["name"].(string); ok {
			return name
		}
	}
	return ""
}
