package stats

import (
	"testing"
	"time"

	"dexter/internal/config"
)

func TestUpdate_InvalidID(t *testing.T) {
	tr := NewTracker()
	tr.Update(0, true, false)
	tr.Update(-1, true, false)
	cfg := config.New(map[string]any{
		"stats": map[string]any{
			"minSampleSize":    float64(0),
			"pokemonCountToKeep": float64(24),
		},
	})
	report := tr.Calculate(cfg)
	total := 0
	for _, ids := range report.Rarity {
		total += len(ids)
	}
	if total != 0 {
		t.Errorf("expected zero pokemon in report for invalid IDs, got %d", total)
	}
}

func TestUpdateAndCalculate(t *testing.T) {
	tr := NewTracker()
	// Add 100 scans for pokemon 25 (pikachu)
	for i := 0; i < 100; i++ {
		tr.Update(25, true, false)
	}
	// Add 1 scan for pokemon 150 (mewtwo)
	tr.Update(150, true, false)

	cfg := config.New(map[string]any{
		"stats": map[string]any{
			"minSampleSize":       float64(0),
			"pokemonCountToKeep":  float64(24),
			"maxPokemonId":        float64(151),
			"rarityGroup2Uncommon": float64(1.0),
			"rarityGroup3Rare":    float64(0.5),
			"rarityGroup4VeryRare": float64(0.03),
			"rarityGroup5UltraRare": float64(0.01),
		},
	})
	report := tr.Calculate(cfg)
	// Pokemon 25 should be in group 1 (common, >1% of total)
	found25 := false
	for _, id := range report.Rarity[1] {
		if id == 25 {
			found25 = true
		}
	}
	if !found25 {
		t.Error("expected pokemon 25 in rarity group 1 (common)")
	}
}

func TestCalculate_ShinyStats(t *testing.T) {
	tr := NewTracker()
	for i := 0; i < 200; i++ {
		shiny := i == 0 // 1 shiny out of 200
		tr.Update(25, true, shiny)
	}

	cfg := config.New(map[string]any{
		"stats": map[string]any{
			"minSampleSize":      float64(0),
			"pokemonCountToKeep": float64(24),
		},
	})
	report := tr.Calculate(cfg)
	stat, ok := report.Shiny[25]
	if !ok {
		t.Fatal("expected shiny stat for pokemon 25")
	}
	if stat.Seen != 1 {
		t.Errorf("Seen = %d, want 1", stat.Seen)
	}
	if stat.Total != 200 {
		t.Errorf("Total = %d, want 200", stat.Total)
	}
	expectedRatio := 200.0
	if stat.Ratio != expectedRatio {
		t.Errorf("Ratio = %f, want %f", stat.Ratio, expectedRatio)
	}
}

func TestLatestReport_Empty(t *testing.T) {
	tr := NewTracker()
	_, ok := tr.LatestReport()
	if ok {
		t.Error("expected ok=false before any StoreReport")
	}
}

func TestStoreAndLatestReport(t *testing.T) {
	tr := NewTracker()
	report := Report{
		Rarity: map[int][]int{1: {25}},
		Shiny:  map[int]ShinyStat{},
		Time:   time.Now(),
	}
	tr.StoreReport(report)
	got, ok := tr.LatestReport()
	if !ok {
		t.Fatal("expected ok=true after StoreReport")
	}
	if len(got.Rarity[1]) != 1 || got.Rarity[1][0] != 25 {
		t.Errorf("unexpected rarity data: %v", got.Rarity)
	}
}
