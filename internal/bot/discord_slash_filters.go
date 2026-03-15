package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"strconv"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/db"
	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
	"poraclego/internal/uicons"
)

const (
	slashFilterRemoveButtonPrefix  = "poracle:filter:remove:"
	slashFilterRestoreButtonPrefix = "poracle:filter:restore:"

	slashFilterCardColorConfirm  = 0x57F287
	slashFilterCardColorAdded    = 0x57F287
	slashFilterCardColorRemoved  = 0xED4245
	slashFilterCardColorRestored = 0x5865F2
)

func (d *Discord) slashUiconsClient() *uicons.Client {
	if d == nil || d.manager == nil || d.manager.cfg == nil {
		return nil
	}
	imgURL, _ := d.manager.cfg.GetString("general.imgUrl")
	if imgURL == "" {
		return nil
	}
	return uicons.CachedClient(imgURL, "png")
}

func (d *Discord) slashFilterIconURL(trackingType string, rows []map[string]any) string {
	if len(rows) == 0 {
		return ""
	}
	client := d.slashUiconsClient()
	if client == nil {
		return ""
	}
	row := rows[0]
	switch trackingType {
	case "pokemon":
		pokemonID := toInt(row["pokemon_id"], 0)
		if pokemonID == 0 {
			return ""
		}
		form := toInt(row["form"], 0)
		evolution := toInt(row["evolution"], 0)
		url, _ := client.PokemonIcon(pokemonID, form, evolution, 0, 0, 0, false, 0)
		return url
	case "raid", "maxbattle":
		pokemonID := toInt(row["pokemon_id"], 9000)
		if pokemonID == 9000 || pokemonID == 0 {
			level := toInt(row["level"], 0)
			if level > 0 {
				url, _ := client.RaidEggIcon(level, false, false)
				return url
			}
			return ""
		}
		form := toInt(row["form"], 0)
		evolution := toInt(row["evolution"], 0)
		url, _ := client.PokemonIcon(pokemonID, form, evolution, 0, 0, 0, false, 0)
		return url
	case "egg":
		level := toInt(row["level"], 0)
		if level > 0 {
			url, _ := client.RaidEggIcon(level, false, false)
			return url
		}
		return ""
	case "quest":
		rewardType := toInt(row["reward_type"], 0)
		reward := toInt(row["reward"], 0)
		switch rewardType {
		case 7:
			if reward == 0 {
				return ""
			}
			form := toInt(row["form"], 0)
			url, _ := client.PokemonIcon(reward, form, 0, 0, 0, 0, false, 0)
			return url
		case 2:
			if reward > 0 {
				url, _ := client.RewardItemIcon(reward)
				return url
			}
		case 3:
			url, _ := client.RewardStardustIcon(reward)
			return url
		case 12:
			url, _ := client.RewardMegaEnergyIcon(reward, 0)
			return url
		case 4:
			url, _ := client.RewardCandyIcon(reward, 0)
			return url
		case 9:
			url, _ := client.RewardXLCandyIcon(reward, 0)
			return url
		}
		return ""
	case "rocket":
		gruntType := strings.TrimSpace(fmt.Sprintf("%v", row["grunt_type"]))
		if id, err := strconv.Atoi(gruntType); err == nil && id > 0 {
			url, _ := client.InvasionIcon(id)
			return url
		}
		return ""
	case "pokestop-event":
		gruntType := strings.TrimSpace(fmt.Sprintf("%v", row["grunt_type"]))
		switch strings.ToLower(gruntType) {
		case "gold-stop":
			url, _ := client.PokestopIcon(0, false, 7, false)
			return url
		case "kecleon":
			url, _ := client.PokemonIcon(352, 0, 0, 0, 0, 0, false, 0)
			return url
		case "showcase":
			url, _ := client.PokestopIcon(0, false, 9, false)
			return url
		}
		return ""
	case "lure":
		lureID := toInt(row["lure_id"], 0)
		if lureID > 0 {
			url, _ := client.RewardItemIcon(lureID)
			return url
		}
		return ""
	case "weather":
		condition := toInt(row["condition"], 0)
		if condition > 0 {
			url, _ := client.WeatherIcon(condition)
			return url
		}
		return ""
	case "gym":
		team := toInt(row["team"], 0)
		url, _ := client.GymIcon(team, 0, false, false)
		return url
	case "nest":
		pokemonID := toInt(row["pokemon_id"], 0)
		if pokemonID == 0 {
			return ""
		}
		form := toInt(row["form"], 0)
		url, _ := client.PokemonIcon(pokemonID, form, 0, 0, 0, 0, false, 0)
		return url
	case "fort":
		return ""
	default:
		return ""
	}
}

