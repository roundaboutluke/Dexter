package webhook

import (
	"sort"
	"sync"
	"time"
)

const recentActivityTTL = 6 * time.Hour

// RecentActivity tracks recently-seen game entities from incoming webhooks.
// Autocomplete handlers use this to prioritise currently-active options.
type RecentActivity struct {
	mu             sync.RWMutex
	raidBosses     map[int]time.Time
	questItems     map[int]time.Time
	questMegaMons  map[int]time.Time
	questPokemon   map[int]time.Time
	questCandyMons map[int]time.Time
	maxbattleMons  map[int]time.Time
}

// NewRecentActivity creates a new tracker.
func NewRecentActivity() *RecentActivity {
	return &RecentActivity{
		raidBosses:     make(map[int]time.Time),
		questItems:     make(map[int]time.Time),
		questMegaMons:  make(map[int]time.Time),
		questPokemon:   make(map[int]time.Time),
		questCandyMons: make(map[int]time.Time),
		maxbattleMons:  make(map[int]time.Time),
	}
}

// RecordRaid notes that a raid boss was seen.
func (r *RecentActivity) RecordRaid(pokemonID int) {
	if r == nil || pokemonID <= 0 {
		return
	}
	r.mu.Lock()
	r.raidBosses[pokemonID] = time.Now()
	r.mu.Unlock()
}

// RecordQuestItem notes that a quest item reward was seen.
func (r *RecentActivity) RecordQuestItem(itemID int) {
	if r == nil || itemID <= 0 {
		return
	}
	r.mu.Lock()
	r.questItems[itemID] = time.Now()
	r.mu.Unlock()
}

// RecordQuestMegaEnergy notes that a mega energy quest reward was seen.
func (r *RecentActivity) RecordQuestMegaEnergy(pokemonID int) {
	if r == nil || pokemonID <= 0 {
		return
	}
	r.mu.Lock()
	r.questMegaMons[pokemonID] = time.Now()
	r.mu.Unlock()
}

// RecordQuestPokemon notes that a pokemon encounter quest reward was seen.
func (r *RecentActivity) RecordQuestPokemon(pokemonID int) {
	if r == nil || pokemonID <= 0 {
		return
	}
	r.mu.Lock()
	r.questPokemon[pokemonID] = time.Now()
	r.mu.Unlock()
}

// RecordQuestCandy notes that a candy quest reward was seen for a pokemon.
func (r *RecentActivity) RecordQuestCandy(pokemonID int) {
	if r == nil || pokemonID <= 0 {
		return
	}
	r.mu.Lock()
	r.questCandyMons[pokemonID] = time.Now()
	r.mu.Unlock()
}

// RecordMaxBattle notes that a max battle boss was seen.
func (r *RecentActivity) RecordMaxBattle(pokemonID int) {
	if r == nil || pokemonID <= 0 {
		return
	}
	r.mu.Lock()
	r.maxbattleMons[pokemonID] = time.Now()
	r.mu.Unlock()
}

// ActiveRaidBosses returns pokemon IDs seen as raid bosses recently, sorted.
func (r *RecentActivity) ActiveRaidBosses() []int {
	return r.activeIDs(r.raidBosses)
}

// ActiveQuestItems returns item IDs seen as quest rewards recently, sorted.
func (r *RecentActivity) ActiveQuestItems() []int {
	return r.activeIDs(r.questItems)
}

// ActiveQuestMegaEnergy returns pokemon IDs seen as mega energy quest rewards recently, sorted.
func (r *RecentActivity) ActiveQuestMegaEnergy() []int {
	return r.activeIDs(r.questMegaMons)
}

// ActiveQuestPokemon returns pokemon IDs seen as quest encounter rewards recently, sorted.
func (r *RecentActivity) ActiveQuestPokemon() []int {
	return r.activeIDs(r.questPokemon)
}

// ActiveQuestCandy returns pokemon IDs seen as quest candy rewards recently, sorted.
func (r *RecentActivity) ActiveQuestCandy() []int {
	return r.activeIDs(r.questCandyMons)
}

// ActiveMaxBattleBosses returns pokemon IDs seen as max battle bosses recently, sorted.
func (r *RecentActivity) ActiveMaxBattleBosses() []int {
	return r.activeIDs(r.maxbattleMons)
}

