package command

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/golang/geo/s2"

	"poraclego/internal/geo"
	"poraclego/internal/i18n"
	"poraclego/internal/tileserver"
	"poraclego/internal/uicons"
)

func infoWeather(ctx *Context, targetID string, args []string, re *RegexSet) string {
	tr := ctx.I18n.Translator(ctx.Language)
	if ctx.Weather == nil {
		return tr.Translate("Weather information is not yet available - wait a few minutes and try again", false)
	}
	lat := 0.0
	lon := 0.0
	if len(args) > 0 {
		if parsedLat, parsedLon, ok := parseLatLon(args[0], re); ok {
			lat = parsedLat
			lon = parsedLon
		} else {
			return tr.Translate("Could not understand the location", false)
		}
	} else {
		human, err := ctx.Query.SelectOneQuery("humans", map[string]any{"id": targetID})
		if err != nil || human == nil {
			return tr.Translate("Weather information is not available.", false)
		}
		lat = getFloat(human["latitude"])
		lon = getFloat(human["longitude"])
		if lat == 0 && lon == 0 {
			return tr.TranslateFormat("You have not set your location, use `{0}{1}`", ctx.Prefix, tr.Translate("help", true))
		}
	}
	cellID := geo.WeatherCellID(lat, lon)
	if cellID == "" {
		return tr.Translate("No weather information is available for this location", false)
	}
	cell := ctx.Weather.EnsureForecast(cellID, lat, lon)
	if cell == nil {
		return tr.Translate("No weather information is available for this location", false)
	}
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	current, ok := cell.Data[currentHour]
	if !ok || current == 0 {
		return tr.Translate("No weather information is available for this location", false)
	}
	platform := normalizePlatform(ctx.Platform)
	weatherName, weatherEmojiKey := weatherInfoFromData(ctx, current)
	weatherEmoji := translateMaybe(tr, lookupEmojiByKey(ctx, weatherEmojiKey, platform))
	message := tr.TranslateFormat("Current Weather: {0} {1}", tr.Translate(weatherName, false), weatherEmoji)

	lines := []string{}
	location := time.Local
	if ctx.Timezone != nil {
		if loc, ok := ctx.Timezone.Location(lat, lon); ok && loc != nil {
			location = loc
		}
	}
	for ts := currentHour + 3600; ts <= currentHour+12*3600; ts += 3600 {
		value, ok := cell.Data[ts]
		if !ok || value == 0 {
			break
		}
		name, emojiKey := weatherInfoFromData(ctx, value)
		emoji := translateMaybe(tr, lookupEmojiByKey(ctx, emojiKey, platform))
		timestamp := time.Unix(ts, 0).In(location)
		lines = append(lines, fmt.Sprintf("%s - %s %s", timestamp.Format("15:04"), tr.Translate(name, false), emoji))
	}
	forecast := tr.Translate("No forecast available", false)
	if len(lines) > 0 {
		forecast = fmt.Sprintf("**%s:**\n%s", tr.Translate("Forecast", false), strings.Join(lines, "\n"))
	}
	staticMap := weatherStaticMap(ctx, cellID, current)
	if staticMap != "" {
		if normalizePlatform(ctx.Platform) == "discord" {
			payload := map[string]any{
				"content": message,
				"embed": map[string]any{
					"description": forecast,
					"image": map[string]any{
						"url": staticMap,
					},
				},
			}
			if raw, err := json.Marshal(payload); err == nil {
				return DiscordEmbedPrefix + string(raw)
			}
		}
		return message + "\n\n" + forecast + "\n\n" + staticMap
	}
	return message + "\n\n" + forecast
}

func weatherInfoFromData(ctx *Context, weatherID int) (string, string) {
	if ctx == nil || ctx.Data == nil {
		return "Unknown", ""
	}
	raw, ok := ctx.Data.UtilData["weather"].(map[string]any)
	if !ok {
		return "Unknown", ""
	}
	entry, ok := raw[fmt.Sprintf("%d", weatherID)].(map[string]any)
	if !ok {
		return "Unknown", ""
	}
	name := fmt.Sprintf("%v", entry["name"])
	emoji := fmt.Sprintf("%v", entry["emoji"])
	return name, emoji
}

func weatherStaticMap(ctx *Context, cellID string, weatherID int) string {
	if ctx == nil || ctx.Config == nil || ctx.Data == nil {
		return ""
	}
	provider, _ := ctx.Config.GetString("geocoding.staticProvider")
	if !strings.EqualFold(provider, "tileservercache") {
		return ""
	}
	base := ""
	if value, ok := ctx.Config.GetString("general.imgUrl"); ok {
		base = value
	}
	if base == "" {
		return ""
	}
	cell, ok := parseCell(cellID)
	if !ok {
		return ""
	}
	data := map[string]any{
		"condition": weatherID,
	}
	if coords := cellCoords(cell); len(coords) > 0 {
		data["coords"] = coords
	}
	center := cell.LatLng()
	data["latitude"] = center.Lat.Degrees()
	data["longitude"] = center.Lng.Degrees()
	ui := uicons.NewClient(base, "png")
	if url, ok := ui.WeatherIcon(weatherID); ok {
		data["imgUrl"] = url
	} else if fallback, ok := ctx.Config.GetString("fallbacks.imgUrlWeather"); ok && fallback != "" {
		data["imgUrl"] = fallback
	}
	opts := tileserver.TileOptions{Type: "staticMap", Pregenerate: true}
	client := tileserver.NewClient(ctx.Config)
	url, err := client.GetMapURL("weather", data, opts)
	if err != nil {
		return ""
	}
	return url
}

func parseCell(cellID string) (s2.CellID, bool) {
	id, err := strconv.ParseUint(cellID, 10, 64)
	if err != nil {
		return 0, false
	}
	return s2.CellID(id), true
}

func cellCoords(cell s2.CellID) [][]float64 {
	if !cell.IsValid() {
		return nil
	}
	c := s2.CellFromCellID(cell)
	coords := make([][]float64, 0, 4)
	for i := 0; i < 4; i++ {
		ll := s2.LatLngFromPoint(c.Vertex(i))
		coords = append(coords, []float64{ll.Lat.Degrees(), ll.Lng.Degrees()})
	}
	return coords
}

func lookupEmojiByKey(ctx *Context, key string, platform string) string {
	if key == "" || ctx == nil || ctx.Data == nil || ctx.Data.UtilData == nil {
		return ""
	}
	raw, ok := ctx.Data.UtilData["emojis"].(map[string]any)
	if !ok {
		return ""
	}
	if value, ok := raw[key]; ok {
		return fmt.Sprintf("%v", value)
	}
	return ""
}

func normalizePlatform(platform string) string {
	if strings.EqualFold(platform, "webhook") {
		return "discord"
	}
	return platform
}

func translateMaybe(tr *i18n.Translator, value string) string {
	if tr == nil || value == "" {
		return value
	}
	return tr.Translate(value, false)
}

func getFloatConfig(ctx *Context, path string, fallback float64) float64 {
	raw, ok := ctx.Config.Get(path)
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return fallback
}
