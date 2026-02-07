package profile

import (
	"encoding/json"
	"fmt"
	"sort"

	"poraclego/internal/db"
)

// Logic manages profile operations for a user.
type Logic struct {
	query      *db.Query
	id         string
	human      map[string]any
	profiles   []map[string]any
	categories []string
}

// New returns a profile logic instance.
func New(query *db.Query, id string) *Logic {
	return &Logic{
		query:      query,
		id:         id,
		categories: []string{"monsters", "raid", "egg", "quest", "invasion", "weather", "lures", "gym", "nests", "forts"},
	}
}

// Init loads the human and profiles.
func (l *Logic) Init() error {
	human, err := l.query.SelectOneQuery("humans", map[string]any{"id": l.id})
	if err != nil {
		return err
	}
	profiles, err := l.query.SelectAllQuery("profiles", map[string]any{"id": l.id})
	if err != nil {
		return err
	}
	l.human = human
	l.profiles = profiles
	return nil
}

// Profiles returns loaded profile rows.
func (l *Logic) Profiles() []map[string]any {
	return l.profiles
}

// Human returns loaded human row.
func (l *Logic) Human() map[string]any {
	return l.human
}

// UpdateHours updates active hours for a profile.
func (l *Logic) UpdateHours(profileNo int, hours any) error {
	value := normalizeJSON(hours)
	_, err := l.query.UpdateQuery("profiles", map[string]any{"active_hours": value}, map[string]any{"id": l.id, "profile_no": profileNo})
	return err
}

// AddProfile adds a new profile for a user.
func (l *Logic) AddProfile(name string, hours any) error {
	if l.human == nil {
		if err := l.Init(); err != nil {
			return err
		}
	}

	used := map[int]bool{}
	for _, profile := range l.profiles {
		used[toInt(profile["profile_no"])] = true
	}

	newProfileNo := 1
	for used[newProfileNo] {
		newProfileNo++
	}

	activeHours := normalizeJSON(hours)

	row := map[string]any{
		"id":           l.id,
		"profile_no":   newProfileNo,
		"name":         name,
		"area":         l.human["area"],
		"latitude":     l.human["latitude"],
		"longitude":    l.human["longitude"],
		"active_hours": activeHours,
	}
	_, err := l.query.InsertQuery("profiles", row)
	return err
}

// CopyProfile copies trackings between profiles.
func (l *Logic) CopyProfile(sourceProfileNo int, destProfileNo int) error {
	for _, category := range l.categories {
		backup, err := l.query.SelectAllQuery(category, map[string]any{"id": l.id, "profile_no": sourceProfileNo})
		if err != nil {
			return err
		}
		_, err = l.query.DeleteQuery(category, map[string]any{"id": l.id, "profile_no": destProfileNo})
		if err != nil {
			return err
		}

		if len(backup) == 0 {
			continue
		}
		rows := make([]map[string]any, 0, len(backup))
		for _, item := range backup {
			cloned := map[string]any{}
			for key, value := range item {
				if key == "uid" {
					continue
				}
				cloned[key] = value
			}
			cloned["profile_no"] = destProfileNo
			rows = append(rows, cloned)
		}
		if _, err := l.query.InsertQuery(category, rows); err != nil {
			return err
		}
	}
	return nil
}

// DeleteProfile removes a profile and its data.
func (l *Logic) DeleteProfile(profileNo int) error {
	if l.human == nil {
		if err := l.Init(); err != nil {
			return err
		}
	}
	_, err := l.query.DeleteQuery("profiles", map[string]any{"id": l.id, "profile_no": profileNo})
	if err != nil {
		return err
	}

	if len(l.profiles) != 1 || profileNo != 1 {
		for _, category := range l.categories {
			if _, err := l.query.DeleteQuery(category, map[string]any{"id": l.id, "profile_no": profileNo}); err != nil {
				return err
			}
		}
	}

	currentProfile := toInt(l.human["current_profile_no"])
	preferredProfile := toInt(l.human["preferred_profile_no"])
	if currentProfile != profileNo && preferredProfile != profileNo {
		return nil
	}

	lowest := findLowestProfile(l.profiles, profileNo)
	update := map[string]any{}
	if lowest == nil {
		if currentProfile == profileNo {
			update["current_profile_no"] = 1
		}
		if preferredProfile == profileNo {
			update["preferred_profile_no"] = 1
		}
		if len(update) == 0 {
			return nil
		}
		_, err = l.query.UpdateQuery("humans", update, map[string]any{"id": l.id})
		return err
	}

	lowestNo := toInt(lowest["profile_no"])
	if currentProfile == profileNo {
		update["current_profile_no"] = lowestNo
		update["area"] = lowest["area"]
		update["latitude"] = lowest["latitude"]
		update["longitude"] = lowest["longitude"]
	}
	if preferredProfile == profileNo {
		update["preferred_profile_no"] = lowestNo
	}
	if len(update) == 0 {
		return nil
	}
	_, err = l.query.UpdateQuery("humans", update, map[string]any{"id": l.id})
	return err
}

func normalizeJSON(value any) string {
	switch v := value.(type) {
	case nil:
		return "{}"
	case string:
		if v == "" {
			return "{}"
		}
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "{}"
		}
		return string(data)
	}
}

func toInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		var out int
		_, _ = fmt.Sscanf(v, "%d", &out)
		return out
	default:
		return 0
	}
}

func findLowestProfile(profiles []map[string]any, exclude int) map[string]any {
	filtered := make([]map[string]any, 0)
	for _, profile := range profiles {
		if toInt(profile["profile_no"]) == exclude {
			continue
		}
		filtered = append(filtered, profile)
	}
	if len(filtered) == 0 {
		return nil
	}
	sort.Slice(filtered, func(i, j int) bool {
		return toInt(filtered[i]["profile_no"]) < toInt(filtered[j]["profile_no"])
	})
	return filtered[0]
}
