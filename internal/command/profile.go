package command

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"poraclego/internal/i18n"
	"poraclego/internal/profile"
)

// ProfileCommand manages user profiles.
type ProfileCommand struct{}

func (c *ProfileCommand) Name() string { return "profile" }

func (c *ProfileCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "profile") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}

	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	logic := profile.New(ctx.Query, result.TargetID)
	if err := logic.Init(); err != nil {
		return "", err
	}
	if len(args) == 0 {
		profileName := currentProfileName(logic.Profiles(), result.ProfileNo)
		lines := []string{}
		if profileName == "" {
			lines = append(lines, tr.Translate("You don't have a profile set", false))
		} else {
			lines = append(lines, fmt.Sprintf("%s %s", tr.Translate("Your profile is currently set to:", false), profileName))
		}
		lines = append(lines, tr.TranslateFormat("Valid commands are `{0}profile <name>`, `{0}profile list`, `{0}profile add <name>`, `{0}profile remove <name>`, `{0}profile settime <times>`, `{0}profile schedule <enable|disable|toggle>`, `{0}profile copyto <name>`", ctx.Prefix))
		lines = append(lines, tr.TranslateFormat("`{0}profile settime` supports switches (e.g. `mon0900 tue1300`) and ranges (e.g. `mon:08:00-12:00 weekday:18:00-23:00`). Ranges allow quiet hours.", ctx.Prefix))
		if helpLine := singleLineHelpText(ctx, "profile", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}
	switch strings.ToLower(args[0]) {
	case "add":
		if len(args) < 2 || strings.EqualFold(args[1], "all") {
			return tr.Translate("That is not a valid profile name", false), nil
		}
		if profileNameExists(logic.Profiles(), args[1]) {
			return tr.Translate("That profile name already exists", false), nil
		}
		if err := logic.AddProfile(args[1], "{}"); err != nil {
			return "", err
		}
		return tr.Translate("Profile added.", false), nil
	case "remove":
		if len(args) < 2 {
			return tr.Translate("That is not a valid profile name", false), nil
		}
		profileNo, errMsg := resolveProfileNumber(logic, args[1])
		if errMsg != "" {
			return tr.Translate(errMsg, false), nil
		}
		if err := logic.DeleteProfile(profileNo); err != nil {
			return "", err
		}
		return tr.Translate("Profile removed.", false), nil
	case "list":
		return profileList(ctx, tr, logic, result.ProfileNo), nil
	case "copyto":
		if len(args) < 2 {
			return tr.Translate("No profiles specified.", false), nil
		}
		currentName := profileNameByNo(logic.Profiles(), result.ProfileNo)
		valid := []string{}
		invalid := []string{}
		for _, arg := range args[1:] {
			if (strings.EqualFold(arg, "all") || profileNameExists(logic.Profiles(), arg)) && !strings.EqualFold(arg, currentName) {
				valid = append(valid, arg)
			} else if !strings.EqualFold(arg, "copyto") {
				invalid = append(invalid, arg)
			}
		}
		targetNumbers := resolveProfileTargets(logic, valid)
		for _, dest := range targetNumbers {
			if dest == result.ProfileNo {
				continue
			}
			if err := logic.CopyProfile(result.ProfileNo, dest); err != nil {
				return "", err
			}
		}
		message := ""
		if len(targetNumbers) > 0 {
			targetNames := []string{}
			for _, name := range valid {
				if strings.EqualFold(name, "all") {
					continue
				}
				targetNames = append(targetNames, name)
			}
			message = fmt.Sprintf("%s%s.", tr.Translate("Current profile copied to: ", false), strings.Join(targetNames, ", "))
			if containsString(valid, "all") {
				message += " (all)"
			}
		}
		if len(targetNumbers) == 0 {
			message = tr.Translate("No valid profiles specified.", false)
		}
		if len(invalid) > 0 {
			message = strings.TrimSpace(message + fmt.Sprintf("\n%s%s.", tr.Translate("These profiles were invalid: ", false), strings.Join(invalid, ", ")))
			if containsString(invalid, currentName) {
				message += "\n" + tr.Translate("Cannot copy over the currently active profile.", false)
			}
		}
		return strings.TrimSpace(message), nil
	case "settime":
		payload, errText := parseProfileSettime(ctx, tr, logic, result, args[1:])
		if errText != "" {
			return errText, nil
		}
		if err := logic.UpdateHours(result.ProfileNo, payload); err != nil {
			return "", err
		}
		return tr.Translate("Profile active hours updated.", false), nil
	case "schedule":
		return handleProfileScheduleToggle(ctx, tr, logic, result, args[1:])
	default:
		if len(args) == 0 {
			profileName := currentProfileName(logic.Profiles(), result.ProfileNo)
			lines := []string{}
			if profileName == "" {
				lines = append(lines, tr.Translate("You don't have a profile set", false))
			} else {
				lines = append(lines, fmt.Sprintf("%s %s", tr.Translate("Your profile is currently set to:", false), profileName))
			}
			lines = append(lines, tr.TranslateFormat("Valid commands are `{0}profile <name>`, `{0}profile list`, `{0}profile add <name>`, `{0}profile remove <name>`, `{0}profile settime <times>`, `{0}profile schedule <enable|disable|toggle>`, `{0}profile copyto <name>`", ctx.Prefix))
			lines = append(lines, tr.TranslateFormat("`{0}profile settime` supports switches (e.g. `mon0900 tue1300`) and ranges (e.g. `mon:08:00-12:00 weekday:18:00-23:00`). Ranges allow quiet hours.", ctx.Prefix))
			if helpLine := singleLineHelpText(ctx, "profile", result.Language, result.Target); helpLine != "" {
				lines = append(lines, helpLine)
			}
			return strings.Join(lines, "\n"), nil
		}
		profileNo, errMsg := resolveProfileNumber(logic, args[0])
		if errMsg != "" {
			return tr.Translate("I can't find that profile", false), nil
		}
		selected := profileByNo(logic.Profiles(), profileNo)
		update := map[string]any{"current_profile_no": profileNo}
		if selected != nil {
			update["area"] = selected["area"]
			update["latitude"] = selected["latitude"]
			update["longitude"] = selected["longitude"]
		}
		if _, err := ctx.Query.UpdateQuery("humans", update, map[string]any{"id": result.TargetID}); err != nil {
			return "", err
		}
		return tr.Translate("Profile set.", false), nil
	}
}

