package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State int

const (
	Closed   State = 0
	HalfOpen State = 1
	Open     State = 2
)

func (s State) String() string {
	switch s {
	case Closed:
		return "closed"
	case HalfOpen:
		return "half-open"
	case Open:
		return "open"
	default:
		return "unknown"
	}
}

// Breaker is a lightweight circuit breaker that tracks consecutive failures
// and temporarily blocks calls when a failure threshold is reached.
type Breaker struct {
	mu                sync.Mutex
	name              string
	state             State
	consecutiveFails  int
	failThreshold     int
	recoveryThreshold int
	halfOpenSuccesses int
	openUntil         time.Time
	cooldown          time.Duration
	onStateChange     func(name string, from, to State)
	now               func() time.Time // for testing
}

// New creates a circuit breaker.
// failThreshold is the number of consecutive failures before opening.
// cooldown is how long the breaker stays open before transitioning to half-open.
func New(name string, failThreshold int, cooldown time.Duration) *Breaker {
	if failThreshold < 1 {
		failThreshold = 5
	}
	if cooldown <= 0 {
		cooldown = 30 * time.Second
	}
	return &Breaker{
		name:              name,
		state:             Closed,
		failThreshold:     failThreshold,
		recoveryThreshold: 2,
		cooldown:          cooldown,
		now:               time.Now,
	}
}

// SetOnStateChange registers a callback that fires on state transitions.
func (b *Breaker) SetOnStateChange(fn func(name string, from, to State)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onStateChange = fn
}

// Allow returns true if the call should proceed.
// When the breaker is open, it returns false until the cooldown expires.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		return true
	case Open:
		if b.now().After(b.openUntil) {
			b.transition(HalfOpen)
			return true
		}
		return false
	case HalfOpen:
		return true
	default:
		return true
	}
}

// RecordSuccess records a successful call.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		b.consecutiveFails = 0
	case HalfOpen:
		b.halfOpenSuccesses++
		if b.halfOpenSuccesses >= b.recoveryThreshold {
			b.halfOpenSuccesses = 0
			b.consecutiveFails = 0
			b.transition(Closed)
		}
	}
}

// RecordFailure records a failed call.
func (b *Breaker) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case Closed:
		b.consecutiveFails++
		if b.consecutiveFails >= b.failThreshold {
			b.openUntil = b.now().Add(b.cooldown)
			b.transition(Open)
		}
	case HalfOpen:
		b.halfOpenSuccesses = 0
		b.openUntil = b.now().Add(b.cooldown)
		b.transition(Open)
	}
}

// State returns the current breaker state.
func (b *Breaker) State() State {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Name returns the breaker name.
func (b *Breaker) Name() string {
	return b.name
}

func (b *Breaker) transition(to State) {
	from := b.state
	b.state = to
	if b.onStateChange != nil && from != to {
		b.onStateChange(b.name, from, to)
	}
}
