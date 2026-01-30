package webhook

import (
	"fmt"
	"sync"

	"poraclego/internal/config"
	"poraclego/internal/db"
	"poraclego/internal/logging"
)

type monsterQuery interface {
	SelectAllQuery(table string, conditions map[string]any) ([]map[string]any, error)
}

// MonsterAlarmCache mirrors PoracleJS's monsterAlarmMatch cache behavior: it preloads the `monsters`
// tracking table and can be refreshed on demand (e.g. via /api/tracking/pokemon/refresh).
//
// It is intentionally minimal (stores raw rows) — the main goal is to avoid repeated DB reads and to
// match the "refresh alerts" semantics in PoracleJS.
type MonsterAlarmCache struct {
	mu        sync.RWMutex
	rows      []map[string]any
	refreshMu sync.Mutex
	refreshing bool
}

func NewMonsterAlarmCache() *MonsterAlarmCache {
	return &MonsterAlarmCache{}
}

func (c *MonsterAlarmCache) Rows() []map[string]any {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.rows
}

func (c *MonsterAlarmCache) Set(rows []map[string]any) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.rows = rows
	c.mu.Unlock()
}

func (c *MonsterAlarmCache) Refresh(cfg *config.Config, query monsterQuery) error {
	if c == nil {
		return nil
	}
	if query == nil {
		return fmt.Errorf("monster cache missing query")
	}
	if cfg != nil {
		fast, _ := cfg.GetBool("tuning.fastMonsters")
		if !fast {
			return nil
		}
	}
	rows, err := query.SelectAllQuery("monsters", map[string]any{})
	if err != nil {
		return err
	}
	c.Set(rows)
	return nil
}

func (c *MonsterAlarmCache) RefreshAsync(cfg *config.Config, query *db.Query) {
	if c == nil || query == nil {
		return
	}
	if cfg != nil {
		fast, _ := cfg.GetBool("tuning.fastMonsters")
		if !fast {
			return
		}
	}
	c.refreshMu.Lock()
	if c.refreshing {
		c.refreshMu.Unlock()
		return
	}
	c.refreshing = true
	c.refreshMu.Unlock()

	go func() {
		defer func() {
			c.refreshMu.Lock()
			c.refreshing = false
			c.refreshMu.Unlock()
		}()
		if err := c.Refresh(cfg, query); err != nil {
			if logger := logging.Get().Webhooks; logger != nil {
				logger.Warnf("monster alarm cache refresh failed: %v", err)
			}
			return
		}
		if logger := logging.Get().Webhooks; logger != nil {
			logger.Infof("Refreshed monster alarm cache")
		}
	}()
}
