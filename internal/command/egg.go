package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// EggCommand handles egg tracking.
type EggCommand struct{}

func (c *EggCommand) Name() string { return "egg" }

func (c *EggCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "egg") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}egg level5`, `{0}egg remove everything`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "egg", result.Language, result.Target); helpLine != "" {
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

	levels := parseLevels(ctx, args, re)
	if len(levels) == 0 {
		return prependWarning(warning, tr.Translate("404 No raid egg levels found", false)), nil
	}

	team := parseTeam(args)
	exclusive := boolToInt(containsWord(args, "ex"))
	rsvpChanges := parseRSVP(args)
	gymID := strings.TrimSpace(parseGymID(args))
	var gymValue any
	if gymID != "" {
		gymValue = gymID
	}

	if remove {
		total := int64(0)
		if len(levels) > 0 {
			removed, err := ctx.Query.DeleteWhereInQuery("egg", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			}, toAnySliceInt(levels), "level")
			if err != nil {
				return "", err
			}
			total += removed
		}
		if commandEverything {
			removed, err := ctx.Query.DeleteQuery("egg", map[string]any{
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
	for _, level := range levels {
		rows = append(rows, map[string]any{
			"id":           result.TargetID,
			"profile_no":   result.ProfileNo,
			"level":        level,
			"team":         team,
			"ping":         ctx.Ping,
			"exclusive":    exclusive,
			"template":     template,
			"distance":     distance,
			"clean":        boolToInt(clean),
			"gym_id":       gymValue,
			"rsvp_changes": rsvpChanges,
		})
	}

	trackedRows, err := ctx.Query.SelectAllQuery("egg", map[string]any{
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
			if toInt(existing["level"], 0) != toInt(candidate["level"], 0) {
				continue
			}
			diffs := eggDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && eggDiffIsUpdate(diffs) {
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
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.EggRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.EggRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.EggRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("egg", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("egg", append(insert, updates...)); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

func eggDiffKeys(existing map[string]any, desired map[string]any) []string {
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

func eggDiffIsUpdate(diffs []string) bool {
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
