package uicons

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Client fetches and resolves uicons image URLs.
type Client struct {
	mu        sync.RWMutex
	baseURL   string
	imageType string
	index     *indexData
	loadedAt  time.Time
}

type indexData struct {
	Pokemon        map[string]bool
	Gym            map[string]bool
	Pokestop       map[string]bool
	Invasion       map[string]bool
	RaidEgg        map[string]bool
	Type           map[string]bool
	Weather        map[string]bool
	Team           map[string]bool
	RewardItem     map[string]bool
	RewardStardust map[string]bool
	RewardMega     map[string]bool
	RewardCandy    map[string]bool
	RewardXLCandy  map[string]bool
}

// NewClient creates a uicons client.
func NewClient(baseURL, imageType string) *Client {
	baseURL = strings.TrimRight(baseURL, "/")
	if imageType == "" {
		imageType = "png"
	}
	return &Client{baseURL: baseURL, imageType: imageType}
}

// IsUiconsRepository verifies that the base URL exposes index.json.
func (c *Client) IsUiconsRepository() (bool, error) {
	c.mu.RLock()
	if c.index != nil && time.Since(c.loadedAt) < time.Hour {
		c.mu.RUnlock()
		return true, nil
	}
	c.mu.RUnlock()

	if c.baseURL == "" {
		return false, fmt.Errorf("missing base url")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Re-check under write lock to avoid redundant fetches.
	if c.index != nil && time.Since(c.loadedAt) < time.Hour {
		return true, nil
	}

	resp, err := httpClient.Get(c.baseURL + "/index.json")
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	var raw map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return false, err
	}
	c.index = buildIndex(raw)
	c.loadedAt = time.Now()
	return c.index != nil, nil
}

// TypeIcon returns a URL for a type icon.
func (c *Client) TypeIcon(typeID int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolve("type", typeID, c.index.Type)
}

// WeatherIcon returns a URL for a weather icon.
func (c *Client) WeatherIcon(weatherID int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolve("weather", weatherID, c.index.Weather)
}

// TeamIcon returns a URL for a team icon.
func (c *Client) TeamIcon(teamID int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolve("team", teamID, c.index.Team)
}

// RewardItemIcon returns a URL for a reward item icon.
func (c *Client) RewardItemIcon(itemID int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolveCandidates("reward/item", rewardCandidates(c.imageType, itemID, 0), c.index.RewardItem)
}

// RewardStardustIcon returns a URL for a stardust reward icon.
func (c *Client) RewardStardustIcon(amount int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolveCandidates("reward/stardust", rewardCandidates(c.imageType, amount, 0), c.index.RewardStardust)
}

// RewardMegaEnergyIcon returns a URL for a mega energy reward icon.
func (c *Client) RewardMegaEnergyIcon(pokemonID, amount int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolveCandidates("reward/mega_resource", rewardCandidates(c.imageType, pokemonID, amount), c.index.RewardMega)
}

// RewardCandyIcon returns a URL for a candy reward icon.
func (c *Client) RewardCandyIcon(pokemonID, amount int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolveCandidates("reward/candy", rewardCandidates(c.imageType, pokemonID, amount), c.index.RewardCandy)
}

// RewardXLCandyIcon returns a URL for an XL candy reward icon.
func (c *Client) RewardXLCandyIcon(pokemonID, amount int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	return c.resolveCandidates("reward/xl_candy", rewardCandidates(c.imageType, pokemonID, amount), c.index.RewardXLCandy)
}

// PokemonIcon returns a URL for a Pokemon icon.
func (c *Client) PokemonIcon(pokemonID, form, evolution, gender, costume, alignment int, shiny bool, bread int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	candidates := pokemonCandidates(c.imageType, pokemonID, form, evolution, gender, costume, alignment, shiny, bread)
	return c.resolveCandidates("pokemon", candidates, c.index.Pokemon)
}

// RaidEggIcon returns a URL for a raid egg icon.
func (c *Client) RaidEggIcon(level int, hatched, ex bool) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	candidates := eggCandidates(c.imageType, level, hatched, ex)
	return c.resolveCandidates("raid/egg", candidates, c.index.RaidEgg)
}

// GymIcon returns a URL for a gym icon.
func (c *Client) GymIcon(teamID, trainerCount int, inBattle, ex bool) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	candidates := gymCandidates(c.imageType, teamID, trainerCount, inBattle, ex)
	return c.resolveCandidates("gym", candidates, c.index.Gym)
}

// PokestopIcon returns a URL for a pokestop icon.
func (c *Client) PokestopIcon(lureID int, invasionActive bool, incidentDisplayType int, questActive bool) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	candidates := pokestopCandidates(c.imageType, lureID, invasionActive, incidentDisplayType, questActive)
	return c.resolveCandidates("pokestop", candidates, c.index.Pokestop)
}

