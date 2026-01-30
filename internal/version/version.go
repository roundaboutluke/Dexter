package version

import (
	"os"
	"path/filepath"
	"strings"
)

// Read returns the version string stored in VERSION, or "dev".
func Read(root string) string {
	path := filepath.Join(root, "VERSION")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "dev"
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		return "dev"
	}
	return value
}
