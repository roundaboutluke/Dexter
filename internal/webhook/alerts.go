package webhook

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang/geo/s2"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/digest"
	"poraclego/internal/dispatch"
	"poraclego/internal/geo"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
	"poraclego/internal/render"
	"poraclego/internal/stats"
	"poraclego/internal/tileserver"
	"poraclego/internal/uicons"
)

type alertTarget struct {
	ID       string
	Type     string
	Name     string
	Language string
	Lat      float64
	Lon      float64
	Areas    []string
	Profile  int
	Template string
	Platform string
}

type alertMatch struct {
	Target alertTarget
	Row    map[string]any
}

func (p *Processor) matchTargets(hook *Hook) ([]alertMatch, error) {
	if testTarget, ok := hook.Message["poracleTest"].(map[string]any); ok {
		target := alertTarget{
			ID:       getString(testTarget["id"]),
			Type:     getString(testTarget["type"]),
			Name:     getString(testTarget["name"]),
			Language: getString(testTarget["language"]),
			Template: getString(testTarget["template"]),
			Lat:      getFloat(testTarget["latitude"]),
			Lon:      getFloat(testTarget["longitude"]),
		}
		if target.Language == "" {
			target.Language = getStringFromConfig(p.cfg, "general.locale", "en")
		}
		if target.Template == "" {
			target.Template = getStringFromConfig(p.cfg, "general.defaultTemplateName", "1")
		}
		target.Platform = platformFromType(target.Type)
		return []alertMatch{{Target: target, Row: map[string]any{}}}, nil
	}
	table := trackingTable(hook.Type)
	if table == "" {
		return nil, nil
	}
	var rows []map[string]any
	if hook.Type == "pokemon" && p.monsterCache != nil {
		rows = p.monsterCache.Rows()
		if len(rows) == 0 {
			if err := p.monsterCache.Refresh(p.cfg, p.query); err == nil {
				rows = p.monsterCache.Rows()
			}
		}
	}
	var err error
	if len(rows) == 0 {
		rows, err = p.query.SelectAllQuery(table, map[string]any{})
	}
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	humans, profiles, err := p.loadHumansForRows(rows)
	if err != nil {
		return nil, err
	}
	categoryKey := alertCategoryKey(hook.Type)
	pvpSec := pvpSecurityEnabled(p.cfg)
	blockedByID := map[string]map[string]bool{}
	hasSchedules := map[string]bool{}
	for _, row := range profiles {
		id := getString(row["id"])
		if id == "" {
			continue
		}
		raw := strings.TrimSpace(fmt.Sprintf("%v", row["active_hours"]))
		if len(raw) > 5 {
			hasSchedules[id] = true
		}
	}
	var questRewardsNoAR map[string]any
	var questRewardsAR map[string]any
	if hook != nil && hook.Type == "quest" {
		questRewardsNoAR = questRewardData(p, hook)
		questRewardsAR = questRewardDataAR(p, hook)
	}

	targets := []alertMatch{}
	for _, row := range rows {
		questMatchNoAR := false
		questMatchAR := false
		if hook.Type == "quest" {
			if ok, noAR, ar := matchQuestWithVariants(hook, row, questRewardsNoAR, questRewardsAR); ok {
				questMatchNoAR = noAR
				questMatchAR = ar
				row["questMatchNoAR"] = boolToInt(noAR)
				row["questMatchAR"] = boolToInt(ar)
			} else {
				continue
			}
		} else if hook.Type == "invasion" {
			if !matchInvasionWithData(p, hook, row) {
				continue
			}
		} else if !rowMatchesHook(p, hook, row) {
			continue
		}
		id := getString(row["id"])
		human := humans[id]
		if human == nil {
			continue
		}
		if getInt(human["enabled"]) == 0 || getInt(human["admin_disable"]) == 1 {
			continue
		}
		scheduleDisabled := getInt(human["schedule_disabled"]) == 1
		currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
		hasSchedule := hasSchedules[id]
		rowProfileNo := numberFromAnyOrDefault(row["profile_no"], 0)
		blocked := blockedByID[id]
		if blocked == nil {
			blocked = parseBlockedAlerts(human["blocked_alerts"])
			blockedByID[id] = blocked
		}
		if categoryKey != "" && blocked[categoryKey] {
			continue
		}
		if (hook.Type == "raid" || hook.Type == "egg" || hook.Type == "gym" || hook.Type == "gym_details") && getString(row["gym_id"]) != "" {
			if blocked["specificgym"] {
				continue
			}
		}
		if hook.Type == "max_battle" {
			rowStationID := getString(row["station_id"])
			if rowStationID != "" {
				hookStationID := getString(hook.Message["id"])
				if hookStationID == "" {
					hookStationID = getString(hook.Message["stationId"])
				}
				if rowStationID == hookStationID && blocked["specificstation"] {
					continue
				}
			}
		}
		if hook.Type == "pokemon" && pvpSec && getInt(row["pvp_ranking_league"]) > 0 {
			if blocked["pvp"] {
				continue
			}
		}
		if hook.Type == "quest" && p.questDigests != nil && hasSchedule && !scheduleDisabled && rowProfileNo > 0 {
			if currentProfile == 0 || rowProfileNo != currentProfile {
				profile := profiles[profileKey(id, rowProfileNo)]
				location := resolveLocation(human, profile)
				if passesLocationFilter(p.fences, p.cfg, location, hook, row) {
					lang := getString(human["language"])
					if lang == "" {
						lang = getStringFromConfig(p.cfg, "general.locale", "en")
					}
					tr := p.i18n.Translator(lang)
					updated := questDigestTime(hook)
					cycleKey := p.questDigests.CycleKeyFor(id, updated)
					if cycleKey == "" {
						cycleKey = digest.CycleKey(updated)
					}
					stopText := questDigestStopText(hook)
					stopKey := questDigestKey(hook)
					if stopKey == "" {
						stopKey = stopText
					}
					addDigest := func(prefix string, rewardData map[string]any) {
						if rewardData == nil {
							return
						}
						reward := questRewardStringFromData(p, rewardData, tr)
						if reward == "" {
							return
						}
						rewardKey := prefix + reward
						seenKey := rewardKey
						if stopKey != "" {
							seenKey = stopKey + "|" + rewardKey
						}
						mode := "any"
						if prefix == "With AR: " {
							mode = "with"
						} else if prefix == "No AR: " {
							mode = "no"
						}
						p.questDigests.Add(id, rowProfileNo, cycleKey, seenKey, stopKey, stopText, mode, rewardKey)
						logger := logging.Get().General
						if logger != nil {
							logger.Infof("quest digest add: user=%s profile=%d cycle=%s reward=%s stop=%s", id, rowProfileNo, cycleKey, rewardKey, stopText)
						}
					}
					if questMatchAR {
						addDigest("With AR: ", questRewardsAR)
					}
					if questMatchNoAR {
						addDigest("No AR: ", questRewardsNoAR)
					}
				}
			}
		}
		if currentProfile == 0 && hasSchedule {
			if scheduleDisabled {
				currentProfile = 1
			} else {
				continue
			}
		}
		profileNo := numberFromAnyOrDefault(row["profile_no"], currentProfile)
		if profileNo != currentProfile {
			continue
		}
		profile := profiles[profileKey(id, profileNo)]
		location := resolveLocation(human, profile)
		if !passesLocationFilter(p.fences, p.cfg, location, hook, row) {
			continue
		}
		templateID := getString(row["template"])
		if templateID == "" {
			templateID = getStringFromConfig(p.cfg, "general.defaultTemplateName", "1")
		}
		platform := platformFromType(getString(human["type"]))
		targets = append(targets, alertMatch{Target: alertTarget{
			ID:       getString(human["id"]),
			Type:     getString(human["type"]),
			Name:     getString(human["name"]),
			Language: getString(human["language"]),
			Lat:      location.Lat,
			Lon:      location.Lon,
			Areas:    location.Areas,
			Profile:  profileNo,
			Template: templateID,
			Platform: platform,
		}, Row: row})
	}
	return targets, nil
}

func (p *Processor) enqueue(job dispatch.MessageJob) {
	if strings.HasPrefix(job.Type, "telegram") {
		if p.telegramQueue != nil {
			p.telegramQueue.Push(job)
		}
		return
	}
	if p.discordQueue != nil {
		p.discordQueue.Push(job)
	}
}

func (p *Processor) formatMessage(hook *Hook, match alertMatch) string {
	template := selectTemplate(p, match.Target, hook)
	data := buildRenderData(p, hook, match)
	meta := renderMeta(match.Target)
	if template != "" {
		rendered, err := render.RenderHandlebars(template, data, meta)
		if err == nil && strings.TrimSpace(rendered) != "" {
			return rendered
		}
	}
	tr := defaultTranslator(p, hook)
	switch hook.Type {
	case "pokemon":
		return fmt.Sprintf("%s %s (%s)", tr.Translate("Pokemon", false), nameOrID(p, hook, "pokemon_id"), getString(hook.Message["pokemon_id"]))
	case "raid":
		return fmt.Sprintf("%s L%d %s", tr.Translate("Raid", false), getInt(hook.Message["level"]), nameOrID(p, hook, "pokemon_id"))
	case "egg":
		return fmt.Sprintf("%s L%d", tr.Translate("Egg", false), getInt(hook.Message["level"]))
	case "max_battle":
		return fmt.Sprintf("%s L%d %s", tr.Translate("Max Battle", false), getInt(hook.Message["level"]), nameOrID(p, hook, "pokemon_id"))
	case "quest":
		return fmt.Sprintf("%s %s", tr.Translate("Quest", false), questRewardSummary(hook))
	case "invasion":
		return fmt.Sprintf("%s %s", tr.Translate("Invasion", false), getString(hook.Message["grunt_type"]))
	case "lure":
		return fmt.Sprintf("%s %s", tr.Translate("Lure", false), getString(hook.Message["lure_id"]))
	case "nest":
		return fmt.Sprintf("%s %s", tr.Translate("Nest", false), nameOrID(p, hook, "pokemon_id"))
	case "gym", "gym_details":
		return fmt.Sprintf("%s %s", tr.Translate("Gym", false), getString(hook.Message["id"]))
	case "weather":
		return fmt.Sprintf("%s %s", tr.Translate("Weather", false), getString(hook.Message["condition"]))
	case "fort_update":
		return fmt.Sprintf("%s %s", tr.Translate("Fort update", false), getString(hook.Message["id"]))
	default:
		return fmt.Sprintf("%s", hook.Type)
	}
}

func (p *Processor) formatPayload(hook *Hook, match alertMatch) (map[string]any, string) {
	template := selectTemplatePayload(p, match.Target, hook)
	data := buildRenderData(p, hook, match)
	meta := renderMeta(match.Target)
	payload := map[string]any{}
	message := ""
	ping := getString(match.Row["ping"])
	if template != nil {
		rendered := renderAny(template, data, meta, p)
		if renderedMap, ok := rendered.(map[string]any); ok {
			payload = renderedMap
			embed, hasEmbed := payload["embed"].(map[string]any)
			if hasEmbed {
				payload["embeds"] = []any{embed}
				delete(payload, "embed")
			}
			if content, ok := payload["content"].(string); ok {
				message = content
			} else if hasEmbed {
				if desc, ok := embed["description"].(string); ok {
					message = desc
				}
			}
		} else if renderedString, ok := rendered.(string); ok {
			message = renderedString
		}
	}
	if ping != "" {
		if content, ok := payload["content"].(string); ok {
			payload["content"] = content + ping
		} else if len(payload) > 0 {
			payload["content"] = ping
		}
		if message != "" {
			message += ping
		} else if ping != "" {
			message = ping
		}
	}
	if message == "" {
		message = p.formatMessage(hook, match)
	}
	return payload, message
}

func renderMeta(target alertTarget) map[string]any {
	return map[string]any{
		"language": target.Language,
		"platform": target.Platform,
	}
}

func nameOrID(p *Processor, hook *Hook, key string) string {
	if p.data == nil {
		return getString(hook.Message[key])
	}
	id := getString(hook.Message[key])
	if id == "" {
		return ""
	}
	form := getInt(hook.Message["form"])
	if form == 0 {
		form = getInt(hook.Message["form_id"])
	}
	if form == 0 {
		form = getInt(hook.Message["pokemon_form"])
	}
	keyWithForm := fmt.Sprintf("%s_%d", id, form)
	if name := lookupMonsterName(p.data, keyWithForm); name != "" {
		return name
	}
	if name := lookupMonsterName(p.data, fmt.Sprintf("%s_0", id)); name != "" {
		return name
	}
	if name := lookupMonsterName(p.data, id); name != "" {
		return name
	}
	return id
}

func lookupMonsterName(data *data.GameData, key string) string {
	if data == nil || data.Monsters == nil {
		return ""
	}
	monster, ok := data.Monsters[key]
	if !ok {
		return ""
	}
	if m, ok := monster.(map[string]any); ok {
		if name, ok := m["name"].(string); ok && name != "" {
			return name
		}
	}
	return ""
}

func applyPokemonEvolutions(p *Processor, data map[string]any, pokemonID, formID int, platform string, tr *i18n.Translator) {
	if p == nil || p.data == nil || data == nil || pokemonID <= 0 {
		return
	}
	monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		return
	}

	hasEvolutions := false
	switch v := monster["evolutions"].(type) {
	case []any:
		hasEvolutions = len(v) > 0
	case []map[string]any:
		hasEvolutions = len(v) > 0
	}
	data["hasEvolutions"] = hasEvolutions

	count := 0
	evolutions := make([]map[string]any, 0)
	megaEvolutions := make([]map[string]any, 0)
	collectPokemonEvolutions(p, monster, &count, &evolutions, &megaEvolutions, platform, tr)

	data["evolutions"] = evolutions
	data["hasMegaEvolutions"] = len(megaEvolutions) > 0
	data["megaEvolutions"] = megaEvolutions
}

func collectPokemonEvolutions(p *Processor, monster map[string]any, totalCount *int, evolutions *[]map[string]any, megaEvolutions *[]map[string]any, platform string, tr *i18n.Translator) {
	if p == nil || monster == nil || totalCount == nil || evolutions == nil || megaEvolutions == nil {
		return
	}
	*totalCount++
	if *totalCount >= 10 {
		return
	}

	switch raw := monster["evolutions"].(type) {
	case []any:
		for _, entry := range raw {
			evo, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			evoID := getInt(evo["evoId"])
			formID := getInt(evo["id"])
			if evoID <= 0 {
				continue
			}
			next := lookupMonster(p.data, fmt.Sprintf("%d_%d", evoID, formID))
			if next == nil && formID != 0 {
				next = lookupMonster(p.data, fmt.Sprintf("%d_0", evoID))
			}
			if next == nil {
				continue
			}

			nameEng := getString(next["name"])
			name := translateMaybe(tr, nameEng)
			formNameEng := ""
			if form, ok := next["form"].(map[string]any); ok {
				formNameEng = getString(form["name"])
			}
			formNormalisedEng := formNameEng
			if strings.EqualFold(formNormalisedEng, "Normal") {
				formNormalisedEng = ""
			}
			formNormalised := translateMaybe(tr, formNormalisedEng)
			fullNameEng := nameEng
			if formNormalisedEng != "" {
				fullNameEng = fmt.Sprintf("%s %s", nameEng, formNormalisedEng)
			}
			fullName := name
			if formNormalised != "" {
				fullName = fmt.Sprintf("%s %s", name, formNormalised)
			}

			typeNames := monsterTypeNames(p, evoID, formID)
			translatedTypes := make([]string, 0, len(typeNames))
			typeEmojis := make([]string, 0, len(typeNames))
			for _, typeName := range typeNames {
				translatedTypes = append(translatedTypes, translateMaybe(tr, typeName))
				if _, emojiKey := typeStyle(p, typeName); emojiKey != "" {
					if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
						typeEmojis = append(typeEmojis, translateMaybe(tr, emoji))
					}
				}
			}

			item := map[string]any{
				"id":                evoID,
				"form":              formID,
				"fullName":          fullName,
				"fullNameEng":       fullNameEng,
				"formNormalised":    formNormalised,
				"formNormalisedEng": formNormalisedEng,
				"name":              name,
				"nameEng":           nameEng,
				"formNameEng":       formNameEng,
				"typeName":          strings.Join(translatedTypes, ", "),
				"typeEmoji":         strings.Join(typeEmojis, ""),
			}
			if stats, ok := next["stats"].(map[string]any); ok {
				item["baseStats"] = stats
			}
			*evolutions = append(*evolutions, item)

			if *totalCount < 10 {
				collectPokemonEvolutions(p, next, totalCount, evolutions, megaEvolutions, platform, tr)
			}
		}
	}

	switch raw := monster["tempEvolutions"].(type) {
	case []any:
		for _, entry := range raw {
			evo, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			tempID := getInt(evo["tempEvoId"])
			if tempID == 0 {
				tempID = getInt(evo["tempEvoID"])
			}
			if tempID == 0 {
				continue
			}
			nameEng := getString(monster["name"])
			name := translateMaybe(tr, nameEng)
			fullNameEng := nameEng
			fullName := name
			if format := megaNameFormat(p, tempID); format != "" {
				fullNameEng = formatTemplate(format, nameEng)
				fullName = formatTemplate(format, name)
			}

			var typesPayload []any
			if types, ok := evo["types"].([]any); ok && len(types) > 0 {
				typesPayload = types
			} else if types, ok := monster["types"].([]any); ok && len(types) > 0 {
				typesPayload = types
			}

			typeNames := make([]string, 0, len(typesPayload))
			for _, entry := range typesPayload {
				if m, ok := entry.(map[string]any); ok {
					if name := getString(m["name"]); name != "" {
						typeNames = append(typeNames, name)
					}
				}
			}
			translatedTypes := make([]string, 0, len(typeNames))
			typeEmojis := make([]string, 0, len(typeNames))
			for _, typeName := range typeNames {
				translatedTypes = append(translatedTypes, translateMaybe(tr, typeName))
				if _, emojiKey := typeStyle(p, typeName); emojiKey != "" {
					if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
						typeEmojis = append(typeEmojis, translateMaybe(tr, emoji))
					}
				}
			}

			item := map[string]any{
				"fullName":    fullName,
				"fullNameEng": fullNameEng,
				"evolution":   tempID,
				"types":       typesPayload,
				"typeName":    strings.Join(translatedTypes, ", "),
				"typeEmoji":   strings.Join(typeEmojis, ""),
			}
			if stats, ok := evo["stats"].(map[string]any); ok {
				item["baseStats"] = stats
			}
			*megaEvolutions = append(*megaEvolutions, item)
		}
	}
}

func questRewardSummary(hook *Hook) string {
	rewardType := getInt(hook.Message["reward_type"])
	reward := getInt(hook.Message["reward"])
	if rewardType == 0 {
		rewardType = getInt(hook.Message["reward_type_id"])
	}
	if reward == 0 {
		reward = getInt(hook.Message["pokemon_id"])
	}
	return fmt.Sprintf("%d:%d", rewardType, reward)
}

func questDigestTime(hook *Hook) time.Time {
	if hook == nil {
		return time.Now()
	}
	updated := getInt(hook.Message["updated"])
	if updated == 0 {
		return time.Now()
	}
	return time.Unix(int64(updated), 0)
}

func questDigestKey(hook *Hook) string {
	if hook == nil {
		return ""
	}
	if id := getString(hook.Message["pokestop_id"]); id != "" {
		return id
	}
	if id := getString(hook.Message["id"]); id != "" {
		return id
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat != 0 || lon != 0 {
		return fmt.Sprintf("%.5f,%.5f", lat, lon)
	}
	return ""
}

func questDigestStopText(hook *Hook) string {
	if hook == nil {
		return ""
	}
	name := getString(hook.Message["pokestop_name"])
	if name == "" {
		name = getString(hook.Message["name"])
	}
	return name
}

func trackingTable(hookType string) string {
	switch hookType {
	case "pokemon":
		return "monsters"
	case "raid":
		return "raid"
	case "egg":
		return "egg"
	case "max_battle":
		return "maxbattle"
	case "quest":
		return "quest"
	case "invasion":
		return "invasion"
	case "lure":
		return "lures"
	case "nest":
		return "nests"
	case "gym", "gym_details":
		return "gym"
	case "weather":
		return "weather"
	case "fort_update":
		return "forts"
	default:
		return ""
	}
}

type locationInfo struct {
	Lat          float64
	Lon          float64
	Areas        []string
	Restrictions []string
}

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
		return humans, nil, nil
	}
	profiles := map[string]map[string]any{}
	for _, row := range profileRows {
		key := profileKey(getString(row["id"]), numberFromAnyOrDefault(row["profile_no"], 1))
		profiles[key] = row
	}
	return humans, profiles, nil
}

