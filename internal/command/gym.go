package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// GymCommand handles gym tracking.
type GymCommand struct{}

func (c *GymCommand) Name() string { return "gym" }

func (c *GymCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "gym") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	disallowGymBattle := true
	if value, ok := ctx.Config.GetBool("tracking.enableGymBattle"); ok {
		disallowGymBattle = !value
	}
	if disallowGymBattle && (containsPhrase(args, "battle changes") || containsWord(args, "battle_changes")) {
		return tr.Translate("You do not have permission to use this option: battle changes", false), nil
	}

	if len(args) == 0 {
		tipMsg := tr.TranslateFormat("Valid commands are e.g. `{0}gym everything`, `{0}gym mystic slot_changes`", ctx.Prefix)
		if !disallowGymBattle {
			tipMsg = tr.TranslateFormat("Valid commands are e.g. `{0}gym everything`, `{0}gym mystic slot_changes`, `{0}gym valor battle_changes`", ctx.Prefix)
			lines := []string{tipMsg}
			if helpLine := singleLineHelpText(ctx, "gym", result.Language, result.Target); helpLine != "" {
				lines = append(lines, helpLine)
			}
			return strings.Join(lines, "\n"), nil
		}
		lines := []string{tipMsg}
		if helpLine := singleLineHelpText(ctx, "gym", result.Language, result.Target); helpLine != "" {
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

	teams := parseGymTeams(args)
	slotChanges := boolToInt(containsPhrase(args, "slot changes") || containsWord(args, "slot_changes"))
	battleChanges := boolToInt(containsPhrase(args, "battle changes") || containsWord(args, "battle_changes"))
	gymID := parseGymID(args)

	if len(teams) == 0 {
		return prependWarning(warning, tr.Translate("404 No team types found", false)), nil
	}

	if remove {
		total := int64(0)
		if len(teams) > 0 {
			removed, err := ctx.Query.DeleteWhereInQuery("gym", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			}, toAnySliceInt(teams), "team")
			if err != nil {
				return "", err
			}
			total += removed
		}
		if commandEverything {
			removed, err := ctx.Query.DeleteQuery("gym", map[string]any{
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
	for _, team := range teams {
		rows = append(rows, map[string]any{
			"id":             result.TargetID,
			"profile_no":     result.ProfileNo,
			"ping":           ctx.Ping,
			"team":           team,
			"template":       template,
			"distance":       distance,
			"clean":          boolToInt(clean),
			"slot_changes":   slotChanges,
			"battle_changes": battleChanges,
			"gym_id":         gymID,
		})
	}

	trackedRows, err := ctx.Query.SelectAllQuery("gym", map[string]any{
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
			if toInt(existing["team"], 0) != toInt(candidate["team"], 0) {
				continue
			}
			diffs := gymDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && gymDiffIsUpdate(diffs) {
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
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.GymRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.GymRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.GymRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("gym", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("gym", append(insert, updates...)); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

func parseGymID(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(strings.ToLower(arg), "gym:") {
			return strings.TrimSpace(arg[4:])
		}
	}
	return ""
}

func parseGymTeams(args []string) []int {
	teams := []int{}
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "normal":
			teams = append(teams, 501)
		case "instinct", "yellow":
			teams = append(teams, 3)
		case "valor", "red":
			teams = append(teams, 2)
		case "mystic", "blue":
			teams = append(teams, 1)
		case "harmony", "gray", "grey", "uncontested":
			teams = append(teams, 0)
		case "everything":
			teams = append(teams, 0, 1, 2, 3)
		}
	}
	return uniqueInts(teams)
}

func gymDiffKeys(existing map[string]any, desired map[string]any) []string {
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

func gymDiffIsUpdate(diffs []string) bool {
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
	return other == "distance" || other == "template" || other == "clean" || other == "slot_changes" || other == "battle_changes"
}
