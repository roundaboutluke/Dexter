package command

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"poraclego/internal/community"
)

// CommunityCommand manages community membership (admin only).
type CommunityCommand struct{}

func (c *CommunityCommand) Name() string { return "community" }

func (c *CommunityCommand) Handle(ctx *Context, args []string) (string, error) {
	if !ctx.IsAdmin {
		return "🙅", nil
	}
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		return tr.TranslateFormat("Valid commands are `{0}community add <name> <targets>`, `{0}community remove <name> <targets>`, `{0}community clear <targets>`, `{0}community show <targets>`, `{0}community list`", ctx.Prefix), nil
	}

	command := strings.ToLower(args[0])
	args = args[1:]

	raw, ok := ctx.Config.Get("areaSecurity.communities")
	if !ok {
		return tr.Translate("No communities configured.", false), nil
	}
	communities, ok := raw.(map[string]any)
	if !ok {
		return tr.Translate("No communities configured.", false), nil
	}

	if command == "list" {
		keys := []string{}
		for key := range communities {
			keys = append(keys, strings.ReplaceAll(key, " ", "_"))
		}
		sort.Strings(keys)
		return fmt.Sprintf("%s\n```\n%s\n```", tr.Translate("These are the valid communities:", false), strings.Join(keys, "\n")), nil
	}

	targetIDs := parseTargetIDs(ctx, args, false)
	note := ""
	if len(targetIDs) == 0 {
		if !strings.Contains(result.Target.Type, "user") {
			targetIDs = []string{result.TargetID}
			note = fmt.Sprintf("No targets listed, assuming target of %s %s", result.TargetID, result.Target.Name)
		} else {
			return tr.Translate("No targets listed", false), nil
		}
	}

	switch command {
	case "clear":
		lines := []string{}
		if note != "" {
			lines = append(lines, note)
		}
		for _, id := range targetIDs {
			lines = append(lines, fmt.Sprintf("Clear target %s", id))
			if _, err := ctx.Query.UpdateQuery("humans", map[string]any{
				"community_membership": "[]",
				"area_restriction":     nil,
			}, map[string]any{"id": id}); err != nil {
				return "", err
			}
			ctx.MarkAlertStateDirty()
		}
		lines = append(lines, "✅")
		return strings.Join(lines, "\n"), nil
	case "add", "remove":
		if len(args) == 0 {
			return tr.TranslateFormat("Valid commands are `{0}community add <name> <targets>`, `{0}community remove <name> <targets>`, `{0}community clear <targets>`, `{0}community show <targets>`, `{0}community list`", ctx.Prefix), nil
		}
		communityName := strings.ToLower(args[0])
		resolved := resolveCommunityName(communities, communityName)
		if resolved != "" {
			communityName = resolved
		}
		lines := []string{}
		if note != "" {
			lines = append(lines, note)
		}
		for _, id := range targetIDs {
			human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": id})
			if err != nil || human == nil {
				continue
			}
			existing := []string{}
			if raw, ok := human["community_membership"].(string); ok && raw != "" {
				_ = json.Unmarshal([]byte(raw), &existing)
			}
			var updated []string
			if command == "add" {
				lines = append(lines, fmt.Sprintf("Add community %s to target %s %v", communityName, id, human["name"]))
				updated = community.AddCommunity(ctx.Config, existing, communityName)
			} else {
				lines = append(lines, fmt.Sprintf("Remove community %s from target %s %v", communityName, id, human["name"]))
				updated = community.RemoveCommunity(ctx.Config, existing, communityName)
			}
			restrictions := community.CalculateLocationRestrictions(ctx.Config, updated)
			if _, err := ctx.Query.UpdateQuery("humans", map[string]any{
				"community_membership": toJSON(updated),
				"area_restriction":     nullableString(toJSON(restrictions)),
			}, map[string]any{"id": id}); err != nil {
				return "", err
			}
			ctx.MarkAlertStateDirty()
		}
		lines = append(lines, "✅")
		return strings.Join(lines, "\n"), nil
	case "show":
		lines := []string{}
		if note != "" {
			lines = append(lines, note)
		}
		for _, id := range targetIDs {
			human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": id})
			if err != nil || human == nil {
				continue
			}
			restrictions := "none"
			if raw := fmt.Sprintf("%v", human["area_restriction"]); raw != "" && raw != "<nil>" {
				restrictions = raw
			}
			lines = append(lines, fmt.Sprintf("User target %s %v has communities %v location restrictions %s", id, human["name"], human["community_membership"], restrictions))
		}
		if len(lines) == 0 {
			return tr.Translate("No users found.", false), nil
		}
		lines = append(lines, "✅")
		return strings.Join(lines, "\n"), nil
	default:
		return tr.TranslateFormat("Valid commands are `{0}community add <name> <targets>`, `{0}community remove <name> <targets>`, `{0}community clear <targets>`, `{0}community show <targets>`, `{0}community list`", ctx.Prefix), nil
	}
}

func resolveCommunityName(entries map[string]any, name string) string {
	for key := range entries {
		if strings.EqualFold(key, name) {
			return key
		}
	}
	return ""
}
