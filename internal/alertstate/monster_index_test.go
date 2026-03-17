package alertstate

import (
	"testing"
)

func TestBuildMonsterIndex(t *testing.T) {
	rows := []map[string]any{
		{"id": "u1", "pokemon_id": 25, "pvp_ranking_league": 0},       // Pikachu specific
		{"id": "u1", "pokemon_id": 0, "pvp_ranking_league": 0},        // catch-all
		{"id": "u2", "pokemon_id": 150, "pvp_ranking_league": 0},      // Mewtwo specific
		{"id": "u3", "pokemon_id": 25, "pvp_ranking_league": 1500},    // PVP specific Great
		{"id": "u3", "pokemon_id": 0, "pvp_ranking_league": 1500},     // PVP everything Great
		{"id": "u4", "pokemon_id": 100, "pvp_ranking_league": 2500},   // PVP specific Ultra
		{"id": "u4", "pokemon_id": 0, "pvp_ranking_league": 500},      // PVP everything Little
	}

	idx := buildMonsterIndex(rows)
	if idx == nil {
		t.Fatal("expected non-nil index")
	}

	// Non-PVP: catch-all (id=0) should have 1 entry
	if got := len(idx.ByPokemonID[0]); got != 1 {
		t.Fatalf("ByPokemonID[0] = %d, want 1", got)
	}
	// Non-PVP: Pikachu should have 1 entry
	if got := len(idx.ByPokemonID[25]); got != 1 {
		t.Fatalf("ByPokemonID[25] = %d, want 1", got)
	}
	// Non-PVP: Mewtwo should have 1 entry
	if got := len(idx.ByPokemonID[150]); got != 1 {
		t.Fatalf("ByPokemonID[150] = %d, want 1", got)
	}
	// Non-PVP: untracked species should have 0
	if got := len(idx.ByPokemonID[999]); got != 0 {
		t.Fatalf("ByPokemonID[999] = %d, want 0", got)
	}

	// PVP specific: Great league should have 1 (Pikachu)
	if got := len(idx.PVPSpecific[1500]); got != 1 {
		t.Fatalf("PVPSpecific[1500] = %d, want 1", got)
	}
	// PVP everything: Great league should have 1
	if got := len(idx.PVPEverything[1500]); got != 1 {
		t.Fatalf("PVPEverything[1500] = %d, want 1", got)
	}
	// PVP specific: Ultra league should have 1
	if got := len(idx.PVPSpecific[2500]); got != 1 {
		t.Fatalf("PVPSpecific[2500] = %d, want 1", got)
	}
	// PVP everything: Little league should have 1
	if got := len(idx.PVPEverything[500]); got != 1 {
		t.Fatalf("PVPEverything[500] = %d, want 1", got)
	}
}

func TestBuildMonsterIndexEmpty(t *testing.T) {
	idx := buildMonsterIndex(nil)
	if idx != nil {
		t.Fatal("expected nil index for nil rows")
	}

	idx = buildMonsterIndex([]map[string]any{})
	if idx != nil {
		t.Fatal("expected nil index for empty rows")
	}
}

func TestBuildMonsterIndexTotalRows(t *testing.T) {
	rows := []map[string]any{
		{"id": "u1", "pokemon_id": 25, "pvp_ranking_league": 0},
		{"id": "u1", "pokemon_id": 0, "pvp_ranking_league": 0},
		{"id": "u2", "pokemon_id": 25, "pvp_ranking_league": 0},
		{"id": "u3", "pokemon_id": 25, "pvp_ranking_league": 1500},
	}

	idx := buildMonsterIndex(rows)

	// Total rows across all buckets should equal input count.
	total := 0
	for _, bucket := range idx.ByPokemonID {
		total += len(bucket)
	}
	for _, bucket := range idx.PVPSpecific {
		total += len(bucket)
	}
	for _, bucket := range idx.PVPEverything {
		total += len(bucket)
	}
	if total != len(rows) {
		t.Fatalf("total indexed rows = %d, want %d", total, len(rows))
	}
}