func (r *RecentActivity) activeIDs(m map[int]time.Time) []int {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	cutoff := time.Now().Add(-recentActivityTTL)
	ids := make([]int, 0, len(m))
	for id, seen := range m {
		if seen.After(cutoff) {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

// Prune removes entries older than the TTL.
func (r *RecentActivity) Prune() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-recentActivityTTL)
	pruneMap(r.raidBosses, cutoff)
	pruneMap(r.questItems, cutoff)
	pruneMap(r.questMegaMons, cutoff)
	pruneMap(r.questPokemon, cutoff)
	pruneMap(r.questCandyMons, cutoff)
	pruneMap(r.maxbattleMons, cutoff)
}

func pruneMap(m map[int]time.Time, cutoff time.Time) {
	for id, seen := range m {
		if seen.Before(cutoff) {
			delete(m, id)
		}
	}
}

// Quest reward type constants (matching PoracleJS/database values).
const (
	rewardTypeItem       = 2
	rewardTypeCandy      = 4
	rewardTypePokemon    = 7
	rewardTypeMegaEnergy = 12
)

// recordRecentActivity extracts entity IDs from incoming hooks and records them.
func (p *Processor) recordRecentActivity(hook *Hook) {
	if p == nil || p.recentActivity == nil || hook == nil {
		return
	}
	switch hook.Type {
	case "raid":
		if id := getInt(hook.Message["pokemon_id"]); id > 0 {
			p.recentActivity.RecordRaid(id)
		}
	case "max_battle":
		id := getInt(hook.Message["battle_pokemon_id"])
		if id <= 0 {
			id = getInt(hook.Message["pokemon_id"])
		}
		if id > 0 {
			p.recentActivity.RecordMaxBattle(id)
		}
	case "quest":
		p.recordQuestRecentActivity(hook)
	}
}

// recordQuestRecentActivity extracts reward info from quest hooks.
// Scanners (Golbat, RDM) typically send rewards as a structured
// quest_rewards JSON array. We parse that first, falling back to
// flat top-level fields if the array is empty.
func (p *Processor) recordQuestRecentActivity(hook *Hook) {
	// Try structured quest_rewards array first (primary path for most scanners).
	rewards := questRewardsFromHook(hook)
	for _, reward := range rewards {
		rewardType := getInt(reward["type"])
		info, _ := reward["info"].(map[string]any)
		switch rewardType {
		case rewardTypeItem:
			id := getIntFromMap(info, "item_id")
			if id == 0 {
				id = getIntFromMap(info, "id")
			}
			p.recentActivity.RecordQuestItem(id)
		case rewardTypeCandy:
			p.recentActivity.RecordQuestCandy(getIntFromMap(info, "pokemon_id"))
		case rewardTypePokemon:
			p.recentActivity.RecordQuestPokemon(getIntFromMap(info, "pokemon_id"))
		case rewardTypeMegaEnergy:
			p.recentActivity.RecordQuestMegaEnergy(getIntFromMap(info, "pokemon_id"))
		}
	}
	// Also record from the alternative/AR quest rewards.
	for _, reward := range questRewardsFromHookAR(hook) {
		rewardType := getInt(reward["type"])
		info, _ := reward["info"].(map[string]any)
		switch rewardType {
		case rewardTypeItem:
			id := getIntFromMap(info, "item_id")
			if id == 0 {
				id = getIntFromMap(info, "id")
			}
			p.recentActivity.RecordQuestItem(id)
		case rewardTypeCandy:
			p.recentActivity.RecordQuestCandy(getIntFromMap(info, "pokemon_id"))
		case rewardTypePokemon:
			p.recentActivity.RecordQuestPokemon(getIntFromMap(info, "pokemon_id"))
		case rewardTypeMegaEnergy:
			p.recentActivity.RecordQuestMegaEnergy(getIntFromMap(info, "pokemon_id"))
		}
	}
	if len(rewards) > 0 {
		return
	}
	// Fallback: flat top-level fields (older scanner formats).
	rewardType := getInt(hook.Message["reward_type"])
	if rewardType == 0 {
		rewardType = getInt(hook.Message["quest_reward_type"])
	}
	if rewardType == 0 {
		rewardType = getInt(hook.Message["reward_type_id"])
	}
	reward := getInt(hook.Message["reward"])
	if reward == 0 {
		switch rewardType {
		case rewardTypeItem:
			reward = getInt(hook.Message["quest_item_id"])
		case rewardTypePokemon:
			reward = getInt(hook.Message["quest_pokemon_id"])
		}
	}
	if reward == 0 {
		reward = getInt(hook.Message["pokemon_id"])
	}
	switch rewardType {
	case rewardTypeItem:
		p.recentActivity.RecordQuestItem(reward)
	case rewardTypeCandy:
		p.recentActivity.RecordQuestCandy(reward)
	case rewardTypePokemon:
		p.recentActivity.RecordQuestPokemon(reward)
	case rewardTypeMegaEnergy:
		p.recentActivity.RecordQuestMegaEnergy(reward)
	}
}
