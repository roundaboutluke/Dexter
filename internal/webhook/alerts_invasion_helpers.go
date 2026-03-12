package webhook

import (
	"fmt"
	"strconv"
	"strings"

	"poraclego/internal/i18n"
)

func applyInvasionData(p *Processor, hook *Hook, data map[string]any, platform string, tr *i18n.Translator) {
	data["gruntRewardsList"] = map[string]any{}
	data["gruntRewards"] = ""
	data["gruntTypeColor"] = 0xBABABA
	data["gender"] = 0
	if stopName := getString(hook.Message["pokestop_name"]); stopName != "" {
		data["name"] = stopName
		data["pokestopName"] = stopName
	} else if stopName := getString(hook.Message["name"]); stopName != "" {
		data["name"] = stopName
		data["pokestopName"] = stopName
	}
	if p != nil && p.cfg != nil {
		url := getString(hook.Message["url"])
		if url == "" {
			url = getStringFromConfig(p.cfg, "fallbacks.pokestopUrl", "")
		}
		if url != "" {
			data["url"] = url
			data["pokestopUrl"] = url
		}
	}
	displayTypeID, gruntTypeID := resolveInvasionTypes(hook)
	rawGruntType := invasionRawGruntType(hook)
	if rawGruntType > 0 && displayTypeID >= 7 {
		displayTypeID = 0
	}
	data["displayTypeId"] = displayTypeID
	data["grunt_type"] = rawGruntType
	incidentExpiration := getInt64(hook.Message["incident_expiration"])
	if incidentExpiration == 0 {
		incidentExpiration = getInt64(hook.Message["incident_expire_timestamp"])
	}
	if incidentExpiration > 0 {
		data["incidentExpiration"] = incidentExpiration
	}
	data["gruntTypeId"] = gruntTypeID
	eventInvasion := isEventInvasion(hook, displayTypeID)
	if eventInvasion {
		if name, color, emojiKey := pokestopEventInfo(p, displayTypeID); name != "" {
			data["gruntName"] = translateMaybe(tr, name)
			data["gruntType"] = strings.ToLower(name)
			if color != "" {
				if parsed, err := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 32); err == nil {
					data["gruntTypeColor"] = int(parsed)
				}
			}
			if emojiKey != "" {
				if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
					data["gruntTypeEmoji"] = translateMaybe(tr, emoji)
				}
			}
		}
		data["gender"] = 0
	} else if gruntTypeID > 0 {
		data["gender"] = 0
		data["gruntName"] = translateMaybe(tr, "Grunt")
		data["gruntType"] = translateMaybe(tr, "Mixed")
		data["gruntRewards"] = ""
		if grunt := findGruntByID(p, gruntTypeID); grunt != nil {
			typeName := getString(grunt["type"])
			if strings.EqualFold(typeName, "Metal") {
				typeName = "Steel"
			}
			if typeName != "" {
				data["gruntType"] = translateMaybe(tr, typeName)
			}
			gruntLabel := getString(grunt["grunt"])
			if typeName != "" && gruntLabel != "" {
				data["gruntName"] = translateMaybe(tr, fmt.Sprintf("%s %s", typeName, gruntLabel))
			}
			if gender := getInt(grunt["gender"]); gender > 0 {
				data["gender"] = gender
			}
			if color, emojiKey := typeStyle(p, typeName); color != 0 {
				data["gruntTypeColor"] = color
				if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
					data["gruntTypeEmoji"] = translateMaybe(tr, emoji)
				}
			}
			if genderEng := genderDataEng(p, getInt(data["gender"])); genderEng != nil {
				data["genderDataEng"] = genderEng
				if name, ok := genderEng["name"].(string); ok {
					data["genderNameEng"] = name
				}
			} else {
				data["genderDataEng"] = map[string]any{"name": "", "emoji": ""}
				data["genderNameEng"] = ""
			}
			rewardText, rewardList := gruntRewardsDetails(p, grunt, tr)
			if rewardText != "" {
				data["gruntRewards"] = rewardText
			}
			if len(rewardList) > 0 {
				data["gruntRewardsList"] = rewardList
			}
		}
	}
	if _, ok := data["gruntType"]; !ok || getString(data["gruntType"]) == "" {
		data["gruntType"] = getString(hook.Message["grunt_type"])
	}
	if _, ok := data["gruntTypeEmoji"]; !ok || getString(data["gruntTypeEmoji"]) == "" {
		data["gruntTypeEmoji"] = gruntTypeEmoji(p, data["gruntType"], platform)
	}
	if _, ok := data["gruntTypeColor"]; !ok || getInt(data["gruntTypeColor"]) == 0 {
		data["gruntTypeColor"] = gruntTypeColor(data["gruntType"])
	}
	if lineup, ok := hook.Message["lineup"].([]any); ok {
		data["gruntLineupList"] = buildGruntLineupList(p, lineup, tr)
	}
}

