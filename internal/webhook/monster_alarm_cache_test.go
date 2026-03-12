package webhook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"poraclego/internal/config"
)

type fakeMonsterQuery struct {
	rows  []map[string]any
	calls int
}

func (f *fakeMonsterQuery) SelectAllQuery(table string, conditions map[string]any) ([]map[string]any, error) {
	f.calls++
	return f.rows, nil
}

func loadFastMonstersConfig(t *testing.T, enabled bool) *config.Config {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	payload := map[string]any{
		"tuning": map[string]any{
			"fastMonsters": enabled,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "default.json"), raw, 0o644); err != nil {
		t.Fatalf("write default.json: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func TestMonsterAlarmCacheRefreshSkippedWhenFastMonstersDisabled(t *testing.T) {
	cfg := loadFastMonstersConfig(t, false)
	cache := NewMonsterAlarmCache()
	query := &fakeMonsterQuery{rows: []map[string]any{{"id": "u1"}}}

	if err := cache.Refresh(cfg, query); err != nil {
		t.Fatalf("refresh error: %v", err)
	}
	if query.calls != 0 {
		t.Fatalf("SelectAllQuery calls=%d, want 0", query.calls)
	}
	if got := cache.Rows(); len(got) != 0 {
		t.Fatalf("rows=%d, want 0", len(got))
	}
}

func TestMonsterAlarmCacheRefreshLoadsRowsWhenEnabled(t *testing.T) {
	cfg := loadFastMonstersConfig(t, true)
	cache := NewMonsterAlarmCache()
	query := &fakeMonsterQuery{rows: []map[string]any{{"id": "u1"}, {"id": "u2"}}}

	if err := cache.Refresh(cfg, query); err != nil {
		t.Fatalf("refresh error: %v", err)
	}
	if query.calls != 1 {
		t.Fatalf("SelectAllQuery calls=%d, want 1", query.calls)
	}
	if got := cache.Rows(); len(got) != 2 {
		t.Fatalf("rows=%d, want 2", len(got))
	}
}
