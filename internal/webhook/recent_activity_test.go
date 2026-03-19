package webhook

import (
	"testing"
	"time"
)

func TestRecentActivityRecordAndRetrieve(t *testing.T) {
	ra := NewRecentActivity()
	ra.RecordRaid(25)
	ra.RecordRaid(150)
	ra.RecordRaid(25) // duplicate

	ids := ra.ActiveRaidBosses()
	if len(ids) != 2 {
		t.Fatalf("ActiveRaidBosses() returned %d IDs, want 2: %v", len(ids), ids)
	}
	if ids[0] != 25 || ids[1] != 150 {
		t.Fatalf("ActiveRaidBosses() = %v, want [25 150]", ids)
	}
}

func TestRecentActivityPrune(t *testing.T) {
	ra := NewRecentActivity()
	ra.mu.Lock()
	ra.raidBosses[25] = time.Now().Add(-7 * time.Hour) // expired
	ra.raidBosses[150] = time.Now()                     // fresh
	ra.mu.Unlock()

	ra.Prune()
	ids := ra.ActiveRaidBosses()
	if len(ids) != 1 || ids[0] != 150 {
		t.Fatalf("after Prune, ActiveRaidBosses() = %v, want [150]", ids)
	}
}

func TestRecordRecentActivityFromHook(t *testing.T) {
	p := &Processor{
		recentActivity: NewRecentActivity(),
	}

	// Raid hook
	p.recordRecentActivity(&Hook{
		Type:    "raid",
		Message: map[string]any{"pokemon_id": float64(150)},
	})
	ids := p.recentActivity.ActiveRaidBosses()
	if len(ids) != 1 || ids[0] != 150 {
		t.Fatalf("after raid hook, ActiveRaidBosses() = %v, want [150]", ids)
	}

	// Egg hook (pokemon_id=0, should not record)
	p.recordRecentActivity(&Hook{
		Type:    "egg",
		Message: map[string]any{"pokemon_id": float64(0)},
	})
	ids = p.recentActivity.ActiveRaidBosses()
	if len(ids) != 1 {
		t.Fatalf("egg hook should not add to raid bosses, got %v", ids)
	}

	// Max battle hook
	p.recordRecentActivity(&Hook{
		Type:    "max_battle",
		Message: map[string]any{"battle_pokemon_id": float64(812)},
	})
	maxIDs := p.recentActivity.ActiveMaxBattleBosses()
	if len(maxIDs) != 1 || maxIDs[0] != 812 {
		t.Fatalf("after max_battle hook, ActiveMaxBattleBosses() = %v, want [812]", maxIDs)
	}

	// Quest item hook via structured quest_rewards array (primary scanner format)
	p.recordRecentActivity(&Hook{
		Type: "quest",
		Message: map[string]any{
			"quest_rewards": []any{
				map[string]any{"type": float64(2), "info": map[string]any{"item_id": float64(1), "amount": float64(3)}},
			},
		},
	})
	questIDs := p.recentActivity.ActiveQuestItems()
	if len(questIDs) != 1 || questIDs[0] != 1 {
		t.Fatalf("after quest item hook (structured), ActiveQuestItems() = %v, want [1]", questIDs)
	}

	// Quest mega energy hook via structured quest_rewards
	p.recordRecentActivity(&Hook{
		Type: "quest",
		Message: map[string]any{
			"quest_rewards": []any{
				map[string]any{"type": float64(12), "info": map[string]any{"pokemon_id": float64(6), "amount": float64(20)}},
			},
		},
	})
	megaIDs := p.recentActivity.ActiveQuestMegaEnergy()
	if len(megaIDs) != 1 || megaIDs[0] != 6 {
		t.Fatalf("after quest mega energy hook (structured), ActiveQuestMegaEnergy() = %v, want [6]", megaIDs)
	}

	// Quest pokemon encounter hook via structured quest_rewards
	p.recordRecentActivity(&Hook{
		Type: "quest",
		Message: map[string]any{
			"quest_rewards": []any{
				map[string]any{"type": float64(7), "info": map[string]any{"pokemon_id": float64(25)}},
			},
		},
	})
	pokemonIDs := p.recentActivity.ActiveQuestPokemon()
	if len(pokemonIDs) != 1 || pokemonIDs[0] != 25 {
		t.Fatalf("after quest pokemon hook (structured), ActiveQuestPokemon() = %v, want [25]", pokemonIDs)
	}

	// Quest candy hook via structured quest_rewards
	p.recordRecentActivity(&Hook{
		Type: "quest",
		Message: map[string]any{
			"quest_rewards": []any{
				map[string]any{"type": float64(4), "info": map[string]any{"pokemon_id": float64(143), "amount": float64(3)}},
			},
		},
	})
	candyIDs := p.recentActivity.ActiveQuestCandy()
	if len(candyIDs) != 1 || candyIDs[0] != 143 {
		t.Fatalf("after quest candy hook (structured), ActiveQuestCandy() = %v, want [143]", candyIDs)
	}

	// Quest item via flat fallback fields (older scanner format)
	p.recordRecentActivity(&Hook{
		Type:    "quest",
		Message: map[string]any{"reward_type": float64(2), "reward": float64(99)},
	})
	questIDs = p.recentActivity.ActiveQuestItems()
	if len(questIDs) != 2 {
		t.Fatalf("after quest item hook (flat), ActiveQuestItems() = %v, want 2 items", questIDs)
	}
}

func TestRecentActivityCountsMatchExpected(t *testing.T) {
	ra := NewRecentActivity()

	// Record several unique raids and some duplicates.
	ra.RecordRaid(25)
	ra.RecordRaid(150)
	ra.RecordRaid(25) // duplicate
	ra.RecordRaid(382)
	ra.RecordMaxBattle(812)
	ra.RecordMaxBattle(812) // duplicate
	ra.RecordQuestItem(1)
	ra.RecordQuestItem(2)
	ra.RecordQuestMegaEnergy(3)

	if got := len(ra.ActiveRaidBosses()); got != 3 {
		t.Errorf("ActiveRaidBosses count = %d, want 3", got)
	}
	if got := len(ra.ActiveMaxBattleBosses()); got != 1 {
		t.Errorf("ActiveMaxBattleBosses count = %d, want 1", got)
	}
	if got := len(ra.ActiveQuestItems()); got != 2 {
		t.Errorf("ActiveQuestItems count = %d, want 2", got)
	}
	if got := len(ra.ActiveQuestMegaEnergy()); got != 1 {
		t.Errorf("ActiveQuestMegaEnergy count = %d, want 1", got)
	}
}
