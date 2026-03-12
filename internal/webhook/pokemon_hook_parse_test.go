package webhook

import (
	"fmt"
	"testing"
	"time"
)

func TestNormalizePvpRankings_FromNestedPvpMap(t *testing.T) {
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pvp": map[string]any{
				"great": []any{
					map[string]any{"pokemon": 735, "form": 3088, "cap": 50, "rank": 1873, "cp": 1498, "capped": true},
				},
				"little": []any{
					map[string]any{"pokemon": 734, "form": 3087, "cap": 50, "rank": 3660, "cp": 491, "capped": true},
				},
			},
		},
	}
	normalizePvpRankings(hook)

	if got := hook.Message["pvp_rankings_great_league"]; got == nil {
		t.Fatalf("missing pvp_rankings_great_league after normalizePvpRankings")
	}
	if got := hook.Message["pvp_rankings_little_league"]; got == nil {
		t.Fatalf("missing pvp_rankings_little_league after normalizePvpRankings")
	}
}

func TestDedupePokemon_DisappearTimeVerifiedActsVerified(t *testing.T) {
	p := &Processor{cache: NewTTLCache()}
	expire := time.Now().Add(time.Hour).Unix()
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"encounter_id":            "enc1",
			"disappear_time_verified": true,
			"disappear_time":          expire,
			"cp":                      103,
			"pokemon_id":              734,
			"form":                    3087,
			"costume":                 0,
			"gender":                  1,
		},
	}
	if !p.dedupePokemon(hook) {
		t.Fatalf("dedupePokemon=false, want true on first call")
	}
	if p.dedupePokemon(hook) {
		t.Fatalf("dedupePokemon=true, want false on duplicate")
	}

	key := fmt.Sprintf("%s%t%s%d:%d:%d:%d", "enc1", true, "103", 734, 3087, 0, 1)
	expiry := p.cache.items[key]
	if expiry.IsZero() {
		t.Fatalf("expected expiry for verified pokemon dedupe key, got zero expiry")
	}
	remaining := time.Until(expiry)
	if remaining < time.Hour || remaining > time.Hour+6*time.Minute {
		t.Fatalf("dedupePokemon TTL=%s, want within [1h, 1h+6m]", remaining)
	}
}
