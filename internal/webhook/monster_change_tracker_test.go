package webhook

import (
	"testing"
	"time"
)

func TestMonsterChangeDetectChangeFlap(t *testing.T) {
	tracker := NewMonsterChangeTracker(nil, "")
	encounterID := "enc-1"
	expires := time.Now().Add(30 * time.Minute).Unix()
	target := alertTarget{
		ID:       "u1",
		Type:     "discord:user",
		Name:     "User 1",
		Language: "en",
		Template: "1",
	}

	hookA := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"encounter_id": encounterID,
			"pokemon_id":   1,
			"form":         0,
			"costume":      0,
			"gender":       1,
			"cp":           100,
		},
	}
	hookB := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"encounter_id": encounterID,
			"pokemon_id":   2,
			"form":         0,
			"costume":      0,
			"gender":       1,
			"cp":           200,
		},
	}
	hookBVariant := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"encounter_id": encounterID,
			"pokemon_id":   2,
			"form":         0,
			"costume":      0,
			"gender":       2,
			"cp":           220,
		},
	}

	tracker.TrackCare(encounterID, target, expires, false, "", "pokemon:enc-1:row-1", hookA)

	if _, _, changed, flapping := tracker.DetectChange(encounterID, hookA, expires); changed || flapping {
		t.Fatalf("expected unchanged state for identical signature, changed=%v flapping=%v", changed, flapping)
	}

	old, cares, changed, flapping := tracker.DetectChange(encounterID, hookB, expires)
	if !changed {
		t.Fatalf("expected change on A->B transition")
	}
	if flapping {
		t.Fatalf("expected first transition not to be marked as flapping")
	}
	if old.PokemonID != 1 {
		t.Fatalf("old pokemon=%d, want 1", old.PokemonID)
	}
	if len(cares) != 1 {
		t.Fatalf("cares=%d, want 1", len(cares))
	}

	old, cares, changed, flapping = tracker.DetectChange(encounterID, hookA, expires)
	if !changed {
		t.Fatalf("expected change on B->A transition")
	}
	if !flapping {
		t.Fatalf("expected A->B->A transition to be marked as flapping")
	}
	if old.PokemonID != 2 {
		t.Fatalf("old pokemon=%d, want 2", old.PokemonID)
	}
	if len(cares) != 1 {
		t.Fatalf("cares=%d, want 1", len(cares))
	}

	// Once A/B flap is marked, additional A<->B flips should be suppressed.
	if _, _, changed, flapping = tracker.DetectChange(encounterID, hookB, expires); changed || flapping {
		t.Fatalf("expected additional A/B flip to be suppressed, changed=%v flapping=%v", changed, flapping)
	}
	if _, _, changed, flapping = tracker.DetectChange(encounterID, hookBVariant, expires); changed || flapping {
		t.Fatalf("expected A/B pair variant to be suppressed, changed=%v flapping=%v", changed, flapping)
	}

	// A third signature should break A/B lock and produce a fresh change.
	hookC := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"encounter_id": encounterID,
			"pokemon_id":   3,
			"form":         0,
			"costume":      0,
			"gender":       1,
			"cp":           300,
		},
	}
	old, cares, changed, flapping = tracker.DetectChange(encounterID, hookC, expires)
	if !changed {
		t.Fatalf("expected third signature transition to produce a fresh change")
	}
	if flapping {
		t.Fatalf("expected third signature transition not to be marked as A/B flap")
	}
	if old.PokemonID != 2 {
		t.Fatalf("old pokemon=%d, want 2", old.PokemonID)
	}
	if len(cares) != 1 {
		t.Fatalf("cares=%d, want 1", len(cares))
	}
}

func TestMonsterChangeUpdateKey(t *testing.T) {
	if got := monsterChangeUpdateKey("pokemon:enc1:uid1"); got != "monsterchange:pokemon:enc1:uid1" {
		t.Fatalf("monsterChangeUpdateKey=%q, want %q", got, "monsterchange:pokemon:enc1:uid1")
	}
	if got := monsterChangeUpdateKey("   "); got != "" {
		t.Fatalf("monsterChangeUpdateKey=%q, want empty", got)
	}
}

