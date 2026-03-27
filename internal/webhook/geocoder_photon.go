package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (g *Geocoder) forwardPhoton(query string) []GeoLocation {
	base, _ := g.cfg.GetString("geocoding.providerURL")
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil
	}
	values := url.Values{}
	values.Set("q", query)
	values.Set("limit", "1")
	if lang, _ := g.cfg.GetString("geocoding.photonLang"); strings.TrimSpace(lang) != "" {
		values.Set("lang", strings.TrimSpace(lang))
	}
	reqURL := base + "/api?" + values.Encode()
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var payload struct {
		Features []struct {
			Geometry struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
			Properties struct {
				City    string `json:"city"`
				Country string `json:"country"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	if len(payload.Features) == 0 {
		return nil
	}
	coords := payload.Features[0].Geometry.Coordinates
	if len(coords) < 2 {
		return nil
	}
	lon := coords[0]
	lat := coords[1]
	city := strings.TrimSpace(payload.Features[0].Properties.City)
	country := strings.TrimSpace(payload.Features[0].Properties.Country)
	return []GeoLocation{{Latitude: lat, Longitude: lon, City: city, Country: country}}
}

func (g *Geocoder) reversePhoton(lat, lon float64) string {
	result := g.reversePhotonDetails(lat, lon)
	if result == nil {
		return ""
	}
	return result.FormattedAddress
}

func (g *Geocoder) reversePhotonDetails(lat, lon float64) *ReverseResult {
	base, _ := g.cfg.GetString("geocoding.providerURL")
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil
	}

	values := url.Values{}
	values.Set("lat", fmt.Sprintf("%f", lat))
	values.Set("lon", fmt.Sprintf("%f", lon))
	size := 1
	if v, ok := g.cfg.GetInt("geocoding.photonResultSize"); ok {
		size = v
	}
	if size <= 0 {
		size = 1
	}
	if size > 10 {
		size = 10
	}
	values.Set("limit", fmt.Sprintf("%d", size))
	if layers, _ := g.cfg.GetString("geocoding.photonLayers"); strings.TrimSpace(layers) != "" {
		for _, layer := range strings.Split(normalizeCSVLower(layers), ",") {
			values.Add("layer", layer)
		}
	}
	if lang, _ := g.cfg.GetString("geocoding.photonLang"); strings.TrimSpace(lang) != "" {
		values.Set("lang", strings.TrimSpace(lang))
	}

	reqURL := base + "/reverse?" + values.Encode()
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}

	var payload struct {
		Features []photonFeature `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	if len(payload.Features) == 0 {
		return nil
	}

	preferred, _ := g.cfg.GetString("geocoding.photonPreferredLayer")
	preferred = strings.ToLower(strings.TrimSpace(preferred))

	// Two-pass scan: first collect venue/shop name and preferred-layer
	// address from across all returned features, then combine them.
	// This lets us return both "Angel Wines" (shop) AND "London Road"
	// (street) from a single reverse call — something Pelias can't do.
	props := payload.Features[0].Properties
	shopName := ""
	preferredFound := false
	for _, feature := range payload.Features {
		if shopName == "" && isPhotonVenue(feature.Properties.OsmKey) && strings.TrimSpace(feature.Properties.Name) != "" {
			shopName = strings.TrimSpace(feature.Properties.Name)
		}
		if !preferredFound && preferred != "" {
			layer := photonLayerFromOSM(feature.Properties.OsmKey, feature.Properties.OsmValue)
			if layer == preferred {
				props = feature.Properties
				preferredFound = true
			}
		}
	}

	// Build formatted address from structured fields.
	addrParts := []string{}
	street := strings.TrimSpace(props.HouseNumber + " " + props.Street)
	if street != "" {
		addrParts = append(addrParts, street)
	}
	if v := strings.TrimSpace(props.City); v != "" {
		addrParts = append(addrParts, v)
	}
	if v := strings.TrimSpace(props.State); v != "" {
		addrParts = append(addrParts, v)
	}
	if v := strings.TrimSpace(props.Country); v != "" {
		addrParts = append(addrParts, v)
	}
	address := strings.Join(addrParts, ", ")

	streetName := strings.TrimSpace(props.Street)
	streetNumber := strings.TrimSpace(props.HouseNumber)
	if streetName == "" && strings.TrimSpace(props.Name) != "" && !isPhotonVenue(props.OsmKey) {
		streetName = strings.TrimSpace(props.Name)
		streetNumber = ""
	}

	return &ReverseResult{
		FormattedAddress: address,
		StreetName:       streetName,
		StreetNumber:     streetNumber,
		City:             strings.TrimSpace(props.City),
		Country:          strings.TrimSpace(props.Country),
		State:            strings.TrimSpace(props.State),
		Zipcode:          strings.TrimSpace(props.Postcode),
		CountryCode:      strings.ToUpper(strings.TrimSpace(props.CountryCode)),
		Neighbourhood:    strings.TrimSpace(props.District),
		Suburb:           strings.TrimSpace(props.County),
		Town:             strings.TrimSpace(props.City),
		Village:          "",
		Shop:             shopName,
	}
}

type photonFeature struct {
	Properties struct {
		Name        string `json:"name"`
		Street      string `json:"street"`
		HouseNumber string `json:"housenumber"`
		City        string `json:"city"`
		District    string `json:"district"`
		County      string `json:"county"`
		State       string `json:"state"`
		Country     string `json:"country"`
		CountryCode string `json:"countrycode"`
		Postcode    string `json:"postcode"`
		OsmKey      string `json:"osm_key"`
		OsmValue    string `json:"osm_value"`
	} `json:"properties"`
}

// isPhotonVenue returns true if the OSM key indicates a venue/shop.
func isPhotonVenue(osmKey string) bool {
	switch strings.ToLower(osmKey) {
	case "shop", "amenity", "tourism", "leisure":
		return true
	}
	return false
}

// photonLayerFromOSM approximates a Pelias-style layer name from OSM key/value.
// Photon doesn't return a "layer" field, so we infer from osm_key + osm_value.
func photonLayerFromOSM(osmKey, osmValue string) string {
	key := strings.ToLower(osmKey)
	value := strings.ToLower(osmValue)
	if isPhotonVenue(key) {
		return "venue"
	}
	if key == "place" {
		switch value {
		case "house":
			return "house"
		case "city", "town", "village", "hamlet":
			return "city"
		case "county":
			return "county"
		case "state":
			return "state"
		case "country":
			return "country"
		case "locality":
			return "locality"
		case "suburb", "neighbourhood", "quarter":
			return "district"
		}
	}
	if key == "highway" || key == "building" {
		return "street"
	}
	if key == "boundary" && value == "administrative" {
		return "state"
	}
	return "other"
}
