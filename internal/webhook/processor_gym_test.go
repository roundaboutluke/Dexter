package webhook

import "testing"

func TestDedupeGymSetsOldFieldsWhenNoCache(t *testing.T) {
	p := &Processor{
		cache:    NewTTLCache(),
		gymCache: NewGymCache(),
	}

	hook := &Hook{
		Type: "gym",
		Message: map[string]any{
			"id":              "gym1",
			"team_id":         0,
			"slots_available": 2,
		},
	}

	if !p.dedupeGym(hook) {
		t.Fatalf("expected dedupeGym to allow first event")
	}
	if got := getInt(hook.Message["old_team_id"]); got != -1 {
		t.Fatalf("old_team_id=%d, want -1", got)
	}
	if got := getInt(hook.Message["old_slots_available"]); got != -1 {
		t.Fatalf("old_slots_available=%d, want -1", got)
	}
	if got := getInt(hook.Message["old_in_battle"]); got != -1 {
		t.Fatalf("old_in_battle=%d, want -1", got)
	}
	if got := getInt(hook.Message["last_owner_id"]); got != -1 {
		t.Fatalf("last_owner_id=%d, want -1", got)
	}

	state := p.gymCache.Get("gym1")
	if state == nil {
		t.Fatalf("expected gym cache entry")
	}
	if state.LastOwnerID != -1 {
		t.Fatalf("cached LastOwnerID=%d, want -1", state.LastOwnerID)
	}
}

func TestDedupeGymCachesLastOwnerWhenTeamPresent(t *testing.T) {
	p := &Processor{
		cache:    NewTTLCache(),
		gymCache: NewGymCache(),
	}

	hook := &Hook{
		Type: "gym",
		Message: map[string]any{
			"id":              "gym2",
			"team_id":         2,
			"slots_available": 3,
		},
	}

	if !p.dedupeGym(hook) {
		t.Fatalf("expected dedupeGym to allow first event")
	}
	state := p.gymCache.Get("gym2")
	if state == nil {
		t.Fatalf("expected gym cache entry")
	}
	if state.LastOwnerID != 2 {
		t.Fatalf("cached LastOwnerID=%d, want 2", state.LastOwnerID)
	}
}
