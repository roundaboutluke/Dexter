package command

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/tileserver"
)

// AreaCommand manages area selection.
type AreaCommand struct{}

func (c *AreaCommand) Name() string { return "area" }

func (c *AreaCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "area") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	if ctx.Fences == nil {
		return tr.Translate("Geofence data is not loaded.", false), nil
	}

	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	row, _, targetID, err := loadTargetRow(ctx, result.Target)
	if err != nil {
		return "", err
	}
	if row == nil {
		return "Target is not registered.", nil
	}

	fences := ctx.Fences.Fences
	areas := selectableAreas(fences, ctx.IsAdmin)
	if areaRestricted(ctx, row, result.TargetIsAdmin) {
		allowed := allowedAreasFromCommunities(ctx, row)
		if len(allowed) == 0 {
			areas = []areaEntry{}
		} else {
			filtered := []areaEntry{}
			for _, entry := range areas {
				if containsString(allowed, entry.LowerName) {
					filtered = append(filtered, entry)
				}
			}
			areas = filtered
		}
	}

	switch strings.ToLower(firstArg(args)) {
	case "add":
		return updateAreas(ctx, tr, targetID, result.ProfileNo, row, areas, args[1:], true)
	case "remove":
		return updateAreas(ctx, tr, targetID, result.ProfileNo, row, areas, args[1:], false)
	case "list":
		return listAreas(tr, areas), nil
	case "overview":
		return showAreaOverview(ctx, tr, row, areas, args[1:])
	case "show":
		return showAreas(ctx, tr, row, areas, args[1:], re)
	default:
		return areaDefault(ctx, tr, row, areas, result.Language, result.Target), nil
	}
}

type areaEntry struct {
	Name        string
	Group       string
	Description string
	LowerName   string
	Selectable  bool
}