func profileKey(id string, profileNo int) string {
	return fmt.Sprintf("%s:%d", id, profileNo)
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
	case brng < 45+22.5:
		return "northwest"
	case brng < 90+22.5:
		return "west"
	case brng < 135+22.5:
		return "southwest"
	case brng < 180+22.5:
		return "south"
	case brng < 225+22.5:
		return "southeast"
	case brng < 270+22.5:
		return "east"
	case brng < 315+22.5:
		return "northeast"
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
	for _, templateType := range templateTypeCandidates(hook) {
		// 1) Exact id match with language preference.
		for _, tpl := range p.templates {
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
		for _, tpl := range p.templates {
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
		for _, tpl := range p.templates {
			if tpl.Hidden || tpl.Platform != target.Platform || tpl.Type != templateType {
				continue
			}
			if tpl.Default {
				return tpl.Template
			}
		}
		// 4) Last-resort first matching template for this type/platform.
		for _, tpl := range p.templates {
			if tpl.Hidden || tpl.Platform != target.Platform || tpl.Type != templateType {
				continue
			}
			if target.Language != "" && tpl.Language != nil && *tpl.Language != target.Language {
				continue
			}
			return tpl.Template
		}
		for _, tpl := range p.templates {
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

func buildRenderData(p *Processor, hook *Hook, match alertMatch) map[string]any {
	data := map[string]any{}
	for key, value := range hook.Message {
		data[key] = value
	}
	if hook != nil && hook.Type == "nest" {
		if name := getString(hook.Message["name"]); name != "" {
			data["nestName"] = name
		}
	}
	if p != nil && p.cfg != nil {
		if raw, ok := p.cfg.Get("general.dtsDictionary"); ok {
			if dict, ok := raw.(map[string]any); ok {
				for key, value := range dict {
					data[key] = value
				}
			}
		}
	}
	prepareMapPosition(p, hook, data)
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	data["lat"] = fmt.Sprintf("%.6f", lat)
	data["lon"] = fmt.Sprintf("%.6f", lon)
	data["latitude"] = data["lat"]
	data["longitude"] = data["lon"]
	matchedAreas := []map[string]any{}
	matched := []string{}
	areasDisplay := []string{}
	if p != nil && p.fences != nil {
		for _, fence := range p.fences.MatchedAreas([]float64{lat, lon}) {
			display := true
			if fence.DisplayInMatch != nil {
				display = *fence.DisplayInMatch
			}
			matchedAreas = append(matchedAreas, map[string]any{
				"name":             fence.Name,
				"description":      fence.Description,
				"displayInMatches": display,
				"group":            fence.Group,
			})
			matched = append(matched, strings.ToLower(fence.Name))
			if display {
				areasDisplay = append(areasDisplay, fence.Name)
			}
		}
	}
	data["matchedAreas"] = matchedAreas
	data["matched"] = matched
	data["areas"] = strings.Join(areasDisplay, ", ")
	if mapLat := getFloat(hook.Message["map_latitude"]); mapLat != 0 {
		data["map_latitude"] = fmt.Sprintf("%.6f", mapLat)
	}
	if mapLon := getFloat(hook.Message["map_longitude"]); mapLon != 0 {
		data["map_longitude"] = fmt.Sprintf("%.6f", mapLon)
	}
	if mapZoom := getFloat(hook.Message["zoom"]); mapZoom != 0 {
		data["zoom"] = mapZoom
	}
	tr := translatorFor(p, match.Target.Language)
	pokemonName := nameOrID(p, hook, "pokemon_id")
	data["pokemon_name"] = translateMaybe(tr, pokemonName)
	data["raid_pokemon_name"] = translateMaybe(tr, pokemonName)
	gymID := getString(hook.Message["gym_id"])
	if gymID == "" && hook.Type != "max_battle" {
		gymID = getString(hook.Message["id"])
	}
	data["gym_id"] = gymID
	data["pokestop_id"] = getString(hook.Message["pokestop_id"])
	data["teamId"] = teamFromHookMessage(hook.Message)
	data["encounterId"] = getString(hook.Message["encounter_id"])
	data["googleMap"] = googleMapURL(hook)
	data["shortUrl"] = shortenURL(p, fmt.Sprintf("%v", data["googleMap"]))
	data["ex"] = getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
	data["fullNameEng"] = pokemonName
	data["fullName"] = translateMaybe(tr, pokemonName)
	data["name"] = data["fullName"]
	data["nameEng"] = pokemonName
	data["pokemonId"] = getInt(hook.Message["pokemon_id"])
	if hook.Type == "max_battle" {
		stationID := getString(hook.Message["stationId"])
		if stationID == "" {
			stationID = getString(hook.Message["id"])
		}
		if stationID != "" {
			data["stationId"] = stationID
		}
		stationName := getString(hook.Message["stationName"])
		if stationName == "" {
			stationName = getString(hook.Message["name"])
		}
		if stationName != "" {
			data["stationName"] = stationName
		}
	}
	switch hook.Type {
	case "pokemon", "raid", "egg":
		data["id"] = data["pokemonId"]
	case "weather":
		data["id"] = match.Target.ID
	}
	if p != nil && p.shinyPossible != nil && data["pokemonId"].(int) > 0 {
		formID := getInt(hook.Message["form"])
		if formID == 0 {
			formID = getInt(hook.Message["form_id"])
		}
		if formID == 0 {
			formID = getInt(hook.Message["pokemon_form"])
		}
		data["shinyPossible"] = p.shinyPossible.IsPossible(data["pokemonId"].(int), formID)
	}
	weatherID := getInt(hook.Message["weather"])
	if weatherID == 0 {
		weatherID = weatherCondition(hook.Message)
	}
	if boosted := getInt(hook.Message["boosted_weather"]); boosted > 0 {
		weatherID = boosted
	}
	if (hook.Type == "raid" || hook.Type == "egg") && weatherID == 0 && p != nil && p.weatherData != nil {
		weatherCellID := geo.WeatherCellID(lat, lon)
		if weatherCellID != "" {
			if cell := p.weatherData.WeatherInfo(weatherCellID); cell != nil {
				now := time.Now().Unix()
				currentHour := now - (now % 3600)
				if current := cell.Data[currentHour]; current > 0 {
					weatherID = current
				}
			}
		}
	}
	data["weather"] = weatherID
	if hook.Type == "pokemon" {
		trackDistanceRaw, hasTrackDistance := match.Row["distance"]
		trackDistance := getInt(trackDistanceRaw)
		data["trackDistanceM"] = trackDistance
		data["isDistanceTrack"] = trackDistance > 0
		data["isAreaTrack"] = hasTrackDistance && trackDistance == 0

		var distance any = ""
		hasUserDistance := false
		userDistanceM := 0
		bearing := ""
		bearingEmoji := ""
		if match.Target.Lat != 0 || match.Target.Lon != 0 {
			hasUserDistance = true
			userDistanceM = distanceMeters(match.Target.Lat, match.Target.Lon, lat, lon)
			distance = userDistanceM
			brng := bearingDegrees(match.Target.Lat, match.Target.Lon, lat, lon)
			bearing = fmt.Sprintf("%.0f", brng)
			if emojiKey := bearingEmojiKey(brng); emojiKey != "" {
				bearingEmoji = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, match.Target.Platform))
			}
		}
		data["distance"] = distance
		data["hasUserDistance"] = hasUserDistance
		data["userDistanceM"] = userDistanceM
		data["bearing"] = bearing
		data["bearingEmoji"] = bearingEmoji

		formID := getInt(hook.Message["form"])
		if formID == 0 {
			formID = getInt(hook.Message["form_id"])
		}
		if formID == 0 {
			formID = getInt(hook.Message["pokemon_form"])
		}
		data["formId"] = formID
		if formName := monsterFormName(p, data["pokemonId"].(int), formID); formName != "" {
			data["formNameEng"] = formName
			data["formName"] = translateMaybe(tr, formName)
			data["formname"] = data["formName"]
		}
		if gen, name, roman := monsterGeneration(p, data["pokemonId"].(int), formID); gen != "" {
			data["generation"] = gen
			data["generationNameEng"] = name
			data["generationRoman"] = roman
		}
		monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", data["pokemonId"].(int), formID))
		if monster == nil && formID != 0 {
			monster = lookupMonster(p.data, fmt.Sprintf("%d_0", data["pokemonId"].(int)))
		}
		if monster == nil {
			monster = lookupMonster(p.data, fmt.Sprintf("%d", data["pokemonId"].(int)))
		}
		if monster != nil {
			if stats, ok := monster["stats"].(map[string]any); ok {
				data["baseStats"] = stats
			}
			applyPokemonEvolutions(p, data, data["pokemonId"].(int), formID, match.Target.Platform, tr)
		}
		displayID := getInt(hook.Message["display_pokemon_id"])
		if displayID > 0 && displayID != data["pokemonId"].(int) {
			displayForm := getInt(hook.Message["display_form"])
			displayMonster := lookupMonster(p.data, fmt.Sprintf("%d_%d", displayID, displayForm))
			if displayMonster == nil && displayForm != 0 {
				displayMonster = lookupMonster(p.data, fmt.Sprintf("%d_0", displayID))
			}
			if displayMonster == nil {
				displayMonster = lookupMonster(p.data, fmt.Sprintf("%d", displayID))
			}
			if displayMonster != nil {
				if name := getString(displayMonster["name"]); name != "" {
					data["disguisePokemonNameEng"] = name
					data["disguisePokemonName"] = translateMaybe(tr, name)
				}
				if form, ok := displayMonster["form"].(map[string]any); ok {
					if name := getString(form["name"]); name != "" {
						data["disguideFormNameEng"] = name
						data["disguiseFormNameEng"] = name
						data["disguiseFormName"] = translateMaybe(tr, name)
					}
				}
			}
		}
		types := monsterTypes(p, data["pokemonId"].(int), formID)
		if len(types) > 0 {
			data["types"] = types
			data["alteringWeathers"] = alteringWeathers(p, types, weatherID)
			boostingWeathers := boostingWeathersForTypes(p, types)
			if len(boostingWeathers) > 0 {
				data["boostingWeathers"] = boostingWeathers
				nonBoosting := []int{}
				for id := 1; id <= 7; id++ {
					if !containsInt(boostingWeathers, id) {
						nonBoosting = append(nonBoosting, id)
					}
				}
				data["nonBoostingWeathers"] = nonBoosting
			}
			typeNames := monsterTypeNames(p, data["pokemonId"].(int), formID)
			if len(typeNames) > 0 {
				translated := make([]string, 0, len(typeNames))
				emojis := make([]string, 0, len(typeNames))
				for _, typeName := range typeNames {
					translated = append(translated, translateMaybe(tr, typeName))
					if _, emojiKey := typeStyle(p, typeName); emojiKey != "" {
						if emoji := lookupEmojiForPlatform(p, emojiKey, match.Target.Platform); emoji != "" {
							emojis = append(emojis, translateMaybe(tr, emoji))
						}
					}
				}
				data["emoji"] = emojis
				data["typeNameEng"] = typeNames
				data["typeName"] = strings.Join(translated, ", ")
				data["emojiString"] = strings.Join(emojis, "")
				data["typeEmoji"] = data["emojiString"]
			}
		}
		if group := rarityGroupForPokemon(p.stats, data["pokemonId"].(int)); group >= 0 {
			data["rarityGroup"] = group
			if name := rarityNameEng(p, group); name != "" {
				data["rarityNameEng"] = name
			}
		}
		if size := getInt(hook.Message["size"]); size > 0 {
			if name := sizeNameEng(p, size); name != "" {
				data["sizeNameEng"] = name
			}
		}
		if shiny := shinyStatsForPokemon(p.stats, data["pokemonId"].(int)); shiny != nil {
			data["shinyStats"] = shiny
		}
		if nameEng := getString(data["nameEng"]); nameEng != "" {
			formNameEng := getString(data["formNameEng"])
			formNormalisedEng := formNameEng
			if strings.EqualFold(formNormalisedEng, "Normal") {
				formNormalisedEng = ""
			}
			data["formNormalisedEng"] = formNormalisedEng
			data["formNormalised"] = translateMaybe(tr, formNormalisedEng)
			fullNameEng := nameEng
			if formNormalisedEng != "" {
				fullNameEng = fmt.Sprintf("%s %s", nameEng, formNormalisedEng)
			}
			data["fullNameEng"] = fullNameEng
			fullName := translateMaybe(tr, nameEng)
			if formNormalised := getString(data["formNormalised"]); formNormalised != "" {
				fullName = fmt.Sprintf("%s %s", fullName, formNormalised)
			}
			data["fullName"] = fullName
			data["name"] = translateMaybe(tr, nameEng)
		}
		data["rarityName"] = translateMaybe(tr, getString(data["rarityNameEng"]))
		data["sizeName"] = translateMaybe(tr, getString(data["sizeNameEng"]))
		if shinyPossible, ok := data["shinyPossible"].(bool); ok && shinyPossible {
			data["shinyPossibleEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, "shiny", match.Target.Platform))
		} else {
			data["shinyPossibleEmoji"] = ""
		}
	}
	if hook.Type == "raid" || hook.Type == "egg" || hook.Type == "max_battle" {
		pokemonID := getInt(hook.Message["pokemon_id"])
		if pokemonID > 0 {
			formID := getInt(hook.Message["form"])
			if formID == 0 {
				formID = getInt(hook.Message["form_id"])
			}
			if formID == 0 {
				formID = getInt(hook.Message["pokemon_form"])
			}
			data["pokemonId"] = pokemonID
			data["formId"] = formID
			if p != nil && p.shinyPossible != nil {
				data["shinyPossible"] = p.shinyPossible.IsPossible(pokemonID, formID)
			} else {
				data["shinyPossible"] = false
			}
			if shinyPossible, ok := data["shinyPossible"].(bool); ok && shinyPossible {
				data["shinyPossibleEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, "shiny", match.Target.Platform))
			} else {
				data["shinyPossibleEmoji"] = ""
			}
			if formName := monsterFormName(p, pokemonID, formID); formName != "" {
				data["formNameEng"] = formName
				data["formName"] = translateMaybe(tr, formName)
				data["formname"] = data["formNameEng"]
			}
			if gen, name, roman := monsterGeneration(p, pokemonID, formID); gen != "" {
				data["generation"] = gen
				data["generationNameEng"] = name
				data["generationRoman"] = roman
				data["generationName"] = translateMaybe(tr, name)
			}
			monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
			if monster == nil && formID != 0 {
				monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
			}
			if monster == nil {
				monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
			}
			if monster != nil {
				if stats, ok := monster["stats"].(map[string]any); ok {
					data["baseStats"] = stats
				}
				if name := getString(monster["name"]); name != "" {
					data["nameEng"] = name
				}
			}
			evolutionID := getInt(hook.Message["evolution"])
			if evolutionID == 0 {
				evolutionID = getInt(hook.Message["evolution_id"])
			}
			if evolutionID > 0 {
				data["evolution"] = evolutionID
			}
			data["evolutionNameEng"] = evolutionName(p, evolutionID)
			data["evolutionName"] = translateMaybe(tr, getString(data["evolutionNameEng"]))
			data["evolutionname"] = data["evolutionNameEng"]
			data["quickMoveId"] = getInt(hook.Message["move_1"])
			data["chargeMoveId"] = getInt(hook.Message["move_2"])
			data["move_1"] = data["quickMoveId"]
			data["move_2"] = data["chargeMoveId"]
			data["move1"] = data["quickMoveName"]
			data["move2"] = data["chargeMoveName"]

			nameEng := getString(data["nameEng"])
			formNameEng := getString(data["formNameEng"])
			formNormalisedEng := formNameEng
			if strings.EqualFold(formNormalisedEng, "Normal") {
				formNormalisedEng = ""
			}
			data["formNormalisedEng"] = formNormalisedEng
			data["formNormalised"] = translateMaybe(tr, formNormalisedEng)
			fullNameEng := nameEng
			if formNormalisedEng != "" {
				fullNameEng = fmt.Sprintf("%s %s", nameEng, formNormalisedEng)
			}
			fullName := translateMaybe(tr, nameEng)
			if formNormalised := getString(data["formNormalised"]); formNormalised != "" {
				fullName = fmt.Sprintf("%s %s", fullName, formNormalised)
			}
			if evolutionID > 0 {
				if format := megaNameFormat(p, evolutionID); format != "" {
					fullNameEng = formatTemplate(format, nameEng)
					fullName = formatTemplate(format, translateMaybe(tr, nameEng))
					data["megaName"] = fullName
				}
			}
			data["fullNameEng"] = fullNameEng
			data["fullName"] = fullName
			data["name"] = translateMaybe(tr, nameEng)
			data["pokemonName"] = data["fullName"]

			types := monsterTypes(p, pokemonID, formID)
			typeNames := monsterTypeNames(p, pokemonID, formID)
			if len(types) > 0 {
				data["types"] = types
			}
			if len(typeNames) > 0 {
				translated := make([]string, 0, len(typeNames))
				emojis := make([]string, 0, len(typeNames))
				for _, typeName := range typeNames {
					translated = append(translated, translateMaybe(tr, typeName))
					if _, emojiKey := typeStyle(p, typeName); emojiKey != "" {
						if emoji := lookupEmojiForPlatform(p, emojiKey, match.Target.Platform); emoji != "" {
							emojis = append(emojis, translateMaybe(tr, emoji))
						}
					}
				}
				data["typeNameEng"] = typeNames
				data["emoji"] = emojis
				data["typeName"] = strings.Join(translated, ", ")
				data["typeEmoji"] = strings.Join(emojis, "")
			}
			boostingWeathers := boostingWeathersForTypes(p, types)
			data["boostingWeathers"] = boostingWeathers
			weatherEmojis := []string{}
			for _, weatherID := range boostingWeathers {
				_, emojiKey := weatherEntry(p, weatherID)
				if emojiKey != "" {
					if emoji := lookupEmojiForPlatform(p, emojiKey, match.Target.Platform); emoji != "" {
						weatherEmojis = append(weatherEmojis, translateMaybe(tr, emoji))
					}
				}
			}
			data["boostingWeathersEmoji"] = strings.Join(weatherEmojis, "")
			data["boosted"] = containsInt(boostingWeathers, weatherID)
			if data["boosted"].(bool) {
				name, emojiKey := weatherEntry(p, weatherID)
				data["boostWeatherNameEng"] = name
				data["boostWeatherId"] = weatherID
				data["boostWeatherName"] = translateMaybe(tr, name)
				data["boostWeatherEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, match.Target.Platform))
			} else {
				data["boostWeatherNameEng"] = ""
				data["boostWeatherId"] = 0
				data["boostWeatherName"] = ""
				data["boostWeatherEmoji"] = ""
			}
			weaknessList, weaknessEmoji := weaknessListForTypes(p, typeNames, match.Target.Platform, tr)
			if len(weaknessList) > 0 {
				data["weaknessList"] = weaknessList
				data["weaknessEmoji"] = weaknessEmoji
			}
		}
	}
	if _, ok := data["color"]; !ok || getString(data["color"]) == "" {
		if color := pokemonTypeColor(p, hook); color != "" {
			data["color"] = color
		}
	}
	data["mapurl"] = data["googleMap"]
	data["googleMapUrl"] = data["googleMap"]
	data["appleMapUrl"] = appleMapURL(lat, lon)
	data["applemap"] = data["appleMapUrl"]
	data["wazeMapUrl"] = wazeMapURL(lat, lon)
	data["time"] = hookTime(p, hook)
	expireForTTH := hookExpiryUnix(hook)
	if hook.Type == "egg" {
		start := getInt64(hook.Message["start"])
		if start == 0 {
			start = getInt64(hook.Message["hatch_time"])
		}
		if start > 0 {
			expireForTTH = start
		}
	}
	if (hook.Type == "gym" || hook.Type == "gym_details") && expireForTTH <= 0 {
		expireForTTH = time.Now().Unix() + 3600
	}
	if expireForTTH > 0 {
		remaining := int(expireForTTH - time.Now().Unix())
		data["tthSeconds"] = remaining
		firstDateWasLater := remaining < 0
		abs := remaining
		if abs < 0 {
			abs = -abs
		}
		days := abs / 86400
		hours := (abs % 86400) / 3600
		minutes := (abs % 3600) / 60
		seconds := abs % 60
		data["tthd"] = days
		data["tthh"], data["tthm"], data["tths"] = hours, minutes, seconds
		data["tth"] = map[string]any{
			"years":             0,
			"months":            0,
			"days":              days,
			"hours":             hours,
			"minutes":           minutes,
			"seconds":           seconds,
			"firstDateWasLater": firstDateWasLater,
		}
	} else {
		data["tthd"] = 0
		data["tthh"], data["tthm"], data["tths"] = 0, 0, 0
		data["tth"] = map[string]any{
			"years":             0,
			"months":            0,
			"days":              0,
			"hours":             0,
			"minutes":           0,
			"seconds":           0,
			"firstDateWasLater": false,
		}
	}
	if hook.Type == "egg" {
		start := getInt64(hook.Message["start"])
		if start == 0 {
			start = getInt64(hook.Message["hatch_time"])
		}
		if start > 0 {
			layout := "15:04:05"
			if p != nil && p.cfg != nil {
				if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
					layout = momentFormatToGoLayout(format)
				}
			}
			data["hatchTime"] = formatUnixInHookLocation(p, hook, start, layout)
			data["hatchtime"] = data["hatchTime"]
			data["time"] = data["hatchTime"]
			remaining := int(start - time.Now().Unix())
			firstDateWasLater := remaining < 0
			abs := remaining
			if abs < 0 {
				abs = -abs
			}
			days := abs / 86400
			h := (abs % 86400) / 3600
			m := (abs % 3600) / 60
			s := abs % 60
			data["tthd"] = days
			data["tthh"], data["tthm"], data["tths"] = h, m, s
			data["tth"] = map[string]any{"years": 0, "months": 0, "days": days, "hours": h, "minutes": m, "seconds": s, "firstDateWasLater": firstDateWasLater}
			data["tthSeconds"] = remaining
		}
	}
	if value := getInt64(hook.Message["disappear_time"]); value > 0 {
		data["disappear_time"] = value
	}
	if p != nil && p.cfg != nil {
		data["reactMapUrl"] = reactMapURL(p.cfg, hook)
		data["diademUrl"] = diademURL(p.cfg, hook)
		data["nightTime"] = false
		data["dawnTime"] = false
		data["duskTime"] = false
		data["style"] = getStringFromConfig(p.cfg, "geocoding.dayStyle", "klokantech-basic")
	}
	applyNightTime(p, hook, data)
	if p != nil && p.geocoder != nil {
		data["intersection"] = p.geocoder.Intersection(lat, lon)
		if details := p.geocoder.ReverseDetails(lat, lon); details != nil {
			data["address"] = details.FormattedAddress
			data["addr"] = details.FormattedAddress
			data["formattedAddress"] = details.FormattedAddress
			data["streetName"] = details.StreetName
			data["streetNumber"] = details.StreetNumber
			data["city"] = details.City
			data["country"] = details.Country
			data["state"] = details.State
			data["zipcode"] = details.Zipcode
			data["countryCode"] = details.CountryCode
			data["neighbourhood"] = details.Neighbourhood
			data["suburb"] = details.Suburb
			data["town"] = details.Town
			data["village"] = details.Village
			data["shop"] = details.Shop
			data["flag"] = countryCodeEmoji(details.CountryCode)
		} else {
			address := p.geocoder.Reverse(lat, lon)
			data["address"] = address
			data["addr"] = data["address"]
			data["formattedAddress"] = address
		}
	}
	if _, ok := data["flag"]; !ok {
		data["flag"] = ""
	}
	if getString(data["addr"]) == "" {
		data["addr"] = "Unknown"
	}
	if getString(data["address"]) == "" {
		data["address"] = getString(data["addr"])
	}
	if _, ok := data["formattedAddress"]; !ok {
		data["formattedAddress"] = getString(data["address"])
	}
	if p != nil && p.cfg != nil {
		if format := getStringFromConfig(p.cfg, "locale.addressFormat", ""); format != "" {
			if rendered, err := render.RenderHandlebars(format, data, nil); err == nil && strings.TrimSpace(rendered) != "" {
				data["addr"] = rendered
			}
		}
	}
	if hook.Type == "weather" && p != nil && p.weather != nil {
		data["weather_summary"] = p.weather.Summary(lat, lon)
	}
	encountered := hasNumeric(hook.Message["individual_attack"]) && hasNumeric(hook.Message["individual_defense"]) && hasNumeric(hook.Message["individual_stamina"])
	ivString := "-1"
	if encountered {
		if ivValue := computeIV(hook); ivValue >= 0 {
			ivString = fmt.Sprintf("%.2f", ivValue)
		}
	}
	data["iv"] = ivString
	ivPercent := normalizeIV(hook, ivString)
	data["ivPercent"] = ivPercent
	if ivPercent != "-1" {
		if ivInt, err := strconv.Atoi(ivPercent); err == nil {
			data["ivPercent"] = ivInt
		}
	}
	if encountered {
		data["atk"] = getInt(hook.Message["individual_attack"])
		data["def"] = getInt(hook.Message["individual_defense"])
		data["sta"] = getInt(hook.Message["individual_stamina"])
		data["cp"] = getInt(hook.Message["cp"])
		data["quickMoveId"] = getInt(hook.Message["move_1"])
		data["chargeMoveId"] = getInt(hook.Message["move_2"])
		if level := getInt(hook.Message["pokemon_level"]); level > 0 {
			data["level"] = level
		} else {
			data["level"] = 0
		}
	} else {
		data["atk"] = 0
		data["def"] = 0
		data["sta"] = 0
		data["cp"] = 0
		data["quickMoveId"] = 0
		data["chargeMoveId"] = 0
		data["level"] = 0
	}
	data["ivColor"] = ivColor(data["iv"])
	data["verified"] = getBool(hook.Message["verified"]) || getBool(hook.Message["disappear_time_verified"]) || getBool(hook.Message["confirmed"])
	data["disappear_time_verified"] = data["verified"]
	data["confirmed"] = data["verified"]
	data["confirmedTime"] = data["verified"]
	data["imgUrl"] = selectImageURL(p, hook)
	if getString(data["imgUrl"]) == "" && p != nil && p.cfg != nil {
		data["imgUrl"] = getStringFromConfig(p.cfg, "fallbacks.imgUrl", "")
	}
	data["imgUrlAlt"] = selectImageURLAlt(p, hook)
	data["stickerUrl"] = selectStickerURL(p, hook)
	data["quickMoveName"] = translateMaybe(tr, moveName(p, getInt(hook.Message["move_1"])))
	data["chargeMoveName"] = translateMaybe(tr, moveName(p, getInt(hook.Message["move_2"])))
	data["quickMoveNameEng"] = moveName(p, getInt(hook.Message["move_1"]))
	data["chargeMoveNameEng"] = moveName(p, getInt(hook.Message["move_2"]))
	data["quickMoveEmoji"] = moveEmoji(p, getInt(hook.Message["move_1"]), match.Target.Platform, tr)
	data["chargeMoveEmoji"] = moveEmoji(p, getInt(hook.Message["move_2"]), match.Target.Platform, tr)
	data["quickMove"] = data["quickMoveName"]
	data["chargeMove"] = data["chargeMoveName"]
	data["move1emoji"] = data["quickMoveEmoji"]
	data["move2emoji"] = data["chargeMoveEmoji"]
	data["individual_attack"] = data["atk"]
	data["individual_defense"] = data["def"]
	data["individual_stamina"] = data["sta"]
	data["pokemon_level"] = data["level"]
	data["move_1"] = data["quickMoveId"]
	data["move_2"] = data["chargeMoveId"]
	if height := getFloat(hook.Message["height"]); height > 0 && encountered {
		data["height"] = fmt.Sprintf("%.2f", height)
	} else {
		data["height"] = "0"
	}
	if weight := getFloat(hook.Message["weight"]); weight > 0 && encountered {
		data["weight"] = fmt.Sprintf("%.2f", weight)
	} else {
		data["weight"] = "0"
	}
	data["size"] = getInt(hook.Message["size"])
	if encountered {
		if baseCatch := getFloat(hook.Message["base_catch"]); baseCatch > 0 {
			data["capture_1"] = baseCatch
			data["catchBase"] = fmt.Sprintf("%.2f", baseCatch*100)
		} else {
			data["catchBase"] = "0"
		}
		if greatCatch := getFloat(hook.Message["great_catch"]); greatCatch > 0 {
			data["capture_2"] = greatCatch
			data["catchGreat"] = fmt.Sprintf("%.2f", greatCatch*100)
		} else {
			data["catchGreat"] = "0"
		}
		if ultraCatch := getFloat(hook.Message["ultra_catch"]); ultraCatch > 0 {
			data["capture_3"] = ultraCatch
			data["catchUltra"] = fmt.Sprintf("%.2f", ultraCatch*100)
		} else {
			data["catchUltra"] = "0"
		}
	} else {
		data["catchBase"] = "0"
		data["catchGreat"] = "0"
		data["catchUltra"] = "0"
	}
	data["gymName"] = gymName(hook)
	data["pokestopName"] = getString(hook.Message["pokestop_name"])
	if hook.Type == "gym" || hook.Type == "gym_details" {
		if name := getString(data["gymName"]); name != "" {
			data["name"] = name
		}
	}
	if hook.Type == "quest" || hook.Type == "lure" || hook.Type == "invasion" || hook.Type == "pokestop" {
		if name := getString(data["pokestopName"]); name != "" {
			data["name"] = name
		}
	}
	if hook.Type == "quest" || hook.Type == "lure" || hook.Type == "pokestop" {
		pokestopURL := getString(hook.Message["pokestop_url"])
		if pokestopURL == "" {
			pokestopURL = getString(hook.Message["url"])
		}
		if pokestopURL == "" && p != nil && p.cfg != nil {
			pokestopURL = getStringFromConfig(p.cfg, "fallbacks.pokestopUrl", "")
		}
		if pokestopURL != "" {
			data["pokestopUrl"] = pokestopURL
			data["pokestop_url"] = pokestopURL
			data["url"] = pokestopURL
		}
	}
	if hook.Type == "pokemon" && getString(data["pokestopName"]) == "" && p != nil && p.cfg != nil {
		if enabled, _ := p.cfg.GetBool("general.populatePokestopName"); enabled && p.scanner != nil {
			stopID := getString(hook.Message["pokestop_id"])
			if stopID != "" {
				if name, err := p.scanner.GetPokestopName(stopID); err == nil && name != "" {
					data["pokestopName"] = name
				}
			}
		}
	}
	data["teamName"], data["teamColor"] = teamInfo(teamFromHookMessage(hook.Message))
	data["slotsAvailable"] = getInt(hook.Message["slots_available"])
	data["previousControlName"], _ = teamInfo(getInt(hook.Message["old_team_id"]))
	data["gymColor"] = data["teamColor"]
	level := getInt(hook.Message["level"])
	if level == 0 {
		level = getInt(hook.Message["raid_level"])
	}
	if level > 0 {
		data["level"] = level
	}
	data["levelName"] = fmt.Sprintf("Level %d", level)
	if hook.Type == "raid" || hook.Type == "egg" || hook.Type == "gym" || hook.Type == "gym_details" {
		teamID := teamFromHookMessage(hook.Message)
		data["team_id"] = teamID
		if getString(data["gym_name"]) == "" {
			data["gym_name"] = getString(data["gymName"])
		}
		teamNameEng, teamEmojiKey, teamColor := teamDetails(p, teamID)
		if hook.Type == "egg" && teamID == 0 {
			teamNameEng = "Harmony"
		}
		if teamNameEng != "" {
			data["teamNameEng"] = teamNameEng
			data["teamName"] = translateMaybe(tr, teamNameEng)
		}
		if teamEmojiKey != "" {
			data["teamEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, teamEmojiKey, match.Target.Platform))
		}
		if teamColor != 0 {
			data["gymColor"] = teamColor
		}
		if hook.Type == "raid" || hook.Type == "egg" {
			data["color"] = data["gymColor"]
		}
		if hook.Type == "gym" || hook.Type == "gym_details" {
			data["gymId"] = getString(hook.Message["id"])
			if data["gymId"] == "" {
				data["gymId"] = getString(hook.Message["gym_id"])
			}
			data["oldTeamId"] = getInt(hook.Message["old_team_id"])
			data["previousControlId"] = getInt(hook.Message["last_owner_id"])
			data["oldSlotsAvailable"] = getInt(hook.Message["old_slots_available"])
			data["trainerCount"] = 6 - getInt(hook.Message["slots_available"])
			data["oldTrainerCount"] = 6 - getInt(hook.Message["old_slots_available"])
			data["inBattle"] = gymInBattle(hook.Message)
			oldTeamNameEng, _, _ := teamDetails(p, getInt(data["oldTeamId"]))
			prevTeamNameEng, prevTeamEmojiKey, _ := teamDetails(p, getInt(data["previousControlId"]))
			data["teamNameEng"] = teamNameEng
			data["teamName"] = translateMaybe(tr, teamNameEng)
			data["teamEmojiEng"] = lookupEmojiForPlatform(p, teamEmojiKey, match.Target.Platform)
			data["teamEmoji"] = translateMaybe(tr, getString(data["teamEmojiEng"]))
			data["oldTeamNameEng"] = oldTeamNameEng
			data["oldTeamName"] = translateMaybe(tr, oldTeamNameEng)
			data["previousControlNameEng"] = prevTeamNameEng
			data["previousControlName"] = translateMaybe(tr, prevTeamNameEng)
			data["previousControlTeamEmojiEng"] = lookupEmojiForPlatform(p, prevTeamEmojiKey, match.Target.Platform)
			data["previousControlTeamEmoji"] = translateMaybe(tr, getString(data["previousControlTeamEmojiEng"]))
			data["color"] = data["gymColor"]
			if loc := hookLocation(p, hook); loc != nil {
				layout := "15:04:05"
				if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
					layout = momentFormatToGoLayout(format)
				}
				data["conqueredTime"] = time.Now().In(loc).Format(layout)
			} else {
				data["conqueredTime"] = time.Now().Format("15:04:05")
			}
			if getInt(data["tthSeconds"]) == 0 {
				data["tthh"] = 1
				data["tthm"] = 0
				data["tths"] = 0
				data["tth"] = map[string]any{"hours": 1, "minutes": 0, "seconds": 0}
				data["tthSeconds"] = 3600
			}
		}
		if level > 0 {
			levelNameEng := raidLevelName(p, level)
			data["levelNameEng"] = levelNameEng
			data["levelName"] = translateMaybe(tr, levelNameEng)
		}
	}
	if hook.Type == "max_battle" {
		level := getInt(hook.Message["battle_level"])
		if level == 0 {
			level = getInt(hook.Message["level"])
		}
		if level > 0 {
			levelNameEng := maxbattleLevelName(p, level)
			data["levelNameEng"] = levelNameEng
			data["levelName"] = translateMaybe(tr, levelNameEng)
		}
	}
	data["weatherName"], data["weatherEmoji"] = weatherInfo(p, weatherID, match.Target.Platform, tr)
	data["boosted"] = weatherID > 0
	if weatherID > 0 {
		data["boostWeatherId"] = weatherID
		data["boostWeatherName"] = data["weatherName"]
		data["boostWeatherEmoji"] = data["weatherEmoji"]
		data["boost"] = data["boostWeatherName"]
		data["boostemoji"] = data["boostWeatherEmoji"]
	} else {
		data["boostWeatherId"] = ""
		data["boostWeatherName"] = ""
		data["boostWeatherEmoji"] = ""
		data["boost"] = ""
		data["boostemoji"] = ""
	}
	data["oldWeatherName"] = ""
	data["oldWeatherEmoji"] = ""
	if hook.Type == "weather" {
		// PoracleJS exposes `condition` (usually sourced from `gameplay_condition`) for templates.
		data["condition"] = weatherID
		cellID := getString(hook.Message["s2_cell_id"])
		if cellID == "" {
			cellID = geo.WeatherCellID(lat, lon)
		}
		if cellID != "" {
			data["weatherCellId"] = cellID
		}
		data["weatherId"] = weatherID
		data["weatherNameEng"] = ""
		data["weatherEmojiEng"] = ""
		if weatherID > 0 {
			nameEng, emojiKey := weatherEntry(p, weatherID)
			data["weatherNameEng"] = nameEng
			data["weatherName"] = translateMaybe(tr, nameEng)
			if emojiKey != "" {
				data["weatherEmojiEng"] = lookupEmojiForPlatform(p, emojiKey, match.Target.Platform)
				data["weatherEmoji"] = translateMaybe(tr, getString(data["weatherEmojiEng"]))
			}
		}
		oldWeather := 0
		if p != nil && p.weatherData != nil && cellID != "" {
			timestamp := getInt64(hook.Message["time_changed"])
			if timestamp == 0 {
				timestamp = getInt64(hook.Message["updated"])
			}
			if timestamp == 0 {
				timestamp = time.Now().Unix()
			}
			updateHour := timestamp - (timestamp % 3600)
			prevHour := updateHour - 3600
			if cell := p.weatherData.WeatherInfo(cellID); cell != nil {
				oldWeather = cell.Data[prevHour]
			}
		}
		if oldWeather > 0 {
			data["oldWeatherId"] = oldWeather
			oldNameEng, oldEmojiKey := weatherEntry(p, oldWeather)
			data["oldWeatherNameEng"] = oldNameEng
			data["oldWeatherName"] = translateMaybe(tr, oldNameEng)
			if oldEmojiKey != "" {
				data["oldWeatherEmojiEng"] = lookupEmojiForPlatform(p, oldEmojiKey, match.Target.Platform)
				data["oldWeatherEmoji"] = translateMaybe(tr, getString(data["oldWeatherEmojiEng"]))
			}
		} else {
			data["oldWeatherId"] = ""
			data["oldWeatherNameEng"] = ""
			data["oldWeatherEmojiEng"] = ""
		}
		data["weather"] = data["weatherName"]
		data["weatheremoji"] = data["weatherEmoji"]
		data["oldweather"] = data["oldWeatherName"]
		data["oldweatheremoji"] = data["oldWeatherEmoji"]
		if p != nil && p.cfg != nil && p.weatherData != nil {
			showAltered := getBoolFromConfig(p.cfg, "weather.showAlteredPokemon", false)
			if showAltered && weatherID > 0 {
				maxCount := getIntFromConfig(p.cfg, "weather.showAlteredPokemonMaxCount", 0)
				active := p.weatherData.ActivePokemons(cellID, match.Target.ID, weatherID, maxCount)
				base := imageBaseURL(p.cfg, "pokemon", "general.images", "general.imgUrl")
				var client *uicons.Client
				if base != "" {
					client = uiconsClient(base, "png")
				}
				activeViews := make([]map[string]any, 0, len(active))
				for _, mon := range active {
					formNormalisedEng := mon.FormName
					if strings.EqualFold(formNormalisedEng, "Normal") {
						formNormalisedEng = ""
					}
					fullNameEng := joinNonEmpty([]string{mon.Name, formNormalisedEng})
					entry := map[string]any{
						"pokemon_id":        mon.PokemonID,
						"form":              mon.Form,
						"nameEng":           mon.Name,
						"name":              translateMaybe(tr, mon.Name),
						"formNameEng":       mon.FormName,
						"formName":          translateMaybe(tr, mon.FormName),
						"formNormalisedEng": formNormalisedEng,
						"formNormalised":    translateMaybe(tr, formNormalisedEng),
						"fullNameEng":       fullNameEng,
						"iv":                mon.IV,
						"cp":                mon.CP,
						"latitude":          mon.Latitude,
						"longitude":         mon.Longitude,
						"disappear_time":    mon.DisappearTime,
						"alteringWeathers":  mon.AlteringWeathers,
					}
					if client != nil {
						if url, ok := client.PokemonIcon(mon.PokemonID, mon.Form, 0, 0, 0, 0, false, 0); ok {
							entry["imgUrl"] = url
						}
					}
					activeViews = append(activeViews, entry)
				}
				data["activePokemons"] = activeViews
			}
		}
		if cellID != "" {
			if cellInt, err := strconv.ParseUint(cellID, 10, 64); err == nil {
				cell := s2.CellFromCellID(s2.CellID(cellInt))
				coords := make([][]float64, 0, 4)
				for i := 0; i < 4; i++ {
					ll := s2.LatLngFromPoint(cell.Vertex(i))
					coords = append(coords, []float64{ll.Lat.Degrees(), ll.Lng.Degrees()})
				}
				data["coords"] = coords
				data["cell_coords"] = coords
			}
		}
	}
	data["pokemonSpawnAvg"] = getFloat(hook.Message["average_spawns"])
	if data["pokemonSpawnAvg"] == 0 {
		data["pokemonSpawnAvg"] = getFloat(hook.Message["pokemon_spawn_avg"])
	}
	if data["pokemonSpawnAvg"] == 0 {
		data["pokemonSpawnAvg"] = getFloat(hook.Message["pokemon_avg"])
	}
	if count := getInt(hook.Message["pokemon_count"]); count > 0 {
		data["pokemonCount"] = count
	}
	if hook.Type == "raid" || hook.Type == "egg" {
		rsvps := normalizeRSVPList(hook.Message["rsvps"])
		if len(rsvps) > 0 {
			nowMs := time.Now().UnixMilli()
			out := make([]map[string]any, 0, len(rsvps))
			layout := "15:04:05"
			if p != nil && p.cfg != nil {
				if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
					layout = momentFormatToGoLayout(format)
				}
			}
			for _, entry := range rsvps {
				timeslot := getInt64(entry["timeslot"])
				if timeslot == 0 || timeslot <= nowMs {
					continue
				}
				entry["timeSlot"] = int64(math.Ceil(float64(timeslot) / 1000))
				entry["time"] = formatUnixInHookLocation(p, hook, timeslot/1000, layout)
				entry["goingCount"] = getInt(entry["going_count"])
				entry["maybeCount"] = getInt(entry["maybe_count"])
				out = append(out, entry)
			}
			data["rsvps"] = out
		}
	}
	if hook.Type == "nest" {
		pokemonID := getInt(hook.Message["pokemon_id"])
		if pokemonID > 0 {
			formID := getInt(hook.Message["form"])
			if formID == 0 {
				formID = getInt(hook.Message["form_id"])
			}
			if formID == 0 {
				formID = getInt(hook.Message["pokemon_form"])
			}
			data["pokemonId"] = pokemonID
			data["formId"] = formID
			if monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID)); monster != nil {
				if name := getString(monster["name"]); name != "" {
					data["nameEng"] = name
				}
			}
			if formName := monsterFormName(p, pokemonID, formID); formName != "" {
				data["formNameEng"] = formName
				data["formName"] = translateMaybe(tr, formName)
			}
			data["name"] = translateMaybe(tr, getString(data["nameEng"]))
			fullNameEng := getString(data["nameEng"])
			if formNameEng := getString(data["formNameEng"]); formNameEng != "" && !strings.EqualFold(formNameEng, "Normal") {
				fullNameEng = fmt.Sprintf("%s %s", fullNameEng, formNameEng)
			}
			data["fullNameEng"] = fullNameEng
			fullName := translateMaybe(tr, getString(data["nameEng"]))
			if formName := getString(data["formName"]); formName != "" && !strings.EqualFold(formName, "Normal") {
				fullName = fmt.Sprintf("%s %s", fullName, formName)
			}
			data["fullName"] = fullName
			if shinyPossible, ok := data["shinyPossible"].(bool); ok && shinyPossible {
				data["shinyPossibleEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, "shiny", match.Target.Platform))
			}
		}
	}
	if reset := getInt64(hook.Message["reset_time"]); reset > 0 {
		dateLayout := "2006-01-02"
		timeLayout := "15:04:05"
		if p != nil && p.cfg != nil {
			if format := getStringFromConfig(p.cfg, "locale.date", ""); format != "" {
				dateLayout = momentFormatToGoLayout(format)
			}
			if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
				timeLayout = momentFormatToGoLayout(format)
			}
		}
		data["resetDate"] = formatUnixInHookLocation(p, hook, reset, dateLayout)
		if hook.Type == "nest" || hook.Type == "fort_update" {
			data["resetTime"] = formatUnixInHookLocation(p, hook, reset, timeLayout)
			expire := reset + 7*24*60*60
			data["disappearDate"] = formatUnixInHookLocation(p, hook, expire, dateLayout)
		}
	}
	data["questStringEng"] = questString(p, hook, "en", nil)
	data["questString"] = questString(p, hook, match.Target.Language, tr)
	data["rewardStringEng"] = rewardString(p, hook, nil)
	data["rewardString"] = rewardString(p, hook, tr)
	if hook.Type == "quest" {
		rewardDataNoAR := questRewardData(p, hook)
		rewardDataAR := questRewardDataAR(p, hook)
		primaryRewardData := rewardDataNoAR
		if questRewardDataIsEmpty(primaryRewardData) && !questRewardDataIsEmpty(rewardDataAR) {
			primaryRewardData = rewardDataAR
		}
		data["withAR"] = getBool(hook.Message["with_ar"])
		data["rewardData"] = primaryRewardData
		data["rewardDataNoAR"] = rewardDataNoAR
		data["rewardDataAR"] = rewardDataAR
		data["rewardStringNoAREng"] = questRewardStringFromData(p, rewardDataNoAR, nil)
		data["rewardStringNoAR"] = questRewardStringFromData(p, rewardDataNoAR, tr)
		data["rewardStringAREng"] = questRewardStringFromData(p, rewardDataAR, nil)
		data["rewardStringAR"] = questRewardStringFromData(p, rewardDataAR, tr)
		data["hasQuestNoAR"] = strings.TrimSpace(getString(data["rewardStringNoAR"])) != ""
		data["hasQuestAR"] = strings.TrimSpace(getString(data["rewardStringAR"])) != ""
		applyQuestRewardDetails(p, data, primaryRewardData, match.Target.Platform, tr)
		applyQuestRewardImages(p, data, primaryRewardData)
	}
	if hook.Type == "fort_update" {
		applyFortUpdateFields(data, hook)
	}
	if hook.Type == "invasion" {
		applyInvasionData(p, hook, data, match.Target.Platform, tr)
	} else {
		data["gruntType"] = getString(hook.Message["grunt_type"])
		data["gruntTypeEmoji"] = gruntTypeEmoji(p, data["gruntType"], match.Target.Platform)
		data["gruntTypeColor"] = gruntTypeColor(data["gruntType"])
		data["gruntRewardsList"] = gruntRewardsList(p, data["gruntType"], tr)
	}
	genderValue := getInt(hook.Message["gender"])
	if hook.Type == "invasion" {
		genderValue = getInt(data["gender"])
	}
	data["genderData"] = genderData(p, genderValue, match.Target.Platform, tr)
	if gender, ok := data["genderData"].(map[string]any); ok {
		data["genderName"] = getString(gender["name"])
		data["genderEmoji"] = getString(gender["emoji"])
	}
	if genderEng := genderDataEng(p, getInt(hook.Message["gender"])); genderEng != nil {
		data["genderDataEng"] = genderEng
		if name, ok := genderEng["name"].(string); ok {
			data["genderNameEng"] = name
		}
	}
	data["lureTypeId"] = getInt(hook.Message["lure_id"])
	if data["lureTypeId"].(int) == 0 {
		data["lureTypeId"] = getInt(hook.Message["lure_type"])
	}
	lureID := 0
	if value, ok := data["lureTypeId"].(int); ok {
		lureID = value
	}
	lureName, lureEmojiKey, lureColor := lureTypeDetails(p, lureID)
	if lureName == "" {
		lureName, lureColor = lureTypeInfo(lureID)
	}
	data["lureTypeNameEng"] = lureName
	data["lureTypeName"] = translateMaybe(tr, lureName)
	data["lureType"] = data["lureTypeName"]
	data["lureTypeColor"] = lureColor
	if lureEmojiKey != "" {
		data["lureTypeEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, lureEmojiKey, match.Target.Platform))
	}
	data["gymUrl"] = getString(hook.Message["gym_url"])
	if data["gymUrl"] == "" {
		data["gymUrl"] = getString(hook.Message["url"])
	}
	if hook.Type == "raid" || hook.Type == "egg" || hook.Type == "gym" || hook.Type == "gym_details" {
		campfireGymID := any(nil)
		if hook != nil {
			campfireGymID = hook.Message["gym_id"]
		}
		campfire := campfireLink(lat, lon, campfireGymID, data["gymName"], data["gymUrl"])
		if campfire != "" {
			data["campfireLink"] = campfire
			data["campfireUrl"] = campfire
		}
	}
	if hook.Type == "pokemon" {
		filters := pvpFiltersFromRow(match.Row)
		filterByTrack := getBoolFromConfig(p.cfg, "pvp.filterByTrack", false)
		displayMaxRank := getIntFromConfig(p.cfg, "pvp.pvpDisplayMaxRank", 10)
		displayGreatMin := getIntFromConfig(p.cfg, "pvp.pvpDisplayGreatMinCP", 0)
		displayUltraMin := getIntFromConfig(p.cfg, "pvp.pvpDisplayUltraMinCP", 0)
		displayLittleMin := getIntFromConfig(p.cfg, "pvp.pvpDisplayLittleMinCP", 0)
		data["pvpDisplayMaxRank"] = displayMaxRank
		data["pvpDisplayGreatMinCP"] = displayGreatMin
		data["pvpDisplayUltraMinCP"] = displayUltraMin
		data["pvpDisplayLittleMinCP"] = displayLittleMin
		data["pvpGreat"] = pvpDisplayList(p, hook.Message["pvp_rankings_great_league"], 1500, displayMaxRank, displayGreatMin, filters, filterByTrack, tr)
		data["pvpUltra"] = pvpDisplayList(p, hook.Message["pvp_rankings_ultra_league"], 2500, displayMaxRank, displayUltraMin, filters, filterByTrack, tr)
		data["pvpLittle"] = pvpDisplayList(p, hook.Message["pvp_rankings_little_league"], 500, displayMaxRank, displayLittleMin, filters, filterByTrack, tr)
		data["pvpGreatBest"] = pvpBestInfo(data["pvpGreat"])
		data["pvpUltraBest"] = pvpBestInfo(data["pvpUltra"])
		data["pvpLittleBest"] = pvpBestInfo(data["pvpLittle"])
		data["pvpAvailable"] = data["pvpGreat"] != nil || data["pvpUltra"] != nil || data["pvpLittle"] != nil
		data["userHasPvpTracks"] = len(filters) > 0
		userRanking := getInt(match.Row["pvp_ranking_worst"])
		if userRanking == 4096 {
			userRanking = 0
		}
		data["pvpUserRanking"] = userRanking

		capsConsidered := pvpCapsFromConfig(p.cfg)
		data["pvpBestRank"] = map[string]any{}
		data["pvpEvolutionData"] = map[string]any{}
		maxRank := getIntFromConfig(p.cfg, "pvp.pvpFilterMaxRank", 4096)
		greatMin := getIntFromConfig(p.cfg, "pvp.pvpFilterGreatMinCP", 0)
		ultraMin := getIntFromConfig(p.cfg, "pvp.pvpFilterUltraMinCP", 0)
		littleMin := getIntFromConfig(p.cfg, "pvp.pvpFilterLittleMinCP", 0)
		evoEnabled, _ := p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")

		pvpBestRank := data["pvpBestRank"].(map[string]any)
		pvpEvolution := data["pvpEvolutionData"].(map[string]any)
		pokemonID := data["pokemonId"].(int)

		bestUltra, bestUltraCP := pvpRankSummary(capsConsidered, 2500, hook.Message["pvp_rankings_ultra_league"], pokemonID, evoEnabled, ultraMin, maxRank, pvpEvolution)
		bestGreat, bestGreatCP := pvpRankSummary(capsConsidered, 1500, hook.Message["pvp_rankings_great_league"], pokemonID, evoEnabled, greatMin, maxRank, pvpEvolution)
		bestLittle, bestLittleCP := pvpRankSummary(capsConsidered, 500, hook.Message["pvp_rankings_little_league"], pokemonID, evoEnabled, littleMin, maxRank, pvpEvolution)

		pvpBestRank["2500"] = bestUltra
		pvpBestRank["1500"] = bestGreat
		pvpBestRank["500"] = bestLittle
		data["bestUltraLeagueRank"] = bestUltraCP.rank
		data["bestUltraLeagueRankCP"] = bestUltraCP.cp
		data["bestGreatLeagueRank"] = bestGreatCP.rank
		data["bestGreatLeagueRankCP"] = bestGreatCP.cp
		data["bestLittleLeagueRank"] = bestLittleCP.rank
		data["bestLittleLeagueRankCP"] = bestLittleCP.cp
	}
	if hook.Type == "invasion" || hook.Type == "pokestop" {
		weatherCellID := geo.WeatherCellID(lat, lon)
		if weatherCellID != "" {
			data["weatherCellId"] = weatherCellID
			if p != nil && p.weatherData != nil {
				if cell := p.weatherData.WeatherInfo(weatherCellID); cell != nil {
					now := time.Now().Unix()
					currentHour := now - (now % 3600)
					if current := cell.Data[currentHour]; current > 0 {
						nameEng, emojiKey := weatherEntry(p, current)
						data["gameWeatherId"] = current
						data["gameWeatherNameEng"] = nameEng
						data["gameWeatherName"] = translateMaybe(tr, nameEng)
						data["gameWeatherEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, match.Target.Platform))
						data["gameweather"] = data["gameWeatherName"]
						data["gameweatheremoji"] = data["gameWeatherEmoji"]
					}
				}
			}
		}
	}
	data["now"] = time.Now()
	data["nowISO"] = time.Now().UTC().Format(time.RFC3339)
	data["weatherChange"] = ""
	data["futureEvent"] = false
	if hook.Type == "pokemon" {
		weatherCellID := getString(hook.Message["s2_cell_id"])
		if weatherCellID == "" {
			weatherCellID = geo.WeatherCellID(lat, lon)
		}
		if weatherCellID != "" {
			data["weatherCellId"] = weatherCellID
		}
		currentCellWeather := 0
		if p != nil && p.weatherData != nil && weatherCellID != "" {
			if cell := p.weatherData.WeatherInfo(weatherCellID); cell != nil {
				now := time.Now().Unix()
				currentHour := now - (now % 3600)
				currentCellWeather = cell.Data[currentHour]
			}
		}
		if currentCellWeather > 0 {
			data["gameWeatherId"] = currentCellWeather
			name, emoji := weatherInfo(p, currentCellWeather, match.Target.Platform, tr)
			data["gameWeatherName"] = name
			data["gameWeatherEmoji"] = emoji
			data["gameweather"] = name
			data["gameweatheremoji"] = emoji
		} else {
			data["gameWeatherId"] = ""
			data["gameWeatherName"] = ""
			data["gameWeatherEmoji"] = ""
			data["gameweather"] = ""
			data["gameweatheremoji"] = ""
		}

		if p != nil && p.cfg != nil && p.weatherData != nil && weatherCellID != "" {
			enabled, _ := p.cfg.GetBool("weather.enableWeatherForecast")
			if enabled {
				expire := hookExpiryUnix(hook)
				now := time.Now().Unix()
				currentHour := now - (now % 3600)
				nextHour := currentHour + 3600
				if expire > nextHour {
					cell := p.weatherData.EnsureForecast(weatherCellID, lat, lon)
					if cell != nil {
						weatherCurrent := cell.Data[currentHour]
						weatherNext := cell.Data[nextHour]
						types, _ := data["types"].([]int)
						pokemonShouldBeBoosted := weatherBoostsTypes(p, weatherCurrent, types)
						pokemonWillBeBoosted := weatherBoostsTypes(p, weatherNext, types)
						if weatherNext > 0 && ((weatherID > 0 && weatherNext != weatherID) || (weatherCurrent > 0 && weatherNext != weatherCurrent) || (pokemonShouldBeBoosted && weatherID == 0)) {
							changeTime := expire - (expire % 3600)
							layout := "15:04:05"
							if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
								layout = momentFormatToGoLayout(format)
							}
							weatherChangeTime := formatUnixInHookLocation(p, hook, changeTime, layout)
							weatherChangeTime = trimWeatherChangeTime(weatherChangeTime)
							if (weatherID > 0 && !pokemonWillBeBoosted) || (weatherID == 0 && pokemonWillBeBoosted) {
								if weatherID > 0 {
									weatherCurrent = weatherID
								}
								if pokemonShouldBeBoosted && weatherID == 0 {
									data["weatherCurrent"] = 0
								} else {
									data["weatherCurrent"] = weatherCurrent
								}
								data["weatherChangeTime"] = weatherChangeTime
								data["weatherNext"] = weatherNext
							}
						}
					}
				}
			}
		}
	}
	if hook.Type == "raid" || hook.Type == "egg" {
		weatherCellID := geo.WeatherCellID(lat, lon)
		if weatherCellID != "" {
			data["weatherCellId"] = weatherCellID
		}
		if weatherID > 0 {
			nameEng, emojiKey := weatherEntry(p, weatherID)
			data["gameWeatherId"] = weatherID
			data["gameWeatherNameEng"] = nameEng
			data["gameWeatherName"] = translateMaybe(tr, nameEng)
			data["gameWeatherEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, match.Target.Platform))
			data["gameweather"] = data["gameWeatherName"]
			data["gameweatheremoji"] = data["gameWeatherEmoji"]
		} else {
			data["gameWeatherId"] = ""
			data["gameWeatherNameEng"] = ""
			data["gameWeatherName"] = ""
			data["gameWeatherEmoji"] = ""
			data["gameweather"] = ""
			data["gameweatheremoji"] = ""
		}
		if p != nil && p.cfg != nil && p.weatherData != nil && weatherCellID != "" {
			enabled, _ := p.cfg.GetBool("weather.enableWeatherForecast")
			if enabled {
				expire := hookExpiryUnix(hook)
				now := time.Now().Unix()
				currentHour := now - (now % 3600)
				nextHour := currentHour + 3600
				if expire > nextHour {
					cell := p.weatherData.EnsureForecast(weatherCellID, lat, lon)
					if cell != nil {
						weatherCurrent := cell.Data[currentHour]
						weatherNext := cell.Data[nextHour]
						types, _ := data["types"].([]int)
						pokemonShouldBeBoosted := weatherBoostsTypes(p, weatherCurrent, types)
						pokemonWillBeBoosted := weatherBoostsTypes(p, weatherNext, types)
						if weatherNext > 0 && ((weatherID > 0 && weatherNext != weatherID) || (weatherCurrent > 0 && weatherNext != weatherCurrent) || (pokemonShouldBeBoosted && weatherID == 0)) {
							changeTime := expire - (expire % 3600)
							layout := "15:04:05"
							if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
								layout = momentFormatToGoLayout(format)
							}
							weatherChangeTime := formatUnixInHookLocation(p, hook, changeTime, layout)
							weatherChangeTime = trimWeatherChangeTime(weatherChangeTime)
							if (weatherID > 0 && !pokemonWillBeBoosted) || (weatherID == 0 && pokemonWillBeBoosted) {
								if weatherID > 0 {
									weatherCurrent = weatherID
								}
								if pokemonShouldBeBoosted && weatherID == 0 {
									data["weatherCurrent"] = 0
								} else {
									data["weatherCurrent"] = weatherCurrent
								}
								data["weatherChangeTime"] = weatherChangeTime
								data["weatherNext"] = weatherNext
							}
						}
					}
				}
			}
		}
	}
	if p != nil && p.eventParser != nil {
		start := time.Now().Unix()
		expire := hookExpiryUnix(hook)
		if expire > 0 {
			var event *EventChange
			switch hook.Type {
			case "pokemon":
				event = p.eventParser.EventChangesSpawn(start, expire, lat, lon, p.tzLocator)
			case "quest":
				event = p.eventParser.EventChangesQuest(start, expire, lat, lon, p.tzLocator)
			}
			if event != nil {
				data["futureEvent"] = true
				data["futureEventTime"] = event.Time
				data["futureEventName"] = event.Name
				data["futureEventTrigger"] = event.Reason
			}
		}
	}
	if p != nil && p.cfg != nil {
		if rdmURL := buildRdmURL(p.cfg, hook, lat, lon); rdmURL != "" {
			data["rdmUrl"] = rdmURL
		}
		if rocketMad := rocketMadURL(p.cfg, lat, lon); rocketMad != "" {
			data["rocketMadUrl"] = rocketMad
		}
	}
	if timeStr, ok := data["time"].(string); ok && timeStr != "" {
		data["disappearTime"] = timeStr
		data["distime"] = timeStr
		data["disTime"] = timeStr
	}
	data["ivcolor"] = data["ivColor"]
	if hook.Type == "pokemon" {
		seenType := getString(hook.Message["seen_type"])
		if seenType != "" {
			switch seenType {
			case "nearby_stop":
				data["seenType"] = "pokestop"
			case "nearby_cell":
				data["seenType"] = "cell"
			case "lure", "lure_wild":
				data["seenType"] = "lure"
			case "lure_encounter", "encounter", "wild":
				data["seenType"] = seenType
			}
		} else {
			stopID := getString(hook.Message["pokestop_id"])
			spawnID := getString(hook.Message["spawnpoint_id"])
			if stopID == "None" && spawnID == "None" {
				data["seenType"] = "cell"
			} else if stopID == "None" {
				if encountered {
					data["seenType"] = "encounter"
				} else {
					data["seenType"] = "wild"
				}
			}
		}
		if data["seenType"] == "cell" {
			if _, ok := data["cell_coords"]; !ok {
				cellID := s2.CellIDFromLatLng(s2.LatLngFromDegrees(lat, lon)).Parent(15)
				cell := s2.CellFromCellID(cellID)
				coords := make([][]float64, 0, 4)
				for i := 0; i < 4; i++ {
					ll := s2.LatLngFromPoint(cell.Vertex(i))
					coords = append(coords, []float64{ll.Lat.Degrees(), ll.Lng.Degrees()})
				}
				data["cell_coords"] = coords
			}
		}
		data["pvpPokemonId"] = data["pokemonId"]
		data["pvpFormId"] = data["formId"]
		weatherNext := getInt(data["weatherNext"])
		if weatherNext > 0 {
			weatherCurrent := getInt(data["weatherCurrent"])
			weatherNextName, weatherNextEmoji := weatherInfo(p, weatherNext, match.Target.Platform, tr)
			data["weatherNextName"] = weatherNextName
			data["weatherNextEmoji"] = weatherNextEmoji
			changeTime := getString(data["weatherChangeTime"])
			if weatherCurrent <= 0 {
				data["weatherCurrentName"] = translateMaybe(tr, "unknown")
				data["weatherCurrentEmoji"] = "❓"
				data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : ➡️ %s %s", translateMaybe(tr, "Possible weather change at"), changeTime, weatherNextName, weatherNextEmoji)
			} else {
				weatherCurrentName, weatherCurrentEmoji := weatherInfo(p, weatherCurrent, match.Target.Platform, tr)
				data["weatherCurrentName"] = weatherCurrentName
				data["weatherCurrentEmoji"] = weatherCurrentEmoji
				data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : %s %s ➡️ %s %s", translateMaybe(tr, "Possible weather change at"), changeTime, weatherCurrentName, weatherCurrentEmoji, weatherNextName, weatherNextEmoji)
			}
		}
	}
	if hook.Type == "raid" {
		weatherNext := getInt(data["weatherNext"])
		if weatherNext > 0 {
			weatherCurrent := getInt(data["weatherCurrent"])
			weatherNextName, weatherNextEmoji := weatherInfo(p, weatherNext, match.Target.Platform, tr)
			data["weatherNextName"] = weatherNextName
			data["weatherNextEmoji"] = weatherNextEmoji
			changeTime := getString(data["weatherChangeTime"])
			if weatherCurrent <= 0 {
				data["weatherCurrentName"] = translateMaybe(tr, "unknown")
				data["weatherCurrentEmoji"] = "❓"
				data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : ➡️ %s %s", translateMaybe(tr, "Possible weather change at"), changeTime, weatherNextName, weatherNextEmoji)
			} else {
				weatherCurrentName, weatherCurrentEmoji := weatherInfo(p, weatherCurrent, match.Target.Platform, tr)
				data["weatherCurrentName"] = weatherCurrentName
				data["weatherCurrentEmoji"] = weatherCurrentEmoji
				data["weatherChange"] = fmt.Sprintf("⚠️ %s %s : %s %s ➡️ %s %s", translateMaybe(tr, "Possible weather change at"), changeTime, weatherCurrentName, weatherCurrentEmoji, weatherNextName, weatherNextEmoji)
			}
		}
	}
	if hook.Type == "pokemon" {
		if getBool(hook.Message["_monsterChange"]) || getBool(hook.Message["monster_change"]) || getBool(hook.Message["monsterChange"]) || getBool(hook.Message["pokemon_change"]) || getBool(hook.Message["pokemonChange"]) {
			oldPokemonID := getInt(hook.Message["oldPokemonId"])
			if oldPokemonID == 0 {
				oldPokemonID = getInt(hook.Message["old_pokemon_id"])
			}
			oldFormID := getInt(hook.Message["oldFormId"])
			if oldFormID == 0 {
				oldFormID = getInt(hook.Message["old_form_id"])
			}
			if oldPokemonID > 0 {
				oldNameEng := monsterName(p, oldPokemonID)
				oldFormNameEng := monsterFormName(p, oldPokemonID, oldFormID)
				oldFullNameEng := oldNameEng
				if oldFormNameEng != "" && !strings.EqualFold(oldFormNameEng, "Normal") {
					oldFullNameEng = fmt.Sprintf("%s %s", oldNameEng, oldFormNameEng)
				}
				data["oldFullNameEng"] = oldFullNameEng
				oldFullName := translateMaybe(tr, oldNameEng)
				if oldFormNameEng != "" && !strings.EqualFold(oldFormNameEng, "Normal") {
					oldFullName = fmt.Sprintf("%s %s", oldFullName, translateMaybe(tr, oldFormNameEng))
				}
				data["oldFullName"] = oldFullName
			}
			if _, ok := data["oldCp"]; !ok {
				data["oldCp"] = getInt(hook.Message["oldCp"])
			}
			if _, ok := data["oldIv"]; !ok {
				data["oldIv"] = getFloat(hook.Message["oldIv"])
			}
			if _, ok := data["oldIvKnown"]; !ok {
				data["oldIvKnown"] = getFloat(data["oldIv"]) >= 0
			}
		}
	}
	if p != nil && p.cfg != nil {
		staticMap := staticMapURL(p, hook, data)
		if staticMap == "" {
			staticMap = getStringFromConfig(p.cfg, "fallbacks.staticMap", "")
		}
		if staticMap != "" {
			data["staticMap"] = staticMap
			data["staticmap"] = staticMap
		}
	}
	return data
}

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
	if p == nil || p.data == nil || p.data.Monsters == nil || p.data.UtilData == nil {
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
	monster := lookupMonster(p.data, key)
	if monster == nil && form != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
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
	utilTypes, ok := p.data.UtilData["types"].(map[string]any)
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

func googleMapURL(hook *Hook) string {
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 && lon == 0 {
		return ""
	}
	return fmt.Sprintf("https://maps.google.com/maps?q=%f,%f", lat, lon)
}

func reactMapURL(cfg *config.Config, hook *Hook) string {
	if cfg == nil || hook == nil {
		return ""
	}
	base := getStringFromConfig(cfg, "general.reactMapURL", "")
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	switch hook.Type {
	case "pokemon":
		if encounter := getString(hook.Message["encounter_id"]); encounter != "" {
			return base + "id/pokemon/" + encounter
		}
	case "raid", "egg", "gym", "gym_details":
		if gym := getString(hook.Message["gym_id"]); gym != "" {
			return base + "id/gyms/" + gym
		}
	case "max_battle":
		stationID := getString(hook.Message["stationId"])
		if stationID == "" {
			stationID = getString(hook.Message["id"])
		}
		if stationID != "" {
			return base + "id/stations/" + stationID + "/16"
		}
	case "quest", "invasion", "lure", "pokestop":
		if stop := getString(hook.Message["pokestop_id"]); stop != "" {
			return base + "id/pokestops/" + stop
		}
	case "nest":
		if nest := getString(hook.Message["nest_id"]); nest != "" {
			return base + "id/nests/" + nest
		}
	case "fort_update":
		fortType := getString(hook.Message["fort_type"])
		if fortType == "" {
			fortType = getString(hook.Message["type"])
		}
		if fortType != "" {
			if id := getString(hook.Message["id"]); id != "" {
				return fmt.Sprintf("%sid/%ss/%s/18", base, fortType, id)
			}
		}
	}
	return ""
}

func diademURL(cfg *config.Config, hook *Hook) string {
	if cfg == nil || hook == nil {
		return ""
	}
	base := getStringFromConfig(cfg, "general.diademURL", "")
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	switch hook.Type {
	case "pokemon":
		if encounter := getString(hook.Message["encounter_id"]); encounter != "" {
			return base + "pokemon/" + encounter
		}
	case "raid", "egg", "gym", "gym_details":
		if gym := getString(hook.Message["gym_id"]); gym != "" {
			return base + "gym/" + gym
		}
	case "max_battle":
		stationID := getString(hook.Message["stationId"])
		if stationID == "" {
			stationID = getString(hook.Message["id"])
		}
		if stationID != "" {
			return base + "station/" + stationID
		}
	case "quest", "invasion", "lure", "pokestop":
		if stop := getString(hook.Message["pokestop_id"]); stop != "" {
			return base + "pokestop/" + stop
		}
	case "nest":
		if nest := getString(hook.Message["nest_id"]); nest != "" {
			return base + "nest/" + nest
		}
	case "fort_update":
		fortType := getString(hook.Message["fort_type"])
		if fortType == "" {
			fortType = getString(hook.Message["type"])
		}
		switch fortType {
		case "gym", "pokestop", "station", "nest", "spawnpoint", "route", "tappable":
			if id := getString(hook.Message["id"]); id != "" {
				return base + fortType + "/" + id
			}
		}
	}
	return ""
}

func shortenURL(p *Processor, url string) string {
	if url == "" || p == nil || p.cfg == nil {
		return url
	}
	shortener := newShortener(p.cfg)
	if shortener == nil {
		return url
	}
	short, err := shortener.Shorten(url)
	if err != nil || short == "" {
		return url
	}
	return short
}

func getStringFromConfig(cfg *config.Config, path, fallback string) string {
	value, ok := cfg.GetString(path)
	if !ok {
		return fallback
	}
	return value
}

func getIntFromConfig(cfg *config.Config, path string, fallback int) int {
	value, ok := cfg.GetInt(path)
	if !ok {
		return fallback
	}
	return value
}

func getStringSliceFromConfig(cfg *config.Config, path string) []string {
	value, ok := cfg.GetStringSlice(path)
	if !ok {
		if single, ok := cfg.GetString(path); ok && strings.TrimSpace(single) != "" {
			return []string{strings.TrimSpace(single)}
		}
		return []string{}
	}
	return value
}

func getBoolFromConfig(cfg *config.Config, path string, fallback bool) bool {
	value, ok := cfg.GetBool(path)
	if !ok {
		return fallback
	}
	return value
}

func appleMapURL(lat, lon float64) string {
	if lat == 0 && lon == 0 {
		return ""
	}
	return fmt.Sprintf("https://maps.apple.com/place?coordinate=%f,%f", lat, lon)
}

func wazeMapURL(lat, lon float64) string {
	if lat == 0 && lon == 0 {
		return ""
	}
	return fmt.Sprintf("https://www.waze.com/ul?ll=%f,%f&navigate=yes&zoom=17", lat, lon)
}

func hookTime(p *Processor, hook *Hook) string {
	expire := hookExpiryUnix(hook)
	if expire == 0 {
		return ""
	}
	layout := "15:04:05"
	if p != nil && p.cfg != nil {
		if format := getStringFromConfig(p.cfg, "locale.time", ""); format != "" {
			layout = momentFormatToGoLayout(format)
		}
	}
	return formatUnixInHookLocation(p, hook, expire, layout)
}

func formatUnixInHookLocation(p *Processor, hook *Hook, unixTime int64, layout string) string {
	if unixTime <= 0 {
		return ""
	}
	instant := time.Unix(unixTime, 0)
	if loc := hookLocation(p, hook); loc != nil {
		instant = instant.In(loc)
	}
	return instant.Format(layout)
}

func hookLocation(p *Processor, hook *Hook) *time.Location {
	if p == nil || hook == nil || p.tzLocator == nil {
		return nil
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 && lon == 0 {
		return nil
	}
	if loc, ok := p.tzLocator.Location(lat, lon); ok {
		return loc
	}
	return nil
}

func momentFormatToGoLayout(format string) string {
	format = strings.TrimSpace(format)
	if format == "" {
		return "15:04:05"
	}
	switch format {
	case "LTS":
		return "15:04:05"
	case "LT":
		return "15:04"
	}
	layout := format
	// Date tokens
	layout = strings.ReplaceAll(layout, "YYYY", "2006")
	layout = strings.ReplaceAll(layout, "YY", "06")
	layout = strings.ReplaceAll(layout, "MM", "01")
	layout = strings.ReplaceAll(layout, "M", "1")
	layout = strings.ReplaceAll(layout, "DD", "02")
	layout = strings.ReplaceAll(layout, "D", "2")
	// Time tokens
	layout = strings.ReplaceAll(layout, "HH", "15")
	layout = strings.ReplaceAll(layout, "H", "15")
	layout = strings.ReplaceAll(layout, "mm", "04")
	layout = strings.ReplaceAll(layout, "m", "04")
	layout = strings.ReplaceAll(layout, "ss", "05")
	layout = strings.ReplaceAll(layout, "s", "05")
	return layout
}

// trimWeatherChangeTime mirrors PoracleJS behavior: it always removes the last 3 characters of the
// formatted time string.
//
// Examples (with en-gb defaults):
// - LTS (HH:mm:ss) -> HH:mm
// - LT (HH:mm)     -> HH
//
// This is mainly used with customMaps like timeEmoji which are often keyed by hour.
func trimWeatherChangeTime(value string) string {
	if len(value) < 3 {
		return ""
	}
	return value[:len(value)-3]
}

func hookTTH(hook *Hook) (int, int, int) {
	expire := hookExpiryUnix(hook)
	if expire == 0 {
		return 0, 0, 0
	}
	remaining := time.Until(time.Unix(expire, 0))
	if remaining < 0 {
		return 0, 0, 0
	}
	total := int(remaining.Seconds())
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return m, s, h
}

func hookExpiryUnix(hook *Hook) int64 {
	if hook == nil || hook.Message == nil {
		return 0
	}
	keys := []string{
		"disappear_time",
		"end",
		"battle_end",
		"lure_expiration",
		"incident_expiration",
		"incident_expire_timestamp",
		"expiration",
		"reset_time",
	}
	for _, key := range keys {
		if value := getInt64(hook.Message[key]); value > 0 {
			if key == "reset_time" && (hook.Type == "nest" || hook.Type == "fort_update") {
				value += 7 * 24 * 60 * 60
			}
			return value
		}
	}
	return 0
}

func selectImageURL(p *Processor, hook *Hook) string {
	if url := getString(hook.Message["imgUrl"]); url != "" {
		return url
	}
	if p == nil || p.cfg == nil {
		return ""
	}
	base := imageBaseURL(p.cfg, hook.Type, "general.images", "general.imgUrl")
	if base != "" {
		if url := uiconsURL(base, "png", hook, shinyPossibleForHook(p, hook)); url != "" {
			return url
		}
	}
	return fallbackImageURL(p.cfg, hook.Type)
}

func selectImageURLAlt(p *Processor, hook *Hook) string {
	if url := getString(hook.Message["imgUrlAlt"]); url != "" {
		return url
	}
	if p == nil || p.cfg == nil {
		return ""
	}
	base := imageBaseURL(p.cfg, hook.Type, "general.images", "general.imgUrlAlt")
	if base != "" {
		if url := uiconsURL(base, "png", hook, shinyPossibleForHook(p, hook)); url != "" {
			return url
		}
	}
	return ""
}

func selectStickerURL(p *Processor, hook *Hook) string {
	if url := getString(hook.Message["stickerUrl"]); url != "" {
		return url
	}
	if p == nil || p.cfg == nil {
		return ""
	}
	base := imageBaseURL(p.cfg, hook.Type, "general.stickers", "general.stickerUrl")
	if base != "" {
		if url := uiconsURL(base, "webp", hook, shinyPossibleForHook(p, hook)); url != "" {
			return url
		}
	}
	return ""
}

func imageBaseURL(cfg *config.Config, hookType, mapPath, fallbackPath string) string {
	if cfg == nil {
		return ""
	}
	lookupType := hookType
	switch hookType {
	case "pokemon":
		lookupType = "monster"
	case "max_battle":
		lookupType = "monster"
	case "gym_details":
		lookupType = "gym"
	case "fort_update":
		lookupType = "fort"
	}
	if raw, ok := cfg.Get(mapPath); ok {
		if mapped, ok := raw.(map[string]any); ok {
			if value, ok := mapped[lookupType]; ok {
				if s := strings.TrimSpace(fmt.Sprintf("%v", value)); s != "" {
					return s
				}
			}
		}
	}
	value, _ := cfg.GetString(fallbackPath)
	return strings.TrimSpace(value)
}

func fallbackImageURL(cfg *config.Config, hookType string) string {
	switch hookType {
	case "weather":
		return getStringFromConfig(cfg, "fallbacks.imgUrlWeather", "")
	case "egg":
		return getStringFromConfig(cfg, "fallbacks.imgUrlEgg", "")
	case "gym", "gym_details":
		return getStringFromConfig(cfg, "fallbacks.imgUrlGym", "")
	case "max_battle":
		if station := getStringFromConfig(cfg, "fallbacks.imgUrlStation", ""); station != "" {
			return station
		}
		return getStringFromConfig(cfg, "fallbacks.imgUrl", "")
	case "lure", "quest", "invasion", "pokestop", "fort_update":
		return getStringFromConfig(cfg, "fallbacks.imgUrlPokestop", "")
	default:
		return getStringFromConfig(cfg, "fallbacks.imgUrl", "")
	}
}

func uiconsURL(baseURL, imageType string, hook *Hook, shinyPossible bool) string {
	if baseURL == "" || hook == nil {
		return ""
	}
	base := strings.TrimRight(baseURL, "/")
	if !isUiconsRepo(baseURL, imageType) {
		return legacyUiconsURL(base, imageType, hook)
	}
	client := uiconsClient(baseURL, imageType)
	switch hook.Type {
	case "pokemon", "raid", "max_battle":
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
		evolution := getInt(hook.Message["evolution"])
		if evolution == 0 {
			evolution = getInt(hook.Message["evolution_id"])
		}
		gender := getInt(hook.Message["gender"])
		costume := getInt(hook.Message["costume"])
		if costume == 0 {
			costume = getInt(hook.Message["costume_id"])
		}
		alignment := getInt(hook.Message["alignment"])
		bread := getInt(hook.Message["bread"])
		if bread == 0 {
			bread = getInt(hook.Message["battle_pokemon_bread_mode"])
		}
		shiny := shinyPossible
		if url, ok := client.PokemonIcon(pokemonID, form, evolution, gender, costume, alignment, shiny, bread); ok {
			return url
		}
		if url, ok := client.PokemonIcon(pokemonID, 0, 0, 0, 0, 0, shiny, bread); ok {
			return url
		}
		return fmt.Sprintf("%s/pokemon/0.%s", base, imageType)
	case "egg":
		level := getInt(hook.Message["level"])
		if level == 0 {
			level = getInt(hook.Message["raid_level"])
		}
		hatched := getBool(hook.Message["hatched"])
		ex := getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
		if url, ok := client.RaidEggIcon(level, hatched, ex); ok {
			return url
		}
		return fmt.Sprintf("%s/raid/egg/0.%s", base, imageType)
	case "gym", "gym_details":
		team := getInt(hook.Message["team_id"])
		if team == 0 {
			team = getInt(hook.Message["team"])
		}
		inBattle := gymInBattle(hook.Message)
		ex := getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
		if url, ok := client.GymIcon(team, 0, inBattle, ex); ok {
			return url
		}
		return fmt.Sprintf("%s/gym/0.%s", base, imageType)
	case "weather":
		weatherID := getInt(hook.Message["condition"])
		if weatherID == 0 {
			weatherID = getInt(hook.Message["weather"])
		}
		if url, ok := client.WeatherIcon(weatherID); ok {
			return url
		}
		return fmt.Sprintf("%s/weather/0.%s", base, imageType)
	case "invasion":
		displayTypeID := invasionDisplayType(hook)
		gruntTypeID := invasionGruntTypeID(hook, displayTypeID)
		if isEventInvasion(hook, displayTypeID) {
			lureID := getInt(hook.Message["lure_id"])
			if lureID == 0 {
				lureID = getInt(hook.Message["lure_type"])
			}
			if url, ok := client.PokestopIcon(lureID, true, displayTypeID, false); ok {
				return url
			}
			return fmt.Sprintf("%s/pokestop/0.%s", base, imageType)
		}
		if gruntTypeID == 0 {
			gruntTypeID = invasionRawGruntType(hook)
		}
		if url, ok := client.InvasionIcon(gruntTypeID); ok {
			return url
		}
		return fmt.Sprintf("%s/invasion/0.%s", base, imageType)
	case "lure", "quest", "fort_update":
		lureID := getInt(hook.Message["lure_id"])
		if lureID == 0 {
			lureID = getInt(hook.Message["lure_type"])
		}
		invasionActive := getInt64(hook.Message["incident_expiration"]) > 0 || getInt64(hook.Message["incident_expire_timestamp"]) > 0
		incidentDisplay := getInt(hook.Message["display_type"])
		questActive := hook.Type == "quest"
		if url, ok := client.PokestopIcon(lureID, invasionActive, incidentDisplay, questActive); ok {
			return url
		}
		return fmt.Sprintf("%s/pokestop/0.%s", base, imageType)
	default:
		return ""
	}
}

func shinyPossibleForHook(p *Processor, hook *Hook) bool {
	if p == nil || p.cfg == nil || hook == nil {
		return false
	}
	allowShiny := getBoolFromConfig(p.cfg, "general.requestShinyImages", false)
	if !allowShiny || p.shinyPossible == nil {
		return false
	}
	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		return false
	}
	form := getInt(hook.Message["form"])
	if form == 0 {
		form = getInt(hook.Message["form_id"])
	}
	if form == 0 {
		form = getInt(hook.Message["pokemon_form"])
	}
	return p.shinyPossible.IsPossible(pokemonID, form)
}

var (
	uiconsClients   = map[string]*uicons.Client{}
	uiconsClientsMu sync.Mutex
)

func isUiconsRepo(baseURL, imageType string) bool {
	key := baseURL + "|" + imageType
	uiconsClientsMu.Lock()
	client, ok := uiconsClients[key]
	if !ok {
		client = uicons.NewClient(baseURL, imageType)
		uiconsClients[key] = client
	}
	uiconsClientsMu.Unlock()
	okRepo, _ := client.IsUiconsRepository()
	return okRepo
}

func uiconsClient(baseURL, imageType string) *uicons.Client {
	key := baseURL + "|" + imageType
	uiconsClientsMu.Lock()
	client, ok := uiconsClients[key]
	if !ok {
		client = uicons.NewClient(baseURL, imageType)
		uiconsClients[key] = client
	}
	uiconsClientsMu.Unlock()
	return client
}

func legacyUiconsURL(base, imageType string, hook *Hook) string {
	switch hook.Type {
	case "pokemon", "raid", "max_battle":
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
		evolution := getInt(hook.Message["evolution"])
		if evolution == 0 {
			evolution = getInt(hook.Message["evolution_id"])
		}
		formStr := "00"
		if form > 0 {
			formStr = strconv.Itoa(form)
		}
		filename := fmt.Sprintf("pokemon_icon_%03d_%s", pokemonID, formStr)
		if evolution > 0 {
			filename = fmt.Sprintf("%s_%d", filename, evolution)
		}
		return fmt.Sprintf("%s/%s.%s", base, filename, imageType)
	case "egg":
		level := getInt(hook.Message["level"])
		if level == 0 {
			level = getInt(hook.Message["raid_level"])
		}
		if level <= 0 {
			return ""
		}
		return fmt.Sprintf("%s/egg%d.%s", base, level, imageType)
	case "weather":
		weatherID := getInt(hook.Message["condition"])
		if weatherID == 0 {
			weatherID = getInt(hook.Message["weather"])
		}
		if weatherID <= 0 {
			weatherID = 0
		}
		return fmt.Sprintf("%s/%d.%s", base, weatherID, imageType)
	default:
		return ""
	}
}

func resolvePokemonIcon(imageType string, pokemonID, form, evolution, gender, costume, alignment int, shiny bool, bread int) string {
	breadSuffixes := []string{""}
	if bread > 0 {
		breadSuffixes = []string{fmt.Sprintf("_b%d", bread), ""}
	}
	evolutionSuffixes := suffixOptions(evolution, "_e")
	formSuffixes := suffixOptions(form, "_f")
	costumeSuffixes := suffixOptions(costume, "_c")
	genderSuffixes := suffixOptions(gender, "_g")
	alignmentSuffixes := suffixOptions(alignment, "_a")
	shinySuffixes := []string{"_s", ""}
	if !shiny {
		shinySuffixes = []string{""}
	}
	for _, breadSuffix := range breadSuffixes {
		for _, evolutionSuffix := range evolutionSuffixes {
			for _, formSuffix := range formSuffixes {
				for _, costumeSuffix := range costumeSuffixes {
					for _, genderSuffix := range genderSuffixes {
						for _, alignmentSuffix := range alignmentSuffixes {
							for _, shinySuffix := range shinySuffixes {
								return fmt.Sprintf("%d%s%s%s%s%s%s%s.%s", pokemonID, breadSuffix, evolutionSuffix, formSuffix, costumeSuffix, genderSuffix, alignmentSuffix, shinySuffix, imageType)
							}
						}
					}
				}
			}
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func resolveEggIcon(imageType string, level int, hatched, ex bool) string {
	hatchedSuffixes := suffixFlag(hatched, "_h")
	exSuffixes := suffixFlag(ex, "_ex")
	for _, hatchedSuffix := range hatchedSuffixes {
		for _, exSuffix := range exSuffixes {
			return fmt.Sprintf("%d%s%s.%s", level, hatchedSuffix, exSuffix, imageType)
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func resolveGymIcon(imageType string, teamID, trainerCount int, inBattle, ex bool) string {
	trainerSuffixes := suffixOptions(trainerCount, "_t")
	inBattleSuffixes := suffixFlag(inBattle, "_b")
	exSuffixes := suffixFlag(ex, "_ex")
	for _, trainerSuffix := range trainerSuffixes {
		for _, inBattleSuffix := range inBattleSuffixes {
			for _, exSuffix := range exSuffixes {
				return fmt.Sprintf("%d%s%s%s.%s", teamID, trainerSuffix, inBattleSuffix, exSuffix, imageType)
			}
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func resolveWeatherIcon(imageType string, weatherID int) string {
	if weatherID <= 0 {
		return fmt.Sprintf("0.%s", imageType)
	}
	return fmt.Sprintf("%d.%s", weatherID, imageType)
}

func resolveInvasionIcon(imageType string, gruntType int) string {
	if gruntType <= 0 {
		return fmt.Sprintf("0.%s", imageType)
	}
	return fmt.Sprintf("%d.%s", gruntType, imageType)
}

func resolvePokestopIcon(imageType string, lureID int, invasionActive bool, incidentDisplayType int, questActive bool) string {
	invasionSuffixes := suffixFlag(invasionActive, "_i")
	displaySuffixes := suffixOptions(incidentDisplayType, "")
	questSuffixes := suffixFlag(questActive, "_q")
	for _, invasionSuffix := range invasionSuffixes {
		for _, displaySuffix := range displaySuffixes {
			for _, questSuffix := range questSuffixes {
				return fmt.Sprintf("%d%s%s%s.%s", lureID, invasionSuffix, displaySuffix, questSuffix, imageType)
			}
		}
	}
	return fmt.Sprintf("0.%s", imageType)
}

func suffixOptions(value int, prefix string) []string {
	if value > 0 {
		return []string{fmt.Sprintf("%s%d", prefix, value), ""}
	}
	return []string{""}
}

func suffixFlag(enabled bool, suffix string) []string {
	if enabled {
		return []string{suffix, ""}
	}
	return []string{""}
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func parseNumber(value string) (any, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, false
	}
	if i, err := strconv.Atoi(trimmed); err == nil {
		return i, true
	}
	if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return f, true
	}
	return nil, false
}

func ivColor(value any) int {
	iv := toFloat(value)
	switch {
	case iv >= 90:
		return 0x00FF00
	case iv >= 80:
		return 0x7FFF00
	case iv >= 66:
		return 0xFFFF00
	case iv >= 50:
		return 0xFFA500
	default:
		return 0xFF0000
	}
}

func moveName(p *Processor, moveID int) string {
	if moveID == 0 || p == nil || p.data == nil {
		return ""
	}
	raw, ok := p.data.Moves[fmt.Sprintf("%d", moveID)]
	if !ok {
		return ""
	}
	if m, ok := raw.(map[string]any); ok {
		if name, ok := m["name"].(string); ok {
			return name
		}
	}
	return ""
}

func moveEmoji(p *Processor, moveID int, platform string, tr *i18n.Translator) string {
	if moveID == 0 || p == nil || p.data == nil {
		return ""
	}
	raw, ok := p.data.Moves[fmt.Sprintf("%d", moveID)]
	if !ok {
		return ""
	}
	move, ok := raw.(map[string]any)
	if !ok {
		return ""
	}
	typeName := getString(move["type"])
	if typeName == "" {
		return ""
	}
	_, emojiKey := typeStyle(p, typeName)
	if emojiKey == "" {
		return ""
	}
	emoji := lookupEmojiForPlatform(p, emojiKey, platform)
	if emoji == "" {
		return ""
	}
	return translateMaybe(tr, emoji)
}

func gymName(hook *Hook) string {
	if name := getString(hook.Message["gym_name"]); name != "" {
		return name
	}
	if name := getString(hook.Message["name"]); name != "" {
		return name
	}
	return ""
}

func campfireLink(lat, lon float64, gymID any, gymName any, gymURL any) string {
	if lat == 0 && lon == 0 {
		return ""
	}
	marker := getString(gymID)
	if marker == "" {
		marker = generateMarkerID()
	}
	latStr := strconv.FormatFloat(lat, 'f', -1, 64)
	lonStr := strconv.FormatFloat(lon, 'f', -1, 64)
	deepLinkData := fmt.Sprintf("r=map&lat=%s&lng=%s&m=%s&g=PGO", latStr, lonStr, marker)
	encodedData := base64.StdEncoding.EncodeToString([]byte(deepLinkData))

	title := getString(gymName)
	if title == "" {
		title = "Gym"
	}
	image := getString(gymURL)
	if image == "" {
		image = "https://social.nianticlabs.com/images/gym-link-social-preview.png"
	}

	return fmt.Sprintf("https://campfire.onelink.me/eBr8?af_dp=campfire://&af_force_deeplink=true&deep_link_sub1=%s&af_og_title=%s&af_og_description=%%20&af_og_image=%s",
		encodedData,
		encodeURIComponent(title),
		encodeURIComponent(image),
	)
}

func generateMarkerID() string {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		raw[0], raw[1], raw[2], raw[3],
		raw[4], raw[5],
		raw[6], raw[7],
		raw[8], raw[9],
		raw[10], raw[11], raw[12], raw[13], raw[14], raw[15],
	)
}

func teamInfo(teamID int) (string, int) {
	if teamID < 0 {
		return "", 0
	}
	switch teamID {
	case 1:
		return "Mystic", 0x1E90FF
	case 2:
		return "Valor", 0xFF0000
	case 3:
		return "Instinct", 0xFFFF00
	default:
		return "Neutral", 0x808080
	}
}

func encodeURIComponent(value string) string {
	escaped := url.QueryEscape(value)
	escaped = strings.ReplaceAll(escaped, "+", "%20")
	replacer := strings.NewReplacer(
		"%21", "!",
		"%27", "'",
		"%28", "(",
		"%29", ")",
		"%2A", "*",
		"%7E", "~",
	)
	return replacer.Replace(escaped)
}

func normalizeCampfireMarker(marker string) string {
	if marker == "" {
		return marker
	}
	if dot := strings.Index(marker, "."); dot > 0 {
		marker = marker[:dot]
	}
	lower := strings.ToLower(marker)
	if len(lower) != 32 {
		return marker
	}
	for _, r := range lower {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return marker
		}
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", lower[0:8], lower[8:12], lower[12:16], lower[16:20], lower[20:32])
}

func weatherInfo(p *Processor, weatherID int, platform string, tr *i18n.Translator) (string, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return "", ""
	}
	weatherRaw, ok := p.data.UtilData["weather"].(map[string]any)
	if !ok {
		return "", ""
	}
	entry, ok := weatherRaw[fmt.Sprintf("%d", weatherID)].(map[string]any)
	if !ok {
		return "", ""
	}
	name := getString(entry["name"])
	emojiKey := getString(entry["emoji"])
	emoji := lookupEmojiForPlatform(p, emojiKey, platform)
	if tr != nil {
		name = tr.Translate(name, false)
		if emoji != "" {
			emoji = tr.Translate(emoji, false)
		}
	}
	return name, emoji
}

func weatherEntry(p *Processor, weatherID int) (string, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return "", ""
	}
	weatherRaw, ok := p.data.UtilData["weather"].(map[string]any)
	if !ok {
		return "", ""
	}
	entry, ok := weatherRaw[fmt.Sprintf("%d", weatherID)].(map[string]any)
	if !ok {
		return "", ""
	}
	return getString(entry["name"]), getString(entry["emoji"])
}

func teamDetails(p *Processor, teamID int) (string, string, int) {
	name, color := teamInfo(teamID)
	emojiKey := ""
	if p != nil && p.data != nil && p.data.UtilData != nil {
		if teams, ok := p.data.UtilData["teams"].(map[string]any); ok {
			if entry, ok := teams[strconv.Itoa(teamID)].(map[string]any); ok {
				if entryName := getString(entry["name"]); entryName != "" {
					name = entryName
				}
				if entryEmoji := getString(entry["emoji"]); entryEmoji != "" {
					emojiKey = entryEmoji
				}
				if entryColor := getString(entry["color"]); entryColor != "" {
					if parsed, err := strconv.ParseInt(strings.TrimPrefix(entryColor, "#"), 16, 32); err == nil {
						color = int(parsed)
					}
				}
			}
		}
	}
	return name, emojiKey, color
}

func raidLevelName(p *Processor, level int) string {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return fmt.Sprintf("Level %d", level)
	}
	raw, ok := p.data.UtilData["raidLevels"].(map[string]any)
	if !ok {
		return fmt.Sprintf("Level %d", level)
	}
	if entry, ok := raw[strconv.Itoa(level)]; ok {
		if name := fmt.Sprintf("%v", entry); name != "" {
			return name
		}
	}
	return fmt.Sprintf("Level %d", level)
}

func maxbattleLevelName(p *Processor, level int) string {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return fmt.Sprintf("Level %d", level)
	}
	raw, ok := p.data.UtilData["maxbattleLevels"].(map[string]any)
	if !ok {
		return fmt.Sprintf("Level %d", level)
	}
	if entry, ok := raw[strconv.Itoa(level)]; ok {
		if name := fmt.Sprintf("%v", entry); name != "" {
			return name
		}
	}
	return fmt.Sprintf("Level %d", level)
}

func evolutionName(p *Processor, evolutionID int) string {
	if p == nil || p.data == nil || p.data.UtilData == nil || evolutionID == 0 {
		return ""
	}
	raw, ok := p.data.UtilData["evolution"].(map[string]any)
	if !ok {
		return ""
	}
	entry, ok := raw[strconv.Itoa(evolutionID)].(map[string]any)
	if !ok {
		return ""
	}
	return getString(entry["name"])
}

func megaNameFormat(p *Processor, evolutionID int) string {
	if p == nil || p.data == nil || p.data.UtilData == nil || evolutionID == 0 {
		return ""
	}
	raw, ok := p.data.UtilData["megaName"].(map[string]any)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%v", raw[strconv.Itoa(evolutionID)])
}

func translatorFor(p *Processor, language string) *i18n.Translator {
	if p == nil || p.i18n == nil {
		return nil
	}
	if language == "" && p.cfg != nil {
		if val, ok := p.cfg.GetString("general.locale"); ok {
			language = val
		}
	}
	return p.i18n.Translator(language)
}

func translateMaybe(tr *i18n.Translator, value string) string {
	if tr == nil || value == "" {
		return value
	}
	return tr.Translate(value, false)
}

func questString(p *Processor, hook *Hook, language string, tr *i18n.Translator) string {
	if hook == nil {
		return ""
	}
	if task := getString(hook.Message["quest_task"]); task != "" {
		if !getBoolFromConfig(p.cfg, "general.ignoreMADQuestString", false) {
			return task
		}
	}
	lang := language
	if lang == "" && p != nil && p.cfg != nil {
		if value, ok := p.cfg.GetString("general.locale"); ok && value != "" {
			lang = value
		}
	}
	if lang == "" {
		lang = "en"
	}
	if p != nil && p.data != nil && p.data.Translations != nil {
		if raw, ok := p.data.Translations[lang]; ok {
			if langMap, ok := raw.(map[string]any); ok {
				if title := getString(hook.Message["title"]); title != "" {
					key := fmt.Sprintf("quest_title_%s", strings.ToLower(title))
					if questTitles, ok := langMap["questTitles"].(map[string]any); ok {
						if text, ok := questTitles[key].(string); ok && text != "" {
							if strings.Contains(strings.ToLower(text), "{{amount_0}}") {
								target := getInt(hook.Message["target"])
								if target == 0 {
									target = getInt(hook.Message["quest_target"])
								}
								if target == 0 {
									target = getInt(hook.Message["target_amount"])
								}
								if target != 0 {
									text = strings.ReplaceAll(text, "{{amount_0}}", fmt.Sprintf("%d", target))
								}
							}
							return text
						}
					}
					if questTypes, ok := langMap["questTypes"].(map[string]any); ok {
						if text, ok := questTypes["quest_0"].(string); ok && text != "" {
							return text
						}
					}
				}
			}
		}
	}
	if title := getString(hook.Message["quest_title"]); title != "" {
		return translateMaybe(tr, title)
	}
	if questType := getInt(hook.Message["quest_type"]); questType > 0 && p != nil && p.data != nil {
		if raw, ok := p.data.QuestTypes[fmt.Sprintf("%d", questType)]; ok {
			if m, ok := raw.(map[string]any); ok {
				if text, ok := m["text"].(string); ok {
					return translateMaybe(tr, text)
				}
			}
		}
	}
	return ""
}

func rewardString(p *Processor, hook *Hook, tr *i18n.Translator) string {
	rewardType := getInt(hook.Message["reward_type"])
	reward := getInt(hook.Message["reward"])
	amount := getInt(hook.Message["amount"])
	if rewardType == 0 {
		rewardType = getInt(hook.Message["reward_type_id"])
	}
	if reward == 0 {
		reward = getInt(hook.Message["pokemon_id"])
	}
	basic := ""
	switch rewardType {
	case 2:
		basic = itemRewardString(p, reward, amount, tr)
	case 3:
		if amount == 0 {
			amount = reward
		}
		if amount > 0 {
			basic = fmt.Sprintf("%d %s", amount, translateMaybe(tr, "Stardust"))
		}
	case 4:
		if amount == 0 {
			amount = 1
		}
		basic = fmt.Sprintf("%d %s", amount, translateMaybe(tr, "Candy"))
	case 7:
		if reward != 0 {
			name := monsterName(p, reward)
			basic = fmt.Sprintf("%s", translateMaybe(tr, name))
		}
	case 12:
		if amount == 0 {
			amount = 1
		}
		basic = fmt.Sprintf("%d %s", amount, translateMaybe(tr, "Mega Energy"))
	}
	if basic != "" {
		return basic
	}
	if hook != nil && hook.Type == "quest" && p != nil {
		rewardData := questRewardData(p, hook)
		temp := map[string]any{}
		applyQuestRewardDetails(p, temp, rewardData, "", tr)
		if text, ok := temp["rewardString"].(string); ok && text != "" {
			return text
		}
	}
	return ""
}

func emptyQuestRewardData() map[string]any {
	return map[string]any{
		"monsters":       []map[string]any{},
		"items":          []map[string]any{},
		"energyMonsters": []map[string]any{},
		"candy":          []map[string]any{},
		"dustAmount":     0,
		"itemAmount":     0,
	}
}

func questRewardDataIsEmpty(rewardData map[string]any) bool {
	if rewardData == nil {
		return true
	}
	if getInt(rewardData["dustAmount"]) > 0 {
		return false
	}
	if items, _ := rewardData["items"].([]map[string]any); len(items) > 0 {
		return false
	}
	if monsters, _ := rewardData["monsters"].([]map[string]any); len(monsters) > 0 {
		return false
	}
	if energy, _ := rewardData["energyMonsters"].([]map[string]any); len(energy) > 0 {
		return false
	}
	if candy, _ := rewardData["candy"].([]map[string]any); len(candy) > 0 {
		return false
	}
	return true
}

func questRewardDataStandard(p *Processor, hook *Hook) map[string]any {
	out := map[string]any{
		"monsters":       []map[string]any{},
		"items":          []map[string]any{},
		"energyMonsters": []map[string]any{},
		"candy":          []map[string]any{},
		"dustAmount":     0,
		"itemAmount":     0,
	}
	if hook == nil {
		return out
	}
	rewards := questRewardsFromHook(hook)
	for _, reward := range rewards {
		rewardType := getInt(reward["type"])
		info, _ := reward["info"].(map[string]any)
		switch rewardType {
		case 2:
			id := getIntFromMap(info, "item_id")
			if id == 0 {
				id = getIntFromMap(info, "id")
			}
			amount := getIntFromMap(info, "amount")
			if id > 0 {
				out["items"] = append(out["items"].([]map[string]any), map[string]any{
					"id":     id,
					"amount": amount,
				})
				if out["itemAmount"].(int) == 0 && amount > 0 {
					out["itemAmount"] = amount
				}
			}
		case 3:
			amount := getIntFromMap(info, "amount")
			if amount > 0 {
				out["dustAmount"] = amount
			}
		case 4:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		case 7:
			pokemonID := getIntFromMap(info, "pokemon_id")
			formID := getIntFromMap(info, "form_id")
			if formID == 0 {
				formID = getIntFromMap(info, "form")
			}
			shiny := getBoolFromMap(info, "shiny")
			if pokemonID > 0 {
				out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
					"pokemonId": pokemonID,
					"formId":    formID,
					"shiny":     shiny,
				})
			}
		case 12:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		}
	}
	if len(rewards) == 0 {
		rewardType := getInt(hook.Message["reward_type"])
		if rewardType == 0 {
			rewardType = getInt(hook.Message["quest_reward_type"])
		}
		if rewardType == 0 {
			rewardType = getInt(hook.Message["reward_type_id"])
		}
		reward := getInt(hook.Message["reward"])
		if reward == 0 {
			if rewardType == 2 {
				reward = getInt(hook.Message["quest_item_id"])
			} else if rewardType == 7 {
				reward = getInt(hook.Message["quest_pokemon_id"])
			}
		}
		if reward == 0 {
			reward = getInt(hook.Message["pokemon_id"])
		}
		amount := getInt(hook.Message["reward_amount"])
		if amount == 0 {
			amount = getInt(hook.Message["quest_reward_amount"])
		}
		if amount == 0 {
			amount = getInt(hook.Message["amount"])
		}
		switch rewardType {
		case 2:
			if reward > 0 {
				out["items"] = append(out["items"].([]map[string]any), map[string]any{
					"id":     reward,
					"amount": amount,
				})
				if amount > 0 {
					out["itemAmount"] = amount
				}
			}
		case 3:
			if amount == 0 {
				amount = reward
			}
			out["dustAmount"] = amount
		case 4:
			out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
				"pokemonId": reward,
				"amount":    amount,
			})
		case 7:
			if reward > 0 {
				out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
					"pokemonId": reward,
					"formId":    getInt(hook.Message["form"]),
					"shiny":     getBool(hook.Message["shiny"]),
				})
			}
		case 12:
			out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
				"pokemonId": reward,
				"amount":    amount,
			})
		}
	}
	return out
}

