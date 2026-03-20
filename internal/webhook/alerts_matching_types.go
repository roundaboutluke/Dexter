package webhook

import (
	"encoding/json"
	"strconv"
	"strings"
)

func matchRaid(hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["pokemon_id"])
	tracked := getInt(row["pokemon_id"])
	if tracked != 9000 && tracked != pokemonID {
		return false
	}
	level := getInt(hook.Message["level"])
	if level == 0 {
		level = getInt(hook.Message["raid_level"])
	}
	if tracked == 9000 {
		trackedLevel := getInt(row["level"])
		if trackedLevel != 90 && trackedLevel != 9000 && trackedLevel != level {
			return false
		}
	}
	team := teamFromHookMessage(hook.Message)
	if trackedTeam := getInt(row["team"]); trackedTeam != 4 && trackedTeam != team {
		return false
	}
	if exclusive := getInt(row["exclusive"]); exclusive > 0 {
		hookExclusive := getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
		if !hookExclusive {
			return false
		}
	}
	form := hookFormID(hook.Message)
	if trackedForm := getInt(row["form"]); trackedForm > 0 && form != trackedForm {
		return false
	}
	if evolution := getInt(row["evolution"]); evolution != 9000 {
		hookEvolution := getInt(hook.Message["evolution"])
		if hookEvolution == 0 {
			hookEvolution = getInt(hook.Message["evolution_id"])
		}
		if hookEvolution != evolution {
			return false
		}
	}
	if move := getInt(row["move"]); move != 9000 {
		move1 := getInt(hook.Message["move_1"])
		move2 := getInt(hook.Message["move_2"])
		if move != move1 && move != move2 {
			return false
		}
	}
	rowGymID := getString(row["gym_id"])
	if rowGymID != "" {
		hookGymID := getString(hook.Message["gym_id"])
		if hookGymID == "" {
			hookGymID = getString(hook.Message["id"])
		}
		if rowGymID != hookGymID {
			return false
		}
	}
	return true
}

func matchEgg(hook *Hook, row map[string]any) bool {
	level := getInt(hook.Message["level"])
	if trackedLevel := getInt(row["level"]); trackedLevel != 90 && trackedLevel != level {
		return false
	}
	team := teamFromHookMessage(hook.Message)
	if trackedTeam := getInt(row["team"]); trackedTeam != 4 && trackedTeam != team {
		return false
	}
	if exclusive := getInt(row["exclusive"]); exclusive > 0 {
		hookExclusive := getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
		if !hookExclusive {
			return false
		}
	}
	rowGymID := getString(row["gym_id"])
	if rowGymID != "" {
		hookGymID := getString(hook.Message["gym_id"])
		if hookGymID == "" {
			hookGymID = getString(hook.Message["id"])
		}
		if rowGymID != hookGymID {
			return false
		}
	}
	return true
}

func matchMaxBattle(hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["battle_pokemon_id"])
	if pokemonID == 0 {
		pokemonID = getInt(hook.Message["pokemon_id"])
	}
	tracked := getInt(row["pokemon_id"])
	if tracked != 9000 && tracked != pokemonID {
		return false
	}

	level := getInt(hook.Message["battle_level"])
	if level == 0 {
		level = getInt(hook.Message["level"])
	}
	if tracked == 9000 {
		trackedLevel := getInt(row["level"])
		if trackedLevel != 90 && trackedLevel != level {
			return false
		}
	}

	gmax := getInt(hook.Message["gmax"])
	if gmax == 0 && level > 6 {
		gmax = 1
	}
	if trackedGmax := getInt(row["gmax"]); trackedGmax != 0 && trackedGmax != gmax {
		return false
	}

	form := getInt(hook.Message["battle_pokemon_form"])
	if form == 0 {
		form = getInt(hook.Message["form"])
	}
	if trackedForm := getInt(row["form"]); trackedForm != 0 && trackedForm != form {
		return false
	}

	if evolution := getInt(row["evolution"]); evolution != 9000 {
		hookEvolution := getInt(hook.Message["evolution"])
		if hookEvolution == 0 {
			hookEvolution = getInt(hook.Message["evolution_id"])
		}
		if hookEvolution != evolution {
			return false
		}
	}

	if move := getInt(row["move"]); move != 9000 {
		move1 := getInt(hook.Message["battle_pokemon_move_1"])
		if move1 == 0 {
			move1 = getInt(hook.Message["move_1"])
		}
		move2 := getInt(hook.Message["battle_pokemon_move_2"])
		if move2 == 0 {
			move2 = getInt(hook.Message["move_2"])
		}
		if move != move1 && move != move2 {
			return false
		}
	}

	return true
}

