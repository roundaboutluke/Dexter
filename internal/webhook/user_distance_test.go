package webhook

import (
	"testing"

	"poraclego/internal/config"
)

func TestPokemonRenderAddsUserDistanceAndTrackingFlags(t *testing.T) {
	p := &Processor{cfg: &config.Config{}}
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id":     1,
			"latitude":       1.0,
			"longitude":      1.0,
			"disappear_time": int64(1700000500),
		},
	}

	payload := buildRenderData(p, hook, alertMatch{
		Target: alertTarget{Platform: "discord", Lat: 1.001, Lon: 1.0},
		Row:    map[string]any{"distance": 500},
	})

	if got := getBool(payload["hasUserDistance"]); !got {
		t.Fatalf("hasUserDistance=%v, want true", got)
	}
	if got := getInt(payload["userDistanceM"]); got <= 0 {
		t.Fatalf("userDistanceM=%d, want >0", got)
	}
	if got := getBool(payload["isDistanceTrack"]); !got {
		t.Fatalf("isDistanceTrack=%v, want true", got)
	}
	if got := getBool(payload["isAreaTrack"]); got {
		t.Fatalf("isAreaTrack=%v, want false", got)
	}
	if got := getInt(payload["trackDistanceM"]); got != 500 {
		t.Fatalf("trackDistanceM=%d, want %d", got, 500)
	}
}

func TestPokemonRenderAreaTrackingStillComputesDistanceWhenUserHasLocation(t *testing.T) {
	p := &Processor{cfg: &config.Config{}}
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id": 1,
			"latitude":   1.0,
			"longitude":  1.0,
		},
	}

	payload := buildRenderData(p, hook, alertMatch{
		Target: alertTarget{Platform: "discord", Lat: 1.001, Lon: 1.0, Areas: []string{"test"}},
		Row:    map[string]any{"distance": 0},
	})

	if got := getBool(payload["isAreaTrack"]); !got {
		t.Fatalf("isAreaTrack=%v, want true", got)
	}
	if got := getBool(payload["hasUserDistance"]); !got {
		t.Fatalf("hasUserDistance=%v, want true", got)
	}
}
