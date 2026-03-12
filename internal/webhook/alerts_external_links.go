package webhook

import (
	"fmt"
	"strings"

	"poraclego/internal/config"
)

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
