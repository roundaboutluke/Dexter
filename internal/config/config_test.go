package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew_NilMap(t *testing.T) {
	cfg := New(nil)
	if cfg == nil {
		t.Fatal("New(nil) returned nil")
	}
	if _, ok := cfg.Get("anything"); ok {
		t.Error("expected missing key to return false")
	}
}

func TestGet_DottedPath(t *testing.T) {
	cfg := New(map[string]any{
		"database": map[string]any{
			"conn": map[string]any{
				"host": "localhost",
			},
		},
	})
	val, ok := cfg.Get("database.conn.host")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if val != "localhost" {
		t.Errorf("got %v, want %q", val, "localhost")
	}
}

func TestGet_MissingKey(t *testing.T) {
	cfg := New(map[string]any{})
	_, ok := cfg.Get("not.here")
	if ok {
		t.Error("expected ok=false for missing key")
	}
}

func TestGetString_FromString(t *testing.T) {
	cfg := New(map[string]any{"name": "dexter"})
	val, ok := cfg.GetString("name")
	if !ok || val != "dexter" {
		t.Errorf("GetString() = (%q, %v), want (%q, true)", val, ok, "dexter")
	}
}

func TestGetString_FromFloat64(t *testing.T) {
	cfg := New(map[string]any{"port": float64(3030)})
	val, ok := cfg.GetString("port")
	if !ok || val != "3030" {
		t.Errorf("GetString() = (%q, %v), want (%q, true)", val, ok, "3030")
	}
}

func TestGetString_FromBool(t *testing.T) {
	cfg := New(map[string]any{"enabled": true})
	val, ok := cfg.GetString("enabled")
	if !ok || val != "true" {
		t.Errorf("GetString() = (%q, %v), want (%q, true)", val, ok, "true")
	}
}

func TestGetBool_FromBool(t *testing.T) {
	cfg := New(map[string]any{"flag": true})
	val, ok := cfg.GetBool("flag")
	if !ok || !val {
		t.Errorf("GetBool() = (%v, %v), want (true, true)", val, ok)
	}
}

func TestGetBool_FromString(t *testing.T) {
	cfg := New(map[string]any{"flag": "true"})
	val, ok := cfg.GetBool("flag")
	if !ok || !val {
		t.Errorf("GetBool() = (%v, %v), want (true, true)", val, ok)
	}
}

func TestGetBool_FromFloat64(t *testing.T) {
	cfg := New(map[string]any{"flag": float64(1)})
	val, ok := cfg.GetBool("flag")
	if !ok || !val {
		t.Errorf("GetBool() = (%v, %v), want (true, true)", val, ok)
	}
}

func TestGetInt_FromFloat64(t *testing.T) {
	cfg := New(map[string]any{"port": float64(3030)})
	val, ok := cfg.GetInt("port")
	if !ok || val != 3030 {
		t.Errorf("GetInt() = (%d, %v), want (3030, true)", val, ok)
	}
}

func TestGetInt_FromString(t *testing.T) {
	cfg := New(map[string]any{"port": "8080"})
	val, ok := cfg.GetInt("port")
	if !ok || val != 8080 {
		t.Errorf("GetInt() = (%d, %v), want (8080, true)", val, ok)
	}
}

func TestGetInt_Missing(t *testing.T) {
	cfg := New(map[string]any{})
	val, ok := cfg.GetInt("port")
	if ok || val != 0 {
		t.Errorf("GetInt() = (%d, %v), want (0, false)", val, ok)
	}
}

func TestGetStringSlice_FromSliceAny(t *testing.T) {
	cfg := New(map[string]any{"items": []any{"a", "b", "c"}})
	val, ok := cfg.GetStringSlice("items")
	if !ok || len(val) != 3 || val[0] != "a" || val[1] != "b" || val[2] != "c" {
		t.Errorf("GetStringSlice() = (%v, %v), want ([a b c], true)", val, ok)
	}
}

func TestGetStringSlice_FromSingleString(t *testing.T) {
	cfg := New(map[string]any{"items": "solo"})
	val, ok := cfg.GetStringSlice("items")
	if !ok || len(val) != 1 || val[0] != "solo" {
		t.Errorf("GetStringSlice() = (%v, %v), want ([solo], true)", val, ok)
	}
}

func TestStripJSONComments_SingleLine(t *testing.T) {
	input := []byte(`{
  // this is a comment
  "key": "value"
}`)
	got := string(StripJSONComments(input))
	// Single-line comments are stripped but newline preserved; leading spaces on comment line remain stripped.
	if want := "{\n  \n  \"key\": \"value\"\n}"; got != want {
		t.Errorf("StripJSONComments() = %q, want %q", got, want)
	}
}

func TestStripJSONComments_MultiLine(t *testing.T) {
	input := []byte(`{
  /* block
     comment */
  "key": "value"
}`)
	got := string(StripJSONComments(input))
	if want := "{\n  \n  \"key\": \"value\"\n}"; got != want {
		t.Errorf("StripJSONComments() = %q, want %q", got, want)
	}
}

func TestStripJSONComments_PreservesURLs(t *testing.T) {
	input := []byte(`{"url": "https://example.com"}`)
	got := string(StripJSONComments(input))
	if got != `{"url": "https://example.com"}` {
		t.Errorf("StripJSONComments() = %q", got)
	}
}

func TestLoad_Basic(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "default.json"), []byte(`{"server": {"port": 3030}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NODE_CONFIG_DIR", "")
	t.Setenv("NODE_CONFIG_ENV", "test-nonexistent")
	t.Setenv("NODE_CONFIG", "")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	port, ok := cfg.GetInt("server.port")
	if !ok || port != 3030 {
		t.Errorf("GetInt(server.port) = (%d, %v), want (3030, true)", port, ok)
	}
}
