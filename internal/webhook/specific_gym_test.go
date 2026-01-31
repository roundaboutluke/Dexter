package webhook

import (
	"os"
	"path/filepath"
	"testing"

	"poraclego/internal/config"
	"poraclego/internal/geofence"
)

func loadAreaSecurityConfig(t *testing.T, root string, enabled, strict bool) *config.Config {
	t.Helper()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	contents := []byte(`{"areaSecurity":{"enabled":false,"strictLocations":false}}`)
	if enabled && strict {
		contents = []byte(`{"areaSecurity":{"enabled":true,"strictLocations":true}}`)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.json"), contents, 0o644); err != nil {
		t.Fatalf("write default.json: %v", err)
	}
	cfg, err := config.Load(root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}

func TestPassesLocationFilterSpecificGymBypassesDistance(t *testing.T) {
	hook := &Hook{
		Type: "raid",
		Message: map[string]any{
			"gym_id":    "gymA",
			"latitude":  0.5,
			"longitude": 0.5,
		},
	}
	row := map[string]any{
		"gym_id":   "gymA",
		"distance": 1, // would normally fail for far-away users
	}
	location := locationInfo{Lat: 50, Lon: 50}

	if !passesLocationFilter(nil, nil, location, hook, row) {
		t.Fatalf("expected specific gym to bypass distance")
	}
}

func TestPassesLocationFilterSpecificGymStillRespectsStrictRestrictions(t *testing.T) {
	root := t.TempDir()
	cfg := loadAreaSecurityConfig(t, root, true, true)
	if enabled, _ := cfg.GetBool("areaSecurity.enabled"); !enabled {
		t.Fatalf("expected areaSecurity.enabled to be true")
	}
	if strict, _ := cfg.GetBool("areaSecurity.strictLocations"); !strict {
		t.Fatalf("expected areaSecurity.strictLocations to be true")
	}

	fences := &geofence.Store{Fences: []geofence.Fence{
		{
			Name: "A",
			Path: [][]float64{{-1, -1}, {-1, 1}, {1, 1}, {1, -1}},
		},
	}}

	hook := &Hook{
		Type: "raid",
		Message: map[string]any{
			"gym_id":    "gymA",
			"latitude":  0.5,
			"longitude": 0.5,
		},
	}
	row := map[string]any{
		"gym_id":   "gymA",
		"distance": 0,
	}
	// Restrict user to an unrelated area; strictLocations should reject.
	location := locationInfo{Restrictions: []string{"b"}}

	if passesLocationFilter(fences, cfg, location, hook, row) {
		t.Fatalf("expected strict area restriction to block specific gym match")
	}
}

func TestPassesLocationFilterSpecificGymMismatchDoesNotFallBackToDistance(t *testing.T) {
	hook := &Hook{
		Type: "raid",
		Message: map[string]any{
			"gym_id":    "gymB",
			"latitude":  0.5,
			"longitude": 0.5,
		},
	}
	row := map[string]any{
		"gym_id":   "gymA",
		"distance": 5000, // would match by distance if we incorrectly fell back
	}
	location := locationInfo{Lat: 0.5, Lon: 0.5}

	if passesLocationFilter(nil, nil, location, hook, row) {
		t.Fatalf("expected specific gym mismatch to fail regardless of distance")
	}
}
