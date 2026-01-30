package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type cleanEntry struct {
	Type      string `json:"type"`
	Target    string `json:"target"`
	MessageID string `json:"message_id"`
	DeleteAt  int64  `json:"delete_at"`
}

type cleanCache struct {
	path    string
	mu      sync.Mutex
	entries map[string]cleanEntry
}

func newCleanCache(path string) *cleanCache {
	return &cleanCache{
		path:    path,
		entries: map[string]cleanEntry{},
	}
}

func (c *cleanCache) key(entry cleanEntry) string {
	return entry.Type + "|" + entry.Target + "|" + entry.MessageID
}

func (c *cleanCache) Add(entry cleanEntry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.entries[c.key(entry)] = entry
	c.mu.Unlock()
}

func (c *cleanCache) Remove(entry cleanEntry) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, c.key(entry))
	c.mu.Unlock()
}

func (c *cleanCache) Load() ([]cleanEntry, error) {
	if c == nil {
		return nil, nil
	}
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return nil, err
	}
	var entries map[string]cleanEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.entries = entries
	c.mu.Unlock()
	out := make([]cleanEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, entry)
	}
	return out, nil
}

func (c *cleanCache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	payload := make(map[string]cleanEntry, len(c.entries))
	for key, entry := range c.entries {
		payload[key] = entry
	}
	c.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(c.path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, raw, 0o644)
}
