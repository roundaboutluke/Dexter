package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDefaultFiles creates local.json and default config files when missing.
func EnsureDefaultFiles(root string) error {
	base := filepath.Join(root, "config")
	if dir := os.Getenv("NODE_CONFIG_DIR"); dir != "" {
		if filepath.IsAbs(dir) {
			base = dir
		} else {
			base = filepath.Join(root, dir)
		}
	}

	defaultPath := filepath.Join(base, "default.json")
	localPath := filepath.Join(base, "local.json")
	if !fileExists(localPath) && fileExists(defaultPath) {
		raw, err := os.ReadFile(defaultPath)
		if err != nil {
			return fmt.Errorf("read default.json: %w", err)
		}
		if err := os.WriteFile(localPath, raw, 0o644); err != nil {
			return fmt.Errorf("write local.json: %w", err)
		}
	}

	defaultFiles := []string{"dts.json", "geofence.json", "pokemonAlias.json", "partials.json", "testdata.json"}
	defaultsDir := filepath.Join(base, "defaults")
	for _, name := range defaultFiles {
		localFile := filepath.Join(base, name)
		if fileExists(localFile) {
			continue
		}
		sourceFile := filepath.Join(defaultsDir, name)
		if !fileExists(sourceFile) {
			continue
		}
		raw, err := os.ReadFile(sourceFile)
		if err != nil {
			return fmt.Errorf("read default %s: %w", name, err)
		}
		if err := os.WriteFile(localFile, raw, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}
