package digest

import "testing"

func TestQuestDigestReplacesLatestRewardPerStopMode(t *testing.T) {
	store := NewStore()
	userID := "u1"
	profileNo := 1
	cycle := "quiet:cycle"
	stopKey := "stop-1"
	stopText := "Test Stop"

	store.Add(
		userID,
		profileNo,
		cycle,
		stopKey+"|No AR: 2 Ultra Ball",
		stopKey,
		stopText,
		"no",
		"No AR: 2 Ultra Ball",
	)
	store.Add(
		userID,
		profileNo,
		cycle,
		stopKey+"|No AR: 3 Razz Berry",
		stopKey,
		stopText,
		"no",
		"No AR: 3 Razz Berry",
	)

	summary, ok := store.Consume(userID, profileNo)
	if !ok || summary == nil {
		t.Fatalf("expected digest summary")
	}
	if summary.Total != 1 {
		t.Fatalf("total=%d, want 1", summary.Total)
	}
	if got := summary.Rewards["No AR: 3 Razz Berry"]; got != 1 {
		t.Fatalf("latest reward count=%d, want 1", got)
	}
	if _, exists := summary.Rewards["No AR: 2 Ultra Ball"]; exists {
		t.Fatalf("old reward should have been replaced")
	}
	if !summary.Stops["No AR: 3 Razz Berry"][stopText] {
		t.Fatalf("latest reward missing stop text")
	}
}

func TestQuestDigestAllowsOneWithARAndOneNoARPerStop(t *testing.T) {
	store := NewStore()
	userID := "u2"
	profileNo := 1
	cycle := "quiet:cycle"
	stopKey := "stop-2"
	stopText := "Stop Two"

	store.Add(
		userID,
		profileNo,
		cycle,
		stopKey+"|With AR: Spinda 09",
		stopKey,
		stopText,
		"with",
		"With AR: Spinda 09",
	)
	store.Add(
		userID,
		profileNo,
		cycle,
		stopKey+"|No AR: Trapinch",
		stopKey,
		stopText,
		"no",
		"No AR: Trapinch",
	)

	summary, ok := store.Consume(userID, profileNo)
	if !ok || summary == nil {
		t.Fatalf("expected digest summary")
	}
	if summary.Total != 2 {
		t.Fatalf("total=%d, want 2", summary.Total)
	}
	if summary.Rewards["With AR: Spinda 09"] != 1 {
		t.Fatalf("missing with-ar reward")
	}
	if summary.Rewards["No AR: Trapinch"] != 1 {
		t.Fatalf("missing no-ar reward")
	}
}

func TestQuestDigestIgnoresDuplicateRescanForSameStopModeReward(t *testing.T) {
	store := NewStore()
	userID := "u3"
	profileNo := 1
	cycle := "quiet:cycle"
	stopKey := "stop-3"
	stopText := "Stop Three"
	reward := "No AR: 2 Ultra Ball"

	store.Add(userID, profileNo, cycle, stopKey+"|"+reward, stopKey, stopText, "no", reward)
	store.Add(userID, profileNo, cycle, stopKey+"|"+reward, stopKey, stopText, "no", reward)

	summary, ok := store.Consume(userID, profileNo)
	if !ok || summary == nil {
		t.Fatalf("expected digest summary")
	}
	if summary.Total != 1 {
		t.Fatalf("total=%d, want 1", summary.Total)
	}
	if summary.Rewards[reward] != 1 {
		t.Fatalf("reward count=%d, want 1", summary.Rewards[reward])
	}
}

func TestQuestDigestFallsBackToSeenKeyWhenStopUnknown(t *testing.T) {
	store := NewStore()
	userID := "u4"
	profileNo := 1
	cycle := "quiet:cycle"
	reward := "No AR: 2 Ultra Ball"

	store.Add(userID, profileNo, cycle, "seen-1", "", "", "no", reward)
	store.Add(userID, profileNo, cycle, "seen-1", "", "", "no", reward)

	summary, ok := store.Consume(userID, profileNo)
	if !ok || summary == nil {
		t.Fatalf("expected digest summary")
	}
	if summary.Total != 1 {
		t.Fatalf("total=%d, want 1", summary.Total)
	}
}
