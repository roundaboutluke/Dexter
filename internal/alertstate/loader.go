package alertstate

import (
	"fmt"
	"time"

	"poraclego/internal/db"
	"poraclego/internal/geofence"
	"poraclego/internal/util"
)

var trackedTables = []string{
	"monsters",
	"raid",
	"egg",
	"quest",
	"invasion",
	"weather",
	"lures",
	"gym",
	"nests",
	"forts",
	"maxbattle",
}

// TrackedTables returns the alert-tracking tables included in the snapshot.
func TrackedTables() []string {
	out := make([]string, len(trackedTables))
	copy(out, trackedTables)
	return out
}

// Load builds a fresh snapshot from the database and current geofence store.
func Load(query *db.Query, fences *geofence.Store) (*Snapshot, error) {
	if query == nil {
		return nil, fmt.Errorf("alert snapshot missing query")
	}

	tables := make(map[string][]map[string]any, len(trackedTables))
	for _, table := range trackedTables {
		rows, err := query.SelectAllQuery(table, map[string]any{})
		if err != nil {
			return nil, fmt.Errorf("load %s: %w", table, err)
		}
		tables[table] = rows
	}

	humanRows, err := query.SelectAllQuery("humans", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("load humans: %w", err)
	}
	humans := make(map[string]map[string]any, len(humanRows))
	for _, row := range humanRows {
		id := getString(row["id"])
		if id == "" {
			continue
		}
		humans[id] = row
	}

	profileRows, err := query.SelectAllQuery("profiles", map[string]any{})
	if err != nil {
		return nil, fmt.Errorf("load profiles: %w", err)
	}
	profiles := make(map[string]map[string]any, len(profileRows))
	hasSchedules := make(map[string]bool, len(profileRows))
	for _, row := range profileRows {
		id := getString(row["id"])
		if id == "" {
			continue
		}
		profiles[profileKey(id, toInt(row["profile_no"], 1))] = row
		if len(getString(row["active_hours"])) > 5 {
			hasSchedules[id] = true
		}
	}

	return &Snapshot{
		Tables:       tables,
		Humans:       humans,
		Profiles:     profiles,
		HasSchedules: hasSchedules,
		Fences:       cloneFenceStore(fences),
		LoadedAt:     time.Now(),
	}, nil
}

func profileKey(id string, profileNo int) string {
	return fmt.Sprintf("%s:%d", id, profileNo)
}

func cloneFenceStore(store *geofence.Store) *geofence.Store {
	if store == nil {
		return &geofence.Store{Fences: []geofence.Fence{}}
	}
	cloned := make([]geofence.Fence, 0, len(store.Fences))
	for _, fence := range store.Fences {
		next := geofence.Fence{
			Name:        fence.Name,
			ID:          fence.ID,
			Color:       fence.Color,
			Group:       fence.Group,
			Description: fence.Description,
		}
		if fence.UserSelectable != nil {
			value := *fence.UserSelectable
			next.UserSelectable = &value
		}
		if fence.DisplayInMatch != nil {
			value := *fence.DisplayInMatch
			next.DisplayInMatch = &value
		}
		if len(fence.Path) > 0 {
			next.Path = clone2DFloats(fence.Path)
		}
		if len(fence.MultiPath) > 0 {
			next.MultiPath = make([][][]float64, 0, len(fence.MultiPath))
			for _, path := range fence.MultiPath {
				next.MultiPath = append(next.MultiPath, clone2DFloats(path))
			}
		}
		cloned = append(cloned, next)
	}
	return &geofence.Store{Fences: cloned}
}

func clone2DFloats(in [][]float64) [][]float64 {
	out := make([][]float64, 0, len(in))
	for _, row := range in {
		cp := make([]float64, len(row))
		copy(cp, row)
		out = append(out, cp)
	}
	return out
}

var getString = util.GetString
var toInt = util.ToInt
