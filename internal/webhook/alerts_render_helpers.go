package webhook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"poraclego/internal/data"
	"poraclego/internal/logging"
	"poraclego/internal/render"
	"poraclego/internal/tileserver"
	"poraclego/internal/uicons"
)

func normalizeIV(hook *Hook, raw any) string {
	if raw != nil {
		switch v := raw.(type) {
		case string:
			trimmed := strings.TrimSpace(v)
			if trimmed == "" {
				break
			}
			if trimmed == "-1" {
				return "-1"
			}
			if parsed, err := strconv.ParseFloat(trimmed, 64); err == nil {
				if parsed > 0 && parsed <= 1 {
					parsed *= 100
				}
				return fmt.Sprintf("%.0f", parsed)
			}
		case int, int64, float32, float64, json.Number:
			parsed := toFloat(v)
			if parsed > 0 && parsed <= 1 {
				parsed *= 100
			}
			return fmt.Sprintf("%.0f", parsed)
		}
	}
	ivValue := computeIV(hook)
	if ivValue < 0 {
		return "-1"
	}
	return fmt.Sprintf("%.0f", ivValue)
}

func pokemonTypeColor(p *Processor, hook *Hook) string {
	d := p.getData()
	if d == nil || d.Monsters == nil || d.UtilData == nil {
		return ""
	}
	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		return ""
	}
	form := getInt(hook.Message["form"])
	if form == 0 {
		form = getInt(hook.Message["form_id"])
	}
	if form == 0 {
		form = getInt(hook.Message["pokemon_form"])
	}
	key := fmt.Sprintf("%d_%d", pokemonID, form)
	monster := lookupMonster(d, key)
	if monster == nil && form != 0 {
		monster = lookupMonster(d, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(d, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return ""
	}
	typesRaw, ok := monster["types"]
	if !ok {
		return ""
	}
	types, ok := typesRaw.([]any)
	if !ok || len(types) == 0 {
		return ""
	}
	first, ok := types[0].(map[string]any)
	if !ok {
		return ""
	}
	typeName := getString(first["name"])
	if typeName == "" {
		return ""
	}
	utilTypes, ok := d.UtilData["types"].(map[string]any)
	if !ok {
		return ""
	}
	typeEntry, ok := utilTypes[typeName].(map[string]any)
	if !ok {
		return ""
	}
	color := getString(typeEntry["color"])
	return color
}

func lookupMonster(data *data.GameData, key string) map[string]any {
	if data == nil || data.Monsters == nil {
		return nil
	}
	raw, ok := data.Monsters[key]
	if !ok {
		return nil
	}
	monster, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return monster
}

func renderAny(value any, data map[string]any, meta map[string]any, p *Processor) any {
	switch v := value.(type) {
	case string:
		if includePath, ok := includeDirective(v); ok {
			content, err := loadInclude(p, includePath)
			if err != nil {
				return fmt.Sprintf("Cannot load @include - %s", v)
			}
			v = content
		}
		v = normalizeHelperBlocks(v)
		rendered, err := render.RenderHandlebars(v, data, meta)
		if err != nil {
			if logger := logging.Get().General; logger != nil {
				snippet := v
				if len(snippet) > 200 {
					snippet = snippet[:200] + "..."
				}
				logger.Errorf("template render failed: %v [%s]", err, snippet)
			}
			return v
		}
		return shortenRenderedString(p, rendered)
	case []any:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, renderAny(item, data, meta, p))
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, item := range v {
			out[key] = renderAny(item, data, meta, p)
		}
		return out
	default:
		return value
	}
}

var shortLinkRe = regexp.MustCompile(`<S<(.*?)>S>`)
var orBlockRe = regexp.MustCompile(`\{\{#or\s+([^\s}]+)\s+([^\s}]+)\s+([^\s}]+)\s*\}\}`)
var andBlockRe = regexp.MustCompile(`\{\{#and\s+([^\s}]+)\s+([^\s}]+)\s+([^\s}]+)\s*\}\}`)

func normalizeHelperBlocks(template string) string {
	if strings.Contains(template, "{{#or") {
		template = orBlockRe.ReplaceAllString(template, "{{#or $1 (or $2 $3)}}")
	}
	if strings.Contains(template, "{{#and") {
		template = andBlockRe.ReplaceAllString(template, "{{#and $1 (and $2 $3)}}")
	}
	return template
}

