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

func TestGymInBattleMatchesPoracleJSTruthiness(t *testing.T) {
	tests := []struct {
		name    string
		message map[string]any
		want    bool
	}{
		{
			name:    "missing",
			message: map[string]any{},
			want:    false,
		},
		{
			name:    "bool false",
			message: map[string]any{"in_battle": false},
			want:    false,
		},
		{
			name:    "bool true",
			message: map[string]any{"in_battle": true},
			want:    true,
		},
		{
			name:    "numeric zero",
			message: map[string]any{"in_battle": 0},
			want:    false,
		},
		{
			name:    "numeric one",
			message: map[string]any{"in_battle": 1},
			want:    true,
		},
		{
			name:    "string true uppercase",
			message: map[string]any{"in_battle": "True"},
			want:    true,
		},
		{
			name:    "string false still truthy like js",
			message: map[string]any{"in_battle": "false"},
			want:    true,
		},
		{
			name:    "empty string falsey",
			message: map[string]any{"in_battle": ""},
			want:    false,
		},
		{
			name:    "prefer is_in_battle when present",
			message: map[string]any{"is_in_battle": "", "in_battle": 1},
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := gymInBattle(tt.message); got != tt.want {
				t.Fatalf("gymInBattle()=%t, want %t", got, tt.want)
			}
		})
	}
}

func TestMatchGymBattleChangesTreatsStringInBattleAsTrue(t *testing.T) {
	hook := &Hook{
		Type: "gym",
		Message: map[string]any{
			"id":                  "gym-battle",
			"team_id":             2,
			"old_team_id":         2,
			"slots_available":     4,
			"old_slots_available": 4,
			"in_battle":           "True",
		},
	}
	row := map[string]any{
		"team":           4,
		"battle_changes": 1,
		"slot_changes":   0,
	}

	if !matchGym(hook, row) {
		t.Fatalf("expected gym battle match for JS-truthy in_battle string")
	}
}
