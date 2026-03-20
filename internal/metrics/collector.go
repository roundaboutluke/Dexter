package metrics

import (
	"context"
	"database/sql"
	"time"
)

// QueueSizer reports the number of pending items.
type QueueSizer interface {
	Len() int
}

// StartCollector runs a background goroutine that periodically samples
// queue depths and database pool statistics. It stops when ctx is cancelled.
func StartCollector(ctx context.Context, webhookQueue QueueSizer, discordQueue QueueSizer, telegramQueue QueueSizer, db *sql.DB) {
	m := Get()
	if m == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				collect(m, webhookQueue, discordQueue, telegramQueue, db)
			}
		}
	}()
}

func collect(m *Metrics, webhookQueue QueueSizer, discordQueue QueueSizer, telegramQueue QueueSizer, db *sql.DB) {
	if webhookQueue != nil {
		m.WebhookQueueDepth.Set(float64(webhookQueue.Len()))
	}
	if discordQueue != nil {
		m.DispatchQueueDepth.WithLabelValues("discord").Set(float64(discordQueue.Len()))
	}
	if telegramQueue != nil {
		m.DispatchQueueDepth.WithLabelValues("telegram").Set(float64(telegramQueue.Len()))
	}
	if db != nil {
		stats := db.Stats()
		m.DBOpenConnections.Set(float64(stats.OpenConnections))
		m.DBIdleConnections.Set(float64(stats.Idle))
	}
}
