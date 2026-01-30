package server

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/geofence"
	"poraclego/internal/profile"
	"poraclego/internal/tileserver"
	"poraclego/internal/tracking"
)

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		port, ok := s.cfg.GetInt("server.port")
		if !ok {
			port = 3030
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"webserver": "happy",
			"query":     r.URL.Query(),
			"port":      port,
		})
	})

	registerTrackingExtras(s, mux)
	registerConfigRoutes(s, mux)
	registerMasterDataRoutes(s, mux)
	registerPostMessageRoutes(s, mux)

	mux.HandleFunc("/api/humans/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 || parts[0] != "api" || parts[1] != "humans" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if len(parts) == 4 && parts[2] == "one" && r.Method == http.MethodGet {
			handleHumanOne(w, s, parts[3])
			return
		}
		id := parts[2]

		switch r.Method {
		case http.MethodGet:
			if len(parts) == 4 && parts[3] == "roles" {
				handleHumanRoles(w, s, id)
				return
			}
			if len(parts) == 4 && parts[3] == "getAdministrationRoles" {
				handleHumanAdministrationRoles(w, s, id)
				return
			}
			if len(parts) == 6 && parts[3] == "checkLocation" {
				lat, err := strconv.ParseFloat(parts[4], 64)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"status":  "error",
						"message": "invalid latitude",
					})
					return
				}
				lon, err := strconv.ParseFloat(parts[5], 64)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"status":  "error",
						"message": "invalid longitude",
					})
					return
				}
				handleCheckLocation(w, s, id, lat, lon)
				return
			}
			if len(parts) != 3 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			handleHumansGet(w, s, id)
		case http.MethodPost:
			if len(parts) == 6 && parts[3] == "roles" && parts[4] == "add" {
				handleHumanRoleUpdate(w, s, id, parts[5], true)
				return
			}
			if len(parts) == 6 && parts[3] == "roles" && parts[4] == "remove" {
				handleHumanRoleUpdate(w, s, id, parts[5], false)
				return
			}
			if len(parts) == 6 && parts[3] == "setLocation" {
				lat, err := strconv.ParseFloat(parts[4], 64)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"status":  "error",
						"message": "invalid latitude",
					})
					return
				}
				lon, err := strconv.ParseFloat(parts[5], 64)
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"status":  "error",
						"message": "invalid longitude",
					})
					return
				}
				handleSetLocation(w, s, id, lat, lon)
				return
			}
			if len(parts) == 5 && parts[3] == "switchProfile" {
				profileNo, err := strconv.Atoi(parts[4])
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"status":  "error",
						"message": "invalid profile",
					})
					return
				}
				handleSwitchProfile(w, s, id, profileNo)
				return
			}
			if len(parts) == 4 && parts[3] == "setAreas" {
				handleSetAreas(w, s, id, r)
				return
			}
			if len(parts) == 4 && parts[3] == "start" {
				handleStartStop(w, s, id, true)
				return
			}
			if len(parts) == 4 && parts[3] == "stop" {
				handleStartStop(w, s, id, false)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/geofence/all/hash", func(w http.ResponseWriter, r *http.Request) {
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

		areas := map[string]string{}
		for _, fence := range s.fences.Fences {
			hash := md5.Sum(mustJSONMarshal(fence.Path))
			areas[fence.Name] = hex.EncodeToString(hash[:])
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
			"areas":  areas,
		})
	})

	mux.HandleFunc("/api/geofence/all", func(w http.ResponseWriter, r *http.Request) {
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
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "ok",
			"geofence": s.fences.Fences,
		})
	})

	mux.HandleFunc("/api/geofence/all/geojson", func(w http.ResponseWriter, r *http.Request) {
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
		outGeo := geofenceToGeoJSON(s.fences.Fences)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"geoJSON": outGeo,
		})
	})

	mux.HandleFunc("/api/geofence/reload", func(w http.ResponseWriter, r *http.Request) {
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

		geofence.FetchKojiFences(s.cfg, s.root, nil)
		reloaded, err := geofence.Load(s.cfg, s.root)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
		if s.fences == nil {
			s.fences = reloaded
		} else {
			s.fences.Replace(reloaded.Fences)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
		})
	})

	mux.HandleFunc("/api/geofence/", func(w http.ResponseWriter, r *http.Request) {
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

		path := strings.TrimPrefix(r.URL.Path, "/api/geofence/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) >= 2 && parts[0] == "locationMap" && len(parts) == 3 {
			if !tileserverAvailable(s.cfg) {
				respondTileserverNotImplemented(w, s.cfg)
				return
			}
			lat, err1 := strconv.ParseFloat(parts[1], 64)
			lon, err2 := strconv.ParseFloat(parts[2], 64)
			if err1 != nil || err2 != nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":  "error",
					"message": "Invalid parameters",
				})
				return
			}
			client := tileserver.NewClient(s.cfg)
			url, err := tileserver.GenerateLocationTile(client, s.cfg, lat, lon)
			if err != nil || url == "" {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":  "error",
					"message": "Exception raised during execution",
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"url":    url,
			})
			return
		}
		if len(parts) >= 2 && parts[0] == "distanceMap" && len(parts) == 4 {
			if !tileserverAvailable(s.cfg) {
				respondTileserverNotImplemented(w, s.cfg)
				return
			}
			lat, err1 := strconv.ParseFloat(parts[1], 64)
			lon, err2 := strconv.ParseFloat(parts[2], 64)
			dist, err3 := strconv.ParseFloat(parts[3], 64)
			if err1 != nil || err2 != nil || err3 != nil || dist < 0 {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":  "error",
					"message": "Invalid parameters",
				})
				return
			}
			client := tileserver.NewClient(s.cfg)
			url, err := tileserver.GenerateDistanceTile(client, s.cfg, lat, lon, dist)
			if err != nil || url == "" {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":  "error",
					"message": "Exception raised during execution",
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"url":    url,
			})
			return
		}
		if len(parts) == 2 && parts[1] == "map" {
			if !tileserverAvailable(s.cfg) {
				respondTileserverNotImplemented(w, s.cfg)
				return
			}
			client := tileserver.NewClient(s.cfg)
			url, err := tileserver.GenerateGeofenceTile(s.fences.Fences, client, s.cfg, parts[0])
			if err != nil || url == "" {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":  "error",
					"message": "Exception raised during execution",
				})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "ok",
				"url":    url,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})

	mux.HandleFunc("/api/profiles/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/profiles/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]

		switch r.Method {
		case http.MethodGet:
			if len(parts) != 1 {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			handleProfilesGet(w, s, id)
		case http.MethodDelete:
			if len(parts) != 3 || parts[1] != "byProfileNo" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			profileNo, err := strconv.Atoi(parts[2])
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]any{
					"status":  "error",
					"message": "invalid profile_no",
				})
				return
			}
			handleProfilesDelete(w, s, id, profileNo)
		case http.MethodPost:
			if len(parts) >= 2 && parts[1] == "add" {
				handleProfilesAdd(w, s, id, r)
				return
			}
			if len(parts) >= 2 && parts[1] == "update" {
				handleProfilesUpdate(w, s, id, r)
				return
			}
			if len(parts) == 4 && parts[1] == "copy" {
				fromNo, err := strconv.Atoi(parts[2])
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"status":  "error",
						"message": "invalid from profile",
					})
					return
				}
				toNo, err := strconv.Atoi(parts[3])
				if err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]any{
						"status":  "error",
						"message": "invalid to profile",
					})
					return
				}
				handleProfilesCopy(w, s, id, fromNo, toNo)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/pokemon/refresh", func(w http.ResponseWriter, r *http.Request) {
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
		if s.processor != nil {
			s.processor.RefreshAlertCacheAsync()
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "ok",
		})
	})

	mux.HandleFunc("/api/tracking/raid/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/raid/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]

		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingRaidGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingRaidDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingRaidDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingRaidUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/egg/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/egg/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]

		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingEggGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingEggDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingEggDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingEggUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/pokemon/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/pokemon/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]

		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingPokemonGet(w, s, id)
				return
			}
			if len(parts) == 3 && parts[1] == "byUid" {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingPokemonDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingPokemonDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingPokemonUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func handleCheckLocation(w http.ResponseWriter, s *Server, id string, lat float64, lon float64) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}

	areaSecurityEnabled, _ := s.cfg.GetBool("areaSecurity.enabled")
	if !areaSecurityEnabled {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "ok",
			"locationOk": true,
		})
		return
	}

	restriction := parseCommunityMembership(human["area_restriction"])
	areas := s.fences.PointInArea([]float64{lat, lon})
	locationOk := false
	for _, fence := range restriction {
		if containsString(areas, strings.ToLower(fence)) {
			locationOk = true
			break
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":     "ok",
		"locationOk": locationOk,
	})
}

