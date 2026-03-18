package server

import (
	"net/http"
	"strconv"
	"strings"

	"dexter/internal/profile"
)

func registerProfileRoutes(s *Server, mux *http.ServeMux) {
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
}

func registerAlertStateRoutes(s *Server, mux *http.ServeMux) {
	alertStateRefreshHandler := func(w http.ResponseWriter, r *http.Request) {
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
		if err := handleAlertStateRefresh(w, s); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}

	mux.HandleFunc("/api/tracking/pokemon/refresh", alertStateRefreshHandler)
	mux.HandleFunc("/api/alert-state/refresh", alertStateRefreshHandler)
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

func handleProfilesDelete(w http.ResponseWriter, s *Server, id string, profileNo int) {
	logic := profile.New(s.query, id)
	changed := false
	logic.SetRefreshAlertState(func() { changed = true })
	if err := logic.DeleteProfile(profileNo); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if changed {
		refreshAlertState(s)
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
	changed := false
	logic.SetRefreshAlertState(func() { changed = true })
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
	if changed {
		refreshAlertState(s)
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
	changed := false
	logic.SetRefreshAlertState(func() { changed = true })
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
	if changed {
		refreshAlertState(s)
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
	changed := false
	logic.SetRefreshAlertState(func() { changed = true })
	if err := logic.CopyProfile(from, to); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"status":  "error",
			"message": "Exception raised during execution",
		})
		return
	}
	if changed {
		refreshAlertState(s)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
}

func handleAlertStateRefresh(w http.ResponseWriter, s *Server) error {
	if err := refreshAlertStateSync(s); err != nil {
		return err
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
	})
	return nil
}
