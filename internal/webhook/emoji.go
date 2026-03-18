package webhook

import (
	"encoding/json"
	"os"
	"path/filepath"

	"dexter/internal/config"
)

func loadCustomEmoji(root string) map[string]map[string]string {
	path := filepath.Join(configDir(root), "emoji.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	clean := config.StripJSONComments(raw)
	var payload map[string]map[string]string
	if err := json.Unmarshal(clean, &payload); err != nil {
		return nil
	}
	return payload
}
