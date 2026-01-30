package server

import (
	"fmt"
	"net/http"
	"strings"

	"poraclego/internal/config"
)

// rejectNotAllowedByIP writes the PoracleJS-style unhappy payload for whitelist/blacklist failures.
// Returns true if a response was written and the handler should stop.
func rejectNotAllowedByIP(cfg *config.Config, r *http.Request, w http.ResponseWriter) bool {
	ip := clientIP(r)
	whitelist, _ := cfg.GetStringSlice("server.ipWhitelist")
	blacklist, _ := cfg.GetStringSlice("server.ipBlacklist")
	if len(whitelist) > 0 && !containsString(whitelist, ip) {
		writeJSON(w, http.StatusOK, map[string]any{
			"webserver": "unhappy",
			"reason":    fmt.Sprintf("ip %s not in whitelist", ip),
		})
		return true
	}
	if len(blacklist) > 0 && containsString(blacklist, ip) {
		writeJSON(w, http.StatusOK, map[string]any{
			"webserver": "unhappy",
			"reason":    fmt.Sprintf("ip %s in blacklist", ip),
		})
		return true
	}
	return false
}

// rejectNotAuthorized writes the PoracleJS-style authError payload when the api secret is missing/incorrect.
// Returns true if a response was written and the handler should stop.
func rejectNotAuthorized(cfg *config.Config, r *http.Request, w http.ResponseWriter) bool {
	secret := r.Header.Get("x-poracle-secret")
	if secret == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "authError",
			"reason": "incorrect or missing api secret",
		})
		return true
	}
	configSecret, ok := cfg.GetString("server.apiSecret")
	if !ok || strings.TrimSpace(configSecret) == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "authError",
			"reason": "incorrect or missing api secret",
		})
		return true
	}
	if secret != configSecret {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "authError",
			"reason": "incorrect or missing api secret",
		})
		return true
	}
	return false
}

