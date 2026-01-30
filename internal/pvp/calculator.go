package pvp

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"

	"poraclego/internal/config"
	"poraclego/internal/data"
)

// Entry describes a PvP ranking entry.
type Entry struct {
	PokemonID  int
	FormID     int
	Rank       int
	CP         int
	Level      float64
	Percentage float64
	Cap        int
	Caps       []int
	Evolution  bool
}

// Calculator computes PvP ranks for a given Pokemon.
type Calculator struct {
	cfg           *config.Config
	data          *data.GameData
	cpMultipliers map[float64]float64
	levels        []float64
	mu            sync.RWMutex
	cache         map[string]map[int]rankResult
}

type rankResult struct {
	Rank       int
	CP         int
	Level      float64
	Product    float64
	Percentage float64
}

// NewCalculator builds a PvP calculator.
func NewCalculator(cfg *config.Config, gameData *data.GameData) *Calculator {
	cpMultipliers := map[float64]float64{}
	levels := []float64{}
	if gameData != nil {
		if raw, ok := gameData.UtilData["cpMultipliers"].(map[string]any); ok {
			for key, value := range raw {
				level, err := strconv.ParseFloat(key, 64)
				if err != nil {
					continue
				}
				switch v := value.(type) {
				case float64:
					cpMultipliers[level] = v
					levels = append(levels, level)
				case int:
					cpMultipliers[level] = float64(v)
					levels = append(levels, level)
				}
			}
		}
	}
	sort.Float64s(levels)
	return &Calculator{
		cfg:           cfg,
		data:          gameData,
		cpMultipliers: cpMultipliers,
		levels:        levels,
		cache:         map[string]map[int]rankResult{},
	}
}

// Rankings computes PvP entries for the given Pokemon, IVs, and evolutions.
func (c *Calculator) Rankings(pokemonID, formID, atk, def, sta int, includeEvolution bool) map[int][]Entry {
	leagues := []int{1500, 2500, 500}
	caps := []int{50}
	if c.cfg != nil {
		if raw, ok := c.cfg.Get("pvp.levelCaps"); ok {
			if list := parseIntSlice(raw); len(list) > 0 {
				caps = list
			}
		}
	}

	result := map[int][]Entry{}
	targets := []pokemonTarget{{PokemonID: pokemonID, FormID: formID, Evolution: false}}
	if includeEvolution {
		targets = append(targets, c.evolutionsFor(pokemonID, formID)...)
	}

	for _, league := range leagues {
		entries := []Entry{}
		for _, cap := range caps {
			for _, target := range targets {
				rank := c.rankFor(target.PokemonID, target.FormID, league, cap, atk, def, sta)
				if rank.Rank == 0 {
					continue
				}
				entries = append(entries, Entry{
					PokemonID:  target.PokemonID,
					FormID:     target.FormID,
					Rank:       rank.Rank,
					CP:         rank.CP,
					Level:      rank.Level,
					Percentage: rank.Percentage,
					Cap:        cap,
					Caps:       []int{cap},
					Evolution:  target.Evolution,
				})
			}
		}
		if len(entries) > 0 {
			result[league] = entries
		}
	}
	return result
}

// Lookup returns raw monster data for a pokemon/form.
func (c *Calculator) Lookup(pokemonID, formID int) map[string]any {
	return c.lookupMonster(pokemonID, formID)
}

type pokemonTarget struct {
	PokemonID int
	FormID    int
	Evolution bool
}

func (c *Calculator) evolutionsFor(pokemonID, formID int) []pokemonTarget {
	if c.data == nil {
		return nil
	}
	entry := c.lookupMonster(pokemonID, formID)
	if entry == nil {
		return nil
	}
	raw, ok := entry["evolutions"].([]any)
	if !ok {
		return nil
	}
	out := []pokemonTarget{}
	for _, evo := range raw {
		m, ok := evo.(map[string]any)
		if !ok {
			continue
		}
		evoID := getInt(m["evoId"])
		if evoID == 0 {
			continue
		}
		evoForm := getInt(m["id"])
		out = append(out, pokemonTarget{PokemonID: evoID, FormID: evoForm, Evolution: true})
	}
	return out
}

func (c *Calculator) rankFor(pokemonID, formID, leagueCap, levelCap, atk, def, sta int) rankResult {
	cacheKey := fmt.Sprintf("%d_%d_%d_%d", pokemonID, formID, leagueCap, levelCap)
	c.mu.RLock()
	cache := c.cache[cacheKey]
	c.mu.RUnlock()
	if cache == nil {
		cache = c.buildCache(pokemonID, formID, leagueCap, levelCap)
	}
	if cache == nil {
		return rankResult{}
	}
	ivKey := ivKey(atk, def, sta)
	return cache[ivKey]
}