func (d *Discord) slashFilterTypedHeading(tr *i18n.Translator, trackingType string, rows []map[string]any) string {
	typeLabel := slashTrackingTitle(tr, trackingType)
	if len(rows) == 0 {
		return typeLabel
	}
	if len(rows) > 1 {
		return fmt.Sprintf("%d %s", len(rows), strings.ToLower(translateOrDefault(tr, "Filters")))
	}
	row := rows[0]
	name := ""
	switch trackingType {
	case "pokemon":
		name = d.slashMonsterName(tr, row)
	case "raid", "maxbattle":
		pokemonID := toInt(row["pokemon_id"], 9000)
		if pokemonID == 9000 || pokemonID == 0 {
			level := toInt(row["level"], 0)
			if level == 90 {
				name = translateOrDefault(tr, "All levels")
			} else if level > 0 {
				name = fmt.Sprintf("%s %d", translateOrDefault(tr, "Level"), level)
			}
		} else {
			name = d.slashMonsterName(tr, row)
		}
	case "egg":
		level := toInt(row["level"], 0)
		if level == 90 {
			name = translateOrDefault(tr, "All levels")
		} else if level > 0 {
			name = fmt.Sprintf("%s %d", translateOrDefault(tr, "Level"), level)
		}
	case "quest":
		name = d.slashQuestRewardName(tr, row)
	case "rocket", "pokestop-event":
		grunt := strings.TrimSpace(fmt.Sprintf("%v", row["grunt_type"]))
		if grunt != "" && !strings.EqualFold(grunt, "everything") {
			name = d.invasionTypeLabel(grunt, tr)
		} else {
			name = translateOrDefault(tr, "Everything")
		}
	case "lure":
		lureID := toInt(row["lure_id"], 0)
		if lureID == 0 {
			name = translateOrDefault(tr, "Everything")
		} else if d.manager != nil && d.manager.data != nil {
			name = tracking.LureRowText(d.manager.cfg, tr, d.manager.data, row)
		}
	case "weather":
		condition := toInt(row["condition"], 0)
		if condition == 0 {
			name = translateOrDefault(tr, "Everything")
		} else if d.manager != nil && d.manager.data != nil {
			name = tracking.WeatherRowText(tr, d.manager.data, row)
		}
	case "gym":
		name = d.slashFilterRowText(tr, trackingType, row)
	case "nest":
		name = d.slashMonsterName(tr, row)
	default:
		name = d.slashFilterRowText(tr, trackingType, row)
	}
	if name == "" {
		return typeLabel
	}
	return fmt.Sprintf("%s: %s", typeLabel, name)
}

func (d *Discord) slashMonsterName(tr *i18n.Translator, row map[string]any) string {
	pokemonID := toInt(row["pokemon_id"], 0)
	if pokemonID == 0 {
		return translateOrDefault(tr, "Everything")
	}
	if d.manager == nil || d.manager.data == nil {
		return fmt.Sprintf("#%d", pokemonID)
	}
	formID := toInt(row["form"], 0)
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if toInt(mon["id"], -1) != pokemonID {
			continue
		}
		form := toMapAny(mon["form"])
		if toInt(form["id"], -1) == formID {
			name := translateOrDefault(tr, fmt.Sprintf("%v", mon["name"]))
			formName := fmt.Sprintf("%v", form["name"])
			if formName != "" && formName != "Normal" && toInt(form["id"], 0) != 0 {
				name = name + " " + translateOrDefault(tr, formName)
			}
			return name
		}
	}
	return fmt.Sprintf("#%d", pokemonID)
}

func (d *Discord) slashQuestRewardName(tr *i18n.Translator, row map[string]any) string {
	rewardType := toInt(row["reward_type"], 0)
	reward := toInt(row["reward"], 0)
	form := toInt(row["form"], 0)
	switch rewardType {
	case 7:
		fakeRow := map[string]any{"pokemon_id": reward, "form": form}
		return d.slashMonsterName(tr, fakeRow)
	case 2:
		if d.manager != nil && d.manager.data != nil {
			if item := d.manager.data.Items[fmt.Sprintf("%d", reward)]; item != nil {
				if m, ok := item.(map[string]any); ok {
					if name := fmt.Sprintf("%v", m["name"]); name != "" {
						return translateOrDefault(tr, name)
					}
				}
			}
		}
		return translateOrDefault(tr, "Item")
	case 3:
		return translateOrDefault(tr, "Stardust")
	case 12:
		name := translateOrDefault(tr, "Mega Energy")
		if reward > 0 {
			fakeRow := map[string]any{"pokemon_id": reward, "form": 0}
			monName := d.slashMonsterName(tr, fakeRow)
			if monName != "" {
				return name + " " + monName
			}
		}
		return name
	case 4:
		if reward == 0 {
			return translateOrDefault(tr, "Rare Candy")
		}
		fakeRow := map[string]any{"pokemon_id": reward, "form": 0}
		return d.slashMonsterName(tr, fakeRow) + " Candy"
	case 9:
		if reward == 0 {
			return translateOrDefault(tr, "Rare Candy XL")
		}
		fakeRow := map[string]any{"pokemon_id": reward, "form": 0}
		return d.slashMonsterName(tr, fakeRow) + " XL Candy"
	case 1:
		return translateOrDefault(tr, "Experience")
	default:
		return translateOrDefault(tr, "Reward")
	}
}

func toMapAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func slashFilterNonDefaultDetailLines(tr *i18n.Translator, trackingType string, row map[string]any) []string {
	lines := []string{}
	switch trackingType {
	case "pokemon":
		addIfNotDefault := func(label string, keys ...string) {
			for _, key := range keys {
				val := toInt(row[key], 0)
				def := monsterDefaultValue(key)
				if val != def {
					lines = append(lines, slashCardDetailLine(label, fmt.Sprintf("%d", val)))
					return
				}
			}
		}
		minIV := toInt(row["min_iv"], 0)
		maxIV := toInt(row["max_iv"], 100)
		if minIV != 0 || maxIV != 100 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "IV"), fmt.Sprintf("%d%% - %d%%", minIV, maxIV)))
		}
		minCP := toInt(row["min_cp"], 0)
		maxCP := toInt(row["max_cp"], 9000)
		if minCP != 0 || maxCP != 9000 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "CP"), fmt.Sprintf("%d - %d", minCP, maxCP)))
		}
		minLvl := toInt(row["min_level"], 0)
		maxLvl := toInt(row["max_level"], 40)
		if minLvl != 0 || maxLvl != 40 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Level"), fmt.Sprintf("%d - %d", minLvl, maxLvl)))
		}
		atk := toInt(row["atk"], 0)
		def_ := toInt(row["def"], 0)
		sta := toInt(row["sta"], 0)
		maxAtk := toInt(row["max_atk"], 15)
		maxDef := toInt(row["max_def"], 15)
		maxSta := toInt(row["max_sta"], 15)
		if atk != 0 || def_ != 0 || sta != 0 || maxAtk != 15 || maxDef != 15 || maxSta != 15 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Stats"), fmt.Sprintf("%d/%d/%d - %d/%d/%d", atk, def_, sta, maxAtk, maxDef, maxSta)))
		}
		pvpLeague := toInt(row["pvp_ranking_league"], 0)
		if pvpLeague != 0 {
			pvpWorst := toInt(row["pvp_ranking_worst"], 0)
			lines = append(lines, slashCardDetailLine("PVP", fmt.Sprintf("%dCP top %d", pvpLeague, pvpWorst)))
		}
		gender := toInt(row["gender"], 0)
		if gender == 1 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Gender"), "♂"))
		} else if gender == 2 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Gender"), "♀"))
		}
		size := toInt(row["size"], 0)
		maxSize := toInt(row["max_size"], 5)
		if size != 0 || maxSize != 5 {
			sizeLabels := map[int]string{1: "XXS", 2: "XS", 3: "M", 4: "XL", 5: "XXL"}
			sizeMin := sizeLabels[size]
			sizeMax := sizeLabels[maxSize]
			if sizeMin == "" {
				sizeMin = fmt.Sprintf("%d", size)
			}
			if sizeMax == "" {
				sizeMax = fmt.Sprintf("%d", maxSize)
			}
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Size"), fmt.Sprintf("%s - %s", sizeMin, sizeMax)))
		}
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
		minTime := toInt(row["min_time"], 0)
		if minTime > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Min time"), fmt.Sprintf("%ds", minTime)))
		}
		_ = addIfNotDefault // suppress unused warning for this helper pattern
	case "raid", "egg", "maxbattle":
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
		team := toInt(row["team"], 4)
		if team != 4 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Team"), fmt.Sprintf("%d", team)))
		}
		exclusive := toInt(row["exclusive"], 0)
		if exclusive != 0 {
			lines = append(lines, slashCardDetailLine("EX", translateOrDefault(tr, "Yes")))
		}
	case "quest":
		amount := toInt(row["amount"], 0)
		if amount > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Amount"), fmt.Sprintf("%d+", amount)))
		}
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
	case "rocket", "pokestop-event":
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
		gender := toInt(row["gender"], 0)
		if gender == 1 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Gender"), "♂"))
		} else if gender == 2 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Gender"), "♀"))
		}
	case "lure":
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
	case "weather":
		// Weather tracking has minimal params
	case "gym":
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
	case "nest":
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
	}
	clean := toInt(row["clean"], 0)
	if clean != 0 {
		lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Clean"), translateOrDefault(tr, "Yes")))
	}
	return lines
}

