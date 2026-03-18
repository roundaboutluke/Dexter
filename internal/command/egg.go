package command

import (
	"strings"

	"dexter/internal/tracking"
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
	gymID := strings.TrimSpace(parseGymID(args))
	allowZeroWithoutArea := gymID != ""
	distance, args := parseDistance(args, re)
	template, args := parseTemplate(args, re)
	clean, args := parseClean(args)
	if template == "" {
		template = defaultTemplateName(ctx)
	}
	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, remove, allowZeroWithoutArea)
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
		if total > 0 {
			ctx.MarkAlertStateDirty()
		}
		return prependWarning(warning, trackedRemovalMessage(ctx, tr, total)), nil
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

	plan := tracking.PlanUpsert(rows, trackedRows, func(candidate, existing map[string]any) bool {
		return toInt(existing["level"], 0) == toInt(candidate["level"], 0)
	}, "distance", "template", "clean", "gym_id")
	message := tracking.ChangeMessage(tr, ctx.Prefix, tr.Translate("tracked", true), plan, func(row map[string]any) string {
		return tracking.EggRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner)
	})

	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(ctx, "egg", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, plan.Updates, plan.Inserts); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}
