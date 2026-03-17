package bot

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"poraclego/internal/config"
	"poraclego/internal/i18n"
)

func profileRowByToken(rows []map[string]any, token string) map[string]any {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if num, err := strconv.Atoi(token); err == nil && num > 0 {
		for _, row := range rows {
			if toInt(row["profile_no"], 0) == num {
				return row
			}
		}
	}
	for _, row := range rows {
		if strings.EqualFold(fmt.Sprintf("%v", row["name"]), token) {
			return row
		}
	}
	return nil
}

func profileRowByNo(rows []map[string]any, number int) map[string]any {
	if number == 0 {
		return nil
	}
	for _, row := range rows {
		if toInt(row["profile_no"], 0) == number {
			return row
		}
	}
	return nil
}

func profileNameExistsRows(rows []map[string]any, name string) bool {
	for _, row := range rows {
		if strings.EqualFold(fmt.Sprintf("%v", row["name"]), name) {
			return true
		}
	}
	return false
}

func fallbackStaticMap(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	value, _ := cfg.GetString("fallbacks.staticMap")
	return strings.TrimSpace(value)
}

func parseProfileAreas(raw any) []string {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if text == "" || text == "[]" {
		return nil
	}
	areas := []string{}
	if err := json.Unmarshal([]byte(text), &areas); err != nil {
		return nil
	}
	out := []string{}
	for _, area := range areas {
		trimmed := strings.TrimSpace(area)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func profileHoursText(tr *i18n.Translator, raw any) string {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if len(text) <= 2 {
		return ""
	}
	var times []map[string]any
	if err := json.Unmarshal([]byte(text), &times); err != nil || len(times) == 0 {
		return ""
	}
	parts := []string{}
	for _, entry := range times {
		day := toInt(entry["day"], 0)
		if day < 1 || day > 7 {
			continue
		}
		hours := toInt(entry["hours"], 0)
		mins := toInt(entry["mins"], 0)
		parts = append(parts, fmt.Sprintf("%s %d:%02d", localizedDayLabel(tr, day), hours, mins))
	}
	return strings.Join(parts, ", ")
}

func profileScheduleText(tr *i18n.Translator, raw any) string {
	entries := scheduleEntriesFromRaw(raw)
	if len(entries) == 0 {
		return ""
	}
	lines := []string{}
	for _, entry := range entries {
		lines = append(lines, scheduleEntryLabel(tr, entry))
	}
	return strings.Join(lines, "\n")
}

type scheduleEntry struct {
	ProfileNo int
	Day       int
	StartMin  int
	EndMin    int
	Legacy    bool
}

func scheduleEntriesFromRaw(raw any) []scheduleEntry {
	text := strings.TrimSpace(fmt.Sprintf("%v", raw))
	if len(text) <= 2 {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	out := []scheduleEntry{}
	for _, row := range rows {
		day := toInt(row["day"], 0)
		if day < 1 || day > 7 {
			continue
		}
		if startHours, ok := row["start_hours"]; ok {
			startMins := toInt(row["start_mins"], 0)
			endHours := toInt(row["end_hours"], 0)
			endMins := toInt(row["end_mins"], 0)
			out = append(out, scheduleEntry{
				Day:      day,
				StartMin: toInt(startHours, 0)*60 + startMins,
				EndMin:   endHours*60 + endMins,
			})
			continue
		}
		if hours, ok := row["hours"]; ok {
			mins := toInt(row["mins"], 0)
			out = append(out, scheduleEntry{
				Day:      day,
				StartMin: toInt(hours, 0)*60 + mins,
				Legacy:   true,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Day != out[j].Day {
			return out[i].Day < out[j].Day
		}
		return out[i].StartMin < out[j].StartMin
	})
	return out
}

func scheduleEntryLabel(tr *i18n.Translator, entry scheduleEntry) string {
	return localizedScheduleEntryLabel(tr, entry)
}

func scheduleEntryValue(entry scheduleEntry) string {
	return fmt.Sprintf("%d|%d|%d|%t", entry.Day, entry.StartMin, entry.EndMin, entry.Legacy)
}

func scheduleRemoveOptions(tr *i18n.Translator, raw any) []discordgo.SelectMenuOption {
	entries := scheduleEntriesFromRaw(raw)
	if len(entries) == 0 {
		return nil
	}
	options := make([]discordgo.SelectMenuOption, 0, len(entries))
	for _, entry := range entries {
		value := scheduleEntryValue(entry)
		options = append(options, discordgo.SelectMenuOption{
			Label: scheduleEntryLabel(tr, entry),
			Value: value,
		})
		if len(options) >= 25 {
			break
		}
	}
	return options
}

func removeScheduleEntry(entries []scheduleEntry, value string) []scheduleEntry {
	parts := strings.Split(value, "|")
	if len(parts) < 4 {
		return entries
	}
	day := toInt(parts[0], 0)
	start := toInt(parts[1], 0)
	end := toInt(parts[2], 0)
	legacy := strings.EqualFold(parts[3], "true")
	out := []scheduleEntry{}
	removed := false
	for _, entry := range entries {
		if !removed && entry.Day == day && entry.StartMin == start && entry.EndMin == end && entry.Legacy == legacy {
			removed = true
			continue
		}
		out = append(out, entry)
	}
	return out
}

func encodeScheduleEntries(entries []scheduleEntry) string {
	rows := []map[string]any{}
	for _, entry := range entries {
		if entry.Legacy {
			rows = append(rows, map[string]any{
				"day":   entry.Day,
				"hours": entry.StartMin / 60,
				"mins":  entry.StartMin % 60,
			})
			continue
		}
		rows = append(rows, map[string]any{
			"day":         entry.Day,
			"start_hours": entry.StartMin / 60,
			"start_mins":  entry.StartMin % 60,
			"end_hours":   entry.EndMin / 60,
			"end_mins":    entry.EndMin % 60,
		})
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		return "[]"
	}
	return string(raw)
}

func scheduleRemoveOptionsGlobal(tr *i18n.Translator, profiles []map[string]any) []discordgo.SelectMenuOption {
	options := []discordgo.SelectMenuOption{}
	for _, profile := range profiles {
		profileNo := toInt(profile["profile_no"], 0)
		if profileNo == 0 {
			continue
		}
		name := localizedProfileName(tr, profile)
		for _, entry := range scheduleEntriesFromRaw(profile["active_hours"]) {
			label := fmt.Sprintf("%s — %s", scheduleEntryLabel(tr, entry), name)
			value := fmt.Sprintf("%d|%s", profileNo, scheduleEntryValue(entry))
			options = append(options, discordgo.SelectMenuOption{Label: label, Value: value})
			if len(options) >= 25 {
				return options
			}
		}
	}
	return options
}

func scheduleEditOptionsGlobal(tr *i18n.Translator, profiles []map[string]any) []discordgo.SelectMenuOption {
	options := []discordgo.SelectMenuOption{}
	for _, profile := range profiles {
		profileNo := toInt(profile["profile_no"], 0)
		if profileNo == 0 {
			continue
		}
		name := localizedProfileName(tr, profile)
		for _, entry := range scheduleEntriesFromRaw(profile["active_hours"]) {
			if entry.Legacy {
				continue
			}
			value := fmt.Sprintf("%d|%s", profileNo, scheduleEntryValue(entry))
			label := fmt.Sprintf("%s — %s", scheduleEntryLabel(tr, entry), name)
			options = append(options, discordgo.SelectMenuOption{Label: label, Value: value})
			if len(options) >= 25 {
				return options
			}
		}
	}
	return options
}

func addScheduleEntry(tr *i18n.Translator, allProfiles []map[string]any, selected map[string]any, day, startMin, endMin int) ([]scheduleEntry, string) {
	if selected == nil {
		return nil, translateOrDefault(tr, "Profile not found.")
	}
	selectedNo := toInt(selected["profile_no"], 0)
	if selectedNo == 0 {
		return nil, translateOrDefault(tr, "Profile not found.")
	}
	if conflicts := scheduleConflicts(tr, allProfiles, day, startMin, endMin, 0, scheduleEntry{}); len(conflicts) > 0 {
		if tr != nil {
			return nil, tr.TranslateFormat("That overlaps with existing schedules: {0}", strings.Join(conflicts, ", "))
		}
		return nil, fmt.Sprintf("That overlaps with existing schedules: %s", strings.Join(conflicts, ", "))
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	entries = append(entries, scheduleEntry{Day: day, StartMin: startMin, EndMin: endMin})
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		return entries[i].StartMin < entries[j].StartMin
	})
	return entries, ""
}

func addScheduleEntriesForDays(tr *i18n.Translator, allProfiles []map[string]any, selected map[string]any, days []int, startMin, endMin int) ([]scheduleEntry, string) {
	if selected == nil {
		return nil, translateOrDefault(tr, "Profile not found.")
	}
	if len(days) == 0 {
		return nil, translateOrDefault(tr, "Please select at least one day.")
	}
	conflicts := []string{}
	for _, day := range days {
		conflicts = append(conflicts, scheduleConflicts(tr, allProfiles, day, startMin, endMin, 0, scheduleEntry{})...)
	}
	if len(conflicts) > 0 {
		if tr != nil {
			return nil, tr.TranslateFormat("That overlaps with existing schedules: {0}", strings.Join(uniqueStrings(conflicts), ", "))
		}
		return nil, fmt.Sprintf("That overlaps with existing schedules: %s", strings.Join(uniqueStrings(conflicts), ", "))
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	for _, day := range days {
		entries = append(entries, scheduleEntry{Day: day, StartMin: startMin, EndMin: endMin})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		return entries[i].StartMin < entries[j].StartMin
	})
	return entries, ""
}

func addScheduleEntryWithIgnore(tr *i18n.Translator, allProfiles []map[string]any, selected map[string]any, day, startMin, endMin int, ignoreProfileNo int, ignoreEntry scheduleEntry) ([]scheduleEntry, string) {
	if selected == nil {
		return nil, translateOrDefault(tr, "Profile not found.")
	}
	selectedNo := toInt(selected["profile_no"], 0)
	if selectedNo == 0 {
		return nil, translateOrDefault(tr, "Profile not found.")
	}
	if conflicts := scheduleConflicts(tr, allProfiles, day, startMin, endMin, ignoreProfileNo, ignoreEntry); len(conflicts) > 0 {
		if tr != nil {
			return nil, tr.TranslateFormat("That overlaps with existing schedules: {0}", strings.Join(conflicts, ", "))
		}
		return nil, fmt.Sprintf("That overlaps with existing schedules: %s", strings.Join(conflicts, ", "))
	}
	entries := scheduleEntriesFromRaw(selected["active_hours"])
	entries = append(entries, scheduleEntry{Day: day, StartMin: startMin, EndMin: endMin})
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Day != entries[j].Day {
			return entries[i].Day < entries[j].Day
		}
		return entries[i].StartMin < entries[j].StartMin
	})
	return entries, ""
}

func scheduleConflicts(tr *i18n.Translator, allProfiles []map[string]any, day, startMin, endMin int, ignoreProfileNo int, ignoreEntry scheduleEntry) []string {
	conflicts := []string{}
	for _, row := range allProfiles {
		profileNo := toInt(row["profile_no"], 0)
		name := localizedProfileName(tr, row)
		for _, entry := range scheduleEntriesFromRaw(row["active_hours"]) {
			if entry.Legacy || entry.Day != day {
				continue
			}
			if profileNo == ignoreProfileNo && entry.Day == ignoreEntry.Day && entry.StartMin == ignoreEntry.StartMin && entry.EndMin == ignoreEntry.EndMin && entry.Legacy == ignoreEntry.Legacy {
				continue
			}
			if startMin < entry.EndMin && entry.StartMin < endMin {
				conflicts = append(conflicts, fmt.Sprintf("%s %s", name, scheduleEntryLabel(tr, entry)))
			}
		}
	}
	return conflicts
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func parseClockMinutes(value string) (int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return 0, false
	}
	hour := toInt(parts[0], -1)
	min := toInt(parts[1], -1)
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, false
	}
	return hour*60 + min, true
}

func parseDayValue(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "mon", "monday":
		return 1
	case "tue", "tuesday":
		return 2
	case "wed", "wednesday":
		return 3
	case "thu", "thursday":
		return 4
	case "fri", "friday":
		return 5
	case "sat", "saturday":
		return 6
	case "sun", "sunday":
		return 7
	}
	return toInt(value, 0)
}

