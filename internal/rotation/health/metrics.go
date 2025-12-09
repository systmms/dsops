package health

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Rotation metrics
	rotationStartedTotal   *prometheus.CounterVec
	rotationCompletedTotal *prometheus.CounterVec
	rotationDuration       *prometheus.HistogramVec
	rollbackTotal          *prometheus.CounterVec

	// Health check metrics
	healthCheckDuration *prometheus.HistogramVec
	healthCheckStatus   *prometheus.GaugeVec

	// Registration guard
	metricsOnce       sync.Once
	metricsRegistered bool
)

// RotationMetrics provides methods to record rotation metrics.
type RotationMetrics struct{}

// NewRotationMetrics creates a new RotationMetrics instance.
// Metrics are lazily registered on first use.
func NewRotationMetrics() *RotationMetrics {
	return &RotationMetrics{}
}

// InitMetrics initializes all Prometheus metrics.
// This should be called once at startup if Prometheus metrics are enabled.
func InitMetrics() {
	metricsOnce.Do(func() {
		rotationStartedTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dsops_rotation_started_total",
				Help: "Total number of rotation operations started",
			},
			[]string{"service", "environment", "strategy"},
		)

		rotationCompletedTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dsops_rotation_completed_total",
				Help: "Total number of rotation operations completed",
			},
			[]string{"service", "environment", "status"},
		)

		rotationDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dsops_rotation_duration_seconds",
				Help:    "Duration of rotation operations in seconds",
				Buckets: []float64{1, 5, 10, 30, 60, 120},
			},
			[]string{"service"},
		)

		rollbackTotal = promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dsops_rollback_total",
				Help: "Total number of rollback operations",
			},
			[]string{"service", "type"},
		)

		healthCheckDuration = promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dsops_health_check_duration_seconds",
				Help:    "Duration of health check operations in seconds",
				Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5},
			},
			[]string{"service", "check_type"},
		)

		healthCheckStatus = promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "dsops_health_check_status",
				Help: "Current health check status (1=healthy, 0=unhealthy)",
			},
			[]string{"service", "check_type"},
		)

		metricsRegistered = true
	})
}

// RecordRotationStarted records a rotation start event.
func (m *RotationMetrics) RecordRotationStarted(service, environment, strategy string) {
	if !metricsRegistered || rotationStartedTotal == nil {
		return
	}
	rotationStartedTotal.WithLabelValues(service, environment, strategy).Inc()
}

// RecordRotationCompleted records a rotation completion event.
func (m *RotationMetrics) RecordRotationCompleted(service, environment, status string, durationSeconds float64) {
	if !metricsRegistered {
		return
	}

	if rotationCompletedTotal != nil {
		rotationCompletedTotal.WithLabelValues(service, environment, status).Inc()
	}

	if rotationDuration != nil {
		rotationDuration.WithLabelValues(service).Observe(durationSeconds)
	}
}

// RecordRollback records a rollback event.
func (m *RotationMetrics) RecordRollback(service, rollbackType string) {
	if !metricsRegistered || rollbackTotal == nil {
		return
	}
	rollbackTotal.WithLabelValues(service, rollbackType).Inc()
}

// RecordHealthCheck records a health check result.
func (m *RotationMetrics) RecordHealthCheck(service, checkType string, healthy bool, durationSeconds float64) {
	if !metricsRegistered {
		return
	}

	if healthCheckDuration != nil {
		healthCheckDuration.WithLabelValues(service, checkType).Observe(durationSeconds)
	}

	if healthCheckStatus != nil {
		value := 0.0
		if healthy {
			value = 1.0
		}
		healthCheckStatus.WithLabelValues(service, checkType).Set(value)
	}
}

// GetRotationStartedTotal returns the rotation started counter for testing.
func GetRotationStartedTotal() *prometheus.CounterVec {
	return rotationStartedTotal
}

// GetRotationCompletedTotal returns the rotation completed counter for testing.
func GetRotationCompletedTotal() *prometheus.CounterVec {
	return rotationCompletedTotal
}

// GetRotationDuration returns the rotation duration histogram for testing.
func GetRotationDuration() *prometheus.HistogramVec {
	return rotationDuration
}

// GetRollbackTotal returns the rollback counter for testing.
func GetRollbackTotal() *prometheus.CounterVec {
	return rollbackTotal
}

// GetHealthCheckDuration returns the health check duration histogram for testing.
func GetHealthCheckDuration() *prometheus.HistogramVec {
	return healthCheckDuration
}

// GetHealthCheckStatus returns the health check status gauge for testing.
func GetHealthCheckStatus() *prometheus.GaugeVec {
	return healthCheckStatus
}

// IsMetricsRegistered returns whether metrics have been initialized.
func IsMetricsRegistered() bool {
	return metricsRegistered
}
