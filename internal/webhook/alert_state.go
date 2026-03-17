package webhook

import (
	"poraclego/internal/alertstate"
	"poraclego/internal/logging"
)

// SetAlertStateLoader overrides the snapshot loader. Intended for tests.
func (p *Processor) SetAlertStateLoader(loader func() (*alertstate.Snapshot, error)) {
	if p == nil {
		return
	}
	p.alertStateRefreshMu.Lock()
	p.alertStateLoader = loader
	p.alertStateRefreshMu.Unlock()
}

// RefreshAlertCacheSync reloads the full in-memory alert state.
func (p *Processor) RefreshAlertCacheSync() error {
	if p == nil {
		return nil
	}
	return p.reloadAlertState()
}

// RefreshAlertCacheAsync refreshes the in-memory alert state used for matching.
func (p *Processor) RefreshAlertCacheAsync() {
	if p == nil {
		return
	}
	p.alertStateRefreshMu.Lock()
	if p.alertStateRefreshing {
		p.alertStatePending = true
		p.alertStateRefreshMu.Unlock()
		return
	}
	p.alertStateRefreshing = true
	p.alertStateRefreshMu.Unlock()

	go p.refreshAlertStateLoop()
}

func (p *Processor) currentAlertState() *alertstate.Snapshot {
	if p == nil || p.alertState == nil {
		return nil
	}
	return p.alertState.Get()
}

// CurrentAlertStateForTest exposes the active snapshot for package-external tests.
func (p *Processor) CurrentAlertStateForTest() *alertstate.Snapshot {
	return p.currentAlertState()
}

func (p *Processor) reloadAlertState() error {
	if p == nil {
		return nil
	}
	p.alertStateRefreshMu.Lock()
	loader := p.alertStateLoader
	p.alertStateRefreshMu.Unlock()
	if loader == nil {
		loader = func() (*alertstate.Snapshot, error) {
			return alertstate.Load(p.query, p.getFences())
		}
	}
	snapshot, err := loader()
	if err != nil {
		return err
	}
	if p.alertState == nil {
		p.alertState = alertstate.NewManager()
	}
	p.alertState.Set(snapshot)
	if logger := logging.Get().Webhooks; logger != nil {
		logger.Debugf("Loaded alert state snapshot: monsters=%d raids=%d eggs=%d quests=%d invasions=%d lures=%d gyms=%d nests=%d forts=%d weather=%d maxbattle=%d humans=%d profiles=%d",
			len(snapshot.Tables["monsters"]),
			len(snapshot.Tables["raid"]),
			len(snapshot.Tables["egg"]),
			len(snapshot.Tables["quest"]),
			len(snapshot.Tables["invasion"]),
			len(snapshot.Tables["lures"]),
			len(snapshot.Tables["gym"]),
			len(snapshot.Tables["nests"]),
			len(snapshot.Tables["forts"]),
			len(snapshot.Tables["weather"]),
			len(snapshot.Tables["maxbattle"]),
			len(snapshot.Humans),
			len(snapshot.Profiles),
		)
	}
	return nil
}

func (p *Processor) refreshAlertStateLoop() {
	for {
		if err := p.reloadAlertState(); err != nil {
			if logger := logging.Get().Webhooks; logger != nil {
				logger.Warnf("alert state refresh failed: %v", err)
			}
		}
		p.alertStateRefreshMu.Lock()
		if !p.alertStatePending {
			p.alertStateRefreshing = false
			p.alertStateRefreshMu.Unlock()
			return
		}
		p.alertStatePending = false
		p.alertStateRefreshMu.Unlock()
	}
}
