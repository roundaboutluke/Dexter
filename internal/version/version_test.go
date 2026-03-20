package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead_FileExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte("4.8.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Read(dir)
	if got != "4.8.4" {
		t.Errorf("Read() = %q, want %q", got, "4.8.4")
	}
}

func TestRead_FileMissing(t *testing.T) {
	dir := t.TempDir()
	got := Read(dir)
	if got != "dev" {
		t.Errorf("Read() = %q, want %q", got, "dev")
	}
}

func TestRead_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "VERSION"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Read(dir)
	if got != "dev" {
		t.Errorf("Read() = %q, want %q", got, "dev")
	}
}