func TestMonsterChangeLockIsEncounterScoped(t *testing.T) {
	tracker := NewMonsterChangeTracker(nil, "")
	expires := time.Now().Add(30 * time.Minute).Unix()
	target := alertTarget{
		ID:       "u1",
		Type:     "discord:user",
		Name:     "User 1",
		Language: "en",
		Template: "1",
	}

	hookAEnc1 := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-1", "pokemon_id": 1, "form": 0, "costume": 0, "gender": 1, "cp": 100}}
	hookBEnc1 := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-1", "pokemon_id": 2, "form": 0, "costume": 0, "gender": 1, "cp": 120}}
	hookA2Enc1 := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-1", "pokemon_id": 1, "form": 0, "costume": 0, "gender": 1, "cp": 130}}

	hookAEnc2 := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-2", "pokemon_id": 1, "form": 0, "costume": 0, "gender": 1, "cp": 100}}
	hookBEnc2 := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-2", "pokemon_id": 2, "form": 0, "costume": 0, "gender": 2, "cp": 200}}

	tracker.TrackCare("enc-1", target, expires, false, "", "pokemon:enc-1:row-1", hookAEnc1)
	tracker.TrackCare("enc-2", target, expires, false, "", "pokemon:enc-2:row-1", hookAEnc2)

	if _, _, changed, _ := tracker.DetectChange("enc-1", hookBEnc1, expires); !changed {
		t.Fatalf("expected enc-1 A->B change")
	}
	if _, _, changed, flapping := tracker.DetectChange("enc-1", hookA2Enc1, expires); !changed || !flapping {
		t.Fatalf("expected enc-1 B->A to flap-lock, changed=%v flapping=%v", changed, flapping)
	}
	if _, _, changed, flapping := tracker.DetectChange("enc-1", hookBEnc1, expires); changed || flapping {
		t.Fatalf("expected enc-1 A/B to be suppressed after lock, changed=%v flapping=%v", changed, flapping)
	}

	// enc-2 should still behave independently and emit its own first change.
	if _, _, changed, flapping := tracker.DetectChange("enc-2", hookBEnc2, expires); !changed || flapping {
		t.Fatalf("expected enc-2 first change unaffected by enc-1 lock, changed=%v flapping=%v", changed, flapping)
	}
}

func TestMonsterChangeLockSuppressesFormVariantsWithinPair(t *testing.T) {
	tracker := NewMonsterChangeTracker(nil, "")
	expires := time.Now().Add(30 * time.Minute).Unix()
	target := alertTarget{
		ID:       "u1",
		Type:     "discord:user",
		Name:     "User 1",
		Language: "en",
		Template: "1",
	}

	hookA := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-form", "pokemon_id": 1, "form": 0, "costume": 0, "gender": 1, "cp": 100}}
	hookB := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-form", "pokemon_id": 2, "form": 0, "costume": 0, "gender": 1, "cp": 120}}
	hookAFlap := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-form", "pokemon_id": 1, "form": 0, "costume": 0, "gender": 1, "cp": 130}}
	hookBFormVariant := &Hook{Type: "pokemon", Message: map[string]any{"encounter_id": "enc-form", "pokemon_id": 2, "form": 99, "costume": 0, "gender": 1, "cp": 140}}

	tracker.TrackCare("enc-form", target, expires, false, "", "pokemon:enc-form:row-1", hookA)

	if _, _, changed, _ := tracker.DetectChange("enc-form", hookB, expires); !changed {
		t.Fatalf("expected first A->B change")
	}
	if _, _, changed, flapping := tracker.DetectChange("enc-form", hookAFlap, expires); !changed || !flapping {
		t.Fatalf("expected B->A flap lock, changed=%v flapping=%v", changed, flapping)
	}
	if _, _, changed, flapping := tracker.DetectChange("enc-form", hookBFormVariant, expires); changed || flapping {
		t.Fatalf("expected B form variant to be suppressed within A/B lock, changed=%v flapping=%v", changed, flapping)
	}
}