// questRewardData returns the "No AR" reward payload for quest hooks.
// Some Golbat setups send quest hooks with a `with_ar` boolean and only one reward payload (in `rewards`).
// In that case, the reward belongs to the "With AR" variant, so this should be empty.
func questRewardData(p *Processor, hook *Hook) map[string]any {
	if hook == nil {
		return emptyQuestRewardData()
	}
	withAR := getBool(hook.Message["with_ar"])
	// If the hook includes explicit alternative quest fields, treat it as a combined payload
	// where the standard quest_* fields represent "No AR" regardless of with_ar.
	hasAlternative := len(questRewardsFromHookAR(hook)) > 0 || getInt(hook.Message["alternative_quest_reward_type"]) > 0
	if withAR && !hasAlternative {
		return emptyQuestRewardData()
	}
	return questRewardDataStandard(p, hook)
}

func questRewardsFromHook(hook *Hook) []map[string]any {
	if hook == nil {
		return nil
	}
	raw := firstNonEmpty(
		hook.Message["quest_rewards"],
		hook.Message["rewards"],
		hook.Message["reward"],
		hook.Message["quest_reward"],
	)
	return decodeQuestRewards(raw)
}

func questRewardsFromHookAR(hook *Hook) []map[string]any {
	if hook == nil {
		return nil
	}
	raw := firstNonEmpty(
		hook.Message["alternative_quest_rewards"],
		hook.Message["alternative_rewards"],
		hook.Message["alternative_reward"],
		hook.Message["alt_rewards"],
		hook.Message["rewards_alt"],
		hook.Message["quest_rewards_alt"],
	)
	return decodeQuestRewards(raw)
}

