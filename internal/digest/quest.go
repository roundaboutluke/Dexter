package digest

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type QuestDigest struct {
	CycleKey string
	Total    int
	Rewards  map[string]int
	Seen     map[string]bool
	Stops    map[string]map[string]bool
	Updated  time.Time
}

type QuestDigestSummary struct {
	Total   int
	Rewards map[string]int
	Stops   map[string]map[string]bool
	Updated time.Time
}

type Store struct {
	mu         sync.Mutex
	items      map[string]map[int]*QuestDigest
	quietCycle map[string]string
}

func NewStore() *Store {
	return &Store{
		items:      map[string]map[int]*QuestDigest{},
		quietCycle: map[string]string{},
	}
}

func (s *Store) BeginQuiet(userID string) {
	if s == nil || userID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.quietCycle == nil {
		s.quietCycle = map[string]string{}
	}
	if _, ok := s.quietCycle[userID]; ok {
		return
	}
	s.quietCycle[userID] = fmt.Sprintf("quiet:%d", time.Now().UTC().UnixNano())
}

func (s *Store) EndQuiet(userID string) {
	if s == nil || userID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.quietCycle == nil {
		return
	}
	delete(s.quietCycle, userID)
}

func (s *Store) CycleKeyFor(userID string, updated time.Time) string {
	if s == nil || userID == "" {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.quietCycle != nil {
		if cycle, ok := s.quietCycle[userID]; ok && cycle != "" {
			return cycle
		}
	}
	return CycleKey(updated)
}

func (s *Store) Add(userID string, profileNo int, cycleKey, seenKey, stopText, reward string) {
	if s == nil || userID == "" || profileNo <= 0 || cycleKey == "" || reward == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles := s.items[userID]
	if profiles == nil {
		profiles = map[int]*QuestDigest{}
		s.items[userID] = profiles
	}
	entry := profiles[profileNo]
	if entry == nil || entry.CycleKey != cycleKey {
		entry = &QuestDigest{
			CycleKey: cycleKey,
			Rewards:  map[string]int{},
			Seen:     map[string]bool{},
			Stops:    map[string]map[string]bool{},
		}
		profiles[profileNo] = entry
	}
	if seenKey != "" {
		if entry.Seen[seenKey] {
			return
		}
		entry.Seen[seenKey] = true
	}
	entry.Total++
	entry.Rewards[reward]++
	if stopText != "" {
		stops := entry.Stops[reward]
		if stops == nil {
			stops = map[string]bool{}
			entry.Stops[reward] = stops
		}
		stops[stopText] = true
	}
	entry.Updated = time.Now()
}

func (s *Store) Consume(userID string, profileNo int) (*QuestDigestSummary, bool) {
	if s == nil || userID == "" || profileNo <= 0 {
		return nil, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	profiles := s.items[userID]
	if profiles == nil {
		return nil, false
	}
	entry := profiles[profileNo]
	if entry == nil || entry.Total == 0 {
		return nil, false
	}
	summary := &QuestDigestSummary{
		Total:   entry.Total,
		Rewards: map[string]int{},
		Stops:   map[string]map[string]bool{},
		Updated: entry.Updated,
	}
	for key, value := range entry.Rewards {
		summary.Rewards[key] = value
	}
	for reward, stops := range entry.Stops {
		outStops := map[string]bool{}
		for stop := range stops {
			outStops[stop] = true
		}
		summary.Stops[reward] = outStops
	}
	delete(profiles, profileNo)
	if len(profiles) == 0 {
		delete(s.items, userID)
	}
	return summary, true
}

func TopRewards(rewards map[string]int, limit int) []string {
	type item struct {
		Name  string
		Count int
	}
	list := make([]item, 0, len(rewards))
	for name, count := range rewards {
		if name == "" || count == 0 {
			continue
		}
		list = append(list, item{Name: name, Count: count})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			return list[i].Name < list[j].Name
		}
		return list[i].Count > list[j].Count
	})
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	out := make([]string, 0, len(list))
	for _, entry := range list {
		out = append(out, entry.Name+" x"+itoa(entry.Count))
	}
	return out
}

func RewardsWithStops(rewards map[string]int, stops map[string]map[string]bool) []string {
	type item struct {
		Name  string
		Count int
	}
	list := make([]item, 0, len(rewards))
	for name, count := range rewards {
		if name == "" || count == 0 {
			continue
		}
		list = append(list, item{Name: name, Count: count})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].Count == list[j].Count {
			return list[i].Name < list[j].Name
		}
		return list[i].Count > list[j].Count
	})
	out := make([]string, 0, len(list))
	for _, entry := range list {
		stopList := []string{}
		if stopMap := stops[entry.Name]; len(stopMap) > 0 {
			for stop := range stopMap {
				stopList = append(stopList, stop)
			}
			sort.Strings(stopList)
		}
		segment := entry.Name
		if len(stopList) > 0 {
			segment = segment + " at " + strings.Join(stopList, ", ")
		}
		out = append(out, segment)
	}
	return out
}

func itoa(value int) string {
	if value == 0 {
		return "0"
	}
	neg := value < 0
	if neg {
		value = -value
	}
	buf := [20]byte{}
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func CycleKey(t time.Time) string {
	return t.Format("2006010215")
}
