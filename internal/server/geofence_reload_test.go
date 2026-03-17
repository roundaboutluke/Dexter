package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/geofence"
	"poraclego/internal/webhook"
)

func writeTestConfig(t *testing.T, root string, cfg map[string]any) *config.Config {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "default.json"), raw, 0o644); err != nil {
		t.Fatalf("write default.json: %v", err)
	}
	loaded, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return loaded
}

func TestGeofenceReloadReplacesStoreInPlace(t *testing.T) {
	root := t.TempDir()
	fencePath := filepath.Join(root, "geofence.json")
	rawFence := []byte(`[{"name":"AreaA","path":[[0,0],[0,1],[1,1],[1,0]]}]`)
	if err := os.WriteFile(fencePath, rawFence, 0o644); err != nil {
		t.Fatalf("write geofence: %v", err)
	}

	cfg := writeTestConfig(t, root, map[string]any{
		"server": map[string]any{
			"apiSecret": "secret",
		},
		"geofence": map[string]any{
			"path": []any{"geofence.json"},
		},
	})

	store := &geofence.Store{Fences: []geofence.Fence{}}
	queue := webhook.NewQueue()

	s, err := New(cfg, queue, nil, nil, nil, nil, store, root, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	if s.getFences() != store {
		t.Fatalf("expected server to hold same fences pointer")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/geofence/reload", nil)
	req.Header.Set("x-poracle-secret", "secret")
	rr := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200", rr.Code)
	}
	var payload map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("status=%v, want ok", payload["status"])
	}
	reloaded := s.getFences()
	if len(reloaded.Fences) != 1 || reloaded.Fences[0].Name != "AreaA" {
		t.Fatalf("fences=%v, want AreaA loaded", reloaded.Fences)
	}
}
