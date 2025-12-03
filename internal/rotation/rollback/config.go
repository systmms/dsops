// Package rollback provides automatic and manual rollback functionality for rotation operations.
package rollback

import "time"

const (
	// DefaultTimeout is the default timeout for rollback operations.
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRetries is the default number of retry attempts for failed rollbacks.
	DefaultMaxRetries = 2
)

// Config holds configuration for rollback behavior.
type Config struct {
	// Automatic enables automatic rollback on verification failure.
	Automatic bool

	// OnVerificationFailure triggers rollback when verification fails.
	OnVerificationFailure bool

	// OnHealthCheckFailure triggers rollback when health checks fail.
	OnHealthCheckFailure bool

	// Timeout is the maximum time for rollback operation.
	Timeout time.Duration

	// MaxRetries is the number of times to retry rollback if it fails.
	MaxRetries int

	// Notifications lists notification channels for rollback events.
	Notifications []string
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() Config {
	return Config{
		Automatic:             true,
		OnVerificationFailure: true,
		OnHealthCheckFailure:  true,
		Timeout:               DefaultTimeout,
		MaxRetries:            DefaultMaxRetries,
	}
}
