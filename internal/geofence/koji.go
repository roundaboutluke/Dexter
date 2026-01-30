package geofence

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"poraclego/internal/config"
)

// FetchKojiFences pulls remote geofences into the local cache folder.
func FetchKojiFences(cfg *config.Config, root string, logger func(string, ...any)) {
	if cfg == nil {
		return
	}
	if logger == nil {
		logger = func(string, ...any) {}
	}
	token, _ := cfg.GetString("geofence.kojiOptions.bearerToken")
	if strings.TrimSpace(token) == "" {
		logger("[KŌJI] Kōji bearer token not found, skipping")
		return
	}
	rawPaths, ok := cfg.Get("geofence.path")
	if !ok {
		return
	}
	urls := coercePathList(rawPaths)
	client := &http.Client{Timeout: 20 * time.Second}
	cacheDir := filepath.Join(root, ".cache")
	_ = os.MkdirAll(cacheDir, 0o755)

	for _, original := range urls {
		if !strings.HasPrefix(original, "http") {
			continue
		}
		cacheName := strings.ReplaceAll(original, "/", "__") + ".json"
		path := filepath.Join(cacheDir, cacheName)
		logger("[KŌJI] Fetching %s...", original)
		req, err := http.NewRequest("GET", original, nil)
		if err != nil {
			logger("[KŌJI] Could not process %s", original)
			continue
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			logger("[KŌJI] Could not process %s", original)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logger("[KŌJI] Could not process %s", original)
			continue
		}
		var payload struct {
			Data any `json:"data"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			logger("[KŌJI] Could not process %s", original)
			continue
		}
		if payload.Data == nil {
			continue
		}
		raw, err := json.MarshalIndent(payload.Data, "", "  ")
		if err != nil {
			logger("[KŌJI] Could not process %s", original)
			continue
		}
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			logger("[KŌJI] Could not process %s", original)
			continue
		}
	}
}

func coercePathList(raw any) []string {
	switch v := raw.(type) {
	case string:
		if v != "" {
			return []string{v}
		}
	case []any:
		out := []string{}
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	}
	return nil
}
