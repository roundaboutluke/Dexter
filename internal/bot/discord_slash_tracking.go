package bot

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func (d *Discord) handleSlashTrack(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	pokemon, ok := optionString(options, "pokemon")
	if !ok || strings.TrimSpace(pokemon) == "" {
		d.startSlashGuide(i, "track", "monster")
		d.logSlashUX(i, "track", "guide_entry", "")
		d.respondWithMonsterOptions(s, i)
		return
	}
	d.logSlashUX(i, "track", "direct_submit", "")
	pokemon = strings.TrimSpace(pokemon)
	args := []string{pokemon}
	if strings.EqualFold(pokemon, "everything") {
		args = []string{"everything"}
	}

	if value, ok := optionString(options, "gender"); ok {
		switch strings.ToLower(value) {
		case "male", "female":
			args = append(args, strings.ToLower(value))
		}
	}
	if value, ok := optionString(options, "size"); ok && !strings.EqualFold(value, "all") {
		args = append(args, "size"+strings.ToLower(value))
	}

	minIV := optionalInt(options, "min_iv")
	maxIV := optionalInt(options, "max_iv")
	args = appendRangeArg(args, "iv", "maxiv", minIV, maxIV)

	minAtk := optionalInt(options, "min_atk")
	maxAtk := optionalInt(options, "max_atk")
	args = appendRangeArg(args, "atk", "maxatk", minAtk, maxAtk)

	minDef := optionalInt(options, "min_def")
	maxDef := optionalInt(options, "max_def")
	args = appendRangeArg(args, "def", "maxdef", minDef, maxDef)

	minSta := optionalInt(options, "min_sta")
	maxSta := optionalInt(options, "max_sta")
	args = appendRangeArg(args, "sta", "maxsta", minSta, maxSta)

	minCP := optionalInt(options, "min_cp")
	maxCP := optionalInt(options, "max_cp")
	args = appendRangeArg(args, "cp", "maxcp", minCP, maxCP)

	minLevel := optionalInt(options, "min_level")
	maxLevel := optionalInt(options, "max_level")
	args = appendRangeArg(args, "level", "maxlevel", minLevel, maxLevel)

	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionInt(options, "min_time"); ok && value > 0 {
		args = append(args, fmt.Sprintf("t%d", value))
	}

	if value, ok := optionString(options, "pvp_league"); ok {
		if ranks, ok := optionInt(options, "pvp_ranks"); ok && ranks > 0 {
			league := strings.ToLower(value)
			if league == "great" || league == "ultra" || league == "little" {
				args = append(args, fmt.Sprintf("%s1-%d", league, ranks))
			}
		}
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	if value, ok := optionString(options, "form"); ok && strings.TrimSpace(value) != "" {
		formValue := strings.TrimSpace(value)
		if strings.EqualFold(formValue, "all") {
			args = append(args, "form:all")
		} else {
			args = append(args, "form:"+formValue)
		}
	} else if !strings.EqualFold(pokemon, "everything") {
		args = append(args, "form:all")
	}

	d.promptSlashConfirmation(s, i, "track", args, d.confirmTitle(i, "track"), d.confirmFields(i))
}

func (d *Discord) handleSlashRaid(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	raidType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(raidType) == "" {
		d.startSlashGuide(i, "raid", "raid")
		d.logSlashUX(i, "raid", "guide_entry", "")
		d.respondWithRaidOptions(s, i)
		return
	}
	d.logSlashUX(i, "raid", "direct_submit", "")
	args := []string{normalizeRaidType(strings.TrimSpace(raidType))}
	if value, ok := optionString(options, "team"); ok {
		switch strings.ToLower(value) {
		case "blue", "red", "yellow", "white":
			args = append(args, strings.ToLower(value))
		}
	}
	if value, ok := optionString(options, "rsvp"); ok {
		switch strings.ToLower(value) {
		case "on":
			args = append(args, "rsvp")
		case "only":
			args = append(args, "rsvp", "only")
		case "off":
			args = append(args, "no", "rsvp")
		}
	}
	if value, ok := optionString(options, "gym"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "gym:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "raid", args, d.confirmTitle(i, "raid"), d.confirmFields(i))
}

func (d *Discord) handleSlashMaxbattle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	mbType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(mbType) == "" {
		d.startSlashGuide(i, "maxbattle", "maxbattle")
		d.logSlashUX(i, "maxbattle", "guide_entry", "")
		d.respondWithMaxbattleOptions(s, i)
		return
	}
	d.logSlashUX(i, "maxbattle", "direct_submit", "")
	args := []string{normalizeRaidType(strings.TrimSpace(mbType))}
	if value, ok := optionBool(options, "gmax_only"); ok && value {
		args = append(args, "gmax")
	}
	if value, ok := optionString(options, "station"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "station:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "maxbattle", args, d.confirmTitle(i, "maxbattle"), d.confirmFields(i))
}

func (d *Discord) handleSlashEgg(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	level, ok := optionString(options, "level")
	if !ok || strings.TrimSpace(level) == "" {
		d.startSlashGuide(i, "egg", "egg")
		d.logSlashUX(i, "egg", "guide_entry", "")
		d.respondWithEggOptions(s, i)
		return
	}
	d.logSlashUX(i, "egg", "direct_submit", "")
	args := []string{}
	if ok && strings.TrimSpace(level) != "" {
		args = append(args, normalizeRaidType(strings.TrimSpace(level)))
	}
	if value, ok := optionString(options, "team"); ok {
		switch strings.ToLower(value) {
		case "blue", "red", "yellow", "white":
			args = append(args, strings.ToLower(value))
		}
	}
	if value, ok := optionString(options, "rsvp"); ok {
		switch strings.ToLower(value) {
		case "on":
			args = append(args, "rsvp")
		case "only":
			args = append(args, "rsvp", "only")
		case "off":
			args = append(args, "no", "rsvp")
		}
	}
	if value, ok := optionString(options, "gym"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "gym:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "egg", args, d.confirmTitle(i, "egg"), d.confirmFields(i))
}

func (d *Discord) handleSlashQuest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	questType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(questType) == "" {
		d.startSlashGuide(i, "quest", "quest")
		d.logSlashUX(i, "quest", "guide_entry", "")
		d.respondWithQuestInput(s, i)
		return
	}
	d.logSlashUX(i, "quest", "direct_submit", "")
	questType = strings.TrimSpace(questType)
	if minAmount, ok := optionInt(options, "min_amount"); ok && minAmount > 0 {
		if strings.HasPrefix(strings.ToLower(questType), "stardust") {
			questType = fmt.Sprintf("stardust%d", minAmount)
		}
	}
	args := []string{formatQuestArg(questType)}
	if value, ok := optionString(options, "ar"); ok {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "ar":
			args = append(args, "ar")
		case "noar":
			args = append(args, "noar")
		}
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "quest", args, d.confirmTitle(i, "quest"), d.confirmFields(i))
}

func (d *Discord) handleSlashIncident(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	incidentType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(incidentType) == "" {
		d.startSlashGuide(i, "invasion", "invasion")
		d.logSlashUX(i, "invasion", "guide_entry", "")
		d.respondWithInvasionInput(s, i)
		return
	}
	d.logSlashUX(i, "invasion", "direct_submit", "")
	args := []string{formatInvasionArg(strings.TrimSpace(incidentType))}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "invasion", args, d.confirmTitle(i, "invasion"), d.confirmFields(i))
}

