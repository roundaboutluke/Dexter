package webhook

import "testing"

func TestQuestRewardsFromHookSupportsMapPayload(t *testing.T) {
	hook := &Hook{
		Type: "quest",
		Message: map[string]any{
			"rewards": map[string]any{
				"type": 2,
				"info": map[string]any{
					"item_id": 2,
					"amount":  5,
				},
			},
		},
	}

	rewardData := questRewardData(nil, hook)
	items, _ := rewardData["items"].([]map[string]any)
	if len(items) != 1 {
		t.Fatalf("items=%d, want 1", len(items))
	}
	if got := getInt(items[0]["id"]); got != 2 {
		t.Fatalf("item id=%d, want 2", got)
	}
	if got := getInt(items[0]["amount"]); got != 5 {
		t.Fatalf("item amount=%d, want 5", got)
	}
}

func TestQuestRewardDataIncludesCandyAndEnergyWithPokemonIDZero(t *testing.T) {
	hook := &Hook{
		Type: "quest",
		Message: map[string]any{
			"rewards": []any{
				map[string]any{"type": 4, "info": map[string]any{"pokemon_id": 0, "amount": 3}},
				map[string]any{"type": 12, "info": map[string]any{"pokemon_id": 0, "amount": 10}},
			},
		},
	}

	rewardData := questRewardData(nil, hook)
	candy, _ := rewardData["candy"].([]map[string]any)
	if len(candy) != 1 {
		t.Fatalf("candy=%d, want 1", len(candy))
	}
	if got := getInt(candy[0]["pokemonId"]); got != 0 {
		t.Fatalf("candy pokemonId=%d, want 0", got)
	}

	energy, _ := rewardData["energyMonsters"].([]map[string]any)
	if len(energy) != 1 {
		t.Fatalf("energyMonsters=%d, want 1", len(energy))
	}
	if got := getInt(energy[0]["pokemonId"]); got != 0 {
		t.Fatalf("energy pokemonId=%d, want 0", got)
	}
}