func requireSecret(cfg *config.Config, r *http.Request) bool {
	secret := r.Header.Get("x-poracle-secret")
	if secret == "" {
		return false
	}
	configSecret, ok := cfg.GetString("server.apiSecret")
	if !ok || configSecret == "" {
		return false
	}
	return secret == configSecret
}

func isAdmin(cfg *config.Config, id string) bool {
	discordAdmins, _ := cfg.GetStringSlice("discord.admins")
	telegramAdmins, _ := cfg.GetStringSlice("telegram.admins")
	return containsString(discordAdmins, id) || containsString(telegramAdmins, id)
}

func parseCommunityMembership(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v == "" {
			return []string{}
		}
		var items []string
		if err := json.Unmarshal([]byte(v), &items); err == nil {
			for i, item := range items {
				items[i] = strings.ToLower(item)
			}
			return items
		}
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.ToLower(s))
			}
		}
		return out
	}
	return []string{}
}

func filterAreas(cfg *config.Config, membership []string, areas []string) []string {
	raw, ok := cfg.Get("areaSecurity.communities")
	if !ok {
		return areas
	}
	communities, ok := raw.(map[string]any)
	if !ok {
		return areas
	}
	allowed := make([]string, 0)
	for _, community := range membership {
		matchKey := ""
		for key := range communities {
			if strings.EqualFold(key, community) {
				matchKey = key
				break
			}
		}
		if matchKey == "" {
			continue
		}
		entry, ok := communities[matchKey].(map[string]any)
		if !ok {
			continue
		}
		rawAllowed, ok := entry["allowedAreas"]
		if !ok {
			continue
		}
		for _, area := range toStringSlice(rawAllowed) {
			if !containsString(allowed, area) {
				allowed = append(allowed, area)
			}
		}
	}
	if len(allowed) == 0 {
		return []string{}
	}
	result := make([]string, 0, len(areas))
	for _, area := range areas {
		if containsString(allowed, area) {
			result = append(result, area)
		}
	}
	return result
}