func shortenRenderedString(p *Processor, rendered string) string {
	if !strings.Contains(rendered, "<S<") {
		return rendered
	}
	return shortLinkRe.ReplaceAllStringFunc(rendered, func(match string) string {
		submatches := shortLinkRe.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		short := shortenURL(p, submatches[1])
		if short == "" {
			return submatches[1]
		}
		return short
	})
}

func includeDirective(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if !strings.HasPrefix(trimmed, "@include") {
		return "", false
	}
	parts := strings.Fields(trimmed)
	if len(parts) < 2 {
		return "", false
	}
	return parts[1], true
}

func loadInclude(p *Processor, includePath string) (string, error) {
	if p == nil || p.root == "" {
		return "", fmt.Errorf("missing root")
	}
	base := configDir(p.root)
	path := includePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(base, "dts", includePath)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func configDir(root string) string {
	base := filepath.Join(root, "config")
	if dir := os.Getenv("NODE_CONFIG_DIR"); dir != "" {
		if filepath.IsAbs(dir) {
			base = dir
		} else {
			base = filepath.Join(root, dir)
		}
	}
	return base
}

func prepareMapPosition(p *Processor, hook *Hook, data map[string]any) {
	if hook == nil {
		return
	}
	switch hook.Type {
	case "nest":
		polygons := parsePolygonPaths(hook.Message["poly_path"])
		if len(polygons) == 0 {
			return
		}
		zoom, lat, lon := tileserver.Autoposition(tileserver.ShapeSet{Polygons: polygons}, 500, 250, 1.25, 17.5)
		if zoom > 16 {
			zoom = 16
		}
		hook.Message["map_latitude"] = lat
		hook.Message["map_longitude"] = lon
		hook.Message["zoom"] = zoom
	case "fort_update":
		markers := []tileserver.Point{}
		if lat, lon, ok := extractLocation(hook.Message["old"]); ok {
			markers = append(markers, tileserver.Point{Latitude: lat, Longitude: lon})
		}
		if lat, lon, ok := extractLocation(hook.Message["new"]); ok {
			markers = append(markers, tileserver.Point{Latitude: lat, Longitude: lon})
		}
		if len(markers) == 0 {
			return
		}
		zoom, lat, lon := tileserver.Autoposition(tileserver.ShapeSet{Markers: markers}, 500, 250, 1.25, 17.5)
		if zoom > 16 {
			zoom = 16
		}
		hook.Message["map_latitude"] = lat
		hook.Message["map_longitude"] = lon
		hook.Message["zoom"] = zoom
	}

	if data != nil {
		if value, ok := hook.Message["map_latitude"]; ok {
			data["map_latitude"] = value
		}
		if value, ok := hook.Message["map_longitude"]; ok {
			data["map_longitude"] = value
		}
		if value, ok := hook.Message["zoom"]; ok {
			data["zoom"] = value
		}
	}
}

func parsePolygonPaths(raw any) [][]tileserver.Point {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		if v == "" {
			return nil
		}
		var decoded any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return parsePolygonPaths(decoded)
		}
	case []any:
		polygons := [][]tileserver.Point{}
		for _, poly := range v {
			points := parsePolygonPoints(poly)
			if len(points) > 0 {
				polygons = append(polygons, points)
			}
		}
		return polygons
	}
	return nil
}

func parsePolygonPoints(raw any) []tileserver.Point {
	switch v := raw.(type) {
	case []any:
		points := []tileserver.Point{}
		for _, entry := range v {
			switch pair := entry.(type) {
			case []any:
				if len(pair) < 2 {
					continue
				}
				lat := getFloat(pair[0])
				lon := getFloat(pair[1])
				points = append(points, tileserver.Point{Latitude: lat, Longitude: lon})
			case map[string]any:
				lat := getFloat(pair["latitude"])
				if lat == 0 {
					lat = getFloat(pair["lat"])
				}
				lon := getFloat(pair["longitude"])
				if lon == 0 {
					lon = getFloat(pair["lon"])
				}
				points = append(points, tileserver.Point{Latitude: lat, Longitude: lon})
			}
		}
		return points
	}
	return nil
}

