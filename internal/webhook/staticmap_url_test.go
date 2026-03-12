package webhook

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"poraclego/internal/config"
)

func loadTestConfigFromMap(tb testing.TB, cfg map[string]any) *config.Config {
	tb.Helper()
	root := tb.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "config"), 0o755); err != nil {
		tb.Fatalf("mkdir config: %v", err)
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		tb.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "config", "default.json"), raw, 0o644); err != nil {
		tb.Fatalf("write default.json: %v", err)
	}
	loaded, err := config.Load(root)
	if err != nil {
		tb.Fatalf("load config: %v", err)
	}
	return loaded
}

func TestStaticMapURL_ProviderNoneReturnsEmpty(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{
		"geocoding": map[string]any{
			"staticProvider": "none",
			"staticKey":      []any{"k"},
			"width":          400,
			"height":         200,
			"zoom":           15,
			"type":           "roadmap",
		},
	})
	p := &Processor{cfg: cfg}
	hook := &Hook{Type: "pokemon", Message: map[string]any{"latitude": 1.23, "longitude": 4.56}}
	if got := staticMapURL(p, hook, map[string]any{}); got != "" {
		t.Fatalf("expected empty static map for provider none, got %q", got)
	}
}

func TestStaticMapURL_ProviderUnknownReturnsEmpty(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{
		"geocoding": map[string]any{
			"staticProvider": "wat",
			"staticKey":      []any{"k"},
		},
	})
	p := &Processor{cfg: cfg}
	hook := &Hook{Type: "pokemon", Message: map[string]any{"latitude": 1.23, "longitude": 4.56}}
	if got := staticMapURL(p, hook, map[string]any{}); got != "" {
		t.Fatalf("expected empty static map for unknown provider, got %q", got)
	}
}

func TestStaticMapURL_ProviderGoogle(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{
		"geocoding": map[string]any{
			"staticProvider": "google",
			"staticKey":      []any{"k"},
			"width":          400,
			"height":         200,
			"zoom":           15,
			"type":           "roadmap",
		},
	})
	p := &Processor{cfg: cfg}
	hook := &Hook{Type: "pokemon", Message: map[string]any{"latitude": 1.23, "longitude": 4.56}}
	got := staticMapURL(p, hook, map[string]any{})
	if !strings.Contains(got, "maps.googleapis.com/maps/api/staticmap?") || !strings.Contains(got, "key=k") {
		t.Fatalf("expected google static map URL, got %q", got)
	}
}

func TestStaticMapURL_ProviderOSMIncludesDefaultMarker(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{
		"geocoding": map[string]any{
			"staticProvider": "osm",
			"staticKey":      []any{"k"},
			"width":          400,
			"height":         200,
			"zoom":           15,
		},
	})
	p := &Processor{cfg: cfg}
	hook := &Hook{Type: "pokemon", Message: map[string]any{"latitude": 1.23, "longitude": 4.56}}
	got := staticMapURL(p, hook, map[string]any{})
	if !strings.Contains(got, "www.mapquestapi.com/staticmap/v5/map?") || !strings.Contains(got, "defaultMarker=marker-md-3B5998-22407F") {
		t.Fatalf("expected mapquest static map URL with defaultMarker, got %q", got)
	}
}

func TestStaticMapURL_ProviderMapboxIncludesOverlayMarker(t *testing.T) {
	cfg := loadTestConfigFromMap(t, map[string]any{
		"geocoding": map[string]any{
			"staticProvider": "mapbox",
			"staticKey":      []any{"k"},
			"width":          400,
			"height":         200,
			"zoom":           15,
		},
	})
	p := &Processor{cfg: cfg}
	hook := &Hook{Type: "pokemon", Message: map[string]any{"latitude": 1.23, "longitude": 4.56}}
	got := staticMapURL(p, hook, map[string]any{})
	if !strings.Contains(got, "api.mapbox.com/styles/v1/mapbox/streets-v10/static/") ||
		!strings.Contains(got, "url-https%3A%2F%2Fi.imgur.com%2FMK4NUzI.png(") ||
		!strings.Contains(got, "access_token=k") {
		t.Fatalf("expected mapbox static map URL with overlay marker, got %q", got)
	}
}
