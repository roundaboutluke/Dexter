package webhook

import (
	"fmt"
	"strings"

	"dexter/internal/config"
)

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
	case "pokemon", "max_battle":
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
