package server

import (
	"fmt"
	"net/http"
	"strings"

	"poraclego/internal/config"
	"poraclego/internal/tracking"
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
		clone["description"] = tracking.GymRowText(s.cfg, translator, s.data, row, s.scanner)
		gymWithDesc = append(gymWithDesc, clone)
	}
	raidWithDesc := make([]map[string]any, 0, len(raid))
	for _, row := range raid {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.RaidRowText(s.cfg, translator, s.data, row, s.scanner)
		raidWithDesc = append(raidWithDesc, clone)
	}
	eggWithDesc := make([]map[string]any, 0, len(egg))
	for _, row := range egg {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.EggRowText(s.cfg, translator, s.data, row, s.scanner)
		eggWithDesc = append(eggWithDesc, clone)
	}
	maxbattleWithDesc := make([]map[string]any, 0, len(maxbattle))
	for _, row := range maxbattle {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.MaxbattleRowText(s.cfg, translator, s.data, row)
		maxbattleWithDesc = append(maxbattleWithDesc, clone)
	}
	pokemonWithDesc := make([]map[string]any, 0, len(pokemon))
	for _, row := range pokemon {
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = tracking.MonsterRowText(s.cfg, translator, s.data, row)
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
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	rows, err := s.query.SelectAllQuery("quest", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "quest": rows})
}

func handleTrackingQuestDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("quest", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingQuestDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("quest", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingQuestUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanQuestRow(s.cfg, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("quest", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}

	updates := make([]map[string]any, 0)
	alreadyPresent := make([]map[string]any, 0)
	for i := len(insert) - 1; i >= 0; i-- {
		toInsert := insert[i]
		for _, existing := range trackedRows {
			if intFromAny(existing["reward_type"]) != intFromAny(toInsert["reward_type"]) || intFromAny(existing["reward"]) != intFromAny(toInsert["reward"]) {
				continue
			}
			diffKeys := diffGeneric(toInsert, existing)
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
			message += translator.Translate("Unchanged: ", false) + tracking.QuestRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.QuestRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.QuestRowText(s.cfg, translator, s.data, row) + "\n"
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
			_, err = s.query.DeleteWhereInQuery("quest", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("quest", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": trimmed})
}

func handleTrackingInvasionGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	rows, err := s.query.SelectAllQuery("invasion", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "invasion": rows})
}

func handleTrackingInvasionDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("invasion", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingInvasionDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("invasion", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingInvasionUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanInvasionRow(s.cfg, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("invasion", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}

	updates := make([]map[string]any, 0)
	alreadyPresent := make([]map[string]any, 0)
	for i := len(insert) - 1; i >= 0; i-- {
		toInsert := insert[i]
		for _, existing := range trackedRows {
			if getString(existing["grunt_type"]) != getString(toInsert["grunt_type"]) {
				continue
			}
			diffKeys := diffGeneric(toInsert, existing)
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
			message += translator.Translate("Unchanged: ", false) + tracking.InvasionRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.InvasionRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.InvasionRowText(s.cfg, translator, s.data, row) + "\n"
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
			_, err = s.query.DeleteWhereInQuery("invasion", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("invasion", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": trimmed})
}

func handleTrackingLureGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	rows, err := s.query.SelectAllQuery("lures", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "lure": rows})
}

func handleTrackingLureDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("lures", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingLureDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("lures", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingLureUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanLureRow(s.cfg, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("lures", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}

	updates := make([]map[string]any, 0)
	alreadyPresent := make([]map[string]any, 0)
	for i := len(insert) - 1; i >= 0; i-- {
		toInsert := insert[i]
		for _, existing := range trackedRows {
			if intFromAny(existing["lure_id"]) != intFromAny(toInsert["lure_id"]) {
				continue
			}
			diffKeys := diffGeneric(toInsert, existing)
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
			message += translator.Translate("Unchanged: ", false) + tracking.LureRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.LureRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.LureRowText(s.cfg, translator, s.data, row) + "\n"
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
			_, err = s.query.DeleteWhereInQuery("lures", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("lures", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": trimmed})
}

func handleTrackingNestGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	rows, err := s.query.SelectAllQuery("nests", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "nest": rows})
}

func handleTrackingNestDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("nests", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingNestDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("nests", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingNestUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanNestRow(s.cfg, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("nests", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}

	updates := make([]map[string]any, 0)
	alreadyPresent := make([]map[string]any, 0)
	for i := len(insert) - 1; i >= 0; i-- {
		toInsert := insert[i]
		for _, existing := range trackedRows {
			if intFromAny(existing["pokemon_id"]) != intFromAny(toInsert["pokemon_id"]) {
				continue
			}
			diffKeys := diffGeneric(toInsert, existing)
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
			message += translator.Translate("Unchanged: ", false) + tracking.NestRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.NestRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.NestRowText(s.cfg, translator, s.data, row) + "\n"
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
			_, err = s.query.DeleteWhereInQuery("nests", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("nests", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": trimmed})
}

func handleTrackingGymGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	rows, err := s.query.SelectAllQuery("gym", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		description := tracking.GymRowText(s.cfg, translator, s.data, row, s.scanner)
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = description
		out = append(out, clone)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "gym": out})
}

func handleTrackingGymDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("gym", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingGymDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("gym", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingGymUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanGymRow(s.cfg, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("gym", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
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
			diffKeys := diffGeneric(toInsert, existing)
			if len(diffKeys) == 0 {
				alreadyPresent = append(alreadyPresent, toInsert)
				insert = append(insert[:i], insert[i+1:]...)
				break
			}
			if len(diffKeys) == 1 && (containsString(diffKeys, "distance") || containsString(diffKeys, "template") || containsString(diffKeys, "clean") || containsString(diffKeys, "slot_changes") || containsString(diffKeys, "battle_changes")) {
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
			message += translator.Translate("Unchanged: ", false) + tracking.GymRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.GymRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.GymRowText(s.cfg, translator, s.data, row, s.scanner) + "\n"
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
			_, err = s.query.DeleteWhereInQuery("gym", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("gym", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": trimmed})
}

func handleTrackingMaxbattleGet(w http.ResponseWriter, s *Server, id string) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)
	rows, err := s.query.SelectAllQuery("maxbattle", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		description := tracking.MaxbattleRowText(s.cfg, translator, s.data, row)
		clone := map[string]any{}
		for key, value := range row {
			clone[key] = value
		}
		clone["description"] = description
		out = append(out, clone)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "maxbattle": out})
}

func handleTrackingMaxbattleDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	_, err := s.query.DeleteQuery("maxbattle", map[string]any{"id": id, "uid": uid})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingMaxbattleDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	_, err = s.query.DeleteWhereInQuery("maxbattle", map[string]any{"id": id}, values, "uid")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingMaxbattleUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	translator := s.i18n.Translator(language)
	currentProfile := numberFromAnyOrDefault(human["current_profile_no"], 1)

	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}

	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cleanMaxbattleRow(s.cfg, id, currentProfile, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery("maxbattle", map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}

	stationValue := func(row map[string]any) string {
		return strings.TrimSpace(getString(row["station_id"]))
	}

	updates := make([]map[string]any, 0)
	alreadyPresent := make([]map[string]any, 0)
	for i := len(insert) - 1; i >= 0; i-- {
		toInsert := insert[i]
		for _, existing := range trackedRows {
			if intFromAny(existing["pokemon_id"]) != intFromAny(toInsert["pokemon_id"]) {
				continue
			}
			if intFromAny(existing["gmax"]) != intFromAny(toInsert["gmax"]) {
				continue
			}
			if intFromAny(existing["level"]) != intFromAny(toInsert["level"]) {
				continue
			}
			if intFromAny(existing["form"]) != intFromAny(toInsert["form"]) {
				continue
			}
			if intFromAny(existing["move"]) != intFromAny(toInsert["move"]) {
				continue
			}
			if intFromAny(existing["evolution"]) != intFromAny(toInsert["evolution"]) {
				continue
			}
			if stationValue(existing) != stationValue(toInsert) {
				continue
			}

			diffKeys := diffGeneric(toInsert, existing)
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
			message += translator.Translate("Unchanged: ", false) + tracking.MaxbattleRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range updates {
			message += translator.Translate("Updated: ", false) + tracking.MaxbattleRowText(s.cfg, translator, s.data, row) + "\n"
		}
		for _, row := range insert {
			message += translator.Translate("New: ", false) + tracking.MaxbattleRowText(s.cfg, translator, s.data, row) + "\n"
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
			_, err = s.query.DeleteWhereInQuery("maxbattle", map[string]any{"id": id, "profile_no": currentProfile}, uids, "uid")
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
				return
			}
		}
		insert = append(insert, insertUpdates...)
	}

	if len(insert) > 0 {
		if _, err := s.query.InsertQuery("maxbattle", insert); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": trimmed})
}

func cleanQuestRow(cfg *config.Config, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	rewardType := defaultInt(row["reward_type"], -1)
	if rewardType != 3 && rewardType != 12 && rewardType != 4 && rewardType != 7 && rewardType != 2 {
		return nil, fmt.Errorf("Unrecognised reward_type value")
	}
	newRow := map[string]any{
		"id":          id,
		"profile_no":  currentProfile,
		"ping":        "",
		"template":    getStringValue(row["template"], nil, ""),
		"distance":    defaultInt(row["distance"], 0),
		"clean":       defaultInt(row["clean"], 0),
		"reward_type": rewardType,
		"reward":      defaultInt(row["reward"], 0),
		"amount":      defaultInt(row["amount"], 0),
		"form":        defaultInt(row["form"], 0),
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func cleanInvasionRow(cfg *config.Config, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	if getString(row["grunt_type"]) == "" {
		return nil, fmt.Errorf("Grunt type mandatory")
	}
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	newRow := map[string]any{
		"id":         id,
		"profile_no": currentProfile,
		"ping":       "",
		"template":   getStringValue(row["template"], nil, ""),
		"distance":   defaultInt(row["distance"], 0),
		"clean":      defaultInt(row["clean"], 0),
		"gender":     defaultInt(row["gender"], 0),
		"grunt_type": getString(row["grunt_type"]),
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func cleanLureRow(cfg *config.Config, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	lureID := defaultInt(row["lure_id"], -1)
	if lureID != 0 && lureID != 501 && lureID != 502 && lureID != 503 && lureID != 504 && lureID != 505 && lureID != 506 {
		return nil, fmt.Errorf("Unrecognised lure_id value")
	}
	newRow := map[string]any{
		"id":         id,
		"profile_no": currentProfile,
		"ping":       "",
		"template":   getStringValue(row["template"], nil, ""),
		"distance":   defaultInt(row["distance"], 0),
		"clean":      defaultInt(row["clean"], 0),
		"lure_id":    lureID,
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func cleanNestRow(cfg *config.Config, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	newRow := map[string]any{
		"id":            id,
		"profile_no":    currentProfile,
		"ping":          "",
		"template":      getStringValue(row["template"], nil, ""),
		"distance":      defaultInt(row["distance"], 0),
		"clean":         defaultInt(row["clean"], 0),
		"pokemon_id":    defaultInt(row["pokemon_id"], 0),
		"min_spawn_avg": defaultInt(row["min_spawn_avg"], 0),
		"form":          defaultInt(row["form"], 0),
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func cleanGymRow(cfg *config.Config, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	team := defaultInt(row["team"], -1)
	if team < 0 || team > 4 {
		return nil, fmt.Errorf("Invalid team")
	}
	newRow := map[string]any{
		"id":             id,
		"profile_no":     currentProfile,
		"ping":           "",
		"template":       getStringValue(row["template"], nil, ""),
		"distance":       defaultInt(row["distance"], 0),
		"clean":          defaultInt(row["clean"], 0),
		"team":           team,
		"slot_changes":   defaultInt(row["slot_changes"], 0),
		"battle_changes": defaultInt(row["battle_changes"], 0),
		"gym_id":         row["gym_id"],
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func cleanMaxbattleRow(cfg *config.Config, id string, currentProfile int, row map[string]any) (map[string]any, error) {
	defaultTemplate, _ := cfg.GetString("general.defaultTemplateName")
	if defaultTemplate == "" {
		defaultTemplate = "1"
	}
	pokemonID := defaultInt(row["pokemon_id"], 0)
	if pokemonID != 9000 && pokemonID <= 0 {
		return nil, fmt.Errorf("Invalid pokemon_id")
	}
	gmax := defaultInt(row["gmax"], 0)
	if gmax != 0 && gmax != 1 {
		return nil, fmt.Errorf("Invalid gmax")
	}
	move := defaultInt(row["move"], 9000)
	evolution := defaultInt(row["evolution"], 9000)
	stationID := strings.TrimSpace(getString(row["station_id"]))
	var stationValue any
	if stationID != "" {
		stationValue = stationID
	} else {
		stationValue = nil
	}
	newRow := map[string]any{
		"id":         id,
		"profile_no": currentProfile,
		"ping":       "",
		"template":   getStringValue(row["template"], nil, ""),
		"distance":   defaultInt(row["distance"], 0),
		"clean":      defaultInt(row["clean"], 0),
		"pokemon_id": pokemonID,
		"gmax":       gmax,
		"level":      defaultInt(row["level"], 0),
		"form":       defaultInt(row["form"], 0),
		"move":       move,
		"evolution":  evolution,
		"station_id": stationValue,
	}
	if newRow["template"] == "" {
		newRow["template"] = defaultTemplate
	}
	return newRow, nil
}

func diffGeneric(candidate map[string]any, existing map[string]any) []string {
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
