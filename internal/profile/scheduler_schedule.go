package profile

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type activeHour struct {
	Day   int
	Hours int
	Mins  int
}

type scheduleRange struct {
	Day      int
	StartMin int
	EndMin   int
}

func parseActiveHours(raw any) []activeHour {
	text := rawJSONText(raw)
	if len(text) <= 2 {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	out := make([]activeHour, 0, len(rows))
	for _, row := range rows {
		day := toInt(row["day"])
		if day < 1 || day > 7 {
			continue
		}
		out = append(out, activeHour{
			Day:   day,
			Hours: toInt(row["hours"]),
			Mins:  toInt(row["mins"]),
		})
	}
	return out
}

func parseScheduleRanges(raw any) []scheduleRange {
	text := rawJSONText(raw)
	if len(text) <= 2 {
		return nil
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(text), &rows); err != nil {
		return nil
	}
	out := []scheduleRange{}
	for _, row := range rows {
		day := toInt(row["day"])
		if day < 1 || day > 7 {
			continue
		}
		startHours, ok := row["start_hours"]
		if !ok {
			continue
		}
		startMins := toInt(row["start_mins"])
		endHours := toInt(row["end_hours"])
		endMins := toInt(row["end_mins"])
		start := toInt(startHours)*60 + startMins
		end := endHours*60 + endMins
		if end <= start {
			continue
		}
		out = append(out, scheduleRange{Day: day, StartMin: start, EndMin: end})
	}
	return out
}

func rawJSONText(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case json.RawMessage:
		return strings.TrimSpace(string(v))
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", raw))
	}
}

func hasRangeSchedules(rows []map[string]any) bool {
	for _, row := range rows {
		if len(parseScheduleRanges(row["active_hours"])) > 0 {
			return true
		}
	}
	return false
}

func inScheduleRange(now time.Time, ranges []scheduleRange) bool {
	if len(ranges) == 0 {
		return false
	}
	nowDow := isoWeekday(now.Weekday())
	nowMin := now.Hour()*60 + now.Minute()
	for _, entry := range ranges {
		if entry.Day != nowDow {
			continue
		}
		if nowMin >= entry.StartMin && nowMin < entry.EndMin {
			return true
		}
	}
	return false
}

func (s *Scheduler) nextScheduleTime(human map[string]any, profiles []map[string]any) (time.Time, bool) {
	var next time.Time
	found := false
	for _, profile := range profiles {
		ranges := parseScheduleRanges(profile["active_hours"])
		if len(ranges) == 0 {
			continue
		}
		now := s.nowForProfile(human, profile)
		if candidate, ok := nextScheduleStart(now, ranges); ok {
			if !found || candidate.UTC().Before(next.UTC()) {
				next = candidate
				found = true
			}
		}
	}
	return next, found
}

func nextScheduleStart(now time.Time, ranges []scheduleRange) (time.Time, bool) {
	if len(ranges) == 0 {
		return time.Time{}, false
	}
	nowDow := isoWeekday(now.Weekday())
	nowMin := now.Hour()*60 + now.Minute()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var next time.Time
	found := false
	for _, entry := range ranges {
		deltaDays := (entry.Day - nowDow + 7) % 7
		if deltaDays == 0 && entry.StartMin <= nowMin {
			deltaDays = 7
		}
		candidate := startOfDay.AddDate(0, 0, deltaDays).Add(time.Duration(entry.StartMin) * time.Minute)
		if !found || candidate.Before(next) {
			next = candidate
			found = true
		}
	}
	return next, found
}

func formatScheduleTime(t time.Time) string {
	return t.Format("Mon 15:04")
}

func isoWeekday(day time.Weekday) int {
	if day == time.Sunday {
		return 7
	}
	return int(day)
}

func getString(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}

func getFloat(value any) float64 {
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
		var out float64
		_, _ = fmt.Sscanf(strings.TrimSpace(v), "%f", &out)
		return out
	default:
		return 0
	}
}