func matchQuest(hook *Hook, row map[string]any) bool {
	rewardType := getInt(hook.Message["reward_type"])
	if rewardType == 0 {
		rewardType = getInt(hook.Message["reward_type_id"])
	}
	if tracked := getInt(row["reward_type"]); tracked > 0 && tracked != rewardType {
		return false
	}
	reward := getInt(hook.Message["reward"])
	if reward == 0 {
		reward = getInt(hook.Message["pokemon_id"])
	}
	if tracked := getInt(row["reward"]); tracked > 0 && tracked != reward {
		return false
	}
	if amount := getInt(row["amount"]); amount > 0 && getInt(hook.Message["amount"]) != amount {
		return false
	}
	if form := getInt(row["form"]); form > 0 && getInt(hook.Message["form"]) != form {
		return false
	}
	return true
}

func matchQuestWithData(hook *Hook, row map[string]any, rewardData map[string]any) bool {
	if hook == nil {
		return false
	}
	if rewardData == nil {
		return matchQuest(hook, row)
	}
	rewardType := getInt(row["reward_type"])
	reward := getInt(row["reward"])
	form := getInt(row["form"])
	amount := getInt(row["amount"])
	shiny := getInt(row["shiny"])

	switch rewardType {
	case 2:
		items, _ := rewardData["items"].([]map[string]any)
		for _, item := range items {
			itemID := getInt(item["id"])
			itemAmount := getInt(item["amount"])
			if reward > 0 && reward != itemID {
				continue
			}
			if amount > 0 && itemAmount < amount {
				continue
			}
			return true
		}
	case 3:
		dustAmount := getInt(rewardData["dustAmount"])
		if dustAmount == 0 {
			return false
		}
		if reward > 0 && reward > dustAmount {
			return false
		}
		return true
	case 4:
		candy, _ := rewardData["candy"].([]map[string]any)
		for _, entry := range candy {
			monID := getInt(entry["pokemonId"])
			entryAmount := getInt(entry["amount"])
			if reward > 0 && reward != monID {
				continue
			}
			if amount > 0 && entryAmount < amount {
				continue
			}
			return true
		}
	case 7:
		monsters, _ := rewardData["monsters"].([]map[string]any)
		for _, monster := range monsters {
			monID := getInt(monster["pokemonId"])
			formID := getInt(monster["formId"])
			isShiny := getBool(monster["shiny"])
			if reward > 0 && reward != monID {
				continue
			}
			if form > 0 && form != formID {
				continue
			}
			if shiny == 1 && !isShiny {
				continue
			}
			return true
		}
	case 12:
		energy, _ := rewardData["energyMonsters"].([]map[string]any)
		for _, entry := range energy {
			monID := getInt(entry["pokemonId"])
			entryAmount := getInt(entry["amount"])
			if reward > 0 && reward != monID {
				continue
			}
			if amount > 0 && entryAmount < amount {
				continue
			}
			return true
		}
	default:
		// PoracleJS only matches quests via rewardData-derived filters. Unsupported reward types
		// (anything other than 2/3/4/7/12) should not match any trackers.
		return false
	}
	return false
}

