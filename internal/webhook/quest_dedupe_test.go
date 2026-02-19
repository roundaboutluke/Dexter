package webhook

import "testing"

func TestDedupeQuest_DistinguishesWithARVariant(t *testing.T) {
	p := &Processor{cache: NewTTLCache()}

	rewards := []any{
		map[string]any{
			"type": 7,
			"info": map[string]any{
				"pokemon_id": 554,
				"form_id":    2063,
			},
		},
	}

	withAR := &Hook{
		Type: "quest",
		Message: map[string]any{
			"pokestop_id": "46715a743526452b9c4512187c9cd830.16",
			"rewards":     rewards,
			"with_ar":     true,
		},
	}
	noAR := &Hook{
		Type: "quest",
		Message: map[string]any{
			"pokestop_id": "46715a743526452b9c4512187c9cd830.16",
			"rewards":     rewards,
			"with_ar":     false,
		},
	}

	if ok := p.dedupeQuest(withAR); !ok {
		t.Fatalf("expected first with_ar quest hook to pass dedupe")
	}
	if ok := p.dedupeQuest(withAR); ok {
		t.Fatalf("expected duplicate with_ar quest hook to be deduped")
	}
	if ok := p.dedupeQuest(noAR); !ok {
		t.Fatalf("expected no_ar quest hook with same rewards to pass as separate variant")
	}
	if ok := p.dedupeQuest(noAR); ok {
		t.Fatalf("expected duplicate no_ar quest hook to be deduped")
	}
}
