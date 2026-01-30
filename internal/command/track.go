package command

import (
	"fmt"
	"regexp"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
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

	distance, warning, errMsg := applyDistanceDefaults(ctx, tr, distance, result, false)
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

	minIv := trackOptions.MinIV
	if minIv == -1 && minimumIvZero(&trackOptions) {
		minIv = 0
	}

	rows := []map[string]any{}
	for _, mon := range monsters {
		row := map[string]any{
			"id":                 result.TargetID,
			"profile_no":         result.ProfileNo,
			"pokemon_id":         mon.ID,
			"ping":               ctx.Ping,
			"template":           template,
			"distance":           distance,
			"min_iv":             minIv,
			"max_iv":             trackOptions.MaxIV,
			"min_cp":             trackOptions.MinCP,
			"max_cp":             trackOptions.MaxCP,
			"min_level":          trackOptions.MinLevel,
			"max_level":          trackOptions.MaxLevel,
			"atk":                trackOptions.MinAtk,
			"def":                trackOptions.MinDef,
			"sta":                trackOptions.MinSta,
			"min_weight":         trackOptions.MinWeight,
			"max_weight":         trackOptions.MaxWeight,
			"form":               mon.FormID,
			"max_atk":            trackOptions.MaxAtk,
			"max_def":            trackOptions.MaxDef,
			"max_sta":            trackOptions.MaxSta,
			"gender":             trackOptions.Gender,
			"clean":              boolToInt(clean),
			"min_time":           trackOptions.MinTime,
			"rarity":             trackOptions.MinRarity,
			"max_rarity":         trackOptions.MaxRarity,
			"size":               trackOptions.MinSize,
			"max_size":           trackOptions.MaxSize,
			"pvp_ranking_league": trackOptions.PvpLeague,
			"pvp_ranking_best":   trackOptions.PvpBest,
			"pvp_ranking_worst":  trackOptions.PvpWorst,
			"pvp_ranking_min_cp": trackOptions.PvpMinCP,
			"pvp_ranking_cap":    trackOptions.PvpCap,
		}
		rows = append(rows, row)
	}

	trackedRows, err := ctx.Query.SelectAllQuery("monsters", map[string]any{
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
			if toInt(existing["pokemon_id"], 0) != toInt(candidate["pokemon_id"], 0) {
				continue
			}
			diffs := monsterDiffKeys(existing, candidate)
			if len(diffs) == 1 && diffs[0] == "uid" {
				unchanged = append(unchanged, candidate)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffs) == 2 && monsterDiffIsUpdate(diffs) {
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
			message += fmt.Sprintf("%s%s\n", tr.Translate("Unchanged: ", false), tracking.MonsterRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range updates {
			message += fmt.Sprintf("%s%s\n", tr.Translate("Updated: ", false), tracking.MonsterRowText(ctx.Config, tr, ctx.Data, row))
		}
		for _, row := range insert {
			message += fmt.Sprintf("%s%s\n", tr.Translate("New: ", false), tracking.MonsterRowText(ctx.Config, tr, ctx.Data, row))
		}
	}

	if len(updates) > 0 {
		_, err = ctx.Query.DeleteWhereInQuery("monsters", map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
		}, extractUids(updates), "uid")
		if err != nil {
			return "", err
		}
	}
	if len(insert)+len(updates) > 0 {
		if _, err := ctx.Query.InsertQuery("monsters", append(insert, updates...)); err != nil {
			return "", err
		}
		if ctx.RefreshAlertCache != nil {
			ctx.RefreshAlertCache()
		}
	}
	return prependWarning(warning, strings.TrimSpace(message)), nil
}

type trackOptions struct {
	MinIV          int
	MaxIV          int
	MinCP          int
	MaxCP          int
	MinLevel       int
	MaxLevel       int
	MinAtk         int
	MaxAtk         int
	MinDef         int
	MaxDef         int
	MinSta         int
	MaxSta         int
	MinWeight      int
	MaxWeight      int
	MinRarity      int
	MaxRarity      int
	MinSize        int
	MaxSize        int
	Gender         int
	MinTime        int
	FormID         int
	FormNames      []string
	IncludeAllForms bool
	TypeFilters    []string
	Everything     bool
	EverythingOnly bool
	PvpCount       int
	GenMin         int
	GenMax         int
	PvpLeague      int
	PvpBest        int
	PvpWorst       int
	PvpMinCP       int
	PvpCap         int
	Error          string
}

func parseTrackOptions(ctx *Context, tr *i18n.Translator, args []string, re *RegexSet) trackOptions {
	opt := trackOptions{
		MinIV:     -1,
		MaxIV:     100,
		MinCP:     0,
		MaxCP:     9000,
		MinLevel:  0,
		MaxLevel:  55,
		MinAtk:    0,
		MaxAtk:    15,
		MinDef:    0,
		MaxDef:    15,
		MinSta:    0,
		MaxSta:    15,
		MinWeight: 0,
		MaxWeight: 9000000,
		MinRarity: -1,
		MaxRarity: 6,
		MinSize:   -1,
		MaxSize:   5,
		Gender:    0,
		MinTime:   0,
		PvpLeague: 0,
		PvpBest:   1,
		PvpWorst:  4096,
		PvpMinCP:  0,
		PvpCap:    0,
	}

	typeFilters := []string{}
	typeKeys := utilTypeKeys(ctx)
	for _, arg := range args {
		if typeKeys[strings.ToLower(arg)] {
			typeFilters = append(typeFilters, strings.ToLower(arg))
		}
	}
	opt.TypeFilters = typeFilters

	for _, arg := range args {
		switch {
		case strings.EqualFold(arg, "male"):
			opt.Gender = 1
		case strings.EqualFold(arg, "female"):
			opt.Gender = 2
		case strings.EqualFold(arg, "genderless"):
			opt.Gender = 3
		case strings.EqualFold(arg, "everything"):
			opt.Everything = true
		case re.Gen.MatchString(arg):
			match := re.Gen.FindStringSubmatch(arg)
			if len(match) > 2 {
				genID := toInt(match[2], 0)
				if raw, ok := ctx.Data.UtilData["genData"].(map[string]any); ok {
					if entry, ok := raw[fmt.Sprintf("%d", genID)].(map[string]any); ok {
						opt.GenMin = toInt(entry["min"], 0)
						opt.GenMax = toInt(entry["max"], 0)
					}
				}
			}
		case re.IV.MatchString(arg):
			min, max, ok, hasMax := parseRangeOptionalMax(arg, re.IV)
			if ok {
				opt.MinIV = min
				if hasMax {
					opt.MaxIV = max
				}
			}
		case re.MaxIV.MatchString(arg):
			opt.MaxIV = toInt(re.MaxIV.FindStringSubmatch(arg)[2], opt.MaxIV)
		case re.Level.MatchString(arg):
			min, max, ok, hasMax := parseRangeOptionalMax(arg, re.Level)
			if ok {
				opt.MinLevel = min
				if hasMax {
					opt.MaxLevel = max
				}
			}
		case re.MaxLevel.MatchString(arg):
			opt.MaxLevel = toInt(re.MaxLevel.FindStringSubmatch(arg)[2], opt.MaxLevel)
		case re.CP.MatchString(arg):
			min, max, ok, hasMax := parseRangeOptionalMax(arg, re.CP)
			if ok {
				opt.MinCP = min
				if hasMax {
					opt.MaxCP = max
				}
			}
		case re.MaxCP.MatchString(arg):
			opt.MaxCP = toInt(re.MaxCP.FindStringSubmatch(arg)[2], opt.MaxCP)
		case re.Atk.MatchString(arg):
			min, max, ok, hasMax := parseRangeOptionalMax(arg, re.Atk)
			if ok {
				opt.MinAtk = min
				if hasMax {
					opt.MaxAtk = max
				}
			}
		case re.MaxAtk.MatchString(arg):
			opt.MaxAtk = toInt(re.MaxAtk.FindStringSubmatch(arg)[2], opt.MaxAtk)
		case re.Def.MatchString(arg):
			min, max, ok, hasMax := parseRangeOptionalMax(arg, re.Def)
			if ok {
				opt.MinDef = min
				if hasMax {
					opt.MaxDef = max
				}
			}
		case re.MaxDef.MatchString(arg):
			opt.MaxDef = toInt(re.MaxDef.FindStringSubmatch(arg)[2], opt.MaxDef)
		case re.Sta.MatchString(arg):
			min, max, ok, hasMax := parseRangeOptionalMax(arg, re.Sta)
			if ok {
				opt.MinSta = min
				if hasMax {
					opt.MaxSta = max
				}
			}
		case re.MaxSta.MatchString(arg):
			opt.MaxSta = toInt(re.MaxSta.FindStringSubmatch(arg)[2], opt.MaxSta)
		case re.Weight.MatchString(arg):
			min, max, ok, hasMax := parseRangeOptionalMax(arg, re.Weight)
			if ok {
				opt.MinWeight = min
				if hasMax {
					opt.MaxWeight = max
				}
			}
		case re.MaxWeight.MatchString(arg):
			opt.MaxWeight = toInt(re.MaxWeight.FindStringSubmatch(arg)[2], opt.MaxWeight)
		case re.Rarity.MatchString(arg):
			match := re.Rarity.FindStringSubmatch(arg)
			if len(match) > 2 {
				opt.MinRarity, opt.MaxRarity = parseRarityRange(ctx, match[2])
			}
		case re.MaxRarity.MatchString(arg):
			match := re.MaxRarity.FindStringSubmatch(arg)
			if len(match) > 2 {
				opt.MaxRarity = parseRarityValue(ctx, match[2], opt.MaxRarity)
			}
		case re.Size.MatchString(arg):
			match := re.Size.FindStringSubmatch(arg)
			if len(match) > 2 {
				opt.MinSize, opt.MaxSize = parseSizeRange(ctx, match[2])
			}
		case re.MaxSize.MatchString(arg):
			match := re.MaxSize.FindStringSubmatch(arg)
			if len(match) > 2 {
				opt.MaxSize = parseSizeValue(ctx, match[2], opt.MaxSize)
			}
		case re.Time.MatchString(arg):
			opt.MinTime = toInt(re.Time.FindStringSubmatch(arg)[2], opt.MinTime)
		case re.Great.MatchString(arg):
			if !commandAllowed(ctx, "pvp") {
				opt.Error = tr.Translate("You do not have permission to use the pvp parameter", false)
				break
			}
			min, max, _ := parseRange(arg, re.Great)
			opt.PvpLeague = 1500
			opt.PvpBest = min
			opt.PvpWorst = max
			opt.PvpCount++
		case re.GreatHigh.MatchString(arg):
			opt.PvpBest = toInt(re.GreatHigh.FindStringSubmatch(arg)[2], opt.PvpBest)
		case re.GreatCP.MatchString(arg):
			opt.PvpMinCP = toInt(re.GreatCP.FindStringSubmatch(arg)[2], opt.PvpMinCP)
		case re.Ultra.MatchString(arg):
			if !commandAllowed(ctx, "pvp") {
				opt.Error = tr.Translate("You do not have permission to use the pvp parameter", false)
				break
			}
			min, max, _ := parseRange(arg, re.Ultra)
			opt.PvpLeague = 2500
			opt.PvpBest = min
			opt.PvpWorst = max
			opt.PvpCount++
		case re.UltraHigh.MatchString(arg):
			opt.PvpBest = toInt(re.UltraHigh.FindStringSubmatch(arg)[2], opt.PvpBest)
		case re.UltraCP.MatchString(arg):
			opt.PvpMinCP = toInt(re.UltraCP.FindStringSubmatch(arg)[2], opt.PvpMinCP)
		case re.Little.MatchString(arg):
			if !commandAllowed(ctx, "pvp") {
				opt.Error = tr.Translate("You do not have permission to use the pvp parameter", false)
				break
			}
			min, max, _ := parseRange(arg, re.Little)
			opt.PvpLeague = 500
			opt.PvpBest = min
			opt.PvpWorst = max
			opt.PvpCount++
		case re.LittleHigh.MatchString(arg):
			opt.PvpBest = toInt(re.LittleHigh.FindStringSubmatch(arg)[2], opt.PvpBest)
		case re.LittleCP.MatchString(arg):
			opt.PvpMinCP = toInt(re.LittleCP.FindStringSubmatch(arg)[2], opt.PvpMinCP)
		case re.Cap.MatchString(arg):
			opt.PvpCap = toInt(re.Cap.FindStringSubmatch(arg)[2], opt.PvpCap)
		case re.Form.MatchString(arg):
			match := re.Form.FindStringSubmatch(arg)
			if len(match) > 2 {
				formName := strings.ToLower(match[2])
				if formName == "all" {
					opt.IncludeAllForms = true
				} else if isKnownForm(ctx, formName) {
					opt.FormNames = append(opt.FormNames, formName)
				} else {
					opt.Error = tr.TranslateFormat("Unrecognised form name {0}", formName)
				}
			}
		}
	}

	if opt.PvpCap != 0 {
		caps := levelCaps(ctx)
		if !containsStringInt(caps, opt.PvpCap) {
			choices := append([]int{0}, caps...)
			opt.Error = tr.TranslateFormat("This level cap is not supported, valid choices are: {0}", joinIntsInt(choices))
		}
	}

	opt.EverythingOnly = opt.Everything && len(opt.TypeFilters) == 0 && len(opt.FormNames) == 0 && !opt.IncludeAllForms && opt.MinIV == -1 && opt.MinCP == 0 && opt.MinLevel == 0 && opt.MinAtk == 0 && opt.MinDef == 0 && opt.MinSta == 0 && opt.MinWeight == 0 && opt.MinRarity == -1 && opt.MinSize == -1
	return opt
}

func levelCaps(ctx *Context) []int {
	if ctx == nil || ctx.Config == nil {
		return []int{50}
	}
	raw, ok := ctx.Config.Get("pvp.levelCaps")
	if !ok {
		return []int{50}
	}
	caps := []int{}
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if num := toInt(item, 0); num > 0 {
				caps = append(caps, num)
			}
		}
	case []int:
		caps = append(caps, v...)
	}
	if len(caps) == 0 {
		return []int{50}
	}
	return caps
}