func decodeQuestRewards(raw any) []map[string]any {
	switch v := raw.(type) {
	case nil:
		return nil
	case []map[string]any:
		return v
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if entry, ok := item.(map[string]any); ok {
				out = append(out, entry)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{v}
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return decodeQuestRewards(string(v))
	case json.RawMessage:
		if len(v) == 0 {
			return nil
		}
		return decodeQuestRewards(string(v))
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var decoded []map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return decoded
		}
		var rawDecoded []any
		if err := json.Unmarshal([]byte(v), &rawDecoded); err == nil {
			out := make([]map[string]any, 0, len(rawDecoded))
			for _, item := range rawDecoded {
				if entry, ok := item.(map[string]any); ok {
					out = append(out, entry)
				}
			}
			return out
		}
	}
	return nil
}

func firstNonEmpty(values ...any) any {
	for _, value := range values {
		switch v := value.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(v) == "" {
				continue
			}
			return v
		default:
			return value
		}
	}
	return nil
}

func questRewardDataFromRewardsAndMessage(p *Processor, rewards []map[string]any, msg map[string]any) map[string]any {
	out := map[string]any{
		"monsters":       []map[string]any{},
		"items":          []map[string]any{},
		"energyMonsters": []map[string]any{},
		"candy":          []map[string]any{},
		"dustAmount":     0,
		"itemAmount":     0,
	}
	for _, reward := range rewards {
		rewardType := getInt(reward["type"])
		info, _ := reward["info"].(map[string]any)
		switch rewardType {
		case 2:
			id := getIntFromMap(info, "item_id")
			if id == 0 {
				id = getIntFromMap(info, "id")
			}
			amount := getIntFromMap(info, "amount")
			if id > 0 {
				out["items"] = append(out["items"].([]map[string]any), map[string]any{
					"id":     id,
					"amount": amount,
				})
				if out["itemAmount"].(int) == 0 && amount > 0 {
					out["itemAmount"] = amount
				}
			}
		case 3:
			amount := getIntFromMap(info, "amount")
			if amount > 0 {
				out["dustAmount"] = amount
			}
		case 4:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		case 7:
			pokemonID := getIntFromMap(info, "pokemon_id")
			formID := getIntFromMap(info, "form_id")
			if formID == 0 {
				formID = getIntFromMap(info, "form")
			}
			shiny := getBoolFromMap(info, "shiny")
			if pokemonID > 0 {
				out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
					"pokemonId": pokemonID,
					"formId":    formID,
					"shiny":     shiny,
				})
			}
		case 12:
			pokemonID := getIntFromMap(info, "pokemon_id")
			amount := getIntFromMap(info, "amount")
			out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
				"pokemonId": pokemonID,
				"amount":    amount,
			})
		}
	}
	if len(rewards) > 0 || msg == nil {
		return out
	}
	rewardType := getInt(msg["reward_type"])
	if rewardType == 0 {
		rewardType = getInt(msg["reward_type_id"])
	}
	reward := getInt(msg["reward"])
	if reward == 0 {
		reward = getInt(msg["pokemon_id"])
	}
	amount := getInt(msg["reward_amount"])
	if amount == 0 {
		amount = getInt(msg["amount"])
	}
	switch rewardType {
	case 2:
		if reward > 0 {
			out["items"] = append(out["items"].([]map[string]any), map[string]any{
				"id":     reward,
				"amount": amount,
			})
			if amount > 0 {
				out["itemAmount"] = amount
			}
		}
	case 3:
		if amount == 0 {
			amount = reward
		}
		out["dustAmount"] = amount
	case 4:
		out["candy"] = append(out["candy"].([]map[string]any), map[string]any{
			"pokemonId": reward,
			"amount":    amount,
		})
	case 7:
		if reward > 0 {
			out["monsters"] = append(out["monsters"].([]map[string]any), map[string]any{
				"pokemonId": reward,
				"formId":    getInt(msg["form"]),
				"shiny":     getBool(msg["shiny"]),
			})
		}
	case 12:
		out["energyMonsters"] = append(out["energyMonsters"].([]map[string]any), map[string]any{
			"pokemonId": reward,
			"amount":    amount,
		})
	}
	return out
}

