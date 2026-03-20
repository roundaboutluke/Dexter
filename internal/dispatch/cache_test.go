package dispatch

import (
	"path/filepath"
	"testing"
	"time"
)

func TestCleanCache_AddSaveLoadRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.json")
	c := newCleanCache(path)

	entry := cleanEntry{
		Type:      "discord:channel",
		Target:    "12345",
		MessageID: "msg1",
		DeleteAt:  time.Now().Add(time.Hour).UnixMilli(),
	}
	c.Add(entry)

	if err := c.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	c2 := newCleanCache(path)
	loaded, err := c2.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded %d entries, want 1", len(loaded))
	}
	if loaded[0].MessageID != "msg1" {
		t.Errorf("MessageID = %q, want %q", loaded[0].MessageID, "msg1")
	}

	c2.Remove(entry)
	c2.mu.Lock()
	count := len(c2.entries)
	c2.mu.Unlock()
	if count != 0 {
		t.Errorf("entries after Remove = %d, want 0", count)
	}
}

func TestUpdateCache_SetGetSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update.json")
	c := newUpdateCache(path)

	entry := updateEntry{
		Key:       "pokemon:25",
		Target:    "user1",
		MessageID: "msg1",
		ChannelID: "ch1",
		DeleteAt:  time.Now().Add(time.Hour).UnixMilli(),
	}
	c.Set(entry)

	got, ok := c.Get("user1", "pokemon:25")
	if !ok {
		t.Fatal("expected entry from Get")
	}
	if got.MessageID != "msg1" {
		t.Errorf("MessageID = %q, want %q", got.MessageID, "msg1")
	}

	if err := c.Save(); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	c2 := newUpdateCache(path)
	if err := c2.Load(); err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	got2, ok := c2.Get("user1", "pokemon:25")
	if !ok {
		t.Fatal("expected entry after Load")
	}
	if got2.ChannelID != "ch1" {
		t.Errorf("ChannelID = %q, want %q", got2.ChannelID, "ch1")
	}
}

func TestUpdateCache_Remove(t *testing.T) {
	dir := t.TempDir()
	c := newUpdateCache(filepath.Join(dir, "update.json"))
	c.Set(updateEntry{Key: "k1", Target: "t1", MessageID: "m1"})
	c.Remove("t1", "k1")
	_, ok := c.Get("t1", "k1")
	if ok {
		t.Error("expected entry to be removed")
	}
}

func TestUpdateCache_SetIgnoresEmpty(t *testing.T) {
	c := newUpdateCache("/dev/null")
	c.Set(updateEntry{Key: "", Target: "t1", MessageID: "m1"})
	c.Set(updateEntry{Key: "k1", Target: "", MessageID: "m1"})
	c.Set(updateEntry{Key: "k1", Target: "t1", MessageID: ""})
	c.mu.Lock()
	count := len(c.entries)
	c.mu.Unlock()
	if count != 0 {
		t.Errorf("entries = %d, want 0 for empty fields", count)
	}
}

func TestCleanCache_NilSafety(t *testing.T) {
	var c *cleanCache
	c.Add(cleanEntry{})
	c.Remove(cleanEntry{})
	if err := c.Save(); err != nil {
		t.Errorf("Save on nil = %v", err)
	}
	entries, err := c.Load()
	if err != nil {
		t.Errorf("Load on nil = %v", err)
	}
	if entries != nil {
		t.Errorf("Load on nil returned %v", entries)
	}
}