func monsterDefaultValue(key string) int {
	switch key {
	case "min_iv":
		return 0
	case "max_iv":
		return 100
	case "min_cp":
		return 0
	case "max_cp":
		return 9000
	case "min_level":
		return 0
	case "max_level":
		return 40
	case "atk", "def", "sta":
		return 0
	case "max_atk", "max_def", "max_sta":
		return 15
	case "gender", "size", "rarity", "distance", "min_time":
		return 0
	case "max_size":
		return 5
	case "max_rarity":
		return 6
	default:
		return 0
	}
}

type slashFilterActionState struct {
	UserID       string
	Table        string
	TrackingType string
	Rows         []map[string]any
	ProfileLabel string
	ExpiresAt    time.Time
}

func cloneSlashFilterRows(rows []map[string]any) []map[string]any {
	cloned := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		copyRow := make(map[string]any, len(row))
		for key, value := range row {
			copyRow[key] = value
		}
		cloned = append(cloned, copyRow)
	}
	return cloned
}

func (d *Discord) nextSlashFilterActionID() string {
	d.filterMu.Lock()
	defer d.filterMu.Unlock()
	d.filterSeq++
	return fmt.Sprintf("%d", d.filterSeq)
}

func (d *Discord) storeSlashFilterAction(state *slashFilterActionState) string {
	if d == nil || state == nil {
		return ""
	}
	id := d.nextSlashFilterActionID()
	d.filterMu.Lock()
	defer d.filterMu.Unlock()
	if d.filterActions == nil {
		d.filterActions = map[string]*slashFilterActionState{}
	}
	now := time.Now()
	for key, action := range d.filterActions {
		if action == nil || action.ExpiresAt.Before(now) {
			delete(d.filterActions, key)
		}
	}
	d.filterActions[id] = &slashFilterActionState{
		UserID:       state.UserID,
		Table:        state.Table,
		TrackingType: state.TrackingType,
		Rows:         cloneSlashFilterRows(state.Rows),
		ProfileLabel: state.ProfileLabel,
		ExpiresAt:    state.ExpiresAt,
	}
	return id
}

func (d *Discord) slashFilterAction(id, userID string) (*slashFilterActionState, string) {
	if d == nil || id == "" {
		return nil, "That filter action is no longer available."
	}
	d.filterMu.Lock()
	defer d.filterMu.Unlock()
	state := d.filterActions[id]
	if state == nil {
		return nil, "That filter action is no longer available."
	}
	if state.ExpiresAt.Before(time.Now()) {
		delete(d.filterActions, id)
		return nil, "That filter action has expired."
	}
	if state.UserID != "" && userID != "" && state.UserID != userID {
		return nil, "That filter action belongs to another user."
	}
	return &slashFilterActionState{
		UserID:       state.UserID,
		Table:        state.Table,
		TrackingType: state.TrackingType,
		Rows:         cloneSlashFilterRows(state.Rows),
		ProfileLabel: state.ProfileLabel,
		ExpiresAt:    state.ExpiresAt,
	}, ""
}

func slashTrackingTypeFromCommand(command string) string {
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "track", "pokemon":
		return "pokemon"
	case "raid":
		return "raid"
	case "egg":
		return "egg"
	case "maxbattle":
		return "maxbattle"
	case "quest":
		return "quest"
	case "invasion", "rocket":
		return "rocket"
	case "incident", "pokestop-event":
		return "pokestop-event"
	case "gym":
		return "gym"
	case "weather":
		return "weather"
	case "lure":
		return "lure"
	case "nest":
		return "nest"
	case "fort":
		return "fort"
	default:
		return ""
	}
}

func slashTrackingTitle(tr *i18n.Translator, trackingType string) string {
	switch trackingType {
	case "pokemon":
		return translateOrDefault(tr, "Pokemon")
	case "rocket":
		return translateOrDefault(tr, "Rocket")
	case "pokestop-event":
		return translateOrDefault(tr, "Pokestop Event")
	default:
		return humanizeOptionName(trackingType)
	}
}