func geofenceToGeoJSON(fences []geofence.Fence) map[string]any {
	out := map[string]any{
		"type":     "FeatureCollection",
		"features": []any{},
	}
	features := make([]any, 0, len(fences))
	for _, fence := range fences {
		userSelectable := true
		if fence.UserSelectable != nil {
			userSelectable = *fence.UserSelectable
		}
		displayInMatches := true
		if fence.DisplayInMatch != nil {
			displayInMatches = *fence.DisplayInMatch
		}
		geometryType := "Polygon"
		if len(fence.MultiPath) > 0 {
			geometryType = "MultiPolygon"
		}
		feature := map[string]any{
			"type": "Feature",
			"properties": map[string]any{
				"name":             fence.Name,
				"color":            fence.Color,
				"id":               fence.ID,
				"group":            fence.Group,
				"description":      fence.Description,
				"userSelectable":   userSelectable,
				"displayInMatches": displayInMatches,
			},
			"geometry": map[string]any{
				"type":        geometryType,
				"coordinates": []any{},
			},
		}
		coords := make([]any, 0)
		if len(fence.MultiPath) > 0 {
			for _, path := range fence.MultiPath {
				sub := pathToGeo(path)
				if len(sub) > 0 {
					coords = append(coords, sub)
				}
			}
		} else {
			sub := pathToGeo(fence.Path)
			if len(sub) > 0 {
				coords = append(coords, sub)
			}
		}
		feature["geometry"].(map[string]any)["coordinates"] = []any{coords}
		features = append(features, feature)
	}
	out["features"] = features
	return out
}

func pathToGeo(path [][]float64) []any {
	out := make([]any, 0, len(path)+1)
	for _, coord := range path {
		if len(coord) < 2 {
			continue
		}
		out = append(out, []float64{coord[1], coord[0]})
	}
	if len(out) > 0 {
		first := out[0].([]float64)
		last := out[len(out)-1].([]float64)
		if first[0] != last[0] || first[1] != last[1] {
			out = append(out, first)
		}
	}
	return out
}

func mustJSONMarshal(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		return []byte{}
	}
	return data
}

func toStringSlice(raw any) []string {
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, strings.ToLower(item))
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.ToLower(s))
			}
		}
		return out
	default:
		return []string{}
	}
}

