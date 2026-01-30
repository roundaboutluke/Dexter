package server

import (
	"encoding/json"
	"net/http"

	"poraclego/internal/dispatch"
)

func registerPostMessageRoutes(s *Server, mux *http.ServeMux) {
	mux.HandleFunc("/api/postMessage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		var payload any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"status":  "error",
				"message": "invalid payload",
			})
			return
		}

		defaultLanguage := getStringValueFromConfig(s.cfg, "general.locale", "")
		jobs := normalizePostMessageJobs(payload, defaultLanguage)
		for _, job := range jobs {
			if s.discordQueue != nil && isDiscordPostMessageType(job.Type) {
				s.discordQueue.Push(job)
			}
			if s.telegramQueue != nil && isTelegramPostMessageType(job.Type) {
				s.telegramQueue.Push(job)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})
}

func normalizePostMessageJobs(payload any, defaultLanguage string) []dispatch.MessageJob {
	items := []map[string]any{}
	switch v := payload.(type) {
	case []any:
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				items = append(items, m)
			}
		}
	case map[string]any:
		items = append(items, v)
	}

	jobs := make([]dispatch.MessageJob, 0, len(items))
	for _, item := range items {
		job := dispatch.MessageJob{
			Lat:          getFloatFromAny(item["lat"], 0),
			Lon:          getFloatFromAny(item["lon"], 0),
			Message:      getStringFromAny(item["message"], ""),
			Target:       getStringFromAny(item["target"], ""),
			Type:         getStringFromAny(item["type"], ""),
			Name:         getStringFromAny(item["name"], ""),
			TTH:          parseTTH(item, "tth"),
			Clean:        getBoolFromAny(item["clean"], false),
			Emoji:        getStringFromAny(item["emoji"], ""),
			LogReference: getStringFromAny(item["logReference"], "WebApi"),
			Language:     getStringFromAny(item["language"], defaultLanguage),
		}
		jobs = append(jobs, job)
	}
	return jobs
}

func parseTTH(item map[string]any, key string) dispatch.TimeToHide {
	raw, ok := item[key]
	if !ok {
		return dispatch.TimeToHide{Hours: 1, Minutes: 0, Seconds: 0}
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return dispatch.TimeToHide{Hours: 1, Minutes: 0, Seconds: 0}
	}
	return dispatch.TimeToHide{
		Hours:   getIntFromAny(m["hours"], 0),
		Minutes: getIntFromAny(m["minutes"], 0),
		Seconds: getIntFromAny(m["seconds"], 0),
	}
}

func isDiscordPostMessageType(value string) bool {
	switch value {
	case "discord:user", "discord:channel", "webhook":
		return true
	default:
		return false
	}
}

func isTelegramPostMessageType(value string) bool {
	switch value {
	case "telegram:user", "telegram:channel", "telegram:group":
		return true
	default:
		return false
	}
}

func getStringFromAny(value any, fallback string) string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return fallback
		}
		return v
	case []byte:
		if len(v) == 0 {
			return fallback
		}
		return string(v)
	default:
		return fallback
	}
}

func getFloatFromAny(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case json.Number:
		if parsed, err := v.Float64(); err == nil {
			return parsed
		}
	}
	return fallback
}

func getIntFromAny(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		if parsed, err := v.Int64(); err == nil {
			return int(parsed)
		}
	}
	return fallback
}

func getBoolFromAny(value any, fallback bool) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		if v == "true" {
			return true
		}
		if v == "false" {
			return false
		}
	}
	return fallback
}
