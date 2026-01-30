package webhook

// RefreshAlertCacheAsync refreshes in-memory caches that affect matching behavior.
// Currently this mirrors PoracleJS's /api/tracking/pokemon/refresh behavior by reloading
// the monster alarm cache when tuning.fastMonsters is enabled.
func (p *Processor) RefreshAlertCacheAsync() {
	if p == nil || p.monsterCache == nil {
		return
	}
	p.monsterCache.RefreshAsync(p.cfg, p.query)
}

