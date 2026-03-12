package webhook

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func (g *Geocoder) forwardGoogle(query string) []GeoLocation {
	key := g.googleAPIKey()
	if key == "" {
		return nil
	}
	base := "https://maps.googleapis.com/maps/api/geocode/json"
	reqURL := fmt.Sprintf("%s?address=%s&key=%s", base, url.QueryEscape(query), url.QueryEscape(key))
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var payload struct {
		Results []struct {
			Geometry struct {
				Location struct {
					Lat float64 `json:"lat"`
					Lng float64 `json:"lng"`
				} `json:"location"`
			} `json:"geometry"`
			Components []struct {
				LongName string   `json:"long_name"`
				Types    []string `json:"types"`
			} `json:"address_components"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	if len(payload.Results) == 0 {
		return nil
	}
	result := payload.Results[0]
	city := ""
	country := ""
	for _, comp := range result.Components {
		for _, t := range comp.Types {
			switch t {
			case "locality":
				city = comp.LongName
			case "country":
				country = comp.LongName
			}
		}
	}
	return []GeoLocation{{Latitude: result.Geometry.Location.Lat, Longitude: result.Geometry.Location.Lng, City: city, Country: country}}
}

func (g *Geocoder) reverseGoogle(lat, lon float64) string {
	result := g.reverseGoogleDetails(lat, lon)
	if result == nil {
		return ""
	}
	return result.FormattedAddress
}

func (g *Geocoder) reverseGoogleDetails(lat, lon float64) *ReverseResult {
	key := g.googleAPIKey()
	if key == "" {
		return nil
	}
	base := "https://maps.googleapis.com/maps/api/geocode/json"
	reqURL := fmt.Sprintf("%s?latlng=%f,%f&key=%s", base, lat, lon, url.QueryEscape(key))
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var payload struct {
		Results []struct {
			FormattedAddress string `json:"formatted_address"`
			Components       []struct {
				LongName  string   `json:"long_name"`
				ShortName string   `json:"short_name"`
				Types     []string `json:"types"`
			} `json:"address_components"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	if len(payload.Results) == 0 {
		return nil
	}
	result := payload.Results[0]
	out := &ReverseResult{FormattedAddress: result.FormattedAddress}
	for _, comp := range result.Components {
		for _, t := range comp.Types {
			switch t {
			case "street_number":
				out.StreetNumber = comp.LongName
			case "route":
				out.StreetName = comp.LongName
			case "postal_code":
				out.Zipcode = comp.LongName
			case "locality":
				out.City = comp.LongName
			case "postal_town":
				if out.City == "" {
					out.City = comp.LongName
				}
				out.Town = comp.LongName
			case "administrative_area_level_1":
				out.State = comp.LongName
			case "country":
				out.Country = comp.LongName
				out.CountryCode = strings.ToUpper(comp.ShortName)
			case "neighborhood":
				out.Neighbourhood = comp.LongName
			case "sublocality", "sublocality_level_1":
				if out.Suburb == "" {
					out.Suburb = comp.LongName
				}
			}
		}
	}
	return out
}
