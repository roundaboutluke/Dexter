package command

import (
	"fmt"
	"strings"
)

// EnableCommand clears admin_disable for users (admin only).
type EnableCommand struct{}

func (c *EnableCommand) Name() string { return "enable" }

func (c *EnableCommand) Handle(ctx *Context, args []string) (string, error) {
	if !ctx.IsAdmin {
		return "🙅", nil
	}
	targets := parseTargetIDs(ctx, args, true)
	for _, id := range targets {
		if _, err := ctx.Query.UpdateQuery("humans", map[string]any{"admin_disable": 0, "disabled_date": nil}, map[string]any{"id": id}); err != nil {
			return "", err
		}
	}
	return "✅", nil
}

// DisableCommand sets admin_disable for users (admin only).
type DisableCommand struct{}

func (c *DisableCommand) Name() string { return "disable" }

func (c *DisableCommand) Handle(ctx *Context, args []string) (string, error) {
	if !ctx.IsAdmin {
		return "🙅", nil
	}
	targets := parseTargetIDs(ctx, args, false)
	for _, id := range targets {
		if _, err := ctx.Query.UpdateQuery("humans", map[string]any{"admin_disable": 1, "disabled_date": nil}, map[string]any{"id": id}); err != nil {
			return "", err
		}
	}
	return "✅", nil
}

func parseTargetIDs(ctx *Context, args []string, allowWebhookName bool) []string {
	ids := []string{}
	re := NewRegexSet(ctx.I18n)
	for _, arg := range args {
		if re.User.MatchString(arg) {
			match := re.User.FindStringSubmatch(arg)
			if len(match) > 2 {
				ids = append(ids, match[2])
				continue
			}
		}
		if id := extractMentionID(arg); id != "" {
			ids = append(ids, id)
			continue
		}
		if num := toInt(arg, 0); num > 0 {
			ids = append(ids, fmt.Sprintf("%d", num))
			continue
		}
		if allowWebhookName {
			human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"name": arg, "type": "webhook"})
			if err == nil && human != nil {
				ids = append(ids, fmt.Sprintf("%v", human["id"]))
			}
		}
	}
	return uniqueStrings(ids)
}

func extractMentionID(value string) string {
	if !strings.Contains(value, "<") || !strings.Contains(value, ">") {
		return ""
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			continue
		}
		j := i
		for j < len(value) && value[j] >= '0' && value[j] <= '9' {
			j++
		}
		return value[i:j]
	}
	return ""
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
