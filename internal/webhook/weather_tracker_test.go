package webhook

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"poraclego/internal/config"
)

func loadTestConfig(t *testing.T, root string, showAltered bool) *config.Config {
	t.Helper()
	configDir := filepath.Join(root, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config: %v", err)
	}
	contents := []byte(`{"weather":{"showAlteredPokemon":false}}`)
	if showAltered {
		contents = []byte(`{"weather":{"showAlteredPokemon":true}}`)
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

func TestWeatherTrackerTrackCareDoesNotStorePokemonsWhenDisabled(t *testing.T) {
	root := t.TempDir()
	cfg := loadTestConfig(t, root, false)
	tracker := NewWeatherTracker(cfg, root)

	tracker.TrackCare("cell1", alertTarget{ID: "u1", Name: "User", Type: "discord:user"}, time.Now().Add(time.Hour).Unix(), false, "", &caredPokemon{PokemonID: 1})

	entry := tracker.CareEntry("cell1", "u1")
	if entry == nil {
		t.Fatalf("expected care entry")
	}
	if len(entry.CaredPokemons) != 0 {
		t.Fatalf("caredPokemons=%d, want 0", len(entry.CaredPokemons))
	}
}

func TestWeatherTrackerTrackCareStoresPokemonsWhenEnabledAndClearsOnDisable(t *testing.T) {
	root := t.TempDir()
	cfgOn := loadTestConfig(t, root, true)
	tracker := NewWeatherTracker(cfgOn, root)

	tracker.TrackCare("cell1", alertTarget{ID: "u1", Name: "User", Type: "discord:user"}, time.Now().Add(time.Hour).Unix(), false, "", &caredPokemon{PokemonID: 1})
	entry := tracker.CareEntry("cell1", "u1")
	if entry == nil || len(entry.CaredPokemons) != 1 {
		t.Fatalf("caredPokemons=%d, want 1", len(entry.CaredPokemons))
	}

	cfgOff := loadTestConfig(t, t.TempDir(), false)
	tracker.cfg = cfgOff
	tracker.TrackCare("cell1", alertTarget{ID: "u1", Name: "User", Type: "discord:user"}, time.Now().Add(time.Hour).Unix(), false, "", &caredPokemon{PokemonID: 2})
	entry = tracker.CareEntry("cell1", "u1")
	if entry == nil {
		t.Fatalf("expected care entry after toggle")
	}
	if len(entry.CaredPokemons) != 0 {
		t.Fatalf("caredPokemons=%d, want 0 after disable", len(entry.CaredPokemons))
	}
}
