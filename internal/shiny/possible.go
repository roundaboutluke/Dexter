package shiny

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"poraclego/internal/logging"
)

const defaultURL = "https://raw.githubusercontent.com/jms412/PkmnShinyMap/main/shinyPossible.json"

type payload struct {
	Map map[string]any `json:"map"`
}

// Possible tracks which Pokemon/forms can be shiny.
type Possible struct {
	mu   sync.RWMutex
	keys map[string]struct{}
	url  string
}

// NewPossible returns a shiny-possible tracker.
func NewPossible(url string) *Possible {
	if url == "" {
		url = defaultURL
	}
	return &Possible{
		keys: map[string]struct{}{},
		url:  url,
	}
}

// Start refreshes the shiny list periodically until ctx is canceled.
func (p *Possible) Start(ctx context.Context, cachePath string) {
	if p == nil {
		return
	}
	if cachePath != "" {
		_ = p.loadCache(cachePath)
	}
	go func() {
		for {
			if err := p.refresh(cachePath); err != nil {
				if logger := logging.Get().General; logger != nil {
					logger.Warnf("shiny list refresh failed: %v", err)
				}
				select {
				case <-time.After(15 * time.Minute):
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case <-time.After(6 * time.Hour):
			case <-ctx.Done():
				return
			}
		}
	}()
}

// IsPossible returns true when the pokemon/form is marked shiny-capable.
func (p *Possible) IsPossible(pokemonID, formID int) bool {
	if p == nil {
		return false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	if _, ok := p.keys[fmt.Sprintf("%d_%d", pokemonID, formID)]; ok {
		return true
	}
	if _, ok := p.keys[fmt.Sprintf("%d", pokemonID)]; ok {
		return true
	}
	return false
}

func (p *Possible) refresh(cachePath string) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(p.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("bad status %d", resp.StatusCode)
	}
	var payload payload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return err
	}
	p.update(payload)
	if cachePath != "" {
		_ = p.saveCache(cachePath, payload)
	}
	return nil
}

func (p *Possible) update(payload payload) {
	next := map[string]struct{}{}
	for key := range payload.Map {
		next[key] = struct{}{}
	}
	p.mu.Lock()
	p.keys = next
	p.mu.Unlock()
}

func (p *Possible) loadCache(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var payload payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	p.update(payload)
	return nil
}

func (p *Possible) saveCache(path string, payload payload) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}
