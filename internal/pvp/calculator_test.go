package pvp

import (
	"os"
	"path/filepath"
	"testing"

	"dexter/internal/config"
	"dexter/internal/data"
)

func TestRankingsBasic(t *testing.T) {
	root := findRoot(t)
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	gameData, err := data.Load(root)
	if err != nil {
		t.Fatalf("load data: %v", err)
	}
	calc := NewCalculator(cfg, gameData)
	results := calc.Rankings(1, 0, 0, 0, 0, false)
	entries := results[1500]
	if len(entries) == 0 {
		t.Fatalf("expected great league rankings")
	}
	found := false
	for _, entry := range entries {
		if entry.PokemonID == 1 {
			found = true
			if entry.Rank <= 0 {
				t.Fatalf("expected rank > 0")
			}
			if entry.CP <= 0 || entry.CP > 1500 {
				t.Fatalf("unexpected CP %d", entry.CP)
			}
		}
	}
	if !found {
		t.Fatalf("missing base pokemon entry")
	}
}

func findRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod")
		}
		dir = parent
	}
}
