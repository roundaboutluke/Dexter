package webhook

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

func (g *Geocoder) reverseNominatim(lat, lon float64) string {
	base, _ := g.cfg.GetString("geocoding.providerURL")
	if base == "" {
		return ""
	}
	endpoint := strings.TrimRight(base, "/") + "/reverse"
	reqURL := fmt.Sprintf("%s?format=json&lat=%f&lon=%f", endpoint, lat, lon)
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var payload struct {
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return ""
	}
	return payload.DisplayName
}

func (g *Geocoder) reverseNominatimDetails(lat, lon float64) *ReverseResult {
	base, _ := g.cfg.GetString("geocoding.providerURL")
	if base == "" {
		return nil
	}
	endpoint := strings.TrimRight(base, "/") + "/reverse"
	reqURL := fmt.Sprintf("%s?format=json&addressdetails=1&lat=%f&lon=%f", endpoint, lat, lon)
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var payload struct {
		DisplayName string `json:"display_name"`
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		Address     struct {
			Road          string `json:"road"`
			Quarter       string `json:"quarter"`
			Cycleway      string `json:"cycleway"`
			HouseNumber   string `json:"house_number"`
			City          string `json:"city"`
			Town          string `json:"town"`
			Village       string `json:"village"`
			Hamlet        string `json:"hamlet"`
			State         string `json:"state"`
			Postcode      string `json:"postcode"`
			Country       string `json:"country"`
			CountryCode   string `json:"country_code"`
			Neighbourhood string `json:"neighbourhood"`
			Suburb        string `json:"suburb"`
			Shop          string `json:"shop"`
		} `json:"address"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	streetName := payload.Address.Road
	if streetName == "" {
		streetName = payload.Address.Quarter
	}
	if streetName == "" {
		streetName = payload.Address.Cycleway
	}
	city := payload.Address.City
	if city == "" {
		city = payload.Address.Town
	}
	if city == "" {
		city = payload.Address.Village
	}
	if city == "" {
		city = payload.Address.Hamlet
	}
	countryCode := strings.ToUpper(payload.Address.CountryCode)
	return &ReverseResult{
		FormattedAddress: payload.DisplayName,
		StreetName:       streetName,
		StreetNumber:     payload.Address.HouseNumber,
		City:             city,
		Country:          payload.Address.Country,
		State:            payload.Address.State,
		Zipcode:          payload.Address.Postcode,
		CountryCode:      countryCode,
		Neighbourhood:    payload.Address.Neighbourhood,
		Suburb:           payload.Address.Suburb,
		Town:             payload.Address.Town,
		Village:          payload.Address.Village,
		Shop:             payload.Address.Shop,
	}
}

func (g *Geocoder) forwardNominatim(query string) []GeoLocation {
	base, _ := g.cfg.GetString("geocoding.providerURL")
	if base == "" {
		base = "https://nominatim.openstreetmap.org"
	}
	endpoint := strings.TrimRight(base, "/") + "/search"
	reqURL := fmt.Sprintf("%s?format=json&limit=1&q=%s", endpoint, url.QueryEscape(query))
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var payload []struct {
		Lat     string `json:"lat"`
		Lon     string `json:"lon"`
		Display string `json:"display_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil
	}
	if len(payload) == 0 {
		return nil
	}
	lat, _ := strconv.ParseFloat(payload[0].Lat, 64)
	lon, _ := strconv.ParseFloat(payload[0].Lon, 64)
	city, country := splitDisplayName(payload[0].Display)
	return []GeoLocation{{Latitude: lat, Longitude: lon, City: city, Country: country}}
}
