package webhook

import (
	"fmt"
	"strings"
	"time"

	"dexter/internal/config"
	"dexter/internal/logging"
)

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
		if logger := logging.Get().General; logger != nil && err != nil {
			logger.Warnf("shortener failed for %s: %v", url, err)
		}
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