// InvasionIcon returns a URL for an invasion icon.
func (c *Client) InvasionIcon(gruntType int) (string, bool) {
	if ok, _ := c.IsUiconsRepository(); !ok {
		return "", false
	}
	candidates := simpleCandidates(c.imageType, gruntType)
	return c.resolveCandidates("invasion", candidates, c.index.Invasion)
}

func (c *Client) resolve(folder string, id int, available map[string]bool) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if id <= 0 || c.index == nil {
		return "", false
	}
	filename := fmt.Sprintf("%d.%s", id, c.imageType)
	if available != nil && !available[filename] {
		return fmt.Sprintf("%s/%s/0.%s", c.baseURL, folder, c.imageType), false
	}
	return fmt.Sprintf("%s/%s/%s", c.baseURL, folder, filename), true
}

func (c *Client) resolveCandidates(folder string, candidates []string, available map[string]bool) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(candidates) == 0 || c.index == nil {
		return "", false
	}
	for _, filename := range candidates {
		if filename == "" {
			continue
		}
		if available == nil || available[filename] {
			return fmt.Sprintf("%s/%s/%s", c.baseURL, folder, filename), true
		}
	}
	return fmt.Sprintf("%s/%s/0.%s", c.baseURL, folder, c.imageType), false
}

func buildIndex(raw map[string]any) *indexData {
	if raw == nil {
		return nil
	}
	rewardItem, rewardStardust, rewardMega, rewardCandy, rewardXLCandy := toRewardSets(raw["reward"])
	return &indexData{
		Pokemon:        toSet(raw["pokemon"]),
		Gym:            toSet(raw["gym"]),
		Pokestop:       toSet(raw["pokestop"]),
		Invasion:       toSet(raw["invasion"]),
		RaidEgg:        toRaidEggSet(raw["raid"]),
		Type:           toSet(raw["type"]),
		Weather:        toSet(raw["weather"]),
		Team:           toSet(raw["team"]),
		RewardItem:     rewardItem,
		RewardStardust: rewardStardust,
		RewardMega:     rewardMega,
		RewardCandy:    rewardCandy,
		RewardXLCandy:  rewardXLCandy,
	}
}

func toRaidEggSet(raw any) map[string]bool {
	if entry, ok := raw.(map[string]any); ok {
		return toSet(entry["egg"])
	}
	return map[string]bool{}
}

func toRewardSets(raw any) (map[string]bool, map[string]bool, map[string]bool, map[string]bool, map[string]bool) {
	if entry, ok := raw.(map[string]any); ok {
		return toSet(entry["item"]),
			toSet(entry["stardust"]),
			toSet(entry["mega_resource"]),
			toSet(entry["candy"]),
			toSet(entry["xl_candy"])
	}
	return map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
}

func toSet(raw any) map[string]bool {
	out := map[string]bool{}
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			out[fmt.Sprintf("%v", item)] = true
		}
	case []string:
		for _, item := range v {
			out[item] = true
		}
	}
	return out
}

func simpleCandidates(imageType string, id int) []string {
	if id <= 0 {
		return []string{}
	}
	return []string{fmt.Sprintf("%d.%s", id, imageType)}
}

func rewardCandidates(imageType string, id, amount int) []string {
	if id <= 0 {
		return []string{}
	}
	candidates := []string{}
	if amount > 0 {
		candidates = append(candidates, fmt.Sprintf("%d_a%d.%s", id, amount, imageType))
	}
	candidates = append(candidates, fmt.Sprintf("%d.%s", id, imageType))
	return candidates
}

func pokemonCandidates(imageType string, pokemonID, form, evolution, gender, costume, alignment int, shiny bool, bread int) []string {
	if pokemonID <= 0 {
		return []string{}
	}
	breadSuffixes := []string{""}
	if bread > 0 {
		breadSuffixes = []string{fmt.Sprintf("_b%d", bread), ""}
	}
	evolutionSuffixes := suffixOptions(evolution, "_e")
	formSuffixes := suffixOptions(form, "_f")
	costumeSuffixes := suffixOptions(costume, "_c")
	genderSuffixes := suffixOptions(gender, "_g")
	alignmentSuffixes := suffixOptions(alignment, "_a")
	shinySuffixes := []string{"_s", ""}
	if !shiny {
		shinySuffixes = []string{""}
	}
	candidates := make([]string, 0, len(breadSuffixes)*len(evolutionSuffixes)*len(formSuffixes)*len(costumeSuffixes)*len(genderSuffixes)*len(alignmentSuffixes)*len(shinySuffixes))
	for _, breadSuffix := range breadSuffixes {
		for _, evolutionSuffix := range evolutionSuffixes {
			for _, formSuffix := range formSuffixes {
				for _, costumeSuffix := range costumeSuffixes {
					for _, genderSuffix := range genderSuffixes {
						for _, alignmentSuffix := range alignmentSuffixes {
							for _, shinySuffix := range shinySuffixes {
								candidates = append(candidates, fmt.Sprintf("%d%s%s%s%s%s%s%s.%s", pokemonID, breadSuffix, evolutionSuffix, formSuffix, costumeSuffix, genderSuffix, alignmentSuffix, shinySuffix, imageType))
							}
						}
					}
				}
			}
		}
	}
	return candidates
}

