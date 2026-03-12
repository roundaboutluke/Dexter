package command

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"poraclego/internal/i18n"
)

func infoRarity(ctx *Context) string {
	tr := ctx.I18n.Translator(ctx.Language)
	if ctx.Stats == nil {
		return tr.Translate("Rarity information not yet calculated - wait a few minutes and try again", false)
	}
	report, ok := ctx.Stats.LatestReport()
	if !ok {
		report = ctx.Stats.Calculate(ctx.Config)
		ctx.Stats.StoreReport(report)
	}
	lines := []string{}
	rarityNames := map[int]string{}
	if rarityRaw, ok := ctx.Data.UtilData["rarity"].(map[string]any); ok {
		for key, value := range rarityRaw {
			rarityNames[toInt(key, 0)] = fmt.Sprintf("%v", value)
		}
	}
	for group := 2; group < 6; group++ {
		names := []string{}
		for _, id := range report.Rarity[group] {
			names = append(names, lookupMonsterNameTranslated(ctx, tr, id))
		}
		label := rarityNames[group]
		if label == "" {
			label = fmt.Sprintf("Group %d", group)
		}
		lines = append(lines, fmt.Sprintf("**%s**: %s", tr.Translate(label, false), strings.Join(names, ", ")))
	}
	if len(lines) == 0 {
		return tr.Translate("Rarity information not yet calculated - wait a few minutes and try again", false)
	}
	return strings.Join(lines, "\n")
}

func infoShiny(ctx *Context) string {
	tr := ctx.I18n.Translator(ctx.Language)
	if ctx.Stats == nil {
		return tr.Translate("Shiny information not yet calculated - wait a few minutes and try again", false)
	}
	report, ok := ctx.Stats.LatestReport()
	if !ok {
		report = ctx.Stats.Calculate(ctx.Config)
		ctx.Stats.StoreReport(report)
	}
	if len(report.Shiny) == 0 {
		return tr.Translate("Shiny information not yet calculated - wait a few minutes and try again", false)
	}
	lines := []string{fmt.Sprintf("**%s**", tr.Translate("Shiny Stats (Last few hours)", false))}
	ids := make([]int, 0, len(report.Shiny))
	for id := range report.Shiny {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		shinyInfo := report.Shiny[id]
		name := lookupMonsterNameTranslated(ctx, tr, id)
		lines = append(lines, fmt.Sprintf("%s: %s %d - %s 1:%.0f",
			name,
			tr.Translate("Seen", false),
			shinyInfo.Seen,
			tr.Translate("Ratio", false),
			shinyInfo.Ratio,
		))
	}
	if len(lines) == 1 {
		return tr.Translate("Shiny information not yet calculated - wait a few minutes and try again", false)
	}
	return strings.Join(lines, "\n")
}

