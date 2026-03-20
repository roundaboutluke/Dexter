package digest

import (
	"testing"
	"time"
)

func TestCycleKey(t *testing.T) {
	got := CycleKey(time.Date(2025, 1, 15, 9, 0, 0, 0, time.UTC))
	want := "2025011509"
	if got != want {
		t.Errorf("CycleKey() = %q, want %q", got, want)
	}
}

func TestAddAndConsume(t *testing.T) {
	s := NewStore()
	s.Add("user1", 1, "cycle1", "", "stop1", "Park Stop", "any", "Stardust x500")
	summary, ok := s.Consume("user1", 1)
	if !ok || summary == nil {
		t.Fatal("expected summary from Consume")
	}
	if summary.Total != 1 {
		t.Errorf("Total = %d, want 1", summary.Total)
	}
	if summary.Rewards["Stardust x500"] != 1 {
		t.Errorf("Rewards[Stardust x500] = %d, want 1", summary.Rewards["Stardust x500"])
	}
}

func TestConsumeEmpty(t *testing.T) {
	s := NewStore()
	_, ok := s.Consume("user1", 1)
	if ok {
		t.Error("expected ok=false for empty store")
	}
}

func TestConsumeClears(t *testing.T) {
	s := NewStore()
	s.Add("user1", 1, "cycle1", "", "stop1", "Stop", "any", "Reward")
	s.Consume("user1", 1)
	_, ok := s.Consume("user1", 1)
	if ok {
		t.Error("expected ok=false after second consume")
	}
}

func TestDeduplicate_SameStopReward(t *testing.T) {
	s := NewStore()
	s.Add("user1", 1, "cycle1", "", "stop1", "Park Stop", "any", "Stardust x500")
	s.Add("user1", 1, "cycle1", "", "stop1", "Park Stop", "any", "Stardust x500")
	summary, ok := s.Consume("user1", 1)
	if !ok || summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Total != 1 {
		t.Errorf("Total = %d, want 1 (dedup)", summary.Total)
	}
}

func TestCycleKeyReset(t *testing.T) {
	s := NewStore()
	s.Add("user1", 1, "cycle1", "", "stop1", "Stop", "any", "Reward1")
	s.Add("user1", 1, "cycle2", "", "stop2", "Stop2", "any", "Reward2")
	summary, ok := s.Consume("user1", 1)
	if !ok || summary == nil {
		t.Fatal("expected summary")
	}
	// New cycle should have only the second reward
	if summary.Total != 1 {
		t.Errorf("Total = %d, want 1", summary.Total)
	}
	if summary.Rewards["Reward2"] != 1 {
		t.Errorf("Rewards[Reward2] = %d, want 1", summary.Rewards["Reward2"])
	}
}

func TestStopModeReplacement(t *testing.T) {
	s := NewStore()
	s.Add("user1", 1, "cycle1", "", "stop1", "Stop", "any", "Reward1")
	s.Add("user1", 1, "cycle1", "", "stop1", "Stop", "any", "Reward2")
	summary, ok := s.Consume("user1", 1)
	if !ok || summary == nil {
		t.Fatal("expected summary")
	}
	if summary.Rewards["Reward1"] != 0 {
		t.Errorf("Rewards[Reward1] = %d, want 0 (replaced)", summary.Rewards["Reward1"])
	}
	if summary.Rewards["Reward2"] != 1 {
		t.Errorf("Rewards[Reward2] = %d, want 1", summary.Rewards["Reward2"])
	}
}

func TestBeginEndQuiet(t *testing.T) {
	s := NewStore()
	s.BeginQuiet("user1")
	key1 := s.CycleKeyFor("user1", time.Now())
	if key1 == "" {
		t.Fatal("expected non-empty cycle key during quiet")
	}
	if key1 == CycleKey(time.Now()) {
		t.Error("quiet cycle key should differ from normal CycleKey")
	}
	s.EndQuiet("user1")
	key2 := s.CycleKeyFor("user1", time.Now())
	if key2 != CycleKey(time.Now()) {
		t.Errorf("after EndQuiet, CycleKeyFor = %q, want %q", key2, CycleKey(time.Now()))
	}
}

func TestTopRewards(t *testing.T) {
	rewards := map[string]int{
		"Stardust x500": 5,
		"Rare Candy":    3,
		"Poke Ball x10": 8,
	}
	got := TopRewards(rewards, 2)
	if len(got) != 2 {
		t.Fatalf("TopRewards returned %d items, want 2", len(got))
	}
	if got[0] != "Poke Ball x10 x8" {
		t.Errorf("got[0] = %q, want %q", got[0], "Poke Ball x10 x8")
	}
	if got[1] != "Stardust x500 x5" {
		t.Errorf("got[1] = %q, want %q", got[1], "Stardust x500 x5")
	}
}

func TestRewardsWithStops(t *testing.T) {
	rewards := map[string]int{"Stardust x500": 2}
	stops := map[string]map[string]bool{
		"Stardust x500": {"stopA": true, "stopB": true},
	}
	names := map[string]string{"stopA": "Park Stop", "stopB": "Gym Stop"}
	got := RewardsWithStops(rewards, stops, names)
	if len(got) != 1 {
		t.Fatalf("got %d items, want 1", len(got))
	}
	// Should contain " at " with sorted stop names
	want := "Stardust x500 at Gym Stop, Park Stop"
	if got[0] != want {
		t.Errorf("got %q, want %q", got[0], want)
	}
}