func questRewardDataAR(p *Processor, hook *Hook) map[string]any {
	if hook == nil {
		return emptyQuestRewardData()
	}
	rewards := questRewardsFromHookAR(hook)
	msg := map[string]any{
		"reward_type":   hook.Message["alternative_quest_reward_type"],
		"reward_amount": hook.Message["alternative_quest_reward_amount"],
		"pokemon_id":    hook.Message["alternative_quest_pokemon_id"],
		"form":          hook.Message["alternative_quest_pokemon_form"],
		"shiny":         hook.Message["alternative_quest_shiny"],
	}
	rewardType := getInt(msg["reward_type"])
	if rewardType == 2 {
		msg["reward"] = hook.Message["alternative_quest_item_id"]
	} else if rewardType == 7 {
		msg["reward"] = hook.Message["alternative_quest_pokemon_id"]
	} else {
		msg["reward"] = hook.Message["alternative_quest_reward"]
	}
	if getInt(msg["reward_amount"]) == 0 {
		msg["reward_amount"] = hook.Message["alternative_quest_amount"]
	}
	out := questRewardDataFromRewardsAndMessage(p, rewards, msg)
	// Fallback: single-quest webhooks with `with_ar=true` use the standard rewards fields.
	if questRewardDataIsEmpty(out) && getBool(hook.Message["with_ar"]) {
		return questRewardDataStandard(p, hook)
	}
	return out
}

func questRewardStringFromData(p *Processor, rewardData map[string]any, tr *i18n.Translator) string {
	if rewardData == nil {
		return ""
	}
	temp := map[string]any{}
	applyQuestRewardDetails(p, temp, rewardData, "", tr)
	if text, ok := temp["rewardString"].(string); ok {
		return text
	}
	return ""
}

func matchQuestWithVariants(hook *Hook, row map[string]any, rewardDataNoAR, rewardDataAR map[string]any) (bool, bool, bool) {
	if hook == nil || row == nil {
		return false, false, false
	}
	arMode := getInt(row["ar"])
	noARMatch := false
	arMatch := false
	if arMode == 0 || arMode == 1 {
		noARMatch = matchQuestWithData(hook, row, rewardDataNoAR)
	}
	if arMode == 0 || arMode == 2 {
		arMatch = matchQuestWithData(hook, row, rewardDataAR)
	}
	switch arMode {
	case 1:
		return noARMatch, noARMatch, false
	case 2:
		return arMatch, false, arMatch
	default:
		return noARMatch || arMatch, noARMatch, arMatch
	}
}