func resolveProfileNumber(logic *profile.Logic, token string) (int, string) {
	if token == "" {
		return 0, "That is not a valid profile name"
	}
	if num, err := strconv.Atoi(token); err == nil && num > 0 {
		if profileByNo(logic.Profiles(), num) != nil {
			return num, ""
		}
		return 0, "That is not a valid profile number"
	}
	for _, row := range logic.Profiles() {
		if strings.EqualFold(fmt.Sprintf("%v", row["name"]), token) {
			return toInt(row["profile_no"], 0), ""
		}
	}
	return 0, "That is not a valid profile name"
}

func profileList(ctx *Context, tr *i18n.Translator, logic *profile.Logic, currentProfile int) string {
	rows := logic.Profiles()
	if len(rows) == 0 {
		return "You do not have any profiles"
	}
	lines := []string{"Currently configured profiles are:"}
	for _, row := range rows {
		name := fmt.Sprintf("%v", row["name"])
		number := toInt(row["profile_no"], 0)
		marker := "."
		if number == currentProfile {
			marker = "*"
		}
		line := fmt.Sprintf("%d%s %s", number, marker, name)
		if area := fmt.Sprintf("%v", row["area"]); area != "" && area != "[]" {
			line += fmt.Sprintf(" - areas: %s", area)
		}
		lat := toFloat(row["latitude"])
		lon := toFloat(row["longitude"])
		if lat != 0 || lon != 0 {
			line += fmt.Sprintf(" - location: %.5f,%.5f", lat, lon)
		}
		lines = append(lines, line)

		if hours := fmt.Sprintf("%v", row["active_hours"]); len(hours) > 3 {
			lines = append(lines, formatProfileTimes(tr, hours)...)
		}
	}
	return strings.Join(lines, "\n")
}

func parseProfileTimes(args []string, re *RegexSet) []map[string]any {
	out := []map[string]any{}
	for _, arg := range args {
		if match := re.Mon.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 1, match[3], match[5])
		}
		if match := re.Tue.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 2, match[3], match[5])
		}
		if match := re.Wed.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 3, match[3], match[5])
		}
		if match := re.Thu.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 4, match[3], match[5])
		}
		if match := re.Fri.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 5, match[3], match[5])
		}
		if match := re.Sat.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 6, match[3], match[5])
		}
		if match := re.Sun.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 7, match[3], match[5])
		}
		if match := re.Weekday.FindStringSubmatch(arg); len(match) >= 6 {
			for day := 1; day <= 5; day++ {
				out = addProfileTime(out, day, match[3], match[5])
			}
		}
		if match := re.Weekend.FindStringSubmatch(arg); len(match) >= 6 {
			out = addProfileTime(out, 6, match[3], match[5])
			out = addProfileTime(out, 7, match[3], match[5])
		}
	}
	return out
}