func extractLocation(raw any) (float64, float64, bool) {
	entry, ok := raw.(map[string]any)
	if !ok {
		return 0, 0, false
	}
	locRaw, ok := entry["location"]
	if !ok {
		locRaw = entry
	}
	loc, ok := locRaw.(map[string]any)
	if !ok {
		return 0, 0, false
	}
	lat := getFloat(loc["lat"])
	if lat == 0 {
		lat = getFloat(loc["latitude"])
	}
	lon := getFloat(loc["lon"])
	if lon == 0 {
		lon = getFloat(loc["longitude"])
	}
	if lat == 0 && lon == 0 {
		return 0, 0, false
	}
	return lat, lon, true
}

func hookEventPosition(hook *Hook) (float64, float64) {
	if hook == nil || hook.Message == nil {
		return 0, 0
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 {
		lat = getFloat(hook.Message["lat"])
	}
	if lon == 0 {
		lon = getFloat(hook.Message["lon"])
		if lon == 0 {
			lon = getFloat(hook.Message["lng"])
		}
	}
	if (lat == 0 && lon == 0) && hook.Type == "fort_update" {
		if lat, lon, ok := extractLocation(hook.Message["new"]); ok {
			return lat, lon
		}
		if lat, lon, ok := extractLocation(hook.Message["old"]); ok {
			return lat, lon
		}
	}
	return lat, lon
}

func staticMapURL(p *Processor, hook *Hook, data map[string]any) string {
	if p == nil || p.cfg == nil {
		return ""
	}
	provider, _ := p.cfg.GetString("geocoding.staticProvider")
	provider = strings.ToLower(provider)
	eventLat, eventLon := hookEventPosition(hook)
	if eventLat == 0 && eventLon == 0 {
		return ""
	}
	if provider == "tileservercache" {
		centerLat, centerLon, zoomOverride := mapPositionForHook(hook)
		if centerLat == 0 && centerLon == 0 {
			centerLat = eventLat
			centerLon = eventLon
		}
		return tileserverMapURL(p, hook, data, centerLat, centerLon, eventLat, eventLon, zoomOverride)
	}
	width := getIntFromConfig(p.cfg, "geocoding.width", 400)
	height := getIntFromConfig(p.cfg, "geocoding.height", 200)
	zoom := getIntFromConfig(p.cfg, "geocoding.zoom", 15)
	keys := getStringSliceFromConfig(p.cfg, "geocoding.staticKey")
	key := ""
	if len(keys) > 0 {
		key = keys[int(time.Now().UnixNano())%len(keys)]
	}
	switch provider {
	case "mapbox":
		if key == "" {
			return ""
		}
		// Match PoracleJS behavior: add a marker icon overlay.
		const marker = "url-https%3A%2F%2Fi.imgur.com%2FMK4NUzI.png"
		return fmt.Sprintf("https://api.mapbox.com/styles/v1/mapbox/streets-v10/static/%s(%f,%f)/%f,%f,%d,0,0/%dx%d?access_token=%s",
			marker, eventLon, eventLat, eventLon, eventLat, zoom, width, height, key)
	case "osm":
		if key == "" {
			return ""
		}
		// Match PoracleJS defaultMarker styling.
		return fmt.Sprintf("https://www.mapquestapi.com/staticmap/v5/map?locations=%f,%f&size=%d,%d&defaultMarker=marker-md-3B5998-22407F&zoom=%d&key=%s",
			eventLat, eventLon, width, height, zoom, key)
	case "google":
		if key == "" {
			return ""
		}
		mapType := getStringFromConfig(p.cfg, "geocoding.type", "roadmap")
		return fmt.Sprintf("https://maps.googleapis.com/maps/api/staticmap?center=%f,%f&markers=color:red|%f,%f&maptype=%s&zoom=%d&size=%dx%d&key=%s",
			eventLat, eventLon, eventLat, eventLon, mapType, zoom, width, height, key)
	default:
		// PoracleJS leaves staticMap blank for "none" and unknown providers (then falls back).
		return ""
	}
}

func tileserverMapURL(p *Processor, hook *Hook, data map[string]any, centerLat, centerLon, eventLat, eventLon float64, zoomOverride float64) string {
	if p == nil || p.cfg == nil {
		return ""
	}
	templateType := tileserverTemplateForHook(hook.Type)
	mapType := tileserverMapTypeForHook(hook.Type)
	opts := tileserver.GetOptions(p.cfg, mapType)
	if strings.EqualFold(opts.Type, "none") {
		return ""
	}
	if hook.Type == "weather" {
		// PoracleJS always uses the pregenerated tileserver endpoint for weather maps. It only suppresses
		// generating a map when the user enabled altered-pokemon static maps but did not enable altered-pokemon
		// tracking at all.
		showAltered := getBoolFromConfig(p.cfg, "weather.showAlteredPokemon", false)
		showAlteredMap := getBoolFromConfig(p.cfg, "weather.showAlteredPokemonStaticMap", false)
		if showAlteredMap && !showAltered {
			return ""
		}
		opts.Pregenerate = true
	}
	boundsZoom := opts.Zoom
	if zoomOverride > 0 {
		opts.Zoom = zoomOverride
	}
	payload := map[string]any{}
	for key, value := range data {
		payload[key] = value
	}
	if eventLat == 0 && eventLon == 0 {
		eventLat = centerLat
		eventLon = centerLon
	}
	payload["latitude"] = eventLat
	payload["longitude"] = eventLon
	if hook.Type == "nest" || hook.Type == "fort_update" {
		payload["map_latitude"] = centerLat
		payload["map_longitude"] = centerLon
		if zoomOverride > 0 {
			payload["zoom"] = opts.Zoom
		}
	}
	if getString(payload["imgUrl"]) == "" {
		imgURL := selectImageURL(p, hook)
		if imgURL == "" {
			imgURL = fallbackImageURL(p.cfg, hook.Type)
		}
		payload["imgUrl"] = imgURL
	}
	if opts.Pregenerate && opts.IncludeStops && p.scanner != nil {
		bounds := tileserver.Limits(eventLat, eventLon, float64(opts.Width), float64(opts.Height), boundsZoom)
		baseURL := ""
		if p.cfg != nil {
			baseURL = getStringFromConfig(p.cfg, "general.imgUrl", "")
		}
		var client *uicons.Client
		if baseURL != "" && isUiconsRepo(baseURL, "png") {
			client = uiconsClient(baseURL, "png")
			if url, _ := client.PokestopIcon(0, false, 0, false); url != "" {
				payload["uiconPokestopUrl"] = url
			}
		}
		stops, err := p.scanner.GetStopData(bounds.MinLat, bounds.MinLon, bounds.MaxLat, bounds.MaxLon)
		if err == nil {
			if len(stops) == 0 {
				payload["nearbyStops"] = []map[string]any{}
			} else {
				nearbyStops := make([]map[string]any, 0, len(stops))
				fallbackGymURL := ""
				if client != nil && p.cfg != nil {
					fallbackGymURL = getStringFromConfig(p.cfg, "fallbacks.imgUrlGym", "")
				}
				for _, stop := range stops {
					entry := map[string]any{
						"latitude":  stop.Latitude,
						"longitude": stop.Longitude,
						"type":      stop.Type,
						"teamId":    stop.TeamID,
						"slots":     stop.Slots,
					}
					if client != nil && stop.Type == "gym" {
						trainerCount := 6 - stop.Slots
						if trainerCount < 0 {
							trainerCount = 0
						}
						url, _ := client.GymIcon(stop.TeamID, trainerCount, false, false)
						if url == "" {
							url = fallbackGymURL
						}
						if url != "" {
							entry["imgUrl"] = url
						}
					}
					nearbyStops = append(nearbyStops, entry)
				}
				payload["nearbyStops"] = nearbyStops
			}
		}
	}
	if hook.Type == "weather" {
		showAltered := getBoolFromConfig(p.cfg, "weather.showAlteredPokemon", false)
		showAlteredMap := getBoolFromConfig(p.cfg, "weather.showAlteredPokemonStaticMap", false)
		if showAltered && !showAlteredMap {
			if _, ok := payload["activePokemons"]; ok {
				filtered := map[string]any{}
				for key, value := range payload {
					if key == "activePokemons" {
						continue
					}
					filtered[key] = value
				}
				payload = filtered
			}
		}
	}
	tileserverPayload := tileserverPayloadForHook(hook.Type, payload, opts.Pregenerate)
	client := tileserver.NewClient(p.cfg)
	url, err := client.GetMapURL(templateType, tileserverPayload, opts)
	if err != nil {
		if logger := logging.Get().Webhooks; logger != nil {
			logger.Warnf("tileserver map failed (%s/%s): %v", templateType, mapType, err)
		}
		return ""
	}
	return url
}

func tileserverPayloadForHook(hookType string, data map[string]any, pregenerate bool) map[string]any {
	if data == nil {
		return nil
	}
	var keys []string
	if pregenerate {
		switch hookType {
		case "pokemon":
			keys = []string{"pokemon_id", "display_pokemon_id", "latitude", "longitude", "verified", "costume", "form", "pokemonId", "generation", "weather", "confirmedTime", "shinyPossible", "seenType", "seen_type", "cell_coords", "imgUrl", "imgUrlAlt", "nightTime", "duskTime", "dawnTime", "style"}
		}
	} else {
		switch hookType {
		case "pokemon":
			keys = []string{"pokemon_id", "latitude", "longitude", "form", "costume", "imgUrl", "imgUrlAlt", "style"}
		case "raid":
			keys = []string{"pokemon_id", "latitude", "longitude", "form", "level", "imgUrl", "style"}
		case "egg":
			keys = []string{"latitude", "longitude", "level", "imgUrl"}
		case "max_battle":
			keys = []string{"battle_pokemon_id", "latitude", "longitude", "battle_pokemon_form", "battle_level", "imgUrl", "style"}
		case "quest":
			keys = []string{"latitude", "longitude", "imgUrl"}
		case "gym", "gym_details":
			keys = []string{"teamId", "latitude", "longitude", "imgUrl", "style"}
		case "invasion", "pokestop":
			keys = []string{"latitude", "longitude", "imgUrl", "gruntTypeId", "displayTypeId", "style"}
		case "lure":
			keys = []string{"latitude", "longitude", "imgUrl", "lureTypeId", "style"}
		case "nest":
			keys = []string{"map_latitude", "map_longitude", "zoom", "imgUrl", "poly_path"}
		case "fort_update":
			keys = []string{"map_latitude", "map_longitude", "longitude", "latitude", "zoom", "imgUrl", "isEditLocation", "oldLatitude", "oldLongitude", "newLatitude", "newLongitude"}
		}
	}
	if len(keys) == 0 {
		return data
	}
	filtered := map[string]any{}
	for _, key := range keys {
		if value, ok := data[key]; ok {
			filtered[key] = value
		}
	}
	if value, ok := data["nearbyStops"]; ok {
		filtered["nearbyStops"] = value
	}
	if value, ok := data["uiconPokestopUrl"]; ok {
		filtered["uiconPokestopUrl"] = value
	}
	return filtered
}

func mapPositionForHook(hook *Hook) (float64, float64, float64) {
	if hook == nil {
		return 0, 0, 0
	}
	lat := getFloat(hook.Message["map_latitude"])
	lon := getFloat(hook.Message["map_longitude"])
	zoom := getFloat(hook.Message["zoom"])
	if lat == 0 && lon == 0 {
		lat = getFloat(hook.Message["latitude"])
		lon = getFloat(hook.Message["longitude"])
	}
	return lat, lon, zoom
}

func tileserverTemplateForHook(hookType string) string {
	switch hookType {
	case "pokemon":
		return "monster"
	case "raid", "egg":
		return "raid"
	case "max_battle":
		return "maxbattle"
	case "quest":
		return "quest"
	case "invasion", "lure", "pokestop":
		return "pokestop"
	case "gym", "gym_details":
		return "gym"
	case "nest":
		return "nest"
	case "weather":
		return "weather"
	case "fort_update":
		return "fort-update"
	default:
		return "location"
	}
}

func tileserverMapTypeForHook(hookType string) string {
	switch hookType {
	case "pokemon":
		return "monster"
	case "raid", "egg":
		return "raid"
	case "max_battle":
		return "maxbattle"
	case "quest":
		return "quest"
	case "invasion", "lure", "pokestop":
		return "pokestop"
	case "gym", "gym_details":
		return "gym"
	case "nest":
		return "nest"
	case "weather":
		return "weather"
	case "fort_update":
		return "fort-update"
	default:
		return "location"
	}
}
