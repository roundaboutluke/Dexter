package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dexter/internal/config"
	"dexter/internal/logging"
)

func TestRequestLoggingCapturesEntryAndServerErrors(t *testing.T) {
	root := t.TempDir()
	initServerLogging(t, root)

	handler := withRequestLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", rr.Code)
	}

	general := readServerLog(t, root, "general")
	if !strings.Contains(general, "API: 203.0.113.10 GET /boom") {
		t.Fatalf("general log missing request entry: %s", general)
	}
	if !strings.Contains(general, "API: 203.0.113.10 GET /boom returned 500") {
		t.Fatalf("general log missing 500 status line: %s", general)
	}

	errors := readServerLog(t, root, "errors")
	if !strings.Contains(errors, "API: 203.0.113.10 GET /boom returned 500") {
		t.Fatalf("errors log missing mirrored 500 line: %s", errors)
	}
}

func TestRejectNotAuthorizedLogsMissingSecret(t *testing.T) {
	root := t.TempDir()
	initServerLogging(t, root)

	cfg := config.New(map[string]any{
		"server": map[string]any{
			"apiSecret": "secret",
		},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rr := httptest.NewRecorder()

	if !rejectNotAuthorized(cfg, req, rr) {
		t.Fatalf("expected request to be rejected")
	}

	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "authError" {
		t.Fatalf("status=%v, want authError", payload["status"])
	}

	general := readServerLog(t, root, "general")
	if !strings.Contains(general, "API: 203.0.113.10 GET /api/test missing api secret") {
		t.Fatalf("general log missing auth warning: %s", general)
	}

	errors := readServerLog(t, root, "errors")
	if !strings.Contains(errors, "API: 203.0.113.10 GET /api/test missing api secret") {
		t.Fatalf("errors log missing mirrored auth warning: %s", errors)
	}
}

func initServerLogging(t *testing.T, root string) {
	t.Helper()
	cfg := config.New(map[string]any{
		"logger": map[string]any{
			"consoleLogLevel": "error",
			"logLevel":        "debug",
			"dailyLogLimit":   7,
			"webhookLogLimit": 12,
			"enableLogs": map[string]any{
				"webhooks": true,
				"discord":  true,
				"telegram": true,
			},
		},
	})
	if err := logging.Init(cfg, root); err != nil {
		t.Fatalf("init logging: %v", err)
	}
	t.Cleanup(func() {
		_ = logging.Close()
	})
}

func readServerLog(t *testing.T, root, prefix string) string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, "logs", prefix+"-*.log"))
	if err != nil {
		t.Fatalf("glob %s log: %v", prefix, err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected one %s log file, got %d", prefix, len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatalf("read %s log: %v", prefix, err)
	}
	return string(data)
}