func (d *Discord) slashTrackingRowsForSelection(selection slashProfileSelection, trackingType string) ([]map[string]any, string) {
	if d == nil || d.manager == nil || d.manager.query == nil {
		return nil, ""
	}
	table := removeTrackingTable(trackingType)
	if table == "" {
		return nil, ""
	}
	where := map[string]any{"id": selection.UserID}
	if selection.Mode != slashProfileScopeAll && selection.ProfileNo > 0 {
		where["profile_no"] = selection.ProfileNo
	}
	rows, err := d.manager.query.SelectAllQuery(table, where)
	if err != nil {
		return nil, table
	}
	return d.filterRowsByTrackingType(rows, trackingType), table
}

func (d *Discord) filterRowsByTrackingType(rows []map[string]any, trackingType string) []map[string]any {
	trackingType = strings.ToLower(strings.TrimSpace(trackingType))
	if trackingType != "rocket" && trackingType != "pokestop-event" {
		return cloneSlashFilterRows(rows)
	}
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		gruntType := strings.TrimSpace(fmt.Sprintf("%v", row["grunt_type"]))
		isEvent := d.isPokestopEventType(gruntType)
		if trackingType == "rocket" && isEvent {
			continue
		}
		if trackingType == "pokestop-event" && !isEvent {
			continue
		}
		filtered = append(filtered, row)
	}
	return cloneSlashFilterRows(filtered)
}

func slashRowUID(row map[string]any) string {
	return strings.TrimSpace(fmt.Sprintf("%v", row["uid"]))
}

func slashRowFingerprint(row map[string]any) string {
	if row == nil {
		return ""
	}
	payload, _ := json.Marshal(row)
	return string(payload)
}

func slashChangedRows(beforeRows, afterRows []map[string]any) []map[string]any {
	beforeByUID := make(map[string]string, len(beforeRows))
	for _, row := range beforeRows {
		uid := slashRowUID(row)
		if uid == "" {
			continue
		}
		beforeByUID[uid] = slashRowFingerprint(row)
	}
	changed := []map[string]any{}
	for _, row := range afterRows {
		uid := slashRowUID(row)
		if uid == "" {
			continue
		}
		if beforeByUID[uid] != slashRowFingerprint(row) {
			changed = append(changed, row)
		}
	}
	return cloneSlashFilterRows(changed)
}

func slashFilterRowsByUID(rows []map[string]any, uid string) []map[string]any {
	if uid == "" {
		return nil
	}
	filtered := []map[string]any{}
	for _, row := range rows {
		if slashRowUID(row) == uid {
			filtered = append(filtered, row)
		}
	}
	return cloneSlashFilterRows(filtered)
}

func slashUIDValues(rows []map[string]any) []any {
	values := make([]any, 0, len(rows))
	seen := map[string]bool{}
	for _, row := range rows {
		uid := slashRowUID(row)
		if uid == "" || seen[uid] {
			continue
		}
		seen[uid] = true
		values = append(values, parseUID(uid))
	}
	return values
}

func (d *Discord) deleteSlashTrackingRows(table string, rows []map[string]any) (int64, error) {
	if d == nil || d.manager == nil || d.manager.query == nil || table == "" || len(rows) == 0 {
		return 0, nil
	}
	scope := map[string]any{}
	if id := strings.TrimSpace(fmt.Sprintf("%v", rows[0]["id"])); id != "" {
		scope["id"] = id
	}
	profiles := map[int]bool{}
	profileOrder := []int{}
	for _, row := range rows {
		profileNo := toInt(row["profile_no"], 0)
		if profiles[profileNo] {
			continue
		}
		profiles[profileNo] = true
		profileOrder = append(profileOrder, profileNo)
	}
	uidsByProfile := map[int][]any{}
	for _, row := range rows {
		profileNo := toInt(row["profile_no"], 0)
		uidsByProfile[profileNo] = append(uidsByProfile[profileNo], parseUID(slashRowUID(row)))
	}
	var removed int64
	err := d.manager.query.WithTx(context.Background(), func(tx *db.Query) error {
		for _, profileNo := range profileOrder {
			deleteScope := map[string]any{}
			for key, value := range scope {
				deleteScope[key] = value
			}
			if profileNo > 0 {
				deleteScope["profile_no"] = profileNo
			}
			affected, err := tx.DeleteWhereInQuery(table, deleteScope, uidsByProfile[profileNo], "uid")
			if err != nil {
				return err
			}
			removed += affected
		}
		tx.AfterCommit(func() {
			if d.manager != nil && d.manager.processor != nil {
				d.manager.processor.RefreshAlertCacheAsync()
			}
		})
		return nil
	})
	return removed, err
}

