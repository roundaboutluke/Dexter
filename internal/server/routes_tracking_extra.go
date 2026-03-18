package server

import (
	"net/http"
	"strings"

	"dexter/internal/i18n"
	"dexter/internal/tracking"
)

func registerTrackingExtras(s *Server, mux *http.ServeMux) {
	mux.HandleFunc("/api/tracking/all/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/all/")
		id := strings.Trim(path, "/")
		if id == "" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handleTrackingAll(w, s, id)
	})

	mux.HandleFunc("/api/tracking/allProfiles/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/allProfiles/")
		id := strings.Trim(path, "/")
		if id == "" || r.Method != http.MethodGet {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		handleTrackingAllProfiles(w, s, id)
	})

	mux.HandleFunc("/api/tracking/quest/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/quest/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingQuestGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingQuestDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingQuestDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingQuestUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/invasion/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/invasion/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingInvasionGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingInvasionDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingInvasionDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingInvasionUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/lure/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/lure/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingLureGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingLureDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingLureDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingLureUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/nest/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/nest/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingNestGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingNestDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingNestDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingNestUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/gym/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/gym/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingGymGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingGymDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingGymDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingGymUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/tracking/maxbattle/", func(w http.ResponseWriter, r *http.Request) {
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		if rejectNotAuthorized(s.cfg, r, w) {
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/tracking/maxbattle/")
		parts := strings.Split(strings.Trim(path, "/"), "/")
		if len(parts) < 1 || parts[0] == "" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		id := parts[0]
		switch r.Method {
		case http.MethodGet:
			if len(parts) == 1 {
				handleTrackingMaxbattleGet(w, s, id)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodDelete:
			if len(parts) == 3 && parts[1] == "byUid" {
				handleTrackingMaxbattleDelete(w, s, id, parts[2])
				return
			}
			w.WriteHeader(http.StatusNotFound)
		case http.MethodPost:
			if len(parts) == 2 && parts[1] == "delete" {
				handleTrackingMaxbattleDeleteBatch(w, s, id, r)
				return
			}
			if len(parts) == 1 {
				handleTrackingMaxbattleUpsert(w, s, id, r)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
}

func handleTrackingAll(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	pokemon, err := s.query.SelectAllQuery("monsters", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	raid, _ := s.query.SelectAllQuery("raid", map[string]any{"id": id, "profile_no": currentProfile})
	egg, _ := s.query.SelectAllQuery("egg", map[string]any{"id": id, "profile_no": currentProfile})
	maxbattle, _ := s.query.SelectAllQuery("maxbattle", map[string]any{"id": id, "profile_no": currentProfile})
	quest, _ := s.query.SelectAllQuery("quest", map[string]any{"id": id, "profile_no": currentProfile})
	invasion, _ := s.query.SelectAllQuery("invasion", map[string]any{"id": id, "profile_no": currentProfile})
	lure, _ := s.query.SelectAllQuery("lures", map[string]any{"id": id, "profile_no": currentProfile})
	nest, _ := s.query.SelectAllQuery("nests", map[string]any{"id": id, "profile_no": currentProfile})
	gym, _ := s.query.SelectAllQuery("gym", map[string]any{"id": id, "profile_no": currentProfile})
	profile, _ := s.query.SelectOneQuery("profiles", map[string]any{"id": id, "profile_no": currentProfile})

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"human":     human,
		"gym":       gym,
		"raid":      raid,
		"egg":       egg,
		"maxbattle": maxbattle,
		"pokemon":   pokemon,
		"invasion":  invasion,
		"lure":      lure,
		"nest":      nest,
		"quest":     quest,
		"profile":   profile,
	})
}

func handleTrackingAllProfiles(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}

	pokemon, err := s.query.SelectAllQuery("monsters", map[string]any{"id": id})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	raid, _ := s.query.SelectAllQuery("raid", map[string]any{"id": id})
	egg, _ := s.query.SelectAllQuery("egg", map[string]any{"id": id})
	maxbattle, _ := s.query.SelectAllQuery("maxbattle", map[string]any{"id": id})
	quest, _ := s.query.SelectAllQuery("quest", map[string]any{"id": id})
	invasion, _ := s.query.SelectAllQuery("invasion", map[string]any{"id": id})
	lure, _ := s.query.SelectAllQuery("lures", map[string]any{"id": id})
	nest, _ := s.query.SelectAllQuery("nests", map[string]any{"id": id})
	gym, _ := s.query.SelectAllQuery("gym", map[string]any{"id": id})
	profile, _ := s.query.SelectAllQuery("profiles", map[string]any{"id": id})

	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)

	gymWithDesc := make([]map[string]any, 0, len(gym))
	for _, row := range gym {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.GymRowText(s.cfg, translator, s.getData(), row, s.scanner)
		gymWithDesc = append(gymWithDesc, clone)
	}
	raidWithDesc := make([]map[string]any, 0, len(raid))
	for _, row := range raid {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.RaidRowText(s.cfg, translator, s.getData(), row, s.scanner)
		raidWithDesc = append(raidWithDesc, clone)
	}
	eggWithDesc := make([]map[string]any, 0, len(egg))
	for _, row := range egg {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.EggRowText(s.cfg, translator, s.getData(), row, s.scanner)
		eggWithDesc = append(eggWithDesc, clone)
	}
	maxbattleWithDesc := make([]map[string]any, 0, len(maxbattle))
	for _, row := range maxbattle {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.MaxbattleRowText(s.cfg, translator, s.getData(), row)
		maxbattleWithDesc = append(maxbattleWithDesc, clone)
	}
	pokemonWithDesc := make([]map[string]any, 0, len(pokemon))
	for _, row := range pokemon {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.MonsterRowText(s.cfg, translator, s.getData(), row)
		pokemonWithDesc = append(pokemonWithDesc, clone)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"human":     human,
		"gym":       gymWithDesc,
		"raid":      raidWithDesc,
		"egg":       eggWithDesc,
		"maxbattle": maxbattleWithDesc,
		"pokemon":   pokemonWithDesc,
		"invasion":  invasion,
		"lure":      lure,
		"nest":      nest,
		"quest":     quest,
		"profile":   profile,
	})
}

func handleTrackingQuestGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, questTrackingRouteConfig(s))
}

func handleTrackingQuestDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "quest")
}

func handleTrackingQuestDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "quest")
}

func handleTrackingQuestUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, questTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.QuestRowText(s.cfg, translator, s.getData(), row)
	})
}

func handleTrackingInvasionGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, invasionTrackingRouteConfig(s))
}

func handleTrackingInvasionDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "invasion")
}

func handleTrackingInvasionDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "invasion")
}

func handleTrackingInvasionUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, invasionTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.InvasionRowText(s.cfg, translator, s.getData(), row)
	})
}

func handleTrackingLureGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, lureTrackingRouteConfig(s))
}

func handleTrackingLureDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "lures")
}

func handleTrackingLureDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "lures")
}

func handleTrackingLureUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, lureTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.LureRowText(s.cfg, translator, s.getData(), row)
	})
}

func handleTrackingNestGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, nestTrackingRouteConfig(s))
}

func handleTrackingNestDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "nests")
}

func handleTrackingNestDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "nests")
}

func handleTrackingNestUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, nestTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.NestRowText(s.cfg, translator, s.getData(), row)
	})
}

func handleTrackingGymGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, gymTrackingRouteConfig(s))
}

func handleTrackingGymDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "gym")
}

func handleTrackingGymDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "gym")
}

func handleTrackingGymUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, gymTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.GymRowText(s.cfg, translator, s.getData(), row, s.scanner)
	})
}

func handleTrackingMaxbattleGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, maxbattleTrackingRouteConfig(s))
}

func handleTrackingMaxbattleDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "maxbattle")
}

func handleTrackingMaxbattleDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "maxbattle")
}

func handleTrackingMaxbattleUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, maxbattleTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.MaxbattleRowText(s.cfg, translator, s.getData(), row)
	})
}
