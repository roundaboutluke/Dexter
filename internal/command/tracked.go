package command

import (
	"encoding/json"
	"fmt"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
)

// TrackedCommand lists tracking entries.
type TrackedCommand struct{}

func (c *TrackedCommand) Name() string { return "tracked" }

func (c *TrackedCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "tracked") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	profileNo := result.ProfileNo
	targetID := result.TargetID

	if len(args) > 0 && strings.EqualFold(args[0], "help") {
		help := &HelpCommand{}
		return help.Handle(ctx, []string{"tracked"})
	}

	human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": targetID})
	if err != nil {
		return "", err
	}
	if human == nil {
		return unregisteredMessage(ctx, tr), nil
	}
	if containsWord(args, "area") {
		areas := parseAreaListFromHuman(human)
		return trackedAreaText(tr, ctx.Fences.Fences, areas), nil
	}

	profile, _ := ctx.Query.SelectOneQuery("profiles", map[string]any{"id": targetID, "profile_no": profileNo})
	blocked := []string{}
	if raw, ok := human["blocked_alerts"].(string); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &blocked)
	}

	adminExplanation := ""
	if ctx.IsAdmin {
		adminExplanation = fmt.Sprintf("Tracking details for **%s**\n", result.Target.Name)
	}

	lat := toFloat(human["latitude"])
	lon := toFloat(human["longitude"])
	locationText := ""
	if lat != 0 && lon != 0 {
		mapLink := fmt.Sprintf("https://maps.google.com/maps?q=%f,%f", lat, lon)
		locationText = "\n" + tr.Translate("Your location is currently set to", false) + " " + mapLink
	} else {
		locationText = "\n" + tr.Translate("You have not set a location yet", false)
	}
	restartExplanation := ""
	if toInt(human["enabled"], 0) == 0 {
		restartExplanation = "\n" + tr.TranslateFormat("You can start receiving alerts again using `{0}{1}`", ctx.Prefix, tr.Translate("start", true))
	}
	status := fmt.Sprintf("%s%s **%s**%s%s",
		adminExplanation,
		tr.Translate("Your alerts are currently", false),
		map[bool]string{true: tr.Translate("enabled", false), false: tr.Translate("disabled", false)}[toInt(human["enabled"], 0) != 0],
		restartExplanation,
		locationText,
	)

	areas := parseAreaListFromHuman(human)
	areaText := trackedAreaText(tr, ctx.Fences.Fences, areas)
	profileText := ""
	if profile != nil {
		if name := strings.TrimSpace(fmt.Sprintf("%v", profile["name"])); name != "" {
			profileText = fmt.Sprintf("%s %s", tr.Translate("Your profile is currently set to:", false), name)
		}
	}

	sections := []string{status}
	if areaText != "" {
		sections = append(sections, areaText)
	}
	if profileText != "" {
		sections = append(sections, profileText)
	}

	message := strings.Join(sections, "\n\n")
	message = message + "\n\n" + trackedCategorySummary(ctx, tr, targetID, profileNo, blocked)
	if len(message) < 4000 {
		return message, nil
	}
	if link, err := createHastebinLink(message); err == nil && link != "" {
		return fmt.Sprintf("%s %s", tr.Translate("Tracking list is quite long. Have a look at", false), link), nil
	}
	reply := buildFileReply(fmt.Sprintf("%s.txt", result.Target.Name), tr.Translate("Tracking list is long, but Hastebin is also down. Tracking list made into a file:", false), message)
	if reply != "" {
		return reply, nil
	}
	return message, nil
}

func trackedAreaText(tr *i18n.Translator, fences []geofence.Fence, selected []string) string {
	if len(selected) == 0 {
		return tr.Translate("You have not selected any area yet", false)
	}
	names := []string{}
	for _, fence := range fences {
		name := strings.ToLower(fence.Name)
		if containsString(selected, name) {
			names = append(names, fence.Name)
		}
	}
	if len(names) == 0 {
		return tr.Translate("You have not selected any area yet", false)
	}
	return fmt.Sprintf("%s %s", tr.Translate("You are currently set to receive alarms in", false), strings.Join(names, ", "))
}