func respondTileserverNotImplemented(w http.ResponseWriter, cfg *config.Config) {
	provider, _ := cfg.GetString("geocoding.staticProvider")
	providerURL, _ := cfg.GetString("geocoding.staticProviderURL")
	if strings.ToLower(provider) != "tileservercache" || !strings.HasPrefix(providerURL, "http") {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "Unsupported configuration for staticProvider",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "error",
		"message": "Tile generation not implemented",
	})
}

func tileserverAvailable(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	provider, _ := cfg.GetString("geocoding.staticProvider")
	providerURL, _ := cfg.GetString("geocoding.staticProviderURL")
	return strings.ToLower(provider) == "tileservercache" && strings.HasPrefix(providerURL, "http")
}

func handleProfilesGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	profiles, err := s.query.SelectAllQuery("profiles", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"profile": profiles,
	})
}

func handleHumansGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}

	allowedAreas := make([]string, 0, len(s.fences.Fences))
	for _, fence := range s.fences.Fences {
		allowedAreas = append(allowedAreas, strings.ToLower(fence.Name))
	}

	areaSecurityEnabled, _ := s.cfg.GetBool("areaSecurity.enabled")
	if areaSecurityEnabled && !isAdmin(s.cfg, id) {
		communityMembership := parseCommunityMembership(human["community_membership"])
		allowedAreas = filterAreas(s.cfg, communityMembership, allowedAreas)
	}

	areas := make([]map[string]any, 0, len(s.fences.Fences))
	for _, fence := range s.fences.Fences {
		if !containsString(allowedAreas, strings.ToLower(fence.Name)) {
			continue
		}
		userSelectable := true
		if fence.UserSelectable != nil {
			userSelectable = *fence.UserSelectable
		}
		areas = append(areas, map[string]any{
			"name":           fence.Name,
			"group":          fence.Group,
			"description":    fence.Description,
			"userSelectable": userSelectable,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"areas":  areas,
	})
}

func handleSetLocation(w http.ResponseWriter, s *Server, id string, lat float64, lon float64) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}

	areaSecurityEnabled, _ := s.cfg.GetBool("areaSecurity.enabled")
	if areaSecurityEnabled && human["area_restriction"] != nil {
		allowed := parseCommunityMembership(human["area_restriction"])
		areas := s.fences.PointInArea([]float64{lat, lon})
		allowedOk := false
		for _, fence := range allowed {
			if containsString(areas, strings.ToLower(fence)) {
				allowedOk = true
				break
			}
		}
		if !allowedOk {
			writeJSON(w, http.StatusOK, map[string]any{
				"status":  "error",
				"message": "Location not permitted",
			})
			return
		}
	}

	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	_, err = s.query.UpdateQuery("humans", map[string]any{"latitude": lat, "longitude": lon}, map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	_, err = s.query.UpdateQuery("profiles", map[string]any{"latitude": lat, "longitude": lon}, map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleSwitchProfile(w http.ResponseWriter, s *Server, id string, profileNo int) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}

	profileRow, err := s.query.SelectOneQuery("profiles", map[string]any{"id": id, "profile_no": profileNo})
	if err != nil || profileRow == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "Profile not found",
		})
		return
	}

	_, err = s.query.UpdateQuery("humans", map[string]any{
		"current_profile_no": profileNo,
		"area":               profileRow["area"],
		"latitude":           profileRow["latitude"],
		"longitude":          profileRow["longitude"],
	}, map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleSetAreas(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}

	areas, err := decodeStringArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}

	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	adminTarget := !(containsString(getAdminIDs(s.cfg, "discord.admins"), id) && containsString(getAdminIDs(s.cfg, "telegram.admins"), id))

	allowedAreas := make([]string, 0, len(s.fences.Fences))
	for _, fence := range s.fences.Fences {
		if !adminTarget || fence.UserSelectable == nil || *fence.UserSelectable {
			allowedAreas = append(allowedAreas, strings.ToLower(fence.Name))
		}
	}

	areaSecurityEnabled, _ := s.cfg.GetBool("areaSecurity.enabled")
	if areaSecurityEnabled && !adminTarget {
		communityMembership := parseCommunityMembership(human["community_membership"])
		allowedAreas = filterAreas(s.cfg, communityMembership, allowedAreas)
	}

	unique := map[string]bool{}
	filtered := make([]string, 0)
	for _, area := range areas {
		if containsString(allowedAreas, area) && !unique[area] {
			unique[area] = true
			filtered = append(filtered, area)
		}
	}

	areasJSON, _ := json.Marshal(filtered)
	_, err = s.query.UpdateQuery("humans", map[string]any{"area": string(areasJSON)}, map[string]any{"id": id})
	if err == nil {
		_, err = s.query.UpdateQuery("profiles", map[string]any{"area": string(areasJSON)}, map[string]any{"id": id, "profile_no": currentProfile})
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "ok",
		"setAreas": filtered,
	})
}

