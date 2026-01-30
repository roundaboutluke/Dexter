package command

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"poraclego/internal/geo"
	"poraclego/internal/i18n"
	"poraclego/internal/webhook"
)

// WeatherCommand adds weather tracking.
type WeatherCommand struct{}

func (c *WeatherCommand) Name() string { return "weather" }

func (c *WeatherCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "weather") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}weather <location> | <condition>`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "weather", result.Language, result.Target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	locationArgs, conditionArgs := splitArgsOnPipe(args)
	if len(locationArgs) == 0 {
		return tr.Translate("Please provide a location.", false), nil
	}

	lat, lon, err := resolveLocation(ctx, tr, locationArgs, re)
	if err != nil {
		return err.Error(), nil
	}
	cell := geo.WeatherCellID(lat, lon)
	if cell == "" {
		return fmt.Sprintf("%s %s: %f,%f", tr.Translate("S2 cell not found", false), strings.Join(locationArgs, " "), lat, lon), nil
	}

	clean, conditionArgs := parseClean(conditionArgs)
	template, conditionArgs := parseTemplate(conditionArgs, re)
	conditions := parseWeatherConditions(ctx, conditionArgs)
	if len(conditions) == 0 {
		return "", nil
	}

	entries := []map[string]any{}
	for _, condition := range conditions {
		entries = append(entries, map[string]any{
			"id":         result.TargetID,
			"profile_no": result.ProfileNo,
			"ping":       ctx.Ping,
			"condition":  condition,
			"cell":       cell,
			"template":   toInt(template, 1),
			"clean":      boolToInt(clean),
		})
	}
	updated, err := ctx.Query.InsertOrUpdateQuery("weather", entries)
	if err != nil {
		return "", err
	}
	client := ""
	if ctx.Config != nil {
		client, _ = ctx.Config.GetString("database.client")
	}
	if updated > 0 || strings.EqualFold(client, "sqlite") {
		return "✅", nil
	}
	return "👌", nil
}

func splitArgsOnPipe(args []string) ([]string, []string) {
	for i, arg := range args {
		if arg == "|" {
			return args[:i], args[i+1:]
		}
	}
	return args, []string{}
}

func resolveLocation(ctx *Context, tr *i18n.Translator, args []string, re *RegexSet) (float64, float64, error) {
	if len(args) == 1 {
		if lat, lon, ok := parseLatLon(args[0], re); ok {
			return lat, lon, nil
		}
	}
	search := strings.Join(args, " ")
	geoClient := webhook.NewGeocoder(ctx.Config)
	results := geoClient.Forward(search)
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("%s: %s", tr.Translate("404 no locations found", false), search)
	}
	return results[0].Latitude, results[0].Longitude, nil
}

func parseWeatherConditions(ctx *Context, args []string) []int {
	if ctx == nil || ctx.Data == nil {
		return nil
	}
	conditions := map[int]bool{}
	for _, arg := range args {
		if strings.EqualFold(arg, "everything") {
			conditions[0] = true
		}
	}
	weathers := map[string]int{}
	validIDs := map[int]bool{}
	if raw, ok := ctx.Data.UtilData["weather"].(map[string]any); ok {
		for id, entry := range raw {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			weatherID := toInt(id, 0)
			if weatherID > 0 {
				validIDs[weatherID] = true
			}
			name := strings.TrimSpace(fmt.Sprintf("%v", entryMap["name"]))
			if name == "" {
				continue
			}
			weathers[strings.ToLower(name)] = weatherID
		}
	}

	for _, arg := range args {
		token := strings.ToLower(strings.TrimSpace(arg))
		if ctx.I18n != nil {
			token = strings.ToLower(strings.TrimSpace(ctx.I18n.ReverseTranslateCommand(arg, true)))
		}
		if token == "" {
			continue
		}
		if strings.EqualFold(token, "everything") {
			conditions[0] = true
			continue
		}
		if id, err := strconv.Atoi(token); err == nil {
			if id == 0 || validIDs[id] {
				conditions[id] = true
			}
			continue
		}
		if weatherID, ok := weathers[token]; ok {
			if weatherID == 0 || validIDs[weatherID] {
				conditions[weatherID] = true
			}
		}
	}

	out := []int{}
	for id := range conditions {
		out = append(out, id)
	}
	sort.Ints(out)
	return out
}
