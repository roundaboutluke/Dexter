package command

import (
	"strings"

	"dexter/internal/tracking"
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

	gymID := parseGymID(args)
	allowZeroWithoutArea := strings.TrimSpace(gymID) != ""
	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, remove, allowZeroWithoutArea)
	if errMsg != "" {
		return errMsg, nil
	}

	teams := parseGymTeams(args)
	slotChanges := boolToInt(containsPhrase(args, "slot changes") || containsWord(args, "slot_changes"))
	battleChanges := boolToInt(containsPhrase(args, "battle changes") || containsWord(args, "battle_changes"))

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
		if total > 0 {
			ctx.MarkAlertStateDirty()
		}
		return prependWarning(warning, trackedRemovalMessage(ctx, tr, total)), nil
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

	plan := tracking.PlanUpsert(rows, trackedRows, func(candidate, existing map[string]any) bool {
		return toInt(existing["team"], 0) == toInt(candidate["team"], 0)
	}, "distance", "template", "clean", "slot_changes", "battle_changes")
	message := tracking.ChangeMessage(tr, ctx.Prefix, tr.Translate("tracked", true), plan, func(row map[string]any) string {
		return tracking.GymRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner)
	})

	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(ctx, "gym", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, plan.Updates, plan.Inserts); err != nil {
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
			teams = append(teams, 4)
		}
	}
	return uniqueInts(teams)
}
