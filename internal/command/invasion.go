package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// InvasionCommand handles grunt tracking.
type InvasionCommand struct{}

func (c *InvasionCommand) Name() string { return "invasion" }

func (c *InvasionCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "invasion") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}incident giovanni`, `{0}incident dragon`, `{0}incident remove everything`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "invasion", result.Language, result.Target); helpLine != "" {
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

	types := parseGruntTypes(ctx, args)
	gender := parseGender(args)
	if len(types) == 0 {
		return prependWarning(warning, tr.Translate("404 No valid invasion types found", false)), nil
	}

	if remove {
		if commandEverything {
			removed, err := ctx.Query.DeleteQuery("invasion", map[string]any{
				"id":         result.TargetID,
				"profile_no": result.ProfileNo,
			})
			if err != nil {
				return "", err
			}
			if removed == 1 {
				return prependWarning(warning, fmt.Sprintf("%s, %s", tr.Translate("I removed 1 entry", false), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
			}
			return prependWarning(warning, fmt.Sprintf("%s, %s", tr.TranslateFormat("I removed {0} entries", removed), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
		}
		total := int64(0)
		removed, err := ctx.Query.DeleteWhereInQuery("invasion", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, toAnySlice(types), "grunt_type")
		if err != nil {
			return "", err
		}
		total += removed
		if total == 1 {
			return prependWarning(warning, fmt.Sprintf("%s, %s", tr.Translate("I removed 1 entry", false), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
		}
		return prependWarning(warning, fmt.Sprintf("%s, %s", tr.TranslateFormat("I removed {0} entries", total), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
	}

	rows := []map[string]any{}
	for _, t := range types {
		row := map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
			"ping":       ctx.Ping,
			"grunt_type": t,
			"gender":     gender,
			"template":   template,
			"distance":   distance,
			"clean":      boolToInt(clean),
		}
		rows = append(rows, row)
	}

	trackedRows, err := ctx.Query.SelectAllQuery("invasion", map[string]any{
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
		candidateType := strings.ToLower(fmt.Sprintf("%v", candidate["grunt_type"]))
		for _, existing := range trackedRows {
			if strings.ToLower(fmt.Sprintf("%v", existing["grunt_type"])) != candidateType {
				continue
			}
			diffs := invasionDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && invasionDiffIsUpdate(diffs) {
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
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.InvasionRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.InvasionRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.InvasionRowText(ctx.Config, tr, ctx.Data, row))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("invasion", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("invasion", append(insert, updates...)); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

func parseGender(args []string) int {
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "male":
			return 1
		case "female":
			return 2
		case "genderless":
			return 3
		}
	}
	return 0
}

func parseGruntTypes(ctx *Context, args []string) []string {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	lookup := map[string]string{}
	for _, raw := range ctx.Data.Grunts {
		if entry, ok := raw.(map[string]any); ok {
			if typ, ok := entry["type"].(string); ok && typ != "" {
				lookup[strings.ToLower(typ)] = typ
			}
			if grunt, ok := entry["grunt"].(string); ok && grunt != "" {
				if _, exists := lookup[strings.ToLower(grunt)]; !exists {
					lookup[strings.ToLower(grunt)] = grunt
				}
			}
		}
	}
	if ctx.Data.UtilData != nil {
		if raw, ok := ctx.Data.UtilData["pokestopEvent"].(map[string]any); ok {
			for _, value := range raw {
				if entry, ok := value.(map[string]any); ok {
					if name, ok := entry["name"].(string); ok && name != "" {
						lookup[strings.ToLower(name)] = name
					}
				}
			}
		}
	}
	types := []string{}
	if containsWord(args, "everything") {
		all := []string{}
		for _, value := range lookup {
			all = append(all, value)
		}
		return uniqueStrings(all)
	}
	for _, arg := range args {
		if mapped, ok := lookup[strings.ToLower(arg)]; ok {
			types = append(types, mapped)
		}
	}
	return uniqueStrings(types)
}

func toAnySlice(values []string) []any {
	out := make([]any, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func extractUids(rows []map[string]any) []any {
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		if uid, ok := row["uid"]; ok {
			out = append(out, uid)
		}
	}
	return out
}

func invasionDiffKeys(existing map[string]any, desired map[string]any) []string {
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

func invasionDiffIsUpdate(diffs []string) bool {
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
