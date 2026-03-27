package webhook

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"dexter/internal/config"
	"dexter/internal/logging"
)

// Geocoder performs reverse geocoding with basic caching.
type Geocoder struct {
	cfg         *config.Config
	client      *http.Client
	cache       map[string]string
	detailCache map[string]ReverseResult
	lastUsed    map[string]time.Time
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
		lastUsed:    map[string]time.Time{},
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
	start := time.Now()
	defer func() {
		if logger := logging.Get().General; logger != nil {
			logger.Logf(logging.TimingLevel(g.cfg), "Geocode %s (%d ms)", query, time.Since(start).Milliseconds())
		}
	}()
	switch strings.ToLower(provider) {
	case "nominatim", "poracle":
		return g.forwardNominatim(query)
	case "pelias":
		return g.forwardPelias(query)
	case "photon":
		return g.forwardPhoton(query)
	case "google":
		return g.forwardGoogle(query)
	default:
		return g.forwardNominatim(query)
	}
}

// reverseProvider returns the normalized geocoding provider name, or "" if reverse
// geocoding is disabled. It handles the nil/none/forwardOnly checks shared by
// Reverse and ReverseDetails.
func (g *Geocoder) reverseProvider() string {
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
	return strings.ToLower(provider)
}

func (g *Geocoder) reverseDetailsByProvider(provider string, lat, lon float64) *ReverseResult {
	switch provider {
	case "nominatim", "poracle":
		return g.reverseNominatimDetails(lat, lon)
	case "pelias":
		return g.reversePeliasDetails(lat, lon)
	case "photon":
		return g.reversePhotonDetails(lat, lon)
	case "google":
		return g.reverseGoogleDetails(lat, lon)
	default:
		return nil
	}
}

// Reverse returns a formatted address for lat/lon.
func (g *Geocoder) Reverse(lat, lon float64) string {
	provider := g.reverseProvider()
	if provider == "" {
		return ""
	}
	start := time.Now()
	defer func() {
		if logger := logging.Get().General; logger != nil {
			logger.Logf(logging.TimingLevel(g.cfg), "Geocode %f,%f (%d ms)", lat, lon, time.Since(start).Milliseconds())
		}
	}()

	cacheKey := geocodeCacheKey(g.cfg, lat, lon)
	if cacheKey != "" {
		g.mu.Lock()
		if cached, ok := g.cache[cacheKey]; ok {
			g.lastUsed[cacheKey] = time.Now()
			g.mu.Unlock()
			return cached
		}
		g.mu.Unlock()
	}

	var address string
	switch provider {
	case "nominatim", "poracle":
		address = g.reverseNominatim(lat, lon)
	case "pelias":
		address = g.reversePelias(lat, lon)
	case "photon":
		address = g.reversePhoton(lat, lon)
	case "google":
		address = g.reverseGoogle(lat, lon)
	}
	if address != "" && cacheKey != "" {
		g.mu.Lock()
		g.cache[cacheKey] = address
		g.lastUsed[cacheKey] = time.Now()
		g.mu.Unlock()
	}
	return address
}

// ReverseDetails returns reverse geocode fields for DTS templates.
func (g *Geocoder) ReverseDetails(lat, lon float64) *ReverseResult {
	provider := g.reverseProvider()
	if provider == "" {
		return nil
	}
	start := time.Now()
	defer func() {
		if logger := logging.Get().General; logger != nil {
			logger.Logf(logging.TimingLevel(g.cfg), "Geocode details %f,%f (%d ms)", lat, lon, time.Since(start).Milliseconds())
		}
	}()

	cacheKey := geocodeCacheKey(g.cfg, lat, lon)
	if cacheKey != "" {
		g.mu.Lock()
		if cached, ok := g.detailCache[cacheKey]; ok {
			g.lastUsed[cacheKey] = time.Now()
			g.mu.Unlock()
			result := cached
			return &result
		}
		g.mu.Unlock()
	}

	result := g.reverseDetailsByProvider(provider, lat, lon)
	if result != nil && result.FormattedAddress != "" && cacheKey != "" {
		g.mu.Lock()
		g.detailCache[cacheKey] = *result
		g.cache[cacheKey] = result.FormattedAddress
		g.lastUsed[cacheKey] = time.Now()
		g.mu.Unlock()
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

func normalizeCSVLower(value string) string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.ToLower(strings.TrimSpace(part))
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return strings.Join(out, ",")
}

func geocodeCacheScope(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}

	provider, _ := cfg.GetString("geocoding.provider")
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "unknown"
	}

	parts := []string{provider}
	if base, _ := cfg.GetString("geocoding.providerURL"); strings.TrimSpace(base) != "" {
		parts = append(parts, strings.TrimSpace(base))
	}

	switch provider {
	case "pelias":
		if layers, _ := cfg.GetString("geocoding.peliasLayers"); strings.TrimSpace(layers) != "" {
			parts = append(parts, "layers="+normalizeCSVLower(layers))
		}
		if preferred, _ := cfg.GetString("geocoding.peliasPreferredLayer"); strings.TrimSpace(preferred) != "" {
			parts = append(parts, "preferred="+strings.ToLower(strings.TrimSpace(preferred)))
		}
		if country, _ := cfg.GetString("geocoding.peliasBoundaryCountry"); strings.TrimSpace(country) != "" {
			parts = append(parts, "country="+strings.ToUpper(strings.TrimSpace(country)))
		}
		if size, ok := cfg.GetInt("geocoding.peliasResultSize"); ok && size > 0 {
			parts = append(parts, fmt.Sprintf("size=%d", size))
		}
	case "photon":
		if layers, _ := cfg.GetString("geocoding.photonLayers"); strings.TrimSpace(layers) != "" {
			parts = append(parts, "layers="+normalizeCSVLower(layers))
		}
		if preferred, _ := cfg.GetString("geocoding.photonPreferredLayer"); strings.TrimSpace(preferred) != "" {
			parts = append(parts, "preferred="+strings.ToLower(strings.TrimSpace(preferred)))
		}
		if lang, _ := cfg.GetString("geocoding.photonLang"); strings.TrimSpace(lang) != "" {
			parts = append(parts, "lang="+strings.ToLower(strings.TrimSpace(lang)))
		}
		if size, ok := cfg.GetInt("geocoding.photonResultSize"); ok && size > 0 {
			parts = append(parts, fmt.Sprintf("size=%d", size))
		}
	}

	raw := strings.Join(parts, "|")
	sum := sha1.Sum([]byte(raw))
	return fmt.Sprintf("%s:%x", provider, sum[:8])
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
	coords := fmt.Sprintf(format, lat, lon)
	if scope := geocodeCacheScope(cfg); scope != "" {
		return scope + ":" + coords
	}
	return coords
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

// PruneStale removes geocoder cache entries not used since the cutoff time.
func (g *Geocoder) PruneStale(cutoff time.Time) int {
	if g == nil {
		return 0
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	removed := 0
	for key, used := range g.lastUsed {
		if used.Before(cutoff) {
			delete(g.cache, key)
			delete(g.detailCache, key)
			delete(g.lastUsed, key)
			removed++
		}
	}
	// Also remove entries that have no lastUsed tracking (pre-existing from older cache files).
	for key := range g.cache {
		if _, tracked := g.lastUsed[key]; !tracked {
			delete(g.cache, key)
			delete(g.detailCache, key)
			removed++
		}
	}
	return removed
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