func applyQuestRewardDetails(p *Processor, data map[string]any, rewardData map[string]any, platform string, tr *i18n.Translator) {
	if data == nil || rewardData == nil {
		return
	}
	monsters, _ := rewardData["monsters"].([]map[string]any)
	items, _ := rewardData["items"].([]map[string]any)
	energyMonsters, _ := rewardData["energyMonsters"].([]map[string]any)
	candy, _ := rewardData["candy"].([]map[string]any)
	dustAmount := getInt(rewardData["dustAmount"])
	data["dustAmount"] = dustAmount
	data["itemAmount"] = getInt(rewardData["itemAmount"])
	if len(items) == 0 {
		data["itemAmount"] = 0
	}
	if len(energyMonsters) > 0 {
		data["energyAmount"] = getInt(energyMonsters[0]["amount"])
	} else {
		data["energyAmount"] = 0
	}
	if len(candy) > 0 {
		data["candyAmount"] = getInt(candy[0]["amount"])
	} else {
		data["candyAmount"] = 0
	}
	if len(monsters) > 0 {
		data["isShiny"] = getBool(monsters[0]["shiny"])
		pokemonID := getInt(monsters[0]["pokemonId"])
		formID := getInt(monsters[0]["formId"])
		if p != nil && p.shinyPossible != nil && pokemonID > 0 {
			data["shinyPossible"] = p.shinyPossible.IsPossible(pokemonID, formID)
		} else {
			data["shinyPossible"] = false
		}
		if p != nil {
			if stats, ok := lookupMonsterStats(p, pokemonID, formID); ok {
				data["baseStats"] = stats
			}
		}
	} else {
		data["isShiny"] = false
		data["shinyPossible"] = false
	}
	for _, monster := range monsters {
		pokemonID := getInt(monster["pokemonId"])
		formID := getInt(monster["formId"])
		name, formName := monsterInfo(p, pokemonID, formID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		if strings.EqualFold(formName, "Normal") {
			formName = ""
		}
		monster["nameEng"] = name
		monster["formEng"] = formName
		monster["name"] = translateMaybe(tr, name)
		monster["form"] = translateMaybe(tr, formName)
		fullNameEng := name
		if formName != "" {
			fullNameEng = fmt.Sprintf("%s %s", name, formName)
		}
		fullName := translateMaybe(tr, name)
		if translatedForm := getString(monster["form"]); translatedForm != "" {
			fullName = fmt.Sprintf("%s %s", fullName, translatedForm)
		}
		monster["fullNameEng"] = fullNameEng
		monster["fullName"] = fullName
	}
	data["monsterNames"] = joinQuestMonsterNames(monsters, "fullName")
	data["monsterNamesEng"] = joinQuestMonsterNames(monsters, "fullNameEng")
	for _, item := range items {
		itemID := getInt(item["id"])
		name := itemName(p, itemID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown item"), itemID)
		}
		item["nameEng"] = name
		item["name"] = translateMaybe(tr, name)
	}
	data["itemNames"] = joinQuestItemNames(items, true)
	data["itemNamesEng"] = joinQuestItemNames(items, false)
	for _, monster := range energyMonsters {
		pokemonID := getInt(monster["pokemonId"])
		name := monsterName(p, pokemonID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		monster["nameEng"] = name
		monster["name"] = translateMaybe(tr, name)
	}
	for _, monster := range candy {
		pokemonID := getInt(monster["pokemonId"])
		name := monsterName(p, pokemonID)
		if name == "" {
			name = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		monster["nameEng"] = name
		monster["name"] = translateMaybe(tr, name)
	}
	data["energyMonstersNames"] = joinQuestEnergyNames(energyMonsters, tr, "Mega Energy")
	data["energyMonstersNamesEng"] = joinQuestEnergyNames(energyMonsters, nil, "Mega Energy")
	data["candyMonstersNames"] = joinQuestEnergyNames(candy, tr, "Candy")
	data["candyMonstersNamesEng"] = joinQuestEnergyNames(candy, nil, "Candy")
	rewardString := []string{
		data["monsterNames"].(string),
		"",
		data["itemNames"].(string),
		data["energyMonstersNames"].(string),
		data["candyMonstersNames"].(string),
	}
	if dustAmount > 0 {
		rewardString[1] = fmt.Sprintf("%d %s", dustAmount, translateMaybe(tr, "Stardust"))
	}
	rewardStringEng := []string{
		data["monsterNamesEng"].(string),
		"",
		data["itemNamesEng"].(string),
		data["energyMonstersNamesEng"].(string),
		data["candyMonstersNamesEng"].(string),
	}
	if dustAmount > 0 {
		rewardStringEng[1] = fmt.Sprintf("%d Stardust", dustAmount)
	}
	data["rewardString"] = joinNonEmpty(rewardString)
	data["rewardStringEng"] = joinNonEmpty(rewardStringEng)
	if shinyPossible, ok := data["shinyPossible"].(bool); ok && shinyPossible {
		data["shinyPossibleEmoji"] = translateMaybe(tr, lookupEmojiForPlatform(p, "shiny", platform))
	} else {
		data["shinyPossibleEmoji"] = ""
	}
}

func applyQuestRewardImages(p *Processor, data map[string]any, rewardData map[string]any) {
	if p == nil || p.cfg == nil || data == nil || rewardData == nil {
		return
	}
	shiny := getBool(data["isShiny"])
	if !shiny {
		if getBoolFromConfig(p.cfg, "general.requestShinyImages", false) && getBool(data["shinyPossible"]) {
			shiny = true
		}
	}
	if base := imageBaseURL(p.cfg, "quest", "general.images", "general.imgUrl"); base != "" {
		if url := questRewardIconURL(base, "png", rewardData, shiny); url != "" {
			data["imgUrl"] = url
		}
	}
	if base := imageBaseURL(p.cfg, "quest", "general.images", "general.imgUrlAlt"); base != "" {
		if url := questRewardIconURL(base, "png", rewardData, shiny); url != "" {
			data["imgUrlAlt"] = url
		}
	}
	if base := imageBaseURL(p.cfg, "quest", "general.stickers", "general.stickerUrl"); base != "" {
		if url := questRewardIconURL(base, "webp", rewardData, shiny); url != "" {
			data["stickerUrl"] = url
		}
	}
}

func questRewardIconURL(baseURL, imageType string, rewardData map[string]any, shiny bool) string {
	if baseURL == "" || rewardData == nil {
		return ""
	}
	if !isUiconsRepo(baseURL, imageType) {
		return ""
	}
	client := uiconsClient(baseURL, imageType)
	monsters, _ := rewardData["monsters"].([]map[string]any)
	if len(monsters) > 0 {
		pokemonID := getInt(monsters[0]["pokemonId"])
		formID := getInt(monsters[0]["formId"])
		if url, ok := client.PokemonIcon(pokemonID, formID, 0, 0, 0, 0, shiny, 0); ok {
			return url
		}
	}
	items, _ := rewardData["items"].([]map[string]any)
	if len(items) > 0 {
		itemID := getInt(items[0]["id"])
		if url, ok := client.RewardItemIcon(itemID); ok {
			return url
		}
	}
	if dustAmount := getInt(rewardData["dustAmount"]); dustAmount > 0 {
		if url, ok := client.RewardStardustIcon(dustAmount); ok {
			return url
		}
	}
	energyMonsters, _ := rewardData["energyMonsters"].([]map[string]any)
	if len(energyMonsters) > 0 {
		pokemonID := getInt(energyMonsters[0]["pokemonId"])
		amount := getInt(energyMonsters[0]["amount"])
		if url, ok := client.RewardMegaEnergyIcon(pokemonID, amount); ok {
			return url
		}
	}
	candy, _ := rewardData["candy"].([]map[string]any)
	if len(candy) > 0 {
		pokemonID := getInt(candy[0]["pokemonId"])
		amount := getInt(candy[0]["amount"])
		if url, ok := client.RewardCandyIcon(pokemonID, amount); ok {
			return url
		}
	}
	return ""
}

func joinQuestMonsterNames(monsters []map[string]any, key string) string {
	names := make([]string, 0, len(monsters))
	for _, monster := range monsters {
		if name := getString(monster[key]); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func joinQuestItemNames(items []map[string]any, translated bool) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		nameKey := "nameEng"
		if translated {
			nameKey = "name"
		}
		name := getString(item[nameKey])
		amount := getInt(item["amount"])
		if name == "" {
			continue
		}
		if amount > 0 {
			names = append(names, fmt.Sprintf("%d %s", amount, name))
		} else {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func joinQuestEnergyNames(items []map[string]any, tr *i18n.Translator, suffix string) string {
	names := make([]string, 0, len(items))
	suffixText := suffix
	if tr != nil && suffix != "" {
		suffixText = translateMaybe(tr, suffix)
	}
	for _, item := range items {
		nameKey := "nameEng"
		if tr != nil {
			nameKey = "name"
		}
		name := getString(item[nameKey])
		amount := getInt(item["amount"])
		if name == "" {
			continue
		}
		if suffixText != "" {
			names = append(names, fmt.Sprintf("%d %s %s", amount, name, suffixText))
		} else {
			names = append(names, fmt.Sprintf("%d %s", amount, name))
		}
	}
	return strings.Join(names, ", ")
}

func joinNonEmpty(values []string) string {
	return joinNonEmptyWithSep(values, ", ")
}

func joinNonEmptyWithSep(values []string, sep string) string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	return strings.Join(out, sep)
}

func formatTemplate(input string, args ...any) string {
	result := input
	for i := len(args) - 1; i >= 0; i-- {
		needle := fmt.Sprintf("{%d}", i)
		result = strings.ReplaceAll(result, needle, fmt.Sprintf("%v", args[i]))
	}
	return result
}

func getIntFromMap(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	return getInt(m[key])
}

func getBoolFromMap(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	return getBool(m[key])
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func lookupMonsterStats(p *Processor, pokemonID, formID int) (map[string]any, bool) {
	if p == nil || p.data == nil {
		return nil, false
	}
	monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return nil, false
	}
	stats, ok := monster["stats"].(map[string]any)
	return stats, ok
}

func itemName(p *Processor, itemID int) string {
	if itemID == 0 || p == nil || p.data == nil {
		return ""
	}
	raw, ok := p.data.Items[fmt.Sprintf("%d", itemID)]
	if !ok {
		return ""
	}
	if m, ok := raw.(map[string]any); ok {
		if name, ok := m["name"].(string); ok {
			return name
		}
	}
	return ""
}

func applyFortUpdateFields(data map[string]any, hook *Hook) {
	if data == nil || hook == nil {
		return
	}
	changeType := getString(hook.Message["change_type"])
	editTypes := []string{}
	switch v := hook.Message["edit_types"].(type) {
	case []string:
		editTypes = append(editTypes, v...)
	case []any:
		for _, item := range v {
			if entry, ok := item.(string); ok {
				editTypes = append(editTypes, entry)
			}
		}
	case string:
		var decoded []string
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			editTypes = append(editTypes, decoded...)
		}
	}
	oldEntry := mapFromAny(hook.Message["old"])
	newEntry := mapFromAny(hook.Message["new"])
	if changeType == "edit" && (getStringFromAnyMap(oldEntry, "name") == "" && getStringFromAnyMap(oldEntry, "description") == "") {
		changeType = "new"
		editTypes = nil
	}
	data["change_type"] = changeType
	if editTypes != nil {
		data["edit_types"] = editTypes
	} else {
		data["edit_types"] = nil
	}
	changeTypes := append([]string{}, editTypes...)
	if changeType != "" {
		changeTypes = append(changeTypes, changeType)
	}
	data["changeTypes"] = changeTypes
	if oldEntry != nil {
		data["old"] = oldEntry
	}
	if newEntry != nil {
		data["new"] = newEntry
	}
	isEmpty := true
	if newEntry != nil && (getString(newEntry["name"]) != "" || getString(newEntry["description"]) != "") {
		isEmpty = false
	}
	if oldEntry != nil && getString(oldEntry["name"]) != "" {
		isEmpty = false
	}
	data["isEmpty"] = isEmpty

	data["isEdit"] = changeType == "edit"
	data["isNew"] = changeType == "new"
	data["isRemoval"] = changeType == "removal"

	data["isEditLocation"] = containsString(changeTypes, "location")
	data["isEditName"] = containsString(changeTypes, "name")
	data["isEditDescription"] = containsString(changeTypes, "description")
	data["isEditImageUrl"] = containsString(changeTypes, "image_url")
	data["isEditImgUrl"] = data["isEditImageUrl"]

	oldName := getStringFromAnyMap(oldEntry, "name")
	oldDescription := getStringFromAnyMap(oldEntry, "description")
	oldImageURL := getStringFromAnyMap(oldEntry, "image_url")
	if oldImageURL == "" {
		oldImageURL = getStringFromAnyMap(oldEntry, "imageUrl")
	}
	oldLat, oldLon, _ := extractLocation(oldEntry)

	newName := getStringFromAnyMap(newEntry, "name")
	newDescription := getStringFromAnyMap(newEntry, "description")
	newImageURL := getStringFromAnyMap(newEntry, "image_url")
	if newImageURL == "" {
		newImageURL = getStringFromAnyMap(newEntry, "imageUrl")
	}
	newLat, newLon, _ := extractLocation(newEntry)

	data["oldName"] = oldName
	data["oldDescription"] = oldDescription
	data["oldImageUrl"] = oldImageURL
	data["oldImgUrl"] = oldImageURL
	data["oldLatitude"] = oldLat
	data["oldLongitude"] = oldLon

	data["newName"] = newName
	data["newDescription"] = newDescription
	data["newImageUrl"] = newImageURL
	data["newImgUrl"] = newImageURL
	data["newLatitude"] = newLat
	data["newLongitude"] = newLon

	fortType := getString(hook.Message["fort_type"])
	if fortType == "" {
		fortType = getString(hook.Message["fortType"])
	}
	if fortType == "" {
		fortType = getStringFromAnyMap(newEntry, "type")
		if fortType == "" {
			fortType = getStringFromAnyMap(oldEntry, "type")
		}
	}
	fortType = strings.ToLower(strings.TrimSpace(fortType))
	if fortType == "" {
		fortType = "unknown"
	}
	data["fortType"] = fortType
	if fortType == "pokestop" {
		data["fortTypeText"] = "Pokestop"
	} else {
		data["fortTypeText"] = "Gym"
	}
	switch changeType {
	case "edit":
		data["changeTypeText"] = "Edit"
	case "removal":
		data["changeTypeText"] = "Removal"
	case "new":
		data["changeTypeText"] = "New"
	}

	name := newName
	if name == "" {
		name = oldName
	}
	if name == "" {
		name = "unknown"
	}
	description := newDescription
	if description == "" {
		description = oldDescription
	}
	if description == "" {
		description = "unknown"
	}
	imgURL := newImageURL
	if imgURL == "" {
		imgURL = oldImageURL
	}
	data["name"] = name
	data["description"] = description
	data["imgUrl"] = imgURL

	if oldEntry != nil {
		oldEntry["imgUrl"] = oldImageURL
		oldEntry["imageUrl"] = oldImageURL
	}
	if newEntry != nil {
		newEntry["imgUrl"] = newImageURL
		newEntry["imageUrl"] = newImageURL
	}
}

func applyNightTime(p *Processor, hook *Hook, data map[string]any) {
	if p == nil || p.cfg == nil || hook == nil || data == nil {
		return
	}
	lat := getFloat(hook.Message["latitude"])
	lon := getFloat(hook.Message["longitude"])
	if lat == 0 && lon == 0 {
		return
	}
	checkTime, ok := nightTimeReference(hook)
	if !ok {
		return
	}
	loc := hookLocation(p, hook)
	if loc == nil {
		loc = time.Local
	}
	checkTime = checkTime.In(loc)
	sunrise, sunset, ok := sunriseSunset(checkTime, lat, lon, loc)
	if !ok {
		return
	}
	night := !(checkTime.After(sunrise) && checkTime.Before(sunset))
	dawn := checkTime.After(sunrise) && checkTime.Before(sunrise.Add(time.Hour))
	dusk := checkTime.After(sunset.Add(-time.Hour)) && checkTime.Before(sunset)
	data["nightTime"] = night
	data["dawnTime"] = dawn
	data["duskTime"] = dusk

	style := getStringFromConfig(p.cfg, "geocoding.dayStyle", "klokantech-basic")
	if dawn {
		if value := getStringFromConfig(p.cfg, "geocoding.dawnStyle", ""); value != "" {
			style = value
		}
	} else if dusk {
		if value := getStringFromConfig(p.cfg, "geocoding.duskStyle", ""); value != "" {
			style = value
		}
	} else if night {
		if value := getStringFromConfig(p.cfg, "geocoding.nightStyle", ""); value != "" {
			style = value
		}
	}
	data["style"] = style
}

func nightTimeReference(hook *Hook) (time.Time, bool) {
	if hook == nil {
		return time.Time{}, false
	}
	switch hook.Type {
	case "egg":
		start := getInt64(hook.Message["start"])
		if start == 0 {
			start = getInt64(hook.Message["hatch_time"])
		}
		if start > 0 {
			return time.Unix(start, 0), true
		}
	case "gym", "gym_details", "weather":
		return time.Now(), true
	default:
		if expire := hookExpiryUnix(hook); expire > 0 {
			return time.Unix(expire, 0), true
		}
	}
	return time.Time{}, false
}

func sunriseSunset(day time.Time, lat, lon float64, loc *time.Location) (time.Time, time.Time, bool) {
	if loc == nil {
		loc = time.Local
	}
	year, month, dayOfMonth := day.Date()
	n := dayOfYear(year, int(month), dayOfMonth)
	sunrise, ok1 := solarEventUTC(n, lat, lon, true)
	sunset, ok2 := solarEventUTC(n, lat, lon, false)
	if !ok1 || !ok2 {
		return time.Time{}, time.Time{}, false
	}
	base := time.Date(year, month, dayOfMonth, 0, 0, 0, 0, time.UTC)
	sunriseTime := base.Add(time.Duration(sunrise * float64(time.Hour))).In(loc)
	sunsetTime := base.Add(time.Duration(sunset * float64(time.Hour))).In(loc)
	return sunriseTime, sunsetTime, true
}

func dayOfYear(year, month, day int) int {
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.UTC)
	current := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int(current.Sub(start).Hours()/24) + 1
}

func solarEventUTC(dayOfYear int, lat, lon float64, sunrise bool) (float64, bool) {
	if lat > 89.8 {
		lat = 89.8
	}
	if lat < -89.8 {
		lat = -89.8
	}
	zenith := 90.833
	lngHour := lon / 15.0
	t := float64(dayOfYear) + ((6.0 - lngHour) / 24.0)
	if !sunrise {
		t = float64(dayOfYear) + ((18.0 - lngHour) / 24.0)
	}
	m := (0.9856 * t) - 3.289
	l := m + (1.916 * math.Sin(degToRad(m))) + (0.020 * math.Sin(2*degToRad(m))) + 282.634
	l = math.Mod(l+360, 360)
	ra := radToDeg(math.Atan(0.91764 * math.Tan(degToRad(l))))
	ra = math.Mod(ra+360, 360)
	lQuadrant := math.Floor(l/90.0) * 90.0
	raQuadrant := math.Floor(ra/90.0) * 90.0
	ra = ra + (lQuadrant - raQuadrant)
	ra = ra / 15.0

	sinDec := 0.39782 * math.Sin(degToRad(l))
	cosDec := math.Cos(math.Asin(sinDec))

	cosH := (math.Cos(degToRad(zenith)) - (sinDec * math.Sin(degToRad(lat)))) / (cosDec * math.Cos(degToRad(lat)))
	if cosH > 1 || cosH < -1 {
		return 0, false
	}
	var h float64
	if sunrise {
		h = 360 - radToDeg(math.Acos(cosH))
	} else {
		h = radToDeg(math.Acos(cosH))
	}
	h = h / 15.0
	tVal := h + ra - (0.06571 * t) - 6.622
	ut := tVal - lngHour
	ut = math.Mod(ut+24, 24)
	return ut, true
}

func degToRad(deg float64) float64 {
	return deg * (math.Pi / 180.0)
}

func radToDeg(rad float64) float64 {
	return rad * (180.0 / math.Pi)
}

func mapFromAny(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case map[string]any:
		return v
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var decoded map[string]any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return decoded
		}
	}
	return nil
}

func getStringFromAnyMap(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	return getString(m[key])
}

func itemRewardString(p *Processor, itemID, amount int, tr *i18n.Translator) string {
	if itemID == 0 {
		return ""
	}
	name := ""
	if p != nil && p.data != nil {
		if raw, ok := p.data.Items[fmt.Sprintf("%d", itemID)]; ok {
			if m, ok := raw.(map[string]any); ok {
				if text, ok := m["name"].(string); ok {
					name = text
				}
			}
		}
	}
	if name == "" {
		name = fmt.Sprintf("Item %d", itemID)
	}
	if amount > 1 {
		return fmt.Sprintf("%d %s", amount, translateMaybe(tr, name))
	}
	return translateMaybe(tr, name)
}

func monsterName(p *Processor, pokemonID int) string {
	if pokemonID == 0 || p == nil || p.data == nil || p.data.Monsters == nil {
		return ""
	}
	if raw, ok := p.data.Monsters[fmt.Sprintf("%d_0", pokemonID)]; ok {
		if m, ok := raw.(map[string]any); ok {
			if name, ok := m["name"].(string); ok && name != "" {
				return name
			}
		}
	}
	if raw, ok := p.data.Monsters[fmt.Sprintf("%d", pokemonID)]; ok {
		if m, ok := raw.(map[string]any); ok {
			if name, ok := m["name"].(string); ok && name != "" {
				return name
			}
		}
	}
	for _, raw := range p.data.Monsters {
		if m, ok := raw.(map[string]any); ok {
			if getInt(m["id"]) == pokemonID {
				if name, ok := m["name"].(string); ok && name != "" {
					return name
				}
			}
		}
	}
	return fmt.Sprintf("Pokemon %d", pokemonID)
}

func monsterInfo(p *Processor, pokemonID, formID int) (string, string) {
	if p == nil || p.data == nil {
		return "", ""
	}
	monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return "", ""
	}
	name := getString(monster["name"])
	formName := ""
	if form, ok := monster["form"].(map[string]any); ok {
		formName = getString(form["name"])
	}
	return name, formName
}

func monsterFormName(p *Processor, pokemonID, formID int) string {
	if p == nil || p.data == nil {
		return ""
	}
	key := fmt.Sprintf("%d_%d", pokemonID, formID)
	monster := lookupMonster(p.data, key)
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return ""
	}
	if form, ok := monster["form"].(map[string]any); ok {
		if name, ok := form["name"].(string); ok {
			return name
		}
	}
	return ""
}

func monsterGeneration(p *Processor, pokemonID, formID int) (string, string, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil || pokemonID <= 0 {
		return "", "", ""
	}
	if exceptions, ok := p.data.UtilData["genException"].(map[string]any); ok {
		key := fmt.Sprintf("%d_%d", pokemonID, formID)
		if value, ok := exceptions[key]; ok {
			gen := fmt.Sprintf("%v", value)
			if name, roman := genDetails(p, gen); name != "" {
				return gen, name, roman
			}
			return gen, "", ""
		}
	}
	if genData, ok := p.data.UtilData["genData"].(map[string]any); ok {
		for genKey, raw := range genData {
			entry, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			min := getInt(entry["min"])
			max := getInt(entry["max"])
			if pokemonID >= min && pokemonID <= max {
				name, _ := entry["name"].(string)
				roman, _ := entry["roman"].(string)
				return genKey, name, roman
			}
		}
	}
	return "", "", ""
}

func genDetails(p *Processor, gen string) (string, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return "", ""
	}
	genData, ok := p.data.UtilData["genData"].(map[string]any)
	if !ok {
		return "", ""
	}
	entry, ok := genData[gen].(map[string]any)
	if !ok {
		return "", ""
	}
	name, _ := entry["name"].(string)
	roman, _ := entry["roman"].(string)
	return name, roman
}

func gruntTypeEmoji(p *Processor, gruntType any, platform string) string {
	// PoracleJS defaults gruntTypeEmoji to "grunt-unknown" and then overrides it when more specific
	// information is available (type emoji, event invasion emoji, etc.).
	_ = gruntType
	return lookupEmojiForPlatform(p, "grunt-unknown", platform)
}

func applyInvasionData(p *Processor, hook *Hook, data map[string]any, platform string, tr *i18n.Translator) {
	data["gruntRewardsList"] = map[string]any{}
	data["gruntRewards"] = ""
	data["gruntTypeColor"] = 0xBABABA
	data["gender"] = 0
	if stopName := getString(hook.Message["pokestop_name"]); stopName != "" {
		data["name"] = stopName
		data["pokestopName"] = stopName
	} else if stopName := getString(hook.Message["name"]); stopName != "" {
		data["name"] = stopName
		data["pokestopName"] = stopName
	}
	if p != nil && p.cfg != nil {
		url := getString(hook.Message["url"])
		if url == "" {
			url = getStringFromConfig(p.cfg, "fallbacks.pokestopUrl", "")
		}
		if url != "" {
			data["url"] = url
			data["pokestopUrl"] = url
		}
	}
	displayTypeID, gruntTypeID := resolveInvasionTypes(hook)
	rawGruntType := invasionRawGruntType(hook)
	if rawGruntType > 0 && displayTypeID >= 7 {
		displayTypeID = 0
	}
	data["displayTypeId"] = displayTypeID
	data["grunt_type"] = rawGruntType
	incidentExpiration := getInt64(hook.Message["incident_expiration"])
	if incidentExpiration == 0 {
		incidentExpiration = getInt64(hook.Message["incident_expire_timestamp"])
	}
	if incidentExpiration > 0 {
		data["incidentExpiration"] = incidentExpiration
	}
	data["gruntTypeId"] = gruntTypeID
	eventInvasion := isEventInvasion(hook, displayTypeID)
	if eventInvasion {
		if name, color, emojiKey := pokestopEventInfo(p, displayTypeID); name != "" {
			data["gruntName"] = translateMaybe(tr, name)
			data["gruntType"] = strings.ToLower(name)
			if color != "" {
				if parsed, err := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 32); err == nil {
					data["gruntTypeColor"] = int(parsed)
				}
			}
			if emojiKey != "" {
				if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
					data["gruntTypeEmoji"] = translateMaybe(tr, emoji)
				}
			}
		}
		data["gender"] = 0
	} else if gruntTypeID > 0 {
		data["gender"] = 0
		data["gruntName"] = translateMaybe(tr, "Grunt")
		data["gruntType"] = translateMaybe(tr, "Mixed")
		data["gruntRewards"] = ""
		if grunt := findGruntByID(p, gruntTypeID); grunt != nil {
			typeName := getString(grunt["type"])
			if strings.EqualFold(typeName, "Metal") {
				typeName = "Steel"
			}
			if typeName != "" {
				data["gruntType"] = translateMaybe(tr, typeName)
			}
			gruntLabel := getString(grunt["grunt"])
			if typeName != "" && gruntLabel != "" {
				data["gruntName"] = translateMaybe(tr, fmt.Sprintf("%s %s", typeName, gruntLabel))
			}
			if gender := getInt(grunt["gender"]); gender > 0 {
				data["gender"] = gender
			}
			if color, emojiKey := typeStyle(p, typeName); color != 0 {
				data["gruntTypeColor"] = color
				if emoji := lookupEmojiForPlatform(p, emojiKey, platform); emoji != "" {
					data["gruntTypeEmoji"] = translateMaybe(tr, emoji)
				}
			}
			if genderEng := genderDataEng(p, getInt(data["gender"])); genderEng != nil {
				data["genderDataEng"] = genderEng
				if name, ok := genderEng["name"].(string); ok {
					data["genderNameEng"] = name
				}
			} else {
				data["genderDataEng"] = map[string]any{"name": "", "emoji": ""}
				data["genderNameEng"] = ""
			}
			rewardText, rewardList := gruntRewardsDetails(p, grunt, tr)
			if rewardText != "" {
				data["gruntRewards"] = rewardText
			}
			if len(rewardList) > 0 {
				data["gruntRewardsList"] = rewardList
			}
		}
	}
	if _, ok := data["gruntType"]; !ok || getString(data["gruntType"]) == "" {
		data["gruntType"] = getString(hook.Message["grunt_type"])
	}
	if _, ok := data["gruntTypeEmoji"]; !ok || getString(data["gruntTypeEmoji"]) == "" {
		data["gruntTypeEmoji"] = gruntTypeEmoji(p, data["gruntType"], platform)
	}
	if _, ok := data["gruntTypeColor"]; !ok || getInt(data["gruntTypeColor"]) == 0 {
		data["gruntTypeColor"] = gruntTypeColor(data["gruntType"])
	}
	if lineup, ok := hook.Message["lineup"].([]any); ok {
		data["gruntLineupList"] = buildGruntLineupList(p, lineup, tr)
	}
}

func invasionDisplayType(hook *Hook) int {
	displayType, _ := resolveInvasionTypes(hook)
	return displayType
}

func invasionGruntTypeID(hook *Hook, displayTypeID int) int {
	_, gruntType := resolveInvasionTypes(hook)
	return gruntType
}

func resolveInvasionTypes(hook *Hook) (int, int) {
	if hook == nil {
		return 0, 0
	}
	displayType := 0
	if raw, ok := hook.Message["display_type"]; ok {
		displayType = getInt(raw)
	} else {
		displayType = getInt(hook.Message["incident_display_type"])
	}
	incidentGruntType := getInt(hook.Message["incident_grunt_type"])
	gruntType := getInt(hook.Message["grunt_type"])
	if incidentGruntType != 0 && incidentGruntType != 352 {
		return displayType, incidentGruntType
	}
	if gruntType != 0 && displayType <= 6 {
		return displayType, gruntType
	}
	if incidentGruntType == 352 {
		return 8, 0
	}
	return displayType, 0
}

func invasionRawGruntType(hook *Hook) int {
	if hook == nil {
		return 0
	}
	if getInt(hook.Message["incident_grunt_type"]) == 352 {
		return 0
	}
	return getInt(hook.Message["grunt_type"])
}

func isEventInvasion(hook *Hook, displayTypeID int) bool {
	return invasionRawGruntType(hook) == 0 && displayTypeID >= 7
}

func pokestopEventInfo(p *Processor, displayTypeID int) (string, string, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return "", "", ""
	}
	raw, ok := p.data.UtilData["pokestopEvent"].(map[string]any)
	if !ok {
		return "", "", ""
	}
	entry, ok := raw[strconv.Itoa(displayTypeID)].(map[string]any)
	if !ok {
		return "", "", ""
	}
	return getString(entry["name"]), getString(entry["color"]), getString(entry["emoji"])
}

func typeStyle(p *Processor, typeName string) (int, string) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return 0, ""
	}
	types, ok := p.data.UtilData["types"].(map[string]any)
	if !ok {
		return 0, ""
	}
	entry, ok := types[typeName].(map[string]any)
	if !ok {
		return 0, ""
	}
	colorStr := getString(entry["color"])
	emojiKey := getString(entry["emoji"])
	if colorStr != "" {
		if parsed, err := strconv.ParseInt(strings.TrimPrefix(colorStr, "#"), 16, 32); err == nil {
			return int(parsed), emojiKey
		}
	}
	return 0, emojiKey
}

func lookupEmoji(p *Processor, key string) string {
	return lookupEmojiForPlatform(p, key, "")
}

func lookupEmojiForPlatform(p *Processor, key string, platform string) string {
	if p == nil || p.data == nil || p.data.UtilData == nil || key == "" {
		return ""
	}
	if platform == "" && p.customEmoji != nil && len(p.customEmoji) == 1 {
		for name := range p.customEmoji {
			platform = name
			break
		}
	}
	if platform == "" && p.customEmoji != nil {
		if _, ok := p.customEmoji["discord"]; ok {
			platform = "discord"
		}
	}
	if platform != "" && p.customEmoji != nil {
		if platformMap, ok := p.customEmoji[platform]; ok {
			if val, ok := platformMap[key]; ok {
				return val
			}
		}
	}
	raw, ok := p.data.UtilData["emojis"].(map[string]any)
	if !ok {
		return ""
	}
	if value, ok := raw[key]; ok {
		return fmt.Sprintf("%v", value)
	}
	return ""
}

func genderDataEng(p *Processor, gender int) map[string]any {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return nil
	}
	raw, ok := p.data.UtilData["genders"].(map[string]any)
	if !ok {
		return nil
	}
	entry, ok := raw[fmt.Sprintf("%d", gender)].(map[string]any)
	if !ok {
		return nil
	}
	name, _ := entry["name"].(string)
	emoji, _ := entry["emoji"].(string)
	return map[string]any{"name": name, "emoji": emoji}
}

func gruntTypeColor(gruntType any) int {
	switch strings.ToLower(fmt.Sprintf("%v", gruntType)) {
	case "dragon":
		return 0x7038F8
	case "fire":
		return 0xF08030
	case "water":
		return 0x6890F0
	default:
		return 0x808080
	}
}

