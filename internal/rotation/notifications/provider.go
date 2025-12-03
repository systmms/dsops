// Package notifications provides notification infrastructure for rotation events.
package notifications

import (
	"context"
)

// NotificationProvider defines the interface for sending rotation notifications.
type NotificationProvider interface {
	// Name returns the provider name (e.g., "slack", "email", "pagerduty", "webhook").
	Name() string

	// Send sends a notification for the given rotation event.
	Send(ctx context.Context, event RotationEvent) error

	// SupportsEvent returns true if this provider handles the given event type.
	SupportsEvent(eventType EventType) bool

	// Validate checks if the provider configuration is valid.
	Validate(ctx context.Context) error
}