func (d *Discord) restoreSlashTrackingRows(table string, rows []map[string]any) (int64, error) {
	if d == nil || d.manager == nil || d.manager.query == nil || table == "" || len(rows) == 0 {
		return 0, nil
	}
	var inserted int64
	err := d.manager.query.WithTx(context.Background(), func(tx *db.Query) error {
		affected, err := tx.InsertQuery(table, cloneSlashFilterRows(rows))
		if err != nil {
			return err
		}
		inserted = affected
		tx.AfterCommit(func() {
			if d.manager != nil && d.manager.processor != nil {
				d.manager.processor.RefreshAlertCacheAsync()
			}
		})
		return nil
	})
	return inserted, err
}

func (d *Discord) slashFilterRowText(tr *i18n.Translator, trackingType string, row map[string]any) string {
	if d == nil || d.manager == nil {
		return ""
	}
	if d.manager.data == nil {
		if uid := slashRowUID(row); uid != "" {
			if tr != nil {
				return tr.TranslateFormat("Filter UID {0}", uid)
			}
			return fmt.Sprintf("Filter UID %s", uid)
		}
		return humanizeOptionName(trackingType)
	}
	switch trackingType {
	case "pokemon":
		return tracking.MonsterRowText(d.manager.cfg, tr, d.manager.data, row)
	case "raid":
		return tracking.RaidRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner)
	case "egg":
		return tracking.EggRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner)
	case "maxbattle":
		return tracking.MaxbattleRowText(d.manager.cfg, tr, d.manager.data, row)
	case "quest":
		return tracking.QuestRowText(d.manager.cfg, tr, d.manager.data, row)
	case "rocket", "pokestop-event":
		return tracking.InvasionRowText(d.manager.cfg, tr, d.manager.data, row)
	case "lure":
		return tracking.LureRowText(d.manager.cfg, tr, d.manager.data, row)
	case "weather":
		return tracking.WeatherRowText(tr, d.manager.data, row)
	case "gym":
		return tracking.GymRowText(d.manager.cfg, tr, d.manager.data, row, d.manager.scanner)
	case "nest":
		return tracking.NestRowText(d.manager.cfg, tr, d.manager.data, row)
	case "fort":
		return tracking.FortUpdateRowText(d.manager.cfg, tr, d.manager.data, row)
	default:
		return ""
	}
}

func (d *Discord) slashFilterSummary(tr *i18n.Translator, trackingType string, rows []map[string]any) string {
	lines := []string{}
	for idx, row := range rows {
		if idx >= 5 {
			lines = append(lines, tr.TranslateFormat("And {0} more...", len(rows)-idx))
			break
		}
		label := d.slashFilterRowText(tr, trackingType, row)
		if label == "" {
			continue
		}
		lines = append(lines, "- "+label)
	}
	return strings.Join(lines, "\n")
}

func slashCardDetailLine(label, value string) string {
	label = strings.TrimSpace(label)
	value = strings.TrimSpace(value)
	if label == "" || value == "" {
		return ""
	}
	return fmt.Sprintf("- %s: **%s**", label, value)
}

func slashCardHeading(text string) string {
	text = strings.TrimSpace(text)
	text = strings.TrimRight(text, ":")
	text = strings.TrimRight(text, "：")
	return strings.TrimSpace(text)
}

func slashCardDescription(headline, intro string, detailLines []string, summary string) string {
	sections := []string{}
	if intro = strings.TrimSpace(intro); intro != "" {
		sections = append(sections, intro)
	}
	if headline = strings.TrimSpace(headline); headline != "" {
		sections = append(sections, "## "+headline)
	}
	lines := []string{}
	for _, line := range detailLines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > 0 {
		sections = append(sections, strings.Join(lines, "\n"))
	}
	if summary = strings.TrimSpace(summary); summary != "" {
		sections = append(sections, summary)
	}
	return strings.Join(sections, "\n\n")
}

func slashFilterCardColor(action string) int {
	switch action {
	case "added":
		return slashFilterCardColorAdded
	case "removed":
		return slashFilterCardColorRemoved
	case "restored":
		return slashFilterCardColorRestored
	default:
		return slashFilterCardColorConfirm
	}
}

func (d *Discord) slashFilterCardHeadline(tr *i18n.Translator, trackingType string, rows []map[string]any) string {
	if len(rows) == 1 {
		if label := strings.TrimSpace(d.slashFilterRowText(tr, trackingType, rows[0])); label != "" {
			return label
		}
	}
	if len(rows) > 1 {
		return fmt.Sprintf("%d %s", len(rows), strings.ToLower(translateOrDefault(tr, "Filters")))
	}
	return slashTrackingTitle(tr, trackingType)
}

