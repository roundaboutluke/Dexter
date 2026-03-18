package command

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"dexter/internal/db"
	"dexter/internal/tileserver"
	"dexter/internal/webhook"
)

var postalCodeRe = regexp.MustCompile(`^\d{1,5}$`)

// LocationCommand updates a user's location.
type LocationCommand struct{}

func (c *LocationCommand) Name() string { return "location" }

func (c *LocationCommand) Handle(ctx *Context, args []string) (string, error) {
	result := buildTarget(ctx, args)
	tr := ctx.I18n.Translator(result.Language)
	if !result.CanContinue {
		return result.Message, nil
	}
	if !commandAllowed(ctx, "location") {
		return tr.Translate("You do not have permission to execute this command", false), nil
	}
	re := NewRegexSet(ctx.I18n)
	_, args = resolveTarget(ctx, args, re)
	target := result.Target
	if len(args) == 0 {
		lines := []string{tr.TranslateFormat("Valid commands are e.g. `{0}location <lat>,<lon>`, `{0}location <your address>`", ctx.Prefix)}
		if helpLine := singleLineHelpText(ctx, "location", result.Language, target); helpLine != "" {
			lines = append(lines, helpLine)
		}
		return strings.Join(lines, "\n"), nil
	}

	profileNo := result.ProfileNo
	targetID := result.TargetID

	remove := false
	if len(args) == 1 && strings.EqualFold(args[0], "remove") {
		remove = true
	}

	lat := 0.0
	lon := 0.0
	placeConfirmation := ""
	if !remove {
		if len(args) == 1 {
			if parsedLat, parsedLon, ok := parseLatLon(args[0], re); ok {
				lat = parsedLat
				lon = parsedLon
			}
		}
		if lat == 0 && lon == 0 && len(args) == 1 {
			if !postalCodeRe.MatchString(args[0]) {
				return tr.Translate("Oops, you need to specify more than just a city name to locate accurately your position", false), nil
			}
		}
		if lat == 0 && lon == 0 {
			search := strings.Join(args, " ")
			geo := webhook.NewGeocoder(ctx.Config)
			results := geo.Forward(search)
			if len(results) == 0 {
				return "🙅", nil
			}
			lat = results[0].Latitude
			lon = results[0].Longitude
			if results[0].City != "" && results[0].Country != "" {
				placeConfirmation = fmt.Sprintf(" **%s - %s** ", results[0].City, results[0].Country)
			} else if results[0].Country != "" {
				placeConfirmation = fmt.Sprintf(" **%s** ", results[0].Country)
			}
		}

		if enabled, _ := ctx.Config.GetBool("areaSecurity.enabled"); enabled && !ctx.IsAdmin {
			human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": targetID})
			if err != nil {
				return "", err
			}
			if raw, ok := human["area_restriction"].(string); ok && raw != "" {
				allowed := []string{}
				_ = json.Unmarshal([]byte(raw), &allowed)
				if ctx.Fences != nil && len(allowed) > 0 {
					areas := ctx.Fences.PointInArea([]float64{lat, lon})
					permitted := false
					for _, area := range areas {
						if containsString(allowed, area) {
							permitted = true
							break
						}
					}
					if !permitted {
						return tr.Translate("This location is not your permitted area", false), nil
					}
				}
			}
		}
	}

	if remove {
		lat = 0
		lon = 0
	}

	if err := commitAlertStateTx(ctx, func(query *db.Query) error {
		if _, err := query.UpdateQuery("humans", map[string]any{"latitude": lat, "longitude": lon}, map[string]any{"id": targetID}); err != nil {
			return err
		}
		if _, err := query.UpdateQuery("profiles", map[string]any{"latitude": lat, "longitude": lon}, map[string]any{"id": targetID, "profile_no": profileNo}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return "", err
	}

	if remove {
		return tr.TranslateFormat("I have removed {0}'s  location", target.Name), nil
	}
	mapLink := fmt.Sprintf("https://maps.google.com/maps?q=%s,%s", formatFloat(lat), formatFloat(lon))
	message := fmt.Sprintf("👋, %s%s%s:\n%s", tr.Translate("I set ", false), target.Name, tr.Translate("'s location to the following coordinates in", false)+placeConfirmation, mapLink)
	platform := resolveHelpPlatform(target, ctx.Platform)
	if strings.EqualFold(platform, "discord") {
		if provider, _ := ctx.Config.GetString("geocoding.staticProvider"); strings.EqualFold(provider, "tileservercache") {
			opts := tileserver.GetOptions(ctx.Config, "location")
			if !strings.EqualFold(opts.Type, "none") {
				client := tileserver.NewClient(ctx.Config)
				if staticMap, err := tileserver.GenerateConfiguredLocationTile(client, ctx.Config, lat, lon); err == nil && staticMap != "" {
					payload := map[string]any{
						"embeds": []map[string]any{{
							"color":       0x00ff00,
							"title":       tr.Translate("New location", false),
							"description": fmt.Sprintf("%s%s%s", tr.Translate("I set ", false), target.Name, tr.Translate("'s location to the following coordinates in", false)+placeConfirmation),
							"image":       map[string]any{"url": staticMap},
							"url":         mapLink,
						}},
					}
					if raw, err := json.Marshal(payload); err == nil {
						return DiscordEmbedPrefix + string(raw), nil
					}
				}
			}
		}
	}
	return message, nil
}

func parseLatLon(arg string, re *RegexSet) (float64, float64, bool) {
	if !re.LatLon.MatchString(arg) {
		return 0, 0, false
	}
	match := re.LatLon.FindStringSubmatch(arg)
	if len(match) < 3 {
		return 0, 0, false
	}
	lat, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return 0, 0, false
	}
	lon, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return 0, 0, false
	}
	return lat, lon, true
}

func formatFloat(value float64) string {
	if value == 0 {
		return "0"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.7f", value), "0"), ".")
}
