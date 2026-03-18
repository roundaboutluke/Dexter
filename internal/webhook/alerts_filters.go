package webhook

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"dexter/internal/alertstate"
	"dexter/internal/config"
	"dexter/internal/geofence"
)

func (p *Processor) loadHumansForRows(rows []map[string]any) (map[string]map[string]any, map[string]map[string]any, error) {
	idSet := map[string]bool{}
	for _, row := range rows {
		id := getString(row["id"])
		if id != "" {
			idSet[id] = true
		}
	}
	ids := make([]any, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	humanRows, err := p.query.SelectWhereInQuery("humans", ids, "id")
	if err != nil {
		return nil, nil, err
	}
	humans := map[string]map[string]any{}
	for _, row := range humanRows {
		humans[getString(row["id"])] = row
	}

	profileRows, err := p.query.SelectWhereInQuery("profiles", ids, "id")
	if err != nil {
		if logger := p.webhooksLogger(); logger != nil {
			logger.Warnf("failed to load profiles: %v", err)
		}
		return humans, nil, nil
	}
	profiles := map[string]map[string]any{}
	for _, row := range profileRows {
		key := alertstate.ProfileKey(getString(row["id"]), numberFromAnyOrDefault(row["profile_no"], 1))
		profiles[key] = row
	}
	return humans, profiles, nil
}

func resolveLocation(human, profile map[string]any) locationInfo {
	areaRaw := human["area"]
	restrictRaw := human["area_restriction"]
	lat := getFloat(human["latitude"])
	lon := getFloat(human["longitude"])
	if profile != nil {
		areaRaw = profile["area"]
		if v := getFloat(profile["latitude"]); v != 0 {
			lat = v
		}
		if v := getFloat(profile["longitude"]); v != 0 {
			lon = v
		}
	}
	return locationInfo{
		Lat:          lat,
		Lon:          lon,
		Areas:        parseAreas(areaRaw),
		Restrictions: parseAreas(restrictRaw),
	}
}

func parseAreas(raw any) []string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var items []string
		if err := json.Unmarshal([]byte(v), &items); err == nil {
			for i := range items {
				items[i] = normalizeAreaName(items[i])
			}
			return items
		}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, normalizeAreaName(s))
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, normalizeAreaName(item))
		}
		return out
	}
	return nil
}

func normalizeAreaName(name string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(name), "_", " "))
}

func passesLocationFilter(fences *geofence.Store, cfg *config.Config, location locationInfo, hook *Hook, row map[string]any) bool {
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 && lon == 0 {
		return true
	}

	specificGymMatch := false
	if hook != nil && (hook.Type == "raid" || hook.Type == "egg" || hook.Type == "gym" || hook.Type == "gym_details") {
		rowGymID := getString(row["gym_id"])
		if rowGymID != "" {
			hookGymID := getString(hook.Message["gym_id"])
			if hookGymID == "" {
				hookGymID = getString(hook.Message["id"])
			}
			// Match PoracleJS behavior: when a row is tied to a specific gym, it should only ever match that gym.
			if hookGymID == "" || rowGymID != hookGymID {
				return false
			}
			specificGymMatch = true
		}
	}

	specificStationMatch := false
	if hook != nil && hook.Type == "max_battle" {
		rowStationID := getString(row["station_id"])
		if rowStationID != "" {
			hookStationID := getString(hook.Message["id"])
			if hookStationID == "" {
				hookStationID = getString(hook.Message["stationId"])
			}
			// Match PoracleJS maxbattle tracking: station_id rows are specific to that station and do not use distance/area.
			if hookStationID == "" || rowStationID != hookStationID {
				return false
			}
			specificStationMatch = true
		}
	}

	distanceRaw, hasDistance := row["distance"]
	distance := getInt(distanceRaw)
	locationMatched := false
	switch {
	case specificGymMatch:
		locationMatched = true
	case specificStationMatch:
		locationMatched = true
	case distance > 0:
		if location.Lat != 0 || location.Lon != 0 {
			computed := distanceMeters(location.Lat, location.Lon, lat, lon)
			locationMatched = computed < distance
		}
	case hasDistance && distance == 0 && fences != nil && len(location.Areas) > 0:
		areas := fences.PointInArea([]float64{lat, lon})
		for _, area := range areas {
			if containsString(location.Areas, normalizeAreaName(area)) {
				locationMatched = true
				break
			}
		}
	case hasDistance && distance == 0:
		locationMatched = false
	default:
		locationMatched = true
	}
	if !locationMatched {
		return false
	}

	if cfg != nil {
		enabled, _ := cfg.GetBool("areaSecurity.enabled")
		strict, _ := cfg.GetBool("areaSecurity.strictLocations")
		if enabled && strict && location.Restrictions != nil {
			if fences == nil {
				return false
			}
			areas := fences.PointInArea([]float64{lat, lon})
			for _, area := range areas {
				if containsString(location.Restrictions, normalizeAreaName(area)) {
					return true
				}
			}
			return false
		}
	}
	return true
}