func selectableAreas(fences []geofence.Fence, includeAll bool) []areaEntry {
	entries := []areaEntry{}
	for _, fence := range fences {
		selectable := true
		if fence.UserSelectable != nil {
			selectable = *fence.UserSelectable
		}
		if !includeAll && !selectable {
			continue
		}
		entries = append(entries, areaEntry{
			Name:        fence.Name,
			Group:       fence.Group,
			Description: fence.Description,
			LowerName:   strings.ToLower(fence.Name),
			Selectable:  selectable,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Group == entries[j].Group {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Group < entries[j].Group
	})
	return entries
}

func areaRestricted(ctx *Context, human map[string]any, targetIsAdmin bool) bool {
	enabled, _ := ctx.Config.GetBool("areaSecurity.enabled")
	if !enabled {
		return false
	}
	if targetIsAdmin {
		return false
	}
	return human["area_restriction"] != nil
}

func allowedAreasFromCommunities(ctx *Context, human map[string]any) []string {
	raw := human["community_membership"]
	if raw == nil {
		return []string{}
	}
	var communities []string
	switch v := raw.(type) {
	case string:
		_ = json.Unmarshal([]byte(v), &communities)
	}
	return filterAreasByCommunity(ctx, communities)
}

func filterAreasByCommunity(ctx *Context, communities []string) []string {
	raw, ok := ctx.Config.Get("areaSecurity.communities")
	if !ok {
		return []string{}
	}
	entries, ok := raw.(map[string]any)
	if !ok {
		return []string{}
	}
	allowed := map[string]bool{}
	for _, community := range communities {
		key := ""
		for name := range entries {
			if strings.EqualFold(name, community) {
				key = name
				break
			}
		}
		if key == "" {
			continue
		}
		entry, ok := entries[key].(map[string]any)
		if !ok {
			continue
		}
		if rawAreas, ok := entry["allowedAreas"].([]any); ok {
			for _, item := range rawAreas {
				if s, ok := item.(string); ok {
					allowed[strings.ToLower(s)] = true
				}
			}
		}
	}
	out := []string{}
	for key := range allowed {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func updateAreas(ctx *Context, tr *i18n.Translator, targetID string, profileNo int, human map[string]any, available []areaEntry, args []string, add bool) (string, error) {
	current := parseAreaListFromHuman(human)
	input := []string{}
	for _, arg := range args {
		input = append(input, strings.ToLower(strings.ReplaceAll(arg, "_", " ")))
	}
	entriesByName := map[string]areaEntry{}
	valid := map[string]bool{}
	for _, entry := range available {
		valid[entry.LowerName] = true
		entriesByName[entry.LowerName] = entry
	}
	updated := []string{}
	if add {
		updated = append(updated, current...)
		for _, value := range input {
			if valid[value] && !containsString(updated, value) {
				updated = append(updated, value)
			}
		}
	} else {
		for _, value := range current {
			if !containsString(input, value) {
				updated = append(updated, value)
			}
		}
	}
	filtered := []string{}
	for _, value := range updated {
		if valid[value] {
			filtered = append(filtered, value)
		}
	}
	payload, _ := json.Marshal(filtered)
	if _, err := ctx.Query.UpdateQuery("humans", map[string]any{"area": string(payload)}, map[string]any{"id": targetID}); err != nil {
		return "", err
	}
	if _, err := ctx.Query.UpdateQuery("profiles", map[string]any{"area": string(payload)}, map[string]any{"id": targetID, "profile_no": profileNo}); err != nil {
		return "", err
	}

	lines := []string{}
	if add {
		addAreas := []areaEntry{}
		for _, name := range input {
			if entry, ok := entriesByName[name]; ok {
				addAreas = append(addAreas, entry)
			}
		}
		if len(addAreas) == 0 {
			lines = append(lines, tr.TranslateFormat("No valid areas. Use `{0}{1} list`", ctx.Prefix, tr.Translate("area", true)))
		}
		added := []string{}
		for _, name := range input {
			if !containsString(current, name) && valid[name] {
				if entry, ok := entriesByName[name]; ok {
					added = append(added, entry.Name)
				}
			}
		}
		if len(added) > 0 {
			lines = append(lines, fmt.Sprintf("%s %s", tr.Translate("Added areas:", false), strings.Join(added, ", ")))
		}
	} else {
		removeAreas := []areaEntry{}
		for _, name := range input {
			if entry, ok := entriesByName[name]; ok {
				removeAreas = append(removeAreas, entry)
			}
		}
		if len(removeAreas) == 0 {
			lines = append(lines, tr.TranslateFormat("No valid areas. Use `{0}{1} list`", ctx.Prefix, tr.Translate("area", true)))
		}
		removed := []string{}
		for _, name := range input {
			if containsString(current, name) && valid[name] {
				if entry, ok := entriesByName[name]; ok {
					removed = append(removed, entry.Name)
				}
			}
		}
		if len(removed) > 0 {
			lines = append(lines, fmt.Sprintf("%s %s", tr.Translate("Removed areas:", false), strings.Join(removed, ", ")))
		}
	}
	lines = append(lines, currentAreaText(tr, ctx.Fences.Fences, filtered))
	return strings.Join(lines, "\n"), nil
}

func listAreas(tr *i18n.Translator, areas []areaEntry) string {
	header := tr.Translate("Current configured areas are:", false)
	lines := []string{header, "```"}
	currentGroup := ""
	for _, entry := range areas {
		if entry.Group != currentGroup {
			currentGroup = entry.Group
			if currentGroup != "" {
				lines = append(lines, currentGroup)
			}
		}
		name := strings.ReplaceAll(entry.Name, " ", "_")
		line := fmt.Sprintf("   %s", name)
		if !entry.Selectable {
			line += "🚫"
		}
		if entry.Description != "" {
			line += fmt.Sprintf(" - %s", entry.Description)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "```")
	return strings.Join(lines, "\n")
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func parseAreaListFromHuman(human map[string]any) []string {
	current := []string{}
	if raw, ok := human["area"].(string); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &current)
	}
	return current
}

func currentAreaText(tr *i18n.Translator, fences []geofence.Fence, selected []string) string {
	return trackedAreaText(tr, fences, selected)
}

func areaDefault(ctx *Context, tr *i18n.Translator, human map[string]any, areas []areaEntry, language string, target Target) string {
	current := parseAreaListFromHuman(human)
	lines := []string{currentAreaText(tr, ctx.Fences.Fences, current)}
	lines = append(lines, tr.TranslateFormat("Valid commands are `{0}area list`, `{0}area add <areaname>`, `{0}area remove <areaname>`", ctx.Prefix))
	if helpLine := singleLineHelpText(ctx, "area", language, target); helpLine != "" {
		lines = append(lines, helpLine)
	}
	return strings.Join(lines, "\n")
}

func showAreaOverview(ctx *Context, tr *i18n.Translator, human map[string]any, available []areaEntry, args []string) (string, error) {
	provider, _ := ctx.Config.GetString("geocoding.staticProvider")
	if !strings.EqualFold(provider, "tileservercache") {
		return "", nil
	}
	areas := args
	if len(areas) == 0 {
		areas = parseAreaListFromHuman(human)
	}
	client := tileserver.NewClient(ctx.Config)
	url, err := tileserver.GenerateGeofenceOverviewTile(ctx.Fences.Fences, client, ctx.Config, areas)
	if err != nil {
		return "", err
	}
	if url == "" {
		return "", nil
	}
	return fmt.Sprintf("%s: %s", tr.Translate("Overview display", false), url), nil
}

func showAreas(ctx *Context, tr *i18n.Translator, human map[string]any, available []areaEntry, args []string, re *RegexSet) (string, error) {
	provider, _ := ctx.Config.GetString("geocoding.staticProvider")
	if !strings.EqualFold(provider, "tileservercache") {
		return "", nil
	}
	areas := args
	if len(areas) == 0 {
		areas = parseAreaListFromHuman(human)
	}
	client := tileserver.NewClient(ctx.Config)
	lines := []string{}
	for _, area := range areas {
		if distance, ok := parseDistanceArg(area, re); ok {
			lat := toFloat(human["latitude"])
			lon := toFloat(human["longitude"])
			if lat == 0 && lon == 0 {
				lines = append(lines, tr.Translate("You have not set a location yet", false))
				continue
			}
			url, err := tileserver.GenerateDistanceTile(client, ctx.Config, lat, lon, distance)
			if err != nil {
				return "", err
			}
			if url != "" {
				lines = append(lines, fmt.Sprintf("%s %s", tr.TranslateFormat("Area display: {0}", area), url))
			}
			continue
		}
		url, err := tileserver.GenerateGeofenceTile(ctx.Fences.Fences, client, ctx.Config, area)
		if err != nil {
			return "", err
		}
		if url != "" {
			lines = append(lines, fmt.Sprintf("%s %s", tr.TranslateFormat("Area display: {0}", area), url))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func parseDistanceArg(arg string, re *RegexSet) (float64, bool) {
	if !re.Distance.MatchString(arg) {
		return 0, false
	}
	match := re.Distance.FindStringSubmatch(arg)
	if len(match) < 3 {
		return 0, false
	}
	return toFloat(match[2]), true
}
