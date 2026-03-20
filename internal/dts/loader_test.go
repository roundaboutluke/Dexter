package dts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Basic(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NODE_CONFIG_DIR", "")

	templates := []Template{
		{Platform: "discord", Type: "monster", ID: "default"},
	}
	raw, _ := json.Marshal(templates)
	if err := os.WriteFile(filepath.Join(configDir, "dts.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d templates, want 1", len(got))
	}
	if got[0].Platform != "discord" || got[0].Type != "monster" {
		t.Errorf("template = %+v, unexpected", got[0])
	}
}

func TestLoad_MergesExtra(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	dtsDir := filepath.Join(configDir, "dts")
	if err := os.MkdirAll(dtsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NODE_CONFIG_DIR", "")

	base := []Template{{Platform: "discord", Type: "monster", ID: "base"}}
	extra := []Template{{Platform: "telegram", Type: "raid", ID: "extra"}}
	baseRaw, _ := json.Marshal(base)
	extraRaw, _ := json.Marshal(extra)
	os.WriteFile(filepath.Join(configDir, "dts.json"), baseRaw, 0o644)
	os.WriteFile(filepath.Join(dtsDir, "raids.json"), extraRaw, 0o644)

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d templates, want 2", len(got))
	}
}

func TestLoad_WithComments(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NODE_CONFIG_DIR", "")

	content := `[
  // A template entry
  {"platform": "discord", "type": "quest", "id": "default"}
]`
	os.WriteFile(filepath.Join(configDir, "dts.json"), []byte(content), 0o644)

	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d templates, want 1", len(got))
	}
}
