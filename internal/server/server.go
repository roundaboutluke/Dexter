package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"poraclego/internal/bot"
	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/db"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/geofence"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
	"poraclego/internal/scanner"
	"poraclego/internal/webhook"
)

// Server wraps the HTTP server.
type Server struct {
	srv *http.Server

	cfg       *config.Config
	query     *db.Query
	fences    *geofence.Store
	root      string
	data      *data.GameData
	i18n      *i18n.Factory
	dts       []dts.Template
	scanner   *scanner.Client
	processor *webhook.Processor

	discordQueue  *dispatch.Queue
	telegramQueue *dispatch.Queue
	botManager    *bot.Manager
}

// New constructs a server with routes and config bindings.
func New(cfg *config.Config, queue *webhook.Queue, processor *webhook.Processor, discordQueue *dispatch.Queue, telegramQueue *dispatch.Queue, query *db.Query, fences *geofence.Store, root string, gameData *data.GameData, i18nFactory *i18n.Factory, templates []dts.Template, scannerClient *scanner.Client) (*Server, error) {
	host, _ := cfg.GetString("server.host")
	port, ok := cfg.GetInt("server.port")
	if !ok {
		port = 3030
	}

	addr := fmt.Sprintf("%s:%d", host, port)

	mux := http.NewServeMux()
	s := &Server{
		cfg:           cfg,
		query:         query,
		fences:        fences,
		root:          root,
		data:          gameData,
		i18n:          i18nFactory,
		dts:           templates,
		scanner:       scannerClient,
		processor:     processor,
		discordQueue:  discordQueue,
		telegramQueue: telegramQueue,
	}
	s.registerRoutes(mux)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if rejectNotAllowedByIP(cfg, r, w) {
			return
		}

		var payload any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			if logger := logging.Get().General; logger != nil {
				logger.Errorf("API: %s %s %s invalid payload: %v", clientIP(r), r.Method, r.URL.Path, err)
			}
			writeJSON(w, http.StatusBadRequest, map[string]any{
				"webserver": "unhappy",
				"reason":    "invalid payload",
			})
			return
		}

		switch data := payload.(type) {
		case []any:
			queue.Push(data...)
			if logger := logging.Get().General; logger != nil {
				logger.Infof("API: %s %s %s queued %d webhook payloads", clientIP(r), r.Method, r.URL.Path, len(data))
			}
		default:
			queue.Push(data)
			if logger := logging.Get().General; logger != nil {
				logger.Infof("API: %s %s %s queued 1 webhook payload", clientIP(r), r.Method, r.URL.Path)
			}
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"webserver": "happy",
		})
	})

	srv := &http.Server{
		Addr:              addr,
		Handler:           withRequestLogging(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	s.srv = srv
	return s, nil
}

// Start runs the HTTP server in a goroutine.
func (s *Server) Start() {
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger := logging.Get().General
			if logger != nil {
				logger.Errorf("http server error: %v", err)
			} else {
				fmt.Fprintf(os.Stderr, "http server error: %v\n", err)
			}
		}
	}()
}

// Shutdown stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Shutdown(ctx)
}

// UpdateTemplates replaces the DTS template list.
func (s *Server) UpdateTemplates(templates []dts.Template) {
	if s == nil {
		return
	}
	s.dts = templates
}

// UpdateData replaces the game data used by API handlers.
func (s *Server) UpdateData(game *data.GameData) {
	if s == nil || game == nil {
		return
	}
	s.data = game
}

// SetBotManager supplies the bot manager for API integrations that need Discord access.
func (s *Server) SetBotManager(manager *bot.Manager) {
	if s == nil {
		return
	}
	s.botManager = manager
}

func ipAllowed(cfg *config.Config, r *http.Request) bool {
	ip := clientIP(r)
	whitelist, _ := cfg.GetStringSlice("server.ipWhitelist")
	blacklist, _ := cfg.GetStringSlice("server.ipBlacklist")
	if len(whitelist) > 0 && !containsString(whitelist, ip) {
		return false
	}
	if len(blacklist) > 0 && containsString(blacklist, ip) {
		return false
	}
	return true
}

func clientIP(r *http.Request) string {
	host := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		parts := strings.Split(forwarded, ",")
		if len(parts) > 0 {
			host = strings.TrimSpace(parts[0])
		}
	}
	ip, _, err := net.SplitHostPort(host)
	if err != nil {
		return host
	}
	return ip
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger := logging.Get().General
		if logger != nil {
			logger.Warnf("failed to write response: %v", err)
		} else {
			fmt.Fprintf(os.Stderr, "failed to write response: %v\n", err)
		}
	}
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func withRequestLogging(next http.Handler) http.Handler {
	if next == nil {
		return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logRequest(r)
		rec := &loggingResponseWriter{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if recovered := recover(); recovered != nil {
				logger := logging.Get().General
				if logger != nil {
					logger.Errorf("API: %s %s %s panic: %v", clientIP(r), r.Method, r.URL.Path, recovered)
				}
				http.Error(rec, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if rec.status >= http.StatusInternalServerError {
				logger := logging.Get().General
				if logger != nil {
					logger.Errorf("API: %s %s %s returned %d", clientIP(r), r.Method, r.URL.Path, rec.status)
				}
			}
		}()
		next.ServeHTTP(rec, r)
	})
}

func logRequest(r *http.Request) {
	logger := logging.Get().General
	if logger == nil || r == nil {
		return
	}
	logger.Infof("API: %s %s %s", clientIP(r), r.Method, r.URL.Path)
}
