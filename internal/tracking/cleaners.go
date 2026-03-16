package tracking

import (
	"fmt"
	"strconv"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
)

// CleanMonsterRow normalizes a pokemon tracking rule for persistence.
func CleanMonsterRow(cfg *config.Config, scope RuleScope, row map[string]any) (map[string]any, error) {
	if row["pokemon_id"] == nil {
		return nil, fmt.Errorf("Pokemon id must be specified")
	}
	template := defaultTemplate(templateName(cfg))
	distance := floatFromAnyRule(row["distance"])
	if distance == 0 {
		if def, ok := cfg.GetInt("tracking.defaultDistance"); ok {
			distance = float64(def)
		}
	}
	maxDistance, _ := cfg.GetInt("tracking.maxDistance")
	if maxDistance == 0 {
		maxDistance = 40000000
	}
	if distance > float64(maxDistance) {
		distance = float64(maxDistance)
	}
	newRow := map[string]any{
		"id":         scope.UserID,
		"profile_no": numberFromAnyOrDefaultRule(row["profile_no"], scope.ProfileNo),
		"ping":       "",
		"template":   getStringValueRule(row["template"], template),
	}
	newRow["pokemon_id"] = intFromAny(row["pokemon_id"])
	newRow["distance"] = int(distance)
	newRow["min_iv"] = defaultIntRule(row["min_iv"], -1)
	newRow["max_iv"] = defaultIntRule(row["max_iv"], 100)
	newRow["min_cp"] = defaultIntRule(row["min_cp"], 0)
	newRow["max_cp"] = defaultIntRule(row["max_cp"], 9000)
	newRow["min_level"] = defaultIntRule(row["min_level"], 0)
	newRow["max_level"] = defaultIntRule(row["max_level"], 55)
	newRow["atk"] = defaultIntRule(row["atk"], 0)
	newRow["def"] = defaultIntRule(row["def"], 0)
	newRow["sta"] = defaultIntRule(row["sta"], 0)
	newRow["min_weight"] = defaultIntRule(row["min_weight"], 0)
	newRow["max_weight"] = defaultIntRule(row["max_weight"], 9000000)
	newRow["form"] = defaultIntRule(row["form"], 0)
	newRow["max_atk"] = defaultIntRule(row["max_atk"], 15)
	newRow["max_def"] = defaultIntRule(row["max_def"], 15)
	newRow["max_sta"] = defaultIntRule(row["max_sta"], 15)
	newRow["gender"] = defaultIntRule(row["gender"], 0)
	newRow["clean"] = defaultIntRule(row["clean"], 0)
	newRow["pvp_ranking_league"] = defaultIntRule(row["pvp_ranking_league"], 0)
	newRow["pvp_ranking_best"] = defaultIntRule(row["pvp_ranking_best"], 1)
	newRow["pvp_ranking_worst"] = defaultIntRule(row["pvp_ranking_worst"], 4096)
	newRow["pvp_ranking_min_cp"] = defaultIntRule(row["pvp_ranking_min_cp"], 0)
	newRow["pvp_ranking_cap"] = defaultIntRule(row["pvp_ranking_cap"], 0)
	newRow["size"] = defaultIntRule(row["size"], -1)
	newRow["max_size"] = defaultIntRule(row["max_size"], 5)
	newRow["rarity"] = defaultIntRule(row["rarity"], -1)
	newRow["max_rarity"] = defaultIntRule(row["max_rarity"], 6)
	newRow["min_time"] = defaultIntRule(row["min_time"], 0)

	if uid := row["uid"]; uid != nil {
		newRow["uid"] = uid
	}
	return newRow, nil
}

// CleanRaidRow normalizes a raid tracking rule for persistence.
func CleanRaidRow(cfg *config.Config, game *data.GameData, scope RuleScope, row map[string]any) (map[string]any, error) {
	template := defaultTemplate(templateName(cfg))
	monsterID := defaultIntRule(row["pokemon_id"], 9000)
	level := 9000
	if monsterID == 9000 {
		level = defaultIntRule(row["level"], -1)
		maxLevel := raidMaxLevel(game.UtilData)
		if level < 1 || (level > maxLevel && level != 90) {
			return nil, fmt.Errorf("Invalid level (must be specified if no pokemon_id")
		}
	}
	team := defaultIntRule(row["team"], 4)
	if team < 0 || team > 4 {
		team = 4
	}
	rsvp := defaultIntRule(row["rsvp_changes"], 0)
	if rsvp < 0 || rsvp > 2 {
		rsvp = 0
	}
	return map[string]any{
		"id":           scope.UserID,
		"profile_no":   scope.ProfileNo,
		"ping":         "",
		"template":     getStringValueRule(row["template"], template),
		"pokemon_id":   monsterID,
		"exclusive":    defaultIntRule(row["exclusive"], 0),
		"distance":     defaultIntRule(row["distance"], 0),
		"team":         team,
		"clean":        defaultIntRule(row["clean"], 0),
		"level":        level,
		"form":         defaultIntRule(row["form"], 0),
		"move":         defaultIntRule(row["move"], 9000),
		"evolution":    defaultIntRule(row["evolution"], 9000),
		"gym_id":       row["gym_id"],
		"rsvp_changes": rsvp,
	}, nil
}

