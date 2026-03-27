package validate

import (
	"fmt"
	"strings"

	"dexter/internal/config"
	"dexter/internal/dts"
	"dexter/internal/geofence"
)

// CheckConfig logs warnings for common config issues.
func CheckConfig(cfg *config.Config, logger func(string, ...any)) {
	if cfg == nil {
		return
	}
	if logger == nil {
		logger = func(string, ...any) {}
	}

	provider, _ := cfg.GetString("geocoding.provider")
	switch strings.ToLower(provider) {
	case "none", "nominatim", "pelias", "google", "openstreetmap", "photon":
	default:
		logger("Config Check: geocoding/provider is not one of none,nominatim,pelias,google,openstreetmap,photon")
	}
	if strings.EqualFold(provider, "nominatim") || strings.EqualFold(provider, "pelias") || strings.EqualFold(provider, "photon") {
		if url, _ := cfg.GetString("geocoding.providerURL"); url != "" && !strings.HasPrefix(url, "http") {
			logger("Config Check: geocoding/providerURL does not start with http")
		}
	}

	if enabled, _ := cfg.GetBool("discord.enabled"); enabled {
		if guilds, _ := cfg.GetStringSlice("discord.guilds"); len(guilds) == 0 {
			logger("Config Check: discord guilds entry missing or empty - will cause reconciliation failures")
		}
	}

	staticProvider, _ := cfg.GetString("geocoding.staticProvider")
	switch strings.ToLower(staticProvider) {
	case "none", "tileservercache", "google", "osm", "mapbox":
	default:
		logger("Config Check: static provider is not one of none,tileservercache,google,osm,mapbox")
	}
	if strings.EqualFold(staticProvider, "tileservercache") {
		if url, _ := cfg.GetString("geocoding.staticProviderURL"); url != "" && !strings.HasPrefix(url, "http") {
			logger("Config Check: geocoding/staticProviderURL does not start with http")
		}
	}

	roleMode, _ := cfg.GetString("general.roleCheckMode")
	switch roleMode {
	case "ignore", "delete", "disable-user":
	default:
		logger("Config Check: roleCheckMode is not one of ignore,delete,disable-user")
	}

	if _, ok := cfg.Get("discord.limitSec"); ok {
		logger("Config Check: legacy option “discord.limitSec” given and ignored, replace with “alertLimits.timingPeriod”")
	}
	if _, ok := cfg.Get("discord.limitAmount"); ok {
		logger("Config Check: legacy option “discord.limitAmount” given and ignored, replace with “alertLimits.dmLimit/channelLimit”")
	}

	everythingFlag, _ := cfg.GetString("tracking.everythingFlagPermissions")
	switch everythingFlag {
	case "allow-any", "allow-and-always-individually", "allow-and-ignore-individually", "deny":
	default:
		logger("Config Check: everything flag permissions is not one of allow-any,allow-and-always-individually,allow-and-ignore-individually,deny")
	}

	if _, ok := cfg.Get("tracking.disableEverythingTracking"); ok {
		logger("Config Check: legacy option “tracking.disableEverythingTracking” given and ignored, replace with “tracking.everythingFlagPermissions”")
	}
	if _, ok := cfg.Get("tracking.forceEverythingSeparately"); ok {
		logger("Config Check: legacy option “tracking.forceEverythingSeparately” given and ignored, replace with “tracking.everythingFlagPermissions”")
	}
	if _, ok := cfg.Get("general.roleCheckDeletionsAllowed"); ok {
		logger("Config Check: legacy option “roleCheckDeletionsAllowed“ given and ignored, replace with “general.roleCheckMode“")
	}
}

// CheckDTS logs warnings for missing template coverage.
func CheckDTS(templates []dts.Template, cfg *config.Config, logger func(string, ...any)) {
	if cfg == nil {
		return
	}
	if logger == nil {
		logger = func(string, ...any) {}
	}
	platforms := []string{}
	if enabled, _ := cfg.GetBool("discord.enabled"); enabled {
		platforms = append(platforms, "discord")
	}
	if enabled, _ := cfg.GetBool("telegram.enabled"); enabled {
		platforms = append(platforms, "telegram")
	}

	languages := []string{}
	defaultLang, _ := cfg.GetString("general.locale")
	if defaultLang != "" {
		languages = append(languages, defaultLang)
	}
	if raw, ok := cfg.Get("general.availableLanguages"); ok {
		if m, ok := raw.(map[string]any); ok {
			for key := range m {
				if key != defaultLang {
					languages = append(languages, key)
				}
			}
		}
	}

	templateTypes := []string{"monster", "monsterNoIv", "raid", "egg", "quest", "invasion", "lure", "weatherchange", "greeting"}
	defaultTemplateName, _ := cfg.Get("general.defaultTemplateName")
	defaultTemplateID := fmt.Sprintf("%v", defaultTemplateName)

	for _, platform := range platforms {
		for _, language := range languages {
			for _, typ := range templateTypes {
				if !hasTemplate(templates, platform, language, typ, func(t dts.Template) bool { return t.Default }) {
					logger("Config Check: DTS - No default entry found for platform:%s language:%s type:%s", platform, language, typ)
				}
				if !hasTemplate(templates, platform, language, typ, func(t dts.Template) bool {
					return fmt.Sprintf("%v", t.ID) == defaultTemplateID
				}) {
					logger("Config Check: DTS - No entry found for template “%s” platform:%s language:%s type:%s - this is the one that users will get if no template override", defaultTemplateID, platform, language, typ)
				}
				for _, tpl := range templates {
					if tpl.Platform == platform && tpl.Type == typ && languageMatches(tpl.Language, language) {
						if fmt.Sprintf("%v", tpl.ID) == "" {
							logger("Config Check: DTS - Template name blank in platform:%s language:%s type:%s", platform, language, typ)
						}
					}
				}
			}
		}
	}
}

// CheckGeofence logs warnings for invalid geofence entries.
func CheckGeofence(fences []geofence.Fence, logger func(string, ...any)) {
	if logger == nil {
		logger = func(string, ...any) {}
	}
	if len(fences) == 0 {
		logger("Config Check: geofence.json is empty")
		return
	}
	for _, fence := range fences {
		if strings.TrimSpace(fence.Name) == "" {
			logger("Config Check: geofence.json has entry with blank name")
		}
		if len(fence.Path) == 0 && len(fence.MultiPath) == 0 {
			logger("Config Check: geofence.json has empty path")
		}
	}
}

func hasTemplate(templates []dts.Template, platform, language, typ string, predicate func(dts.Template) bool) bool {
	for _, tpl := range templates {
		if tpl.Platform == platform && tpl.Type == typ && languageMatches(tpl.Language, language) && predicate(tpl) {
			return true
		}
	}
	return false
}

func languageMatches(lang *string, target string) bool {
	if lang == nil {
		return false
	}
	return *lang == target
}
