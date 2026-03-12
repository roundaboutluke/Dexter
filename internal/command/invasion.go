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
	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, remove, false)
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
			if removed > 0 {
				ctx.MarkAlertStateDirty()
			}
			return prependWarning(warning, trackedRemovalMessage(ctx, tr, removed)), nil
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
		if removed > 0 {
			ctx.MarkAlertStateDirty()
		}
		return prependWarning(warning, trackedRemovalMessage(ctx, tr, total)), nil
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

	plan := tracking.PlanUpsert(rows, trackedRows, func(candidate, existing map[string]any) bool {
		return strings.EqualFold(fmt.Sprintf("%v", existing["grunt_type"]), fmt.Sprintf("%v", candidate["grunt_type"]))
	}, "distance", "template", "clean")
	message := tracking.ChangeMessage(tr, ctx.Prefix, tr.Translate("tracked", true), plan, func(row map[string]any) string {
		return tracking.InvasionRowText(ctx.Config, tr, ctx.Data, row)
	})

	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(ctx, "invasion", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, plan.Updates, plan.Inserts); err != nil {
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
