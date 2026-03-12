package command

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"poraclego/internal/i18n"
	"poraclego/internal/profile"
)

func quietHoursActive(human map[string]any) bool {
	if human == nil {
		return false
	}
	return toInt(human["schedule_disabled"], 0) == 0 && toInt(human["current_profile_no"], 1) == 0
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
			if toInt(human["preferred_profile_no"], 0) == 0 {
				update["preferred_profile_no"] = lowest
			}
		}
	}
	if _, err := ctx.Query.UpdateQuery("humans", update, map[string]any{"id": result.TargetID}); err != nil {
		return "", err
	}
	ctx.MarkAlertStateDirty()
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
