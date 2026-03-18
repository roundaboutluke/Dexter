package tz

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/evanoberholster/timezoneLookup"

	"dexter/internal/config"
	"dexter/internal/logging"
)

type Locator struct {
	cfg      *config.Config
	root     string
	loadOnce sync.Once
	loadErr  error
	store    timezoneLookup.TimezoneInterface
	cacheMu  sync.Mutex
	cache    map[string]*time.Location
}

func NewLocator(cfg *config.Config, root string) *Locator {
	return &Locator{
		cfg:   cfg,
		root:  root,
		cache: map[string]*time.Location{},
	}
}

func (l *Locator) Location(lat, lon float64) (*time.Location, bool) {
	if l == nil || (lat == 0 && lon == 0) {
		return nil, false
	}
	l.ensureLoaded()
	if l.store == nil {
		return nil, false
	}
	key := fmt.Sprintf("%.4f,%.4f", lat, lon)
	l.cacheMu.Lock()
	if loc, ok := l.cache[key]; ok {
		l.cacheMu.Unlock()
		if loc == nil {
			return nil, false
		}
		return loc, true
	}
	l.cacheMu.Unlock()
	tzid, err := l.store.Query(timezoneLookup.Coord{
		Lat: float32(lat),
		Lon: float32(lon),
	})
	if err != nil || tzid == "" || tzid == "Error" {
		l.cacheMu.Lock()
		l.cache[key] = nil
		l.cacheMu.Unlock()
		return nil, false
	}
	loc, err := time.LoadLocation(tzid)
	if err != nil {
		l.cacheMu.Lock()
		l.cache[key] = nil
		l.cacheMu.Unlock()
		return nil, false
	}
	l.cacheMu.Lock()
	l.cache[key] = loc
	l.cacheMu.Unlock()
	return loc, true
}

func (l *Locator) ensureLoaded() {
	l.loadOnce.Do(func() {
		base := l.dbBasePath()
		if base == "" {
			l.loadErr = fmt.Errorf("timezone db path not configured")
			l.warn(l.loadErr)
			return
		}
		dbType := getString(l.cfg, "general.timezoneDbType", "boltdb")
		encStr := strings.ToLower(getString(l.cfg, "general.timezoneDbEncoding", "msgpack"))
		encoding, err := timezoneLookup.EncodingFromString(encStr)
		if err != nil {
			encoding = timezoneLookup.EncMsgPack
		}
		snappy := getBool(l.cfg, "general.timezoneDbSnappy", true)
		store, err := timezoneLookup.LoadTimezones(timezoneLookup.Config{
			DatabaseType: dbType,
			DatabaseName: base,
			Snappy:       snappy,
			Encoding:     encoding,
		})
		if err != nil {
			l.loadErr = err
			l.warn(err)
			return
		}
		l.store = store
	})
}

func (l *Locator) dbBasePath() string {
	path := getString(l.cfg, "general.timezoneDbPath", "")
	if path == "" && l.root != "" {
		path = filepath.Join(l.root, "internal", "tz", "timezone")
	}
	if path == "" {
		return ""
	}
	if !filepath.IsAbs(path) && l.root != "" {
		path = filepath.Join(l.root, path)
	}
	return normalizeBase(path)
}

func normalizeBase(path string) string {
	base := path
	base = strings.TrimSuffix(base, ".db")
	for {
		changed := false
		for _, suffix := range []string{".msgpack", ".json", ".protobuf", ".snap"} {
			if strings.HasSuffix(base, suffix) {
				base = strings.TrimSuffix(base, suffix)
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	return base
}

func (l *Locator) warn(err error) {
	if err == nil {
		return
	}
	if logger := logging.Get().General; logger != nil {
		logger.Warnf("timezone lookup disabled: %v", err)
	}
}

func getString(cfg *config.Config, path, fallback string) string {
	if cfg == nil {
		return fallback
	}
	value, ok := cfg.GetString(path)
	if !ok {
		return fallback
	}
	return value
}

func getBool(cfg *config.Config, path string, fallback bool) bool {
	if cfg == nil {
		return fallback
	}
	value, ok := cfg.GetBool(path)
	if !ok {
		return fallback
	}
	return value
}
