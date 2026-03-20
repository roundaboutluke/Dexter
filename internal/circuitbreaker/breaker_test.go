package circuitbreaker

import (
	"sync"
	"testing"
	"time"
)

func TestNewDefaults(t *testing.T) {
	b := New("test", 0, 0)
	if b.failThreshold < 1 {
		t.Fatal("failThreshold should default to positive value")
	}
	if b.cooldown <= 0 {
		t.Fatal("cooldown should default to positive value")
	}
	if b.State() != Closed {
		t.Fatal("initial state should be Closed")
	}
}

func TestClosedToOpen(t *testing.T) {
	b := New("test", 3, time.Minute)
	var transitions []State
	b.SetOnStateChange(func(_ string, _, to State) {
		transitions = append(transitions, to)
	})

	for i := 0; i < 3; i++ {
		if !b.Allow() {
			t.Fatalf("should allow in closed state, attempt %d", i)
		}
		b.RecordFailure()
	}

	if b.State() != Open {
		t.Fatalf("expected Open, got %v", b.State())
	}
	if len(transitions) != 1 || transitions[0] != Open {
		t.Fatalf("expected one transition to Open, got %v", transitions)
	}
}

func TestOpenBlocksCalls(t *testing.T) {
	b := New("test", 1, time.Minute)
	b.RecordFailure()

	if b.Allow() {
		t.Fatal("should block in open state")
	}
}

func TestOpenToHalfOpenAfterCooldown(t *testing.T) {
	now := time.Now()
	b := New("test", 1, time.Second)
	b.now = func() time.Time { return now }

	b.RecordFailure()
	if b.State() != Open {
		t.Fatal("expected Open")
	}

	// Advance past cooldown.
	b.now = func() time.Time { return now.Add(2 * time.Second) }
	if !b.Allow() {
		t.Fatal("should allow after cooldown")
	}
	if b.State() != HalfOpen {
		t.Fatalf("expected HalfOpen, got %v", b.State())
	}
}

func TestHalfOpenToClosedOnRecovery(t *testing.T) {
	now := time.Now()
	b := New("test", 1, time.Second)
	b.now = func() time.Time { return now }

	b.RecordFailure()

	b.now = func() time.Time { return now.Add(2 * time.Second) }
	b.Allow() // triggers half-open

	b.RecordSuccess()
	b.RecordSuccess()

	if b.State() != Closed {
		t.Fatalf("expected Closed after recovery, got %v", b.State())
	}
}

func TestHalfOpenToOpenOnFailure(t *testing.T) {
	now := time.Now()
	b := New("test", 1, time.Second)
	b.now = func() time.Time { return now }

	b.RecordFailure()

	b.now = func() time.Time { return now.Add(2 * time.Second) }
	b.Allow() // triggers half-open

	b.RecordFailure()

	if b.State() != Open {
		t.Fatalf("expected Open, got %v", b.State())
	}
}

func TestSuccessResetsFails(t *testing.T) {
	b := New("test", 3, time.Minute)

	b.RecordFailure()
	b.RecordFailure()
	b.RecordSuccess() // resets counter
	b.RecordFailure()
	b.RecordFailure()

	if b.State() != Closed {
		t.Fatal("should still be closed after reset")
	}
}

func TestConcurrentAccess(t *testing.T) {
	b := New("test", 100, time.Second)
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Allow()
			b.RecordFailure()
			b.Allow()
			b.RecordSuccess()
		}()
	}

	wg.Wait()
	// No panic means the test passed.
	_ = b.State()
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{Closed, "closed"},
		{HalfOpen, "half-open"},
		{Open, "open"},
		{State(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
