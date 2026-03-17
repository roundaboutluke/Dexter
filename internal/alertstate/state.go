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
	Monsters     *MonsterIndex
	LoadedAt     time.Time
}

// MonsterIndex groups monster tracking rows for fast lookup by pokemon_id
// and PVP league, avoiding full linear scans during matching.
type MonsterIndex struct {
	// ByPokemonID maps pokemon_id to tracking rows. Key 0 holds catch-all entries.
	ByPokemonID map[int][]map[string]any
	// PVPSpecific maps pvp_ranking_league to tracking rows where pokemon_id != 0.
	PVPSpecific map[int][]map[string]any
	// PVPEverything maps pvp_ranking_league to tracking rows where pokemon_id == 0.
	PVPEverything map[int][]map[string]any
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
