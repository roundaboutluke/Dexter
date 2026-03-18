package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"dexter/internal/alertstate"
	"dexter/internal/config"
	"dexter/internal/webhook"
)

func TestAlertStateRefreshRoutesUseFullSnapshotReload(t *testing.T) {
	cfg := config.New(map[string]any{
		"server": map[string]any{
			"apiSecret": "secret",
		},
	})
	processor := &webhook.Processor{}
	processor.SetAlertStateLoader(func() (*alertstate.Snapshot, error) {
		return &alertstate.Snapshot{
			Tables: map[string][]map[string]any{
				"monsters": {{"id": "user-2"}},
			},
			Humans:       map[string]map[string]any{},
			Profiles:     map[string]map[string]any{},
			HasSchedules: map[string]bool{},
		}, nil
	})
	processor.RefreshAlertCacheSync()

	processor.SetAlertStateLoader(func() (*alertstate.Snapshot, error) {
		return &alertstate.Snapshot{
			Tables: map[string][]map[string]any{
				"monsters": {{"id": "user-3"}},
			},
			Humans:       map[string]map[string]any{},
			Profiles:     map[string]map[string]any{},
			HasSchedules: map[string]bool{},
		}, nil
	})

	s := &Server{cfg: cfg, processor: processor}
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	for _, path := range []string{"/api/alert-state/refresh", "/api/tracking/pokemon/refresh"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("x-poracle-secret", "secret")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s status=%d, want 200", path, w.Code)
		}
	}

	snapshot := processor.CurrentAlertStateForTest()
	if snapshot == nil {
		t.Fatalf("snapshot missing after manual refresh")
	}
	rows := snapshot.Rows("monsters")
	if len(rows) != 1 || rows[0]["id"] != "user-3" {
		t.Fatalf("manual refresh did not load latest snapshot: %#v", rows)
	}
}