func matchInvasion(hook *Hook, row map[string]any) bool {
	tracked := strings.ToLower(getString(row["grunt_type"]))
	if tracked != "" && tracked != "all" && tracked != "everything" {
		if strings.ToLower(getString(hook.Message["grunt_type"])) != tracked {
			return false
		}
	}
	if gender := getInt(row["gender"]); gender > 0 && getInt(hook.Message["gender"]) != gender {
		return false
	}
	return true
}

func matchInvasionWithData(p *Processor, hook *Hook, row map[string]any) bool {
	tracked := strings.ToLower(getString(row["grunt_type"]))
	if tracked != "" && tracked != "all" && tracked != "everything" {
		if id, err := strconv.Atoi(tracked); err == nil && id > 0 {
			displayTypeID, gruntTypeID := resolveInvasionTypes(hook)
			rawGruntType := invasionRawGruntType(hook)
			if gruntTypeID == 0 {
				gruntTypeID = rawGruntType
			}
			if id != gruntTypeID && !(rawGruntType == 0 && displayTypeID >= 7 && id == displayTypeID) {
				return false
			}
		} else if invasionTrackType(p, hook) != tracked {
			return false
		}
	}
	if gender := getInt(row["gender"]); gender > 0 {
		if invasionGender(p, hook) != gender {
			return false
		}
	}
	return true
}

func invasionTrackType(p *Processor, hook *Hook) string {
	displayTypeID, gruntTypeID := resolveInvasionTypes(hook)
	rawGruntType := invasionRawGruntType(hook)
	if gruntTypeID > 0 {
		if grunt := findGruntByID(p, gruntTypeID); grunt != nil {
			typeName := getString(grunt["type"])
			if typeName != "" {
				return strings.ToLower(typeName)
			}
		}
	}
	if rawGruntType == 0 && displayTypeID >= 7 {
		if name, _, _ := pokestopEventInfo(p, displayTypeID); name != "" {
			return strings.ToLower(name)
		}
	}
	if rawGruntType == 0 {
		return ""
	}
	return strings.ToLower(getString(hook.Message["grunt_type"]))
}

func invasionGender(p *Processor, hook *Hook) int {
	_, gruntTypeID := resolveInvasionTypes(hook)
	if gruntTypeID > 0 {
		if grunt := findGruntByID(p, gruntTypeID); grunt != nil {
			return getInt(grunt["gender"])
		}
	}
	return 0
}

func matchLure(hook *Hook, row map[string]any) bool {
	lureID := getInt(hook.Message["lure_id"])
	if lureID == 0 {
		lureID = getInt(hook.Message["lure_type"])
	}
	if tracked := getInt(row["lure_id"]); tracked > 0 && tracked != lureID {
		return false
	}
	return true
}

func matchNest(hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["pokemon_id"])
	if tracked := getInt(row["pokemon_id"]); tracked > 0 && tracked != pokemonID {
		return false
	}
	if form := getInt(row["form"]); form > 0 && getInt(hook.Message["form"]) != form {
		return false
	}
	average := getFloat(hook.Message["average_spawns"])
	if average == 0 {
		average = getFloat(hook.Message["pokemon_avg"])
	}
	if minSpawn := getInt(row["min_spawn_avg"]); minSpawn > 0 && average < float64(minSpawn) {
		return false
	}
	return true
}

func matchGym(hook *Hook, row map[string]any) bool {
	team := teamFromHookMessage(hook.Message)
	if tracked := getInt(row["team"]); tracked != 4 && tracked != team {
		return false
	}
	if trackedGym := getString(row["gym_id"]); trackedGym != "" {
		hookGymID := getString(hook.Message["id"])
		if hookGymID == "" {
			hookGymID = getString(hook.Message["gym_id"])
		}
		if trackedGym != hookGymID {
			return false
		}
	}
	oldTeamRaw, hasOldTeam := hook.Message["old_team_id"]
	oldTeam := getInt(oldTeamRaw)
	inBattle := gymInBattle(hook.Message)
	if hasOldTeam && oldTeam == team {
		oldSlots := getInt(hook.Message["old_slots_available"])
		newSlots := getInt(hook.Message["slots_available"])
		slotChanged := getInt(row["slot_changes"]) == 1 && oldSlots != newSlots
		battleChanged := getInt(row["battle_changes"]) == 1 && inBattle
		if !slotChanged && !battleChanged {
			return false
		}
	}
	return true
}