func invasionDisplayType(hook *Hook) int {
	displayType, _ := resolveInvasionTypes(hook)
	return displayType
}

func invasionGruntTypeID(hook *Hook, displayTypeID int) int {
	_, gruntType := resolveInvasionTypes(hook)
	return gruntType
}

func resolveInvasionTypes(hook *Hook) (int, int) {
	if hook == nil {
		return 0, 0
	}
	displayType := 0
	if raw, ok := hook.Message["display_type"]; ok {
		displayType = getInt(raw)
	} else {
		displayType = getInt(hook.Message["incident_display_type"])
	}
	incidentGruntType := getInt(hook.Message["incident_grunt_type"])
	gruntType := getInt(hook.Message["grunt_type"])
	if incidentGruntType != 0 && incidentGruntType != 352 {
		return displayType, incidentGruntType
	}
	if gruntType != 0 && displayType <= 6 {
		return displayType, gruntType
	}
	if incidentGruntType == 352 {
		return 8, 0
	}
	return displayType, 0
}

func invasionRawGruntType(hook *Hook) int {
	if hook == nil {
		return 0
	}
	if getInt(hook.Message["incident_grunt_type"]) == 352 {
		return 0
	}
	return getInt(hook.Message["grunt_type"])
}

func isEventInvasion(hook *Hook, displayTypeID int) bool {
	return invasionRawGruntType(hook) == 0 && displayTypeID >= 7
}

func pokestopEventInfo(p *Processor, displayTypeID int) (string, string, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return "", "", ""
	}
	raw, ok := p.data.UtilData["pokestopEvent"].(map[string]any)
	if !ok {
		return "", "", ""
	}
	entry, ok := raw[strconv.Itoa(displayTypeID)].(map[string]any)
	if !ok {
		return "", "", ""
	}
	return getString(entry["name"]), getString(entry["color"]), getString(entry["emoji"])
}

func typeStyle(p *Processor, typeName string) (int, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return 0, ""
	}
	types, ok := p.data.UtilData["types"].(map[string]any)
	if !ok {
		return 0, ""
	}
	entry, ok := types[typeName].(map[string]any)
	if !ok {
		return 0, ""
	}
	colorStr := getString(entry["color"])
	emojiKey := getString(entry["emoji"])
	if colorStr != "" {
		if parsed, err := strconv.ParseInt(strings.TrimPrefix(colorStr, "#"), 16, 32); err == nil {
			return int(parsed), emojiKey
		}
	}
	return 0, emojiKey
}

func lookupEmoji(p *Processor, key string) string {
	return lookupEmojiForPlatform(p, key, "")
}

func lookupEmojiForPlatform(p *Processor, key string, platform string) string {
	if p == nil || p.data == nil || p.data.UtilData == nil || key == "" {
		return ""
	}
	if platform == "" && p.customEmoji != nil && len(p.customEmoji) == 1 {
		for name := range p.customEmoji {
			platform = name
			break
		}
	}
	if platform == "" && p.customEmoji != nil {
		if _, ok := p.customEmoji["discord"]; ok {
			platform = "discord"
		}
	}
	if platform != "" && p.customEmoji != nil {
		if platformMap, ok := p.customEmoji[platform]; ok {
			if val, ok := platformMap[key]; ok {
				return val
			}
		}
	}
	raw, ok := p.data.UtilData["emojis"].(map[string]any)
	if !ok {
		return ""
	}
	if value, ok := raw[key]; ok {
		return fmt.Sprintf("%v", value)
	}
	return ""
}

