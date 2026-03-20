package server

import (
	"net/http"

	"dexter/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	registerHealthRoutes(s, mux)
	registerMetricsRoute(s, mux)
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

func registerMetricsRoute(s *Server, mux *http.ServeMux) {
	m := metrics.Get()
	if m == nil {
		return
	}
	path := "/metrics"
	if p, ok := s.cfg.GetString("server.metrics.path"); ok && p != "" {
		path = p
	}
	handler := promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}
		handler.ServeHTTP(w, r)
	})
}
