package server

import (
	"net/http"
	"strings"

	"dexter/internal/i18n"
	"dexter/internal/tracking"
)

type trackingRouteConfig struct {
	table         string
	responseKey   string
	describe      func(*Server, *i18n.Translator, map[string]any) string
	clean         func(*Server, tracking.RuleScope, map[string]any) (map[string]any, error)
	sameIdentity  func(candidate, existing map[string]any) bool
	mutableFields []string
}

func loadTrackingContext(s *Server, id string) (map[string]any, string, *i18n.Translator, int, bool) {
	human, err := s.query.SelectOneQuery("humans", map[string]any{"id": id})
	if err != nil || human == nil {
		return nil, "", nil, 0, false
	}
	language := getStringValue(human["language"], s.cfg, "general.locale")
	return human, language, s.i18n.Translator(language), numberFromAnyOrDefault(human["current_profile_no"], 1), true
}

func trackingScope(id string, profileNo int) tracking.RuleScope {
	return tracking.RuleScope{UserID: id, ProfileNo: profileNo}
}

func handleTrackingGetGeneric(w http.ResponseWriter, s *Server, id string, cfg trackingRouteConfig) {
	human, _, translator, currentProfile, ok := loadTrackingContext(s, id)
	if !ok || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	rows, err := s.query.SelectAllQuery(cfg.table, map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	if cfg.describe == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", cfg.responseKey: rows})
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		clone := tracking.CloneRow(row)
		clone["description"] = cfg.describe(s, translator, row)
		out = append(out, clone)
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", cfg.responseKey: out})
}

func handleTrackingDeleteGeneric(w http.ResponseWriter, s *Server, id string, uid string, table string) {
	if _, err := s.query.DeleteQuery(table, map[string]any{"id": id, "uid": uid}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	refreshAlertState(s)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingDeleteBatchGeneric(w http.ResponseWriter, s *Server, id string, r *http.Request, table string) {
	payload, err := decodeAnyArray(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}
	values := make([]any, 0, len(payload))
	for _, item := range payload {
		values = append(values, item)
	}
	if _, err := s.query.DeleteWhereInQuery(table, map[string]any{"id": id}, values, "uid"); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}
	refreshAlertState(s)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

func handleTrackingUpsertGeneric(w http.ResponseWriter, s *Server, id string, r *http.Request, cfg trackingRouteConfig, rowText func(*i18n.Translator, map[string]any) string) {
	human, language, translator, currentProfile, ok := loadTrackingContext(s, id)
	if !ok || human == nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "error", "message": "User not found"})
		return
	}
	rows, err := decodeJSONRows(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": "invalid payload"})
		return
	}

	scope := trackingScope(id, currentProfile)
	insert := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		cleanRow, err := cfg.clean(s, scope, row)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"status": "error", "message": err.Error()})
			return
		}
		insert = append(insert, cleanRow)
	}

	trackedRows, err := s.query.SelectAllQuery(cfg.table, map[string]any{"id": id, "profile_no": currentProfile})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
		return
	}

	plan := tracking.PlanUpsert(insert, trackedRows, cfg.sameIdentity, cfg.mutableFields...)
	message := tracking.ChangeMessage(translator, "!", translator.Translate("tracked", false), plan, func(row map[string]any) string {
		return rowText(translator, row)
	})
	if len(plan.Inserts)+len(plan.Updates) > 0 {
		if err := replaceTrackedRowsTx(s, cfg.table, map[string]any{"id": id, "profile_no": currentProfile}, plan.Updates, plan.Inserts); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"status": "error", "message": "Exception raised during execution"})
			return
		}
	}

	trimmed := strings.TrimSpace(message)
	refreshAlertState(s)
	sendTrackingMessage(s, human, trimmed, language)
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "message": trimmed})
}

func raidTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "raid",
		responseKey: "raid",
		describe: func(s *Server, tr *i18n.Translator, row map[string]any) string {
			return tracking.RaidRowText(s.cfg, tr, s.getData(), row, s.scanner)
		},
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanRaidRow(s.cfg, s.getData(), scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return intFromAny(existing["team"]) == intFromAny(candidate["team"])
		},
		mutableFields: []string{"distance", "template", "clean"},
	}
}

func eggTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "egg",
		responseKey: "egg",
		describe: func(s *Server, tr *i18n.Translator, row map[string]any) string {
			return tracking.EggRowText(s.cfg, tr, s.getData(), row, s.scanner)
		},
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanEggRow(s.cfg, s.getData(), scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return intFromAny(existing["team"]) == intFromAny(candidate["team"])
		},
		mutableFields: []string{"distance", "template", "clean"},
	}
}

func questTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "quest",
		responseKey: "quest",
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanQuestRow(s.cfg, scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return intFromAny(existing["reward_type"]) == intFromAny(candidate["reward_type"]) &&
				intFromAny(existing["reward"]) == intFromAny(candidate["reward"])
		},
		mutableFields: []string{"distance", "template", "clean"},
	}
}

func invasionTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "invasion",
		responseKey: "invasion",
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanInvasionRow(s.cfg, scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return getString(existing["grunt_type"]) == getString(candidate["grunt_type"])
		},
		mutableFields: []string{"distance", "template", "clean"},
	}
}

func lureTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "lures",
		responseKey: "lure",
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanLureRow(s.cfg, scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return intFromAny(existing["lure_id"]) == intFromAny(candidate["lure_id"])
		},
		mutableFields: []string{"distance", "template", "clean"},
	}
}

func nestTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "nests",
		responseKey: "nest",
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanNestRow(s.cfg, scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return intFromAny(existing["pokemon_id"]) == intFromAny(candidate["pokemon_id"])
		},
		mutableFields: []string{"distance", "template", "clean"},
	}
}

func gymTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "gym",
		responseKey: "gym",
		describe: func(s *Server, tr *i18n.Translator, row map[string]any) string {
			return tracking.GymRowText(s.cfg, tr, s.getData(), row, s.scanner)
		},
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanGymRow(s.cfg, scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return intFromAny(existing["team"]) == intFromAny(candidate["team"])
		},
		mutableFields: []string{"distance", "template", "clean", "slot_changes", "battle_changes"},
	}
}

func maxbattleTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "maxbattle",
		responseKey: "maxbattle",
		describe: func(s *Server, tr *i18n.Translator, row map[string]any) string {
			return tracking.MaxbattleRowText(s.cfg, tr, s.getData(), row)
		},
		clean: func(s *Server, scope tracking.RuleScope, row map[string]any) (map[string]any, error) {
			return tracking.CleanMaxbattleRow(s.cfg, scope, row)
		},
		sameIdentity: func(candidate, existing map[string]any) bool {
			return intFromAny(existing["pokemon_id"]) == intFromAny(candidate["pokemon_id"]) &&
				intFromAny(existing["gmax"]) == intFromAny(candidate["gmax"]) &&
				intFromAny(existing["level"]) == intFromAny(candidate["level"]) &&
				intFromAny(existing["form"]) == intFromAny(candidate["form"]) &&
				intFromAny(existing["move"]) == intFromAny(candidate["move"]) &&
				intFromAny(existing["evolution"]) == intFromAny(candidate["evolution"]) &&
				strings.TrimSpace(getString(existing["station_id"])) == strings.TrimSpace(getString(candidate["station_id"]))
		},
		mutableFields: []string{"distance", "template", "clean"},
	}
}

func monsterTrackingRouteConfig(s *Server) trackingRouteConfig {
	return trackingRouteConfig{
		table:       "monsters",
		responseKey: "pokemon",
		describe: func(s *Server, tr *i18n.Translator, row map[string]any) string {
			return tracking.MonsterRowText(s.cfg, tr, s.getData(), row)
		},
	}
}