func trackedCategorySummary(ctx *Context, tr *i18n.Translator, targetID string, profileNo int, blocked []string) string {
	sections := []string{}

	if !isDisabled(ctx.Config, "general.disablePokemon") {
		if containsString(blocked, "monster") {
			sections = append(sections, tr.Translate("You do not have permission to track monsters", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("monsters", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any monsters", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following monsters:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.MonsterRowText(ctx.Config, tr, ctx.Data, row))
				}
				if containsString(blocked, "pvp") {
					lines = append(lines, tr.Translate("Your permission level means you will not get results from PVP tracking", false))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableRaid") {
		if containsString(blocked, "raid") {
			sections = append(sections, tr.Translate("You do not have permission to track raids", false))
		} else if containsString(blocked, "egg") {
			sections = append(sections, tr.Translate("You do not have permission to track eggs", false))
		} else {
			raids, _ := ctx.Query.SelectAllQuery("raid", map[string]any{"id": targetID, "profile_no": profileNo})
			eggs, _ := ctx.Query.SelectAllQuery("egg", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(raids) == 0 && len(eggs) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any raids", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following raids:", false)}
				for _, row := range raids {
					lines = append(lines, tracking.RaidRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
				}
				for _, row := range eggs {
					lines = append(lines, tracking.EggRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableMaxBattle") {
		if containsString(blocked, "maxbattle") {
			sections = append(sections, tr.Translate("You do not have permission to track max battles", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("maxbattle", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any max battles", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following max battles:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.MaxbattleRowText(ctx.Config, tr, ctx.Data, row))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableQuest") {
		if containsString(blocked, "quest") {
			sections = append(sections, tr.Translate("You do not have permission to track quests", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("quest", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any quests", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following quests:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.QuestRowText(ctx.Config, tr, ctx.Data, row))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableInvasion") {
		if containsString(blocked, "invasion") {
			sections = append(sections, tr.Translate("You do not have permission to track invasions", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("invasion", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any invasions", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following invasions:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.InvasionRowText(ctx.Config, tr, ctx.Data, row))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableLure") {
		if containsString(blocked, "lure") {
			sections = append(sections, tr.Translate("You do not have permission to track lures", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("lures", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any lures", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following lures:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.LureRowText(ctx.Config, tr, ctx.Data, row))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableNest") {
		if containsString(blocked, "nest") {
			sections = append(sections, tr.Translate("You do not have permission to track nests", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("nests", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any nests", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following nests:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.NestRowText(ctx.Config, tr, ctx.Data, row))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableGym") {
		if containsString(blocked, "gym") {
			sections = append(sections, tr.Translate("You do not have permission to track gyms", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("gym", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any gyms", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following gyms:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.GymRowText(ctx.Config, tr, ctx.Data, row, ctx.Scanner))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if !isDisabled(ctx.Config, "general.disableFortUpdate") {
		if containsString(blocked, "forts") {
			sections = append(sections, tr.Translate("You do not have permission to track fort changes", false))
		} else {
			rows, _ := ctx.Query.SelectAllQuery("forts", map[string]any{"id": targetID, "profile_no": profileNo})
			if len(rows) == 0 {
				sections = append(sections, tr.Translate("You're not tracking any fort changes", false))
			} else {
				lines := []string{tr.Translate("You're tracking the following fort changes:", false)}
				for _, row := range rows {
					lines = append(lines, tracking.FortUpdateRowText(ctx.Config, tr, ctx.Data, row))
				}
				sections = append(sections, strings.Join(lines, "\n"))
			}
		}
	}

	if len(sections) == 0 {
		return tr.Translate("No tracking entries found.", false)
	}
	return strings.Join(sections, "\n\n")
}

func isDisabled(cfg *config.Config, key string) bool {
	if cfg == nil {
		return false
	}
	disabled, _ := cfg.GetBool(key)
	return disabled
}