func matchWeather(hook *Hook, row map[string]any) bool {
	condition := weatherCondition(hook.Message)
	if tracked := getInt(row["condition"]); tracked > 0 && tracked != condition {
		return false
	}
	if cell := getString(row["cell"]); cell != "" && cell != getString(hook.Message["s2_cell_id"]) {
		return false
	}
	return true
}

func matchFort(hook *Hook, row map[string]any) bool {
	fortType := strings.ToLower(getString(row["fort_type"]))
	oldEntry := mapFromAny(hook.Message["old"])
	newEntry := mapFromAny(hook.Message["new"])

	hookType := strings.ToLower(getString(hook.Message["type"]))
	if hookType == "" {
		hookType = strings.ToLower(getStringFromAnyMap(newEntry, "type"))
	}
	if hookType == "" {
		hookType = strings.ToLower(getStringFromAnyMap(oldEntry, "type"))
	}
	if hookType == "" {
		hookType = "unknown"
	}
	if fortType != "" && fortType != "everything" {
		if hookType != fortType {
			return false
		}
	}

	changeType := getString(hook.Message["change_type"])
	editTypes := []string{}
	switch v := hook.Message["edit_types"].(type) {
	case []string:
		editTypes = append(editTypes, v...)
	case []any:
		for _, item := range v {
			if entry, ok := item.(string); ok {
				editTypes = append(editTypes, entry)
			}
		}
	case string:
		var decoded []string
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			editTypes = append(editTypes, decoded...)
		}
	}
	if changeType == "edit" && (getStringFromAnyMap(oldEntry, "name") == "" && getStringFromAnyMap(oldEntry, "description") == "") {
		changeType = "new"
		editTypes = nil
	}
	changeTypes := make([]string, 0, len(editTypes)+1)
	for _, entry := range editTypes {
		if entry = strings.ToLower(strings.TrimSpace(entry)); entry != "" {
			changeTypes = append(changeTypes, entry)
		}
	}
	if changeType != "" {
		changeTypes = append(changeTypes, strings.ToLower(strings.TrimSpace(changeType)))
	}

	isEmpty := true
	if newEntry != nil && (getStringFromAnyMap(newEntry, "name") != "" || getStringFromAnyMap(newEntry, "description") != "") {
		isEmpty = false
	}
	if oldEntry != nil && getStringFromAnyMap(oldEntry, "name") != "" {
		isEmpty = false
	}
	if isEmpty && getInt(row["include_empty"]) == 0 {
		return false
	}

	trackedTypes := []string{}
	switch v := row["change_types"].(type) {
	case string:
		if strings.TrimSpace(v) != "" && strings.TrimSpace(v) != "[]" {
			_ = json.Unmarshal([]byte(v), &trackedTypes)
		}
	case []byte:
		if len(v) > 0 && strings.TrimSpace(string(v)) != "[]" {
			_ = json.Unmarshal(v, &trackedTypes)
		}
	}
	if len(trackedTypes) > 0 {
		match := false
		for _, tracked := range trackedTypes {
			tracked = strings.ToLower(strings.TrimSpace(tracked))
			if tracked == "" {
				continue
			}
			if containsString(changeTypes, tracked) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func normalizeRSVPList(raw any) []map[string]any {
	if raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case []map[string]any:
		return value
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if entry, ok := item.(map[string]any); ok {
				out = append(out, entry)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{value}
	default:
		return nil
	}
}

// Keep RSVP helpers minimal for parity with PoracleJS.
