package webhook

import (
	"testing"

	"poraclego/internal/data"
)

func TestInvasionTrackTypeKeepsMetalForMatching(t *testing.T) {
	p := &Processor{
		data: &data.GameData{
			Grunts: map[string]any{
				"1": map[string]any{"type": "Metal"},
			},
		},
	}

	hook := &Hook{
		Type: "invasion",
		Message: map[string]any{
			"display_type": 0,
			"grunt_type":   1,
		},
	}

	if got := invasionTrackType(p, hook); got != "metal" {
		t.Fatalf("invasionTrackType=%q, want %q", got, "metal")
	}

	row := map[string]any{
		"grunt_type": "metal",
		"gender":     0,
	}
	if got := matchInvasionWithData(p, hook, row); !got {
		t.Fatalf("matchInvasionWithData=false, want true")
	}
}