// CleanEggRow normalizes an egg tracking rule for persistence.
func CleanEggRow(cfg *config.Config, game *data.GameData, scope RuleScope, row map[string]any) (map[string]any, error) {
	template := defaultTemplate(templateName(cfg))
	level := defaultIntRule(row["level"], -1)
	maxLevel := raidMaxLevel(game.UtilData)
	if level < 1 || (level > maxLevel && level != 90) {
		return nil, fmt.Errorf("Invalid level")
	}
	team := defaultIntRule(row["team"], 4)
	if team < 0 || team > 4 {
		team = 4
	}
	rsvp := defaultIntRule(row["rsvp_changes"], 0)
	if rsvp < 0 || rsvp > 2 {
		rsvp = 0
	}
	return map[string]any{
		"id":           scope.UserID,
		"profile_no":   scope.ProfileNo,
		"ping":         "",
		"template":     getStringValueRule(row["template"], template),
		"exclusive":    defaultIntRule(row["exclusive"], 0),
		"distance":     defaultIntRule(row["distance"], 0),
		"team":         team,
		"clean":        defaultIntRule(row["clean"], 0),
		"level":        level,
		"gym_id":       row["gym_id"],
		"rsvp_changes": rsvp,
	}, nil
}

// CleanQuestRow normalizes a quest tracking rule for persistence.
func CleanQuestRow(cfg *config.Config, scope RuleScope, row map[string]any) (map[string]any, error) {
	template := defaultTemplate(templateName(cfg))
	rewardType := defaultIntRule(row["reward_type"], -1)
	validRewardTypes := map[int]bool{1: true, 2: true, 3: true, 4: true, 7: true, 9: true, 12: true}
	if !validRewardTypes[rewardType] {
		return nil, fmt.Errorf("Unrecognised reward_type value")
	}
	return map[string]any{
		"id":          scope.UserID,
		"profile_no":  scope.ProfileNo,
		"ping":        "",
		"template":    getStringValueRule(row["template"], template),
		"distance":    defaultIntRule(row["distance"], 0),
		"clean":       defaultIntRule(row["clean"], 0),
		"reward_type": rewardType,
		"reward":      defaultIntRule(row["reward"], 0),
		"amount":      defaultIntRule(row["amount"], 0),
		"form":        defaultIntRule(row["form"], 0),
	}, nil
}

// CleanInvasionRow normalizes an invasion tracking rule for persistence.
func CleanInvasionRow(cfg *config.Config, scope RuleScope, row map[string]any) (map[string]any, error) {
	if getString(row["grunt_type"]) == "" {
		return nil, fmt.Errorf("Grunt type mandatory")
	}
	template := defaultTemplate(templateName(cfg))
	return map[string]any{
		"id":         scope.UserID,
		"profile_no": scope.ProfileNo,
		"ping":       "",
		"template":   getStringValueRule(row["template"], template),
		"distance":   defaultIntRule(row["distance"], 0),
		"clean":      defaultIntRule(row["clean"], 0),
		"gender":     defaultIntRule(row["gender"], 0),
		"grunt_type": getString(row["grunt_type"]),
	}, nil
}

// CleanLureRow normalizes a lure tracking rule for persistence.
func CleanLureRow(cfg *config.Config, scope RuleScope, row map[string]any) (map[string]any, error) {
	template := defaultTemplate(templateName(cfg))
	lureID := defaultIntRule(row["lure_id"], -1)
	if lureID != 0 && lureID != 501 && lureID != 502 && lureID != 503 && lureID != 504 && lureID != 505 && lureID != 506 {
		return nil, fmt.Errorf("Unrecognised lure_id value")
	}
	return map[string]any{
		"id":         scope.UserID,
		"profile_no": scope.ProfileNo,
		"ping":       "",
		"template":   getStringValueRule(row["template"], template),
		"distance":   defaultIntRule(row["distance"], 0),
		"clean":      defaultIntRule(row["clean"], 0),
		"lure_id":    lureID,
	}, nil
}

