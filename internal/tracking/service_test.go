package tracking

import "testing"

func TestPlanUpsert(t *testing.T) {
	desired := []map[string]any{
		{"pokemon_id": 1, "distance": 500, "template": "1", "clean": 0},
		{"pokemon_id": 2, "distance": 0, "template": "1", "clean": 0},
		{"pokemon_id": 3, "distance": 0, "template": "1", "clean": 0},
	}
	existing := []map[string]any{
		{"uid": 11, "pokemon_id": 1, "distance": 500, "template": "1", "clean": 0},
		{"uid": 12, "pokemon_id": 2, "distance": 250, "template": "1", "clean": 0},
	}

	plan := PlanUpsert(desired, existing, func(candidate, existing map[string]any) bool {
		return candidate["pokemon_id"] == existing["pokemon_id"]
	}, "distance", "template", "clean")

	if len(plan.Unchanged) != 1 {
		t.Fatalf("expected 1 unchanged row, got %d", len(plan.Unchanged))
	}
	if len(plan.Updates) != 1 {
		t.Fatalf("expected 1 update row, got %d", len(plan.Updates))
	}
	if len(plan.Inserts) != 1 {
		t.Fatalf("expected 1 insert row, got %d", len(plan.Inserts))
	}
	if plan.Updates[0]["uid"] != 12 {
		t.Fatalf("expected update uid to be preserved, got %v", plan.Updates[0]["uid"])
	}
}

func TestPlanUpsertContinuesScanningSameIdentityRowsBeforeInserting(t *testing.T) {
	desired := []map[string]any{
		{"team": 1, "level": 5, "distance": 250, "template": "1", "clean": 0},
	}
	existing := []map[string]any{
		{"uid": 21, "team": 1, "level": 4, "distance": 250, "template": "1", "clean": 0},
		{"uid": 22, "team": 1, "level": 5, "distance": 500, "template": "1", "clean": 0},
	}

	plan := PlanUpsert(desired, existing, func(candidate, existing map[string]any) bool {
		return candidate["team"] == existing["team"]
	}, "distance", "template", "clean")

	if len(plan.Inserts) != 0 {
		t.Fatalf("expected candidate to be classified as update, got insert")
	}
	if len(plan.Updates) != 1 {
		t.Fatalf("expected 1 update row, got %d", len(plan.Updates))
	}
	if plan.Updates[0]["uid"] != 22 {
		t.Fatalf("expected update uid 22, got %v", plan.Updates[0]["uid"])
	}
}

func TestPlanUpsertPrefersExactMatchOverEarlierMutableUpdate(t *testing.T) {
	desired := []map[string]any{
		{"team": 1, "distance": 500, "template": "1", "clean": 0},
	}
	existing := []map[string]any{
		{"uid": 31, "team": 1, "distance": 250, "template": "1", "clean": 0},
		{"uid": 32, "team": 1, "distance": 500, "template": "1", "clean": 0},
	}

	plan := PlanUpsert(desired, existing, func(candidate, existing map[string]any) bool {
		return candidate["team"] == existing["team"]
	}, "distance", "template", "clean")

	if len(plan.Unchanged) != 1 {
		t.Fatalf("expected exact match to be unchanged, got %d unchanged", len(plan.Unchanged))
	}
	if len(plan.Updates) != 0 {
		t.Fatalf("expected no update when an exact match exists, got %d", len(plan.Updates))
	}
	if len(plan.Inserts) != 0 {
		t.Fatalf("expected no insert when an exact match exists, got %d", len(plan.Inserts))
	}
}

func TestChangeMessage(t *testing.T) {
	plan := UpsertPlan{
		Unchanged: []map[string]any{{"name": "alpha"}},
		Updates:   []map[string]any{{"name": "beta"}},
		Inserts:   []map[string]any{{"name": "gamma"}},
	}

	got := ChangeMessage(nil, "!", "tracked", plan, func(row map[string]any) string {
		return row["name"].(string)
	})

	want := "Unchanged: alpha\nUpdated: beta\nNew: gamma"
	if got != want {
		t.Fatalf("ChangeMessage() = %q, want %q", got, want)
	}
}

func TestChangeMessageLargePlan(t *testing.T) {
	plan := UpsertPlan{
		Inserts: make([]map[string]any, 51),
	}

	got := ChangeMessage(nil, "!", "tracked", plan, func(row map[string]any) string {
		return ""
	})

	want := "I have made a lot of changes. See !tracked for details"
	if got != want {
		t.Fatalf("ChangeMessage() = %q, want %q", got, want)
	}
}

func TestIsSingleMutableFieldUpdate(t *testing.T) {
	if !IsSingleMutableFieldUpdate([]string{"uid", "distance"}, "distance", "template") {
		t.Fatalf("expected uid+distance to be treated as mutable update")
	}
	if IsSingleMutableFieldUpdate([]string{"uid", "pokemon_id"}, "distance", "template") {
		t.Fatalf("did not expect uid+pokemon_id to be treated as mutable update")
	}
}
