package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"poraclego/internal/config"
)

func isAdmin(cfg *config.Config, id string) bool {
	discordAdmins, _ := cfg.GetStringSlice("discord.admins")
	telegramAdmins, _ := cfg.GetStringSlice("telegram.admins")
	return containsString(discordAdmins, id) || containsString(telegramAdmins, id)
}

func parseCommunityMembership(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return []string{}
		}
		var items []string
		if err := json.Unmarshal([]byte(v), &items); err == nil {
			for i, item := range items {
				items[i] = strings.ToLower(item)
			}
			return items
		}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.ToLower(s))
			}
		}
		return out
	}
	return []string{}
}

func filterAreas(cfg *config.Config, membership []string, areas []string) []string {
	raw, ok := cfg.Get("areaSecurity.communities")
	if !ok {
		return areas
	}
	communities, ok := raw.(map[string]any)
	if !ok {
		return areas
	}
	allowed := make([]string, 0)
	for _, community := range membership {
		matchKey := ""
		for key := range communities {
			if strings.EqualFold(key, community) {
				matchKey = key
				break
			}
		}
		if matchKey == "" {
			continue
		}
		entry, ok := communities[matchKey].(map[string]any)
		if !ok {
			continue
		}
		rawAllowed, ok := entry["allowedAreas"]
		if !ok {
			continue
		}
		for _, area := range toStringSlice(rawAllowed) {
			if !containsString(allowed, area) {
				allowed = append(allowed, area)
			}
		}
	}
	if len(allowed) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(areas))
	for _, area := range areas {
		if containsString(allowed, area) {
			result = append(result, area)
		}
	}
	return result
}

func mustJSONMarshal(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte{}
	}
	return data
}

func toStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, strings.ToLower(item))
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.ToLower(s))
			}
		}
		return out
	default:
		return []string{}
	}
}

func decodeJSONRows(r *http.Request) ([]map[string]any, error) {
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out, nil
	case map[string]any:
		return []map[string]any{v}, nil
	default:
		return nil, fmt.Errorf("invalid payload")
	}
}

func decodeStringArray(r *http.Request) ([]string, error) {
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.ToLower(s))
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid payload")
	}
}

func decodeAnyArray(r *http.Request) ([]any, error) {
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case []any:
		return v, nil
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid payload")
	}
}

func numberFromAnyOrDefault(value any, fallback int) int {
	if n, ok := numberFromAny(value); ok {
		return n
	}
	return fallback
}

func getAdminIDs(cfg *config.Config, path string) []string {
	list, _ := cfg.GetStringSlice(path)
	return list
}

func numberFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func defaultInt(value any, fallback int) int {
	if value == nil {
		return fallback
	}
	if v, ok := numberFromAny(value); ok {
		return v
	}
	return fallback
}

func intFromAny(value any) int {
	if v, ok := numberFromAny(value); ok {
		return v
	}
	return 0
}

func floatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func getStringValue(value any, cfg *config.Config, path string) string {
	if value == nil {
		if cfg == nil {
			return ""
		}
		fallback, _ := cfg.GetString(path)
		return fallback
	}
	return fmt.Sprintf("%v", value)
}

func getString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}
