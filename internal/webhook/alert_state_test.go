package webhook

import (
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"dexter/internal/alertstate"
	"dexter/internal/config"
)

func testSnapshotConfig(tb testing.TB) *config.Config {
	tb.Helper()
	return loadTestConfigFromMap(tb, map[string]any{
		"general": map[string]any{
			"locale":              "en",
			"defaultTemplateName": "1",
		},
	})
}

func emptySnapshot() *alertstate.Snapshot {
	return &alertstate.Snapshot{
		Tables:       map[string][]map[string]any{},
		Humans:       map[string]map[string]any{},
		Profiles:     map[string]map[string]any{},
		HasSchedules: map[string]bool{},
	}
}

func TestMatchTargetsUsesSnapshotWithoutDatabase(t *testing.T) {
	cfg := testSnapshotConfig(t)
	row := baseMonsterRow(25, 0)
	row["id"] = "user-1"
	row["profile_no"] = 1
	row["template"] = "1"

	p := &Processor{
		cfg:        cfg,
		alertState: alertstate.NewManager(),
	}
	p.alertState.Set(&alertstate.Snapshot{
		Tables: map[string][]map[string]any{
			"monsters": {row},
		},
		Humans: map[string]map[string]any{
			"user-1": {
				"id":                 "user-1",
				"type":               "discord",
				"name":               "User 1",
				"language":           "en",
				"enabled":            1,
				"admin_disable":      0,
				"schedule_disabled":  0,
				"current_profile_no": 1,
			},
		},
		Profiles:     map[string]map[string]any{},
		HasSchedules: map[string]bool{},
	})

	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id":         25,
			"form":               0,
			"cp":                 500,
			"pokemon_level":      20,
			"gender":             0,
			"individual_attack":  10,
			"individual_defense": 10,
			"individual_stamina": 10,
			"latitude":           1.23,
			"longitude":          4.56,
		},
	}

	matches, err := p.matchTargets(hook)
	if err != nil {
		t.Fatalf("matchTargets error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matchTargets len=%d, want 1", len(matches))
	}
	if matches[0].Target.ID != "user-1" {
		t.Fatalf("target id=%q, want user-1", matches[0].Target.ID)
	}
	if matches[0].Target.Template != "1" {
		t.Fatalf("template=%q, want 1", matches[0].Target.Template)
	}
}

func TestMatchTargetsQuestSnapshotDoesNotMutateStoredRow(t *testing.T) {
	cfg := testSnapshotConfig(t)
	row := map[string]any{
		"id":          "user-1",
		"profile_no":  1,
		"reward_type": 2,
		"reward":      1,
		"amount":      3,
		"form":        0,
		"shiny":       0,
		"ar":          0,
	}

	p := &Processor{
		cfg:        cfg,
		alertState: alertstate.NewManager(),
	}
	p.alertState.Set(&alertstate.Snapshot{
		Tables: map[string][]map[string]any{
			"quest": {row},
		},
		Humans: map[string]map[string]any{
			"user-1": {
				"id":                 "user-1",
				"type":               "discord",
				"name":               "User 1",
				"language":           "en",
				"enabled":            1,
				"admin_disable":      0,
				"schedule_disabled":  0,
				"current_profile_no": 1,
			},
		},
		Profiles:     map[string]map[string]any{},
		HasSchedules: map[string]bool{},
	})

	hook := &Hook{
		Type: "quest",
		Message: map[string]any{
			"reward_type":   2,
			"reward":        1,
			"reward_amount": 3,
			"latitude":      1.23,
			"longitude":     4.56,
		},
	}

	matches, err := p.matchTargets(hook)
	if err != nil {
		t.Fatalf("matchTargets error: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matchTargets len=%d, want 1", len(matches))
	}
	if got := getInt(matches[0].Row["questMatchNoAR"]); got != 1 {
		t.Fatalf("questMatchNoAR=%d, want 1", got)
	}
	if _, ok := row["questMatchNoAR"]; ok {
		t.Fatalf("stored row mutated with questMatchNoAR")
	}
	if _, ok := row["questMatchAR"]; ok {
		t.Fatalf("stored row mutated with questMatchAR")
	}
}

