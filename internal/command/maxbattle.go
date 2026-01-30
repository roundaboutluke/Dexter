package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// MaxbattleCommand handles max battle tracking.
type MaxbattleCommand struct{}

func (c *MaxbattleCommand) Name() string { return "maxbattle" }

func (c *MaxbattleCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "maxbattle") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}maxbattle level3`, `{0}maxbattle pikachu`, `{0}maxbattle gmax level7`, `{0}maxbattle remove everything`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "maxbattle", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	remove := containsWord(args, "remove")
	commandEverything := containsWord(args, "everything")
	distance, args := parseDistance(args, re)
	template, args := parseTemplate(args, re)
	clean, args := parseClean(args)
	if template == "" {
		template = defaultTemplateName(ctx)
	}
	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, remove)
	if errMsg != "" {
		return errMsg, nil
	}

	args = expandPokemonAliases(ctx, args)
	formNames := parseRaidFormNames(ctx, args, re)
	argTypes := parseRaidTypes(ctx, args)
	monsters := selectRaidMonsters(ctx, args, formNames, argTypes, commandEverything)
	if min, max, ok := parseRaidGenRange(ctx, args, re); ok {
		filtered := []raidMonster{}
		for _, mon := range monsters {
			if mon.ID >= min && mon.ID <= max {
				filtered = append(filtered, mon)
			}
		}
		monsters = filtered
	}
	levels := parseMaxbattleLevels(ctx, args, re)
	if len(monsters) == 0 && len(levels) == 0 {
		return prependWarning(warning, tr.Translate("404 no valid tracks found", false)), nil
	}

	gmax := boolToInt(containsWord(args, "gmax") || containsWord(args, "gigantamax"))
	moveID, moveErr := parseRaidMove(ctx, args, re)
	if moveErr != "" {
		return moveErr, nil
	}
	stationID := strings.TrimSpace(parseStationID(args))
	var stationValue any
	if stationID != "" {
		stationValue = stationID
	}

	if remove {
		total := int64(0)
		if len(monsters) > 0 {
			ids := make([]int, 0, len(monsters))
			for _, mon := range monsters {
				ids = append(ids, mon.ID)
			}
			removed, err := ctx.Query.DeleteWhereInQuery("maxbattle", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			}, toAnySliceInt(ids), "pokemon_id")
			if err != nil {
				return "", err
			}
			total += removed
		}
		if len(levels) > 0 {
			removed, err := ctx.Query.DeleteWhereInQuery("maxbattle", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			}, toAnySliceInt(levels), "level")
			if err != nil {
				return "", err
			}
			total += removed
		}
		if commandEverything {
			removed, err := ctx.Query.DeleteQuery("maxbattle", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			})
			if err != nil {
				return "", err
			}
			total += removed
		}
		if total == 1 {
			return prependWarning(warning, fmt.Sprintf("%s, %s", tr.Translate("I removed 1 entry", false), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
		}
		return prependWarning(warning, fmt.Sprintf("%s, %s", tr.TranslateFormat("I removed {0} entries", total), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
	}

	rows := []map[string]any{}
	for _, mon := range monsters {
		rows = append(rows, map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
			"pokemon_id": mon.ID,
			"ping":       ctx.Ping,
			"template":   template,
			"distance":   distance,
			"clean":      boolToInt(clean),
			"gmax":       gmax,
			"level":      0,
			"form":       mon.FormID,
			"evolution":  9000,
			"move":       moveID,
			"station_id": stationValue,
		})
	}
	for _, level := range levels {
		rows = append(rows, map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
			"pokemon_id": 9000,
			"ping":       ctx.Ping,
			"template":   template,
			"distance":   distance,
			"clean":      boolToInt(clean),
			"gmax":       gmax,
			"level":      level,
			"form":       0,
			"evolution":  9000,
			"move":       moveID,
			"station_id": stationValue,
		})
	}

	trackedRows, err := ctx.Query.SelectAllQuery("maxbattle", map[string]any{
		"id":         result.TargetID,
		"profile_no": result.ProfileNo,
	})
	if err != nil {
		return "", err
	}

	insert := append([]map[string]any{}, rows...)
	updates := []map[string]any{}
	unchanged := []map[string]any{}
	for i := len(insert) - 1; i >= 0; i-- {
		candidate := insert[i]
		for _, existing := range trackedRows {
			if toInt(existing["pokemon_id"], 0) != toInt(candidate["pokemon_id"], 0) {
				continue
			}
			if toInt(existing["level"], 0) != toInt(candidate["level"], 0) {
				continue
			}
			if toInt(existing["gmax"], 0) != toInt(candidate["gmax"], 0) {
				continue
			}
			if toInt(existing["form"], 0) != toInt(candidate["form"], 0) {
				continue
			}
			if toInt(existing["move"], 0) != toInt(candidate["move"], 0) {
				continue
			}
			if strings.TrimSpace(fmt.Sprintf("%v", existing["station_id"])) != strings.TrimSpace(fmt.Sprintf("%v", candidate["station_id"])) {
				continue
			}
			diffs := maxbattleDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && maxbattleDiffIsUpdate(diffs) {
				updated := map[string]any{}
				for key, value := range candidate {
					updated[key] = value
				}
				updated["uid"] = existing["uid"]
				updates = append(updates, updated)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
		}
	}

	message := ""
	if len(unchanged)+len(updates)+len(insert) > 50 {
		message = tr.TranslateFormat("I have made a lot of changes. See {0}{1} for details", ctx.Prefix, tr.Translate("tracked", true))
	} else {
		for _, row := range unchanged {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.MaxbattleRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.MaxbattleRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.MaxbattleRowText(ctx.Config, tr, ctx.Data, row))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("maxbattle", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("maxbattle", append(insert, updates...)); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

func parseStationID(args []string) string {
	for _, arg := range args {
		lower := strings.ToLower(arg)
		if strings.HasPrefix(lower, "station:") {
			return strings.TrimSpace(arg[len("station:"):])
		}
		if strings.HasPrefix(lower, "stationid:") {
			return strings.TrimSpace(arg[len("stationid:"):])
		}
	}
	return ""
}

func parseMaxbattleLevels(ctx *Context, args []string, re *RegexSet) []int {
	levels := []int{}
	if containsWord(args, "everything") && ctx != nil {
		if raw, ok := ctx.Data.UtilData["maxbattleLevels"].(map[string]any); ok {
			for key := range raw {
				if value := toInt(key, 0); value > 0 {
					levels = append(levels, value)
				}
			}
		}
		if len(levels) > 0 {
			return levels
		}
	}
	for _, arg := range args {
		if min, max, ok := parseRange(arg, re.Level); ok {
			for value := min; value <= max; value++ {
				if value > 0 {
					levels = append(levels, value)
				}
			}
		}
	}
	return levels
}

func maxbattleDiffKeys(existing map[string]any, desired map[string]any) []string {
	diffs := []string{}
	if _, ok := existing["uid"]; ok {
		diffs = append(diffs, "uid")
	}
	for key, desiredValue := range desired {
		if key == "uid" {
			continue
		}
		if fmt.Sprintf("%v", existing[key]) != fmt.Sprintf("%v", desiredValue) {
			diffs = append(diffs, key)
		}
	}
	return diffs
}

func maxbattleDiffIsUpdate(diffs []string) bool {
	if len(diffs) != 2 {
		return false
	}
	if diffs[0] != "uid" && diffs[1] != "uid" {
		return false
	}
	other := diffs[0]
	if other == "uid" {
		other = diffs[1]
	}
	return other == "distance" || other == "template" || other == "clean"
}
