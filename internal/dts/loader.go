package dts

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dexter/internal/config"
)

// Template represents a Discord/Telegram template entry.
type Template struct {
	Platform string  `json:"platform"`
	Type     string  `json:"type"`
	Language *string `json:"language,omitempty"`
	ID       any     `json:"id"`
	Template any     `json:"template"`
	Hidden   bool    `json:"hidden"`
	Default  bool    `json:"default"`
}

// Load reads dts.json and config/dts/*.json.
func Load(root string) ([]Template, error) {
	base := configDir(root)
	dtsPath := filepath.Join(base, "dts.json")

	var templates []Template
	if err := loadJSON(dtsPath, &templates); err != nil {
		return nil, fmt.Errorf("%s - %w", dtsPath, err)
	}

	dirPath := filepath.Join(base, "dts")
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if os.IsNotExist(err) {
			return templates, nil
		}
		return nil, fmt.Errorf("%s - %w", dirPath, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
			continue
		}
		path := filepath.Join(dirPath, entry.Name())
		var addition []Template
		if err := loadJSON(path, &addition); err != nil {
			return nil, fmt.Errorf("%s - %w", path, err)
		}
		templates = append(templates, addition...)
	}

	return templates, nil
}

func configDir(root string) string {
	base := filepath.Join(root, "config")
	if dir := os.Getenv("NODE_CONFIG_DIR"); dir != "" {
		if filepath.IsAbs(dir) {
			base = dir
		} else {
			base = filepath.Join(root, dir)
		}
	}
	return base
}

func loadJSON(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	clean := config.StripJSONComments(raw)
	return json.Unmarshal(clean, target)
}
