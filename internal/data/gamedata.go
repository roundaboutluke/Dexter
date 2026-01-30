package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// GameData mirrors PoracleJS util data loading.
type GameData struct {
	UtilData    map[string]any
	Monsters    map[string]any
	Moves       map[string]any
	Items       map[string]any
	Grunts      map[string]any
	QuestTypes  map[string]any
	Types       map[string]any
	Translations map[string]any
}

var requiredFiles = []string{"monsters", "moves", "items", "grunts", "questTypes", "types", "translations"}

// Load reads util JSON data from disk.
func Load(root string) (*GameData, error) {
	data := &GameData{}
	utilPath := filepath.Join(root, "util", "util.json")
	if err := loadJSON(utilPath, &data.UtilData); err != nil {
		return nil, fmt.Errorf("load util.json: %w", err)
	}

	for _, name := range requiredFiles {
		path := filepath.Join(root, "util", fmt.Sprintf("%s.json", name))
		var payload map[string]any
		if err := loadJSON(path, &payload); err != nil {
			return nil, fmt.Errorf("load %s.json: %w", name, err)
		}
		switch name {
		case "monsters":
			data.Monsters = payload
		case "moves":
			data.Moves = payload
		case "items":
			data.Items = payload
		case "grunts":
			data.Grunts = payload
		case "questTypes":
			data.QuestTypes = payload
		case "types":
			data.Types = payload
		case "translations":
			data.Translations = payload
		}
	}

	return data, nil
}

func loadJSON(path string, target any) error {
	payload, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(payload, target)
}
