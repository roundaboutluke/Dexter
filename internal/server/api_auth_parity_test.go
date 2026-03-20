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

func TestAPIWhitelistRejectsWithPoracleJSShape(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "default.json"), []byte(`{"server":{"ipWhitelist":["1.2.3.4"],"apiSecret":"secret"}}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	s, err := New(cfg, webhook.NewQueue(), nil, nil, nil, nil, &geofence.Store{}, root, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config/templates", nil)
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
	if payload["webserver"] != "unhappy" {
		t.Fatalf("webserver=%v, want unhappy", payload["webserver"])
	}
	if payload["reason"] != "ip 203.0.113.10 not in whitelist" {
		t.Fatalf("reason=%v, want whitelist message", payload["reason"])
	}
}

func TestAPISecretRejectsWithPoracleJSShape(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "default.json"), []byte(`{"server":{"apiSecret":"secret"}}`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	s, err := New(cfg, webhook.NewQueue(), nil, nil, nil, nil, &geofence.Store{}, root, nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/masterdata/monsters", nil)
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
	if payload["status"] != "authError" {
		t.Fatalf("status=%v, want authError", payload["status"])
	}
	if payload["reason"] != "incorrect or missing api secret" {
		t.Fatalf("reason=%v, want api secret message", payload["reason"])
	}
}
