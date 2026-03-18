package webhook

import (
	"os"
	"path/filepath"
	"time"

	"dexter/internal/data"
	"dexter/internal/dts"
	"dexter/internal/geofence"
	"dexter/internal/logging"
	"dexter/internal/pvp"
)

func (p *Processor) startCachePruner() {
	if p == nil {
		return
	}
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			if p.cache != nil {
				p.cache.PruneExpired(now)
			}
			p.pruneRaidSeen(now)
			if p.gymCache != nil {
				p.gymCache.PruneStale(now.Add(-24 * time.Hour))
			}
			if p.geocoder != nil {
				p.geocoder.PruneStale(now.Add(-24 * time.Hour))
			}
			if p.weatherData != nil {
				p.weatherData.PruneStaleCares(now.Unix())
			}
		case <-p.stopCh:
			return
		}
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

// UpdateTemplates replaces the DTS template list atomically.
func (p *Processor) UpdateTemplates(templates []dts.Template) {
	if p == nil {
		return
	}
	p.templates.Store(&templates)
}

// Stop signals background goroutines to exit.
func (p *Processor) Stop() {
	if p == nil {
		return
	}
	select {
	case <-p.stopCh:
	default:
		close(p.stopCh)
	}
}

// SaveCaches persists cache data to disk for warm restarts.
func (p *Processor) SaveCaches() {
	if p == nil || p.cacheDir == "" {
		return
	}
	if err := os.MkdirAll(p.cacheDir, 0o755); err != nil {
		if logger := logging.Get().Webhooks; logger != nil {
			logger.Warnf("failed to create cache dir %s: %v", p.cacheDir, err)
		}
		return
	}
	if p.gymCache != nil {
		if err := saveJSONFile(filepath.Join(p.cacheDir, "gymCache.json"), p.gymCache.Snapshot()); err != nil {
			if logger := logging.Get().Webhooks; logger != nil {
				logger.Warnf("failed to save gym cache: %v", err)
			}
		}
	}
	if p.geocoder != nil {
		if err := p.geocoder.SaveCache(filepath.Join(p.cacheDir, "geocoderCache.json")); err != nil {
			if logger := logging.Get().Webhooks; logger != nil {
				logger.Warnf("failed to save geocoder cache: %v", err)
			}
		}
	}
	if p.weather != nil {
		if err := p.weather.SaveCache(filepath.Join(p.cacheDir, "weatherCache.json")); err != nil {
			if logger := logging.Get().Webhooks; logger != nil {
				logger.Warnf("failed to save weather cache: %v", err)
			}
		}
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

// UpdateData replaces the game data set used for webhook processing atomically.
func (p *Processor) UpdateData(game *data.GameData) {
	if p == nil || game == nil {
		return
	}
	p.data.Store(game)
	p.pvpCalc.Store(pvp.NewCalculator(p.cfg, game))
}

// UpdateFences replaces the geofence store atomically.
func (p *Processor) UpdateFences(store *geofence.Store) {
	if p == nil || store == nil {
		return
	}
	p.fences.Store(store)
}
