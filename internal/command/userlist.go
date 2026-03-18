package command

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"dexter/internal/community"
	"dexter/internal/config"
)

// UserListCommand lists registered humans (admin only).
type UserListCommand struct{}

func (c *UserListCommand) Name() string { return "userlist" }

func (c *UserListCommand) Handle(ctx *Context, args []string) (string, error) {
	communityFilter := []string{}
	if ctx.IsAdmin {
		communityFilter = []string{}
	} else if ctx.Platform == "telegram" {
		if list := community.IsTelegramCommunityAdmin(ctx.Config, ctx.UserID); len(list) > 0 {
			communityFilter = append(communityFilter, list...)
		}
	}
	if !ctx.IsAdmin && len(communityFilter) == 0 {
		return "🙅", nil
	}
	tr := ctx.I18n.Translator(ctx.Language)
	rows, err := ctx.Query.SelectAllQuery("humans", map[string]any{})
	if err != nil {
		return "", err
	}
	filtered := filterHumans(rows, args)
	if enabled, _ := ctx.Config.GetBool("areaSecurity.enabled"); enabled {
		for _, arg := range args {
			name := communityNameFromConfig(ctx.Config, arg)
			if name != "" {
				communityFilter = append(communityFilter, strings.ToLower(name))
			}
		}
	}
	if len(communityFilter) > 0 {
		filtered = filterByCommunities(filtered, communityFilter)
	}
	sort.Slice(filtered, func(i, j int) bool {
		typeA := fmt.Sprintf("%v", filtered[i]["type"])
		typeB := fmt.Sprintf("%v", filtered[j]["type"])
		if typeA == typeB {
			nameA := fmt.Sprintf("%v", filtered[i]["name"])
			nameB := fmt.Sprintf("%v", filtered[j]["name"])
			return nameA < nameB
		}
		return typeA < typeB
	})
	lines := []string{tr.Translate("These users are registered with Poracle:", false)}
	for _, row := range filtered {
		humanType := fmt.Sprintf("%v", row["type"])
		name := fmt.Sprintf("%v", row["name"])
		id := fmt.Sprintf("%v", row["id"])
		communityMembership := fmt.Sprintf("%v", row["community_membership"])
		disabled := ""
		if toInt(row["admin_disable"], 0) == 1 {
			disabled = " 🚫"
		}
		if humanType == "webhook" {
			lines = append(lines, fmt.Sprintf("%s • %s%s", humanType, name, disabled))
		} else {
			lines = append(lines, fmt.Sprintf("%s • %s | (%s) %s %s", humanType, name, id, communityMembership, disabled))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func filterHumans(rows []map[string]any, args []string) []map[string]any {
	if len(args) == 0 {
		return rows
	}
	filtered := rows
	for _, arg := range args {
		switch strings.ToLower(arg) {
		case "disabled":
			filtered = filterByBool(filtered, "admin_disable", 1)
		case "enabled":
			filtered = filterByBool(filtered, "admin_disable", 0)
		case "discord":
			filtered = filterByPrefix(filtered, "type", "discord")
		case "telegram":
			filtered = filterByPrefix(filtered, "type", "telegram")
		case "webhook":
			filtered = filterByPrefix(filtered, "type", "webhook")
		case "user":
			filtered = filterByContains(filtered, "type", "user")
		case "group":
			filtered = filterByContains(filtered, "type", "group")
		case "channel":
			filtered = filterByContains(filtered, "type", "channel")
		}
	}
	return filtered
}

func filterByBool(rows []map[string]any, key string, target int) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		if toInt(row[key], 0) == target {
			out = append(out, row)
		}
	}
	return out
}

func filterByPrefix(rows []map[string]any, key string, prefix string) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		value := fmt.Sprintf("%v", row[key])
		if strings.HasPrefix(value, prefix) {
			out = append(out, row)
		}
	}
	return out
}

func filterByContains(rows []map[string]any, key string, needle string) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		value := fmt.Sprintf("%v", row[key])
		if strings.Contains(value, needle) {
			out = append(out, row)
		}
	}
	return out
}

func communityNameFromConfig(cfg *config.Config, name string) string {
	if cfg == nil {
		return ""
	}
	raw, ok := cfg.Get("areaSecurity.communities")
	if !ok {
		return ""
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	for key := range entries {
		if strings.EqualFold(key, name) {
			return key
		}
	}
	return ""
}

func filterByCommunities(rows []map[string]any, communities []string) []map[string]any {
	out := []map[string]any{}
	for _, row := range rows {
		raw := fmt.Sprintf("%v", row["community_membership"])
		if raw == "" || raw == "<nil>" {
			continue
		}
		var list []string
		if err := json.Unmarshal([]byte(raw), &list); err != nil {
			continue
		}
		if len(list) == 0 {
			continue
		}
		for _, entry := range list {
			if containsString(communities, strings.ToLower(entry)) {
				out = append(out, row)
				break
			}
		}
	}
	return out
}
