package profile

import (
	"context"
	"sync"
	"time"

	"poraclego/internal/config"
	"poraclego/internal/db"
	"poraclego/internal/digest"
	"poraclego/internal/dispatch"
	"poraclego/internal/dts"
	"poraclego/internal/i18n"
	"poraclego/internal/logging"
	"poraclego/internal/tz"
)

// Scheduler checks active hours and switches profiles as needed.
type Scheduler struct {
	cfg           *config.Config
	query         *db.Query
	i18n          *i18n.Factory
	tzLocator     *tz.Locator
	discordQueue  *dispatch.Queue
	telegramQueue *dispatch.Queue
	questDigests  *digest.Store
	templatesMu   sync.RWMutex
	templates     []dts.Template
	refreshMu     sync.RWMutex
	refreshState  func()
}

// NewScheduler creates a profile scheduler.
func NewScheduler(cfg *config.Config, query *db.Query, i18nFactory *i18n.Factory, tzLocator *tz.Locator, discordQueue, telegramQueue *dispatch.Queue, digestStore *digest.Store, templates []dts.Template) *Scheduler {
	return &Scheduler{
		cfg:           cfg,
		query:         query,
		i18n:          i18nFactory,
		tzLocator:     tzLocator,
		discordQueue:  discordQueue,
		telegramQueue: telegramQueue,
		questDigests:  digestStore,
		templates:     templates,
	}
}

// UpdateTemplates refreshes DTS templates used by scheduler-driven quest digest rendering.
func (s *Scheduler) UpdateTemplates(templates []dts.Template) {
	if s == nil {
		return
	}
	s.templatesMu.Lock()
	s.templates = append([]dts.Template(nil), templates...)
	s.templatesMu.Unlock()
}

// SetRefreshAlertState registers a callback used after scheduler-driven profile changes.
func (s *Scheduler) SetRefreshAlertState(refresh func()) {
	if s == nil {
		return
	}
	s.refreshMu.Lock()
	s.refreshState = refresh
	s.refreshMu.Unlock()
}

// Start begins the periodic profile checks.
func (s *Scheduler) Start(ctx context.Context) {
	go s.run(ctx)
}

func (s *Scheduler) run(ctx context.Context) {
	s.waitForBoundary(ctx, time.Minute)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		s.check()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s *Scheduler) waitForBoundary(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		return
	}
	now := time.Now()
	next := now.Truncate(interval).Add(interval)
	delay := time.Until(next)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

func (s *Scheduler) refreshAlertState() {
	if s == nil {
		return
	}
	s.refreshMu.RLock()
	refresh := s.refreshState
	s.refreshMu.RUnlock()
	if refresh != nil {
		refresh()
	}
}

func (s *Scheduler) logf(format string, args ...any) {
	logger := logging.Get().General
	if logger == nil {
		return
	}
	logger.Infof(format, args...)
}
