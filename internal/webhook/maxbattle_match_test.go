package webhook

import "testing"

func TestMatchMaxBattle_PokemonAndWildcards(t *testing.T) {
	hook := &Hook{
		Type: "max_battle",
		Message: map[string]any{
			"battle_pokemon_id":     25,
			"battle_pokemon_form":   0,
			"battle_level":          2,
			"battle_pokemon_move_1": 221,
			"battle_pokemon_move_2": 222,
		},
	}

	row := map[string]any{
		"pokemon_id": 25,
		"gmax":       0,
		"form":       0,
		"evolution":  9000,
		"move":       9000,
		"level":      0,
	}
	if !matchMaxBattle(hook, row) {
		t.Fatalf("expected exact pokemon match to pass")
	}

	rowLevel := map[string]any{
		"pokemon_id": 9000,
		"level":      2,
		"gmax":       0,
		"form":       0,
		"evolution":  9000,
		"move":       9000,
	}
	if !matchMaxBattle(hook, rowLevel) {
		t.Fatalf("expected pokemon_id=9000 level match to pass")
	}

	rowLevelMismatch := map[string]any{
		"pokemon_id": 9000,
		"level":      3,
		"gmax":       0,
		"form":       0,
		"evolution":  9000,
		"move":       9000,
	}
	if matchMaxBattle(hook, rowLevelMismatch) {
		t.Fatalf("expected pokemon_id=9000 level mismatch to fail")
	}

	rowMove := map[string]any{
		"pokemon_id": 25,
		"level":      0,
		"gmax":       0,
		"form":       0,
		"evolution":  9000,
		"move":       222,
	}
	if !matchMaxBattle(hook, rowMove) {
		t.Fatalf("expected move match against move_2 to pass")
	}
}

func TestMatchMaxBattle_GmaxAndFormFilters(t *testing.T) {
	hook := &Hook{
		Type: "max_battle",
		Message: map[string]any{
			"battle_pokemon_id":   1,
			"battle_pokemon_form": 3,
			"battle_level":        7,
		},
	}

	row := map[string]any{
		"pokemon_id": 1,
		"level":      0,
		"gmax":       1,
		"form":       3,
		"evolution":  9000,
		"move":       9000,
	}
	if !matchMaxBattle(hook, row) {
		t.Fatalf("expected gmax + form match to pass")
	}

	rowFormMismatch := map[string]any{
		"pokemon_id": 1,
		"level":      0,
		"gmax":       1,
		"form":       4,
		"evolution":  9000,
		"move":       9000,
	}
	if matchMaxBattle(hook, rowFormMismatch) {
		t.Fatalf("expected form mismatch to fail")
	}

	rowGmaxMismatch := map[string]any{
		"pokemon_id": 1,
		"level":      0,
		"gmax":       1,
		"form":       3,
		"evolution":  9000,
		"move":       9000,
	}
	hookLow := &Hook{
		Type: "max_battle",
		Message: map[string]any{
			"battle_pokemon_id":   1,
			"battle_pokemon_form": 3,
			"battle_level":        5,
		},
	}
	if matchMaxBattle(hookLow, rowGmaxMismatch) {
		t.Fatalf("expected gmax mismatch to fail when level <= 6")
	}
}
