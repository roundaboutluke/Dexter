package command

import (
	"encoding/json"
	"strings"

	"dexter/internal/tracking"
)

// FortCommand handles fort update tracking.
type FortCommand struct{}

func (c *FortCommand) Name() string { return "fort" }

func (c *FortCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "fort") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}fort everything`, `{0}fort pokestop include_empty`, `{0}fort gym removal`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "fort", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	remove := containsWord(args, "remove")
	commandEverything := containsWord(args, "everything")
	distance, args := parseDistance(args, re)
	template, args := parseTemplate(args, re)
	if template == "" {
		template = defaultTemplateName(ctx)
	}

	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, remove, false)
	if errMsg != "" {
		return errMsg, nil
	}

	fortType := ""
	if containsWord(args, "pokestop") {
		fortType = "pokestop"
	} else if containsWord(args, "gym") {
		fortType = "gym"
	} else if containsWord(args, "everything") {
		fortType = "everything"
	}

	includeEmpty := containsPhrase(args, "include empty") || containsWord(args, "include_empty")
	changes := []string{}
	if containsWord(args, "location") {
		changes = append(changes, "location")
	}
	if containsWord(args, "name") {
		changes = append(changes, "name")
	}
	if containsWord(args, "photo") {
		changes = append(changes, "image_url")
	}
	if containsWord(args, "removal") {
		changes = append(changes, "removal")
	}
	if containsWord(args, "new") {
		changes = append(changes, "new")
	}

	if remove {
		total := int64(0)
		if commandEverything {
			removed, err := ctx.Query.DeleteQuery("forts", map[string]any{
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

	changeTypes, _ := json.Marshal(changes)
	rows := []map[string]any{{
		"id":            result.TargetID,
		"profile_no":    result.ProfileNo,
		"ping":          ctx.Ping,
		"template":      template,
		"distance":      distance,
		"fort_type":     fortType,
		"include_empty": boolToInt(includeEmpty),
		"change_types":  string(changeTypes),
	}}

	trackedRows, err := ctx.Query.SelectAllQuery("forts", map[string]any{
		"id":         result.TargetID,
		"profile_no": result.ProfileNo,
	})
	if err != nil {
		return "", err
	}

	plan := tracking.PlanUpsert(rows, trackedRows, func(candidate, existing map[string]any) bool {
		return true
	}, "distance", "template", "clean")
	message := tracking.ChangeMessage(tr, ctx.Prefix, tr.Translate("tracked", true), plan, func(row map[string]any) string {
		return tracking.FortUpdateRowText(ctx.Config, tr, ctx.Data, row)
	})

	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(ctx, "forts", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, plan.Updates, plan.Inserts); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}
