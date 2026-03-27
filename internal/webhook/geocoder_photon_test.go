package webhook

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"dexter/internal/config"
)

func TestGeocoderPhotonReverseDetails(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/reverse" {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}
			body := `{
  "features": [{
    "properties": {
      "name": "10 Downing Street",
      "street": "Downing Street",
      "housenumber": "10",
      "city": "London",
      "state": "England",
      "country": "United Kingdom",
      "countrycode": "gb",
      "postcode": "SW1A 2AA",
      "district": "Westminster",
      "county": "City of Westminster",
      "osm_key": "office",
      "osm_value": "government"
    }
  }]
}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":    "photon",
			"providerURL": "http://photon.test",
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(51.5034, -0.1276)
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.StreetName != "Downing Street" {
		t.Fatalf("street name mismatch: %q", result.StreetName)
	}
	if result.StreetNumber != "10" {
		t.Fatalf("street number mismatch: %q", result.StreetNumber)
	}
	if result.City != "London" {
		t.Fatalf("city mismatch: %q", result.City)
	}
	if result.CountryCode != "GB" {
		t.Fatalf("country code mismatch: %q", result.CountryCode)
	}
	if result.State != "England" {
		t.Fatalf("state mismatch: %q", result.State)
	}
	if result.Zipcode != "SW1A 2AA" {
		t.Fatalf("zipcode mismatch: %q", result.Zipcode)
	}
	if result.Neighbourhood != "Westminster" {
		t.Fatalf("neighbourhood mismatch: %q", result.Neighbourhood)
	}
	if result.Suburb != "City of Westminster" {
		t.Fatalf("suburb mismatch: %q", result.Suburb)
	}
	if result.Shop != "" {
		t.Fatalf("expected empty shop, got %q", result.Shop)
	}
	if result.FormattedAddress != "10 Downing Street, London, England, United Kingdom" {
		t.Fatalf("formatted address mismatch: %q", result.FormattedAddress)
	}
}

func TestGeocoderPhotonReverseDetailsPreferredLayer(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/reverse" {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}
			q := r.URL.Query()
			if q.Get("limit") != "5" {
				t.Fatalf("expected limit=5, got %q", q.Get("limit"))
			}
			if q.Get("lang") != "en" {
				t.Fatalf("expected lang=en, got %q", q.Get("lang"))
			}
			layers := q["layer"]
			if len(layers) != 2 || layers[0] != "house" || layers[1] != "street" {
				t.Fatalf("expected layer=house&layer=street, got %v", layers)
			}
			body := `{
  "features": [
    {
      "properties": {
        "name": "Angel Wines",
        "street": "Lowther Road",
        "housenumber": "65",
        "city": "Dunstable",
        "state": "England",
        "country": "United Kingdom",
        "countrycode": "gb",
        "postcode": "LU6 3XX",
        "osm_key": "shop",
        "osm_value": "convenience"
      }
    },
    {
      "properties": {
        "name": "Miletree Crescent",
        "city": "Dunstable",
        "state": "England",
        "country": "United Kingdom",
        "countrycode": "gb",
        "osm_key": "highway",
        "osm_value": "residential"
      }
    }
  ]
}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":              "photon",
			"providerURL":           "http://photon.test",
			"photonLayers":          "house,street",
			"photonPreferredLayer":  "street",
			"photonResultSize":      5,
			"photonLang":            "en",
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(51.87506, -0.51162)
	if result == nil {
		t.Fatalf("expected result")
	}
	// Preferred layer is "street"; second feature has osm_key=highway which maps to "street".
	if result.StreetName != "Miletree Crescent" {
		t.Fatalf("street name mismatch: %q", result.StreetName)
	}
	if result.StreetNumber != "" {
		t.Fatalf("expected empty street number, got %q", result.StreetNumber)
	}
	if result.CountryCode != "GB" {
		t.Fatalf("country code mismatch: %q", result.CountryCode)
	}
}

