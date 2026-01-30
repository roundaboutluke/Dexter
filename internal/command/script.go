package command

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ScriptCommand outputs a script to recreate tracking.
type ScriptCommand struct{}

func (c *ScriptCommand) Name() string { return "script" }

func (c *ScriptCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "script") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		return tr.TranslateFormat("Valid commands are e.g. `{0}script everything`, `{0}script pokemon raids eggs quest lures invasion nests gym forts`, `{0}script everything allprofiles`, `{0}script everything link`", ctx.Prefix), nil
	}

	everything := containsWord(args, "everything")
	output := []string{}
	defaultTemplate := defaultTemplateName(ctx)

	addProfile := func(profileNo int, targetID string) error {
		if everything || containsWord(args, "pokemon") {
			rows, err := ctx.Query.SelectAllQuery("monsters", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			for _, row := range rows {
				name := "everything"
				if id := toInt(row["pokemon_id"], 0); id != 0 {
					if mon := findMonsterByID(ctx, id, toInt(row["form"], 0)); mon != "" {
						name = strings.ReplaceAll(mon, " ", "_")
					} else {
						name = fmt.Sprintf("%d", id)
					}
				}
				cmd := fmt.Sprintf("%strack %s", ctx.Prefix, name)
				if formID := toInt(row["form"], 0); formID != 0 {
					if mon := findMonsterByKey(ctx, toInt(row["pokemon_id"], 0), formID); mon != nil {
						if formName := strings.TrimSpace(fmt.Sprintf("%v", getMapValue(mon, "form", "name"))); formName != "" {
							cmd += fmt.Sprintf(" form:%s", strings.ReplaceAll(formName, " ", "_"))
						}
					}
				}
				cmd = appendMonsterParams(cmd, row, defaultTemplate)
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "raids") {
			rows, err := ctx.Query.SelectAllQuery("raid", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			teamNames := []string{"harmony", "mystic", "valour", "instinct"}
			for _, row := range rows {
				cmd := fmt.Sprintf("%sraid ", ctx.Prefix)
				if toInt(row["pokemon_id"], 0) == 9000 {
					cmd += fmt.Sprintf("level:%d", toInt(row["level"], 0))
				} else if mon := findMonsterByKey(ctx, toInt(row["pokemon_id"], 0), toInt(row["form"], 0)); mon != nil {
					cmd += strings.ReplaceAll(fmt.Sprintf("%v", mon["name"]), " ", "_")
					if formID := toInt(row["form"], 0); formID != 0 {
						formName := strings.TrimSpace(fmt.Sprintf("%v", getMapValue(mon, "form", "name")))
						if formName != "" {
							cmd += fmt.Sprintf(" form:%s", strings.ReplaceAll(formName, " ", "_"))
						}
					}
				}
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				team := toInt(row["team"], 0)
				if team >= 0 && team < len(teamNames) && team != 4 {
					cmd += fmt.Sprintf(" team:%s", teamNames[team])
				}
				if toInt(row["exclusive"], 0) != 0 {
					cmd += " ex"
				}
				switch toInt(row["rsvp_changes"], 0) {
				case 1:
					cmd += " rsvp"
				case 2:
					cmd += " rsvp_only"
				}
				if toInt(row["clean"], 0) != 0 {
					cmd += " clean"
				}
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "eggs") {
			rows, err := ctx.Query.SelectAllQuery("egg", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			teamNames := []string{"harmony", "mystic", "valour", "instinct"}
			for _, row := range rows {
				cmd := fmt.Sprintf("%segg level:%d", ctx.Prefix, toInt(row["level"], 0))
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				team := toInt(row["team"], 0)
				if team >= 0 && team < len(teamNames) && team != 4 {
					cmd += fmt.Sprintf(" team:%s", teamNames[team])
				}
				if toInt(row["exclusive"], 0) != 0 {
					cmd += " ex"
				}
				switch toInt(row["rsvp_changes"], 0) {
				case 1:
					cmd += " rsvp"
				case 2:
					cmd += " rsvp_only"
				}
				if toInt(row["clean"], 0) != 0 {
					cmd += " clean"
				}
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "invasion") {
			rows, err := ctx.Query.SelectAllQuery("invasion", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			for _, row := range rows {
				cmd := fmt.Sprintf("%sinvasion %s", ctx.Prefix, strings.ReplaceAll(fmt.Sprintf("%v", row["grunt_type"]), " ", "_"))
				if gender := toInt(row["gender"], 0); gender > 0 {
					cmd += fmt.Sprintf(" %s", genderLabel(gender))
				}
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				if toInt(row["clean"], 0) != 0 {
					cmd += " clean"
				}
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "quest") {
			rows, err := ctx.Query.SelectAllQuery("quest", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			for _, row := range rows {
				cmd := fmt.Sprintf("%squest", ctx.Prefix)
				cmd = appendQuestParams(ctx, cmd, row)
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				if toInt(row["clean"], 0) != 0 {
					cmd += " clean"
				}
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "gym") {
			rows, err := ctx.Query.SelectAllQuery("gym", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			teamNames := []string{"uncontested", "mystic", "valor", "instinct"}
			for _, row := range rows {
				team := toInt(row["team"], 0)
				if team < 0 || team >= len(teamNames) {
					team = 0
				}
				cmd := fmt.Sprintf("%sgym %s", ctx.Prefix, teamNames[team])
				if toInt(row["slot_changes"], 0) != 0 {
					cmd += " slot_changes"
				}
				if toInt(row["battle_changes"], 0) != 0 {
					cmd += " battle_changes"
				}
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				if toInt(row["clean"], 0) != 0 {
					cmd += " clean"
				}
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "forts") {
			rows, err := ctx.Query.SelectAllQuery("forts", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			for _, row := range rows {
				cmd := fmt.Sprintf("%sfort %s", ctx.Prefix, row["fort_type"])
				if toInt(row["include_empty"], 1) != 0 {
					cmd += " include_empty"
				}
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "lures") {
			rows, err := ctx.Query.SelectAllQuery("lures", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			lureTypes := map[int]string{
				0:   "everything",
				501: "normal",
				502: "glacial",
				503: "mossy",
				504: "magnetic",
				505: "rainy",
				506: "sparkly",
			}
			for _, row := range rows {
				lureID := toInt(row["lure_id"], 0)
				lureName := lureTypes[lureID]
				if lureName == "" {
					lureName = "everything"
				}
				cmd := fmt.Sprintf("%slure %s", ctx.Prefix, lureName)
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				if toInt(row["clean"], 0) != 0 {
					cmd += " clean"
				}
				output = append(output, cmd)
			}
		}
		if everything || containsWord(args, "nests") {
			rows, err := ctx.Query.SelectAllQuery("nests", map[string]any{"id": targetID, "profile_no": profileNo})
			if err != nil {
				return err
			}
			for _, row := range rows {
				name := "everything"
				if id := toInt(row["pokemon_id"], 0); id != 0 {
					if mon := findMonsterByID(ctx, id, 0); mon != "" {
						name = strings.ReplaceAll(mon, " ", "_")
					} else {
						name = fmt.Sprintf("%d", id)
					}
				}
				cmd := fmt.Sprintf("%snest %s", ctx.Prefix, name)
				if minSpawn := toInt(row["min_spawn_avg"], 0); minSpawn != 0 {
					cmd += fmt.Sprintf(" minspawn:%d", minSpawn)
				}
				cmd = appendTemplateDistance(cmd, row, defaultTemplate)
				if toInt(row["clean"], 0) != 0 {
					cmd += " clean"
				}
				output = append(output, cmd)
			}
		}
		return nil
	}

	if containsWord(args, "allprofiles") {
		profiles, err := ctx.Query.SelectAllQuery("profiles", map[string]any{"id": result.TargetID})
		if err != nil {
			return "", err
		}
		sort.Slice(profiles, func(i, j int) bool {
			return toInt(profiles[i]["profile_no"], 0) < toInt(profiles[j]["profile_no"], 0)
		})
		for _, profile := range profiles {
			name := fmt.Sprintf("%v", profile["name"])
			if name == "" {
				continue
			}
			output = append(output, fmt.Sprintf("%sprofile add %s", ctx.Prefix, name))
			output = append(output, fmt.Sprintf("%sprofile %s", ctx.Prefix, name))
			if lat := formatFloat(getFloat(profile["latitude"])); lat != "0" {
				lon := formatFloat(getFloat(profile["longitude"]))
				output = append(output, fmt.Sprintf("%slocation %s,%s", ctx.Prefix, lat, lon))
			}
			areas := parseAreaList(profile["area"])
			if len(areas) > 0 {
				for i, area := range areas {
					areas[i] = strings.ReplaceAll(area, " ", "_")
				}
				output = append(output, fmt.Sprintf("%sarea add %s", ctx.Prefix, strings.Join(areas, " ")))
			}
			if err := addProfile(toInt(profile["profile_no"], 1), result.TargetID); err != nil {
				return "", err
			}
		}
	} else {
		if err := addProfile(result.ProfileNo, result.TargetID); err != nil {
			return "", err
		}
	}

	if len(output) == 0 {
		return tr.Translate("The script specified is empty", false), nil
	}
	message := strings.Join(output, "\n")
	if containsWord(args, "link") {
		if link, err := createHastebinLink(message); err == nil && link != "" {
			return fmt.Sprintf("%s %s", tr.Translate("Your backup is at", false), link), nil
		}
		return tr.Translate("Hastebin seems down", false), nil
	}
	reply := buildFileReply(fmt.Sprintf("%s.txt", result.Target.Name), tr.Translate("Your backup", false), message)
	if reply != "" {
		return reply, nil
	}
	return message, nil
}

func findMonsterByID(ctx *Context, id int, form int) string {
	for _, raw := range ctx.Data.Monsters {
		if mon, ok := raw.(map[string]any); ok {
			if toInt(mon["id"], 0) != id {
				continue
			}
			if formData, ok := mon["form"].(map[string]any); ok {
				if toInt(formData["id"], 0) != form {
					continue
				}
			}
			return fmt.Sprintf("%v", mon["name"])
		}
	}
	return ""
}

func findMonsterByKey(ctx *Context, id int, form int) map[string]any {
	if ctx == nil || ctx.Data == nil || ctx.Data.Monsters == nil {
		return nil
	}
	key := fmt.Sprintf("%d_%d", id, form)
	if raw, ok := ctx.Data.Monsters[key]; ok {
		if mon, ok := raw.(map[string]any); ok {
			return mon
		}
	}
	return nil
}

func getMapValue(mon map[string]any, key, subkey string) any {
	if mon == nil {
		return nil
	}
	raw, ok := mon[key].(map[string]any)
	if !ok {
		return nil
	}
	return raw[subkey]
}

func appendTemplateDistance(cmd string, row map[string]any, defaultTemplate string) string {
	if template := fmt.Sprintf("%v", row["template"]); template != "" && template != defaultTemplate {
		cmd += fmt.Sprintf(" template:%s", template)
	}
	if distance := toInt(row["distance"], 0); distance != 0 {
		cmd += fmt.Sprintf(" d:%d", distance)
	}
	return cmd
}

func appendMonsterParams(cmd string, row map[string]any, defaultTemplate string) string {
	defaults := map[string]any{
		"min_iv":     -1,
		"max_iv":     100,
		"min_cp":     0,
		"max_cp":     9000,
		"min_level":  0,
		"max_level":  55,
		"atk":        0,
		"def":        0,
		"sta":        0,
		"max_atk":    15,
		"max_def":    15,
		"max_sta":    15,
		"min_weight": 0,
		"max_weight": 9000000,
		"rarity":     -1,
		"max_rarity": 6,
		"min_time":   0,
		"template":   defaultTemplate,
		"distance":   0,
	}
	keyMap := map[string]string{
		"min_iv":     "iv",
		"max_iv":     "maxiv",
		"min_cp":     "mincp",
		"max_cp":     "maxcp",
		"min_level":  "level",
		"max_level":  "maxlevel",
		"atk":        "atk",
		"def":        "def",
		"sta":        "sta",
		"max_atk":    "maxatk",
		"max_def":    "maxdef",
		"max_sta":    "maxsta",
		"min_weight": "weight",
		"max_weight": "maxweight",
		"rarity":     "rarity",
		"max_rarity": "maxrarity",
		"min_time":   "t",
		"template":   "template",
		"distance":   "d",
	}
	for field, param := range keyMap {
		value := row[field]
		if field == "template" {
			value = fmt.Sprintf("%v", row["template"])
		}
		if !valueDiff(value, defaults[field]) {
			continue
		}
		cmd += fmt.Sprintf(" %s:%v", param, value)
	}
	if toInt(row["clean"], 0) != 0 {
		cmd += " clean"
	}
	if gender := toInt(row["gender"], 0); gender > 0 {
		cmd += fmt.Sprintf(" %s", genderLabel(gender))
	}
	if league := toInt(row["pvp_ranking_league"], 0); league != 0 {
		leagueName := map[int]string{500: "little", 1500: "great", 2500: "ultra"}[league]
		if leagueName != "" {
			cmd += fmt.Sprintf(" %s:%d %scp:%d", leagueName, toInt(row["pvp_ranking_worst"], 0), leagueName, toInt(row["pvp_ranking_min_cp"], 0))
			if best := toInt(row["pvp_ranking_best"], 1); best > 1 {
				cmd += fmt.Sprintf(" %shigh:%d", leagueName, best)
			}
		}
	}
	return cmd
}

func appendQuestParams(ctx *Context, cmd string, row map[string]any) string {
	rewardType := toInt(row["reward_type"], 0)
	reward := toInt(row["reward"], 0)
	switch rewardType {
	case 3:
		if reward > 0 {
			cmd += fmt.Sprintf(" stardust:%d", reward)
		} else {
			cmd += " stardust"
		}
	case 12:
		if reward > 0 {
			if mon := findMonsterByKey(ctx, reward, 0); mon != nil {
				cmd += fmt.Sprintf(" energy:%s", strings.ReplaceAll(fmt.Sprintf("%v", mon["name"]), " ", "_"))
			} else {
				cmd += " energy"
			}
		} else {
			cmd += " energy"
		}
	case 4:
		if reward > 0 {
			if mon := findMonsterByKey(ctx, reward, 0); mon != nil {
				cmd += fmt.Sprintf(" candy:%s", strings.ReplaceAll(fmt.Sprintf("%v", mon["name"]), " ", "_"))
			} else {
				cmd += " candy"
			}
		} else {
			cmd += " candy"
		}
	case 7:
		if reward > 0 {
			mon := findMonsterByKey(ctx, reward, toInt(row["form"], 0))
			if mon != nil {
				cmd += fmt.Sprintf(" %s", strings.ReplaceAll(fmt.Sprintf("%v", mon["name"]), " ", "_"))
				if formID := toInt(row["form"], 0); formID != 0 {
					formName := strings.TrimSpace(fmt.Sprintf("%v", getMapValue(mon, "form", "name")))
					if formName != "" {
						cmd += fmt.Sprintf(" form:%s", strings.ReplaceAll(formName, " ", "_"))
					}
				}
			}
		}
	case 2:
		if ctx != nil && ctx.Data != nil && ctx.Data.Items != nil {
			if item, ok := ctx.Data.Items[strconv.Itoa(reward)].(map[string]any); ok {
				if name := strings.TrimSpace(fmt.Sprintf("%v", item["name"])); name != "" {
					cmd += fmt.Sprintf(" %s", strings.ReplaceAll(name, " ", "_"))
				}
			}
		}
	}
	if toInt(row["shiny"], 0) != 0 {
		cmd += " shiny"
	}
	return cmd
}

func valueDiff(value any, expected any) bool {
	switch v := expected.(type) {
	case int:
		return toInt(value, 0) != v
	case string:
		return fmt.Sprintf("%v", value) != v
	default:
		return fmt.Sprintf("%v", value) != fmt.Sprintf("%v", expected)
	}
}

func genderLabel(gender int) string {
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

func parseAreaList(raw any) []string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		var areas []string
		if err := json.Unmarshal([]byte(v), &areas); err == nil {
			return areas
		}
	case []any:
		areas := make([]string, 0, len(v))
		for _, item := range v {
			areas = append(areas, fmt.Sprintf("%v", item))
		}
		return areas
	}
	return nil
}
