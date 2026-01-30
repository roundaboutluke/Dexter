package webhook

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"poraclego/internal/config"
)

type MonsterChangeTracker struct {
	cfg      *config.Config
	cacheDir string
	path     string

	mu         sync.Mutex
	lastPurge  int64
	encounters map[string]*monsterChangeEncounter
}

type monsterChangeEncounter struct {
	Signature string `json:"signature"`
	Expires   int64  `json:"expires"`

	PokemonID int     `json:"pokemon_id"`
	Form      int     `json:"form"`
	Costume   int     `json:"costume"`
	Gender    int     `json:"gender"`
	CP        int     `json:"cp"`
	IV        float64 `json:"iv"`

	Cares map[string]*monsterChangeCare `json:"cares"`
}

type monsterChangeCare struct {
	TargetID   string `json:"target_id"`
	TargetType string `json:"target_type"`
	TargetName string `json:"target_name"`
	Language   string `json:"language"`
	Template   string `json:"template"`
	Ping       string `json:"ping"`
	Clean      bool   `json:"clean"`
	UpdateKey  string `json:"update_key"`
}

func NewMonsterChangeTracker(cfg *config.Config, root string) *MonsterChangeTracker {
	cacheDir := ""
	if root != "" {
		cacheDir = filepath.Join(root, ".cache")
	}
	return &MonsterChangeTracker{
		cfg:        cfg,
		cacheDir:   cacheDir,
		path:       filepath.Join(cacheDir, "monsterChangeCache.json"),
		encounters: map[string]*monsterChangeEncounter{},
	}
}

func (t *MonsterChangeTracker) LoadCache() {
	if t == nil || t.cacheDir == "" || t.path == "" {
		return
	}
	var payload map[string]*monsterChangeEncounter
	if err := loadJSONFile(t.path, &payload); err != nil {
		return
	}
	now := time.Now().Unix()
	for key, entry := range payload {
		if entry == nil {
			delete(payload, key)
			continue
		}
		if entry.Expires > 0 && entry.Expires <= now {
			delete(payload, key)
		}
	}
	t.mu.Lock()
	t.encounters = payload
	t.mu.Unlock()
}

func (t *MonsterChangeTracker) SaveCache() {
	if t == nil || t.cacheDir == "" || t.path == "" {
		return
	}
	t.mu.Lock()
	payload := make(map[string]*monsterChangeEncounter, len(t.encounters))
	for key, value := range t.encounters {
		payload[key] = value
	}
	t.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return
	}
	_ = saveJSONFile(t.path, payload)
}

func (t *MonsterChangeTracker) TrackCare(encounterID string, target alertTarget, expires int64, clean bool, ping, updateKey string, hook *Hook) {
	if t == nil || encounterID == "" || updateKey == "" || hook == nil {
		return
	}
	now := time.Now().Unix()
	if expires > 0 && expires <= now {
		return
	}

	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		pokemonID = getInt(hook.Message["pokemonId"])
	}
	form := getInt(hook.Message["form"])
	costume := getInt(hook.Message["costume"])
	gender := getInt(hook.Message["gender"])
	cp := getInt(hook.Message["cp"])
	iv := computeIV(hook)
	signature := monsterChangeSignature(pokemonID, form, costume, gender)

	t.mu.Lock()
	defer t.mu.Unlock()
	t.purgeExpiredLocked(now)
	entry := t.encounters[encounterID]
	if entry == nil {
		entry = &monsterChangeEncounter{Cares: map[string]*monsterChangeCare{}}
		t.encounters[encounterID] = entry
	}
	if entry.Expires == 0 || (expires > 0 && expires > entry.Expires) {
		entry.Expires = expires
	}
	if entry.Signature == "" {
		entry.Signature = signature
		entry.PokemonID = pokemonID
		entry.Form = form
		entry.Costume = costume
		entry.Gender = gender
		entry.CP = cp
		entry.IV = iv
	}
	if entry.Expires > 0 && entry.Expires <= now {
		delete(t.encounters, encounterID)
		return
	}
	// Stable key: a single tracked message (per row) can be updated even if the user tracks multiple filters.
	key := target.ID + "|" + updateKey
	care := entry.Cares[key]
	if care == nil {
		care = &monsterChangeCare{
			TargetID:   target.ID,
			TargetType: target.Type,
			TargetName: target.Name,
			Language:   target.Language,
			Template:   target.Template,
			UpdateKey:  updateKey,
		}
		entry.Cares[key] = care
	}
	care.Ping = ping
	care.Clean = care.Clean || clean
}

