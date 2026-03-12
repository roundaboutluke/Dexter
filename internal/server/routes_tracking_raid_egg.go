package server

import (
	"net/http"
	"strings"

	"poraclego/internal/i18n"
	"poraclego/internal/tracking"
)

func registerTrackingRaidRoutes(s *Server, mux *http.ServeMux) {
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
}

func registerTrackingEggRoutes(s *Server, mux *http.ServeMux) {
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
}

func handleTrackingRaidGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, raidTrackingRouteConfig(s))
}

func handleTrackingRaidDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "raid")
}

func handleTrackingRaidDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "raid")
}

func handleTrackingRaidUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, raidTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.RaidRowText(s.cfg, translator, s.data, row, s.scanner)
	})
}

func handleTrackingEggGet(w http.ResponseWriter, s *Server, id string) {
	handleTrackingGetGeneric(w, s, id, eggTrackingRouteConfig(s))
}

func handleTrackingEggDelete(w http.ResponseWriter, s *Server, id string, uid string) {
	handleTrackingDeleteGeneric(w, s, id, uid, "egg")
}

func handleTrackingEggDeleteBatch(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingDeleteBatchGeneric(w, s, id, r, "egg")
}

func handleTrackingEggUpsert(w http.ResponseWriter, s *Server, id string, r *http.Request) {
	handleTrackingUpsertGeneric(w, s, id, r, eggTrackingRouteConfig(s), func(translator *i18n.Translator, row map[string]any) string {
		return tracking.EggRowText(s.cfg, translator, s.data, row, s.scanner)
	})
}
