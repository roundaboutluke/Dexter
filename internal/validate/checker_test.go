package validate

import (
	"strings"
	"testing"

	"dexter/internal/config"
	"dexter/internal/geofence"
)

func collectWarnings(fn func(logger func(string, ...any))) []string {
	var warnings []string
	fn(func(format string, args ...any) {
		msg := format
		if len(args) > 0 {
			msg = strings.ReplaceAll(msg, "%s", "X") // simplified; just capture raw format
		}
		warnings = append(warnings, format)
	})
	return warnings
}

func TestCheckConfig_NilConfig(t *testing.T) {
	// Should not panic.
	CheckConfig(nil, func(string, ...any) {
		t.Error("unexpected warning for nil config")
	})
}

func TestCheckConfig_InvalidGeoProvider(t *testing.T) {
	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":       "badprovider",
			"staticProvider": "none",
		},
		"general": map[string]any{
			"roleCheckMode": "ignore",
		},
		"tracking": map[string]any{
			"everythingFlagPermissions": "allow-any",
		},
	})
	var warnings []string
	CheckConfig(cfg, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "geocoding/provider") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected geocoding provider warning, got: %v", warnings)
	}
}

func TestCheckConfig_ValidConfig(t *testing.T) {
	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":       "nominatim",
			"staticProvider": "osm",
		},
		"general": map[string]any{
			"roleCheckMode": "ignore",
		},
		"tracking": map[string]any{
			"everythingFlagPermissions": "allow-any",
		},
	})
	var warnings []string
	CheckConfig(cfg, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	if len(warnings) > 0 {
		t.Errorf("expected zero warnings for valid config, got: %v", warnings)
	}
}

func TestCheckConfig_DiscordEmptyGuilds(t *testing.T) {
	cfg := config.New(map[string]any{
		"discord": map[string]any{
			"enabled": true,
		},
		"geocoding": map[string]any{
			"provider":       "none",
			"staticProvider": "none",
		},
		"general": map[string]any{
			"roleCheckMode": "ignore",
		},
		"tracking": map[string]any{
			"everythingFlagPermissions": "allow-any",
		},
	})
	var warnings []string
	CheckConfig(cfg, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "reconciliation") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected discord guilds warning, got: %v", warnings)
	}
}

func TestCheckConfig_LegacyOption(t *testing.T) {
	cfg := config.New(map[string]any{
		"discord": map[string]any{
			"limitSec": float64(10),
		},
		"geocoding": map[string]any{
			"provider":       "none",
			"staticProvider": "none",
		},
		"general": map[string]any{
			"roleCheckMode": "ignore",
		},
		"tracking": map[string]any{
			"everythingFlagPermissions": "allow-any",
		},
	})
	var warnings []string
	CheckConfig(cfg, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "legacy") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected legacy warning, got: %v", warnings)
	}
}

func TestCheckGeofence_Empty(t *testing.T) {
	var warnings []string
	CheckGeofence(nil, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	if len(warnings) == 0 {
		t.Error("expected warning for empty geofence")
	}
}

func TestCheckGeofence_BlankName(t *testing.T) {
	fences := []geofence.Fence{
		{Name: "", Path: [][]float64{{0, 0}, {1, 1}, {0, 1}}},
	}
	var warnings []string
	CheckGeofence(fences, func(format string, args ...any) {
		warnings = append(warnings, format)
	})
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "blank name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected blank name warning, got: %v", warnings)
	}
}
