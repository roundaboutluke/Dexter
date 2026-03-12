package webhook

import (
	"os"
	"path/filepath"
	"time"

	"poraclego/internal/data"
	"poraclego/internal/dts"
	"poraclego/internal/pvp"
)

func (p *Processor) startCachePruner() {
	if p == nil {
		return
	}
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		if p.cache != nil {
			p.cache.PruneExpired(now)
		}
		p.pruneRaidSeen(now)
	}
}

func (p *Processor) pruneRaidSeen(now time.Time) {
	if p == nil {
		return
	}
	p.raidSeenMu.Lock()
	defer p.raidSeenMu.Unlock()
	for key, entry := range p.raidSeen {
		if entry.Expires.IsZero() {
			continue
		}
		if now.After(entry.Expires) {
			delete(p.raidSeen, key)
		}
	}
}

// UpdateTemplates replaces the DTS template list.
func (p *Processor) UpdateTemplates(templates []dts.Template) {
	if p == nil {
		return
	}
	p.templates = templates
}

// SaveCaches persists cache data to disk for warm restarts.
func (p *Processor) SaveCaches() {
	if p == nil || p.cacheDir == "" {
		return
	}
	_ = os.MkdirAll(p.cacheDir, 0o755)
	if p.gymCache != nil {
		_ = saveJSONFile(filepath.Join(p.cacheDir, "gymCache.json"), p.gymCache.Snapshot())
	}
	if p.geocoder != nil {
		_ = p.geocoder.SaveCache(filepath.Join(p.cacheDir, "geocoderCache.json"))
	}
	if p.weather != nil {
		_ = p.weather.SaveCache(filepath.Join(p.cacheDir, "weatherCache.json"))
	}
	if p.weatherData != nil {
		p.weatherData.SaveCaches()
	}
	if p.monsterChange != nil {
		p.monsterChange.SaveCache()
	}
}

func (p *Processor) loadCaches() {
	if p == nil || p.cacheDir == "" {
		return
	}
	_ = os.MkdirAll(p.cacheDir, 0o755)
	if p.gymCache != nil {
		var payload map[string]GymState
		if err := loadJSONFile(filepath.Join(p.cacheDir, "gymCache.json"), &payload); err == nil {
			p.gymCache.Load(payload)
		}
	}
	if p.geocoder != nil {
		p.geocoder.LoadCache(filepath.Join(p.cacheDir, "geocoderCache.json"))
	}
	if p.weather != nil {
		p.weather.LoadCache(filepath.Join(p.cacheDir, "weatherCache.json"))
	}
	if p.monsterChange != nil {
		p.monsterChange.LoadCache()
	}
}

func cacheDir(root string) string {
	if root == "" {
		return ""
	}
	return filepath.Join(root, ".cache")
}

// UpdateData replaces the game data set used for webhook processing.
func (p *Processor) UpdateData(game *data.GameData) {
	if p == nil || game == nil {
		return
	}
	p.data = game
	p.pvpCalc = pvp.NewCalculator(p.cfg, game)
}