func genderDataEng(p *Processor, gender int) map[string]any {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return nil
	}
	raw, ok := p.data.UtilData["genders"].(map[string]any)
	if !ok {
		return nil
	}
	entry, ok := raw[fmt.Sprintf("%d", gender)].(map[string]any)
	if !ok {
		return nil
	}
	name, _ := entry["name"].(string)
	emoji, _ := entry["emoji"].(string)
	return map[string]any{"name": name, "emoji": emoji}
}

func gruntTypeColor(gruntType any) int {
	switch strings.ToLower(fmt.Sprintf("%v", gruntType)) {
	case "dragon":
		return 0x7038F8
	case "fire":
		return 0xF08030
	case "water":
		return 0x6890F0
	default:
		return 0x808080
	}
}

func gruntRewardsList(p *Processor, gruntType any, tr *i18n.Translator) map[string]any {
	out := map[string]any{}
	if p == nil || p.data == nil {
		return out
	}
	grunt := findGrunt(p, gruntType)
	if grunt == nil {
		return out
	}
	encounters, ok := grunt["encounters"].(map[string]any)
	if !ok {
		return out
	}
	first := rewardListFromEncounters(p, encounters["first"], tr)
	second := rewardListFromEncounters(p, encounters["second"], tr)
	if len(first) > 0 {
		out["first"] = map[string]any{
			"chance":   85,
			"monsters": first,
		}
	}
	if len(second) > 0 {
		out["second"] = map[string]any{
			"chance":   15,
			"monsters": second,
		}
	}
	return out
}

func findGrunt(p *Processor, gruntType any) map[string]any {
	if p == nil || p.data == nil {
		return nil
	}
	needle := strings.ToLower(fmt.Sprintf("%v", gruntType))
	for _, raw := range p.data.Grunts {
		if m, ok := raw.(map[string]any); ok {
			if typ, ok := m["type"].(string); ok && strings.ToLower(typ) == needle {
				return m
			}
		}
	}
	return nil
}

func findGruntByID(p *Processor, gruntTypeID int) map[string]any {
	if p == nil || p.data == nil || gruntTypeID <= 0 {
		return nil
	}
	raw, ok := p.data.Grunts[strconv.Itoa(gruntTypeID)]
	if !ok {
		return nil
	}
	grunt, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return grunt
}

func gruntRewardsDetails(p *Processor, grunt map[string]any, tr *i18n.Translator) (string, map[string]any) {
	out := map[string]any{}
	if p == nil || p.data == nil || grunt == nil {
		return "", out
	}
	encounters, ok := grunt["encounters"].(map[string]any)
	if !ok {
		return "", out
	}
	secondReward := getBool(grunt["secondReward"])
	thirdReward := getBool(grunt["thirdReward"])
	firstList := rewardListFromEncountersDetailed(p, encounters[firstEncounterKey(thirdReward)], tr)
	secondList := rewardListFromEncountersDetailed(p, encounters["second"], tr)
	rewardText := ""
	if secondReward && len(firstList) > 0 && len(secondList) > 0 {
		out["first"] = map[string]any{"chance": 85, "monsters": firstList}
		out["second"] = map[string]any{"chance": 15, "monsters": secondList}
		rewardText = fmt.Sprintf("85%%: %s\\n15%%: %s", rewardNames(firstList), rewardNames(secondList))
		return rewardText, out
	}
	if len(firstList) > 0 {
		out["first"] = map[string]any{"chance": 100, "monsters": firstList}
		rewardText = rewardNames(firstList)
	}
	return rewardText, out
}

func firstEncounterKey(thirdReward bool) string {
	if thirdReward {
		return "third"
	}
	return "first"
}