func eggCandidates(imageType string, level int, hatched, ex bool) []string {
	if level <= 0 {
		return []string{}
	}
	hatchedSuffixes := suffixFlag(hatched, "_h")
	exSuffixes := suffixFlag(ex, "_ex")
	candidates := make([]string, 0, len(hatchedSuffixes)*len(exSuffixes))
	for _, hatchedSuffix := range hatchedSuffixes {
		for _, exSuffix := range exSuffixes {
			candidates = append(candidates, fmt.Sprintf("%d%s%s.%s", level, hatchedSuffix, exSuffix, imageType))
		}
	}
	return candidates
}

func gymCandidates(imageType string, teamID, trainerCount int, inBattle, ex bool) []string {
	if teamID < 0 {
		return []string{}
	}
	trainerSuffixes := suffixOptions(trainerCount, "_t")
	inBattleSuffixes := suffixFlag(inBattle, "_b")
	exSuffixes := suffixFlag(ex, "_ex")
	candidates := make([]string, 0, len(trainerSuffixes)*len(inBattleSuffixes)*len(exSuffixes))
	for _, trainerSuffix := range trainerSuffixes {
		for _, inBattleSuffix := range inBattleSuffixes {
			for _, exSuffix := range exSuffixes {
				candidates = append(candidates, fmt.Sprintf("%d%s%s%s.%s", teamID, trainerSuffix, inBattleSuffix, exSuffix, imageType))
			}
		}
	}
	return candidates
}

func pokestopCandidates(imageType string, lureID int, invasionActive bool, incidentDisplayType int, questActive bool) []string {
	if lureID < 0 {
		return []string{}
	}
	invasionSuffixes := suffixFlag(invasionActive, "_i")
	displaySuffixes := suffixOptions(incidentDisplayType, "")
	questSuffixes := suffixFlag(questActive, "_q")
	candidates := make([]string, 0, len(invasionSuffixes)*len(displaySuffixes)*len(questSuffixes))
	for _, invasionSuffix := range invasionSuffixes {
		for _, displaySuffix := range displaySuffixes {
			for _, questSuffix := range questSuffixes {
				candidates = append(candidates, fmt.Sprintf("%d%s%s%s.%s", lureID, invasionSuffix, displaySuffix, questSuffix, imageType))
			}
		}
	}
	return candidates
}

func suffixOptions(value int, prefix string) []string {
	if value > 0 {
		return []string{fmt.Sprintf("%s%d", prefix, value), ""}
	}
	return []string{""}
}

func suffixFlag(enabled bool, suffix string) []string {
	if enabled {
		return []string{suffix, ""}
	}
	return []string{""}
}

var (
	cachedClients   = map[string]*Client{}
	cachedClientsMu sync.RWMutex
)

// CachedClient returns a shared Client for the given base URL and image type.
// Clients are cached by key and reused across calls. Thread-safe.
func CachedClient(baseURL, imageType string) *Client {
	if baseURL == "" {
		return nil
	}
	if imageType == "" {
		imageType = "png"
	}
	key := baseURL + "|" + imageType

	// Fast path: read lock for cache hits.
	cachedClientsMu.RLock()
	client, ok := cachedClients[key]
	cachedClientsMu.RUnlock()
	if ok {
		return client
	}

	// Slow path: write lock for cache misses.
	cachedClientsMu.Lock()
	defer cachedClientsMu.Unlock()
	// Double-check under write lock.
	if client, ok := cachedClients[key]; ok {
		return client
	}
	client = NewClient(baseURL, imageType)
	cachedClients[key] = client
	return client
}

// IsCachedRepo checks whether a cached client for the given URL is a valid uicons repository.
func IsCachedRepo(baseURL, imageType string) bool {
	client := CachedClient(baseURL, imageType)
	if client == nil {
		return false
	}
	ok, _ := client.IsUiconsRepository()
	return ok
}
