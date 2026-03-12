package webhook

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (g *Geocoder) forwardPelias(query string) []GeoLocation {
	base, _ := g.cfg.GetString("geocoding.providerURL")
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil
	}
	values := url.Values{}
	values.Set("text", query)
	values.Set("size", "1")
	if key := g.peliasAPIKey(); key != "" {
		values.Set("api_key", key)
	}
	reqURL := base + "/v1/search?" + values.Encode()
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
				Locality string `json:"locality"`
				Country  string `json:"country"`
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
	city := strings.TrimSpace(payload.Features[0].Properties.Locality)
	country := strings.TrimSpace(payload.Features[0].Properties.Country)
	return []GeoLocation{{Latitude: lat, Longitude: lon, City: city, Country: country}}
}

func (g *Geocoder) reversePelias(lat, lon float64) string {
	result := g.reversePeliasDetails(lat, lon)
	if result == nil {
		return ""
	}
	return result.FormattedAddress
}

func (g *Geocoder) reversePeliasDetails(lat, lon float64) *ReverseResult {
	base, _ := g.cfg.GetString("geocoding.providerURL")
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	if base == "" {
		return nil
	}

	values := url.Values{}
	values.Set("point.lat", fmt.Sprintf("%f", lat))
	values.Set("point.lon", fmt.Sprintf("%f", lon))
	size := 1
	if v, ok := g.cfg.GetInt("geocoding.peliasResultSize"); ok {
		size = v
	}
	if size <= 0 {
		size = 1
	}
	if size > 10 {
		size = 10
	}
	values.Set("size", fmt.Sprintf("%d", size))
	if layers, _ := g.cfg.GetString("geocoding.peliasLayers"); strings.TrimSpace(layers) != "" {
		values.Set("layers", normalizeCSVLower(layers))
	}
	if country, _ := g.cfg.GetString("geocoding.peliasBoundaryCountry"); strings.TrimSpace(country) != "" {
		values.Set("boundary.country", strings.ToUpper(strings.TrimSpace(country)))
	}
	if key := g.peliasAPIKey(); key != "" {
		values.Set("api_key", key)
	}
	reqURL := base + "/v1/reverse?" + values.Encode()
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
			Properties struct {
				Layer         string `json:"layer"`
				Name          string `json:"name"`
				Label         string `json:"label"`
				Street        string `json:"street"`
				HouseNumber   string `json:"housenumber"`
				Locality      string `json:"locality"`
				LocalAdmin    string `json:"localadmin"`
				Region        string `json:"region"`
				Country       string `json:"country"`
				CountryCode   string `json:"country_a"`
				PostalCode    string `json:"postalcode"`
				Neighbourhood string `json:"neighbourhood"`
				Borough       string `json:"borough"`
			} `json:"properties"`
		} `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	if len(payload.Features) == 0 {
		return nil
	}

	preferred, _ := g.cfg.GetString("geocoding.peliasPreferredLayer")
	preferred = strings.ToLower(strings.TrimSpace(preferred))
	if preferred == "" {
		preferred = "address"
	}

	props := payload.Features[0].Properties
	venueName := ""
	for _, feature := range payload.Features {
		layer := strings.ToLower(strings.TrimSpace(feature.Properties.Layer))
		if venueName == "" && layer == "venue" && strings.TrimSpace(feature.Properties.Name) != "" {
			venueName = strings.TrimSpace(feature.Properties.Name)
		}
		if layer == preferred {
			props = feature.Properties
			break
		}
	}

	address := strings.TrimSpace(props.Label)
	if address == "" {
		parts := []string{}
		street := strings.TrimSpace(strings.TrimSpace(props.HouseNumber + " " + props.Street))
		if street != "" {
			parts = append(parts, street)
		}
		if v := strings.TrimSpace(props.Locality); v != "" {
			parts = append(parts, v)
		} else if v := strings.TrimSpace(props.LocalAdmin); v != "" {
			parts = append(parts, v)
		}
		if v := strings.TrimSpace(props.Region); v != "" {
			parts = append(parts, v)
		}
		if v := strings.TrimSpace(props.Country); v != "" {
			parts = append(parts, v)
		}
		address = strings.Join(parts, ", ")
	}

	city := strings.TrimSpace(props.Locality)
	if city == "" {
		city = strings.TrimSpace(props.LocalAdmin)
	}
	countryCode := strings.ToUpper(strings.TrimSpace(props.CountryCode))

	streetName := strings.TrimSpace(props.Street)
	streetNumber := strings.TrimSpace(props.HouseNumber)
	if strings.EqualFold(strings.TrimSpace(props.Layer), "street") && streetName == "" {
		streetName = strings.TrimSpace(props.Name)
		streetNumber = ""
	}

	return &ReverseResult{
		FormattedAddress: address,
		StreetName:       streetName,
		StreetNumber:     streetNumber,
		City:             city,
		Country:          strings.TrimSpace(props.Country),
		State:            strings.TrimSpace(props.Region),
		Zipcode:          strings.TrimSpace(props.PostalCode),
		CountryCode:      countryCode,
		Neighbourhood:    strings.TrimSpace(props.Neighbourhood),
		Suburb:           strings.TrimSpace(props.Borough),
		Town:             strings.TrimSpace(props.Locality),
		Village:          strings.TrimSpace(props.LocalAdmin),
		Shop:             venueName,
	}
}