func gruntRewardsList(p *Processor, gruntType any, tr *i18n.Translator) map[string]any {
	out := map[string]any{}
	if p == nil || p.data == nil {
		return out
	}
	grunt := findGrunt(p, gruntType)
	if grunt == nil {
		return out
	}
	encounters, ok := grunt["encounters"].(map[string]any)
	if !ok {
		return out
	}
	first := rewardListFromEncounters(p, encounters["first"], tr)
	second := rewardListFromEncounters(p, encounters["second"], tr)
	if len(first) > 0 {
		out["first"] = map[string]any{
			"chance":   85,
			"monsters": first,
		}
	}
	if len(second) > 0 {
		out["second"] = map[string]any{
			"chance":   15,
			"monsters": second,
		}
	}
	return out
}

func findGrunt(p *Processor, gruntType any) map[string]any {
	if p == nil || p.data == nil {
		return nil
	}
	needle := strings.ToLower(fmt.Sprintf("%v", gruntType))
	for _, raw := range p.data.Grunts {
		if m, ok := raw.(map[string]any); ok {
			if typ, ok := m["type"].(string); ok && strings.ToLower(typ) == needle {
				return m
			}
		}
	}
	return nil
}

func findGruntByID(p *Processor, gruntTypeID int) map[string]any {
	if p == nil || p.data == nil || gruntTypeID <= 0 {
		return nil
	}
	raw, ok := p.data.Grunts[strconv.Itoa(gruntTypeID)]
	if !ok {
		return nil
	}
	grunt, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	return grunt
}

func gruntRewardsDetails(p *Processor, grunt map[string]any, tr *i18n.Translator) (string, map[string]any) {
	out := map[string]any{}
	if p == nil || p.data == nil || grunt == nil {
		return "", out
	}
	encounters, ok := grunt["encounters"].(map[string]any)
	if !ok {
		return "", out
	}
	secondReward := getBool(grunt["secondReward"])
	thirdReward := getBool(grunt["thirdReward"])
	firstList := rewardListFromEncountersDetailed(p, encounters[firstEncounterKey(thirdReward)], tr)
	secondList := rewardListFromEncountersDetailed(p, encounters["second"], tr)
	rewardText := ""
	if secondReward && len(firstList) > 0 && len(secondList) > 0 {
		out["first"] = map[string]any{"chance": 85, "monsters": firstList}
		out["second"] = map[string]any{"chance": 15, "monsters": secondList}
		rewardText = fmt.Sprintf("85%%: %s\\n15%%: %s", rewardNames(firstList), rewardNames(secondList))
		return rewardText, out
	}
	if len(firstList) > 0 {
		out["first"] = map[string]any{"chance": 100, "monsters": firstList}
		rewardText = rewardNames(firstList)
	}
	return rewardText, out
}

func firstEncounterKey(thirdReward bool) string {
	if thirdReward {
		return "third"
	}
	return "first"
}

func rewardNames(items []map[string]any) string {
	names := make([]string, 0, len(items))
	for _, item := range items {
		if name, ok := item["fullName"].(string); ok && name != "" {
			names = append(names, name)
		} else if name, ok := item["name"].(string); ok && name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func rewardListFromEncountersDetailed(p *Processor, raw any, tr *i18n.Translator) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := getInt(m["id"])
		formID := getInt(m["form"])
		nameEng, formNameEng := monsterInfo(p, id, formID)
		if nameEng == "" {
			nameEng = fmt.Sprintf("Pokemon %d", id)
		}
		name := translateMaybe(tr, nameEng)
		formName := translateMaybe(tr, formNameEng)
		fullName := name
		// Match PoracleJS: the "Normal" check is done on the English form name before translation.
		if formNameEng != "" && !strings.EqualFold(formNameEng, "Normal") {
			fullName = fmt.Sprintf("%s %s", formName, name)
		}
		out = append(out, map[string]any{
			"id":       id,
			"formId":   formID,
			"name":     name,
			"formName": formName,
			"fullName": fullName,
		})
	}
	return out
}

func buildGruntLineupList(p *Processor, lineup []any, tr *i18n.Translator) map[string]any {
	out := map[string]any{
		"confirmed": true,
		"monsters":  []map[string]any{},
	}
	monsters := make([]map[string]any, 0, len(lineup))
	for _, item := range lineup {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := getInt(m["pokemon_id"])
		formID := getInt(m["form"])
		nameEng, formNameEng := monsterInfo(p, id, formID)
		if nameEng == "" {
			nameEng = fmt.Sprintf("Pokemon %d", id)
		}
		name := translateMaybe(tr, nameEng)
		formName := translateMaybe(tr, formNameEng)
		fullName := name
		// Match PoracleJS: the "Normal" check is done on the English form name before translation.
		if formNameEng != "" && !strings.EqualFold(formNameEng, "Normal") {
			fullName = fmt.Sprintf("%s %s", formName, name)
		}
		monsters = append(monsters, map[string]any{
			"id":       id,
			"formId":   formID,
			"name":     name,
			"formName": formName,
			"fullName": fullName,
		})
	}
	out["monsters"] = monsters
	return out
}

func rewardListFromEncounters(p *Processor, raw any, tr *i18n.Translator) []map[string]any {
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		id := getInt(m["id"])
		out = append(out, map[string]any{
			"name": translateMaybe(tr, monsterName(p, id)),
		})
	}
	return out
}

func genderData(p *Processor, gender int, platform string, tr *i18n.Translator) map[string]any {
	// Match PoracleJS: gender labels + emoji keys come from utilData.genders and pass through emoji lookup.
	entry := genderDataEng(p, gender)
	if entry == nil {
		return map[string]any{"name": translateMaybe(tr, "Unknown"), "emoji": ""}
	}
	nameEng := getString(entry["name"])
	emojiKey := getString(entry["emoji"])
	emoji := ""
	if emojiKey != "" {
		emoji = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, platform))
	}
	return map[string]any{
		"name":  translateMaybe(tr, nameEng),
		"emoji": emoji,
	}
}

func lureTypeInfo(lureID int) (string, int) {
	switch lureID {
	case 501:
		return "Glacial", 0x00FFFF
	case 502:
		return "Mossy", 0x00FF7F
	case 503:
		return "Magnetic", 0xAAAAAA
	case 504:
		return "Rainy", 0x1E90FF
	case 505:
		return "Sparkly", 0xFF69B4
	default:
		return "Normal", 0x00FF00
	}
}

func lureTypeDetails(p *Processor, lureID int) (string, string, int) {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return "", "", 0
	}
	raw, ok := p.data.UtilData["lures"].(map[string]any)
	if !ok {
		return "", "", 0
	}
	entry, ok := raw[strconv.Itoa(lureID)].(map[string]any)
	if !ok {
		return "", "", 0
	}
	name := getString(entry["name"])
	emojiKey := getString(entry["emoji"])
	color := getString(entry["color"])
	if color != "" {
		if parsed, err := strconv.ParseInt(strings.TrimPrefix(color, "#"), 16, 32); err == nil {
			return name, emojiKey, int(parsed)
		}
	}
	return name, emojiKey, 0
}

func monsterTypes(p *Processor, pokemonID, formID int) []int {
	if p == nil || p.data == nil {
		return nil
	}
	monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return nil
	}
	raw, ok := monster["types"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]int, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, getInt(m["id"]))
		}
	}
	return out
}

func monsterTypeNames(p *Processor, pokemonID, formID int) []string {
	if p == nil || p.data == nil {
		return nil
	}
	monster := lookupMonster(p.data, fmt.Sprintf("%d_%d", pokemonID, formID))
	if monster == nil && formID != 0 {
		monster = lookupMonster(p.data, fmt.Sprintf("%d_0", pokemonID))
	}
	if monster == nil {
		monster = lookupMonster(p.data, fmt.Sprintf("%d", pokemonID))
	}
	if monster == nil {
		return nil
	}
	raw, ok := monster["types"].([]any)
	if !ok || len(raw) == 0 {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			if name := getString(m["name"]); name != "" {
				out = append(out, name)
			}
		}
	}
	return out
}

func caredPokemonFromHook(p *Processor, hook *Hook) *caredPokemon {
	if p == nil || hook == nil {
		return nil
	}
	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		return nil
	}
	formID := getInt(hook.Message["form"])
	if formID == 0 {
		formID = getInt(hook.Message["form_id"])
	}
	if formID == 0 {
		formID = getInt(hook.Message["pokemon_form"])
	}
	nameEng := monsterName(p, pokemonID)
	formName := monsterFormName(p, pokemonID, formID)
	fullName := nameEng
	if formName != "" && !strings.EqualFold(formName, "Normal") {
		fullName = fmt.Sprintf("%s %s", nameEng, formName)
	}
	weatherID := getInt(hook.Message["weather"])
	if boosted := getInt(hook.Message["boosted_weather"]); boosted > 0 {
		weatherID = boosted
	}
	if weatherID == 0 {
		weatherID = weatherCondition(hook.Message)
	}
	types := monsterTypes(p, pokemonID, formID)
	altering := alteringWeathers(p, types, weatherID)
	ivString := "-1"
	if hasNumeric(hook.Message["individual_attack"]) && hasNumeric(hook.Message["individual_defense"]) && hasNumeric(hook.Message["individual_stamina"]) {
		if ivValue := computeIV(hook); ivValue >= 0 {
			ivString = fmt.Sprintf("%.2f", ivValue)
		}
	}
	return &caredPokemon{
		PokemonID:        pokemonID,
		Form:             formID,
		Name:             nameEng,
		FormName:         formName,
		FullName:         fullName,
		IV:               ivString,
		CP:               getInt(hook.Message["cp"]),
		Latitude:         getFloat(hook.Message["latitude"]),
		Longitude:        getFloat(hook.Message["longitude"]),
		DisappearTime:    getInt64(hook.Message["disappear_time"]),
		AlteringWeathers: altering,
	}
}

func alteringWeathers(p *Processor, types []int, weather int) []int {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return nil
	}
	raw, ok := p.data.UtilData["weatherTypeBoost"].(map[string]any)
	if !ok || len(types) == 0 {
		return nil
	}
	boosting := []int{}
	for key, value := range raw {
		weatherID, err := strconv.Atoi(key)
		if err != nil {
			continue
		}
		items, ok := value.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			typeID := getInt(item)
			if containsInt(types, typeID) {
				boosting = append(boosting, weatherID)
				break
			}
		}
	}
	if weather > 0 {
		nonBoosting := []int{}
		for id := 1; id <= 7; id++ {
			if !containsInt(boosting, id) {
				nonBoosting = append(nonBoosting, id)
			}
		}
		return nonBoosting
	}
	return boosting
}

func weatherBoostsTypes(p *Processor, weatherID int, types []int) bool {
	if p == nil || p.data == nil || p.data.UtilData == nil || weatherID == 0 || len(types) == 0 {
		return false
	}
	raw, ok := p.data.UtilData["weatherTypeBoost"].(map[string]any)
	if !ok {
		return false
	}
	items, ok := raw[strconv.Itoa(weatherID)].([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if containsInt(types, getInt(item)) {
			return true
		}
	}
	return false
}

func boostingWeathersForTypes(p *Processor, types []int) []int {
	if p == nil || p.data == nil || p.data.UtilData == nil || len(types) == 0 {
		return nil
	}
	raw, ok := p.data.UtilData["weatherTypeBoost"].(map[string]any)
	if !ok {
		return nil
	}
	out := []int{}
	for _, typeID := range types {
		weatherID := 0
		for key, value := range raw {
			items, ok := value.([]any)
			if !ok {
				continue
			}
			for _, item := range items {
				if getInt(item) == typeID {
					if parsed, err := strconv.Atoi(key); err == nil {
						weatherID = parsed
					}
					break
				}
			}
			if weatherID > 0 {
				break
			}
		}
		if weatherID > 0 {
			out = append(out, weatherID)
		}
	}
	return out
}

func weaknessListForTypes(p *Processor, typeNames []string, platform string, tr *i18n.Translator) ([]map[string]any, string) {
	if p == nil || p.data == nil || len(typeNames) == 0 {
		return nil, ""
	}
	rawTypes := p.data.Types
	if rawTypes == nil {
		return nil, ""
	}
	utilTypes, ok := p.data.UtilData["types"].(map[string]any)
	if !ok {
		return nil, ""
	}
	weaknesses := map[string]float64{}
	for _, typeName := range typeNames {
		entry, ok := rawTypes[typeName].(map[string]any)
		if !ok {
			continue
		}
		for _, item := range []struct {
			key    string
			factor float64
		}{
			{"weaknesses", 2},
			{"resistances", 0.5},
			{"immunes", 0.25},
		} {
			rawList, ok := entry[item.key].([]any)
			if !ok {
				continue
			}
			for _, raw := range rawList {
				m, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				target := getString(m["typeName"])
				if target == "" {
					continue
				}
				if _, ok := weaknesses[target]; !ok {
					weaknesses[target] = 1
				}
				weaknesses[target] *= item.factor
			}
		}
	}
	typeObj := []struct {
		key   string
		value float64
		text  string
	}{
		{"extraWeak", 4, "Very vulnerable to"},
		{"weak", 2, "Vulnerable to"},
		{"resist", 0.5, "Resistant to"},
		{"immune", 0.25, "Very resistant to"},
		{"extraImmune", 0.125, "Extremely resistant to"},
	}
	list := []map[string]any{}
	weaknessEmoji := ""
	for _, entry := range typeObj {
		group := map[string]any{
			"value": entry.value,
			"text":  translateMaybe(tr, entry.text),
			"types": []map[string]any{},
		}
		types := group["types"].([]map[string]any)
		for name, value := range weaknesses {
			if value != entry.value {
				continue
			}
			emojiKey := ""
			if utilEntry, ok := utilTypes[name].(map[string]any); ok {
				emojiKey = getString(utilEntry["emoji"])
			}
			emoji := ""
			if emojiKey != "" {
				emoji = translateMaybe(tr, lookupEmojiForPlatform(p, emojiKey, platform))
			}
			types = append(types, map[string]any{
				"nameEng": name,
				"name":    translateMaybe(tr, name),
				"emoji":   emoji,
			})
		}
		if len(types) == 0 {
			continue
		}
		group["types"] = types
		typeEmoji := ""
		for _, entry := range types {
			typeEmoji += getString(entry["emoji"])
		}
		group["typeEmoji"] = typeEmoji
		weaknessEmoji = weaknessEmoji + fmt.Sprintf("%gx%s ", entry.value, typeEmoji)
		list = append(list, group)
	}
	return list, strings.TrimSpace(weaknessEmoji)
}

func rarityGroupForPokemon(tracker *stats.Tracker, pokemonID int) int {
	if tracker == nil || pokemonID <= 0 {
		return -1
	}
	report, ok := tracker.LatestReport()
	if !ok {
		return -1
	}
	for group, ids := range report.Rarity {
		for _, id := range ids {
			if id == pokemonID {
				return group
			}
		}
	}
	return -1
}

func rarityNameEng(p *Processor, group int) string {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return ""
	}
	raw, ok := p.data.UtilData["rarity"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := raw[strconv.Itoa(group)].(string)
	return name
}

func sizeNameEng(p *Processor, size int) string {
	if p == nil || p.data == nil || p.data.UtilData == nil {
		return ""
	}
	raw, ok := p.data.UtilData["size"].(map[string]any)
	if !ok {
		return ""
	}
	name, _ := raw[strconv.Itoa(size)].(string)
	return name
}

func shinyStatsForPokemon(tracker *stats.Tracker, pokemonID int) any {
	if tracker == nil || pokemonID <= 0 {
		return nil
	}
	report, ok := tracker.LatestReport()
	if !ok {
		return nil
	}
	stat, ok := report.Shiny[pokemonID]
	if !ok || stat.Ratio <= 0 {
		return nil
	}
	return fmt.Sprintf("%.0f", stat.Ratio)
}

type pvpBestSummary struct {
	rank int
	cp   int
}

func pvpCapsFromConfig(cfg *config.Config) []int {
	if cfg == nil {
		return []int{50}
	}
	if raw, ok := cfg.Get("pvp.levelCaps"); ok {
		if list := parseIntSlice(raw); len(list) > 0 {
			return list
		}
	}
	return []int{50}
}

func parseIntSlice(raw any) []int {
	switch v := raw.(type) {
	case []int:
		return v
	case []any:
		out := make([]int, 0, len(v))
		for _, item := range v {
			if num := getInt(item); num > 0 {
				out = append(out, num)
			}
		}
		return out
	case []string:
		out := make([]int, 0, len(v))
		for _, item := range v {
			if parsed, err := strconv.Atoi(strings.TrimSpace(item)); err == nil {
				out = append(out, parsed)
			}
		}
		return out
	default:
		return nil
	}
}

func pvpRankSummary(capsConsidered []int, league int, raw any, pokemonID int, evoEnabled bool, minCp int, maxRank int, pvpEvolution map[string]any) ([]map[string]any, pvpBestSummary) {
	best := map[int]pvpBestSummary{}
	for _, cap := range capsConsidered {
		best[cap] = pvpBestSummary{rank: 4096, cp: 0}
	}

	entries := pvpAnySlice(raw)
	if len(entries) == 0 {
		return summarizeBestRanks(best)
	}

	for _, item := range entries {
		stats, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rank := getInt(stats["rank"])
		if rank == 0 {
			continue
		}
		cp := getInt(stats["cp"])
		caps := pvpCapsForEntry(stats, capsConsidered)
		for _, cap := range caps {
			details := best[cap]
			if rank < details.rank {
				details.rank = rank
				details.cp = cp
				best[cap] = details
			} else if rank == details.rank && cp > details.cp {
				details.cp = cp
				best[cap] = details
			}
		}

		if !evoEnabled {
			continue
		}
		if getBool(stats["evolution"]) {
			continue
		}
		statsPokemon := getInt(stats["pokemon"])
		if statsPokemon == 0 || statsPokemon == pokemonID {
			continue
		}
		if rank > maxRank || cp < minCp {
			continue
		}
		entry := map[string]any{
			"rank":       rank,
			"percentage": getFloat(stats["percentage"]),
			"pokemon":    statsPokemon,
			"form":       getInt(stats["form"]),
			"level":      getFloat(stats["level"]),
			"cp":         cp,
			"caps":       caps,
		}
		addPvpEvolutionEntry(pvpEvolution, statsPokemon, league, entry)
	}

	return summarizeBestRanks(best)
}

func pvpCapsForEntry(stats map[string]any, capsConsidered []int) []int {
	capVal := getInt(stats["cap"])
	capped := getBool(stats["capped"])
	if capVal == 0 && !capped {
		return []int{50}
	}
	if capped {
		out := []int{}
		for _, cap := range capsConsidered {
			if cap >= capVal {
				out = append(out, cap)
			}
		}
		return out
	}
	if capVal > 0 && containsInt(capsConsidered, capVal) {
		return []int{capVal}
	}
	return nil
}

func summarizeBestRanks(best map[int]pvpBestSummary) ([]map[string]any, pvpBestSummary) {
	bestRanks := []map[string]any{}
	summary := pvpBestSummary{rank: 4096, cp: 0}
	for cap, details := range best {
		if details.rank < summary.rank {
			summary.rank = details.rank
		}
		if summary.cp == 0 || details.cp < summary.cp {
			summary.cp = details.cp
		}
		found := false
		for _, existing := range bestRanks {
			if getInt(existing["rank"]) == details.rank && getInt(existing["cp"]) == details.cp {
				if caps, ok := existing["caps"].([]int); ok {
					existing["caps"] = append(caps, cap)
				} else if caps, ok := existing["caps"].([]any); ok {
					existing["caps"] = append(caps, cap)
				} else {
					existing["caps"] = []int{cap}
				}
				found = true
				break
			}
		}
		if !found {
			bestRanks = append(bestRanks, map[string]any{
				"rank": details.rank,
				"cp":   details.cp,
				"caps": []int{cap},
			})
		}
	}
	return bestRanks, summary
}

func addPvpEvolutionEntry(pvpEvolution map[string]any, pokemonID int, league int, entry map[string]any) {
	key := strconv.Itoa(pokemonID)
	leagueKey := strconv.Itoa(league)
	raw, ok := pvpEvolution[key]
	if !ok {
		pvpEvolution[key] = map[string]any{leagueKey: []map[string]any{entry}}
		return
	}
	leagueMap, ok := raw.(map[string]any)
	if !ok {
		pvpEvolution[key] = map[string]any{leagueKey: []map[string]any{entry}}
		return
	}
	if list, ok := leagueMap[leagueKey].([]map[string]any); ok {
		leagueMap[leagueKey] = append(list, entry)
		return
	}
	if list, ok := leagueMap[leagueKey].([]any); ok {
		leagueMap[leagueKey] = append(list, entry)
		return
	}
	leagueMap[leagueKey] = []map[string]any{entry}
}

func buildRdmURL(cfg *config.Config, hook *Hook, lat, lon float64) string {
	if cfg == nil || hook == nil {
		return ""
	}
	base := getStringFromConfig(cfg, "general.rdmURL", "")
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	switch hook.Type {
	case "pokemon":
		if encounter := getString(hook.Message["encounter_id"]); encounter != "" {
			return base + "@pokemon/" + encounter
		}
	case "raid", "egg", "gym", "gym_details":
		if gym := getString(hook.Message["gym_id"]); gym != "" {
			return base + "@gym/" + gym
		}
		if gym := getString(hook.Message["id"]); gym != "" {
			return base + "@gym/" + gym
		}
	case "quest", "invasion", "lure", "pokestop":
		if stop := getString(hook.Message["pokestop_id"]); stop != "" {
			return base + "@pokestop/" + stop
		}
	case "nest", "fort_update":
		if lat != 0 && lon != 0 {
			return fmt.Sprintf("%s@%f/@%f/18", base, lat, lon)
		}
	}
	return ""
}

func rocketMadURL(cfg *config.Config, lat, lon float64) string {
	if cfg == nil || lat == 0 || lon == 0 {
		return ""
	}
	base := getStringFromConfig(cfg, "general.rocketMadURL", "")
	if base == "" {
		return ""
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return fmt.Sprintf("%s?lat=%f&lon=%f&zoom=18.0", base, lat, lon)
}

func pvpRankingList(p *Processor, hook *Hook, key string, tr *i18n.Translator) []map[string]any {
	raw := hook.Message[key]
	items := pvpAnySlice(raw)
	if len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pokemonID := getInt(m["pokemon"])
		if pokemonID == 0 {
			pokemonID = getInt(m["pokemon_id"])
		}
		out = append(out, map[string]any{
			"fullName":     translateMaybe(tr, monsterName(p, pokemonID)),
			"rank":         getInt(m["rank"]),
			"cp":           getInt(m["cp"]),
			"level":        getFloat(m["level"]),
			"levelWithCap": getFloat(m["level"]),
		})
	}
	return out
}

type pvpFilter struct {
	League int
	Worst  int
	Cap    int
}

func pvpFiltersFromRow(row map[string]any) []pvpFilter {
	if row == nil {
		return nil
	}
	worst := getInt(row["pvp_ranking_worst"])
	if worst <= 0 || worst >= 4096 {
		return nil
	}
	return []pvpFilter{{
		League: getInt(row["pvp_ranking_league"]),
		Worst:  worst,
		Cap:    getInt(row["pvp_ranking_cap"]),
	}}
}

func pvpDisplayList(p *Processor, raw any, leagueCap int, maxRank int, minCp int, filters []pvpFilter, filterByTrack bool, tr *i18n.Translator) []map[string]any {
	items := pvpAnySlice(raw)
	if len(items) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		rank := getInt(m["rank"])
		cp := getInt(m["cp"])
		if rank == 0 || rank > maxRank || cp < minCp {
			continue
		}
		pokemonID := getInt(m["pokemon"])
		if pokemonID == 0 {
			pokemonID = getInt(m["pokemon_id"])
		}
		if pokemonID == 0 {
			continue
		}
		formID := getInt(m["form"])
		if formID == 0 {
			formID = getInt(m["form_id"])
		}
		evolutionID := getInt(m["evolution"])
		level := getFloat(m["level"])
		cap := getInt(m["cap"])
		capped := getBool(m["capped"])
		levelWithCap := any(level)
		if cap > 0 && !capped {
			levelWithCap = fmt.Sprintf("%.0f/%d", level, cap)
		}
		percentage := getFloat(m["percentage"])
		if percentage <= 1 {
			percentage = percentage * 100
		}
		stats := map[string]any{
			"baseAttack":  0,
			"baseDefense": 0,
			"baseStamina": 0,
		}
		if found, ok := lookupMonsterStats(p, pokemonID, formID); ok {
			stats = found
		}
		nameEng := monsterName(p, pokemonID)
		formEng := monsterFormName(p, pokemonID, formID)
		if nameEng == "" || strings.HasPrefix(nameEng, "Pokemon ") {
			nameEng = fmt.Sprintf("%s %d", translateMaybe(tr, "Unknown monster"), pokemonID)
		}
		if strings.EqualFold(formEng, "Normal") {
			formEng = ""
		}
		name := translateMaybe(tr, nameEng)
		form := translateMaybe(tr, formEng)
		fullNameEng := joinNonEmptyWithSep([]string{nameEng, formEng}, " ")
		fullName := joinNonEmptyWithSep([]string{name, form}, " ")
		if evolutionID != 0 {
			if format := megaNameFormat(p, evolutionID); format != "" {
				fullNameEng = formatTemplate(format, fullNameEng)
				fullName = formatTemplate(format, fullName)
			}
		}
		passesFilter := true
		matchesUserTrack := false
		if len(filters) > 0 {
			passesFilter = false
			for _, filter := range filters {
				leagueMatch := filter.League == 0 || filter.League == leagueCap
				capMatch := filter.Cap == 0 || filter.Cap == cap || capped
				if leagueMatch && capMatch && filter.Worst >= rank {
					passesFilter = true
					matchesUserTrack = true
					break
				}
			}
		}
		displayRank := map[string]any{
			"rank":             rank,
			"formId":           formID,
			"evolution":        evolutionID,
			"level":            level,
			"cap":              cap,
			"capped":           capped,
			"levelWithCap":     levelWithCap,
			"cp":               cp,
			"pokemonId":        pokemonID,
			"percentage":       fmt.Sprintf("%.2f", percentage),
			"baseStats":        stats,
			"nameEng":          nameEng,
			"formEng":          formEng,
			"name":             name,
			"form":             form,
			"fullNameEng":      fullNameEng,
			"fullName":         fullName,
			"passesFilter":     passesFilter,
			"matchesUserTrack": matchesUserTrack,
		}
		if !filterByTrack || passesFilter {
			out = append(out, displayRank)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func pvpBestInfo(raw any) map[string]any {
	if raw == nil {
		return nil
	}
	list, ok := raw.([]map[string]any)
	if !ok {
		if items, ok := raw.([]any); ok {
			parsed := make([]map[string]any, 0, len(items))
			for _, item := range items {
				if m, ok := item.(map[string]any); ok {
					parsed = append(parsed, m)
				}
			}
			list = parsed
		}
	}
	if len(list) == 0 {
		return nil
	}
	bestRank := 4096
	bestList := []map[string]any{}
	for _, entry := range list {
		rank := getInt(entry["rank"])
		if rank == bestRank {
			bestList = append(bestList, entry)
		} else if rank < bestRank {
			bestRank = rank
			bestList = []map[string]any{entry}
		}
	}
	if len(bestList) == 0 {
		return nil
	}
	nameSet := map[string]bool{}
	nameEngSet := map[string]bool{}
	names := []string{}
	namesEng := []string{}
	for _, entry := range bestList {
		if name := getString(entry["fullName"]); name != "" && !nameSet[name] {
			nameSet[name] = true
			names = append(names, name)
		}
		if name := getString(entry["fullNameEng"]); name != "" && !nameEngSet[name] {
			nameEngSet[name] = true
			namesEng = append(namesEng, name)
		}
	}
	return map[string]any{
		"rank":    bestRank,
		"list":    bestList,
		"name":    strings.Join(names, ", "),
		"nameEng": strings.Join(namesEng, ", "),
	}
}

func defaultTranslator(p *Processor, hook *Hook) *i18nTranslator {
	language := getString(hook.Message["language"])
	if language == "" && p.cfg != nil {
		if v, ok := p.cfg.GetString("general.locale"); ok {
			language = v
		}
	}
	return &i18nTranslator{factory: p.i18n, language: language}
}

type i18nTranslator struct {
	factory  *i18n.Factory
	language string
}

func (t *i18nTranslator) Translate(key string, fallback bool) string {
	if t.factory == nil {
		return key
	}
	tr := t.factory.Translator(t.language)
	return tr.Translate(key, fallback)
}

func rowMatchesHook(p *Processor, hook *Hook, row map[string]any) bool {
	switch hook.Type {
	case "pokemon":
		return matchPokemon(p, hook, row)
	case "raid":
		return matchRaid(hook, row)
	case "egg":
		return matchEgg(hook, row)
	case "max_battle":
		return matchMaxBattle(hook, row)
	case "quest":
		return matchQuest(hook, row)
	case "invasion":
		return matchInvasion(hook, row)
	case "lure":
		return matchLure(hook, row)
	case "nest":
		return matchNest(hook, row)
	case "gym", "gym_details":
		return matchGym(hook, row)
	case "weather":
		return matchWeather(hook, row)
	case "fort_update":
		return matchFort(hook, row)
	default:
		return false
	}
}

func matchPokemon(p *Processor, hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["pokemon_id"])
	trackedID := getInt(row["pokemon_id"])
	league := getInt(row["pvp_ranking_league"])
	trackedForm := getInt(row["form"])
	spawnForm := getInt(hook.Message["form"])
	evoEnabled := false
	if p != nil && p.cfg != nil {
		evoEnabled, _ = p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")
	}
	if trackedID != 0 && trackedID != pokemonID {
		if league == 0 || !evoEnabled {
			return false
		}
	} else {
		if trackedForm > 0 && spawnForm != trackedForm {
			return false
		}
	}
	if league > 0 {
		if !matchPvpLeague(p, hook, row, league, pokemonID, spawnForm, trackedID, trackedForm) {
			return false
		}
	}
	iv := computeIV(hook)
	minIV := getInt(row["min_iv"])
	maxIV := getInt(row["max_iv"])
	if iv < float64(minIV) {
		return false
	}
	if iv > float64(maxIV) {
		return false
	}
	cp := getInt(hook.Message["cp"])
	if min := getInt(row["min_cp"]); cp < min {
		return false
	}
	if max := getInt(row["max_cp"]); cp > max {
		return false
	}
	level := getInt(hook.Message["pokemon_level"])
	if min := getInt(row["min_level"]); level < min {
		return false
	}
	if max := getInt(row["max_level"]); level > max {
		return false
	}
	atk := ivStatValue(hook.Message["individual_attack"])
	def := ivStatValue(hook.Message["individual_defense"])
	sta := ivStatValue(hook.Message["individual_stamina"])
	if minAtk := getInt(row["atk"]); atk < minAtk {
		return false
	}
	if minDef := getInt(row["def"]); def < minDef {
		return false
	}
	if minSta := getInt(row["sta"]); sta < minSta {
		return false
	}
	if maxAtk := getInt(row["max_atk"]); atk > maxAtk {
		return false
	}
	if maxDef := getInt(row["max_def"]); def > maxDef {
		return false
	}
	if maxSta := getInt(row["max_sta"]); sta > maxSta {
		return false
	}
	if minTime := getInt(row["min_time"]); minTime > 0 {
		expire := hookExpiryUnix(hook)
		if expire == 0 {
			return false
		}
		remaining := int(time.Until(time.Unix(expire, 0)).Seconds())
		if remaining < 0 {
			remaining = 0
		}
		if remaining < minTime {
			return false
		}
	}
	weight := int(getFloat(hook.Message["weight"]) * 1000)
	if minWeight := getInt(row["min_weight"]); weight < minWeight {
		return false
	}
	if maxWeight := getInt(row["max_weight"]); weight > maxWeight {
		return false
	}
	size := getInt(hook.Message["size"])
	if minSize := getInt(row["size"]); minSize >= 0 && size < minSize {
		return false
	}
	if maxSize := getInt(row["max_size"]); size > maxSize {
		return false
	}
	if gender := getInt(row["gender"]); gender > 0 && getInt(hook.Message["gender"]) != gender {
		return false
	}
	if p != nil {
		rarityGroup := getInt(hook.Message["rarityGroup"])
		if rarityGroup == 0 {
			rarityGroup = getInt(hook.Message["rarity_group"])
		}
		if rarityGroup == 0 {
			rarityGroup = rarityGroupForPokemon(p.stats, pokemonID)
		}
		minRarity := getInt(row["rarity"])
		maxRarity := getInt(row["max_rarity"])
		if rarityGroup < minRarity {
			return false
		}
		if rarityGroup > maxRarity {
			return false
		}
	}
	return true
}

type pvpEntry struct {
	PokemonID int
	FormID    int
	Rank      int
	CP        int
	Caps      []int
	Evolution bool
}

func matchPvpLeague(p *Processor, hook *Hook, row map[string]any, league int, spawnPokemonID int, spawnFormID int, trackedPokemonID int, trackedFormID int) bool {
	capsConsidered := pvpCapsFromConfig(nil)
	if p != nil {
		capsConsidered = pvpCapsFromConfig(p.cfg)
	}
	entries := pvpEntriesFromHookWithCaps(hook, league, capsConsidered)
	if len(entries) == 0 {
		return false
	}
	pvpQueryLimit := 100
	evoEnabled := false
	evoMinCP := 0
	if p != nil && p.cfg != nil {
		if limit, ok := p.cfg.GetInt("pvp.pvpQueryMaxRank"); ok && limit > 0 {
			pvpQueryLimit = limit
		} else if limit, ok := p.cfg.GetInt("pvp.pvpFilterMaxRank"); ok && limit > 0 {
			pvpQueryLimit = limit
		}
		evoEnabled, _ = p.cfg.GetBool("pvp.pvpEvolutionDirectTracking")
		switch league {
		case 1500:
			evoMinCP = getIntFromConfig(p.cfg, "pvp.pvpFilterGreatMinCP", 0)
		case 2500:
			evoMinCP = getIntFromConfig(p.cfg, "pvp.pvpFilterUltraMinCP", 0)
		case 500:
			evoMinCP = getIntFromConfig(p.cfg, "pvp.pvpFilterLittleMinCP", 0)
		}
	}
	rowCap := getInt(row["pvp_ranking_cap"])
	minCp := getInt(row["pvp_ranking_min_cp"])
	best := getInt(row["pvp_ranking_best"])
	worst := getInt(row["pvp_ranking_worst"])

	for _, entry := range entries {
		if entry.Rank > pvpQueryLimit {
			continue
		}
		switch {
		case trackedPokemonID == 0:
			if entry.PokemonID != spawnPokemonID || entry.FormID != spawnFormID {
				continue
			}
		case trackedPokemonID == spawnPokemonID:
			if entry.PokemonID != spawnPokemonID || entry.FormID != spawnFormID {
				continue
			}
		default:
			if !evoEnabled {
				continue
			}
			if entry.Evolution {
				continue
			}
			if entry.PokemonID != trackedPokemonID {
				continue
			}
			if trackedFormID > 0 && entry.FormID != trackedFormID {
				continue
			}
			if evoMinCP > 0 && entry.CP < evoMinCP {
				continue
			}
		}
		if entry.Rank > worst || entry.Rank < best {
			continue
		}
		if entry.CP < minCp {
			continue
		}
		if rowCap > 0 && len(entry.Caps) > 0 && !containsInt(entry.Caps, rowCap) {
			continue
		}
		return true
	}
	return false
}

func pvpEntriesFromHookWithCaps(hook *Hook, league int, capsConsidered []int) []pvpEntry {
	key := pvpLeagueKeyForMatch(league)
	if key == "" {
		return nil
	}
	raw := hook.Message[key]
	items := pvpAnySlice(raw)
	if len(items) == 0 {
		return nil
	}
	out := []pvpEntry{}
	for _, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pokemonID := getInt(m["pokemon"])
		if pokemonID == 0 {
			pokemonID = getInt(m["pokemon_id"])
		}
		entry := pvpEntry{
			PokemonID: pokemonID,
			FormID:    getInt(m["form"]),
			Rank:      getInt(m["rank"]),
			CP:        getInt(m["cp"]),
			Evolution: getBool(m["evolution"]),
		}
		if caps, ok := m["caps"].([]any); ok {
			for _, cap := range caps {
				entry.Caps = append(entry.Caps, getInt(cap))
			}
		} else if cap := getInt(m["cap"]); cap > 0 || getBool(m["capped"]) {
			entry.Caps = pvpCapsForEntry(m, capsConsidered)
		}
		out = append(out, entry)
	}
	return out
}

func pvpLeagueKeyForMatch(league int) string {
	switch league {
	case 1500:
		return "pvp_rankings_great_league"
	case 2500:
		return "pvp_rankings_ultra_league"
	case 500:
		return "pvp_rankings_little_league"
	default:
		return ""
	}
}

func normalizePvpRankings(hook *Hook) {
	if hook == nil || hook.Message == nil {
		return
	}
	pvpRaw := hook.Message["pvp"]
	ohbemRaw := hook.Message["ohbem_pvp"]
	for _, league := range []int{500, 1500, 2500} {
		key := pvpLeagueKeyForMatch(league)
		if key == "" || hook.Message[key] != nil {
			continue
		}
		name := pvpLeagueName(league)
		if name == "" {
			continue
		}
		if list := pvpListFromMap(pvpRaw, name); len(list) > 0 {
			hook.Message[key] = list
			continue
		}
		if list := pvpListFromMap(ohbemRaw, name); len(list) > 0 {
			hook.Message[key] = list
			continue
		}
	}
}

func pvpLeagueName(league int) string {
	switch league {
	case 1500:
		return "great"
	case 2500:
		return "ultra"
	case 500:
		return "little"
	default:
		return ""
	}
}

func pvpListFromMap(raw any, leagueName string) []any {
	if leagueName == "" {
		return nil
	}
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	if list := pvpAnySlice(m[leagueName]); len(list) > 0 {
		return list
	}
	if list := pvpAnySlice(m[leagueName+"_league"]); len(list) > 0 {
		return list
	}
	return nil
}

func pvpAnySlice(raw any) []any {
	switch v := raw.(type) {
	case []any:
		return v
	case []map[string]any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = v[i]
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		var decoded []any
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			return decoded
		}
		var decodedMap []map[string]any
		if err := json.Unmarshal([]byte(v), &decodedMap); err == nil {
			out := make([]any, len(decodedMap))
			for i := range decodedMap {
				out[i] = decodedMap[i]
			}
			return out
		}
	case []byte:
		if len(v) == 0 {
			return nil
		}
		var decoded []any
		if err := json.Unmarshal(v, &decoded); err == nil {
			return decoded
		}
		var decodedMap []map[string]any
		if err := json.Unmarshal(v, &decodedMap); err == nil {
			out := make([]any, len(decodedMap))
			for i := range decodedMap {
				out[i] = decodedMap[i]
			}
			return out
		}
	}
	return nil
}