type profileScheduleRange struct {
	Day      int
	StartMin int
	EndMin   int
}

func handleProfileScheduleToggle(ctx *Context, tr *i18n.Translator, logic *profile.Logic, result TargetResult, args []string) (string, error) {
	if ctx == nil || ctx.Query == nil {
		return "🙅", nil
	}
	human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": result.TargetID})
	if err != nil || human == nil {
		return tr.Translate("Target is not registered.", false), nil
	}
	disabled := toInt(human["schedule_disabled"], 0) == 1
	action := strings.ToLower(strings.TrimSpace(firstArg(args)))
	if action == "" || action == "status" {
		value := tr.Translate("Enabled", false)
		if disabled {
			value = tr.Translate("Disabled", false)
		}
		return fmt.Sprintf("%s: %s", tr.Translate("Scheduler", false), value), nil
	}
	switch action {
	case "enable", "on":
		disabled = false
	case "disable", "off":
		disabled = true
	case "toggle":
		disabled = !disabled
	default:
		return tr.TranslateFormat("Valid commands are `{0}profile schedule status`, `{0}profile schedule enable`, `{0}profile schedule disable`, `{0}profile schedule toggle`", ctx.Prefix), nil
	}

	update := map[string]any{"schedule_disabled": 0}
	if disabled {
		update["schedule_disabled"] = 1
		if toInt(human["current_profile_no"], 0) == 0 {
			lowest := 0
			for _, row := range logic.Profiles() {
				no := toInt(row["profile_no"], 0)
				if no > 0 && (lowest == 0 || no < lowest) {
					lowest = no
				}
			}
			if lowest == 0 {
				lowest = 1
			}
			update["current_profile_no"] = lowest
		}
	}
	if _, err := ctx.Query.UpdateQuery("humans", update, map[string]any{"id": result.TargetID}); err != nil {
		return "", err
	}
	if disabled {
		return tr.Translate("Scheduler disabled.", false), nil
	}
	return tr.Translate("Scheduler enabled.", false), nil
}

func parseProfileSettime(ctx *Context, tr *i18n.Translator, logic *profile.Logic, result TargetResult, args []string) (any, string) {
	if ctx == nil || ctx.I18n == nil {
		return nil, "🙅"
	}
	legacy, ranges := parseProfileTimeTokens(ctx.I18n, args)
	if len(legacy) > 0 && len(ranges) > 0 {
		return nil, tr.Translate("Please do not mix switch and range schedules in the same command.", false)
	}
	if len(ranges) > 0 {
		if errText := validateRangeSchedule(ctx, tr, logic, result, ranges); errText != "" {
			return nil, errText
		}
		return ranges, ""
	}
	return legacy, ""
}

func parseProfileTimeTokens(factory *i18n.Factory, args []string) ([]map[string]any, []map[string]any) {
	legacy := []map[string]any{}
	ranges := []map[string]any{}
	for _, arg := range args {
		dayKey, rest, ok := parseDayPrefix(factory, arg)
		if !ok {
			continue
		}
		days := daysFromKey(dayKey)
		if len(days) == 0 {
			continue
		}
		if strings.Contains(rest, "-") {
			start, end, ok := parseClockRange(rest)
			if !ok || end <= start {
				continue
			}
			for _, day := range days {
				ranges = append(ranges, map[string]any{
					"day":         day,
					"start_hours": start / 60,
					"start_mins":  start % 60,
					"end_hours":   end / 60,
					"end_mins":    end % 60,
				})
			}
			continue
		}
		minutes, ok := parseClockMinutesFlexible(rest)
		if !ok {
			continue
		}
		for _, day := range days {
			legacy = append(legacy, map[string]any{"day": day, "hours": minutes / 60, "mins": minutes % 60})
		}
	}
	return legacy, ranges
}

func parseDayPrefix(factory *i18n.Factory, arg string) (string, string, bool) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return "", "", false
	}
	keys := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun", "weekday", "weekend"}
	for _, key := range keys {
		variants := append([]string{key}, factory.TranslateCommand(key)...)
		for _, v := range variants {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			if len(arg) < len(v) {
				continue
			}
			if !strings.EqualFold(arg[:len(v)], v) {
				continue
			}
			rest := strings.TrimSpace(arg[len(v):])
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimSpace(rest)
			return key, rest, true
		}
	}
	return "", "", false
}

