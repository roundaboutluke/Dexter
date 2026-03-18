package stats

import (
	"sync"
	"time"

	"dexter/internal/config"
)

// PokemonCount tracks scan counts for a pokemon.
type PokemonCount struct {
	AllScanned   int
	IvScanned    int
	ShinyScanned int
}

// ShinyStat summarizes shiny odds.
type ShinyStat struct {
	Total int     `json:"total"`
	Ratio float64 `json:"ratio"`
	Seen  int     `json:"seen"`
}

// Report contains rarity and shiny summaries.
type Report struct {
	Rarity map[int][]int     `json:"rarity"`
	Shiny  map[int]ShinyStat `json:"shiny"`
	Time   time.Time         `json:"time"`
}

// Tracker keeps pokemon stats in memory for rarity calculations.
type Tracker struct {
	mu         sync.Mutex
	perHour    map[int64]map[int]*PokemonCount
	lastReport Report
}

// NewTracker creates a tracker instance.
func NewTracker() *Tracker {
	return &Tracker{
		perHour: map[int64]map[int]*PokemonCount{},
	}
}

// Update increments counters for a pokemon.
func (t *Tracker) Update(pokemonID int, ivScanned bool, shiny bool) {
	if pokemonID <= 0 {
		return
	}
	now := time.Now().Unix()
	currentHour := now - (now % 3600)
	t.mu.Lock()
	hourMap := t.perHour[currentHour]
	if hourMap == nil {
		hourMap = map[int]*PokemonCount{}
		t.perHour[currentHour] = hourMap
	}
	entry := hourMap[pokemonID]
	if entry == nil {
		entry = &PokemonCount{}
		hourMap[pokemonID] = entry
	}
	entry.AllScanned++
	if ivScanned {
		entry.IvScanned++
		if shiny {
			entry.ShinyScanned++
		}
	}
	t.mu.Unlock()
}

// Calculate builds a rarity report based on config settings.
func (t *Tracker) Calculate(cfg *config.Config) Report {
	now := time.Now()
	report := Report{
		Rarity: map[int][]int{1: {}, 2: {}, 3: {}, 4: {}, 5: {}, 6: {}},
		Shiny:  map[int]ShinyStat{},
		Time:   now,
	}
	if cfg == nil {
		return report
	}
	maxPokemonID := getInt(cfg, "stats.maxPokemonId", 0)
	minSample := getInt(cfg, "stats.minSampleSize", 0)
	keepHours := getInt(cfg, "stats.pokemonCountToKeep", 1)
	exclude := getIntSet(cfg, "stats.excludeFromRare")
	group2 := getFloat(cfg, "stats.rarityGroup2Uncommon", 1.0)
	group3 := getFloat(cfg, "stats.rarityGroup3Rare", 0.5)
	group4 := getFloat(cfg, "stats.rarityGroup4VeryRare", 0.03)
	group5 := getFloat(cfg, "stats.rarityGroup5UltraRare", 0.01)

	nowTimestamp := now.Unix()
	currentHour := nowTimestamp - (nowTimestamp % 3600)
	expireBefore := currentHour - int64(keepHours)*3600

	totalCounts := map[int]*PokemonCount{}
	totalAll := 0

	t.mu.Lock()
	for hour, counts := range t.perHour {
		if hour < expireBefore {
			delete(t.perHour, hour)
			continue
		}
		for id, entry := range counts {
			total := totalCounts[id]
			if total == nil {
				total = &PokemonCount{}
				totalCounts[id] = total
			}
			total.AllScanned += entry.AllScanned
			total.IvScanned += entry.IvScanned
			total.ShinyScanned += entry.ShinyScanned
			totalAll += entry.AllScanned
		}
	}
	t.mu.Unlock()

	if totalAll < minSample {
		return report
	}

	if maxPokemonID <= 0 {
		for id := range totalCounts {
			if id > maxPokemonID {
				maxPokemonID = id
			}
		}
	}
	for id := 1; id <= maxPokemonID; id++ {
		if exclude[id] {
			continue
		}
		if _, ok := totalCounts[id]; !ok {
			report.Rarity[6] = append(report.Rarity[6], id)
		}
	}

	for id, entry := range totalCounts {
		if exclude[id] {
			continue
		}
		percent := float64(entry.AllScanned) / float64(totalAll) * 100
		switch {
		case percent <= group5:
			report.Rarity[5] = append(report.Rarity[5], id)
		case percent <= group4:
			report.Rarity[4] = append(report.Rarity[4], id)
		case percent <= group3:
			report.Rarity[3] = append(report.Rarity[3], id)
		case percent <= group2:
			report.Rarity[2] = append(report.Rarity[2], id)
		default:
			report.Rarity[1] = append(report.Rarity[1], id)
		}

		if entry.IvScanned > 100 && entry.ShinyScanned > 0 {
			report.Shiny[id] = ShinyStat{
				Total: entry.IvScanned,
				Ratio: float64(entry.IvScanned) / float64(entry.ShinyScanned),
				Seen:  entry.ShinyScanned,
			}
		}
	}
	return report
}

// StoreReport caches the latest calculated report.
func (t *Tracker) StoreReport(report Report) {
	t.mu.Lock()
	t.lastReport = report
	t.mu.Unlock()
}

// LatestReport returns the most recent report.
func (t *Tracker) LatestReport() (Report, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastReport.Time.IsZero() {
		return Report{}, false
	}
	return t.lastReport, true
}

func getInt(cfg *config.Config, path string, fallback int) int {
	raw, ok := cfg.Get(path)
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return fallback
}

func getFloat(cfg *config.Config, path string, fallback float64) float64 {
	raw, ok := cfg.Get(path)
	if !ok {
		return fallback
	}
	switch v := raw.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return fallback
}

func getIntSet(cfg *config.Config, path string) map[int]bool {
	set := map[int]bool{}
	raw, ok := cfg.Get(path)
	if !ok {
		return set
	}
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			switch n := item.(type) {
			case int:
				set[n] = true
			case int64:
				set[int(n)] = true
			case float64:
				set[int(n)] = true
			}
		}
	case []int:
		for _, n := range v {
			set[n] = true
		}
	}
	return set
}
