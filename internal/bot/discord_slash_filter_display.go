package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"
	"poraclego/internal/i18n"
	"poraclego/internal/uicons"
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
		pokemonID := toInt(row["pokemon_id"], pokemonIDWildcard)
		if pokemonID == pokemonIDWildcard || pokemonID == 0 {
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
		case questRewardPokemon:
			if reward == 0 {
				return ""
			}
			form := toInt(row["form"], 0)
			url, _ := client.PokemonIcon(reward, form, 0, 0, 0, 0, false, 0)
			return url
		case questRewardItem:
			if reward > 0 {
				url, _ := client.RewardItemIcon(reward)
				return url
			}
		case questRewardStardust:
			url, _ := client.RewardStardustIcon(reward)
			return url
		case questRewardMegaEnergy:
			url, _ := client.RewardMegaEnergyIcon(reward, 0)
			return url
		case questRewardCandy:
			url, _ := client.RewardCandyIcon(reward, 0)
			return url
		case questRewardXLCandy:
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
		pokemonID := toInt(row["pokemon_id"], pokemonIDWildcard)
		if pokemonID == pokemonIDWildcard || pokemonID == 0 {
			level := toInt(row["level"], 0)
			if level == allLevelsSentinel {
				name = translateOrDefault(tr, "All levels")
			} else if level > 0 {
				name = fmt.Sprintf("%s %d", translateOrDefault(tr, "Level"), level)
			}
		} else {
			name = d.slashMonsterName(tr, row)
		}
	case "egg":
		level := toInt(row["level"], 0)
		if level == allLevelsSentinel {
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
		} else {
			name = d.slashLureName(tr, lureID)
		}
	case "weather":
		condition := toInt(row["condition"], 0)
		if condition == 0 {
			name = translateOrDefault(tr, "Everything")
		} else {
			name = d.slashWeatherName(tr, condition)
		}
	case "gym":
		name = d.slashGymTeamName(tr, row)
	case "nest":
		name = d.slashMonsterName(tr, row)
	case "fort":
		fortType := strings.TrimSpace(fmt.Sprintf("%v", row["fort_type"]))
		if fortType == "" || strings.EqualFold(fortType, "everything") {
			name = translateOrDefault(tr, "Everything")
		} else {
			name = translateOrDefault(tr, humanizeOptionName(fortType))
		}
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
	case questRewardPokemon:
		fakeRow := map[string]any{"pokemon_id": reward, "form": form}
		return d.slashMonsterName(tr, fakeRow)
	case questRewardItem:
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
	case questRewardStardust:
		return translateOrDefault(tr, "Stardust")
	case questRewardMegaEnergy:
		name := translateOrDefault(tr, "Mega Energy")
		if reward > 0 {
			fakeRow := map[string]any{"pokemon_id": reward, "form": 0}
			monName := d.slashMonsterName(tr, fakeRow)
			if monName != "" {
				return name + " " + monName
			}
		}
		return name
	case questRewardCandy:
		if reward == 0 {
			return translateOrDefault(tr, "Rare Candy")
		}
		fakeRow := map[string]any{"pokemon_id": reward, "form": 0}
		return d.slashMonsterName(tr, fakeRow) + " Candy"
	case questRewardXLCandy:
		if reward == 0 {
			return translateOrDefault(tr, "Rare Candy XL")
		}
		fakeRow := map[string]any{"pokemon_id": reward, "form": 0}
		return d.slashMonsterName(tr, fakeRow) + " XL Candy"
	case questRewardExperience:
		return translateOrDefault(tr, "Experience")
	default:
		return translateOrDefault(tr, "Reward")
	}
}