func parseDayValues(values []string) []int {
	if len(values) == 0 {
		return nil
	}
	out := []int{}
	seen := map[int]bool{}
	for _, value := range values {
		parts := strings.Split(strings.TrimSpace(value), ",")
		for _, part := range parts {
			day := parseDayValue(part)
			if day >= 1 && day <= 7 && !seen[day] {
				seen[day] = true
				out = append(out, day)
			}
		}
	}
	sort.Ints(out)
	return out
}

func parseDayList(value string) []int {
	parts := strings.FieldsFunc(strings.TrimSpace(value), func(r rune) bool {
		return r == ',' || r == '.'
	})
	return parseDayValues(parts)
}

func joinDayList(days []int) string {
	if len(days) == 0 {
		return ""
	}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		parts = append(parts, fmt.Sprintf("%d", day))
	}
	return strings.Join(parts, ".")
}

func labelDayList(tr *i18n.Translator, days []int) string {
	if len(days) == 0 {
		return ""
	}
	labels := []string{}
	for _, day := range days {
		if day < 1 || day > 7 {
			continue
		}
		labels = append(labels, localizedDayLabel(tr, day))
	}
	return strings.Join(labels, ", ")
}

func parseAssignPayloadDays(value string) ([]int, int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), "-")
	if len(parts) != 3 {
		return nil, 0, 0, false
	}
	start := toInt(parts[1], 0)
	end := toInt(parts[2], 0)
	days := parseDayList(parts[0])
	if len(days) == 0 || start < 0 || end <= start {
		return nil, 0, 0, false
	}
	return days, start, end, true
}