func (d *Discord) handleSlashGym(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	team, ok := optionString(options, "team")
	if !ok || strings.TrimSpace(team) == "" {
		d.startSlashGuide(i, "gym", "gym")
		d.logSlashUX(i, "gym", "guide_entry", "")
		d.respondWithGymTeamSelect(s, i)
		return
	}
	d.logSlashUX(i, "gym", "direct_submit", "")
	args := []string{strings.ToLower(strings.TrimSpace(team))}
	if value, ok := optionBool(options, "slot_changes"); ok && value {
		args = append(args, "slot_changes")
	}
	if value, ok := optionBool(options, "battle_changes"); ok && value {
		args = append(args, "battle_changes")
	}
	if value, ok := optionString(options, "gym"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "gym:"+strings.TrimSpace(value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "gym", args, d.confirmTitle(i, "gym"), d.confirmFields(i))
}

func (d *Discord) handleSlashFort(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	args := []string{}
	if value, ok := optionString(options, "type"); ok && strings.TrimSpace(value) != "" {
		args = append(args, strings.ToLower(strings.TrimSpace(value)))
	}
	if value, ok := optionBool(options, "include_empty"); ok && value {
		args = append(args, "include_empty")
	}
	if value, ok := optionBool(options, "location"); ok && value {
		args = append(args, "location")
	}
	if value, ok := optionBool(options, "name"); ok && value {
		args = append(args, "name")
	}
	if value, ok := optionBool(options, "photo"); ok && value {
		args = append(args, "photo")
	}
	if value, ok := optionBool(options, "removal"); ok && value {
		args = append(args, "removal")
	}
	if value, ok := optionBool(options, "new"); ok && value {
		args = append(args, "new")
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	if len(args) == 0 {
		d.startSlashGuide(i, "fort", "fort")
		d.logSlashUX(i, "fort", "guide_entry", "")
		d.respondWithFortTypeSelect(s, i)
		return
	}
	d.logSlashUX(i, "fort", "direct_submit", "")
	d.promptSlashConfirmation(s, i, "fort", args, d.confirmTitle(i, "fort"), d.confirmFields(i))
}

func (d *Discord) handleSlashNest(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	pokemon, ok := optionString(options, "pokemon")
	if !ok || strings.TrimSpace(pokemon) == "" {
		d.startSlashGuide(i, "nest", "nest")
		d.logSlashUX(i, "nest", "guide_entry", "")
		d.respondWithMonsterSearch(s, i)
		return
	}
	d.logSlashUX(i, "nest", "direct_submit", "")
	args := []string{strings.TrimSpace(pokemon)}
	if value, ok := optionInt(options, "min_spawn"); ok && value > 0 {
		args = append(args, fmt.Sprintf("minspawn%d", value))
	}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "nest", args, d.confirmTitle(i, "nest"), d.confirmFields(i))
}

func (d *Discord) handleSlashWeather(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	condition, ok := optionString(options, "condition")
	if !ok || strings.TrimSpace(condition) == "" {
		d.startSlashGuide(i, "weather", "weather")
		d.logSlashUX(i, "weather", "guide_entry", "")
		d.respondWithWeatherConditionSelect(s, i)
		return
	}
	d.logSlashUX(i, "weather", "direct_submit", "")
	location, _ := optionString(options, "location")
	location = strings.TrimSpace(location)
	if location == "" {
		userID, _ := slashUser(i)
		if d.manager != nil && d.manager.query != nil && userID != "" {
			if row, err := d.manager.query.SelectOneQuery("humans", map[string]any{"id": userID}); err == nil && row != nil {
				lat := toFloat(row["latitude"])
				lon := toFloat(row["longitude"])
				if lat != 0 || lon != 0 {
					location = fmt.Sprintf("%s,%s", formatFloat(lat), formatFloat(lon))
				} else if d.manager.fences != nil {
					areas := parseAreaListFromHuman(row)
					if len(areas) > 0 {
						target := strings.ToLower(strings.TrimSpace(areas[0]))
						for _, fence := range d.manager.fences.Fences {
							if strings.EqualFold(strings.TrimSpace(fence.Name), target) {
								if centerLat, centerLon, ok := fenceCentroid(fence); ok {
									location = fmt.Sprintf("%s,%s", formatFloat(centerLat), formatFloat(centerLon))
								}
								break
							}
						}
					}
				}
			}
		}
	}
	if location == "" {
		d.respondEphemeral(s, i, d.slashText(i, "Please set your location in /profile, or provide a location."))
		return
	}
	args := []string{}
	args = append(args, strings.Fields(location)...)
	args = append(args, "|", strings.TrimSpace(condition))
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "weather", args, d.confirmTitle(i, "weather"), d.confirmFields(i))
}

func (d *Discord) handleSlashLure(s *discordgo.Session, i *discordgo.InteractionCreate) {
	options := slashOptions(i.ApplicationCommandData())
	lureType, ok := optionString(options, "type")
	if !ok || strings.TrimSpace(lureType) == "" {
		d.startSlashGuide(i, "lure", "lure")
		d.logSlashUX(i, "lure", "guide_entry", "")
		d.respondWithLureTypeSelect(s, i)
		return
	}
	d.logSlashUX(i, "lure", "direct_submit", "")
	args := []string{strings.TrimSpace(lureType)}
	if value, ok := optionInt(options, "distance"); ok && value > 0 {
		args = append(args, fmt.Sprintf("d%d", value))
	}
	if optionEnabled(options, "clean") {
		args = append(args, "clean")
	}
	if value, ok := optionString(options, "template"); ok && strings.TrimSpace(value) != "" {
		args = append(args, "template:"+strings.TrimSpace(value))
	}
	d.promptSlashConfirmation(s, i, "lure", args, d.confirmTitle(i, "lure"), d.confirmFields(i))
}
