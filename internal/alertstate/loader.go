package alertstate

import (
	"fmt"
	"time"

	"dexter/internal/db"
	"dexter/internal/geofence"
	"dexter/internal/util"
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
		profiles[ProfileKey(id, toInt(row["profile_no"], 1))] = row
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
		Monsters:     buildMonsterIndex(tables["monsters"]),
		LoadedAt:     time.Now(),
	}, nil
}

// buildMonsterIndex groups monster tracking rows by pokemon_id and PVP league.
func buildMonsterIndex(rows []map[string]any) *MonsterIndex {
	if len(rows) == 0 {
		return nil
	}
	idx := &MonsterIndex{
		ByPokemonID:   make(map[int][]map[string]any),
		PVPSpecific:   make(map[int][]map[string]any),
		PVPEverything: make(map[int][]map[string]any),
	}
	for _, row := range rows {
		pokemonID := toInt(row["pokemon_id"], 0)
		league := toInt(row["pvp_ranking_league"], 0)

		if league != 0 {
			if pokemonID != 0 {
				idx.PVPSpecific[league] = append(idx.PVPSpecific[league], row)
			} else {
				idx.PVPEverything[league] = append(idx.PVPEverything[league], row)
			}
		} else {
			idx.ByPokemonID[pokemonID] = append(idx.ByPokemonID[pokemonID], row)
		}
	}
	return idx
}

// ProfileKey builds the lookup key for a human profile.
func ProfileKey(id string, profileNo int) string {
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
	result := &geofence.Store{Fences: cloned}
	result.BuildIndex()
	return result
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
