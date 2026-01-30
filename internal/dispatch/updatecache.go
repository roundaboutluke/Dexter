package dispatch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type updateEntry struct {
	Key       string `json:"key"`
	Target    string `json:"target"`
	MessageID string `json:"message_id"`
	ChannelID string `json:"channel_id"`
	DeleteAt  int64  `json:"delete_at"`
}

type updateCache struct {
	path    string
	mu      sync.Mutex
	entries map[string]updateEntry
}

func newUpdateCache(path string) *updateCache {
	return &updateCache{
		path:    path,
		entries: map[string]updateEntry{},
	}
}

func (c *updateCache) key(target, key string) string {
	return target + "|" + key
}

func (c *updateCache) Get(target, key string) (updateEntry, bool) {
	if c == nil {
		return updateEntry{}, false
	}
	c.mu.Lock()
	entry, ok := c.entries[c.key(target, key)]
	c.mu.Unlock()
	return entry, ok
}

func (c *updateCache) Set(entry updateEntry) {
	if c == nil || entry.Target == "" || entry.Key == "" || entry.MessageID == "" {
		return
	}
	c.mu.Lock()
	c.entries[c.key(entry.Target, entry.Key)] = entry
	c.mu.Unlock()
}

func (c *updateCache) Remove(target, key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	delete(c.entries, c.key(target, key))
	c.mu.Unlock()
}

func (c *updateCache) Load() error {
	if c == nil {
		return nil
	}
	raw, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	var entries map[string]updateEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return err
	}
	now := time.Now().UnixMilli()
	for key, entry := range entries {
		if entry.DeleteAt > 0 && entry.DeleteAt <= now {
			delete(entries, key)
		}
	}
	c.mu.Lock()
	c.entries = entries
	c.mu.Unlock()
	return nil
}

func (c *updateCache) Save() error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	payload := make(map[string]updateEntry, len(c.entries))
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