func containsInt(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func computeIV(hook *Hook) float64 {
	rawAtk, okAtk := hook.Message["individual_attack"]
	rawDef, okDef := hook.Message["individual_defense"]
	rawSta, okSta := hook.Message["individual_stamina"]
	if !okAtk || !okDef || !okSta {
		return -1
	}
	if !hasNumeric(rawAtk) || !hasNumeric(rawDef) || !hasNumeric(rawSta) {
		return -1
	}
	atk := int(toFloat(rawAtk))
	def := int(toFloat(rawDef))
	sta := int(toFloat(rawSta))
	return float64(atk+def+sta) / 45.0 * 100.0
}

func hasNumeric(value any) bool {
	switch v := value.(type) {
	case int, int64, float64, float32, json.Number:
		return toFloat(v) >= 0
	case string:
		if strings.TrimSpace(v) == "" {
			return false
		}
		parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return err == nil && parsed >= 0
	default:
		return false
	}
}

func ivStatValue(value any) int {
	if !hasNumeric(value) {
		return 0
	}
	if v := int(toFloat(value)); v > 0 {
		return v
	}
	return 0
}

func matchRaid(hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["pokemon_id"])
	tracked := getInt(row["pokemon_id"])
	if tracked != 9000 && tracked != pokemonID {
		return false
	}
	level := getInt(hook.Message["level"])
	if level == 0 {
		level = getInt(hook.Message["raid_level"])
	}
	if tracked == 9000 {
		trackedLevel := getInt(row["level"])
		if trackedLevel != 90 && trackedLevel != 9000 && trackedLevel != level {
			return false
		}
	}
	team := teamFromHookMessage(hook.Message)
	if trackedTeam := getInt(row["team"]); trackedTeam != 4 && trackedTeam != team {
		return false
	}
	if exclusive := getInt(row["exclusive"]); exclusive > 0 {
		hookExclusive := getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
		if !hookExclusive {
			return false
		}
	}
	form := getInt(hook.Message["form"])
	if form == 0 {
		form = getInt(hook.Message["form_id"])
	}
	if form == 0 {
		form = getInt(hook.Message["pokemon_form"])
	}
	if trackedForm := getInt(row["form"]); trackedForm > 0 && form != trackedForm {
		return false
	}
	if evolution := getInt(row["evolution"]); evolution != 9000 {
		hookEvolution := getInt(hook.Message["evolution"])
		if hookEvolution == 0 {
			hookEvolution = getInt(hook.Message["evolution_id"])
		}
		if hookEvolution != evolution {
			return false
		}
	}
	if move := getInt(row["move"]); move != 9000 {
		move1 := getInt(hook.Message["move_1"])
		move2 := getInt(hook.Message["move_2"])
		if move != move1 && move != move2 {
			return false
		}
	}
	rowGymID := getString(row["gym_id"])
	if rowGymID != "" {
		hookGymID := getString(hook.Message["gym_id"])
		if hookGymID == "" {
			hookGymID = getString(hook.Message["id"])
		}
		if rowGymID != hookGymID {
			return false
		}
	}
	return true
}

func matchEgg(hook *Hook, row map[string]any) bool {
	level := getInt(hook.Message["level"])
	if trackedLevel := getInt(row["level"]); trackedLevel != 90 && trackedLevel != level {
		return false
	}
	team := teamFromHookMessage(hook.Message)
	if trackedTeam := getInt(row["team"]); trackedTeam != 4 && trackedTeam != team {
		return false
	}
	if exclusive := getInt(row["exclusive"]); exclusive > 0 {
		hookExclusive := getBool(hook.Message["is_exclusive"]) || getBool(hook.Message["exclusive"])
		if !hookExclusive {
			return false
		}
	}
	rowGymID := getString(row["gym_id"])
	if rowGymID != "" {
		hookGymID := getString(hook.Message["gym_id"])
		if hookGymID == "" {
			hookGymID = getString(hook.Message["id"])
		}
		if rowGymID != hookGymID {
			return false
		}
	}
	return true
}

func matchMaxBattle(hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["battle_pokemon_id"])
	if pokemonID == 0 {
		pokemonID = getInt(hook.Message["pokemon_id"])
	}
	tracked := getInt(row["pokemon_id"])
	if tracked != 9000 && tracked != pokemonID {
		return false
	}

	level := getInt(hook.Message["battle_level"])
	if level == 0 {
		level = getInt(hook.Message["level"])
	}
	if tracked == 9000 {
		trackedLevel := getInt(row["level"])
		if trackedLevel != 90 && trackedLevel != level {
			return false
		}
	}

	gmax := getInt(hook.Message["gmax"])
	if gmax == 0 && level > 6 {
		gmax = 1
	}
	if trackedGmax := getInt(row["gmax"]); trackedGmax != 0 && trackedGmax != gmax {
		return false
	}

	form := getInt(hook.Message["battle_pokemon_form"])
	if form == 0 {
		form = getInt(hook.Message["form"])
	}
	if trackedForm := getInt(row["form"]); trackedForm != 0 && trackedForm != form {
		return false
	}

	if evolution := getInt(row["evolution"]); evolution != 9000 {
		hookEvolution := getInt(hook.Message["evolution"])
		if hookEvolution == 0 {
			hookEvolution = getInt(hook.Message["evolution_id"])
		}
		if hookEvolution != evolution {
			return false
		}
	}

	if move := getInt(row["move"]); move != 9000 {
		move1 := getInt(hook.Message["battle_pokemon_move_1"])
		if move1 == 0 {
			move1 = getInt(hook.Message["move_1"])
		}
		move2 := getInt(hook.Message["battle_pokemon_move_2"])
		if move2 == 0 {
			move2 = getInt(hook.Message["move_2"])
		}
		if move != move1 && move != move2 {
			return false
		}
	}

	return true
}

func matchQuest(hook *Hook, row map[string]any) bool {
	rewardType := getInt(hook.Message["reward_type"])
	if rewardType == 0 {
		rewardType = getInt(hook.Message["reward_type_id"])
	}
	if tracked := getInt(row["reward_type"]); tracked > 0 && tracked != rewardType {
		return false
	}
	reward := getInt(hook.Message["reward"])
	if reward == 0 {
		reward = getInt(hook.Message["pokemon_id"])
	}
	if tracked := getInt(row["reward"]); tracked > 0 && tracked != reward {
		return false
	}
	if amount := getInt(row["amount"]); amount > 0 && getInt(hook.Message["amount"]) != amount {
		return false
	}
	if form := getInt(row["form"]); form > 0 && getInt(hook.Message["form"]) != form {
		return false
	}
	return true
}

func matchQuestWithData(hook *Hook, row map[string]any, rewardData map[string]any) bool {
	if hook == nil {
		return false
	}
	if rewardData == nil {
		return matchQuest(hook, row)
	}
	rewardType := getInt(row["reward_type"])
	reward := getInt(row["reward"])
	form := getInt(row["form"])
	amount := getInt(row["amount"])
	shiny := getInt(row["shiny"])

	switch rewardType {
	case 2:
		items, _ := rewardData["items"].([]map[string]any)
		for _, item := range items {
			itemID := getInt(item["id"])
			itemAmount := getInt(item["amount"])
			if reward > 0 && reward != itemID {
				continue
			}
			if amount > 0 && itemAmount < amount {
				continue
			}
			return true
		}
	case 3:
		dustAmount := getInt(rewardData["dustAmount"])
		if dustAmount == 0 {
			return false
		}
		if reward > 0 && reward > dustAmount {
			return false
		}
		return true
	case 4:
		candy, _ := rewardData["candy"].([]map[string]any)
		for _, entry := range candy {
			monID := getInt(entry["pokemonId"])
			entryAmount := getInt(entry["amount"])
			if reward > 0 && reward != monID {
				continue
			}
			if amount > 0 && entryAmount < amount {
				continue
			}
			return true
		}
	case 7:
		monsters, _ := rewardData["monsters"].([]map[string]any)
		for _, monster := range monsters {
			monID := getInt(monster["pokemonId"])
			formID := getInt(monster["formId"])
			isShiny := getBool(monster["shiny"])
			if reward > 0 && reward != monID {
				continue
			}
			if form > 0 && form != formID {
				continue
			}
			if shiny == 1 && !isShiny {
				continue
			}
			return true
		}
	case 12:
		energy, _ := rewardData["energyMonsters"].([]map[string]any)
		for _, entry := range energy {
			monID := getInt(entry["pokemonId"])
			entryAmount := getInt(entry["amount"])
			if reward > 0 && reward != monID {
				continue
			}
			if amount > 0 && entryAmount < amount {
				continue
			}
			return true
		}
	default:
		// PoracleJS only matches quests via rewardData-derived filters. Unsupported reward types
		// (anything other than 2/3/4/7/12) should not match any trackers.
		return false
	}
	return false
}

func matchInvasion(hook *Hook, row map[string]any) bool {
	tracked := strings.ToLower(getString(row["grunt_type"]))
	if tracked != "" && tracked != "all" && tracked != "everything" {
		if strings.ToLower(getString(hook.Message["grunt_type"])) != tracked {
			return false
		}
	}
	if gender := getInt(row["gender"]); gender > 0 && getInt(hook.Message["gender"]) != gender {
		return false
	}
	return true
}

func matchInvasionWithData(p *Processor, hook *Hook, row map[string]any) bool {
	tracked := strings.ToLower(getString(row["grunt_type"]))
	if tracked != "" && tracked != "all" && tracked != "everything" {
		if id, err := strconv.Atoi(tracked); err == nil && id > 0 {
			displayTypeID, gruntTypeID := resolveInvasionTypes(hook)
			rawGruntType := invasionRawGruntType(hook)
			if gruntTypeID == 0 {
				gruntTypeID = rawGruntType
			}
			if id != gruntTypeID && !(rawGruntType == 0 && displayTypeID >= 7 && id == displayTypeID) {
				return false
			}
		} else if invasionTrackType(p, hook) != tracked {
			return false
		}
	}
	if gender := getInt(row["gender"]); gender > 0 {
		if invasionGender(p, hook) != gender {
			return false
		}
	}
	return true
}

func invasionTrackType(p *Processor, hook *Hook) string {
	displayTypeID, gruntTypeID := resolveInvasionTypes(hook)
	rawGruntType := invasionRawGruntType(hook)
	if gruntTypeID > 0 {
		if grunt := findGruntByID(p, gruntTypeID); grunt != nil {
			typeName := getString(grunt["type"])
			if typeName != "" {
				return strings.ToLower(typeName)
			}
		}
	}
	if rawGruntType == 0 && displayTypeID >= 7 {
		if name, _, _ := pokestopEventInfo(p, displayTypeID); name != "" {
			return strings.ToLower(name)
		}
	}
	if rawGruntType == 0 {
		return ""
	}
	return strings.ToLower(getString(hook.Message["grunt_type"]))
}

func invasionGender(p *Processor, hook *Hook) int {
	_, gruntTypeID := resolveInvasionTypes(hook)
	if gruntTypeID > 0 {
		if grunt := findGruntByID(p, gruntTypeID); grunt != nil {
			return getInt(grunt["gender"])
		}
	}
	return 0
}

func matchLure(hook *Hook, row map[string]any) bool {
	lureID := getInt(hook.Message["lure_id"])
	if lureID == 0 {
		lureID = getInt(hook.Message["lure_type"])
	}
	if tracked := getInt(row["lure_id"]); tracked > 0 && tracked != lureID {
		return false
	}
	return true
}

func matchNest(hook *Hook, row map[string]any) bool {
	pokemonID := getInt(hook.Message["pokemon_id"])
	if tracked := getInt(row["pokemon_id"]); tracked > 0 && tracked != pokemonID {
		return false
	}
	if form := getInt(row["form"]); form > 0 && getInt(hook.Message["form"]) != form {
		return false
	}
	average := getFloat(hook.Message["average_spawns"])
	if average == 0 {
		average = getFloat(hook.Message["pokemon_avg"])
	}
	if minSpawn := getInt(row["min_spawn_avg"]); minSpawn > 0 && average < float64(minSpawn) {
		return false
	}
	return true
}

func matchGym(hook *Hook, row map[string]any) bool {
	team := teamFromHookMessage(hook.Message)
	if tracked := getInt(row["team"]); tracked != 4 && tracked != team {
		return false
	}
	if trackedGym := getString(row["gym_id"]); trackedGym != "" {
		hookGymID := getString(hook.Message["id"])
		if hookGymID == "" {
			hookGymID = getString(hook.Message["gym_id"])
		}
		if trackedGym != hookGymID {
			return false
		}
	}
	oldTeamRaw, hasOldTeam := hook.Message["old_team_id"]
	oldTeam := getInt(oldTeamRaw)
	inBattle := gymInBattle(hook.Message)
	if hasOldTeam && oldTeam == team {
		oldSlots := getInt(hook.Message["old_slots_available"])
		newSlots := getInt(hook.Message["slots_available"])
		slotChanged := getInt(row["slot_changes"]) == 1 && oldSlots != newSlots
		battleChanged := getInt(row["battle_changes"]) == 1 && inBattle
		if !slotChanged && !battleChanged {
			return false
		}
	}
	return true
}

func matchWeather(hook *Hook, row map[string]any) bool {
	condition := weatherCondition(hook.Message)
	if tracked := getInt(row["condition"]); tracked > 0 && tracked != condition {
		return false
	}
	if cell := getString(row["cell"]); cell != "" && cell != getString(hook.Message["s2_cell_id"]) {
		return false
	}
	return true
}

func matchFort(hook *Hook, row map[string]any) bool {
	fortType := strings.ToLower(getString(row["fort_type"]))
	oldEntry := mapFromAny(hook.Message["old"])
	newEntry := mapFromAny(hook.Message["new"])

	hookType := strings.ToLower(getString(hook.Message["type"]))
	if hookType == "" {
		hookType = strings.ToLower(getStringFromAnyMap(newEntry, "type"))
	}
	if hookType == "" {
		hookType = strings.ToLower(getStringFromAnyMap(oldEntry, "type"))
	}
	if hookType == "" {
		hookType = "unknown"
	}
	if fortType != "" && fortType != "everything" {
		if hookType != fortType {
			return false
		}
	}

	changeType := getString(hook.Message["change_type"])
	editTypes := []string{}
	switch v := hook.Message["edit_types"].(type) {
	case []string:
		editTypes = append(editTypes, v...)
	case []any:
		for _, item := range v {
			if entry, ok := item.(string); ok {
				editTypes = append(editTypes, entry)
			}
		}
	case string:
		var decoded []string
		if err := json.Unmarshal([]byte(v), &decoded); err == nil {
			editTypes = append(editTypes, decoded...)
		}
	}
	if changeType == "edit" && (getStringFromAnyMap(oldEntry, "name") == "" && getStringFromAnyMap(oldEntry, "description") == "") {
		changeType = "new"
		editTypes = nil
	}
	changeTypes := make([]string, 0, len(editTypes)+1)
	for _, entry := range editTypes {
		if entry = strings.ToLower(strings.TrimSpace(entry)); entry != "" {
			changeTypes = append(changeTypes, entry)
		}
	}
	if changeType != "" {
		changeTypes = append(changeTypes, strings.ToLower(strings.TrimSpace(changeType)))
	}

	isEmpty := true
	if newEntry != nil && (getStringFromAnyMap(newEntry, "name") != "" || getStringFromAnyMap(newEntry, "description") != "") {
		isEmpty = false
	}
	if oldEntry != nil && getStringFromAnyMap(oldEntry, "name") != "" {
		isEmpty = false
	}
	if isEmpty && getInt(row["include_empty"]) == 0 {
		return false
	}

	trackedTypes := []string{}
	switch v := row["change_types"].(type) {
	case string:
		if strings.TrimSpace(v) != "" && strings.TrimSpace(v) != "[]" {
			_ = json.Unmarshal([]byte(v), &trackedTypes)
		}
	case []byte:
		if len(v) > 0 && strings.TrimSpace(string(v)) != "[]" {
			_ = json.Unmarshal(v, &trackedTypes)
		}
	}
	if len(trackedTypes) > 0 {
		match := false
		for _, tracked := range trackedTypes {
			tracked = strings.ToLower(strings.TrimSpace(tracked))
			if tracked == "" {
				continue
			}
			if containsString(changeTypes, tracked) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	return true
}

func normalizeRSVPList(raw any) []map[string]any {
	if raw == nil {
		return nil
	}
	switch value := raw.(type) {
	case []map[string]any:
		return value
	case []any:
		out := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if entry, ok := item.(map[string]any); ok {
				out = append(out, entry)
			}
		}
		return out
	case map[string]any:
		return []map[string]any{value}
	default:
		return nil
	}
}

// Keep RSVP helpers minimal for parity with PoracleJS.
