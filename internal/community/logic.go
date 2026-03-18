package community

import (
	"fmt"
	"sort"
	"strings"

	"dexter/internal/config"
)

// AddCommunity validates and adds a community.
func AddCommunity(cfg *config.Config, existing []string, communityToAdd string) []string {
	if cfg == nil {
		return existing
	}
	raw, ok := cfg.Get("areaSecurity.communities")
	if !ok {
		return existing
	}
	communities, ok := raw.(map[string]any)
	if !ok {
		return existing
	}

	valid := map[string]bool{}
	for key := range communities {
		valid[strings.ToLower(key)] = true
	}

	newCommunities := make([]string, 0, len(existing)+1)
	seen := map[string]bool{}
	for _, value := range existing {
		value = strings.ToLower(value)
		if value != "" && valid[value] && !seen[value] {
			seen[value] = true
			newCommunities = append(newCommunities, value)
		}
	}
	add := strings.ToLower(communityToAdd)
	if add != "" && valid[add] && !seen[add] {
		newCommunities = append(newCommunities, add)
	}
	sort.Strings(newCommunities)
	return newCommunities
}

// RemoveCommunity removes a community from membership.
func RemoveCommunity(cfg *config.Config, existing []string, communityToRemove string) []string {
	if cfg == nil {
		return existing
	}
	raw, ok := cfg.Get("areaSecurity.communities")
	if !ok {
		return existing
	}
	communities, ok := raw.(map[string]any)
	if !ok {
		return existing
	}

	valid := map[string]bool{}
	for key := range communities {
		valid[strings.ToLower(key)] = true
	}

	remove := strings.ToLower(communityToRemove)
	newCommunities := []string{}
	for _, value := range existing {
		value = strings.ToLower(value)
		if value == remove {
			continue
		}
		if value != "" && valid[value] {
			newCommunities = append(newCommunities, value)
		}
	}
	sort.Strings(newCommunities)
	return newCommunities
}

// CalculateLocationRestrictions returns the union of location fences for communities.
func CalculateLocationRestrictions(cfg *config.Config, communities []string) []string {
	result := map[string]bool{}
	raw, ok := cfg.Get("areaSecurity.communities")
	if !ok {
		return []string{}
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return []string{}
	}
	for _, community := range communities {
		match := ""
		for key := range entries {
			if strings.EqualFold(key, community) {
				match = key
				break
			}
		}
		if match == "" {
			continue
		}
		entry, ok := entries[match].(map[string]any)
		if !ok {
			continue
		}
		value, ok := entry["locationFence"]
		if !ok {
			continue
		}
		switch v := value.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					result[strings.ToLower(s)] = true
				}
			}
		case string:
			if v != "" {
				result[strings.ToLower(v)] = true
			}
		}
	}
	out := make([]string, 0, len(result))
	for key := range result {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// IsTelegramCommunityAdmin returns the community list for a telegram admin.
func IsTelegramCommunityAdmin(cfg *config.Config, id string) []string {
	if cfg == nil {
		return nil
	}
	raw, ok := cfg.Get("areaSecurity.communities")
	if !ok {
		return nil
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	result := []string{}
	for key, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		telegram, ok := entryMap["telegram"].(map[string]any)
		if !ok {
			continue
		}
		admins, ok := telegram["admins"].([]any)
		if !ok {
			continue
		}
		for _, admin := range admins {
			if fmt.Sprintf("%v", admin) == id {
				result = append(result, strings.ToLower(key))
				break
			}
		}
	}
	if len(result) == 0 {
		return nil
	}
	sort.Strings(result)
	return result
}