func handleStartStop(w http.ResponseWriter, s *Server, id string, start bool) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	enabled := 0
	if start {
		enabled = 1
	}
	_, err = s.query.UpdateQuery("humans", map[string]any{"enabled": enabled}, map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleHumanOne(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"human":  human,
	})
}

func decodeStringArray(r *http.Request) ([]string, error) {
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.ToLower(s))
			}
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid payload")
	}
}

func decodeAnyArray(r *http.Request) ([]any, error) {
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case []any:
		return v, nil
	case []string:
		out := make([]any, 0, len(v))
		for _, item := range v {
			out = append(out, item)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("invalid payload")
	}
}

func numberFromAnyOrDefault(value any, fallback int) int {
	if n, ok := numberFromAny(value); ok {
		return n
	}
	return fallback
}

func getAdminIDs(cfg *config.Config, path string) []string {
	list, _ := cfg.GetStringSlice(path)
	return list
}

func handleProfilesDelete(w http.ResponseWriter, s *Server, id string, profileNo int) {
	logic := profile.New(s.query, id)
	if err := logic.DeleteProfile(profileNo); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleProfilesAdd(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}
	logic := profile.New(s.query, id)
	for _, row := range rows {
		name, ok := row["name"].(string)
		if !ok || name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"status":  "error",
				"message": "name must be specified",
			})
			return
		}
		if err := logic.AddProfile(name, row["active_hours"]); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleProfilesUpdate(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}
	logic := profile.New(s.query, id)
	for _, row := range rows {
		profileNo, ok := numberFromAny(row["profile_no"])
		if !ok {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"status":  "error",
				"message": "profile_no must be specified",
			})
			return
		}
		if err := logic.UpdateHours(profileNo, row["active_hours"]); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleProfilesCopy(w http.ResponseWriter, s *Server, id string, from int, to int) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	logic := profile.New(s.query, id)
	if err := logic.CopyProfile(from, to); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func decodeJSONRows(r *http.Request) ([]map[string]any, error) {
	var payload any
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return nil, err
	}
	switch v := payload.(type) {
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if row, ok := item.(map[string]any); ok {
				out = append(out, row)
			}
		}
		return out, nil
	case map[string]any:
		return []map[string]any{v}, nil
	default:
		return nil, fmt.Errorf("invalid payload")
	}
}

func numberFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	case int64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

func handleTrackingPokemonGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := s.query.SelectAllQuery("monsters", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		description := tracking.MonsterRowText(s.cfg, translator, s.data, row)
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = description
		out = append(out, clone)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"pokemon": out,
	})
}

func handleTrackingPokemonDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("monsters", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if s.processor != nil {
		s.processor.RefreshAlertCacheAsync()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleTrackingPokemonDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("monsters", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if s.processor != nil {
		s.processor.RefreshAlertCacheAsync()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleTrackingPokemonUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}

	insert := make([]map[string]any, 0)
	updates := make([]map[string]any, 0)
	for _, row := range rows {
		cleanRow, err := cleanMonsterRow(s.cfg, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
		if cleanRow["uid"] != nil {
			updates = append(updates, cleanRow)
		} else {
			insert = append(insert, cleanRow)
		}
	}

	existing, err := s.query.SelectAllQuery("monsters", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	alreadyPresent := make([]map[string]any, 0)
	filteredInsert := make([]map[string]any, 0)
	filteredUpdates := updates
	for _, candidate := range insert {
		if intFromAny(candidate["pokemon_id"]) == 0 {
			filteredInsert = append(filteredInsert, candidate)
			continue
		}
		matched := false
		for _, existingRow := range existing {
			if intFromAny(existingRow["pokemon_id"]) != intFromAny(candidate["pokemon_id"]) {
				continue
			}
			diffKeys := diffMonster(candidate, existingRow)
			if len(diffKeys) == 0 {
				alreadyPresent = append(alreadyPresent, candidate)
				matched = true
				break
			}
			if len(diffKeys) == 1 && (containsString(diffKeys, "min_iv") || containsString(diffKeys, "distance") || containsString(diffKeys, "template") || containsString(diffKeys, "clean")) {
				candidate["uid"] = existingRow["uid"]
				filteredUpdates = append(filteredUpdates, candidate)
				matched = true
				break
			}
		}
		if !matched {
			filteredInsert = append(filteredInsert, candidate)
		}
	}

	message := ""
	total := len(alreadyPresent) + len(filteredUpdates) + len(filteredInsert)
	if total > 50 {
		message = translator.TranslateFormat("I have made a lot of changes. See {0}{1} for details", "!", translator.Translate("tracked", false))
	} else {
		for _, row := range alreadyPresent {
			message += translator.Translate("Unchanged: ", false) + tracking.MonsterRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range filteredUpdates {
			message += translator.Translate("Updated: ", false) + tracking.MonsterRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range filteredInsert {
			message += translator.Translate("New: ", false) + tracking.MonsterRowText(s.cfg, translator, s.data, row) + "\n"
		}
	}

	if len(filteredInsert) > 0 {
		if _, err := s.query.InsertQuery("monsters", filteredInsert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}
	for _, row := range filteredUpdates {
		uid := row["uid"]
		delete(row, "uid")
		if _, err := s.query.UpdateQuery("monsters", row, map[string]any{"uid": uid}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	if s.processor != nil {
		s.processor.RefreshAlertCacheAsync()
	}
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": trimmed,
	})
}

func handleTrackingRaidGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := s.query.SelectAllQuery("raid", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		description := tracking.RaidRowText(s.cfg, translator, s.data, row, s.scanner)
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = description
		out = append(out, clone)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"raid":   out,
	})
}

func handleTrackingRaidDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("raid", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleTrackingRaidDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("raid", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleTrackingRaidUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanRaidRow(s.cfg, s.data, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("raid", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	updates := make([]map[string]any, 0)
	alreadyPresent := make([]map[string]any, 0)
	for i := len(insert) - 1; i >= 0; i-- {
		toInsert := insert[i]
		for _, existing := range trackedRows {
			if intFromAny(existing["team"]) != intFromAny(toInsert["team"]) {
				continue
			}
			diffKeys := diffRaid(toInsert, existing)
			if len(diffKeys) == 0 {
				alreadyPresent = append(alreadyPresent, toInsert)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffKeys) == 1 && (containsString(diffKeys, "distance") || containsString(diffKeys, "template") || containsString(diffKeys, "clean")) {
				clone := map[string]any{}
				for key, value := range toInsert {
					clone[key] = value
				}
				clone["uid"] = existing["uid"]
				updates = append(updates, clone)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
		}
	}

	message := ""
	total := len(alreadyPresent) + len(updates) + len(insert)
	if total > 50 {
		message = translator.TranslateFormat("I have made a lot of changes. See {0}{1} for details", "!", translator.Translate("tracked", false))
	} else {
		for _, row := range alreadyPresent {
			message += translator.Translate("Unchanged: ", false) + tracking.RaidRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.RaidRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.RaidRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
	}

	if len(updates) > 0 {
		uids := make([]any, 0, len(updates))
		insertUpdates := make([]map[string]any, 0, len(updates))
		for _, row := range updates {
			if row["uid"] != nil {
				uids = append(uids, row["uid"])
			}
			clone := map[string]any{}
			for key, value := range row {
				if key == "uid" {
					continue
				}
				clone[key] = value
			}
			insertUpdates = append(insertUpdates, clone)
		}
		if len(uids) > 0 {
			_, err = s.query.DeleteWhereInQuery("raid", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"status":  "error",
					"message": "Exception raised during execution",
				})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("raid", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": trimmed,
	})
}

func handleTrackingEggGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := s.query.SelectAllQuery("egg", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		description := tracking.EggRowText(s.cfg, translator, s.data, row, s.scanner)
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = description
		out = append(out, clone)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"egg":    out,
	})
}

func handleTrackingEggDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("egg", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleTrackingEggDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("egg", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleTrackingEggUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "error",
			"message": "User not found",
		})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"status":  "error",
			"message": "invalid payload",
		})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanEggRow(s.cfg, s.data, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("egg", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	updates := make([]map[string]any, 0)
	alreadyPresent := make([]map[string]any, 0)
	for i := len(insert) - 1; i >= 0; i-- {
		toInsert := insert[i]
		for _, existing := range trackedRows {
			if intFromAny(existing["team"]) != intFromAny(toInsert["team"]) {
				continue
			}
			diffKeys := diffEgg(toInsert, existing)
			if len(diffKeys) == 0 {
				alreadyPresent = append(alreadyPresent, toInsert)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffKeys) == 1 && (containsString(diffKeys, "distance") || containsString(diffKeys, "template") || containsString(diffKeys, "clean")) {
				clone := map[string]any{}
				for key, value := range toInsert {
					clone[key] = value
				}
				clone["uid"] = existing["uid"]
				updates = append(updates, clone)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
		}
	}

	message := ""
	total := len(alreadyPresent) + len(updates) + len(insert)
	if total > 50 {
		message = translator.TranslateFormat("I have made a lot of changes. See {0}{1} for details", "!", translator.Translate("tracked", false))
	} else {
		for _, row := range alreadyPresent {
			message += translator.Translate("Unchanged: ", false) + tracking.EggRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.EggRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.EggRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
	}

	if len(updates) > 0 {
		uids := make([]any, 0, len(updates))
		insertUpdates := make([]map[string]any, 0, len(updates))
		for _, row := range updates {
			if row["uid"] != nil {
				uids = append(uids, row["uid"])
			}
			clone := map[string]any{}
			for key, value := range row {
				if key == "uid" {
					continue
				}
				clone[key] = value
			}
			insertUpdates = append(insertUpdates, clone)
		}
		if len(uids) > 0 {
			_, err = s.query.DeleteWhereInQuery("egg", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"status":  "error",
					"message": "Exception raised during execution",
				})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("egg", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": trimmed,
	})
}

func cleanMonsterRow(cfg *config.Config, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	if row["pokemon_id"] == nil {
		return nil, fmt.Errorf("Pokemon id must be specified")
	}
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	distance := floatFromAny(row["distance"])
	if distance == 0 {
		if def, ok := cfg.GetInt("tracking.defaultDistance"); ok {
			distance = float64(def)
		}
	}
	maxDistance, _ := cfg.GetInt("tracking.maxDistance")
	if maxDistance == 0 {
		maxDistance = 40000000
	}
	if distance > float64(maxDistance) {
		distance = float64(maxDistance)
	}
	newRow := map[string]any{
		"id":         id,
		"profile_no": numberFromAnyOrDefault(row["profile_no"], currentProfile),
		"ping":       "",
		"template":   getStringValue(row["template"], nil, ""),
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	newRow["pokemon_id"] = intFromAny(row["pokemon_id"])
	newRow["distance"] = int(distance)
	newRow["min_iv"] = defaultInt(row["min_iv"], -1)
	newRow["max_iv"] = defaultInt(row["max_iv"], 100)
	newRow["min_cp"] = defaultInt(row["min_cp"], 0)
	newRow["max_cp"] = defaultInt(row["max_cp"], 9000)
	newRow["min_level"] = defaultInt(row["min_level"], 0)
	newRow["max_level"] = defaultInt(row["max_level"], 55)
	newRow["atk"] = defaultInt(row["atk"], 0)
	newRow["def"] = defaultInt(row["def"], 0)
	newRow["sta"] = defaultInt(row["sta"], 0)
	newRow["min_weight"] = defaultInt(row["min_weight"], 0)
	newRow["max_weight"] = defaultInt(row["max_weight"], 9000000)
	newRow["form"] = defaultInt(row["form"], 0)
	newRow["max_atk"] = defaultInt(row["max_atk"], 15)
	newRow["max_def"] = defaultInt(row["max_def"], 15)
	newRow["max_sta"] = defaultInt(row["max_sta"], 15)
	newRow["gender"] = defaultInt(row["gender"], 0)
	newRow["clean"] = defaultInt(row["clean"], 0)
	newRow["pvp_ranking_league"] = defaultInt(row["pvp_ranking_league"], 0)
	newRow["pvp_ranking_best"] = defaultInt(row["pvp_ranking_best"], 1)
	newRow["pvp_ranking_worst"] = defaultInt(row["pvp_ranking_worst"], 4096)
	newRow["pvp_ranking_min_cp"] = defaultInt(row["pvp_ranking_min_cp"], 0)
	newRow["pvp_ranking_cap"] = defaultInt(row["pvp_ranking_cap"], 0)
	newRow["size"] = defaultInt(row["size"], -1)
	newRow["max_size"] = defaultInt(row["max_size"], 5)
	newRow["rarity"] = defaultInt(row["rarity"], -1)
	newRow["max_rarity"] = defaultInt(row["max_rarity"], 6)
	newRow["min_time"] = defaultInt(row["min_time"], 0)

	if uid := row["uid"]; uid != nil {
		newRow["uid"] = uid
	}
	return newRow, nil
}

func cleanRaidRow(cfg *config.Config, game *data.GameData, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	monsterID := defaultInt(row["pokemon_id"], 9000)
	level := 9000
	if monsterID == 9000 {
		level = defaultInt(row["level"], -1)
		maxLevel := raidMaxLevel(game.UtilData)
		if level < 1 || (level > maxLevel && level != 90) {
			return nil, fmt.Errorf("Invalid level (must be specified if no pokemon_id")
		}
	}

	team := defaultInt(row["team"], 4)
	if team < 0 || team > 4 {
		team = 4
	}
	rsvp := defaultInt(row["rsvp_changes"], 0)
	if rsvp < 0 || rsvp > 2 {
		rsvp = 0
	}

	newRow := map[string]any{
		"id":           id,
		"profile_no":   currentProfile,
		"ping":         "",
		"template":     getStringValue(row["template"], nil, ""),
		"pokemon_id":   monsterID,
		"exclusive":    defaultInt(row["exclusive"], 0),
		"distance":     defaultInt(row["distance"], 0),
		"team":         team,
		"clean":        defaultInt(row["clean"], 0),
		"level":        level,
		"form":         defaultInt(row["form"], 0),
		"move":         defaultInt(row["move"], 9000),
		"evolution":    defaultInt(row["evolution"], 9000),
		"gym_id":       row["gym_id"],
		"rsvp_changes": rsvp,
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func diffRaid(candidate map[string]any, existing map[string]any) []string {
	keys := []string{}
	for key, value := range candidate {
		if key == "uid" {
			continue
		}
		if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", existing[key]) {
			keys = append(keys, key)
		}
	}
	return keys
}

func cleanEggRow(cfg *config.Config, game *data.GameData, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	level := defaultInt(row["level"], -1)
	maxLevel := raidMaxLevel(game.UtilData)
	if level < 1 || (level > maxLevel && level != 90) {
		return nil, fmt.Errorf("Invalid level")
	}

	team := defaultInt(row["team"], 4)
	if team < 0 || team > 4 {
		team = 4
	}
	rsvp := defaultInt(row["rsvp_changes"], 0)
	if rsvp < 0 || rsvp > 2 {
		rsvp = 0
	}

	newRow := map[string]any{
		"id":           id,
		"profile_no":   currentProfile,
		"ping":         "",
		"template":     getStringValue(row["template"], nil, ""),
		"exclusive":    defaultInt(row["exclusive"], 0),
		"distance":     defaultInt(row["distance"], 0),
		"team":         team,
		"clean":        defaultInt(row["clean"], 0),
		"level":        level,
		"gym_id":       row["gym_id"],
		"rsvp_changes": rsvp,
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func diffEgg(candidate map[string]any, existing map[string]any) []string {
	keys := []string{}
	for key, value := range candidate {
		if key == "uid" {
			continue
		}
		if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", existing[key]) {
			keys = append(keys, key)
		}
	}
	return keys
}

func raidMaxLevel(util map[string]any) int {
	entry, ok := util["raidLevels"]
	if !ok {
		return 0
	}
	max := 0
	switch v := entry.(type) {
	case map[string]any:
		for key := range v {
			level, err := strconv.Atoi(key)
			if err == nil && level > max {
				max = level
			}
		}
	}
	if max == 0 {
		return 90
	}
	return max
}

func diffMonster(candidate map[string]any, existing map[string]any) []string {
	keys := []string{}
	for key, value := range candidate {
		if key == "uid" {
			continue
		}
		if fmt.Sprintf("%v", value) != fmt.Sprintf("%v", existing[key]) {
			keys = append(keys, key)
		}
	}
	return keys
}

func defaultInt(value any, fallback int) int {
	if value == nil {
		return fallback
	}
	if v, ok := numberFromAny(value); ok {
		return v
	}
	return fallback
}

func intFromAny(value any) int {
	if v, ok := numberFromAny(value); ok {
		return v
	}
	return 0
}

func floatFromAny(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
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

func getStringValue(value any, cfg *config.Config, path string) string {
	if value == nil {
		if cfg == nil {
			return ""
		}
		fallback, _ := cfg.GetString(path)
		return fallback
	}
	return fmt.Sprintf("%v", value)
}

func getString(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%v", value)
}
