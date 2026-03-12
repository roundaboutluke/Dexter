package command

import (
	"fmt"
	"strings"
)

// UntrackCommand removes pokemon tracking.
type UntrackCommand struct{}

func (c *UntrackCommand) Name() string { return "untrack" }

func (c *UntrackCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}untrack charmander`, `{0}untrack everything`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "untrack", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	profileNo := result.ProfileNo
	targetID := result.TargetID

	args = expandPokemonAliases(ctx, args)

	typeArray := utilTypeKeys(ctx)
	argTypes := map[string]bool{}
	for _, arg := range args {
		lower := strings.ToLower(arg)
		if typeArray[lower] {
			argTypes[lower] = true
		}
	}

	monsterIDs := []any{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form := map[string]any{}
		if formRaw, ok := mon["form"].(map[string]any); ok {
			form = formRaw
		}
		if toInt(form["id"], 0) != 0 {
			continue
		}
		monName := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		monID := toInt(mon["id"], 0)
		monIDStr := fmt.Sprintf("%d", monID)

		match := false
		for _, arg := range args {
			if strings.EqualFold(arg, "everything") {
				match = true
				break
			}
			if strings.EqualFold(arg, monName) || arg == monIDStr {
				match = true
				break
			}
		}
		if !match && len(argTypes) > 0 {
			if types, ok := mon["types"].([]any); ok {
				for _, t := range types {
					if tm, ok := t.(map[string]any); ok {
						name := strings.ToLower(fmt.Sprintf("%v", tm["name"]))
						if argTypes[name] {
							match = true
							break
						}
					}
				}
			}
		}
		if match {
			monsterIDs = append(monsterIDs, monID)
		}
	}
	for _, arg := range args {
		if strings.EqualFold(arg, "everything") {
			monsterIDs = append(monsterIDs, 0)
			break
		}
	}
	if len(monsterIDs) == 0 {
		return trackedRemovalMessage(ctx, tr, 0), nil
	}

	rows, err := ctx.Query.DeleteWhereInQuery("monsters", map[string]any{
		"id":         targetID,
		"profile_no": profileNo,
	}, monsterIDs, "pokemon_id")
	if err != nil {
		return "", err
	}
	if rows > 0 {
		ctx.MarkAlertStateDirty()
	}
	return trackedRemovalMessage(ctx, tr, rows), nil
}