func TestMatchTargetsSnapshotEmptyTableSkipsDatabase(t *testing.T) {
	p := &Processor{
		cfg:        testSnapshotConfig(t),
		alertState: alertstate.NewManager(),
	}
	p.alertState.Set(emptySnapshot())

	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id": 25,
			"form":       0,
		},
	}

	matches, err := p.matchTargets(hook)
	if err != nil {
		t.Fatalf("matchTargets error: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("matchTargets len=%d, want 0", len(matches))
	}
}

func TestRefreshAlertCacheSyncRetainsPreviousSnapshotOnFailure(t *testing.T) {
	p := &Processor{alertState: alertstate.NewManager()}
	previous := emptySnapshot()
	previous.Tables["monsters"] = []map[string]any{{"id": "user-1"}}
	p.alertState.Set(previous)
	p.SetAlertStateLoader(func() (*alertstate.Snapshot, error) {
		return nil, errors.New("boom")
	})

	err := p.RefreshAlertCacheSync()
	if err == nil {
		t.Fatalf("expected refresh error")
	}
	if got := p.currentAlertState(); got != previous {
		t.Fatalf("snapshot replaced on failure")
	}
}

func TestRefreshAlertCacheAsyncCoalescesPendingRequests(t *testing.T) {
	p := &Processor{alertState: alertstate.NewManager()}
	firstStarted := make(chan struct{})
	unblockFirst := make(chan struct{})
	var loads atomic.Int32
	p.SetAlertStateLoader(func() (*alertstate.Snapshot, error) {
		call := loads.Add(1)
		if call == 1 {
			close(firstStarted)
			<-unblockFirst
		}
		return emptySnapshot(), nil
	})

	p.RefreshAlertCacheAsync()
	<-firstStarted
	p.RefreshAlertCacheAsync()
	p.RefreshAlertCacheAsync()
	close(unblockFirst)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if loads.Load() == 2 {
			p.alertStateRefreshMu.Lock()
			refreshing := p.alertStateRefreshing
			p.alertStateRefreshMu.Unlock()
			if !refreshing {
				break
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	if got := loads.Load(); got != 2 {
		t.Fatalf("load count=%d, want 2", got)
	}
	p.alertStateRefreshMu.Lock()
	defer p.alertStateRefreshMu.Unlock()
	if p.alertStateRefreshing {
		t.Fatalf("refresh loop still running")
	}
	if p.alertStatePending {
		t.Fatalf("pending refresh flag not cleared")
	}
}

func BenchmarkMatchTargetsPokemonSnapshot(b *testing.B) {
	cfg := testSnapshotConfig(b)
	rows := make([]map[string]any, 0, 512)
	humans := make(map[string]map[string]any, 512)
	for i := 0; i < 512; i++ {
		id := fmt.Sprintf("user-%d", i)
		pokemonID := 999
		if i == 0 {
			pokemonID = 25
		}
		row := baseMonsterRow(pokemonID, 0)
		row["id"] = id
		row["profile_no"] = 1
		row["template"] = "1"
		rows = append(rows, row)
		humans[id] = map[string]any{
			"id":                 id,
			"type":               "discord",
			"name":               id,
			"language":           "en",
			"enabled":            1,
			"admin_disable":      0,
			"schedule_disabled":  0,
			"current_profile_no": 1,
		}
	}

	p := &Processor{
		cfg:        cfg,
		alertState: alertstate.NewManager(),
	}
	p.alertState.Set(&alertstate.Snapshot{
		Tables: map[string][]map[string]any{
			"monsters": rows,
		},
		Humans:       humans,
		Profiles:     map[string]map[string]any{},
		HasSchedules: map[string]bool{},
	})

	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"pokemon_id":         25,
			"form":               0,
			"cp":                 500,
			"pokemon_level":      20,
			"gender":             0,
			"individual_attack":  10,
			"individual_defense": 10,
			"individual_stamina": 10,
			"latitude":           1.23,
			"longitude":          4.56,
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matches, err := p.matchTargets(hook)
		if err != nil {
			b.Fatalf("matchTargets error: %v", err)
		}
		if len(matches) != 1 {
			b.Fatalf("matchTargets len=%d, want 1", len(matches))
		}
	}
}
