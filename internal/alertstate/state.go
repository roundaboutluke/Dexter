package alertstate

import (
	"sync"
	"time"

	"poraclego/internal/geofence"
)

// Snapshot holds an immutable in-memory copy of alert-relevant state.
type Snapshot struct {
	Tables       map[string][]map[string]any
	Humans       map[string]map[string]any
	Profiles     map[string]map[string]any
	HasSchedules map[string]bool
	Fences       *geofence.Store
	LoadedAt     time.Time
}

// Rows returns the tracking rows for a table.
func (s *Snapshot) Rows(table string) []map[string]any {
	if s == nil || table == "" || s.Tables == nil {
		return nil
	}
	return s.Tables[table]
}

// Manager stores the current alert snapshot and swaps it atomically.
type Manager struct {
	mu       sync.RWMutex
	snapshot *Snapshot
}

// NewManager constructs an empty snapshot manager.
func NewManager() *Manager {
	return &Manager{}
}

// Get returns the current snapshot.
func (m *Manager) Get() *Snapshot {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.snapshot
}

// Set replaces the current snapshot atomically.
func (m *Manager) Set(snapshot *Snapshot) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.snapshot = snapshot
	m.mu.Unlock()
}