func infoPokemon(ctx *Context, args []string, re *RegexSet) (string, bool) {
	if ctx == nil || ctx.Data == nil || len(args) == 0 {
		return "", false
	}
	tr := ctx.I18n.Translator(ctx.Language)
	platform := normalizePlatform(ctx.Platform)
	query := strings.ToLower(args[0])
	formNames := []string{}
	for _, arg := range args[1:] {
		match := re.Form.FindStringSubmatch(arg)
		if len(match) >= 3 {
			form := strings.ToLower(ctx.I18n.ReverseTranslateCommand(match[2], true))
			formNames = append(formNames, form)
		}
	}
	matches := []map[string]any{}
	for _, raw := range ctx.Data.Monsters {
		mon, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		name := strings.ToLower(fmt.Sprintf("%v", mon["name"]))
		id := fmt.Sprintf("%v", mon["id"])
		if query != name && query != id {
			continue
		}
		if len(formNames) > 0 {
			form, _ := mon["form"].(map[string]any)
			formName := strings.ToLower(fmt.Sprintf("%v", form["name"]))
			if !containsString(formNames, formName) {
				continue
			}
		}
		matches = append(matches, mon)
	}
	if len(matches) == 0 {
		return "", false
	}
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("**%s:**\n", tr.Translate("Available forms", false)))
	for _, mon := range matches {
		form, _ := mon["form"].(map[string]any)
		formName := strings.TrimSpace(fmt.Sprintf("%v", form["name"]))
		line := fmt.Sprintf("%s", tr.Translate(fmt.Sprintf("%v", mon["name"]), false))
		if formName != "" {
			formLabel := fmt.Sprintf("%s:%s", tr.Translate("form", false), tr.Translate(formName, false))
			line = fmt.Sprintf("%s %s", line, strings.ReplaceAll(formLabel, " ", "\\_"))
		}
		builder.WriteString(line + "\n")
	}

	mon := matches[0]
	monID := toInt(mon["id"], 0)
	builder.WriteString("\n")
	builder.WriteString(fmt.Sprintf("**%s:** %d\n", tr.Translate("Pokédex ID", false), monID))

	typeList := extractTypeNames(mon["types"])
	strengths := map[string][]string{}
	weaknesses := map[string]float64{}
	for _, typeName := range typeList {
		typeInfo := typeInfoByName(ctx, typeName)
		strengths[typeName] = typeNamesFromList(typeInfo["strengths"])
		for _, name := range typeNamesFromList(typeInfo["weaknesses"]) {
			weaknesses[name] = multiplyWeakness(weaknesses[name], 2)
		}
		for _, name := range typeNamesFromList(typeInfo["resistances"]) {
			weaknesses[name] = multiplyWeakness(weaknesses[name], 0.5)
		}
		for _, name := range typeNamesFromList(typeInfo["immunes"]) {
			weaknesses[name] = multiplyWeakness(weaknesses[name], 0.25)
		}
	}
	for i, typeName := range typeList {
		typeLabel := "Primary Type"
		if i > 0 {
			typeLabel = "Secondary Type"
		}
		builder.WriteString(fmt.Sprintf("\n**%s:**  %s\n", tr.Translate(typeLabel, false), typeEmojiLabel(ctx, typeName, tr, platform)))
		if weatherName, weatherEmoji := weatherBoostForType(ctx, typeName, tr, platform); weatherName != "" {
			builder.WriteString(fmt.Sprintf("%s: %s %s\n", tr.Translate("Boosted by", false), weatherEmoji, weatherName))
		}
		if list := strengths[typeName]; len(list) > 0 {
			entries := []string{}
			for _, target := range list {
				entries = append(entries, typeEmojiLabel(ctx, target, tr, platform))
			}
			builder.WriteString(fmt.Sprintf("*%s:* %s\n", tr.Translate("Super Effective Against", false), strings.Join(entries, ",  ")))
		}
	}

	typeBuckets := map[string][]string{
		"Vulnerable to":          {},
		"Very vulnerable to":     {},
		"Resistant to":           {},
		"Very resistant to":      {},
		"Extremely resistant to": {},
	}
	for name, value := range weaknesses {
		label := ""
		switch value {
		case 0.125:
			label = "Extremely resistant to"
		case 0.25:
			label = "Very resistant to"
		case 0.5:
			label = "Resistant to"
		case 2:
			label = "Vulnerable to"
		case 4:
			label = "Very vulnerable to"
		}
		if label == "" {
			continue
		}
		typeBuckets[label] = append(typeBuckets[label], typeEmojiLabel(ctx, name, tr, platform))
	}
	builder.WriteString("\n")
	for _, label := range []string{"Vulnerable to", "Very vulnerable to", "Resistant to", "Very resistant to", "Extremely resistant to"} {
		entries := typeBuckets[label]
		if len(entries) == 0 {
			continue
		}
		builder.WriteString(fmt.Sprintf("*%s:* %s\n", tr.Translate(label, false), strings.Join(entries, ",  ")))
	}

	if ctx.Stats != nil {
		report, ok := ctx.Stats.LatestReport()
		if !ok {
			report = ctx.Stats.Calculate(ctx.Config)
			ctx.Stats.StoreReport(report)
		}
		if shinyInfo, ok := report.Shiny[monID]; ok {
			builder.WriteString(fmt.Sprintf("\n**%s**: %d/%d  (1:%.0f)\n", tr.Translate("Shiny Rate", false), shinyInfo.Seen, shinyInfo.Total, shinyInfo.Ratio))
		}
	}

	if stardust := toInt(mon["thirdMoveStardust"], 0); stardust > 0 {
		candy := toInt(mon["thirdMoveCandy"], 0)
		if candy > 0 {
			builder.WriteString(fmt.Sprintf("\n**%s:**\n", tr.Translate("Third Move Cost", false)))
			builder.WriteString(fmt.Sprintf("%d %s\n", candy, tr.Translate("Candies", false)))
			builder.WriteString(fmt.Sprintf("%s %s\n", formatNumber(ctx.Language, stardust), tr.Translate("Stardust", false)))
		}
	}

	if evolutions, ok := mon["evolutions"].([]any); ok && len(evolutions) > 0 {
		builder.WriteString(fmt.Sprintf("\n**%s:**", tr.Translate("Evolutions", false)))
		for _, raw := range evolutions {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			evoID := toInt(entry["evoId"], 0)
			formID := toInt(entry["id"], 0)
			name := lookupMonsterNameWithForm(ctx, evoID, formID)
			candy := toInt(entry["candyCost"], 0)
			builder.WriteString(fmt.Sprintf("\n%s (%d %s)", tr.Translate(name, false), candy, tr.Translate("Candies", false)))
			if item := strings.TrimSpace(fmt.Sprintf("%v", entry["itemRequirement"])); item != "" && item != "<nil>" {
				builder.WriteString(fmt.Sprintf("\n- %s: %s", tr.Translate("Needed Item", false), tr.Translate(item, false)))
			}
			if getBool(entry["mustBeBuddy"]) {
				builder.WriteString(fmt.Sprintf("\n\u2705 %s", tr.Translate("Must Be Buddy", false)))
			}
			if getBool(entry["onlyNighttime"]) {
				builder.WriteString(fmt.Sprintf("\n\u2705 %s", tr.Translate("Only Nighttime", false)))
			}
			if getBool(entry["onlyDaytime"]) {
				builder.WriteString(fmt.Sprintf("\n\u2705 %s", tr.Translate("Only Daytime", false)))
			}
			if getBool(entry["tradeBonus"]) {
				builder.WriteString(fmt.Sprintf("\n\u2705 %s", tr.Translate("Trade Bonus", false)))
			}
			if questRaw, ok := entry["questRequirement"].(map[string]any); ok {
				if key := strings.TrimSpace(fmt.Sprintf("%v", questRaw["i18n"])); key != "" {
					target := fmt.Sprintf("%v", questRaw["target"])
					builder.WriteString(fmt.Sprintf("\n%s: %s", tr.Translate("Special Requirement", false), strings.ReplaceAll(tr.Translate(key, false), "{{amount}}", target)))
				}
			}
			builder.WriteString("\n")
		}
	}

	builder.WriteString("\n\U0001F4AF:\n")
	for _, level := range []int{15, 20, 25, 40, 50} {
		if cp := perfectCP(ctx, mon, level); cp > 0 {
			builder.WriteString(fmt.Sprintf("%s\n", tr.TranslateFormat("Level {0} CP {1}", level, cp)))
		}
	}
	return strings.TrimSpace(builder.String()), true
}