func daysFromKey(key string) []int {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "mon":
		return []int{1}
	case "tue":
		return []int{2}
	case "wed":
		return []int{3}
	case "thu":
		return []int{4}
	case "fri":
		return []int{5}
	case "sat":
		return []int{6}
	case "sun":
		return []int{7}
	case "weekday":
		return []int{1, 2, 3, 4, 5}
	case "weekend":
		return []int{6, 7}
	default:
		return nil
	}
}

func parseClockRange(value string) (int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	start, ok := parseClockMinutesFlexible(parts[0])
	if !ok {
		return 0, 0, false
	}
	end, ok := parseClockMinutesFlexible(parts[1])
	if !ok {
		return 0, 0, false
	}
	return start, end, true
}

func parseClockMinutesFlexible(value string) (int, bool) {
	value = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(value), ":"))
	if value == "" {
		return 0, true
	}
	if strings.Contains(value, ":") {
		parts := strings.Split(value, ":")
		if len(parts) != 2 {
			return 0, false
		}
		h := toInt(parts[0], -1)
		m := toInt(parts[1], -1)
		if h < 0 || h > 23 || m < 0 || m > 59 {
			return 0, false
		}
		return h*60 + m, true
	}
	digits := strings.TrimSpace(value)
	if len(digits) <= 2 {
		h := toInt(digits, -1)
		if h < 0 || h > 23 {
			return 0, false
		}
		return h * 60, true
	}
	if len(digits) == 3 {
		h := toInt(digits[:1], -1)
		m := toInt(digits[1:], -1)
		if h < 0 || h > 23 || m < 0 || m > 59 {
			return 0, false
		}
		return h*60 + m, true
	}
	if len(digits) == 4 {
		h := toInt(digits[:2], -1)
		m := toInt(digits[2:], -1)
		if h < 0 || h > 23 || m < 0 || m > 59 {
			return 0, false
		}
		return h*60 + m, true
	}
	return 0, false
}

func validateRangeSchedule(ctx *Context, tr *i18n.Translator, logic *profile.Logic, result TargetResult, ranges []map[string]any) string {
	if ctx == nil || ctx.Query == nil {
		return "🙅"
	}
	// Sanity check: no overlaps within this profile's schedule.
	perDay := map[int][]profileScheduleRange{}
	for _, entry := range ranges {
		day := toInt(entry["day"], 0)
		start := toInt(entry["start_hours"], 0)*60 + toInt(entry["start_mins"], 0)
		end := toInt(entry["end_hours"], 0)*60 + toInt(entry["end_mins"], 0)
		if day < 1 || day > 7 || end <= start {
			return tr.Translate("One or more schedule ranges were invalid.", false)
		}
		perDay[day] = append(perDay[day], profileScheduleRange{Day: day, StartMin: start, EndMin: end})
	}
	for _, list := range perDay {
		sortScheduleRanges(list)
		for i := 1; i < len(list); i++ {
			if list[i].StartMin < list[i-1].EndMin {
				return tr.Translate("One or more schedule ranges overlap.", false)
			}
		}
	}
	// Best-effort: prevent overlaps with other profiles if they already use range schedules.
	// If overlaps exist, the scheduler will refuse to switch, so we block here.
	userID := strings.TrimSpace(result.TargetID)
	if userID == "" {
		return ""
	}
	profiles := logic.Profiles()
	if profiles == nil || len(profiles) == 0 {
		var err error
		profiles, err = ctx.Query.SelectAllQuery("profiles", map[string]any{"id": userID})
		if err != nil {
			return ""
		}
	}
	profileNo := result.ProfileNo
	if profileNo == 0 {
		profileNo = 1
	}
	otherRanges := []struct {
		ProfileNo int
		Name      string
		Range     profileScheduleRange
	}{}
	for _, row := range profiles {
		no := toInt(row["profile_no"], 0)
		if no == 0 || no == profileNo {
			continue
		}
		for _, r := range parseScheduleRangesFromRaw(row["active_hours"]) {
			name := strings.TrimSpace(fmt.Sprintf("%v", row["name"]))
			if name == "" {
				name = fmt.Sprintf("Profile %d", no)
			}
			otherRanges = append(otherRanges, struct {
				ProfileNo int
				Name      string
				Range     profileScheduleRange
			}{ProfileNo: no, Name: name, Range: r})
		}
	}
	for _, entry := range ranges {
		day := toInt(entry["day"], 0)
		start := toInt(entry["start_hours"], 0)*60 + toInt(entry["start_mins"], 0)
		end := toInt(entry["end_hours"], 0)*60 + toInt(entry["end_mins"], 0)
		for _, other := range otherRanges {
			if other.Range.Day != day {
				continue
			}
			if start < other.Range.EndMin && other.Range.StartMin < end {
				return fmt.Sprintf("%s %s: %s", tr.Translate("That overlaps with existing schedules in", false), other.Name, formatScheduleRangeLabel(day, other.Range.StartMin, other.Range.EndMin))
			}
		}
	}
	return ""
}

