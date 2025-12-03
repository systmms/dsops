package notifications

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// droppedTotal tracks the number of notifications dropped due to queue overflow.
	droppedTotal prometheus.Counter

	// metricsOnce ensures metrics are only registered once.
	metricsOnce sync.Once

	// metricsRegistered indicates if metrics have been registered.
	metricsRegistered bool
)

// InitMetrics initializes the Prometheus metrics for notifications.
// This should be called once at startup if Prometheus metrics are enabled.
func InitMetrics() {
	metricsOnce.Do(func() {
		droppedTotal = promauto.NewCounter(prometheus.CounterOpts{
			Name: "dsops_notifications_dropped_total",
			Help: "Total number of notification events dropped due to queue overflow",
		})
		metricsRegistered = true
	})
}

// incrementDroppedCounter increments the dropped notifications counter.
// This is safe to call even if metrics have not been initialized.
func incrementDroppedCounter() {
	if metricsRegistered && droppedTotal != nil {
		droppedTotal.Inc()
	}
}

// GetDroppedCounter returns the current dropped counter for testing.
// Returns nil if metrics have not been initialized.
func GetDroppedCounter() prometheus.Counter {
	return droppedTotal
}