func TestGeocoderPhotonReverseDetailsVenue(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			body := `{
  "features": [
    {
      "properties": {
        "name": "Angel Wines",
        "street": "London Road",
        "housenumber": "53",
        "city": "Dunstable",
        "state": "England",
        "country": "United Kingdom",
        "countrycode": "gb",
        "postcode": "LU6 3DH",
        "osm_key": "shop",
        "osm_value": "convenience"
      }
    },
    {
      "properties": {
        "name": "53 London Road",
        "street": "London Road",
        "housenumber": "53",
        "city": "Dunstable",
        "state": "England",
        "country": "United Kingdom",
        "countrycode": "gb",
        "postcode": "LU6 3DH",
        "osm_key": "building",
        "osm_value": "yes"
      }
    }
  ]
}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":             "photon",
			"providerURL":          "http://photon.test",
			"photonPreferredLayer": "street",
			"photonResultSize":     5,
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(51.878058, -0.508682)
	if result == nil {
		t.Fatalf("expected result")
	}
	// Venue name captured from first feature with shop osm_key.
	if result.Shop != "Angel Wines" {
		t.Fatalf("shop mismatch: %q", result.Shop)
	}
	// Preferred layer "street" matches second feature (osm_key=building → street).
	if result.StreetName != "London Road" {
		t.Fatalf("street name mismatch: %q", result.StreetName)
	}
	if result.StreetNumber != "53" {
		t.Fatalf("street number mismatch: %q", result.StreetNumber)
	}
}

// TestGeocoderPhotonReverseDetailsVenueAfterStreet verifies that the shop name
// is captured even when the venue feature appears AFTER the preferred street
// feature — proving we scan all results rather than breaking early.
func TestGeocoderPhotonReverseDetailsVenueAfterStreet(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			body := `{
  "features": [
    {
      "properties": {
        "name": "Miletree Crescent",
        "city": "Dunstable",
        "state": "England",
        "country": "United Kingdom",
        "countrycode": "gb",
        "osm_key": "highway",
        "osm_value": "residential"
      }
    },
    {
      "properties": {
        "name": "Angel Wines",
        "street": "London Road",
        "housenumber": "53",
        "city": "Dunstable",
        "state": "England",
        "country": "United Kingdom",
        "countrycode": "gb",
        "postcode": "LU6 3DH",
        "osm_key": "shop",
        "osm_value": "convenience"
      }
    }
  ]
}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":             "photon",
			"providerURL":          "http://photon.test",
			"photonPreferredLayer": "street",
			"photonResultSize":     5,
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(51.878058, -0.508682)
	if result == nil {
		t.Fatalf("expected result")
	}
	// Street feature (highway → street) is first and matches preferred layer.
	if result.StreetName != "Miletree Crescent" {
		t.Fatalf("street name mismatch: %q", result.StreetName)
	}
	// Venue feature comes second but shop name should still be captured.
	if result.Shop != "Angel Wines" {
		t.Fatalf("shop mismatch: expected Angel Wines, got %q", result.Shop)
	}
}

func TestGeocoderPhotonReverseDetailsEmpty(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(`{"features":[]}`)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":    "photon",
			"providerURL": "http://photon.test",
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(0, 0)
	if result != nil {
		t.Fatalf("expected nil result for empty features")
	}
}

func TestGeocoderPhotonForward(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/api" {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}
			if q := r.URL.Query(); q.Get("lang") != "de" {
				t.Fatalf("expected lang=de, got %q", q.Get("lang"))
			}
			body := `{
  "features": [{
    "geometry": {"type": "Point", "coordinates": [-0.1276, 51.5034]},
    "properties": {"city": "London", "country": "United Kingdom"}
  }]
}`
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader(body)),
				Header:     make(http.Header),
				Request:    r,
			}, nil
		}),
	}

	cfg := config.New(map[string]any{
		"geocoding": map[string]any{
			"provider":    "photon",
			"providerURL": "http://photon.test",
			"photonLang":  "de",
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	results := g.Forward("10 Downing Street")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Latitude != 51.5034 || results[0].Longitude != -0.1276 {
		t.Fatalf("unexpected coordinates: %v,%v", results[0].Latitude, results[0].Longitude)
	}
	if results[0].City != "London" {
		t.Fatalf("unexpected city: %q", results[0].City)
	}
	if results[0].Country != "United Kingdom" {
		t.Fatalf("unexpected country: %q", results[0].Country)
	}
}
