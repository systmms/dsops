// Package health provides health monitoring functionality for rotation operations.
// This is a placeholder package that will be fully implemented in a future phase.
package health

import (
	"context"
	"time"
)

// ProtocolType represents the protocol used by a health checker.
type ProtocolType string

const (
	// ProtocolSQL represents SQL database connections.
	ProtocolSQL ProtocolType = "sql"

	// ProtocolHTTP represents HTTP API endpoints.
	ProtocolHTTP ProtocolType = "http"

	// ProtocolScript represents custom health check scripts.
	ProtocolScript ProtocolType = "script"
)

// HealthChecker defines the interface for performing health checks.
type HealthChecker interface {
	// Name returns the health checker name.
	Name() string

	// Check performs a single health check.
	Check(ctx context.Context, service ServiceConfig) (HealthResult, error)

	// Protocol returns the protocol type this checker supports.
	Protocol() ProtocolType
}

// ServiceConfig holds service configuration for health checks.
// This will be expanded as the health check feature is implemented.
type ServiceConfig struct {
	// Name is the service name.
	Name string

	// Type is the service type (e.g., "postgresql", "mysql", "http").
	Type string

	// Endpoint is the service endpoint for health checks.
	Endpoint string

	// Config holds additional configuration.
	Config map[string]interface{}
}

// HealthResult represents the outcome of a health check.
type HealthResult struct {
	// Healthy indicates whether the health check passed.
	Healthy bool

	// Message provides details about the health check result.
	Message string

	// Duration is how long the health check took.
	Duration time.Duration

	// Timestamp is when the health check was performed.
	Timestamp time.Time

	// Metadata contains additional check-specific data.
	Metadata map[string]interface{}
}

// HealthStatus represents the current health status of a service.
type HealthStatus int

const (
	// StatusUnknown indicates health status has not been checked.
	StatusUnknown HealthStatus = iota

	// StatusHealthy indicates the service is healthy.
	StatusHealthy

	// StatusDegraded indicates the service has some issues but is functional.
	StatusDegraded

	// StatusUnhealthy indicates the service is not healthy.
	StatusUnhealthy
)
