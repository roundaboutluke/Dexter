package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

const (
	gameMasterURL = "https://raw.githubusercontent.com/WatWowMap/Masterfile-Generator/master/master-latest-poracle-v2.json"
	gruntsURL     = "https://raw.githubusercontent.com/WatWowMap/event-info/main/grunts/formatted.json"
	localesIndex  = "https://raw.githubusercontent.com/WatWowMap/pogo-translations/master/index.json"
	localesBase   = "https://raw.githubusercontent.com/WatWowMap/pogo-translations/master/static/enRefMerged/"
)

// Generate downloads game data and writes it to the util folder.
func Generate(root string, latest bool, logger func(string, ...any)) error {
	if logger == nil {
		logger = func(string, ...any) {}
	}
	utilDir := filepath.Join(root, "util")
	localeDir := filepath.Join(utilDir, "locale")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		return fmt.Errorf("create util dir: %w", err)
	}

	client := &http.Client{Timeout: 20 * time.Second}
	logger("Fetching latest Game Master...")
	var gameMaster map[string]any
	if err := fetchJSON(client, gameMasterURL, &gameMaster); err != nil {
		return fmt.Errorf("fetch game master: %w", err)
	}
	logger("Writing Game Master files...")
	updated := 0
	for key, value := range gameMaster {
		path := filepath.Join(utilDir, fmt.Sprintf("%s.json", key))
		changed, err := writeJSON(path, value)
		if err != nil {
			return fmt.Errorf("write %s: %w", key, err)
		}
		if changed {
			updated++
		}
	}

	if latest {
		logger("Fetching latest grunts...")
		var grunts any
		if err := fetchJSON(client, gruntsURL, &grunts); err != nil {
			return fmt.Errorf("fetch grunts: %w", err)
		}
		changed, err := writeJSON(filepath.Join(utilDir, "grunts.json"), grunts)
		if err != nil {
			return fmt.Errorf("write grunts: %w", err)
		}
		if changed {
			updated++
		}
	}

	logger("Fetching locale index...")
	var locales []string
	if err := fetchJSON(client, localesIndex, &locales); err != nil {
		return fmt.Errorf("fetch locales index: %w", err)
	}
	for _, locale := range locales {
		url := localesBase + locale
		var payload any
		if err := fetchJSON(client, url, &payload); err != nil {
			return fmt.Errorf("fetch locale %s: %w", locale, err)
		}
		path := filepath.Join(localeDir, locale)
		changed, err := writeJSON(path, payload)
		if err != nil {
			return fmt.Errorf("write locale %s: %w", locale, err)
		}
		if changed {
			updated++
		}
	}

	if updated == 0 {
		logger("Data unchanged.")
	} else {
		logger("Data updated: %d files.", updated)
	}
	logger("Data generation complete.")
	return nil
}

// GenerateBestEffort attempts to update data and continues on failures.
func GenerateBestEffort(root string, latest bool, logger func(string, ...any)) {
	if logger == nil {
		logger = func(string, ...any) {}
	}
	utilDir := filepath.Join(root, "util")
	localeDir := filepath.Join(utilDir, "locale")
	if err := os.MkdirAll(localeDir, 0o755); err != nil {
		logger("Data generation failed: %v", err)
		return
	}

	client := &http.Client{Timeout: 20 * time.Second}
	logger("Fetching latest Game Master...")
	updated := 0
	var gameMaster map[string]any
	if err := fetchJSON(client, gameMasterURL, &gameMaster); err != nil {
		logger("Could not fetch latest GM, using existing...")
	} else {
		logger("Writing Game Master files...")
		for key, value := range gameMaster {
			path := filepath.Join(utilDir, fmt.Sprintf("%s.json", key))
			changed, err := writeJSON(path, value)
			if err != nil {
				logger("Could not write %s: %v", key, err)
				continue
			}
			if changed {
				updated++
			}
		}
	}

	if latest {
		logger("Fetching latest grunts...")
		var grunts any
		if err := fetchJSON(client, gruntsURL, &grunts); err != nil {
			logger("Could not generate new invasions, using existing...")
		} else {
			changed, err := writeJSON(filepath.Join(utilDir, "grunts.json"), grunts)
			if err != nil {
				logger("Could not write grunts: %v", err)
			} else if changed {
				updated++
			}
		}
	}

	logger("Fetching locale index...")
	var locales []string
	if err := fetchJSON(client, localesIndex, &locales); err != nil {
		logger("Could not generate new locales, using existing...")
		return
	}
	for _, locale := range locales {
		url := localesBase + locale
		var payload any
		if err := fetchJSON(client, url, &payload); err != nil {
			logger("Could not process %s", locale)
			continue
		}
		path := filepath.Join(localeDir, locale)
		changed, err := writeJSON(path, payload)
		if err != nil {
			logger("Could not write locale %s: %v", locale, err)
			continue
		}
		if changed {
			updated++
		}
	}
	if updated == 0 {
		logger("Data unchanged.")
	} else {
		logger("Data updated: %d files.", updated)
	}
	logger("Data generation complete.")
}

func fetchJSON(client *http.Client, url string, target any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, target)
}

func writeJSON(path string, payload any) (bool, error) {
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return false, err
	}
	if existing, err := os.ReadFile(path); err == nil {
		if bytes.Equal(existing, raw) {
			return false, nil
		}
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-poraclego-*.json")
	if err != nil {
		return false, err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.Write(raw); err != nil {
		cleanup()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		cleanup()
		return false, err
	}
	if err := tmp.Chmod(0o644); err != nil {
		cleanup()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return false, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return false, err
	}
	return true, nil
}
