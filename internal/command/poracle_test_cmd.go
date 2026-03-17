package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var disappearRe = regexp.MustCompile(`(?i)^disapeardt:?(20\d\d-(0\d|1[0-2])-[0-3]\dT[0-2]\d:[0-5]\d:[0-5]\d)$`)

// PoracleTestCommand queues test webhook payloads (admin only).
type PoracleTestCommand struct{}

func (c *PoracleTestCommand) Name() string { return "poracle-test" }

func (c *PoracleTestCommand) Handle(ctx *Context, args []string) (string, error) {
	if !ctx.IsAdmin {
		return "🙅", nil
	}
	result := buildTarget(ctx, args)
	if !result.CanContinue {
		return result.Message, nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)

	if len(args) == 0 {
		return "Hooks supported are: pokemon, raid, pokestop, gym, nest, quest, fort-update, max-battle", nil
	}
	validHooks := map[string]bool{
		"pokemon":     true,
		"raid":        true,
		"pokestop":    true,
		"gym":         true,
		"nest":        true,
		"quest":       true,
		"fort-update": true,
		"max-battle":  true,
	}
	if !validHooks[strings.ToLower(args[0])] {
		return "Hooks supported are: pokemon, raid, pokestop, gym, nest, quest, fort-update, max-battle", nil
	}
	hookType := strings.ReplaceAll(strings.ToLower(args[0]), "-", "_")
	testdata, err := loadTestData(ctx.Root)
	if err != nil {
		return "Cannot read testdata.json - see log file for details", nil
	}
	template := defaultTemplateName(ctx)
	language, _ := ctx.Config.GetString("general.locale")
	disappearOverride := ""
	trimmed := []string{args[0]}
	languageNames := map[string]string{}
	if rawNames, ok := ctx.Data.UtilData["languageNames"].(map[string]any); ok {
		for key, value := range rawNames {
			languageNames[key] = fmt.Sprintf("%v", value)
		}
	}
	for _, arg := range args[1:] {
		if re.Template.MatchString(arg) {
			match := re.Template.FindStringSubmatch(arg)
			if len(match) > 2 {
				template = match[2]
			}
			continue
		}
		if re.Language.MatchString(arg) {
			match := re.Language.FindStringSubmatch(arg)
			if len(match) > 2 {
				name := ctx.I18n.ReverseTranslateCommand(match[2], true)
				langKey := name
				for key, label := range languageNames {
					if strings.EqualFold(label, name) {
						langKey = key
						break
					}
				}
				language = langKey
			}
			continue
		}
		if disappearRe.MatchString(arg) {
			match := disappearRe.FindStringSubmatch(arg)
			if len(match) > 1 {
				disappearOverride = match[1]
			}
			continue
		}
		trimmed = append(trimmed, arg)
	}
	if len(trimmed) == 1 {
		tests := []string{}
		for _, entry := range testdata {
			if entry.Type == hookType {
				tests = append(tests, entry.Test)
			}
		}
		lines := []string{fmt.Sprintf("Tests found for hook type %s:", hookType), ""}
		for _, test := range tests {
			lines = append(lines, fmt.Sprintf("  %s", test))
		}
		return strings.Join(lines, "\n"), nil
	}
	if len(trimmed) < 2 {
		lines := []string{fmt.Sprintf("Tests found for hook type %s:", hookType), ""}
		for _, entry := range testdata {
			if entry.Type == hookType {
				lines = append(lines, fmt.Sprintf("  %s", entry.Test))
			}
		}
		return strings.Join(lines, "\n"), nil
	}
	testID := trimmed[1]
	var selected *testEntry
	for _, entry := range testdata {
		if entry.Type == hookType && entry.Test == testID {
			selected = &entry
			break
		}
	}
	if selected == nil {
		return fmt.Sprintf("Cannot find hook type %s test id %s", hookType, testID), nil
	}

	human, _ := ctx.Query.SelectOneQuery("humans", map[string]any{"id": result.TargetID})
	lat := getFloat(human["latitude"])
	lon := getFloat(human["longitude"])

	hook := map[string]any{}
	for key, value := range selected.Webhook {
		hook[key] = value
	}
	if selected.Location != "keep" {
		if _, ok := hook["latitude"]; ok {
			hook["latitude"] = lat
		}
		if _, ok := hook["longitude"]; ok {
			hook["longitude"] = lon
		}
	}
	switch hookType {
	case "pokemon":
		if disappearOverride != "" {
			if ctx.Timezone != nil {
				if loc, ok := ctx.Timezone.Location(lat, lon); ok {
					if parsed, err := time.ParseInLocation("2006-01-02T15:04:05", strings.ToUpper(disappearOverride), loc); err == nil {
						hook["disappear_time"] = parsed.Unix()
						break
					}
				}
			}
			if parsed, err := time.Parse("2006-01-02T15:04:05", strings.ToUpper(disappearOverride)); err == nil {
				hook["disappear_time"] = parsed.Unix()
			}
		} else {
			hook["disappear_time"] = time.Now().Unix() + 10*60
		}
	case "raid":
		start := time.Now().Unix() + 10*60
		hook["start"] = start
		hook["end"] = start + 30*60
	case "pokestop":
		if _, ok := hook["incident_expiration"]; ok {
			hook["incident_expiration"] = time.Now().Unix() + 10*60
		}
		if _, ok := hook["incident_expire_timestamp"]; ok {
			hook["incident_expire_timestamp"] = time.Now().Unix() + 10*60
		}
		if _, ok := hook["lure_expiration"]; ok {
			hook["lure_expiration"] = time.Now().Unix() + 5*60
		}
	case "fort_update":
		if old, ok := hook["old"].(map[string]any); ok {
			if loc, ok := old["location"].(map[string]any); ok {
				loc["lat"] = lat
				loc["lon"] = lon
			}
		}
		if newer, ok := hook["new"].(map[string]any); ok {
			if loc, ok := newer["location"].(map[string]any); ok {
				loc["lat"] = lat + 0.001
				loc["lon"] = lon + 0.001
			}
		}
	}
	hook["poracleTest"] = map[string]any{
		"type":      result.Target.Type,
		"id":        result.TargetID,
		"name":      result.Target.Name,
		"latitude":  lat,
		"longitude": lon,
		"language":  language,
		"template":  template,
	}
	if ctx.WebhookQueue != nil {
		ctx.WebhookQueue.Push(map[string]any{
			"type":    hookType,
			"message": hook,
		})
	}
	return fmt.Sprintf("Queueing %s test hook [%s] template [%s]", hookType, testID, template), nil
}

type testEntry struct {
	Type     string         `json:"type"`
	Test     string         `json:"test"`
	Location string         `json:"location"`
	Webhook  map[string]any `json:"webhook"`
}

func loadTestData(root string) ([]testEntry, error) {
	path := filepath.Join(root, "config", "testdata.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("testdata.json not found")
	}
	payload = stripJSONCommentsLocal(payload)
	var entries []testEntry
	if err := json.Unmarshal(payload, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func getFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func stripJSONCommentsLocal(input []byte) []byte {
	out := make([]byte, 0, len(input))
	inString := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if ch == '"' {
			if i == 0 || input[i-1] != '\\' {
				inString = !inString
			}
			out = append(out, ch)
			continue
		}
		if !inString && ch == '/' && i+1 < len(input) {
			next := input[i+1]
			if next == '/' {
				for i < len(input) && input[i] != '\n' {
					i++
				}
				if i < len(input) {
					out = append(out, '\n')
				}
				continue
			}
			if next == '*' {
				i += 2
				for i+1 < len(input) && !(input[i] == '*' && input[i+1] == '/') {
					i++
				}
				i++
				continue
			}
		}
		out = append(out, ch)
	}
	return out
}
