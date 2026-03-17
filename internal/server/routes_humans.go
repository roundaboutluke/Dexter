package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"poraclego/internal/db"
)

func registerHumanRoutes(s *Server, mux *http.ServeMux) {
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
	fences := s.getFences()
	areas := fences.PointInArea([]float64{lat, lon})
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

	fences := s.getFences()
	allowedAreas := make([]string, 0, len(fences.Fences))
	for _, fence := range fences.Fences {
		allowedAreas = append(allowedAreas, strings.ToLower(fence.Name))
	}

	areaSecurityEnabled, _ := s.cfg.GetBool("areaSecurity.enabled")
	if areaSecurityEnabled && !isAdmin(s.cfg, id) {
		communityMembership := parseCommunityMembership(human["community_membership"])
		allowedAreas = filterAreas(s.cfg, communityMembership, allowedAreas)
	}

	areas := make([]map[string]any, 0, len(fences.Fences))
	for _, fence := range fences.Fences {
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
		areas := s.getFences().PointInArea([]float64{lat, lon})
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
	if err := withAlertStateTx(s, func(query *db.Query) error {
		if _, err := query.UpdateQuery("humans", map[string]any{"latitude": lat, "longitude": lon}, map[string]any{"id": id}); err != nil {
			return err
		}
		if _, err := query.UpdateQuery("profiles", map[string]any{"latitude": lat, "longitude": lon}, map[string]any{"id": id, "profile_no": currentProfile}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	refreshAlertState(s)
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

	refreshAlertState(s)
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

	fences := s.getFences()
	allowedAreas := make([]string, 0, len(fences.Fences))
	for _, fence := range fences.Fences {
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
	if err := withAlertStateTx(s, func(query *db.Query) error {
		if _, err := query.UpdateQuery("humans", map[string]any{"area": string(areasJSON)}, map[string]any{"id": id}); err != nil {
			return err
		}
		if _, err := query.UpdateQuery("profiles", map[string]any{"area": string(areasJSON)}, map[string]any{"id": id, "profile_no": currentProfile}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}

	refreshAlertState(s)
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
	refreshAlertState(s)
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleHumanOne(w http.ResponseWriter, s *Server, id string) {
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
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"human":  human,
	})
}
