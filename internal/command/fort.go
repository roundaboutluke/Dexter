package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"poraclego/internal/tracking"
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
	if total == 1 {
		return prependWarning(warning, fmt.Sprintf("%s, %s", tr.Translate("I removed 1 entry", false), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
	}
	return prependWarning(warning, fmt.Sprintf("%s, %s", tr.TranslateFormat("I removed {0} entries", total), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
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

	insert := append([]map[string]any{}, rows...)
	updates := []map[string]any{}
	unchanged := []map[string]any{}
	for i := len(insert) - 1; i >= 0; i-- {
		candidate := insert[i]
		for _, existing := range trackedRows {
			diffs := fortDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && fortDiffIsUpdate(diffs) {
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
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.FortUpdateRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.FortUpdateRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.FortUpdateRowText(ctx.Config, tr, ctx.Data, row))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("forts", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("forts", append(insert, updates...)); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

func fortDiffKeys(existing map[string]any, desired map[string]any) []string {
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

func fortDiffIsUpdate(diffs []string) bool {
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
