package server

import (
	"crypto/md5"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/geofence"
	"poraclego/internal/tileserver"
)

func registerGeofenceRoutes(s *Server, mux *http.ServeMux) {
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
		refreshAlertState(s)
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
