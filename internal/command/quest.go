package command

import (
	"fmt"
	"strconv"
	"strings"

	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
)

// QuestCommand handles quest tracking.
type QuestCommand struct{}

func (c *QuestCommand) Name() string { return "quest" }

func (c *QuestCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "quest") && !containsWord(args, "remove") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}quest spinda`, `{0}quest energycharizard`, `{0}quest ar rarecandy`, `{0}quest noar rarecandy`, `{0}quest remove everything`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "quest", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	remove := containsWord(args, "remove")
	arMode, args, errMsg := parseQuestARMode(args)
	if errMsg != "" {
		return errMsg, nil
	}
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

	options := buildQuestOptions(ctx, args, re, remove)
	if len(options.Entries) == 0 {
		return prependWarning(warning, tr.Translate("404 no valid tracks found", false)), nil
	}

	if remove {
		return prependWarning(warning, removeQuestEntries(ctx, tr, result, options, arMode)), nil
	}

	rows := []map[string]any{}
	for _, entry := range options.Entries {
		row := map[string]any{
			"id":          result.TargetID,
			"profile_no":  result.ProfileNo,
			"ping":        ctx.Ping,
			"reward":      entry.Reward,
			"template":    template,
			"reward_type": entry.RewardType,
			"amount":      entry.Amount,
			"form":        entry.Form,
			"distance":    distance,
			"clean":       boolToInt(clean),
			"shiny":       entry.Shiny,
			"ar":          arMode,
		}
		rows = append(rows, row)
	}

	trackedRows, err := ctx.Query.SelectAllQuery("quest", map[string]any{
		"id":         result.TargetID,
		"profile_no": result.ProfileNo,
	})
	if err != nil {
		return "", err
	}

	plan := tracking.PlanUpsert(rows, trackedRows, func(candidate, existing map[string]any) bool {
		return toInt(existing["reward_type"], 0) == toInt(candidate["reward_type"], 0) &&
			toInt(existing["reward"], 0) == toInt(candidate["reward"], 0) &&
			toInt(existing["ar"], 0) == toInt(candidate["ar"], 0)
	}, "distance", "template", "clean")
	message := tracking.ChangeMessage(tr, ctx.Prefix, tr.Translate("tracked", true), plan, func(row map[string]any) string {
		return tracking.QuestRowText(ctx.Config, tr, ctx.Data, row)
	})

	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(ctx, "quest", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, plan.Updates, plan.Inserts); err != nil {
			return "", err
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

type questEntry struct {
	RewardType int
	Reward     int
	Form       int
	Amount     int
	Shiny      int
}

type questOptions struct {
	Entries           []questEntry
	ItemIDs           []int
	MonsterIDs        []int
	EnergyTargets     []int
	CandyTargets      []int
	XLCandyTargets    []int
	ExperienceTarget  int
	CommandEverything int
	StardustTracking  int
}

func buildQuestOptions(ctx *Context, args []string, re *RegexSet, remove bool) questOptions {
	options := questOptions{StardustTracking: 9999999}
	shiny := 0
	if containsWord(args, "shiny") {
		shiny = 1
	}

	minDust := 10000000
	energyTargets := []int{}
	candyTargets := []int{}
	xlCandyTargets := []int{}
	expTarget := 0

	allowEverything := everythingAllowed(ctx, remove)
	allPokemon := containsWord(args, "all") && containsWord(args, "pokemon")
	everythingFlag := containsWord(args, "everything") || allPokemon

	for i := 0; i < len(args); i++ {
		arg := args[i]
		lower := strings.ToLower(arg)
		if lower == "xl" && i+1 < len(args) && strings.EqualFold(args[i+1], "candy") {
			xlCandyTargets = append(xlCandyTargets, 0)
			i++
			continue
		}
		switch {
		case strings.EqualFold(arg, "stardust"):
			minDust = 0
			options.StardustTracking = -1
		case re.Stardust.MatchString(arg):
			match := re.Stardust.FindStringSubmatch(arg)
			if len(match) > 2 {
				minDust = toInt(match[2], 0)
				options.StardustTracking = -1
			}
		case re.Energy.MatchString(arg):
			match := re.Energy.FindStringSubmatch(arg)
			if len(match) > 2 {
				if id := findMonsterIDByQuery(ctx, match[2]); id > 0 {
					energyTargets = append(energyTargets, id)
				}
			}
		case strings.EqualFold(arg, "energy"):
			energyTargets = append(energyTargets, 0)
		case re.Candy.MatchString(arg):
			match := re.Candy.FindStringSubmatch(arg)
			if len(match) > 2 {
				if id := findMonsterIDByQuery(ctx, match[2]); id > 0 {
					candyTargets = append(candyTargets, id)
				}
			}
		case strings.EqualFold(arg, "candy"):
			candyTargets = append(candyTargets, 0)
		case strings.HasPrefix(lower, "xlcandy"):
			query := strings.TrimSpace(arg[len("xlcandy"):])
			query = strings.TrimPrefix(query, ":")
			if query == "" {
				xlCandyTargets = append(xlCandyTargets, 0)
				continue
			}
			if id := findMonsterIDByQuery(ctx, query); id > 0 {
				xlCandyTargets = append(xlCandyTargets, id)
			}
		case strings.EqualFold(arg, "xl candy"):
			xlCandyTargets = append(xlCandyTargets, 0)
		case strings.EqualFold(arg, "experience"):
			expTarget = 1
		}
	}

	monsters := questMonsters(ctx, args, re, allowEverything && everythingFlag)
	itemIDs := questItems(ctx, args, allowEverything && containsWord(args, "all") && containsWord(args, "items"))

	if allowEverything && everythingFlag {
		itemIDs = allItemIDs(ctx)
		minDust = 0
		options.StardustTracking = -1
		energyTargets = append(energyTargets, 0)
		candyTargets = append(candyTargets, 0)
		xlCandyTargets = append(xlCandyTargets, 0)
		expTarget = 1
		options.CommandEverything = 1
	}

	entries := []questEntry{}
	if minDust < 10000000 {
		entries = append(entries, questEntry{RewardType: 3, Reward: minDust, Shiny: shiny})
	}
	for _, id := range energyTargets {
		entries = append(entries, questEntry{RewardType: 12, Reward: id, Shiny: shiny})
	}
	for _, id := range candyTargets {
		entries = append(entries, questEntry{RewardType: 4, Reward: id, Shiny: shiny})
	}
	for _, id := range xlCandyTargets {
		entries = append(entries, questEntry{RewardType: 9, Reward: id, Shiny: shiny})
	}
	if expTarget == 1 {
		entries = append(entries, questEntry{RewardType: 1, Reward: 0, Shiny: shiny})
	}
	for _, mon := range monsters {
		formID := toInt(mon["form"], 0)
		entries = append(entries, questEntry{RewardType: 7, Reward: toInt(mon["id"], 0), Form: formID, Shiny: shiny})
	}
	for _, id := range itemIDs {
		entries = append(entries, questEntry{RewardType: 2, Reward: id, Shiny: shiny})
	}

	options.Entries = entries
	options.ItemIDs = itemIDs
	options.MonsterIDs = extractMonsterIDs(monsters)
	options.EnergyTargets = energyTargets
	options.CandyTargets = candyTargets
	options.XLCandyTargets = xlCandyTargets
	options.ExperienceTarget = expTarget
	return options
}

func everythingAllowed(ctx *Context, remove bool) bool {
	if ctx == nil || ctx.Config == nil {
		return false
	}
	mode, _ := ctx.Config.GetString("tracking.everythingFlagPermissions")
	mode = strings.ToLower(mode)
	switch mode {
	case "allow-any", "allow-and-always-individually", "allow-and-ignore-individually":
		return true
	default:
		return ctx.IsAdmin || remove
	}
}

func questMonsters(ctx *Context, args []string, re *RegexSet, includeEverything bool) []map[string]any {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	typeNames := utilTypeKeys(ctx)
	argTypes := []string{}
	formNames := []string{}
	lowerArgs := make([]string, 0, len(args))
	for _, arg := range args {
		lowerArgs = append(lowerArgs, strings.ToLower(arg))
		if typeNames[strings.ToLower(arg)] {
			argTypes = append(argTypes, strings.ToLower(arg))
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
		typeMatch := false
		if len(argTypes) > 0 {
			for _, t := range typeNamesFromList(mon["types"]) {
				if containsString(argTypes, strings.ToLower(t)) {
					typeMatch = true
					break
				}
			}
		}
		nameMatch := containsString(lowerArgs, name) || containsString(lowerArgs, id)
		if len(formNames) > 0 {
			if (nameMatch || typeMatch || includeEverything) && containsString(formNames, formName) {
				monsters = append(monsters, mon)
			}
			continue
		}
		if (nameMatch || typeMatch || includeEverything) && formID == 0 {
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
	return monsters
}

func questItems(ctx *Context, args []string, includeAll bool) []int {
	if ctx == nil || ctx.Data == nil || ctx.Data.Items == nil {
		return nil
	}
	tr := ctx.I18n.Translator(ctx.Language)
	items := []int{}
	lowerArgs := make([]string, 0, len(args))
	for _, arg := range args {
		lowerArgs = append(lowerArgs, strings.ToLower(arg))
	}
	for key, raw := range ctx.Data.Items {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", item["name"]))
		translated := strings.ToLower(tr.Translate(name, false))
		if containsString(lowerArgs, name) || (translated != "" && containsString(lowerArgs, translated)) {
			items = append(items, toInt(key, 0))
		}
	}
	if includeAll || (containsWord(args, "all") && containsWord(args, "items")) {
		return allItemIDs(ctx)
	}
	return items
}

func allItemIDs(ctx *Context) []int {
	if ctx == nil || ctx.Data == nil || ctx.Data.Items == nil {
		return nil
	}
	out := make([]int, 0, len(ctx.Data.Items))
	for key := range ctx.Data.Items {
		out = append(out, toInt(key, 0))
	}
	return out
}

func findMonsterIDByQuery(ctx *Context, query string) int {
	if ctx == nil || ctx.Data == nil {
		return 0
	}
	tr := ctx.I18n.Translator(ctx.Language)
	q := strings.ToLower(query)
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		translated := strings.ToLower(tr.Translate(name, false))
		if strings.Contains(name, q) || (translated != "" && strings.Contains(translated, q)) {
			return toInt(mon["id"], 0)
		}
	}
	return 0
}

func extractMonsterIDs(monsters []map[string]any) []int {
	out := make([]int, 0, len(monsters))
	for _, mon := range monsters {
		out = append(out, toInt(mon["id"], 0))
	}
	return out
}

func removeQuestEntries(ctx *Context, tr *i18n.Translator, result TargetResult, options questOptions, arMode int) string {
	items := append([]int{}, options.ItemIDs...)
	monsters := append([]int{}, options.MonsterIDs...)
	energy := append([]int{}, options.EnergyTargets...)
	candy := append([]int{}, options.CandyTargets...)
	xlCandy := append([]int{}, options.XLCandyTargets...)
	expAll := options.ExperienceTarget

	if len(items) == 0 {
		items = append(items, 0)
	}
	if len(monsters) == 0 {
		monsters = append(monsters, 0)
	}
	if len(energy) == 0 {
		energy = append(energy, 10000)
	}
	if len(candy) == 0 {
		candy = append(candy, 10000)
	}
	if len(xlCandy) == 0 {
		xlCandy = append(xlCandy, 10000)
	}
	arWhere := ""
	if arMode > 0 {
		arWhere = fmt.Sprintf(" AND ar=%d", arMode)
	}
	query := fmt.Sprintf(
		"DELETE FROM quest WHERE id='%s' AND profile_no=%d%s AND ((reward_type=2 AND reward IN (%s)) OR (reward_type=7 AND reward IN (%s)) OR (reward_type=3 AND reward > %d) OR (reward_type=12 AND reward IN (%s)) OR (reward_type=12 AND %d=1) OR (reward_type=4 AND reward IN (%s)) OR (reward_type=4 AND %d=1) OR (reward_type=9 AND reward IN (%s)) OR (reward_type=9 AND %d=1) OR (reward_type=1 AND %d=1))",
		result.TargetID,
		result.ProfileNo,
		arWhere,
		joinInts(items),
		joinInts(monsters),
		options.StardustTracking,
		joinInts(energy),
		options.CommandEverything,
		joinInts(candy),
		options.CommandEverything,
		joinInts(xlCandy),
		options.CommandEverything,
		expAll,
	)
	affected, err := ctx.Query.ExecQuery(query)
	if err != nil {
		return tr.Translate("There was a problem removing entries", false)
	}
	if affected > 0 {
		ctx.MarkAlertStateDirty()
	}
	return trackedRemovalMessage(ctx, tr, affected)
}

func joinInts(values []int) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, strconv.Itoa(value))
	}
	return strings.Join(out, ",")
}

// parseQuestARMode returns 0 (any), 1 (no AR), or 2 (with AR).
func parseQuestARMode(args []string) (int, []string, string) {
	mode := 0
	out := make([]string, 0, len(args))
	for _, arg := range args {
		lower := strings.ToLower(strings.TrimSpace(arg))
		if lower == "" {
			continue
		}
		switch lower {
		case "ar", "withar", "with_ar":
			if mode != 0 && mode != 2 {
				return 0, args, "Please pick only one of `ar` or `noar`."
			}
			mode = 2
		case "noar", "no_ar", "withoutar", "without_ar":
			if mode != 0 && mode != 1 {
				return 0, args, "Please pick only one of `ar` or `noar`."
			}
			mode = 1
		default:
			out = append(out, arg)
		}
	}
	return mode, out, ""
}
