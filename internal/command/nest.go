package command

import (
	"fmt"
	"strings"

	"poraclego/internal/tracking"
)

// NestCommand handles nest tracking.
type NestCommand struct{}

func (c *NestCommand) Name() string { return "nest" }

func (c *NestCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "nest") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}nest bulbasaur`, `{0}nest remove everything`, `{0}nest hoppip minspawn5`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "nest", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	remove := containsWord(args, "remove")
	distance, args := parseDistance(args, re)
	template, args := parseTemplate(args, re)
	minSpawn, args := parseMinSpawn(args, re)
	clean, args := parseClean(args)
	if template == "" {
		template = defaultTemplateName(ctx)
	}
	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, remove)
	if errMsg != "" {
		return errMsg, nil
	}

	args = expandPokemonAliases(ctx, args)
	lowerArgs := make([]string, 0, len(args))
	for _, arg := range args {
		lowerArgs = append(lowerArgs, strings.ToLower(arg))
	}
	typeNames := utilTypeKeys(ctx)
	argTypes := []string{}
	formNames := []string{}
	for _, arg := range args {
		lower := strings.ToLower(arg)
		if _, ok := typeNames[lower]; ok {
			argTypes = append(argTypes, lower)
		}
		if re.Form.MatchString(arg) {
			match := re.Form.FindStringSubmatch(arg)
			if len(match) > 2 {
				form := strings.ToLower(ctx.I18n.ReverseTranslateCommand(match[2], true))
				formNames = append(formNames, form)
			}
		}
	}

	monsters := []map[string]any{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := strings.ToLower(fmt.Sprintf("%v", mon["id"]))
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		formName := strings.ToLower(fmt.Sprintf("%v", form["name"]))
		formID := toInt(form["id"], 0)
		types := typeNamesFromList(mon["types"])
		typeMatch := false
		if len(argTypes) > 0 {
			for _, t := range types {
				if containsString(argTypes, strings.ToLower(t)) {
					typeMatch = true
					break
				}
			}
		}
		nameMatch := containsString(lowerArgs, name) || containsString(lowerArgs, id)
		everything := containsString(lowerArgs, "everything")
		if len(formNames) > 0 {
			if (nameMatch || typeMatch || everything) && containsString(formNames, formName) {
				monsters = append(monsters, mon)
			}
			continue
		}
		if (nameMatch || typeMatch) && formID == 0 {
			monsters = append(monsters, mon)
		}
	}

	genRange := genDataRange(ctx, args, re)
	if genRange.min > 0 {
		filtered := []map[string]any{}
		for _, mon := range monsters {
			id := toInt(mon["id"], 0)
			if id >= genRange.min && id <= genRange.max {
				filtered = append(filtered, mon)
			}
		}
		monsters = filtered
	}

	if containsString(lowerArgs, "everything") && len(formNames) == 0 {
		monsters = append(monsters, map[string]any{
			"id":   0,
			"form": map[string]any{"id": 0},
		})
	}

	if len(monsters) == 0 && !containsString(lowerArgs, "everything") {
		return prependWarning(warning, tr.Translate("404 no valid tracks found", false)), nil
	}

	if remove {
		ids := make([]any, 0, len(monsters))
		for _, mon := range monsters {
			ids = append(ids, toInt(mon["id"], 0))
		}
		values := make([]any, 0, len(ids))
		for _, id := range ids {
			values = append(values, id)
		}
		removed, err := ctx.Query.DeleteWhereInQuery("nests", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, values, "pokemon_id")
		if err != nil {
			return "", err
		}
		if removed == 1 {
			return prependWarning(warning, fmt.Sprintf("%s, %s", tr.Translate("I removed 1 entry", false), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
		}
		return prependWarning(warning, fmt.Sprintf("%s, %s", tr.TranslateFormat("I removed {0} entries", removed), tr.TranslateFormat("use `{0}{1}` to see what you are currently tracking", ctx.Prefix, tr.Translate("tracked", true)))), nil
	}

	rows := []map[string]any{}
	for _, mon := range monsters {
		form := map[string]any{}
		if entry, ok := mon["form"].(map[string]any); ok {
			form = entry
		}
		rows = append(rows, map[string]any{
			"id":            result.TargetID,
			"profile_no":    result.ProfileNo,
			"pokemon_id":    toInt(mon["id"], 0),
			"ping":          ctx.Ping,
			"template":      template,
			"distance":      distance,
			"clean":         boolToInt(clean),
			"min_spawn_avg": minSpawn,
			"form":          toInt(form["id"], 0),
		})
	}

	existing, err := ctx.Query.SelectAllQuery("nests", map[string]any{
		"id":         result.TargetID,
		"profile_no": result.ProfileNo,
	})
	if err != nil {
		return "", err
	}
	existingMap := map[string]map[string]any{}
	for _, row := range existing {
		key := nestKey(row)
		existingMap[key] = row
	}

	unchanged := []map[string]any{}
	updates := []map[string]any{}
	inserts := []map[string]any{}

	for _, row := range rows {
		key := nestKey(row)
		if existingRow, ok := existingMap[key]; ok {
			if nestRowEqual(existingRow, row) {
				unchanged = append(unchanged, existingRow)
				continue
			}
			if uid := toInt(existingRow["uid"], 0); uid != 0 {
				if _, err := ctx.Query.UpdateQuery("nests", map[string]any{
					"ping":          row["ping"],
					"template":      row["template"],
					"distance":      row["distance"],
					"clean":         row["clean"],
					"min_spawn_avg": row["min_spawn_avg"],
					"form":          row["form"],
				}, map[string]any{"uid": uid}); err != nil {
					return "", err
				}
			}
			updates = append(updates, row)
			continue
		}
		inserts = append(inserts, row)
	}

	if len(inserts) > 0 {
		if _, err := ctx.Query.InsertQuery("nests", inserts); err != nil {
			return "", err
		}
	}

	total := len(unchanged) + len(updates) + len(inserts)
	if total > 50 {
		return prependWarning(warning, tr.TranslateFormat("I have made a lot of changes. See {0}{1} for details", ctx.Prefix, tr.Translate("tracked", true))), nil
	}

	lines := []string{}
	for _, row := range unchanged {
		lines = append(lines, fmt.Sprintf("%s%s", tr.Translate("Unchanged: ", false), tracking.NestRowText(ctx.Config, tr, ctx.Data, row)))
	}
	for _, row := range updates {
		lines = append(lines, fmt.Sprintf("%s%s", tr.Translate("Updated: ", false), tracking.NestRowText(ctx.Config, tr, ctx.Data, row)))
	}
	for _, row := range inserts {
		lines = append(lines, fmt.Sprintf("%s%s", tr.Translate("New: ", false), tracking.NestRowText(ctx.Config, tr, ctx.Data, row)))
	}
	if len(lines) == 0 {
		return warning, nil
	}
	return prependWarning(warning, strings.Join(lines, "\n")), nil
}

type genRange struct {
	min int
	max int
}

func genDataRange(ctx *Context, args []string, re *RegexSet) genRange {
	if ctx == nil || ctx.Data == nil || ctx.Data.UtilData == nil {
		return genRange{}
	}
	raw, ok := ctx.Data.UtilData["genData"].(map[string]any)
	if !ok {
		return genRange{}
	}
	for _, arg := range args {
		if !re.Gen.MatchString(arg) {
			continue
		}
		match := re.Gen.FindStringSubmatch(arg)
		if len(match) < 3 {
			continue
		}
		genID := toInt(match[2], 0)
		if genID == 0 {
			continue
		}
		if entry, ok := raw[fmt.Sprintf("%d", genID)].(map[string]any); ok {
			return genRange{
				min: toInt(entry["min"], 0),
				max: toInt(entry["max"], 0),
			}
		}
	}
	return genRange{}
}

func nestKey(row map[string]any) string {
	return fmt.Sprintf("%d:%d", toInt(row["pokemon_id"], 0), toInt(row["form"], 0))
}

func nestRowEqual(existing map[string]any, candidate map[string]any) bool {
	if toInt(existing["pokemon_id"], 0) != toInt(candidate["pokemon_id"], 0) {
		return false
	}
	if toInt(existing["form"], 0) != toInt(candidate["form"], 0) {
		return false
	}
	if strings.TrimSpace(fmt.Sprintf("%v", existing["template"])) != strings.TrimSpace(fmt.Sprintf("%v", candidate["template"])) {
		return false
	}
	if toInt(existing["distance"], 0) != toInt(candidate["distance"], 0) {
		return false
	}
	if toInt(existing["clean"], 0) != toInt(candidate["clean"], 0) {
		return false
	}
	if toInt(existing["min_spawn_avg"], 0) != toInt(candidate["min_spawn_avg"], 0) {
		return false
	}
	return true
}