func (d *Discord) slashFilterPreviewEmbed(i *discordgo.InteractionCreate, title, commandLine, profileLabel string, fields []*discordgo.MessageEmbedField) *discordgo.MessageEmbed {
	tr := d.slashInteractionTranslator(i)
	headline := slashCardHeading(title)
	start := 0
	if len(fields) > 0 && fields[0] != nil && strings.TrimSpace(fields[0].Value) != "" {
		headline = strings.TrimSpace(fmt.Sprintf("%s: %s", strings.TrimSpace(fields[0].Name), strings.TrimSpace(fields[0].Value)))
		start = 1
	}
	detailLines := []string{}
	if profileLabel != "" {
		detailLines = append(detailLines, slashCardDetailLine(tr.Translate("Profile", false), profileLabel))
	}
	for idx := start; idx < len(fields); idx++ {
		field := fields[idx]
		if field == nil {
			continue
		}
		if line := slashCardDetailLine(field.Name, field.Value); line != "" {
			detailLines = append(detailLines, line)
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: slashCardDescription(headline, "", detailLines, ""),
		Color:       slashFilterCardColor("confirm"),
	}
	if commandLine = strings.TrimSpace(commandLine); commandLine != "" {
		embed.Footer = &discordgo.MessageEmbedFooter{Text: commandLine}
	}
	if iconURL := d.slashPreviewIconURL(commandLine); iconURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: iconURL}
	}
	return embed
}

func (d *Discord) slashPreviewIconURL(commandLine string) string {
	parts := strings.Fields(commandLine)
	if len(parts) < 2 {
		return ""
	}
	command := strings.ToLower(parts[0])
	trackingType := slashTrackingTypeFromCommand(command)
	if trackingType == "" {
		return ""
	}
	client := d.slashUiconsClient()
	if client == nil {
		return ""
	}
	switch trackingType {
	case "pokemon":
		name := parts[1]
		if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.PokemonIcon(id, 0, 0, 0, 0, 0, false, 0)
			return url
		}
	case "raid", "maxbattle":
		name := parts[1]
		if strings.HasPrefix(strings.ToLower(name), "level") {
			if len(parts) > 2 {
				if level, err := strconv.Atoi(parts[2]); err == nil && level > 0 {
					url, _ := client.RaidEggIcon(level, false, false)
					return url
				}
			}
		} else if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.PokemonIcon(id, 0, 0, 0, 0, 0, false, 0)
			return url
		}
	case "egg":
		name := parts[1]
		if strings.HasPrefix(strings.ToLower(name), "level") && len(parts) > 2 {
			if level, err := strconv.Atoi(parts[2]); err == nil && level > 0 {
				url, _ := client.RaidEggIcon(level, false, false)
				return url
			}
		} else if level, err := strconv.Atoi(name); err == nil && level > 0 {
			url, _ := client.RaidEggIcon(level, false, false)
			return url
		}
	}
	return ""
}

func (d *Discord) pokemonIDFromName(name string) int {
	if d.manager == nil || d.manager.data == nil {
		return 0
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	if id, err := strconv.Atoi(lower); err == nil {
		return id
	}
	for _, raw := range d.manager.data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		monName := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		if monName == lower {
			return toInt(mon["id"], 0)
		}
	}
	return 0
}

func (d *Discord) slashFilterMutationEmbed(i *discordgo.InteractionCreate, action, trackingType string, rows []map[string]any, profileLabel string) *discordgo.MessageEmbed {
	tr := d.slashInteractionTranslator(i)
	headline := d.slashFilterTypedHeading(tr, trackingType, rows)
	detailLines := []string{}
	if profileLabel != "" {
		detailLines = append(detailLines, slashCardDetailLine(tr.Translate("Profile", false), profileLabel))
	}
	if len(rows) == 1 {
		detailLines = append(detailLines, slashFilterNonDefaultDetailLines(tr, trackingType, rows[0])...)
	} else if len(rows) > 1 {
		for idx, row := range rows {
			if idx >= 5 {
				detailLines = append(detailLines, fmt.Sprintf("*%s*", tr.TranslateFormat("And {0} more...", len(rows)-idx)))
				break
			}
			label := d.slashFilterRowText(tr, trackingType, row)
			if label != "" {
				detailLines = append(detailLines, "- "+label)
			}
		}
	}
	embed := &discordgo.MessageEmbed{
		Title:       tr.Translate(slashFilterMutationTitleKey(action, len(rows)), false),
		Description: slashCardDescription(headline, "", detailLines, ""),
		Color:       slashFilterCardColor(action),
	}
	if iconURL := d.slashFilterIconURL(trackingType, rows); iconURL != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: iconURL}
	}
	return embed
}

