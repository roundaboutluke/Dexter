package command

import (
	"os"
	"path/filepath"
	"testing"
)

func shippedLocaleRoot(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if fileExists(filepath.Join(root, "go.mod")) && dirExists(filepath.Join(root, "locale")) {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("repo root not found from %q", root)
		}
		root = parent
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
