package command

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"poraclego/internal/dispatch"
	"poraclego/internal/i18n"
)

// BroadcastCommand sends a broadcast message to users (admin only).
type BroadcastCommand struct{}

func (c *BroadcastCommand) Name() string { return "broadcast" }

func (c *BroadcastCommand) Handle(ctx *Context, args []string) (string, error) {
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

	matchedAreas := []string{}
	distance := 0
	latitude := 0.0
	longitude := 0.0
	test := false

	for i := len(args) - 1; i >= 0; i-- {
		if re.Area.MatchString(args[i]) {
			match := re.Area.FindStringSubmatch(args[i])
			if len(match) > 2 {
				matchedAreas = append(matchedAreas, match[2])
			}
			args = append(args[:i], args[i+1:]...)
		} else if re.Distance.MatchString(args[i]) {
			match := re.Distance.FindStringSubmatch(args[i])
			if len(match) > 2 {
				distance = toInt(match[2], 0)
			}
			args = append(args[:i], args[i+1:]...)
		}
	}

	if len(args) > 0 && re.LatLon.MatchString(args[0]) {
		match := re.LatLon.FindStringSubmatch(args[0])
		if len(match) >= 3 {
			latitude = toFloat(match[1])
			longitude = toFloat(match[2])
			args = args[1:]
		}
	}

	if latitude == 0 && longitude == 0 && len(matchedAreas) == 0 {
		if len(args) > 0 && strings.EqualFold(args[0], "test") {
			test = true
			args = args[1:]
		} else {
			return tr.Translate("No location or areas specified", false), nil
		}
	}

	if len(args) == 0 {
		return tr.Translate("Blank message!", false), nil
	}
	if latitude != 0 && distance == 0 {
		return tr.Translate("Location specified without any distance", false), nil
	}

	templateID := strings.Join(args, " ")
	messages, err := loadBroadcastMessages(ctx.Root)
	if err != nil {
		return tr.Translate("Cannot read broadcast.json - see log file for details", false), nil
	}
	discordMessage := ""
	telegramMessage := ""
	for _, entry := range messages {
		if fmt.Sprintf("%v", entry.ID) != templateID {
			continue
		}
		if entry.Platform == "" || entry.Platform == "discord" {
			discordMessage = entry.Template
		}
		if entry.Platform == "" || entry.Platform == "telegram" {
			telegramMessage = entry.Template
		}
	}
	if discordMessage == "" && telegramMessage == "" {
		return tr.Translate("Cannot find this broadcast message", false), nil
	}

	if test {
		return sendBroadcastJob(ctx, tr, result.TargetID, result.Target.Name, result.Target.Type, discordMessage, telegramMessage), nil
	}

	query := buildBroadcastQuery(latitude, longitude, distance, matchedAreas)
	rows, err := ctx.Query.MysteryQuery(query)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return tr.Translate("No recipients matched.", false), nil
	}
	for _, row := range rows {
		targetType := fmt.Sprintf("%v", row["type"])
		if strings.HasPrefix(targetType, "telegram") && telegramMessage == "" {
			return tr.Translate("Not sending any messages - You do not have a message defined for all platforms in your distribution list", false), nil
		}
		if !strings.HasPrefix(targetType, "telegram") && discordMessage == "" {
			return tr.Translate("Not sending any messages - You do not have a message defined for all platforms in your distribution list", false), nil
		}
	}
	names := []string{}
	for _, row := range rows {
		targetType := fmt.Sprintf("%v", row["type"])
		targetID := fmt.Sprintf("%v", row["id"])
		name := fmt.Sprintf("%v", row["name"])
		names = append(names, name)
		message := discordMessage
		if strings.HasPrefix(targetType, "telegram") {
			message = telegramMessage
		}
		job := dispatch.MessageJob{
			Type:       targetType,
			Target:     targetID,
			Name:       name,
			Message:    message,
			Clean:      false,
			TTH:        dispatch.TimeToHide{Hours: 1},
			AlwaysSend: true,
		}
		if strings.HasPrefix(targetType, "telegram") {
			if ctx.TelegramQueue != nil {
				ctx.TelegramQueue.Push(job)
			}
		} else {
			if ctx.DiscordQueue != nil {
				ctx.DiscordQueue.Push(job)
			}
		}
	}
	return fmt.Sprintf("%s %s", tr.Translate("I sent your message to", false), strings.Join(names, ", ")), nil
}

type broadcastEntry struct {
	ID       any    `json:"id"`
	Platform string `json:"platform"`
	Template string `json:"template"`
}

func loadBroadcastMessages(root string) ([]broadcastEntry, error) {
	path := filepath.Join(root, "config", "broadcast.json")
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("broadcast.json not found")
	}
	payload = stripJSONComments(payload)
	var entries []broadcastEntry
	if err := json.Unmarshal(payload, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func buildBroadcastQuery(lat, lon float64, distance int, areas []string) string {
	areaFilter := "1 = 0"
	for _, area := range areas {
		safe := strings.ReplaceAll(area, "'", "\\'")
		areaFilter += fmt.Sprintf(" OR humans.area like '%%\"%s\"%%'", safe)
	}
	locationFilter := ""
	if lat != 0 && lon != 0 && distance > 0 {
		locationFilter = fmt.Sprintf(`
			(
				round(
					6371000
					* acos(
						cos( radians(%f) )
						* cos( radians( humans.latitude ) )
						* cos( radians( humans.longitude ) - radians(%f) )
						+ sin( radians(%f) )
						* sin( radians( humans.latitude ) )
					)
				) < %d
			) OR`, lat, lon, lat, distance)
	}
	query := fmt.Sprintf(`
		select humans.id, humans.name, humans.type, humans.language, humans.latitude, humans.longitude
		from humans
		where humans.enabled = 1 and humans.admin_disable = 0 and humans.type like '%%:user'
		and (
			%s
			(%s)
		)
	`, locationFilter, areaFilter)
	return query
}

func sendBroadcastJob(ctx *Context, tr *i18n.Translator, targetID, targetName, targetType, discordMessage, telegramMessage string) string {
	message := discordMessage
	if strings.HasPrefix(targetType, "telegram") {
		message = telegramMessage
	}
	if message == "" {
		return tr.Translate("You do not have a message defined for this platform", false)
	}
	job := dispatch.MessageJob{
		Type:       targetType,
		Target:     targetID,
		Name:       targetName,
		Message:    message,
		Clean:      false,
		TTH:        dispatch.TimeToHide{Hours: 1},
		AlwaysSend: true,
	}
	if strings.HasPrefix(targetType, "telegram") {
		if ctx.TelegramQueue != nil {
			ctx.TelegramQueue.Push(job)
		}
	} else {
		if ctx.DiscordQueue != nil {
			ctx.DiscordQueue.Push(job)
		}
	}
	return "✅"
}

func stripJSONComments(input []byte) []byte {
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
