package metrics

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

func TestInitReturnsMetrics(t *testing.T) {
	defer Reset()
	m := Init()
	if m == nil {
		t.Fatal("Init returned nil")
	}
	if m.Registry == nil {
		t.Fatal("Registry is nil")
	}
}

func TestInitIsSingleton(t *testing.T) {
	defer Reset()
	m1 := Init()
	m2 := Init()
	if m1 != m2 {
		t.Fatal("Init returned different instances")
	}
}

func TestGetBeforeInit(t *testing.T) {
	defer Reset()
	Reset()
	if Get() != nil {
		t.Fatal("Get should return nil before Init")
	}
}

func TestCounterIncrement(t *testing.T) {
	defer Reset()
	m := Init()
	m.DiscordRateLimitTotal.Inc()
	m.DiscordRateLimitTotal.Inc()

	val := counterValue(t, m.DiscordRateLimitTotal)
	if val != 2 {
		t.Fatalf("expected 2, got %v", val)
	}
}

func TestHistogramObserve(t *testing.T) {
	defer Reset()
	m := Init()
	m.DispatchSendDuration.WithLabelValues("discord", "webhook").Observe(0.5)
	m.DispatchSendDuration.WithLabelValues("discord", "webhook").Observe(1.5)

	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	found := false
	for _, fam := range families {
		if fam.GetName() == "dexter_dispatch_send_duration_seconds" {
			for _, metric := range fam.GetMetric() {
				h := metric.GetHistogram()
				if h != nil && h.GetSampleCount() == 2 {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatal("histogram not found or wrong count")
	}
}

func TestGaugeVecLabels(t *testing.T) {
	defer Reset()
	m := Init()
	m.DispatchQueueDepth.WithLabelValues("discord").Set(42)
	m.DispatchQueueDepth.WithLabelValues("telegram").Set(7)

	families, err := m.Registry.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() != "dexter_dispatch_queue_depth" {
			continue
		}
		if len(fam.GetMetric()) != 2 {
			t.Fatalf("expected 2 metrics, got %d", len(fam.GetMetric()))
		}
	}
}

type fakeQueue struct{ n int }

func (q *fakeQueue) Len() int { return q.n }

func TestCollectorSamplesQueues(t *testing.T) {
	defer Reset()
	m := Init()
	wq := &fakeQueue{n: 10}
	dq := &fakeQueue{n: 5}
	tq := &fakeQueue{n: 3}

	collect(m, wq, dq, tq, nil)

	if v := gaugeValue(t, m.WebhookQueueDepth); v != 10 {
		t.Fatalf("webhook queue: expected 10, got %v", v)
	}
}

func TestStartCollectorStopsOnCancel(t *testing.T) {
	defer Reset()
	Init()
	ctx, cancel := context.WithCancel(context.Background())
	StartCollector(ctx, &fakeQueue{}, &fakeQueue{}, &fakeQueue{}, nil)
	cancel()
	// Allow goroutine to exit.
	time.Sleep(20 * time.Millisecond)
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	var m io_prometheus_client.Metric
	if err := c.Write(&m); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return m.GetCounter().GetValue()
}

func gaugeValue(t *testing.T, g prometheus.Gauge) float64 {
	t.Helper()
	var m io_prometheus_client.Metric
	if err := g.Write(&m); err != nil {
		t.Fatalf("write metric: %v", err)
	}
	return m.GetGauge().GetValue()
}
