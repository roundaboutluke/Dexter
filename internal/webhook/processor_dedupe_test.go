package webhook

import (
	"fmt"
	"testing"
	"time"
)

func TestDedupePokemon_VerifiedMissingExpireUsesBoundedTTL(t *testing.T) {
	p := &Processor{cache: NewTTLCache()}
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"encounter_id":   "enc1",
			"verified":       true,
			"cp":             123,
			"disappear_time": 0,
		},
	}
	if !p.dedupePokemon(hook) {
		t.Fatalf("dedupePokemon=false, want true on first call")
	}
	key := fmt.Sprintf("%s%t%s%d:%d:%d:%d", "enc1", true, "123", 0, 0, 0, 0)
	expiry := p.cache.items[key]
	if expiry.IsZero() {
		t.Fatalf("expected bounded TTL for verified pokemon with missing expire, got zero expiry (no TTL)")
	}
	remaining := time.Until(expiry)
	if remaining <= 0 || remaining > 6*time.Minute {
		t.Fatalf("dedupePokemon TTL=%s, want within (0, 6m]", remaining)
	}
}

func TestDedupeMaxBattle_UsesBattleEndTTLAndSuppressesDuplicates(t *testing.T) {
	p := &Processor{cache: NewTTLCache()}
	end := time.Now().Add(2 * time.Hour).Unix()
	hook := &Hook{
		Type: "max_battle",
		Message: map[string]any{
			"id":                  "station1",
			"battle_end":          end,
			"battle_pokemon_id":   25,
			"battle_level":        2,
			"battle_pokemon_form": 0,
		},
	}
	if !p.dedupeMaxBattle(hook) {
		t.Fatalf("dedupeMaxBattle=false, want true on first call")
	}
	if p.dedupeMaxBattle(hook) {
		t.Fatalf("dedupeMaxBattle=true, want false on duplicate")
	}
	key := fmt.Sprintf("maxbattle:%s:%d:%d:%d:%d", "station1", end, 25, 0, 2)
	expiry := p.cache.items[key]
	if expiry.IsZero() {
		t.Fatalf("expected expiry for maxbattle dedupe key, got zero expiry")
	}
	remaining := time.Until(expiry)
	if remaining < 2*time.Hour || remaining > 2*time.Hour+6*time.Minute {
		t.Fatalf("dedupeMaxBattle TTL=%s, want within [2h, 2h+6m]", remaining)
	}
}

func TestDedupeMaxBattle_MissingBattleEndFallsBackTo90m(t *testing.T) {
	p := &Processor{cache: NewTTLCache()}
	hook := &Hook{
		Type: "max_battle",
		Message: map[string]any{
			"id":                "station1",
			"battle_pokemon_id": 25,
		},
	}
	if !p.dedupeMaxBattle(hook) {
		t.Fatalf("dedupeMaxBattle=false, want true on first call")
	}
	key := fmt.Sprintf("maxbattle:%s:%d:%d:%d:%d", "station1", int64(0), 25, 0, 0)
	expiry := p.cache.items[key]
	if expiry.IsZero() {
		t.Fatalf("expected expiry for maxbattle dedupe key, got zero expiry")
	}
	remaining := time.Until(expiry)
	if remaining < 89*time.Minute || remaining > 91*time.Minute {
		t.Fatalf("dedupeMaxBattle TTL=%s, want about 90m", remaining)
	}
}