func parseBlockedAlerts(raw any) map[string]bool {
	out := map[string]bool{}
	if raw == nil {
		return out
	}
	value := strings.TrimSpace(getString(raw))
	if value == "" || strings.EqualFold(value, "null") {
		return out
	}
	var items []string
	if err := json.Unmarshal([]byte(value), &items); err == nil {
		for _, item := range items {
			item = strings.ToLower(strings.TrimSpace(item))
			if item != "" {
				out[item] = true
			}
		}
		return out
	}
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '|' || r == ' ' || r == ';'
	})
	for _, part := range parts {
		part = strings.ToLower(strings.TrimSpace(part))
		if part != "" {
			out[part] = true
		}
	}
	return out
}

func alertCategoryKey(hookType string) string {
	switch hookType {
	case "pokemon":
		return "monster"
	case "gym_details":
		return "gym"
	case "fort_update":
		return "forts"
	case "max_battle":
		return "maxbattle"
	default:
		return hookType
	}
}

func pvpSecurityEnabled(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	raw, ok := cfg.Get("discord.commandSecurity")
	if !ok {
		return false
	}
	security, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	value, ok := security["pvp"]
	if !ok || value == nil {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case []any:
		return len(v) > 0
	case []string:
		return len(v) > 0
	default:
		return true
	}
}

func haversineKm(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadius * c
}

func distanceMeters(lat1, lon1, lat2, lon2 float64) int {
	const earthRadius = 6371e3
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(phi1)*math.Cos(phi2)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return int(math.Ceil(earthRadius * c))
}

func bearingDegrees(lat1, lon1, lat2, lon2 float64) float64 {
	phi1 := lat1 * math.Pi / 180
	phi2 := lat2 * math.Pi / 180
	lambda1 := lon1 * math.Pi / 180
	lambda2 := lon2 * math.Pi / 180
	y := math.Sin(lambda2-lambda1) * math.Cos(phi2)
	x := math.Cos(phi1)*math.Sin(phi2) - math.Sin(phi1)*math.Cos(phi2)*math.Cos(lambda2-lambda1)
	theta := math.Atan2(y, x)
	return math.Mod(theta*180/math.Pi+360, 360)
}

func bearingEmojiKey(brng float64) string {
	switch {
	case brng < 22.5:
		return "north"
	case brng < 67.5:
		return "northeast"
	case brng < 112.5:
		return "east"
	case brng < 157.5:
		return "southeast"
	case brng < 202.5:
		return "south"
	case brng < 247.5:
		return "southwest"
	case brng < 292.5:
		return "west"
	case brng < 337.5:
		return "northwest"
	default:
		return "north"
	}
}

func countryCodeEmoji(code string) string {
	clean := strings.TrimSpace(strings.ToUpper(code))
	if len(clean) != 2 {
		return ""
	}
	first := rune(clean[0])
	second := rune(clean[1])
	if first < 'A' || first > 'Z' || second < 'A' || second > 'Z' {
		return ""
	}
	runes := []rune{
		0x1F1E6 + (first - 'A'),
		0x1F1E6 + (second - 'A'),
	}
	return string(runes)
}