func joinIntsInt(values []int) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, fmt.Sprintf("%d", value))
	}
	return strings.Join(out, ", ")
}

func resolveMonsterIDs(ctx *Context, args []string, typeFilters []string, genMin int, genMax int, includeAll bool) []int {
	if includeAll {
		return filterMonsterIDs(ctx, allBaseMonsterIDs(ctx), typeFilters, genMin, genMax)
	}
	ids := monsterIDsFromArgs(ctx, args)
	if len(ids) == 0 && len(typeFilters) > 0 {
		ids = allBaseMonsterIDs(ctx)
	}
	return filterMonsterIDs(ctx, ids, typeFilters, genMin, genMax)
}

func isKnownForm(ctx *Context, formName string) bool {
	for _, raw := range ctx.Data.Monsters {
		if mon, ok := raw.(map[string]any); ok {
			if form, ok := mon["form"].(map[string]any); ok {
				if strings.EqualFold(fmt.Sprintf("%v", form["name"]), formName) {
					return true
				}
			}
		}
	}
	return false
}

func containsStringInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func uniqueInts(values []int) []int {
	seen := map[int]bool{}
	out := []int{}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

type everythingSettings struct {
	deny                bool
	forceSeparate       bool
	individuallyAllowed bool
	ignoreIndividually  bool
}

func everythingMode(ctx *Context) everythingSettings {
	mode := ""
	if ctx != nil && ctx.Config != nil {
		mode, _ = ctx.Config.GetString("tracking.everythingFlagPermissions")
	}
	mode = strings.ToLower(mode)
	settings := everythingSettings{
		individuallyAllowed: true,
	}
	switch mode {
	case "allow-and-always-individually":
		settings.forceSeparate = true
	case "allow-and-ignore-individually":
		settings.ignoreIndividually = true
		settings.individuallyAllowed = false
	case "allow-any":
		// default
	case "deny":
		settings.deny = true
	default:
		settings.deny = true
	}
	return settings
}

func applyTrackDefaults(ctx *Context, opt *trackOptions) {
	if opt == nil || ctx == nil || ctx.Config == nil {
		return
	}
	normalizeRange(&opt.MinIV, &opt.MaxIV, -1, 100)
	normalizeRange(&opt.MinCP, &opt.MaxCP, 0, 9000)
	normalizeRange(&opt.MinLevel, &opt.MaxLevel, 0, 55)
	normalizeRange(&opt.MinAtk, &opt.MaxAtk, 0, 15)
	normalizeRange(&opt.MinDef, &opt.MaxDef, 0, 15)
	normalizeRange(&opt.MinSta, &opt.MaxSta, 0, 15)
	normalizeRange(&opt.MinWeight, &opt.MaxWeight, 0, 9000000)
	normalizeRange(&opt.MinRarity, &opt.MaxRarity, -1, 6)
	normalizeRange(&opt.MinSize, &opt.MaxSize, -1, 5)
	if opt.MinTime < 0 {
		opt.MinTime = 0
	}
	if opt.PvpLeague != 0 {
		maxRank := getIntConfig(ctx.Config, "pvp.pvpFilterMaxRank", 4096)
		if maxRank > 4096 {
			maxRank = 4096
		}
		if maxRank > 0 && opt.PvpWorst > maxRank {
			opt.PvpWorst = maxRank
		}
		if opt.PvpBest < 1 {
			opt.PvpBest = 1
		}
		if opt.PvpWorst < 1 {
			opt.PvpWorst = 1
		}
		if opt.PvpBest > opt.PvpWorst {
			opt.PvpBest, opt.PvpWorst = opt.PvpWorst, opt.PvpBest
		}
		switch opt.PvpLeague {
		case 500:
			min := getIntConfig(ctx.Config, "pvp.pvpFilterLittleMinCP", 0)
			if opt.PvpMinCP < min {
				opt.PvpMinCP = min
			}
		case 1500:
			min := getIntConfig(ctx.Config, "pvp.pvpFilterGreatMinCP", 0)
			if opt.PvpMinCP < min {
				opt.PvpMinCP = min
			}
		case 2500:
			min := getIntConfig(ctx.Config, "pvp.pvpFilterUltraMinCP", 0)
			if opt.PvpMinCP < min {
				opt.PvpMinCP = min
			}
		}
		if opt.PvpCap == 0 {
			opt.PvpCap = getIntConfig(ctx.Config, "tracking.defaultUserTrackingLevelCap", 0)
		}
	}
}

func normalizeRange(min, max *int, minAllowed, maxAllowed int) {
	if min == nil || max == nil {
		return
	}
	if *min < minAllowed {
		*min = minAllowed
	}
	if *min > maxAllowed {
		*min = maxAllowed
	}
	if *max < minAllowed {
		*max = minAllowed
	}
	if *max > maxAllowed {
		*max = maxAllowed
	}
	if *max < *min {
		*max = *min
	}
}

func parseRangeOptionalMax(arg string, re *regexp.Regexp) (int, int, bool, bool) {
	if !re.MatchString(arg) {
		return 0, 0, false, false
	}
	match := re.FindStringSubmatch(arg)
	if len(match) < 3 {
		return 0, 0, false, false
	}
	values := strings.Split(match[2], "-")
	if len(values) == 1 {
		val := toInt(values[0], 0)
		return val, 0, true, false
	}
	return toInt(values[0], 0), toInt(values[1], 0), true, true
}

func minimumIvZero(opt *trackOptions) bool {
	if opt == nil {
		return false
	}
	return opt.MinCP != 0 || opt.MaxCP != 9000 || opt.MinLevel != 0 || opt.MaxLevel != 55 ||
		opt.MinAtk != 0 || opt.MaxAtk != 15 || opt.MinDef != 0 || opt.MaxDef != 15 ||
		opt.MinSta != 0 || opt.MaxSta != 15 || opt.MinWeight != 0 || opt.MaxWeight != 9000000
}

type trackMonster struct {
	ID     int
	FormID int
}

func buildTrackMonsters(ctx *Context, ids []int, formNames []string, includeAllForms bool) []trackMonster {
	if len(ids) == 0 {
		return nil
	}
	if containsStringInt(ids, 0) && len(formNames) == 0 && !includeAllForms {
		return []trackMonster{{ID: 0, FormID: 0}}
	}
	if includeAllForms && ctx != nil && ctx.Data != nil && !containsStringInt(ids, 0) {
		idSet := map[int]bool{}
		for _, id := range ids {
			idSet[id] = true
		}
		baseForm := map[int]bool{}
		for _, raw := range ctx.Data.Monsters {
			mon, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := toInt(mon["id"], 0)
			if !idSet[id] {
				continue
			}
			form, _ := mon["form"].(map[string]any)
			if toInt(form["id"], 0) == 0 {
				baseForm[id] = true
			}
		}
		out := []trackMonster{}
		for _, raw := range ctx.Data.Monsters {
			mon, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := toInt(mon["id"], 0)
			if !idSet[id] {
				continue
			}
			form, _ := mon["form"].(map[string]any)
			formName := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", form["name"])))
			if baseForm[id] && formName == "normal" && toInt(form["id"], 0) != 0 {
				continue
			}
			out = append(out, trackMonster{ID: id, FormID: toInt(form["id"], 0)})
		}
		if len(out) > 0 {
			return out
		}
	}
	formLookup := map[string]bool{}
	for _, name := range formNames {
		formLookup[strings.ToLower(name)] = true
	}
	out := []trackMonster{}
	if len(formLookup) > 0 && ctx != nil && ctx.Data != nil {
		idSet := map[int]bool{}
		for _, id := range ids {
			idSet[id] = true
		}
		for _, raw := range ctx.Data.Monsters {
			mon, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			id := toInt(mon["id"], 0)
			if !idSet[id] {
				continue
			}
			form, _ := mon["form"].(map[string]any)
			formName := strings.ToLower(fmt.Sprintf("%v", form["name"]))
			if formLookup[formName] {
				out = append(out, trackMonster{ID: id, FormID: toInt(form["id"], 0)})
			}
		}
		return out
	}
	for _, id := range ids {
		out = append(out, trackMonster{ID: id, FormID: 0})
	}
	return out
}

func allBaseMonsterIDs(ctx *Context) []int {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	ids := []int{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		ids = append(ids, toInt(mon["id"], 0))
	}
	return uniqueInts(ids)
}

func filterMonsterIDs(ctx *Context, ids []int, typeFilters []string, genMin int, genMax int) []int {
	if ctx == nil || ctx.Data == nil {
		return ids
	}
	idSet := map[int]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	typeLookup := map[string]bool{}
	for _, t := range typeFilters {
		typeLookup[strings.ToLower(t)] = true
	}
	filtered := []int{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id := toInt(mon["id"], 0)
		form, _ := mon["form"].(map[string]any)
		if toInt(form["id"], 0) != 0 {
			continue
		}
		if len(idSet) > 0 && !idSet[id] {
			continue
		}
		if genMin > 0 && genMax > 0 && (id < genMin || id > genMax) {
			continue
		}
		if len(typeLookup) > 0 {
			matches := false
			if types, ok := mon["types"].([]any); ok {
				for _, t := range types {
					if tm, ok := t.(map[string]any); ok {
						name := strings.ToLower(fmt.Sprintf("%v", tm["name"]))
						if typeLookup[name] {
							matches = true
							break
						}
					}
				}
			}
			if !matches {
				continue
			}
		}
		filtered = append(filtered, id)
	}
	return uniqueInts(filtered)
}

func monsterIDsFromArgs(ctx *Context, args []string) []int {
	ids := lookupMonsterIDs(ctx, args)
	for _, arg := range args {
		if !strings.HasSuffix(arg, "+") {
			continue
		}
		base := strings.TrimSuffix(arg, "+")
		base = strings.ToLower(ctx.I18n.ReverseTranslateCommand(base, true))
		if mon := findMonsterByNameOrID(ctx, base); mon != nil {
			id := toInt(mon["id"], 0)
			ids = append(ids, id)
			addEvolutions(ctx, mon, &ids, 0)
		}
	}
	return uniqueInts(ids)
}

func findMonsterByNameOrID(ctx *Context, query string) map[string]any {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
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
		id := fmt.Sprintf("%v", mon["id"])
		if query == name || query == id {
			return mon
		}
	}
	return nil
}

