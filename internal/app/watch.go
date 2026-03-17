package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"poraclego/internal/config"
	"poraclego/internal/data"
	"poraclego/internal/dts"
	"poraclego/internal/geofence"
)

func (a *App) startWatchers(ctx context.Context, root string) {
	watchInterval := 5 * time.Second
	go a.watchDTS(ctx, root, watchInterval)
	go a.watchGeofence(ctx, root, watchInterval)
	go a.watchGameData(ctx, root, watchInterval)
	go a.refreshGameData(ctx, root, 6*time.Hour)
}

func (a *App) watchDTS(ctx context.Context, root string, interval time.Duration) {
	known := map[string]time.Time{}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			paths := dtsPaths(root)
			changed := syncTimestamps(known, paths)
			if changed {
				templates, err := dts.Load(root)
				if err != nil {
					logf("DTS reload failed: %v", err)
					continue
				}
				a.dts = templates
				if a.processor != nil {
					a.processor.UpdateTemplates(templates)
				}
				if a.server != nil {
					a.server.UpdateTemplates(templates)
				}
				if a.botManager != nil {
					a.botManager.UpdateTemplates(templates)
				}
				if a.profileSchedule != nil {
					a.profileSchedule.UpdateTemplates(templates)
				}
				logf("DTS reloaded")
			}
		}
	}
}

func (a *App) watchGeofence(ctx context.Context, root string, interval time.Duration) {
	known := map[string]time.Time{}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			paths := geofencePaths(a.config, root)
			changed := syncTimestamps(known, paths)
			if changed {
				store, err := geofence.Load(a.config, root)
				if err != nil {
					logf("Geofence reload failed: %v", err)
					continue
				}
				if a.fences != nil {
					a.fences.Replace(store.Fences)
				}
				if a.processor != nil {
					a.processor.UpdateFences(store)
					a.processor.RefreshAlertCacheAsync()
				}
				logf("Geofence reloaded")
			}
		}
	}
}

func (a *App) watchGameData(ctx context.Context, root string, interval time.Duration) {
	known := map[string]time.Time{}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			paths := gameDataPaths(root)
			changed := syncTimestamps(known, paths)
			if changed {
				game, err := data.Load(root)
				if err != nil {
					logf("Game data reload failed: %v", err)
					continue
				}
				a.data = game
				if a.processor != nil {
					a.processor.UpdateData(game)
				}
				if a.server != nil {
					a.server.UpdateData(game)
				}
				if a.botManager != nil {
					a.botManager.UpdateData(game)
				}
				logf("Game data reloaded")
			}
		}
	}
}

func (a *App) refreshGameData(ctx context.Context, root string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			data.GenerateBestEffort(root, true, logf)
		}
	}
}

func dtsPaths(root string) []string {
	base := filepath.Join(root, "config")
	if dir := os.Getenv("NODE_CONFIG_DIR"); dir != "" {
		if filepath.IsAbs(dir) {
			base = dir
		} else {
			base = filepath.Join(root, dir)
		}
	}
	paths := []string{filepath.Join(base, "dts.json")}
	dir := filepath.Join(base, "dts")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return paths
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	return paths
}

func geofencePaths(cfg *config.Config, root string) []string {
	if cfg == nil {
		return nil
	}
	raw, ok := cfg.Get("geofence.path")
	if !ok {
		return nil
	}
	paths := []string{}
	switch v := raw.(type) {
	case string:
		if v != "" {
			paths = append(paths, v)
		}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				paths = append(paths, s)
			}
		}
	case []string:
		paths = append(paths, v...)
	}
	resolved := []string{}
	for _, path := range paths {
		if path == "" {
			continue
		}
		if filepath.IsAbs(path) {
			resolved = append(resolved, path)
			continue
		}
		if len(path) >= 4 && path[:4] == "http" {
			cacheName := filepath.Join(root, ".cache", strings.ReplaceAll(path, "/", "__")+".json")
			resolved = append(resolved, cacheName)
			continue
		}
		resolved = append(resolved, filepath.Join(root, path))
	}
	return resolved
}

func gameDataPaths(root string) []string {
	base := filepath.Join(root, "util")
	files := []string{
		"util.json",
		"monsters.json",
		"moves.json",
		"items.json",
		"grunts.json",
		"questTypes.json",
		"types.json",
		"translations.json",
	}
	paths := make([]string, 0, len(files))
	for _, name := range files {
		paths = append(paths, filepath.Join(base, name))
	}
	return paths
}

func syncTimestamps(known map[string]time.Time, paths []string) bool {
	changed := false
	current := map[string]time.Time{}
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		mod := info.ModTime()
		current[path] = mod
		if prev, ok := known[path]; !ok || !prev.Equal(mod) {
			changed = true
		}
	}
	if len(current) != len(known) {
		changed = true
	}
	for key := range known {
		if _, ok := current[key]; !ok {
			changed = true
		}
	}
	for key, value := range current {
		known[key] = value
	}
	for key := range known {
		if _, ok := current[key]; !ok {
			delete(known, key)
		}
	}
	return changed
}