func (d *Discord) slashFilterMutationComponents(i *discordgo.InteractionCreate, trackingType, customID string, count int, restoring bool) []discordgo.MessageComponent {
	tr := d.slashInteractionTranslator(i)
	label := tr.Translate("Remove Filter", false)
	style := discordgo.DangerButton
	if count > 1 {
		label = tr.Translate("Remove Filters", false)
	}
	if restoring {
		label = tr.Translate("Restore Filter", false)
		style = discordgo.SuccessButton
		if count > 1 {
			label = tr.Translate("Restore Filters", false)
		}
	}
	return []discordgo.MessageComponent{
		discordgo.ActionsRow{Components: []discordgo.MessageComponent{
			discordgo.Button{CustomID: customID, Label: label, Style: style},
		}},
	}
}

func slashFilterMutationTitleKey(action string, count int) string {
	if count == 1 {
		switch action {
		case "added":
			return "Filter Added"
		case "removed":
			return "Filter Removed"
		case "restored":
			return "Filter Restored"
		}
	}
	switch action {
	case "added":
		return "Filters Added"
	case "removed":
		return "Filters Removed"
	case "restored":
		return "Filters Restored"
	default:
		return "Confirm Command:"
	}
}

func slashFilterMutationProfileKey(action string) string {
	switch action {
	case "added":
		return "Saved to {0}."
	case "removed":
		return "Removed from {0}."
	case "restored":
		return "Restored in {0}."
	default:
		return ""
	}
}

func (d *Discord) slashFilterMutationResponse(i *discordgo.InteractionCreate, action, trackingType, table, profileLabel, userID string, rows []map[string]any) ([]*discordgo.MessageEmbed, []discordgo.MessageComponent, bool) {
	if len(rows) == 0 {
		return nil, nil, false
	}
	customID := ""
	switch action {
	case "added", "restored":
		customID = slashFilterRemoveButtonPrefix + d.storeSlashFilterAction(&slashFilterActionState{
			UserID:       userID,
			Table:        table,
			TrackingType: trackingType,
			Rows:         rows,
			ProfileLabel: profileLabel,
			ExpiresAt:    time.Now().Add(15 * time.Minute),
		})
	case "removed":
		customID = slashFilterRestoreButtonPrefix + d.storeSlashFilterAction(&slashFilterActionState{
			UserID:       userID,
			Table:        table,
			TrackingType: trackingType,
			Rows:         rows,
			ProfileLabel: profileLabel,
			ExpiresAt:    time.Now().Add(15 * time.Minute),
		})
	}
	if customID == "" {
		return nil, nil, false
	}
	embed := d.slashFilterMutationEmbed(i, action, trackingType, rows, profileLabel)
	return []*discordgo.MessageEmbed{embed}, d.slashFilterMutationComponents(i, trackingType, customID, len(rows), action == "removed"), true
}

func (d *Discord) handleSlashFilterRemoveAction(s *discordgo.Session, i *discordgo.InteractionCreate, id string) {
	userID, _ := slashUser(i)
	action, errKey := d.slashFilterAction(id, userID)
	if errKey != "" {
		d.respondEphemeralError(s, i, d.slashText(i, errKey))
		return
	}
	removed, err := d.deleteSlashTrackingRows(action.Table, action.Rows)
	if err != nil {
		d.respondEphemeralError(s, i, err.Error())
		return
	}
	if removed == 0 {
		d.respondEphemeralError(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	embeds, components, ok := d.slashFilterMutationResponse(i, "removed", action.TrackingType, action.Table, action.ProfileLabel, userID, action.Rows)
	if !ok {
		d.respondEphemeralError(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", embeds, components)
}

func (d *Discord) handleSlashFilterRestoreAction(s *discordgo.Session, i *discordgo.InteractionCreate, id string) {
	userID, _ := slashUser(i)
	action, errKey := d.slashFilterAction(id, userID)
	if errKey != "" {
		d.respondEphemeralError(s, i, d.slashText(i, errKey))
		return
	}
	restored, err := d.restoreSlashTrackingRows(action.Table, action.Rows)
	if err != nil {
		d.respondEphemeralError(s, i, err.Error())
		return
	}
	if restored == 0 {
		d.respondEphemeralError(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	embeds, components, ok := d.slashFilterMutationResponse(i, "restored", action.TrackingType, action.Table, action.ProfileLabel, userID, action.Rows)
	if !ok {
		d.respondEphemeralError(s, i, d.slashText(i, "No filters were changed."))
		return
	}
	d.respondUpdateComponentsEmbed(s, i, "", embeds, components)
}
