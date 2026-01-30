package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// RaidCommand handles raid tracking.
type RaidCommand struct{}

func (c *RaidCommand) Name() string { return "raid" }

func (c *RaidCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "raid") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}raid level5`, `{0}raid articuno`, `{0}raid remove everything`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "raid", result.Language, result.Target); helpLine != "" {
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
	levels := parseLevels(ctx, args, re)
	if len(monsters) == 0 && len(levels) == 0 {
		return prependWarning(warning, tr.Translate("404 no valid tracks found", false)), nil
	}

	team := parseTeam(args)
	exclusive := boolToInt(containsWord(args, "ex"))
	rsvpChanges := parseRSVP(args)
	moveID, moveErr := parseRaidMove(ctx, args, re)
	if moveErr != "" {
		return moveErr, nil
	}
	gymID := strings.TrimSpace(parseGymID(args))
	var gymValue any
	if gymID != "" {
		gymValue = gymID
	}

	if remove {
		total := int64(0)
		if len(monsters) > 0 {
			ids := make([]int, 0, len(monsters))
			for _, mon := range monsters {
				ids = append(ids, mon.ID)
			}
			removed, err := ctx.Query.DeleteWhereInQuery("raid", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			}, toAnySliceInt(ids), "pokemon_id")
			if err != nil {
				return "", err
			}
			total += removed
		}
		if len(levels) > 0 {
			removed, err := ctx.Query.DeleteWhereInQuery("raid", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			}, toAnySliceInt(levels), "level")
			if err != nil {
				return "", err
			}
			total += removed
		}
		if commandEverything {
			removed, err := ctx.Query.DeleteQuery("raid", map[string]any{
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
			"id":           result.TargetID,
			"profile_no":   result.ProfileNo,
			"pokemon_id":   mon.ID,
			"ping":         ctx.Ping,
			"exclusive":    exclusive,
			"template":     template,
			"distance":     distance,
			"team":         team,
			"clean":        boolToInt(clean),
			"level":        9000,
			"form":         mon.FormID,
			"evolution":    9000,
			"move":         moveID,
			"gym_id":       gymValue,
			"rsvp_changes": rsvpChanges,
		})
	}
	for _, level := range levels {
		rows = append(rows, map[string]any{
			"id":           result.TargetID,
			"profile_no":   result.ProfileNo,
			"pokemon_id":   9000,
			"ping":         ctx.Ping,
			"exclusive":    exclusive,
			"template":     template,
			"distance":     distance,
			"team":         team,
			"clean":        boolToInt(clean),
			"level":        level,
			"form":         0,
			"evolution":    9000,
			"move":         moveID,
			"gym_id":       gymValue,
			"rsvp_changes": rsvpChanges,
		})
	}

	trackedRows, err := ctx.Query.SelectAllQuery("raid", map[string]any{
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
			diffs := raidDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && raidDiffIsUpdate(diffs) {
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
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.RaidRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.RaidRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.RaidRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("raid", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("raid", append(insert, updates...)); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

type raidMonster struct {
	ID     int
	FormID int
}

func parseRaidFormNames(ctx *Context, args []string, re *RegexSet) []string {
	formNames := []string{}
	for _, arg := range args {
		if !re.Form.MatchString(arg) {
			continue
		}
		match := re.Form.FindStringSubmatch(arg)
		if len(match) < 3 {
			continue
		}
		form := strings.ToLower(ctx.I18n.ReverseTranslateCommand(match[2], true))
		if form != "" {
			formNames = append(formNames, form)
		}
	}
	return formNames
}

func parseRaidTypes(ctx *Context, args []string) []string {
	typeLookup := map[string]bool{}
	if ctx != nil && ctx.Data != nil {
		if raw, ok := ctx.Data.UtilData["types"].(map[string]any); ok {
			for key := range raw {
				typeLookup[strings.ToLower(key)] = true
			}
		}
		if len(typeLookup) == 0 {
			for key := range ctx.Data.Types {
				typeLookup[strings.ToLower(key)] = true
			}
		}
	}
	out := []string{}
	for _, arg := range args {
		if typeLookup[strings.ToLower(arg)] {
			out = append(out, strings.ToLower(arg))
		}
	}
	return uniqueStrings(out)
}

func selectRaidMonsters(ctx *Context, args []string, formNames []string, argTypes []string, includeEverything bool) []raidMonster {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	argLookup := map[string]bool{}
	for _, arg := range args {
		argLookup[strings.ToLower(arg)] = true
	}
	formLookup := map[string]bool{}
	for _, name := range formNames {
		formLookup[strings.ToLower(name)] = true
	}
	typeLookup := map[string]bool{}
	for _, typ := range argTypes {
		typeLookup[strings.ToLower(typ)] = true
	}

	out := []raidMonster{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := toInt(mon["id"], 0)
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		formName := strings.ToLower(fmt.Sprintf("%v", form["name"]))
		formID := toInt(form["id"], 0)

		nameMatch := argLookup[name] || argLookup[fmt.Sprintf("%d", id)]
		typeMatch := false
		if len(typeLookup) > 0 {
			if types, ok := mon["types"].([]any); ok {
				for _, entry := range types {
					if m, ok := entry.(map[string]any); ok {
						typeName := strings.ToLower(fmt.Sprintf("%v", m["name"]))
						if typeLookup[typeName] {
							typeMatch = true
							break
						}
					}
				}
			}
		}

		if len(formLookup) > 0 {
			if (nameMatch || typeMatch || includeEverything) && formLookup[formName] {
				out = append(out, raidMonster{ID: id, FormID: formID})
			}
			continue
		}
		if (nameMatch || typeMatch) && formID == 0 {
			out = append(out, raidMonster{ID: id, FormID: formID})
		}
	}
	return out
}

func parseRaidGenRange(ctx *Context, args []string, re *RegexSet) (int, int, bool) {
	for _, arg := range args {
		if !re.Gen.MatchString(arg) {
			continue
		}
		match := re.Gen.FindStringSubmatch(arg)
		if len(match) < 3 {
			continue
		}
		genKey := match[2]
		if ctx == nil || ctx.Data == nil {
			return 0, 0, false
		}
		raw, ok := ctx.Data.UtilData["genData"].(map[string]any)
		if !ok {
			return 0, 0, false
		}
		entry, ok := raw[genKey]
		if !ok {
			entry, ok = raw[fmt.Sprintf("%d", toInt(genKey, 0))]
		}
		gen, ok := entry.(map[string]any)
		if !ok {
			return 0, 0, false
		}
		min := toInt(gen["min"], 0)
		max := toInt(gen["max"], 0)
		if min > 0 && max > 0 {
			return min, max, true
		}
		return 0, 0, false
	}
	return 0, 0, false
}

func parseRaidMove(ctx *Context, args []string, re *RegexSet) (int, string) {
	for _, arg := range args {
		if !re.Move.MatchString(arg) {
			continue
		}
		match := re.Move.FindStringSubmatch(arg)
		if len(match) < 3 {
			continue
		}
		moveText := strings.ToLower(match[2])
		parts := strings.Split(moveText, "/")
		moveName := ctx.I18n.ReverseTranslateCommand(strings.TrimSpace(parts[0]), true)
		moveType := ""
		if len(parts) > 1 {
			moveType = ctx.I18n.ReverseTranslateCommand(strings.TrimSpace(parts[1]), true)
		}
		for key, raw := range ctx.Data.Moves {
			move, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name := strings.ToLower(fmt.Sprintf("%v", move["name"]))
			typ := strings.ToLower(fmt.Sprintf("%v", move["type"]))
			if name == strings.ToLower(moveName) && (moveType == "" || typ == strings.ToLower(moveType)) {
				return toInt(key, 9000), ""
			}
		}
		return 9000, fmt.Sprintf("Unrecognised move name %s", strings.TrimSpace(match[2]))
	}
	return 9000, ""
}

func containsWord(args []string, word string) bool {
	for _, arg := range args {
		if strings.EqualFold(arg, word) {
			return true
		}
		if strings.EqualFold(word, "remove") && strings.EqualFold(arg, "delete") {
			return true
		}
	}
	return false
}

func parseLevels(ctx *Context, args []string, re *RegexSet) []int {
	levels := []int{}
	if containsWord(args, "everything") && ctx != nil {
		if raw, ok := ctx.Data.UtilData["raidLevels"].(map[string]any); ok {
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

func parseTeam(args []string) int {
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "instinct", "yellow":
			return 3
		case "valor", "red":
			return 2
		case "mystic", "blue":
			return 1
		case "harmony", "gray", "grey", "uncontested", "white":
			return 0
		}
	}
	return 4
}

func parseRSVP(args []string) int {
	joined := strings.ToLower(strings.Join(args, " "))
	if strings.Contains(joined, "rsvp only") {
		return 2
	}
	if strings.Contains(joined, "no rsvp") {
		return 0
	}
	if strings.Contains(joined, "rsvp") {
		return 1
	}
	return 0
}

func raidDiffKeys(existing map[string]any, desired map[string]any) []string {
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

func raidDiffIsUpdate(diffs []string) bool {
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
	return other == "distance" || other == "template" || other == "clean" || other == "gym_id"
}