// slashLureName resolves a lure name from its ID using game data.
func (d *Discord) slashLureName(tr *i18n.Translator, lureID int) string {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return fmt.Sprintf("#%d", lureID)
	}
	entry, ok := d.manager.data.UtilData["lures"]
	if !ok {
		return fmt.Sprintf("#%d", lureID)
	}
	switch v := entry.(type) {
	case []any:
		if lureID >= 0 && lureID < len(v) {
			if m, ok := v[lureID].(map[string]any); ok {
				if name := fmt.Sprintf("%v", m["name"]); name != "" {
					return translateOrDefault(tr, name)
				}
			}
		}
	case map[string]any:
		if value, ok := v[fmt.Sprintf("%d", lureID)]; ok {
			if m, ok := value.(map[string]any); ok {
				if name := fmt.Sprintf("%v", m["name"]); name != "" {
					return translateOrDefault(tr, name)
				}
			}
		}
	}
	return fmt.Sprintf("#%d", lureID)
}

// slashWeatherName resolves a weather condition name from its ID using game data.
func (d *Discord) slashWeatherName(tr *i18n.Translator, condition int) string {
	if d.manager == nil || d.manager.data == nil || d.manager.data.UtilData == nil {
		return fmt.Sprintf("#%d", condition)
	}
	if raw, ok := d.manager.data.UtilData["weather"].(map[string]any); ok {
		if entry, ok := raw[fmt.Sprintf("%d", condition)].(map[string]any); ok {
			if name := fmt.Sprintf("%v", entry["name"]); name != "" {
				return translateOrDefault(tr, name)
			}
		}
	}
	return fmt.Sprintf("#%d", condition)
}

// slashGymTeamName resolves a gym team name for display in headings.
func (d *Discord) slashGymTeamName(tr *i18n.Translator, row map[string]any) string {
	team := toInt(row["team"], 4)
	switch team {
	case 0:
		return translateOrDefault(tr, "Uncontested")
	case 1:
		return translateOrDefault(tr, "Mystic")
	case 2:
		return translateOrDefault(tr, "Valor")
	case 3:
		return translateOrDefault(tr, "Instinct")
	case 4:
		return translateOrDefault(tr, "Everything")
	default:
		return translateOrDefault(tr, "Everything")
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
		minIV := toInt(row["min_iv"], -1)
		maxIV := toInt(row["max_iv"], 100)
		if minIV != -1 || maxIV != 100 {
			displayMin := minIV
			if displayMin < 0 {
				displayMin = 0
			}
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "IV"), fmt.Sprintf("%d%% - %d%%", displayMin, maxIV)))
		}
		minCP := toInt(row["min_cp"], 0)
		maxCP := toInt(row["max_cp"], defaultMaxCP)
		if minCP != 0 || maxCP != defaultMaxCP {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "CP"), fmt.Sprintf("%d - %d", minCP, maxCP)))
		}
		minLvl := toInt(row["min_level"], 0)
		maxLvl := toInt(row["max_level"], 55)
		if minLvl != 0 || maxLvl != 55 {
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
		size := toInt(row["size"], -1)
		maxSize := toInt(row["max_size"], 5)
		if size != -1 || maxSize != 5 {
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
	case "fort":
		distance := toInt(row["distance"], 0)
		if distance > 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Distance"), fmt.Sprintf("%dm", distance)))
		}
		changeTypes := strings.TrimSpace(fmt.Sprintf("%v", row["change_types"]))
		if changeTypes != "" && changeTypes != "[]" && changeTypes != "null" {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Changes"), changeTypes))
		}
		includeEmpty := toInt(row["include_empty"], 0)
		if includeEmpty != 0 {
			lines = append(lines, slashCardDetailLine(translateOrDefault(tr, "Include empty"), translateOrDefault(tr, "Yes")))
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
		return -1
	case "max_iv":
		return 100
	case "min_cp":
		return 0
	case "max_cp":
		return defaultMaxCP
	case "min_level":
		return 0
	case "max_level":
		return 55
	case "atk", "def", "sta":
		return 0
	case "max_atk", "max_def", "max_sta":
		return 15
	case "gender":
		return 0
	case "size":
		return -1
	case "max_size":
		return 5
	case "rarity":
		return -1
	case "max_rarity":
		return 6
	case "distance", "min_time":
		return 0
	default:
		return 0
	}
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
		detailLines = append(detailLines, slashCardDetailLine(translateOrDefault(tr,"Profile"), profileLabel))
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
		if id := d.pokemonIDFromName(parts[1]); id > 0 {
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
	case "quest":
		return d.slashPreviewQuestIconURL(client, parts[1:])
	case "gym":
		teamName := strings.ToLower(parts[1])
		teamID := slashGymTeamID(teamName)
		if teamID >= 0 {
			url, _ := client.GymIcon(teamID, 0, false, false)
			return url
		}
	case "nest":
		if id := d.pokemonIDFromName(parts[1]); id > 0 {
			url, _ := client.PokemonIcon(id, 0, 0, 0, 0, 0, false, 0)
			return url
		}
	}
	return ""
}