// CleanNestRow normalizes a nest tracking rule for persistence.
func CleanNestRow(cfg *config.Config, scope RuleScope, row map[string]any) (map[string]any, error) {
	template := defaultTemplate(templateName(cfg))
	return map[string]any{
		"id":            scope.UserID,
		"profile_no":    scope.ProfileNo,
		"ping":          "",
		"template":      getStringValueRule(row["template"], template),
		"distance":      defaultIntRule(row["distance"], 0),
		"clean":         defaultIntRule(row["clean"], 0),
		"pokemon_id":    defaultIntRule(row["pokemon_id"], 0),
		"min_spawn_avg": defaultIntRule(row["min_spawn_avg"], 0),
		"form":          defaultIntRule(row["form"], 0),
	}, nil
}

// CleanGymRow normalizes a gym tracking rule for persistence.
func CleanGymRow(cfg *config.Config, scope RuleScope, row map[string]any) (map[string]any, error) {
	template := defaultTemplate(templateName(cfg))
	team := defaultIntRule(row["team"], -1)
	if team < 0 || team > 4 {
		return nil, fmt.Errorf("Invalid team")
	}
	return map[string]any{
		"id":             scope.UserID,
		"profile_no":     scope.ProfileNo,
		"ping":           "",
		"template":       getStringValueRule(row["template"], template),
		"distance":       defaultIntRule(row["distance"], 0),
		"clean":          defaultIntRule(row["clean"], 0),
		"team":           team,
		"slot_changes":   defaultIntRule(row["slot_changes"], 0),
		"battle_changes": defaultIntRule(row["battle_changes"], 0),
		"gym_id":         row["gym_id"],
	}, nil
}

// CleanMaxbattleRow normalizes a max battle tracking rule for persistence.
func CleanMaxbattleRow(cfg *config.Config, scope RuleScope, row map[string]any) (map[string]any, error) {
	template := defaultTemplate(templateName(cfg))
	pokemonID := defaultIntRule(row["pokemon_id"], 0)
	if pokemonID != 9000 && pokemonID <= 0 {
		return nil, fmt.Errorf("Invalid pokemon_id")
	}
	gmax := defaultIntRule(row["gmax"], 0)
	if gmax != 0 && gmax != 1 {
		return nil, fmt.Errorf("Invalid gmax")
	}
	move := defaultIntRule(row["move"], 9000)
	evolution := defaultIntRule(row["evolution"], 9000)
	stationID := strings.TrimSpace(getString(row["station_id"]))
	var stationValue any
	if stationID != "" {
		stationValue = stationID
	}
	return map[string]any{
		"id":         scope.UserID,
		"profile_no": scope.ProfileNo,
		"ping":       "",
		"template":   getStringValueRule(row["template"], template),
		"distance":   defaultIntRule(row["distance"], 0),
		"clean":      defaultIntRule(row["clean"], 0),
		"pokemon_id": pokemonID,
		"gmax":       gmax,
		"level":      defaultIntRule(row["level"], 0),
		"form":       defaultIntRule(row["form"], 0),
		"move":       move,
		"evolution":  evolution,
		"station_id": stationValue,
	}, nil
}

func defaultIntRule(value any, fallback int) int {
	if value == nil {
		return fallback
	}
	if v, ok := numberFromAnyRule(value); ok {
		return v
	}
	return fallback
}

func numberFromAnyRule(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case jsonNumber:
		i, err := strconv.Atoi(string(v))
		if err == nil {
			return i, true
		}
	case string:
		if v == "" {
			return 0, false
		}
		i, err := strconv.Atoi(v)
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func numberFromAnyOrDefaultRule(value any, fallback int) int {
	if v, ok := numberFromAnyRule(value); ok {
		return v
	}
	return fallback
}

func floatFromAnyRule(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func getStringValueRule(value any, fallback string) string {
	if value == nil {
		return fallback
	}
	return fmt.Sprintf("%v", value)
}

type jsonNumber string

func templateName(cfg *config.Config) string {
	if cfg == nil {
		return "1"
	}
	value, _ := cfg.GetString("general.defaultTemplateName")
	return value
}

func raidMaxLevel(util map[string]any) int {
	entry, ok := util["raidLevels"]
	if !ok {
		return 0
	}
	max := 0
	switch v := entry.(type) {
	case map[string]any:
		for key := range v {
			level, err := strconv.Atoi(key)
			if err == nil && level > max {
				max = level
			}
		}
	}
	if max == 0 {
		return 90
	}
	return max
}