func parseGlobalScheduleValue(value string) (int, string, bool) {
	parts := strings.Split(strings.TrimSpace(value), "|")
	if len(parts) != 5 {
		return 0, "", false
	}
	profileNo := toInt(parts[0], 0)
	if profileNo == 0 {
		return 0, "", false
	}
	entry := strings.Join(parts[1:], "|")
	return profileNo, entry, true
}

func parseScheduleValue(value string) (scheduleEntry, bool) {
	parts := strings.Split(strings.TrimSpace(value), "|")
	if len(parts) != 5 {
		return scheduleEntry{}, false
	}
	profileNo := toInt(parts[0], 0)
	day := toInt(parts[1], 0)
	start := toInt(parts[2], 0)
	end := toInt(parts[3], 0)
	legacy := strings.EqualFold(parts[4], "true")
	if profileNo == 0 || day < 1 || day > 7 || start < 0 || end < 0 {
		return scheduleEntry{}, false
	}
	return scheduleEntry{ProfileNo: profileNo, Day: day, StartMin: start, EndMin: end, Legacy: legacy}, true
}

func parseEditAssignPayload(value string) (scheduleEntry, int, int, int, bool) {
	parts := strings.Split(strings.TrimSpace(value), ":")
	if len(parts) != 2 {
		return scheduleEntry{}, 0, 0, 0, false
	}
	entry, ok := parseScheduleValue(parts[0])
	if !ok {
		return scheduleEntry{}, 0, 0, 0, false
	}
	newParts := strings.Split(parts[1], "-")
	if len(newParts) != 3 {
		return scheduleEntry{}, 0, 0, 0, false
	}
	day := toInt(newParts[0], 0)
	start := toInt(newParts[1], 0)
	end := toInt(newParts[2], 0)
	if day < 1 || day > 7 || end <= start {
		return scheduleEntry{}, 0, 0, 0, false
	}
	return entry, day, start, end, true
}
