package webhook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"poraclego/internal/config"
)

func writeJSON(t *testing.T, path string, data any) {
	t.Helper()
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func testConfig(t *testing.T, pvpEvolution bool, greatMinCP int) *config.Config {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	writeJSON(t, filepath.Join(root, "config", "default.json"), map[string]any{
		"pvp": map[string]any{
			"pvpEvolutionDirectTracking": pvpEvolution,
			"pvpQueryMaxRank":            100,
			"pvpFilterMaxRank":           4096,
			"pvpFilterGreatMinCP":        greatMinCP,
			"pvpFilterUltraMinCP":        0,
			"pvpFilterLittleMinCP":       0,
			"levelCaps":                  []int{50},
		},
	})
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func baseMonsterRow(pokemonID int, league int) map[string]any {
	return map[string]any{
		"pokemon_id":         pokemonID,
		"min_iv":             -1,
		"max_iv":             100,
		"min_cp":             0,
		"max_cp":             9000,
		"min_level":          0,
		"max_level":          55,
		"atk":                0,
		"def":                0,
		"sta":                0,
		"max_atk":            15,
		"max_def":            15,
		"max_sta":            15,
		"min_time":           0,
		"min_weight":         0,
		"max_weight":         999999999,
		"rarity":             -1,
		"max_rarity":         999,
		"size":               -1,
		"max_size":           999,
		"gender":             0,
		"form":               0,
		"pvp_ranking_league": league,
		"pvp_ranking_best":   1,
		"pvp_ranking_worst":  1,
		"pvp_ranking_min_cp": 0,
		"pvp_ranking_cap":    0,
	}
}

func TestMatchPokemonPvpEvolutionDirectTrackingEnabled(t *testing.T) {
	p := &Processor{cfg: testConfig(t, true, 1400)}
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id":         133,
			"form":               0,
			"cp":                 2000,
			"pokemon_level":      30,
			"gender":             0,
			"individual_attack":  0,
			"individual_defense": 0,
			"individual_stamina": 0,
			"pvp_rankings_great_league": []any{
				map[string]any{
					"pokemon":   134,
					"form":      0,
					"rank":      1,
					"cp":        1500,
					"cap":       50,
					"evolution": false,
				},
			},
		},
	}
	row := baseMonsterRow(134, 1500)
	if !matchPokemon(p, hook, row) {
		t.Fatalf("expected evolution direct-tracking match, got false")
	}
}

func TestMatchPokemonPvpEvolutionDirectTrackingDisabled(t *testing.T) {
	p := &Processor{cfg: testConfig(t, false, 1400)}
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id":         133,
			"form":               0,
			"cp":                 2000,
			"pokemon_level":      30,
			"gender":             0,
			"individual_attack":  0,
			"individual_defense": 0,
			"individual_stamina": 0,
			"pvp_rankings_great_league": []any{
				map[string]any{
					"pokemon":   134,
					"form":      0,
					"rank":      1,
					"cp":        1500,
					"cap":       50,
					"evolution": false,
				},
			},
		},
	}
	row := baseMonsterRow(134, 1500)
	if matchPokemon(p, hook, row) {
		t.Fatalf("expected no match when evolution direct-tracking is disabled")
	}
}

func TestMatchPokemonPvpEverythingDoesNotMatchEvolutionEntries(t *testing.T) {
	p := &Processor{cfg: testConfig(t, true, 1400)}
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id":         133,
			"form":               0,
			"cp":                 2000,
			"pokemon_level":      30,
			"gender":             0,
			"individual_attack":  0,
			"individual_defense": 0,
			"individual_stamina": 0,
			"pvp_rankings_great_league": []any{
				map[string]any{
					"pokemon":   134,
					"form":      0,
					"rank":      1,
					"cp":        1500,
					"cap":       50,
					"evolution": false,
				},
			},
		},
	}
	row := baseMonsterRow(0, 1500)
	if matchPokemon(p, hook, row) {
		t.Fatalf("expected everything tracker to not match solely via evolution entries")
	}
}

func TestMatchPokemonPvpEvolutionHonorsGlobalMinCP(t *testing.T) {
	p := &Processor{cfg: testConfig(t, true, 1400)}
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id":         133,
			"form":               0,
			"cp":                 2000,
			"pokemon_level":      30,
			"gender":             0,
			"individual_attack":  0,
			"individual_defense": 0,
			"individual_stamina": 0,
			"pvp_rankings_great_league": []any{
				map[string]any{
					"pokemon":   134,
					"form":      0,
					"rank":      1,
					"cp":        1200, // below pvpFilterGreatMinCP
					"cap":       50,
					"evolution": false,
				},
			},
		},
	}
	row := baseMonsterRow(134, 1500)
	if matchPokemon(p, hook, row) {
		t.Fatalf("expected no match when evolution entry is below global min CP")
	}
}