func parseScheduleRangesFromRaw(raw any) []profileScheduleRange {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if len(text) <= 2 {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	out := []profileScheduleRange{}
	for _, row := range rows {
		day := toInt(row["day"], 0)
		if day < 1 || day > 7 {
			continue
		}
		if _, ok := row["start_hours"]; !ok {
			continue
		}
		start := toInt(row["start_hours"], 0)*60 + toInt(row["start_mins"], 0)
		end := toInt(row["end_hours"], 0)*60 + toInt(row["end_mins"], 0)
		if end <= start {
			continue
		}
		out = append(out, profileScheduleRange{Day: day, StartMin: start, EndMin: end})
	}
	return out
}

func sortScheduleRanges(ranges []profileScheduleRange) {
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Day != ranges[j].Day {
			return ranges[i].Day < ranges[j].Day
		}
		return ranges[i].StartMin < ranges[j].StartMin
	})
}

func formatScheduleRangeLabel(day, startMin, endMin int) string {
	dayLabel := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}[day-1]
	return fmt.Sprintf("%s %02d:%02d-%02d:%02d", dayLabel, startMin/60, startMin%60, endMin/60, endMin%60)
}

func addProfileTime(out []map[string]any, day int, hours string, mins string) []map[string]any {
	h := toInt(hours, 0)
	m := toInt(mins, 0)
	out = append(out, map[string]any{"day": day, "hours": h, "mins": m})
	return out
}

func resolveProfileTargets(logic *profile.Logic, names []string) []int {
	targets := []int{}
	profiles := logic.Profiles()
	for _, name := range names {
		if name == "all" {
			for _, row := range profiles {
				no := toInt(row["profile_no"], 0)
				if no > 0 {
					targets = append(targets, no)
				}
			}
			continue
		}
		for _, row := range profiles {
			if strings.EqualFold(fmt.Sprintf("%v", row["name"]), name) {
				no := toInt(row["profile_no"], 0)
				if no > 0 {
					targets = append(targets, no)
				}
			}
		}
	}
	return targets
}

func profileNameExists(rows []map[string]any, name string) bool {
	for _, row := range rows {
		if strings.EqualFold(fmt.Sprintf("%v", row["name"]), name) {
			return true
		}
	}
	return false
}

func profileByNo(rows []map[string]any, number int) map[string]any {
	for _, row := range rows {
		if toInt(row["profile_no"], 0) == number {
			return row
		}
	}
	return nil
}

func profileNameByNo(rows []map[string]any, number int) string {
	if row := profileByNo(rows, number); row != nil {
		return fmt.Sprintf("%v", row["name"])
	}
	return ""
}

func currentProfileName(rows []map[string]any, number int) string {
	return profileNameByNo(rows, number)
}

func formatProfileTimes(tr *i18n.Translator, hoursRaw string) []string {
	out := []string{}
	var times []map[string]any
	if err := json.Unmarshal([]byte(hoursRaw), &times); err != nil {
		return out
	}
	dayNames := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	for _, entry := range times {
		day := toInt(entry["day"], 0)
		if day < 1 || day > 7 {
			continue
		}
		label := dayNames[day-1]
		if startHours, ok := entry["start_hours"]; ok {
			startMins := toInt(entry["start_mins"], 0)
			endHours := toInt(entry["end_hours"], 0)
			endMins := toInt(entry["end_mins"], 0)
			out = append(out, fmt.Sprintf("    %s %02d:%02d-%02d:%02d", tr.Translate(label, false), toInt(startHours, 0), startMins, endHours, endMins))
			continue
		}
		hours := toInt(entry["hours"], 0)
		mins := toInt(entry["mins"], 0)
		out = append(out, fmt.Sprintf("    %s %d:%02d (switch)", tr.Translate(label, false), hours, mins))
	}
	return out
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return 0
}