func lookupMonsterName(ctx *Context, id int) string {
	for _, raw := range ctx.Data.Monsters {
		if mon, ok := raw.(map[string]any); ok {
			if toInt(mon["id"], 0) == id {
				form, _ := mon["form"].(map[string]any)
				if toInt(form["id"], 0) == 0 {
					return fmt.Sprintf("%v", mon["name"])
				}
			}
		}
	}
	return fmt.Sprintf("Unknown monster %d", id)
}

func formatUptime(d time.Duration) string {
	seconds := int(d.Seconds())
	pad := func(value int) string {
		if value < 10 {
			return fmt.Sprintf("0%d", value)
		}
		return strconv.Itoa(value)
	}
	days := seconds / (24 * 60 * 60)
	seconds %= 24 * 60 * 60
	hours := seconds / (60 * 60)
	seconds %= 60 * 60
	minutes := seconds / 60
	seconds %= 60
	return fmt.Sprintf("%s:%s:%s:%s", pad(days), pad(hours), pad(minutes), pad(seconds))
}

func lookupMonsterNameTranslated(ctx *Context, tr *i18n.Translator, id int) string {
	if ctx == nil || ctx.Data == nil {
		if tr != nil {
			return fmt.Sprintf("%s %d", tr.Translate("Unknown monster", false), id)
		}
		return fmt.Sprintf("Unknown monster %d", id)
	}
	if tr == nil {
		return lookupMonsterName(ctx, id)
	}
	for _, raw := range ctx.Data.Monsters {
		if mon, ok := raw.(map[string]any); ok {
			if toInt(mon["id"], 0) == id {
				form, _ := mon["form"].(map[string]any)
				if toInt(form["id"], 0) == 0 {
					return tr.Translate(fmt.Sprintf("%v", mon["name"]), false)
				}
			}
		}
	}
	return fmt.Sprintf("%s %d", tr.Translate("Unknown monster", false), id)
}

func lookupMonsterNameWithForm(ctx *Context, id, formID int) string {
	key := fmt.Sprintf("%d_%d", id, formID)
	if raw, ok := ctx.Data.Monsters[key]; ok {
		if mon, ok := raw.(map[string]any); ok {
			return fmt.Sprintf("%v", mon["name"])
		}
	}
	return lookupMonsterName(ctx, id)
}

func typeInfoByName(ctx *Context, typeName string) map[string]any {
	if ctx == nil || ctx.Data == nil || ctx.Data.Types == nil {
		return map[string]any{}
	}
	raw, ok := ctx.Data.Types[typeName]
	if !ok {
		return map[string]any{}
	}
	entry, _ := raw.(map[string]any)
	if entry == nil {
		return map[string]any{}
	}
	return entry
}

