package webhook

import (
	"testing"

	"poraclego/internal/config"
)

func TestGeocodeCacheKeyScopesProviderAndURL(t *testing.T) {
	t.Parallel()

	cfgPelias := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":    "pelias",
			"providerURL": "http://pelias.test",
			"cacheDetail": 4,
		},
	})
	cfgNominatim := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":    "nominatim",
			"providerURL": "http://nominatim.test",
			"cacheDetail": 4,
		},
	})

	keyPelias := geocodeCacheKey(cfgPelias, 51.87506, -0.51162)
	keyNominatim := geocodeCacheKey(cfgNominatim, 51.87506, -0.51162)
	if keyPelias == "" || keyNominatim == "" {
		t.Fatalf("expected non-empty keys")
	}
	if keyPelias == keyNominatim {
		t.Fatalf("expected keys to differ across providers: %q", keyPelias)
	}

	cfgPelias2 := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":    "pelias",
			"providerURL": "http://pelias.other",
			"cacheDetail": 4,
		},
	})
	keyPelias2 := geocodeCacheKey(cfgPelias2, 51.87506, -0.51162)
	if keyPelias2 == keyPelias {
		t.Fatalf("expected keys to differ across providerURL: %q", keyPelias)
	}
}
