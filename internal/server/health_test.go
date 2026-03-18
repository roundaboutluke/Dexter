package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"dexter/internal/config"
	"dexter/internal/geofence"
	"dexter/internal/webhook"
)

func TestHealthReturnsHappyWithQueryAndPort(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "default.json"), []byte(`{"server":{"port":3031}}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	s, err := New(cfg, webhook.NewQueue(), nil, nil, nil, nil, &geofence.Store{}, root, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health?x=1&x=2", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rr := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["webserver"] != "happy" {
		t.Fatalf("webserver=%v, want happy", payload["webserver"])
	}
	if payload["port"] != float64(3031) {
		t.Fatalf("port=%v, want 3031", payload["port"])
	}
	query, ok := payload["query"].(map[string]any)
	if !ok {
		t.Fatalf("query=%T, want object", payload["query"])
	}
	x, ok := query["x"].([]any)
	if !ok || len(x) != 2 {
		t.Fatalf("query.x=%v, want 2 values", query["x"])
	}
}

func TestHealthBlockedByWhitelist(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "default.json"), []byte(`{"server":{"ipWhitelist":["1.2.3.4"],"port":3030}}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	s, err := New(cfg, webhook.NewQueue(), nil, nil, nil, nil, &geofence.Store{}, root, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.RemoteAddr = "203.0.113.10:1234"
	rr := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
}
