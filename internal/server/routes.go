package server

import (
	"net/http"
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
	registerHumanRoutes(s, mux)
	registerGeofenceRoutes(s, mux)
	registerProfileRoutes(s, mux)
	registerAlertStateRoutes(s, mux)
	registerTrackingRaidRoutes(s, mux)
	registerTrackingEggRoutes(s, mux)
	registerTrackingPokemonRoutes(s, mux)
}
