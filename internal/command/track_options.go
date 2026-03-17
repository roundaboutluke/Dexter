package command

import (
	"fmt"
	"regexp"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
)

type trackOptions struct {
	MinIV           int
	MaxIV           int
	MinCP           int
	MaxCP           int
	MinLevel        int
	MaxLevel        int
	MinAtk          int
	MaxAtk          int
	MinDef          int
	MaxDef          int
	MinSta          int
	MaxSta          int
	MinWeight       int
	MaxWeight       int
	MinRarity       int
	MaxRarity       int
	MinSize         int
	MaxSize         int
	Gender          int
	MinTime         int
	FormID          int
	FormNames       []string
	IncludeAllForms bool
	TypeFilters     []string
	Everything      bool
	EverythingOnly  bool
	PvpCount        int
	GenMin          int
	GenMax          int
	PvpLeague       int
	PvpBest         int
	PvpWorst        int
	PvpMinCP        int
	PvpCap          int
	Error           string
}

type everythingSettings struct {
	deny                bool
	forceSeparate       bool
	individuallyAllowed bool
	ignoreIndividually  bool
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
		if !containsInt(caps, opt.PvpCap) {
			choices := append([]int{0}, caps...)
			opt.Error = tr.TranslateFormat("This level cap is not supported, valid choices are: {0}", joinIntsInt(choices))
		}
	}

	opt.EverythingOnly = opt.Everything &&
		len(opt.TypeFilters) == 0 &&
		len(opt.FormNames) == 0 &&
		!opt.IncludeAllForms &&
		opt.MinIV == -1 &&
		opt.MinCP == 0 &&
		opt.MinLevel == 0 &&
		opt.MinAtk == 0 &&
		opt.MinDef == 0 &&
		opt.MinSta == 0 &&
		opt.MinWeight == 0 &&
		opt.MinRarity == -1 &&
		opt.MinSize == -1
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

func getIntConfig(cfg *config.Config, path string, fallback int) int {
	if cfg == nil {
		return fallback
	}
	if value, ok := cfg.GetInt(path); ok {
		return value
	}
	return fallback
}
