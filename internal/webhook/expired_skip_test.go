package webhook

import (
	"testing"
	"time"
)

func TestShouldSkipExpiredHook(t *testing.T) {
	now := time.Now().Unix()
	hook := &Hook{
		Type: "pokemon",
		Message: map[string]any{
			"disappear_time": now - 10,
		},
	}
	if !shouldSkipExpiredHook(hook) {
		t.Fatalf("expected expired pokemon hook to be skipped")
	}

	hook2 := &Hook{
		Type: "egg",
		Message: map[string]any{
			"start": now - 1,
		},
	}
	if !shouldSkipExpiredHook(hook2) {
		t.Fatalf("expected expired egg hook to be skipped")
	}

	hook3 := &Hook{
		Type: "gym",
		Message: map[string]any{
			"id": "x",
		},
	}
	if shouldSkipExpiredHook(hook3) {
		t.Fatalf("did not expect gym hook to be skipped")
	}

	hook4 := &Hook{
		Type: "nest",
		Message: map[string]any{
			"reset_time": now - 10,
		},
	}
	if shouldSkipExpiredHook(hook4) {
		t.Fatalf("did not expect nest hook to be skipped")
	}

	hook5 := &Hook{
		Type: "fort_update",
		Message: map[string]any{
			"reset_time": now - 10,
		},
	}
	if shouldSkipExpiredHook(hook5) {
		t.Fatalf("did not expect fort_update hook to be skipped")
	}

	hook6 := &Hook{
		Type: "max_battle",
		Message: map[string]any{
			"battle_end": now - 10,
		},
	}
	if !shouldSkipExpiredHook(hook6) {
		t.Fatalf("expected expired max_battle hook to be skipped")
	}
}
