package webhook

import (
	"fmt"
	"strconv"
	"strings"

	"dexter/internal/uicons"
)

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
		if url, ok := client.PokemonIcon(pokemonID, form, evolution, gender, costume, alignment, shinyPossible, bread); ok {
			return url
		}
		if url, ok := client.PokemonIcon(pokemonID, 0, 0, 0, 0, 0, shinyPossible, bread); ok {
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

func isUiconsRepo(baseURL, imageType string) bool {
	return uicons.IsCachedRepo(baseURL, imageType)
}

func uiconsClient(baseURL, imageType string) *uicons.Client {
	return uicons.CachedClient(baseURL, imageType)
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