func addEvolutions(ctx *Context, mon map[string]any, ids *[]int, depth int) {
	if ctx == nil || ctx.Data == nil || mon == nil || depth > 20 {
		return
	}
	raw, ok := mon["evolutions"].([]any)
	if !ok {
		return
	}
	for _, entry := range raw {
		evo, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		evoID := toInt(evo["evoId"], 0)
		formID := toInt(evo["id"], 0)
		if evoID == 0 {
			continue
		}
		*ids = append(*ids, evoID)
		key := fmt.Sprintf("%d_%d", evoID, formID)
		if next, ok := ctx.Data.Monsters[key].(map[string]any); ok {
			addEvolutions(ctx, next, ids, depth+1)
		}
	}
}

func parseRarityRange(ctx *Context, raw string) (int, int) {
	parts := strings.Split(raw, "-")
	if len(parts) == 1 {
		value := parseRarityValue(ctx, parts[0], -1)
		return value, value
	}
	return parseRarityValue(ctx, parts[0], -1), parseRarityValue(ctx, parts[1], 6)
}

func parseRarityValue(ctx *Context, raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if value := toInt(raw, -999); value != -999 {
		return value
	}
	if ctx == nil || ctx.Data == nil || ctx.Data.UtilData == nil {
		return fallback
	}
	if rarity, ok := ctx.Data.UtilData["rarity"].(map[string]any); ok {
		for key, value := range rarity {
			if strings.EqualFold(fmt.Sprintf("%v", value), raw) {
				return toInt(key, fallback)
			}
		}
	}
	return fallback
}

func parseSizeRange(ctx *Context, raw string) (int, int) {
	parts := strings.Split(raw, "-")
	if len(parts) == 1 {
		value := parseSizeValue(ctx, parts[0], -1)
		return value, value
	}
	return parseSizeValue(ctx, parts[0], -1), parseSizeValue(ctx, parts[1], 5)
}

func parseSizeValue(ctx *Context, raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if value := toInt(raw, -999); value != -999 {
		return value
	}
	if ctx == nil || ctx.Data == nil || ctx.Data.UtilData == nil {
		return fallback
	}
	if sizes, ok := ctx.Data.UtilData["size"].(map[string]any); ok {
		for key, value := range sizes {
			if strings.EqualFold(fmt.Sprintf("%v", value), raw) {
				return toInt(key, fallback)
			}
		}
	}
	return fallback
}

func monsterDiffKeys(existing map[string]any, desired map[string]any) []string {
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

func monsterDiffIsUpdate(diffs []string) bool {
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
	return other == "min_iv" || other == "distance" || other == "template" || other == "clean"
}

func getIntConfig(cfg *config.Config, path string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	if value, ok := cfg.GetInt(path); ok {
		return value
	}
	return fallback
}
