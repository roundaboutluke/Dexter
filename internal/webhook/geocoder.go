package webhook

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"poraclego/internal/config"
)

// Geocoder performs reverse geocoding with basic caching.
type Geocoder struct {
	cfg         *config.Config
	client      *http.Client
	cache       map[string]string
	detailCache map[string]ReverseResult
	mu          sync.Mutex
}

// ReverseResult mirrors the JS geocoder payload fields needed by DTS.
type ReverseResult struct {
	FormattedAddress string `json:"formattedAddress"`
	City             string `json:"city"`
	Country          string `json:"country"`
	State            string `json:"state"`
	Zipcode          string `json:"zipcode"`
	StreetName       string `json:"streetName"`
	StreetNumber     string `json:"streetNumber"`
	CountryCode      string `json:"countryCode"`
	Neighbourhood    string `json:"neighbourhood"`
	Suburb           string `json:"suburb"`
	Town             string `json:"town"`
	Village          string `json:"village"`
	Shop             string `json:"shop"`
}

// GeoLocation represents a forward geocode result.
type GeoLocation struct {
	Latitude  float64
	Longitude float64
	City      string
	Country   string
}

// NewGeocoder creates a geocoder instance.
func NewGeocoder(cfg *config.Config) *Geocoder {
	return &Geocoder{
		cfg:         cfg,
		client:      &http.Client{Timeout: 8 * time.Second},
		cache:       map[string]string{},
		detailCache: map[string]ReverseResult{},
	}
}

type geocoderCachePayload struct {
	Cache   map[string]string        `json:"cache"`
	Details map[string]ReverseResult `json:"details"`
}

// LoadCache populates the geocoder cache from disk.
func (g *Geocoder) LoadCache(path string) {
	if g == nil || path == "" {
		return
	}
	var decoded geocoderCachePayload
	if err := loadJSONFile(path, &decoded); err == nil && (decoded.Cache != nil || decoded.Details != nil) {
		g.mu.Lock()
		if decoded.Cache != nil {
			g.cache = decoded.Cache
		}
		if decoded.Details != nil {
			g.detailCache = decoded.Details
		}
		g.mu.Unlock()
		return
	}
	// Backward-compatible: older cache files stored a raw map of cacheKey -> formattedAddress.
	var legacy map[string]string
	if err := loadJSONFile(path, &legacy); err != nil {
		return
	}
	g.mu.Lock()
	g.cache = legacy
	g.mu.Unlock()
}

// SaveCache writes the geocoder cache to disk.
func (g *Geocoder) SaveCache(path string) error {
	if g == nil || path == "" {
		return nil
	}
	g.mu.Lock()
	payload := geocoderCachePayload{
		Cache:   make(map[string]string, len(g.cache)),
		Details: make(map[string]ReverseResult, len(g.detailCache)),
	}
	for key, value := range g.cache {
		payload.Cache[key] = value
	}
	for key, value := range g.detailCache {
		payload.Details[key] = value
	}
	g.mu.Unlock()
	return saveJSONFile(path, payload)
}

// Forward resolves a location string to coordinates.
func (g *Geocoder) Forward(query string) []GeoLocation {
	if g == nil || g.cfg == nil {
		return nil
	}
	provider, _ := g.cfg.GetString("geocoding.provider")
	if strings.ToLower(provider) == "none" {
		return nil
	}
	if strings.TrimSpace(query) == "" {
		return nil
	}
	switch strings.ToLower(provider) {
	case "nominatim", "poracle":
		return g.forwardNominatim(query)
	case "pelias":
		return g.forwardPelias(query)
	case "google":
		return g.forwardGoogle(query)
	default:
		return g.forwardNominatim(query)
	}
}

// Reverse returns a formatted address for lat/lon.
func (g *Geocoder) Reverse(lat, lon float64) string {
	if g == nil || g.cfg == nil {
		return ""
	}
	provider, _ := g.cfg.GetString("geocoding.provider")
	if strings.ToLower(provider) == "none" {
		return ""
	}
	forwardOnly, _ := g.cfg.GetBool("geocoding.forwardOnly")
	if forwardOnly {
		return ""
	}

	cacheKey := geocodeCacheKey(g.cfg, lat, lon)
	if cacheKey != "" {
		g.mu.Lock()
		if cached, ok := g.cache[cacheKey]; ok {
			g.mu.Unlock()
			return cached
		}
		g.mu.Unlock()
	}

	var address string
	switch strings.ToLower(provider) {
	case "nominatim", "poracle":
		address = g.reverseNominatim(lat, lon)
	case "pelias":
		address = g.reversePelias(lat, lon)
	case "google":
		address = g.reverseGoogle(lat, lon)
	default:
		address = ""
	}
	if address != "" {
		if cacheKey != "" {
			g.mu.Lock()
			g.cache[cacheKey] = address
			g.mu.Unlock()
		}
	}
	return address
}

