// Package gradual provides gradual rollout strategies for rotation operations.
// This is a placeholder package that will be fully implemented in a future phase.
package gradual

import (
	"context"
	"time"
)

// RolloutStrategy defines the interface for gradual rollout behavior.
type RolloutStrategy interface {
	// Name returns the strategy name (e.g., "canary", "percentage", "group").
	Name() string

	// Plan generates the rollout waves for the given service.
	Plan(ctx context.Context, service ServiceConfig) ([]RolloutWave, error)

	// Execute runs the rollout plan.
	Execute(ctx context.Context, plan []RolloutWave) error
}

// ServiceConfig holds service configuration for gradual rollout.
// This will be expanded as the gradual rollout feature is implemented.
type ServiceConfig struct {
	// Name is the service name.
	Name string

	// Environment is the environment.
	Environment string

	// Instances lists the service instances.
	Instances []Instance
}

// Instance represents a single service instance.
type Instance struct {
	// ID is the unique instance identifier.
	ID string

	// Labels are key-value labels for instance selection.
	Labels map[string]string

	// Endpoint is the instance endpoint.
	Endpoint string
}

// RolloutWave represents a single phase of gradual rollout.
type RolloutWave struct {
	// Instances lists the instance IDs to rotate in this wave.
	Instances []string

	// Percentage is the percentage of total instances in this wave.
	Percentage int

	// WaitDuration is the time to wait before the next wave.
	WaitDuration time.Duration

	// HealthMonitoringDuration is how long to monitor health after this wave.
	HealthMonitoringDuration time.Duration
}

// RolloutStatus represents the current status of a gradual rollout.
type RolloutStatus struct {
	// CurrentWave is the index of the current wave (0-based).
	CurrentWave int

	// TotalWaves is the total number of waves.
	TotalWaves int

	// CompletedInstances lists instances that have been rotated.
	CompletedInstances []string

	// FailedInstances lists instances that failed rotation.
	FailedInstances []string

	// InProgress indicates whether rollout is currently running.
	InProgress bool

	// Paused indicates whether rollout is paused awaiting manual approval.
	Paused bool
}
