package server

import (
	"context"
	"net/http"
	"time"
)

func registerHealthRoutes(s *Server, mux *http.ServeMux) {
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if rejectNotAllowedByIP(s.cfg, r, w) {
			return
		}

		result := map[string]any{
			"status": "ok",
		}
		status := http.StatusOK

		// Database connectivity check.
		if s.db != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			if err := s.db.PingContext(ctx); err != nil {
				result["database"] = "unhealthy"
				result["database_error"] = err.Error()
				result["status"] = "degraded"
				status = http.StatusServiceUnavailable
			} else {
				result["database"] = "healthy"
			}
		}

		// Queue depths.
		if s.webhookQueue != nil {
			result["webhook_queue_depth"] = s.webhookQueue.Len()
		}
		if s.discordQueue != nil {
			result["discord_queue_depth"] = s.discordQueue.Len()
		}
		if s.telegramQueue != nil {
			result["telegram_queue_depth"] = s.telegramQueue.Len()
		}

		writeJSON(w, status, result)
	})
}