func (c *Calculator) buildCache(pokemonID, formID, leagueCap, levelCap int) map[int]rankResult {
	cacheKey := fmt.Sprintf("%d_%d_%d_%d", pokemonID, formID, leagueCap, levelCap)
	entry := c.lookupMonster(pokemonID, formID)
	if entry == nil {
		return nil
	}
	stats, ok := entry["stats"].(map[string]any)
	if !ok {
		return nil
	}
	baseAtk := toFloat(stats["baseAttack"])
	baseDef := toFloat(stats["baseDefense"])
	baseSta := toFloat(stats["baseStamina"])
	if baseAtk == 0 || baseDef == 0 || baseSta == 0 {
		return nil
	}

	levels := []float64{}
	for _, level := range c.levels {
		if level <= float64(levelCap) {
			levels = append(levels, level)
		}
	}
	if len(levels) == 0 {
		return nil
	}

	results := make(map[int]rankResult, 4096)
	for a := 0; a <= 15; a++ {
		for d := 0; d <= 15; d++ {
			for s := 0; s <= 15; s++ {
				bestProduct := 0.0
				bestCP := 0
				bestLevel := 0.0
				for _, level := range levels {
					cpm := c.cpMultipliers[level]
					if cpm == 0 {
						continue
					}
					cp := calcCP(baseAtk, baseDef, baseSta, float64(a), float64(d), float64(s), cpm)
					if cp > leagueCap {
						continue
					}
					product := (baseAtk + float64(a)) * (baseDef + float64(d)) * (baseSta + float64(s)) * math.Pow(cpm, 3)
					if product > bestProduct || (product == bestProduct && cp > bestCP) {
						bestProduct = product
						bestCP = cp
						bestLevel = level
					}
				}
				if bestCP == 0 {
					continue
				}
				results[ivKey(a, d, s)] = rankResult{
					CP:      bestCP,
					Level:   bestLevel,
					Product: bestProduct,
				}
			}
		}
	}

	if len(results) == 0 {
		return nil
	}

	keys := make([]int, 0, len(results))
	for key := range results {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		a := results[keys[i]]
		b := results[keys[j]]
		if a.Product == b.Product {
			return a.CP > b.CP
		}
		return a.Product > b.Product
	})
	topProduct := results[keys[0]].Product
	rank := 0
	prevProduct := -1.0
	prevCP := -1
	for idx, key := range keys {
		entry := results[key]
		if entry.Product != prevProduct || entry.CP != prevCP {
			rank = idx + 1
			prevProduct = entry.Product
			prevCP = entry.CP
		}
		entry.Rank = rank
		entry.Percentage = entry.Product / topProduct * 100
		results[key] = entry
	}

	c.mu.Lock()
	c.cache[cacheKey] = results
	c.mu.Unlock()
	return results
}

func (c *Calculator) lookupMonster(pokemonID, formID int) map[string]any {
	if c.data == nil {
		return nil
	}
	key := fmt.Sprintf("%d_%d", pokemonID, formID)
	if entry, ok := c.data.Monsters[key]; ok {
		if m, ok := entry.(map[string]any); ok {
			return m
		}
	}
	if formID != 0 {
		key = fmt.Sprintf("%d_%d", pokemonID, 0)
		if entry, ok := c.data.Monsters[key]; ok {
			if m, ok := entry.(map[string]any); ok {
				return m
			}
		}
	}
	return nil
}

func ivKey(atk, def, sta int) int {
	return (atk << 8) | (def << 4) | sta
}

func calcCP(baseAtk, baseDef, baseSta, ivAtk, ivDef, ivSta, cpm float64) int {
	atk := baseAtk + ivAtk
	def := baseDef + ivDef
	sta := baseSta + ivSta
	cp := int(math.Floor((atk*math.Sqrt(def)*math.Sqrt(sta)*cpm*cpm)/10.0 + 0.0000001))
	if cp < 10 {
		cp = 10
	}
	return cp
}

func getInt(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func toFloat(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		return 0
	}
}

func parseIntSlice(raw any) []int {
	switch v := raw.(type) {
	case []int:
		return v
	case []any:
		out := []int{}
		for _, item := range v {
			out = append(out, getInt(item))
		}
		return out
	case []float64:
		out := []int{}
		for _, item := range v {
			out = append(out, int(item))
		}
		return out
	}
	return nil
}
