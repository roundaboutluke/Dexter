package stats

import (
	"time"

	"dexter/internal/config"
)

// Worker periodically calculates rarity statistics.
type Worker struct {
	tracker *Tracker
	cfg     *config.Config
	stop    chan struct{}
}

// NewWorker creates a stats worker.
func NewWorker(tracker *Tracker, cfg *config.Config) *Worker {
	return &Worker{
		tracker: tracker,
		cfg:     cfg,
		stop:    make(chan struct{}),
	}
}

// Start begins periodic stats calculation.
func (w *Worker) Start() {
	if w == nil || w.tracker == nil || w.cfg == nil {
		return
	}
	intervalMin, _ := w.cfg.GetInt("stats.rarityRefreshInterval")
	if intervalMin <= 0 {
		intervalMin = 60
	}
	ticker := time.NewTicker(time.Duration(intervalMin) * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				report := w.tracker.Calculate(w.cfg)
				w.tracker.StoreReport(report)
			case <-w.stop:
				return
			}
		}
	}()
}

// Stop halts the worker.
func (w *Worker) Stop() {
	if w == nil {
		return
	}
	select {
	case <-w.stop:
	default:
		close(w.stop)
	}
}
