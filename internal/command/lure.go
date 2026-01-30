package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// LureCommand handles lure tracking.
type LureCommand struct{}

func (c *LureCommand) Name() string { return "lure" }

func (c *LureCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "lure") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}lure mossy`, `{0}lure remove everything`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "lure", result.Language, result.Target); helpLine != "" {
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

	lureIDs := parseLureIDs(ctx, args)
	if len(lureIDs) == 0 {
		return prependWarning(warning, tr.Translate("404 no valid tracks found", false)), nil
	}

	if remove {
		total := int64(0)
		if len(lureIDs) > 0 {
			removed, err := ctx.Query.DeleteWhereInQuery("lures", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			}, toAnySliceInt(lureIDs), "lure_id")
			if err != nil {
				return "", err
			}
			total += removed
		}
		if commandEverything {
			removed, err := ctx.Query.DeleteQuery("lures", map[string]any{
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
	for _, lureID := range lureIDs {
		rows = append(rows, map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
			"ping":       ctx.Ping,
			"lure_id":    lureID,
			"template":   template,
			"distance":   distance,
			"clean":      boolToInt(clean),
		})
	}

	trackedRows, err := ctx.Query.SelectAllQuery("lures", map[string]any{
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
			if toInt(existing["lure_id"], 0) != toInt(candidate["lure_id"], 0) {
				continue
			}
			diffs := lureDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && lureDiffIsUpdate(diffs) {
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
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.LureRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.LureRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.LureRowText(ctx.Config, tr, ctx.Data, row))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("lures", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("lures", append(insert, updates...)); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

func parseLureIDs(ctx *Context, args []string) []int {
	lureMap := map[string]int{
		"normal":   501,
		"glacial":  502,
		"mossy":    503,
		"magnetic": 504,
		"rainy":    505,
		"sparkly":  506,
		"golden":   506,
	}
	lureIDs := []int{}
	if containsWord(args, "everything") || containsWord(args, "any") {
		lureIDs = append(lureIDs, 0)
	}
	if ctx != nil && ctx.Data != nil && ctx.Data.UtilData != nil {
		if raw, ok := ctx.Data.UtilData["lures"].(map[string]any); ok {
			for key, entry := range raw {
				if m, ok := entry.(map[string]any); ok {
					name := strings.ToLower(fmt.Sprintf("%v", m["name"]))
					short := strings.TrimSuffix(name, " lure")
					lureMap[name] = toInt(key, 0)
					lureMap[short] = toInt(key, 0)
				}
			}
		}
	}
	for _, arg := range args {
		if id, ok := lureMap[strings.ToLower(arg)]; ok {
			lureIDs = append(lureIDs, id)
		}
	}
	return uniqueInts(lureIDs)
}

func toAnySliceInt(values []int) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func lureDiffKeys(existing map[string]any, desired map[string]any) []string {
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

func lureDiffIsUpdate(diffs []string) bool {
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