type monsterChangeSnapshot struct {
	PokemonID int
	Form      int
	Costume   int
	Gender    int
	CP        int
	IV        float64
	Expires   int64
}

func (t *MonsterChangeTracker) DetectChange(encounterID string, hook *Hook, expires int64) (monsterChangeSnapshot, []monsterChangeCare, bool) {
	if t == nil || encounterID == "" || hook == nil {
		return monsterChangeSnapshot{}, nil, false
	}
	now := time.Now().Unix()
	if expires > 0 && expires <= now {
		return monsterChangeSnapshot{}, nil, false
	}

	pokemonID := getInt(hook.Message["pokemon_id"])
	if pokemonID == 0 {
		pokemonID = getInt(hook.Message["pokemonId"])
	}
	form := getInt(hook.Message["form"])
	costume := getInt(hook.Message["costume"])
	gender := getInt(hook.Message["gender"])
	cp := getInt(hook.Message["cp"])
	iv := computeIV(hook)
	signature := monsterChangeSignature(pokemonID, form, costume, gender)

	t.mu.Lock()
	defer t.mu.Unlock()
	t.purgeExpiredLocked(now)
	entry := t.encounters[encounterID]
	if entry == nil {
		return monsterChangeSnapshot{}, nil, false
	}
	if entry.Expires > 0 && entry.Expires <= now {
		delete(t.encounters, encounterID)
		return monsterChangeSnapshot{}, nil, false
	}
	if entry.Signature == "" || signature == "" {
		return monsterChangeSnapshot{}, nil, false
	}
	if entry.Signature == signature {
		if entry.Expires == 0 || (expires > 0 && expires > entry.Expires) {
			entry.Expires = expires
		}
		return monsterChangeSnapshot{}, nil, false
	}
	old := monsterChangeSnapshot{
		PokemonID: entry.PokemonID,
		Form:      entry.Form,
		Costume:   entry.Costume,
		Gender:    entry.Gender,
		CP:        entry.CP,
		IV:        entry.IV,
		Expires:   entry.Expires,
	}
	entry.Signature = signature
	entry.PokemonID = pokemonID
	entry.Form = form
	entry.Costume = costume
	entry.Gender = gender
	entry.CP = cp
	entry.IV = iv
	if entry.Expires == 0 || (expires > 0 && expires > entry.Expires) {
		entry.Expires = expires
	}
	cares := make([]monsterChangeCare, 0, len(entry.Cares))
	for _, care := range entry.Cares {
		if care == nil || care.TargetID == "" || care.UpdateKey == "" {
			continue
		}
		cares = append(cares, *care)
	}
	return old, cares, true
}

func monsterChangeSignature(pokemonID, formID, costume, gender int) string {
	if pokemonID <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%d", pokemonID, formID, costume, gender)
}

func (t *MonsterChangeTracker) purgeExpiredLocked(now int64) {
	if t == nil {
		return
	}
	// Avoid full-map scans on every hook.
	if t.lastPurge != 0 && now-t.lastPurge < 300 {
		return
	}
	for key, entry := range t.encounters {
		if entry == nil {
			delete(t.encounters, key)
			continue
		}
		if entry.Expires > 0 && entry.Expires <= now {
			delete(t.encounters, key)
		}
	}
	t.lastPurge = now
}