// ReverseDetails returns reverse geocode fields for DTS templates.
func (g *Geocoder) ReverseDetails(lat, lon float64) *ReverseResult {
	if g == nil || g.cfg == nil {
		return nil
	}
	provider, _ := g.cfg.GetString("geocoding.provider")
	if strings.ToLower(provider) == "none" {
		return nil
	}
	forwardOnly, _ := g.cfg.GetBool("geocoding.forwardOnly")
	if forwardOnly {
		return nil
	}

	cacheKey := geocodeCacheKey(g.cfg, lat, lon)
	if cacheKey != "" {
		g.mu.Lock()
		if cached, ok := g.detailCache[cacheKey]; ok {
			g.mu.Unlock()
			result := cached
			return &result
		}
		g.mu.Unlock()
	}

	var result *ReverseResult
	switch strings.ToLower(provider) {
	case "nominatim", "poracle":
		result = g.reverseNominatimDetails(lat, lon)
	case "pelias":
		result = g.reversePeliasDetails(lat, lon)
	case "google":
		result = g.reverseGoogleDetails(lat, lon)
	default:
	}
	if result != nil && result.FormattedAddress != "" {
		if cacheKey != "" {
			g.mu.Lock()
			g.detailCache[cacheKey] = *result
			g.cache[cacheKey] = result.FormattedAddress
			g.mu.Unlock()
		}
	}
	return result
}

func (g *Geocoder) googleAPIKey() string {
	if g == nil || g.cfg == nil {
		return ""
	}
	keys, _ := g.cfg.GetStringSlice("geocoding.geocodingKey")
	candidates := make([]string, 0, len(keys))
	for _, entry := range keys {
		if strings.TrimSpace(entry) != "" {
			candidates = append(candidates, strings.TrimSpace(entry))
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return candidates[rng.Intn(len(candidates))]
}

func (g *Geocoder) peliasAPIKey() string {
	if g == nil || g.cfg == nil {
		return ""
	}
	key, _ := g.cfg.GetString("geocoding.providerKey")
	return strings.TrimSpace(key)
}

func geocodeCacheKey(cfg *config.Config, lat, lon float64) string {
	if cfg == nil {
		return fmt.Sprintf("%.4f,%.4f", lat, lon)
	}
	precision, ok := cfg.GetInt("geocoding.cacheDetail")
	if !ok {
		precision = 3
	}
	if precision == 0 {
		return ""
	}
	if precision < 0 {
		precision = 0
	}
	format := fmt.Sprintf("%%.%df,%%.%df", precision, precision)
	return fmt.Sprintf(format, lat, lon)
}

// Intersection returns a nearby intersection or "No Intersection" if unavailable.
func (g *Geocoder) Intersection(lat, lon float64) string {
	if g == nil || g.cfg == nil {
		return "No Intersection"
	}
	users := intersectionUsersFromConfig(g.cfg)
	if len(users) == 0 {
		return "No Intersection"
	}
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	user := users[rng.Intn(len(users))]
	reqURL := fmt.Sprintf("http://api.geonames.org/findNearestIntersectionJSON?lat=%f&lng=%f&username=%s", lat, lon, url.QueryEscape(user))
	resp, err := g.client.Get(reqURL)
	if err != nil {
		return "No Intersection"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "No Intersection"
	}
	var payload struct {
		Intersection struct {
			Street1 string `json:"street1"`
			Street2 string `json:"street2"`
		} `json:"intersection"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "No Intersection"
	}
	if payload.Intersection.Street1 == "" || payload.Intersection.Street2 == "" {
		return "No Intersection"
	}
	return fmt.Sprintf("%s & %s", payload.Intersection.Street1, payload.Intersection.Street2)
}

func intersectionUsersFromConfig(cfg *config.Config) []string {
	raw, ok := cfg.Get("geocoding.intersectionUsers")
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if strings.TrimSpace(item) != "" {
				out = append(out, item)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	default:
		return nil
	}
}

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

func splitDisplayName(value string) (string, string) {
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return "", ""
	}
	country := strings.TrimSpace(parts[len(parts)-1])
	city := ""
	if len(parts) > 1 {
		city = strings.TrimSpace(parts[len(parts)-2])
	}
	return city, country
}

func (g *Geocoder) reverseGoogle(lat, lon float64) string {
	result := g.reverseGoogleDetails(lat, lon)
	if result == nil {
		return ""
	}
	return result.FormattedAddress
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
	values.Set("size", "1")
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

	props := payload.Features[0].Properties
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

	return &ReverseResult{
		FormattedAddress: address,
		StreetName:       strings.TrimSpace(props.Street),
		StreetNumber:     strings.TrimSpace(props.HouseNumber),
		City:             city,
		Country:          strings.TrimSpace(props.Country),
		State:            strings.TrimSpace(props.Region),
		Zipcode:          strings.TrimSpace(props.PostalCode),
		CountryCode:      countryCode,
		Neighbourhood:    strings.TrimSpace(props.Neighbourhood),
		Suburb:           strings.TrimSpace(props.Borough),
		Town:             strings.TrimSpace(props.Locality),
		Village:          strings.TrimSpace(props.LocalAdmin),
	}
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
