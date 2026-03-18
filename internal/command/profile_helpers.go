package command

import (
	"fmt"
	"strconv"
	"strings"

	"dexter/internal/i18n"
	"dexter/internal/profile"
)

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

func profileList(tr *i18n.Translator, logic *profile.Logic, currentProfile int) string {
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
