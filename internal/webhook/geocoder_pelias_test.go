package webhook

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"poraclego/internal/config"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestGeocoderPeliasReverseDetails(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/reverse" {
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
      "label": "10 Downing Street, London, England, United Kingdom",
      "street": "Downing Street",
      "housenumber": "10",
      "locality": "London",
      "region": "England",
      "country": "United Kingdom",
      "country_a": "GB",
      "postalcode": "SW1A 2AA",
      "neighbourhood": "Westminster",
      "borough": "City of Westminster",
      "localadmin": "Westminster"
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
			"provider":    "pelias",
			"providerURL": "http://pelias.test",
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(51.5034, -0.1276)
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.FormattedAddress == "" {
		t.Fatalf("expected formatted address")
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
}

func TestGeocoderPeliasReverseDetailsPreferredStreet(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/reverse" {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}
			q := r.URL.Query()
			if q.Get("layers") != "address,street" {
				t.Fatalf("expected layers query param, got %q", q.Get("layers"))
			}
			if q.Get("boundary.country") != "GB" {
				t.Fatalf("expected boundary.country query param, got %q", q.Get("boundary.country"))
			}
			body := `{
  "features": [
    {
      "properties": {
        "layer": "address",
        "label": "65 Lowther Road, Dunstable, England, United Kingdom",
        "street": "Lowther Road",
        "housenumber": "65",
        "locality": "Dunstable",
        "region": "England",
        "country": "United Kingdom",
        "country_a": "GB",
        "postalcode": "LU6 3XX"
      }
    },
    {
      "properties": {
        "layer": "street",
        "name": "Miletree Crescent",
        "label": "Miletree Crescent, Dunstable, England, United Kingdom",
        "locality": "Dunstable",
        "region": "England",
        "country": "United Kingdom",
        "country_a": "GB"
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
			"provider":              "pelias",
			"providerURL":           "http://pelias.test",
			"peliasLayers":          "address,street",
			"peliasPreferredLayer":  "street",
			"peliasBoundaryCountry": "GB",
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(51.87506, -0.51162)
	if result == nil {
		t.Fatalf("expected result")
	}
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

func TestGeocoderPeliasReverseDetailsCapturesVenueName(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/reverse" {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}
			if r.URL.Query().Get("size") != "5" {
				t.Fatalf("expected size=5, got %q", r.URL.Query().Get("size"))
			}
			body := `{
  "features": [
    {
      "properties": {
        "layer": "venue",
        "name": "Angel Wines/Convenience Store",
        "label": "Angel Wines/Convenience Store, Dunstable, England, United Kingdom",
        "locality": "Dunstable",
        "region": "England",
        "country": "United Kingdom",
        "country_a": "GB"
      }
    },
    {
      "properties": {
        "layer": "address",
        "label": "53 London Road, Dunstable, England, United Kingdom",
        "street": "London Road",
        "housenumber": "53",
        "locality": "Dunstable",
        "region": "England",
        "country": "United Kingdom",
        "country_a": "GB",
        "postalcode": "LU6 3DH"
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
			"provider":             "pelias",
			"providerURL":          "http://pelias.test",
			"peliasLayers":         "venue,address,street",
			"peliasPreferredLayer": "address",
			"peliasResultSize":     5,
		},
	})
	g := NewGeocoder(cfg)
	g.client = client

	result := g.ReverseDetails(51.878058, -0.508682)
	if result == nil {
		t.Fatalf("expected result")
	}
	if result.StreetName != "London Road" {
		t.Fatalf("street name mismatch: %q", result.StreetName)
	}
	if result.StreetNumber != "53" {
		t.Fatalf("street number mismatch: %q", result.StreetNumber)
	}
	if result.Shop != "Angel Wines/Convenience Store" {
		t.Fatalf("shop mismatch: %q", result.Shop)
	}
}

func TestGeocoderPeliasForward(t *testing.T) {
	t.Parallel()

	client := &http.Client{
		Transport: roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v1/search" {
				return &http.Response{
					StatusCode: 404,
					Body:       io.NopCloser(strings.NewReader("")),
					Header:     make(http.Header),
					Request:    r,
				}, nil
			}
			body := `{
  "features": [{
    "geometry": {"coordinates": [-0.1276, 51.5034]},
    "properties": {"locality": "London", "country": "United Kingdom"}
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
			"provider":    "pelias",
			"providerURL": "http://pelias.test",
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
