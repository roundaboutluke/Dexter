package command

import (
	"fmt"
	"strings"

	"dexter/internal/tracking"
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
	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, remove, false)
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
		if total > 0 {
			ctx.MarkAlertStateDirty()
		}
		return prependWarning(warning, trackedRemovalMessage(ctx, tr, total)), nil
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

	plan := tracking.PlanUpsert(rows, trackedRows, func(candidate, existing map[string]any) bool {
		return toInt(existing["lure_id"], 0) == toInt(candidate["lure_id"], 0)
	}, "distance", "template", "clean")
	message := tracking.ChangeMessage(tr, ctx.Prefix, tr.Translate("tracked", true), plan, func(row map[string]any) string {
		return tracking.LureRowText(ctx.Config, tr, ctx.Data, row)
	})

	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(ctx, "lures", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, plan.Updates, plan.Inserts); err != nil {
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

