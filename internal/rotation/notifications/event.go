package notifications

import (
	"time"
)

// EventType represents the type of rotation event.
type EventType string

const (
	// EventTypeStarted indicates a rotation has started.
	EventTypeStarted EventType = "started"

	// EventTypeCompleted indicates a rotation has completed successfully.
	EventTypeCompleted EventType = "completed"

	// EventTypeFailed indicates a rotation has failed.
	EventTypeFailed EventType = "failed"

	// EventTypeRollback indicates a rollback has occurred.
	EventTypeRollback EventType = "rollback"
)

// RotationStatus represents the outcome status of a rotation.
type RotationStatus string

const (
	// StatusSuccess indicates the rotation completed successfully.
	StatusSuccess RotationStatus = "success"

	// StatusFailure indicates the rotation failed.
	StatusFailure RotationStatus = "failure"

	// StatusRolledBack indicates the rotation was rolled back.
	StatusRolledBack RotationStatus = "rolled_back"
)

// RotationEvent represents a rotation lifecycle event for notifications.
type RotationEvent struct {
	// Type is the type of event (started, completed, failed, rollback).
	Type EventType

	// Service is the name of the service being rotated.
	Service string

	// Environment is the environment name (e.g., "production", "staging").
	Environment string

	// Strategy is the rotation strategy used (e.g., "two-key", "immediate").
	Strategy string

	// Status is the outcome status (success, failure, rolled_back).
	Status RotationStatus

	// Error contains the error if the rotation failed.
	Error error

	// Duration is how long the rotation took.
	Duration time.Duration

	// Metadata contains additional context about the rotation.
	Metadata map[string]string

	// Timestamp is when the event occurred.
	Timestamp time.Time

	// RotationID is the unique identifier for this rotation.
	RotationID string

	// PreviousVersion is the secret version before rotation.
	PreviousVersion string

	// NewVersion is the secret version after rotation.
	NewVersion string

	// InitiatedBy indicates who or what initiated the rotation.
	InitiatedBy string
}

// AllEventTypes returns all valid event types.
func AllEventTypes() []EventType {
	return []EventType{
		EventTypeStarted,
		EventTypeCompleted,
		EventTypeFailed,
		EventTypeRollback,
	}
}
