package webhook

import (
	"sync"
	"time"
)

// TTLCache stores keys with optional expiration.
type TTLCache struct {
	mu    sync.RWMutex
	items map[string]time.Time
}

// NewTTLCache constructs an empty TTL cache.
func NewTTLCache() *TTLCache {
	return &TTLCache{items: map[string]time.Time{}}
}

// Get returns true if the key exists and has not expired.
func (c *TTLCache) Get(key string) bool {
	// Fast path: read lock for cache misses (the common case).
	c.mu.RLock()
	expiry, ok := c.items[key]
	if !ok {
		c.mu.RUnlock()
		return false
	}
	if expiry.IsZero() || !time.Now().After(expiry) {
		c.mu.RUnlock()
		return true
	}
	c.mu.RUnlock()
	// Slow path: upgrade to write lock to delete expired entry.
	c.mu.Lock()
	// Re-check under write lock in case another goroutine already cleaned it up.
	if expiry, ok := c.items[key]; ok && !expiry.IsZero() && time.Now().After(expiry) {
		delete(c.items, key)
	}
	c.mu.Unlock()
	return false
}

// Set stores a key with an optional TTL. A non-positive ttl means no expiry.
func (c *TTLCache) Set(key string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttl <= 0 {
		c.items[key] = time.Time{}
		return
	}
	c.items[key] = time.Now().Add(ttl)
}

// PruneExpired removes expired entries and returns the number removed.
// Note: keys with zero expiry are never pruned.
func (c *TTLCache) PruneExpired(now time.Time) int {
	if c == nil {
		return 0
	}
	if now.IsZero() {
		now = time.Now()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	for key, expiry := range c.items {
		if expiry.IsZero() {
			continue
		}
		if now.After(expiry) {
			delete(c.items, key)
			removed++
		}
	}
	return removed
}

func (c *TTLCache) Len() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// GymState stores last seen gym details for dedupe.
type GymState struct {
	TeamID         int       `json:"team_id"`
	SlotsAvailable int       `json:"slots_available"`
	LastOwnerID    int       `json:"last_owner_id"`
	InBattle       bool      `json:"in_battle"`
	LastSeen       time.Time `json:"last_seen"`
}

// GymCache stores gym states by id.
type GymCache struct {
	mu    sync.Mutex
	items map[string]GymState
}

// NewGymCache constructs an empty gym cache.
func NewGymCache() *GymCache {
	return &GymCache{items: map[string]GymState{}}
}

// Get returns the cached gym state if present.
func (c *GymCache) Get(id string) *GymState {
	c.mu.Lock()
	defer c.mu.Unlock()
	state, ok := c.items[id]
	if !ok {
		return nil
	}
	copy := state
	return &copy
}

// Set stores the gym state with a last-seen timestamp.
func (c *GymCache) Set(id string, state GymState) {
	c.mu.Lock()
	state.LastSeen = time.Now()
	c.items[id] = state
	c.mu.Unlock()
}

// Load replaces the gym cache contents.
func (c *GymCache) Load(items map[string]GymState) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.items = items
	c.mu.Unlock()
}

// Snapshot returns a copy of the gym cache.
func (c *GymCache) Snapshot() map[string]GymState {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]GymState, len(c.items))
	for key, value := range c.items {
		out[key] = value
	}
	return out
}

// PruneStale removes gym entries not seen since the cutoff time.
func (c *GymCache) PruneStale(cutoff time.Time) int {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	removed := 0
	for key, state := range c.items {
		if state.LastSeen.Before(cutoff) {
			delete(c.items, key)
			removed++
		}
	}
	return removed
}
