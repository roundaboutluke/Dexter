package webhook

import "testing"

func TestNormalizeRaidOrEggHookPromotesRaidFieldsAndConvertsEgg(t *testing.T) {
	raid := &Hook{
		Type: "raid",
		Message: map[string]any{
			"id":         "gym-1",
			"pokemonId":  25,
			"raid_level": 5,
		},
	}

	normalizeRaidOrEggHook(raid)

	if raid.Type != "raid" {
		t.Fatalf("hook.Type=%q, want raid", raid.Type)
	}
	if got := getString(raid.Message["gym_id"]); got != "gym-1" {
		t.Fatalf("gym_id=%q, want gym-1", got)
	}
	if got := getInt(raid.Message["pokemon_id"]); got != 25 {
		t.Fatalf("pokemon_id=%d, want 25", got)
	}
	if got := getInt(raid.Message["level"]); got != 5 {
		t.Fatalf("level=%d, want 5", got)
	}

	egg := &Hook{
		Type: "raid",
		Message: map[string]any{
			"id":         "gym-2",
			"raid_level": 3,
		},
	}
	normalizeRaidOrEggHook(egg)
	if egg.Type != "egg" {
		t.Fatalf("hook.Type=%q, want egg", egg.Type)
	}
	if got := getString(egg.Message["gym_id"]); got != "gym-2" {
		t.Fatalf("gym_id=%q, want gym-2", got)
	}
}

func TestNormalizeMaxBattleHookPromotesBattleFields(t *testing.T) {
	hook := &Hook{
		Type: "max_battle",
		Message: map[string]any{
			"id":                    "station-1",
			"name":                  "Power Spot",
			"battle_pokemon_id":     150,
			"battle_pokemon_form":   61,
			"battle_level":          6,
			"battle_pokemon_move_1": 10,
			"battle_pokemon_move_2": 20,
		},
	}

	normalizeMaxBattleHook(hook)

	if got := getString(hook.Message["stationId"]); got != "station-1" {
		t.Fatalf("stationId=%q, want station-1", got)
	}
	if got := getString(hook.Message["stationName"]); got != "Power Spot" {
		t.Fatalf("stationName=%q, want Power Spot", got)
	}
	if got := getInt(hook.Message["pokemon_id"]); got != 150 {
		t.Fatalf("pokemon_id=%d, want 150", got)
	}
	if got := getInt(hook.Message["form"]); got != 61 {
		t.Fatalf("form=%d, want 61", got)
	}
	if got := getInt(hook.Message["level"]); got != 6 {
		t.Fatalf("level=%d, want 6", got)
	}
	if got := getInt(hook.Message["move_1"]); got != 10 {
		t.Fatalf("move_1=%d, want 10", got)
	}
	if got := getInt(hook.Message["move_2"]); got != 20 {
		t.Fatalf("move_2=%d, want 20", got)
	}
	if got := getInt(hook.Message["gmax"]); got != 0 {
		t.Fatalf("gmax=%d, want 0 for level 6", got)
	}
	if got := getString(hook.Message["color"]); got != "D000C0" {
		t.Fatalf("color=%q, want D000C0", got)
	}
	if got := getInt(hook.Message["evolution"]); got != 0 {
		t.Fatalf("evolution=%d, want 0", got)
	}
}
