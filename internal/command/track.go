package command

import (
	"fmt"
	"strings"

	"dexter/internal/tracking"
)

// TrackCommand handles pokemon tracking.
type TrackCommand struct{}

func (c *TrackCommand) Name() string { return "track" }

func (c *TrackCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "monster") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	args = expandPokemonAliases(ctx, args)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}track charmander`, `{0}track everything iv100`, `{0}track gible d500`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "track", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	distance, args := parseDistance(args, re)
	template, args := parseTemplate(args, re)
	clean, args := parseClean(args)
	if template == "" {
		template = defaultTemplateName(ctx)
	}

	trackOptions := parseTrackOptions(ctx, tr, args, re)
	if trackOptions.Error != "" {
		return trackOptions.Error, nil
	}
	if trackOptions.EverythingOnly && !ctx.IsAdmin {
		lines := []string{tr.Translate("This would result in too many alerts. You need to provide additional filters to limit the number of valid candidates.", false)}
		if helpLine := singleLineHelpText(ctx, "track", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	mode := everythingMode(ctx)
	individually := containsWord(args, "individually") && mode.individuallyAllowed
	forceSeparate := mode.forceSeparate
	if trackOptions.Everything && mode.ignoreIndividually {
		individually = false
	}
	everythingWildcard := trackOptions.Everything && !individually && len(trackOptions.TypeFilters) == 0 && len(trackOptions.FormNames) == 0 && trackOptions.GenMin == 0 && !forceSeparate

	if trackOptions.Everything && mode.deny && !ctx.IsAdmin {
		return tr.Translate("Tracking everything is not permitted.", false), nil
	}

	if trackOptions.PvpLeague != 0 && trackOptions.PvpCount > 1 {
		return fmt.Sprintf("%s `%s%s`", tr.Translate("Oops, more than one league PVP parameters were set in command! - check the", false), ctx.Prefix, tr.Translate("help", true)), nil
	}

	warning := trackOptions.Warning
	distance, distWarning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, false, false)
	warning = prependWarning(warning, distWarning)
	if errMsg != "" && result.Target.Type != "webhook" {
		return errMsg, nil
	}

	applyTrackDefaults(ctx, &trackOptions)

	ids := []int{}
	if everythingWildcard {
		ids = []int{0}
	} else {
		includeAll := trackOptions.Everything || forceSeparate || individually
		ids = resolveMonsterIDs(ctx, args, trackOptions.TypeFilters, trackOptions.GenMin, trackOptions.GenMax, includeAll)
		if len(ids) == 0 {
			return prependWarning(warning, tr.Translate("No monsters matched.", false)), nil
		}
	}

	monsters := buildTrackMonsters(ctx, ids, trackOptions.FormNames, trackOptions.IncludeAllForms)
	if len(monsters) == 0 {
		return prependWarning(warning, tr.Translate("No monsters matched.", false)), nil
	}

	rows := buildTrackRows(result.TargetID, result.ProfileNo, ctx.Ping, template, distance, clean, trackOptions, monsters)

	trackedRows, err := ctx.Query.SelectAllQuery("monsters", map[string]any{
		"id":         result.TargetID,
		"profile_no": result.ProfileNo,
	})
	if err != nil {
		return "", err
	}

	plan := tracking.PlanUpsert(rows, trackedRows, func(candidate, existing map[string]any) bool {
		return toInt(existing["pokemon_id"], 0) == toInt(candidate["pokemon_id"], 0)
	}, "min_iv", "distance", "template", "clean")
	message := tracking.ChangeMessage(tr, ctx.Prefix, tr.Translate("tracked", true), plan, func(row map[string]any) string {
		return tracking.MonsterRowText(ctx.Config, tr, ctx.Data, row)
	})

	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(ctx, "monsters", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, plan.Updates, plan.Inserts); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}
