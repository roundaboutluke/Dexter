package server

import (
	"net/http"
	"strings"

	"poraclego/internal/db"
	"poraclego/internal/tracking"
)

func registerTrackingPokemonRoutes(s *Server, mux *http.ServeMux) {
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

func handleTrackingPokemonGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, monsterTrackingRouteConfig(s))
}

func handleTrackingPokemonDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "monsters")
}

func handleTrackingPokemonDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "monsters")
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
		cleanRow, err := tracking.CleanMonsterRow(s.cfg, tracking.RuleScope{UserID: id, ProfileNo: currentProfile}, row)
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
	planned := make([]map[string]any, 0, len(insert))
	for _, candidate := range insert {
		if intFromAny(candidate["pokemon_id"]) == 0 {
			filteredInsert = append(filteredInsert, candidate)
			continue
		}
		planned = append(planned, candidate)
	}
	plan := tracking.PlanUpsert(planned, existing, func(candidate, existing map[string]any) bool {
		return intFromAny(existing["pokemon_id"]) == intFromAny(candidate["pokemon_id"])
	}, "min_iv", "distance", "template", "clean")
	alreadyPresent = append(alreadyPresent, plan.Unchanged...)
	filteredUpdates = append(filteredUpdates, plan.Updates...)
	filteredInsert = append(filteredInsert, plan.Inserts...)

	message := tracking.ChangeMessage(
		translator,
		"!",
		translator.Translate("tracked", false),
		tracking.UpsertPlan{
			Unchanged: alreadyPresent,
			Updates:   filteredUpdates,
			Inserts:   filteredInsert,
		},
		func(row map[string]any) string {
			return tracking.MonsterRowText(s.cfg, translator, s.data, row)
		},
	)

	if len(filteredInsert)+len(filteredUpdates) > 0 {
		if err := withAlertStateTx(s, func(query *db.Query) error {
			if len(filteredInsert) > 0 {
				if _, err := query.InsertQuery("monsters", filteredInsert); err != nil {
					return err
				}
			}
			for _, row := range filteredUpdates {
				uid := row["uid"]
				update := map[string]any{}
				for key, value := range row {
					if key == "uid" {
						continue
					}
					update[key] = value
				}
				if _, err := query.UpdateQuery("monsters", update, map[string]any{"uid": uid}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"status":  "error",
				"message": "Exception raised during execution",
			})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	refreshAlertState(s)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"message": trimmed,
	})
}
