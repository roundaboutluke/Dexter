package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Metrics holds all Prometheus metric collectors for Dexter.
type Metrics struct {
	Registry *prometheus.Registry

	// Dispatch metrics.
	DispatchSendDuration *prometheus.HistogramVec
	DispatchSendTotal    *prometheus.CounterVec
	DispatchQueueDepth   *prometheus.GaugeVec
	DispatchRetryTotal   *prometheus.CounterVec

	// Webhook processing metrics.
	WebhookReceivedTotal    *prometheus.CounterVec
	WebhookProcessDuration  *prometheus.HistogramVec
	WebhookMatchedTotal     *prometheus.CounterVec
	WebhookQueueDepth       prometheus.Gauge

	// Platform-specific rate limits.
	DiscordRateLimitTotal  prometheus.Counter
	TelegramRateLimitTotal prometheus.Counter

	// Circuit breaker.
	CircuitBreakerState *prometheus.GaugeVec
	CircuitBreakerTrips *prometheus.CounterVec

	// Database pool.
	DBOpenConnections prometheus.Gauge
	DBIdleConnections prometheus.Gauge
}

var (
	global *Metrics
	mu     sync.Mutex
)

// Init creates the global metrics instance and registers all collectors.
func Init() *Metrics {
	mu.Lock()
	defer mu.Unlock()
	if global != nil {
		return global
	}

	reg := prometheus.NewRegistry()
	reg.MustRegister(collectors.NewGoCollector())
	reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		Registry: reg,

		DispatchSendDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dexter_dispatch_send_duration_seconds",
			Help:    "Time spent sending dispatch messages.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}, []string{"platform", "type"}),

		DispatchSendTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dexter_dispatch_send_total",
			Help: "Total dispatch send attempts.",
		}, []string{"platform", "type", "status"}),

		DispatchQueueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dexter_dispatch_queue_depth",
			Help: "Number of jobs queued for dispatch.",
		}, []string{"platform"}),

		DispatchRetryTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dexter_dispatch_retry_total",
			Help: "Total dispatch retries due to rate limits or timeouts.",
		}, []string{"platform"}),

		WebhookReceivedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dexter_webhook_received_total",
			Help: "Total webhooks received by type.",
		}, []string{"hook_type"}),

		WebhookProcessDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "dexter_webhook_process_duration_seconds",
			Help:    "Time spent processing a webhook.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1},
		}, []string{"hook_type"}),

		WebhookMatchedTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dexter_webhook_matched_total",
			Help: "Total webhooks that matched at least one tracking target.",
		}, []string{"hook_type"}),

		WebhookQueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dexter_webhook_queue_depth",
			Help: "Number of webhooks queued for processing.",
		}),

		DiscordRateLimitTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dexter_discord_rate_limit_total",
			Help: "Total Discord 429 rate limit responses.",
		}),

		TelegramRateLimitTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "dexter_telegram_rate_limit_total",
			Help: "Total Telegram 429 rate limit responses.",
		}),

		CircuitBreakerState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "dexter_circuit_breaker_state",
			Help: "Circuit breaker state: 0=closed, 1=half-open, 2=open.",
		}, []string{"name"}),

		CircuitBreakerTrips: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dexter_circuit_breaker_trips_total",
			Help: "Total circuit breaker trips to open state.",
		}, []string{"name"}),

		DBOpenConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dexter_db_open_connections",
			Help: "Number of open database connections.",
		}),

		DBIdleConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "dexter_db_idle_connections",
			Help: "Number of idle database connections.",
		}),
	}

	reg.MustRegister(
		m.DispatchSendDuration,
		m.DispatchSendTotal,
		m.DispatchQueueDepth,
		m.DispatchRetryTotal,
		m.WebhookReceivedTotal,
		m.WebhookProcessDuration,
		m.WebhookMatchedTotal,
		m.WebhookQueueDepth,
		m.DiscordRateLimitTotal,
		m.TelegramRateLimitTotal,
		m.CircuitBreakerState,
		m.CircuitBreakerTrips,
		m.DBOpenConnections,
		m.DBIdleConnections,
	)

	global = m
	return m
}

// Get returns the global metrics instance, or nil if not initialized.
func Get() *Metrics {
	mu.Lock()
	defer mu.Unlock()
	return global
}

// Reset clears the global instance. Used only in tests.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	global = nil
}
