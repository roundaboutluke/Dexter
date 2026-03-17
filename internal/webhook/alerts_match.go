package webhook

import (
	"fmt"
	"strings"

	"poraclego/internal/alertstate"
	"poraclego/internal/digest"
	"poraclego/internal/logging"
)

// monsterCandidates returns only the tracking rows that could possibly match
// the pokemon in the hook, using the pre-built index instead of scanning all rows.
func monsterCandidates(idx *alertstate.MonsterIndex, hook *Hook) []map[string]any {
	pokemonID := getInt(hook.Message["pokemon_id"])

	// Collect non-PVP candidates: catch-all (id=0) + species-specific.
	var candidates []map[string]any
	candidates = append(candidates, idx.ByPokemonID[0]...)
	if pokemonID != 0 {
		candidates = append(candidates, idx.ByPokemonID[pokemonID]...)
	}

	// Collect PVP candidates for each league present in the hook.
	for _, league := range []int{500, 1500, 2500} {
		key := pvpLeagueKey(league)
		if hook.Message[key] == nil {
			continue
		}
		candidates = append(candidates, idx.PVPEverything[league]...)
		candidates = append(candidates, idx.PVPSpecific[league]...)
	}

	return candidates
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
	var (
		rows         []map[string]any
		humans       map[string]map[string]any
		profiles     map[string]map[string]any
		hasSchedules map[string]bool
	)
	fenceStore := p.fences
	snapshot := p.currentAlertState()
	if snapshot != nil {
		rows = snapshot.Rows(table)
		humans = snapshot.Humans
		profiles = snapshot.Profiles
		hasSchedules = snapshot.HasSchedules
		if snapshot.Fences != nil {
			fenceStore = snapshot.Fences
		}
	}
	var err error
	if snapshot == nil {
		rows, err = p.query.SelectAllQuery(table, map[string]any{})
		if err != nil {
			return nil, err
		}
	}
	if len(rows) == 0 {
		return nil, nil
	}
	if humans == nil || profiles == nil {
		humans, profiles, err = p.loadHumansForRows(rows)
		if err != nil {
			return nil, err
		}
	}
	categoryKey := alertCategoryKey(hook.Type)
	pvpSec := pvpSecurityEnabled(p.cfg)
	blockedByID := map[string]map[string]bool{}
	if hasSchedules == nil {
		hasSchedules = map[string]bool{}
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
	}
	// Use indexed lookup for pokemon matching when available.
	if hook.Type == "pokemon" && table == "monsters" && snapshot != nil && snapshot.Monsters != nil {
		rows = monsterCandidates(snapshot.Monsters, hook)
	}

	var questRewardsNoAR map[string]any
	var questRewardsAR map[string]any
	if hook != nil && hook.Type == "quest" {
		questRewardsNoAR = questRewardData(p, hook)
		questRewardsAR = questRewardDataAR(p, hook)
	}

	targets := []alertMatch{}
	for _, row := range rows {
		matchRow := row
		questMatchNoAR := false
		questMatchAR := false
		if hook.Type == "quest" {
			if ok, noAR, ar := matchQuestWithVariants(hook, row, questRewardsNoAR, questRewardsAR); ok {
				questMatchNoAR = noAR
				questMatchAR = ar
				matchRow = cloneMatchRow(row)
				matchRow["questMatchNoAR"] = boolToInt(noAR)
				matchRow["questMatchAR"] = boolToInt(ar)
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
				if passesLocationFilter(fenceStore, p.cfg, location, hook, matchRow) {
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
		if !passesLocationFilter(fenceStore, p.cfg, location, hook, matchRow) {
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
		}, Row: matchRow})
	}
	return targets, nil
}

func cloneMatchRow(row map[string]any) map[string]any {
	if row == nil {
		return nil
	}
	cloned := make(map[string]any, len(row)+2)
	for key, value := range row {
		cloned[key] = value
	}
	return cloned
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