func rewardNames(items []map[string]any) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if name, ok := item["fullName"].(string); ok && name != "" {
			names = append(names, name)
		} else if name, ok := item["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func rewardListFromEncountersDetailed(p *Processor, raw any, tr *i18n.Translator) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := getInt(m["id"])
		formID := getInt(m["form"])
		nameEng, formNameEng := monsterInfo(p, id, formID)
		if nameEng == "" {
			nameEng = fmt.Sprintf("Pokemon %d", id)
		}
		name := translateMaybe(tr, nameEng)
		formName := translateMaybe(tr, formNameEng)
		fullName := name
		// Match PoracleJS: the "Normal" check is done on the English form name before translation.
		if formNameEng != "" && !strings.EqualFold(formNameEng, "Normal") {
			fullName = fmt.Sprintf("%s %s", formName, name)
		}
		out = append(out, map[string]any{
			"id":       id,
			"formId":   formID,
			"name":     name,
			"formName": formName,
			"fullName": fullName,
		})
	}
	return out
}

func buildGruntLineupList(p *Processor, lineup []any, tr *i18n.Translator) map[string]any {
	out := map[string]any{
		"confirmed": true,
		"monsters":  []map[string]any{},
	}
	monsters := make([]map[string]any, 0, len(lineup))
	for _, item := range lineup {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := getInt(m["pokemon_id"])
		formID := getInt(m["form"])
		nameEng, formNameEng := monsterInfo(p, id, formID)
		if nameEng == "" {
			nameEng = fmt.Sprintf("Pokemon %d", id)
		}
		name := translateMaybe(tr, nameEng)
		formName := translateMaybe(tr, formNameEng)
		fullName := name
		// Match PoracleJS: the "Normal" check is done on the English form name before translation.
		if formNameEng != "" && !strings.EqualFold(formNameEng, "Normal") {
			fullName = fmt.Sprintf("%s %s", formName, name)
		}
		monsters = append(monsters, map[string]any{
			"id":       id,
			"formId":   formID,
			"name":     name,
			"formName": formName,
			"fullName": fullName,
		})
	}
	out["monsters"] = monsters
	return out
}

func rewardListFromEncounters(p *Processor, raw any, tr *i18n.Translator) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := getInt(m["id"])
		out = append(out, map[string]any{
			"name": translateMaybe(tr, monsterName(p, id)),
		})
	}
	return out
}

func genderData(p *Processor, gender int, platform string, tr *i18n.Translator) map[string]any {
	// Match PoracleJS: gender labels + emoji keys come from utilData.genders and pass through emoji lookup.
	entry := genderDataEng(p, gender)
	if entry == nil {
		return map[string]any{"name": translateMaybe(tr, "Unknown"), "emoji": ""}
	}
	nameEng := getString(entry["name"])
	emojiKey := getString(entry["emoji"])
	emoji := ""
	if emojiKey != "" {
		emoji = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, platform))
	}
	return map[string]any{
		"name":  translateMaybe(tr, nameEng),
		"emoji": emoji,
	}
}

func lureTypeInfo(lureID int) (string, int) {
	switch lureID {
	case 501:
		return "Glacial", 0x00FFFF
	case 502:
		return "Mossy", 0x00FF7F
	case 503:
		return "Magnetic", 0xAAAAAA
	case 504:
		return "Rainy", 0x1E90FF
	case 505:
		return "Sparkly", 0xFF69B4
	default:
		return "Normal", 0x00FF00
	}
}

func lureTypeDetails(p *Processor, lureID int) (string, string, int) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return "", "", 0
	}
	raw, ok := p.data.UtilData["lures"].(map[string]any)
	if !ok {
		return "", "", 0
	}
	entry, ok := raw[strconv.Itoa(lureID)].(map[string]any)
	if !ok {
		return "", "", 0
	}
	name := getString(entry["name"])
	emojiKey := getString(entry["emoji"])
	color := getString(entry["color"])
	if color != "" {
		if parsed, err := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 32); err == nil {
			return name, emojiKey, int(parsed)
		}
	}
	return name, emojiKey, 0
}

func monsterTypes(p *Processor, pokemonID, formID int) []int {
	if p == nil || p.data == nil {
		return nil
	}
	monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return nil
	}
	raw, ok := monster["types"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]int, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, getInt(m["id"]))
		}
	}
	return out
}

func monsterTypeNames(p *Processor, pokemonID, formID int) []string {
	if p == nil || p.data == nil {
		return nil
	}
	monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return nil
	}
	raw, ok := monster["types"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			if name := getString(m["name"]); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}