func typeNamesFromList(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", entry["typeName"]))
		if name != "" && name != "<nil>" {
			out = append(out, name)
		}
	}
	return out
}

func extractTypeNames(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := []string{}
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name := strings.TrimSpace(fmt.Sprintf("%v", entry["name"]))
		if name != "" && name != "<nil>" {
			out = append(out, name)
		}
	}
	return out
}

func multiplyWeakness(value float64, factor float64) float64 {
	if value == 0 {
		value = 1
	}
	return value * factor
}

func typeEmojiLabel(ctx *Context, typeName string, tr *i18n.Translator, platform string) string {
	emojiKey := ""
	if ctx != nil && ctx.Data != nil && ctx.Data.UtilData != nil {
		if types, ok := ctx.Data.UtilData["types"].(map[string]any); ok {
			if entry, ok := types[typeName].(map[string]any); ok {
				emojiKey = fmt.Sprintf("%v", entry["emoji"])
			}
		}
	}
	emoji := translateMaybe(tr, lookupEmojiByKey(ctx, emojiKey, platform))
	if emoji != "" {
		return fmt.Sprintf("%s %s", emoji, tr.Translate(typeName, false))
	}
	return tr.Translate(typeName, false)
}

func weatherBoostForType(ctx *Context, typeName string, tr *i18n.Translator, platform string) (string, string) {
	if ctx == nil || ctx.Data == nil || ctx.Data.UtilData == nil {
		return "", ""
	}
	types, ok := ctx.Data.UtilData["types"].(map[string]any)
	if !ok {
		return "", ""
	}
	typeEntry, ok := types[typeName].(map[string]any)
	if !ok {
		return "", ""
	}
	typeID := toInt(typeEntry["id"], 0)
	if typeID == 0 {
		return "", ""
	}
	boosts, ok := ctx.Data.UtilData["weatherTypeBoost"].(map[string]any)
	if !ok {
		return "", ""
	}
	weatherID := 0
	for key, value := range boosts {
		list, ok := value.([]any)
		if !ok {
			continue
		}
		for _, entry := range list {
			if toInt(entry, 0) == typeID {
				weatherID, _ = strconv.Atoi(key)
				break
			}
		}
		if weatherID != 0 {
			break
		}
	}
	if weatherID == 0 {
		return "", ""
	}
	weather, ok := ctx.Data.UtilData["weather"].(map[string]any)
	if !ok {
		return "", ""
	}
	weatherEntry, ok := weather[strconv.Itoa(weatherID)].(map[string]any)
	if !ok {
		return "", ""
	}
	name := strings.TrimSpace(fmt.Sprintf("%v", weatherEntry["name"]))
	emojiKey := strings.TrimSpace(fmt.Sprintf("%v", weatherEntry["emoji"]))
	emoji := translateMaybe(tr, lookupEmojiByKey(ctx, emojiKey, platform))
	return capitalize(tr.Translate(name, false)), emoji
}

func perfectCP(ctx *Context, mon map[string]any, level int) int {
	if ctx == nil || ctx.Data == nil || ctx.Data.UtilData == nil {
		return 0
	}
	stats, ok := mon["stats"].(map[string]any)
	if !ok {
		return 0
	}
	cpMult := getCPMultiplier(ctx.Data.UtilData, level)
	if cpMult == 0 {
		return 0
	}
	atk := float64(toInt(stats["baseAttack"], 0) + 15)
	def := float64(toInt(stats["baseDefense"], 0) + 15)
	sta := float64(toInt(stats["baseStamina"], 0) + 15)
	cp := math.Floor(atk * math.Sqrt(def) * math.Sqrt(sta) * cpMult * cpMult / 10.0)
	if cp < 10 {
		cp = 10
	}
	return int(cp)
}

func getCPMultiplier(util map[string]any, level int) float64 {
	raw, ok := util["cpMultipliers"].(map[string]any)
	if !ok {
		return 0
	}
	value, ok := raw[strconv.Itoa(level)]
	if !ok {
		return 0
	}
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func capitalize(value string) string {
	if value == "" {
		return value
	}
	return strings.ToUpper(value[:1]) + value[1:]
}

func formatNumber(language string, value int) string {
	input := strconv.Itoa(value)
	if len(input) <= 3 {
		return input
	}
	var buf strings.Builder
	count := 0
	for i := len(input) - 1; i >= 0; i-- {
		if count == 3 {
			buf.WriteByte(',')
			count = 0
		}
		buf.WriteByte(input[i])
		count++
	}
	out := buf.String()
	runes := []rune(out)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func getBool(value any) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(v)
		return err == nil && parsed
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}