func numberFromAnyOrDefault(value any, fallback int) int {
	if n, ok := numberFromAny(value); ok {
		return n
	}
	return fallback
}

func numberFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed), true
		}
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return int(parsed), true
		}
	case []byte:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(string(v)), 64); err == nil {
			return int(parsed), true
		}
	}
	return 0, false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func platformFromType(targetType string) string {
	if strings.HasPrefix(targetType, "telegram") {
		return "telegram"
	}
	if targetType == "webhook" {
		return "discord"
	}
	return "discord"
}

func selectTemplate(p *Processor, target alertTarget, hook *Hook) string {
	if p == nil {
		return ""
	}
	raw := selectTemplatePayload(p, target, hook)
	if raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	case map[string]any:
		if content, ok := v["content"].(string); ok {
			return content
		}
	}
	return ""
}

func selectTemplatePayload(p *Processor, target alertTarget, hook *Hook) any {
	if p == nil {
		return nil
	}
	templates := p.getTemplates()
	for _, templateType := range templateTypeCandidates(hook) {
		// 1) Exact id match with language preference.
		for _, tpl := range templates {
			if tpl.Hidden || tpl.Platform != target.Platform || tpl.Type != templateType {
				continue
			}
			if target.Language != "" && tpl.Language != nil && *tpl.Language != target.Language {
				continue
			}
			if target.Template != "" && !strings.EqualFold(strings.TrimSpace(fmt.Sprintf("%v", tpl.ID)), strings.TrimSpace(target.Template)) {
				continue
			}
			if target.Template != "" {
				return tpl.Template
			}
		}
		// 2) Default template for this type/platform/language.
		for _, tpl := range templates {
			if tpl.Hidden || tpl.Platform != target.Platform || tpl.Type != templateType {
				continue
			}
			if target.Language != "" && tpl.Language != nil && *tpl.Language != target.Language {
				continue
			}
			if tpl.Default {
				return tpl.Template
			}
		}
		// 3) Any default template for this type/platform.
		for _, tpl := range templates {
			if tpl.Hidden || tpl.Platform != target.Platform || tpl.Type != templateType {
				continue
			}
			if tpl.Default {
				return tpl.Template
			}
		}
		// 4) Last-resort first matching template for this type/platform.
		for _, tpl := range templates {
			if tpl.Hidden || tpl.Platform != target.Platform || tpl.Type != templateType {
				continue
			}
			if target.Language != "" && tpl.Language != nil && *tpl.Language != target.Language {
				continue
			}
			return tpl.Template
		}
		for _, tpl := range templates {
			if tpl.Hidden || tpl.Platform != target.Platform || tpl.Type != templateType {
				continue
			}
			return tpl.Template
		}
	}
	return nil
}

func templateTypeCandidates(hook *Hook) []string {
	primary := templateTypeForHook(hook)
	if hook == nil {
		return []string{primary}
	}
	// Support both historical and current template keys for fort updates.
	if hook.Type == "fort_update" {
		if primary == "fort-update" {
			return []string{"fort-update", "fort"}
		}
		return []string{primary, "fort-update"}
	}
	return []string{primary}
}

func templateTypeForHook(hook *Hook) string {
	switch hook.Type {
	case "weather":
		if getBool(hook.Message["_weatherChange"]) || getBool(hook.Message["weather_change"]) || getBool(hook.Message["weatherChange"]) {
			return "weatherchange"
		}
		return "weather"
	case "max_battle":
		return "maxbattle"
	case "pokemon":
		if getBool(hook.Message["_monsterChange"]) || getBool(hook.Message["monster_change"]) || getBool(hook.Message["monsterChange"]) || getBool(hook.Message["pokemon_change"]) || getBool(hook.Message["pokemonChange"]) {
			if computeIV(hook) < 0 {
				return "monsterchangeNoIv"
			}
			return "monsterchange"
		}
		if computeIV(hook) < 0 {
			return "monsterNoIv"
		}
		return "monster"
	case "gym_details":
		return "gym"
	case "fort_update":
		return "fort-update"
	default:
		return hook.Type
	}
}
