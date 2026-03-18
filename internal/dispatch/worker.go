package dispatch

import (
	"fmt"
	"os"
	"sync"
	"time"

	"dexter/internal/config"
	"dexter/internal/logging"
)

// Worker drains a dispatch queue at a fixed interval.
type Worker struct {
	queue       *Queue
	interval    time.Duration
	name        string
	cfg         *config.Config
	sender      *Sender
	startOnce   sync.Once
	sem         chan struct{}
	semWebhook  chan struct{}
	mu          sync.Mutex
	pending     map[string][]MessageJob
	runningKeys map[string]bool
}

// NewWorker constructs a queue worker.
func NewWorker(queue *Queue, cfg *config.Config, name string, interval time.Duration, root string) *Worker {
	if interval <= 0 {
		interval = 500 * time.Millisecond
	}
	return &Worker{
		queue:       queue,
		name:        name,
		interval:    interval,
		cfg:         cfg,
		sender:      NewSender(cfg, root),
		pending:     map[string][]MessageJob{},
		runningKeys: map[string]bool{},
	}
}

func (w *Worker) logger() *logging.Logger {
	loggers := logging.Get()
	switch w.name {
	case "discord":
		return loggers.Discord
	case "telegram":
		return loggers.Telegram
	default:
		return loggers.General
	}
}

// Start begins draining the queue in a goroutine.
// It is safe to call multiple times; only the first call has effect.
func (w *Worker) Start() {
	if w == nil {
		return
	}
	w.startOnce.Do(func() {
		w.configureConcurrency()
		if w.sender != nil {
			w.sender.LoadCleanCaches(w.name)
		}
		go func() {
			ticker := time.NewTicker(w.interval)
			defer ticker.Stop()
			for range ticker.C {
				jobs := w.queue.Drain()
				if len(jobs) == 0 {
					continue
				}
				w.enqueue(jobs)
			}
		}()
	})
}

func (w *Worker) enqueue(jobs []MessageJob) {
	if w == nil {
		return
	}
	w.mu.Lock()
	if w.pending == nil {
		w.pending = map[string][]MessageJob{}
	}
	if w.runningKeys == nil {
		w.runningKeys = map[string]bool{}
	}
	toStart := []string{}
	for _, job := range jobs {
		target := job.Target
		if target == "" {
			target = "_"
		}
		w.pending[target] = append(w.pending[target], job)
		if !w.runningKeys[target] {
			w.runningKeys[target] = true
			toStart = append(toStart, target)
		}
	}
	w.mu.Unlock()

	for _, target := range toStart {
		go w.runTarget(target)
	}
}

func (w *Worker) runTarget(target string) {
	for {
		job, ok := w.nextJob(target)
		if !ok {
			return
		}
		w.processJob(job)
	}
}

func (w *Worker) nextJob(target string) (MessageJob, bool) {
	if w == nil {
		return MessageJob{}, false
	}
	if target == "" {
		target = "_"
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	queue := w.pending[target]
	if len(queue) == 0 {
		delete(w.pending, target)
		delete(w.runningKeys, target)
		return MessageJob{}, false
	}
	job := queue[0]
	w.pending[target] = queue[1:]
	return job, true
}

func (w *Worker) SaveCaches() {
	if w == nil || w.sender == nil {
		return
	}
	w.sender.SaveCleanCaches(w.name)
}

func (w *Worker) configureConcurrency() {
	if w == nil {
		return
	}
	switch w.name {
	case "discord":
		limit := w.intSetting("tuning.concurrentDiscordDestinationsPerBot", 1)
		w.sem = make(chan struct{}, limit)
		webhookLimit := w.intSetting("tuning.concurrentDiscordWebhookConnections", limit)
		w.semWebhook = make(chan struct{}, webhookLimit)
	case "telegram":
		limit := w.intSetting("tuning.concurrentTelegramDestinationsPerBot", 1)
		w.sem = make(chan struct{}, limit)
	default:
		w.sem = make(chan struct{}, 1)
	}
}

func (w *Worker) processJob(job MessageJob) {
	sem := w.sem
	if job.Type == "webhook" && w.semWebhook != nil {
		sem = w.semWebhook
	}
	if sem != nil {
		sem <- struct{}{}
		defer func() { <-sem }()
	}
	if err := w.sender.Send(job); err != nil {
		logger := w.logger()
		if logger != nil {
			logger.Errorf("%s send failed: %v", w.name, err)
		} else {
			fmt.Fprintf(os.Stderr, "%s send failed: %v\n", w.name, err)
		}
	}
}

func (w *Worker) intSetting(path string, fallback int) int {
	if w.cfg == nil {
		return fallback
	}
	if value, ok := w.cfg.GetInt(path); ok && value > 0 {
		return value
	}
	return fallback
}
