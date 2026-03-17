package server

import "net/http"

func registerMasterDataRoutes(s *Server, mux *http.ServeMux) {
	mux.HandleFunc("/api/masterdata/grunts", func(w http.ResponseWriter, r *http.Request) {
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

		writeJSON(w, http.StatusOK, s.getData().Grunts)
	})

	mux.HandleFunc("/api/masterdata/monsters", func(w http.ResponseWriter, r *http.Request) {
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

		writeJSON(w, http.StatusOK, s.getData().Monsters)
	})
}