// slashPreviewQuestIconURL resolves an icon for a quest from command line args.
// Args are the parts after the command name, e.g. ["energy:charizard", "d500"].
func (d *Discord) slashPreviewQuestIconURL(client *uicons.Client, args []string) string {
	if len(args) == 0 {
		return ""
	}
	arg := strings.ToLower(args[0])
	// Stardust: "stardust" or "stardust500"
	if strings.HasPrefix(arg, "stardust") {
		url, _ := client.RewardStardustIcon(0)
		return url
	}
	// Mega energy: "energy:charizard"
	if strings.HasPrefix(arg, "energy:") {
		name := strings.TrimPrefix(arg, "energy:")
		name = strings.Trim(name, "\"")
		if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.RewardMegaEnergyIcon(id, 0)
			return url
		}
		return ""
	}
	// Candy: "candy:pikachu"
	if strings.HasPrefix(arg, "candy:") {
		name := strings.TrimPrefix(arg, "candy:")
		name = strings.Trim(name, "\"")
		if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.RewardCandyIcon(id, 0)
			return url
		}
		return ""
	}
	// XL Candy: "xlcandy:pikachu"
	if strings.HasPrefix(arg, "xlcandy:") {
		name := strings.TrimPrefix(arg, "xlcandy:")
		name = strings.Trim(name, "\"")
		if id := d.pokemonIDFromName(name); id > 0 {
			url, _ := client.RewardXLCandyIcon(id, 0)
			return url
		}
		return ""
	}
	// Pokemon quest reward: try to resolve as a pokemon name
	name := strings.Trim(args[0], "\"")
	if id := d.pokemonIDFromName(name); id > 0 {
		url, _ := client.PokemonIcon(id, 0, 0, 0, 0, 0, false, 0)
		return url
	}
	// Item quest reward: try to resolve as an item name
	if id := d.itemIDFromName(name); id > 0 {
		url, _ := client.RewardItemIcon(id)
		return url
	}
	return ""
}

// slashGymTeamID returns the team ID for a gym team name, or -1 if unknown.
func slashGymTeamID(name string) int {
	switch name {
	case "uncontested", "harmony":
		return 0
	case "mystic", "blue":
		return 1
	case "valor", "valour", "red":
		return 2
	case "instinct", "yellow":
		return 3
	case "everything":
		return 4
	default:
		return -1
	}
}

// itemIDFromName looks up an item ID by name from game data.
func (d *Discord) itemIDFromName(name string) int {
	if d.manager == nil || d.manager.data == nil || d.manager.data.Items == nil {
		return 0
	}
	lower := strings.ToLower(strings.Trim(strings.TrimSpace(name), "\""))
	for key, raw := range d.manager.data.Items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		itemName := strings.ToLower(fmt.Sprintf("%v", item["name"]))
		if itemName == lower {
			id, _ := strconv.Atoi(key)
			return id
		}
	}
	return 0
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
