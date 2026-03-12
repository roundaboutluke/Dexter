package server

import (
	"net/http"
	"strconv"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/dts"
	"poraclego/internal/version"
)

func registerConfigRoutes(s *Server, mux *http.ServeMux) {
	mux.HandleFunc("/api/config/poracleWeb", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		defaultTemplate := getStringValueFromConfig(s.cfg, "general.defaultTemplateName", "1")
		pvpCaps := getIntSliceFromConfig(s.cfg, "pvp.levelCaps", []int{50})
		forceMinCp, _ := s.cfg.GetBool("pvp.forceMinCp")
		dataSource, _ := s.cfg.GetString("pvp.dataSource")

		payload := map[string]any{
			"status":                 "ok",
			"version":                version.Read(s.root),
			"locale":                 getStringValueFromConfig(s.cfg, "general.locale", ""),
			"prefix":                 getStringValueFromConfig(s.cfg, "discord.prefix", ""),
			"providerURL":            getStringValueFromConfig(s.cfg, "geocoding.providerURL", ""),
			"addressFormat":          getStringValueFromConfig(s.cfg, "locale.addressFormat", ""),
			"staticKey":              getAnyValueFromConfig(s.cfg, "geocoding.staticKey"),
			"pvpFilterMaxRank":       getIntValueFromConfig(s.cfg, "pvp.pvpFilterMaxRank", 0),
			"pvpFilterGreatMinCP":    getIntValueFromConfig(s.cfg, "pvp.pvpFilterGreatMinCP", 0),
			"pvpFilterUltraMinCP":    getIntValueFromConfig(s.cfg, "pvp.pvpFilterUltraMinCP", 0),
			"pvpFilterLittleMinCP":   getIntValueFromConfig(s.cfg, "pvp.pvpFilterLittleMinCP", 0),
			"pvpLittleLeagueAllowed": true,
			"pvpCaps":                pvpCaps,
			"pvpRequiresMinCp":       forceMinCp && strings.EqualFold(dataSource, "webhook"),
			"defaultPvpCap":          getIntValueFromConfig(s.cfg, "tracking.defaultUserTrackingLevelCap", 0),
			"defaultTemplateName":    defaultTemplate,
			"channelNotesContainsCategory": getBoolValueFromConfig(s.cfg, "discord.checkRole", false) &&
				getBoolValueFromConfig(s.cfg, "reconciliation.discord.updateChannelNotes", false),
			"admins": map[string]any{
				"discord":  getStringSliceValueFromConfig(s.cfg, "discord.admins"),
				"telegram": getStringSliceValueFromConfig(s.cfg, "telegram.admins"),
			},
			"maxDistance":               getIntValueFromConfig(s.cfg, "tracking.maxDistance", 0),
			"defaultDistance":           getIntValueFromConfig(s.cfg, "tracking.defaultDistance", 0),
			"everythingFlagPermissions": getAnyValueFromConfig(s.cfg, "tracking.everythingFlagPermissions"),
			"disabledHooks":             disabledHooks(s),
			"gymBattles":                getBoolValueFromConfig(s.cfg, "tracking.enableGymBattle", false),
		}

		writeJSON(w, http.StatusOK, payload)
	})

	mux.HandleFunc("/api/config/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		payload := map[string]any{
			"status":   "ok",
			"discord":  templateGroups(s, "discord"),
			"telegram": templateGroups(s, "telegram"),
		}
		writeJSON(w, http.StatusOK, payload)
	})
}

func templateGroups(s *Server, platform string) map[string]any {
	types := uniqueTemplateTypes(s, platform)
	out := map[string]any{}
	for _, templateType := range types {
		languages := uniqueTemplateLanguages(s, platform, templateType)
		langMap := map[string]any{}
		for _, language := range languages {
			langMap[language] = templateIDs(s, platform, templateType, language)
		}
		out[templateType] = langMap
	}
	return out
}

func uniqueTemplateTypes(s *Server, platform string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, tpl := range s.dts {
		if tpl.Hidden || tpl.Platform != platform {
			continue
		}
		if seen[tpl.Type] {
			continue
		}
		seen[tpl.Type] = true
		out = append(out, tpl.Type)
	}
	return out
}

func uniqueTemplateLanguages(s *Server, platform, templateType string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, tpl := range s.dts {
		if tpl.Hidden || tpl.Platform != platform || tpl.Type != templateType {
			continue
		}
		language := templateLanguage(tpl)
		if language == "" {
			language = "%"
		}
		if seen[language] {
			continue
		}
		seen[language] = true
		out = append(out, language)
	}
	return out
}

func templateIDs(s *Server, platform, templateType, language string) []any {
	out := []any{}
	for _, tpl := range s.dts {
		if tpl.Hidden || tpl.Platform != platform || tpl.Type != templateType {
			continue
		}
		tplLanguage := templateLanguage(tpl)
		if language == "%" {
			if tplLanguage != "" {
				continue
			}
		} else if tplLanguage != language {
			continue
		}
		out = append(out, tpl.ID)
	}
	return out
}

func templateLanguage(tpl dts.Template) string {
	if tpl.Language == nil {
		return ""
	}
	if strings.TrimSpace(*tpl.Language) == "" {
		return ""
	}
	return *tpl.Language
}

func disabledHooks(s *Server) []string {
	hookTypes := []string{"Pokemon", "Raid", "Pokestop", "Invasion", "Lure", "Quest", "Weather", "Nest", "Gym"}
	disabled := []string{}
	for _, hookType := range hookTypes {
		key := "general.disable" + hookType
		if getBoolValueFromConfig(s.cfg, key, false) {
			disabled = append(disabled, strings.ToLower(hookType))
		}
	}
	return disabled
}

func getAnyValueFromConfig(cfg *config.Config, path string) any {
	val, ok := cfg.Get(path)
	if !ok {
		return nil
	}
	return val
}

func getStringValueFromConfig(cfg *config.Config, path, fallback string) string {
	value, ok := cfg.GetString(path)
	if !ok {
		return fallback
	}
	return value
}

func getIntValueFromConfig(cfg *config.Config, path string, fallback int) int {
	value, ok := cfg.GetInt(path)
	if !ok {
		return fallback
	}
	return value
}

func getBoolValueFromConfig(cfg *config.Config, path string, fallback bool) bool {
	value, ok := cfg.GetBool(path)
	if !ok {
		return fallback
	}
	return value
}

func getStringSliceValueFromConfig(cfg *config.Config, path string) []string {
	value, ok := cfg.GetStringSlice(path)
	if !ok {
		return []string{}
	}
	return value
}

func getIntSliceFromConfig(cfg *config.Config, path string, fallback []int) []int {
	raw, ok := cfg.Get(path)
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case []int:
		return v
	case []any:
		out := make([]int, 0, len(v))
		for _, item := range v {
			switch num := item.(type) {
			case int:
				out = append(out, num)
			case int64:
				out = append(out, int(num))
			case float64:
				out = append(out, int(num))
			case string:
				if parsed, err := strconv.Atoi(num); err == nil {
					out = append(out, parsed)
				}
			}
		}
		if len(out) > 0 {
			return out
		}
	case float64:
		return []int{int(v)}
	case int:
		return []int{v}
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return []int{parsed}
		}
	}
	return fallback
}
