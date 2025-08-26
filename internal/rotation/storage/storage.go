package storage

import (
	"time"
)

// Storage defines the interface for rotation metadata storage
type Storage interface {
	// SaveStatus saves the current rotation status for a service
	SaveStatus(status *RotationStatus) error
	
	// GetStatus retrieves the current rotation status for a service
	GetStatus(serviceName string) (*RotationStatus, error)
	
	// SaveHistory saves a rotation history entry
	SaveHistory(entry *HistoryEntry) error
	
	// GetHistory retrieves rotation history for a service
	GetHistory(serviceName string, limit int) ([]HistoryEntry, error)
	
	// GetAllHistory retrieves rotation history for all services
	GetAllHistory(limit int) ([]HistoryEntry, error)
	
	// CleanupOldEntries removes history entries older than the specified duration
	CleanupOldEntries(olderThan time.Duration) error
}

// RotationStatus represents the current status of a service's rotation
type RotationStatus struct {
	ServiceName      string        `json:"service_name"`
	Status           string        `json:"status"` // active, failed, needs_rotation, never_rotated
	LastRotation     time.Time     `json:"last_rotation"`
	NextRotation     *time.Time    `json:"next_rotation,omitempty"`
	LastResult       string        `json:"last_result"`
	LastError        string        `json:"last_error,omitempty"`
	RotationCount    int           `json:"rotation_count"`
	SuccessCount     int           `json:"success_count"`
	FailureCount     int           `json:"failure_count"`
	RotationInterval time.Duration `json:"rotation_interval,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// HistoryEntry represents a single rotation event
type HistoryEntry struct {
	ID              string            `json:"id"`
	Timestamp       time.Time         `json:"timestamp"`
	ServiceName     string            `json:"service_name"`
	CredentialType  string            `json:"credential_type"`
	Action          string            `json:"action"` // rotate, rollback, verify
	Status          string            `json:"status"` // success, failed, partial, rolled_back
	Duration        time.Duration     `json:"duration"`
	Error           string            `json:"error,omitempty"`
	User            string            `json:"user,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	
	// Rotation details
	OldVersion      string            `json:"old_version,omitempty"`
	NewVersion      string            `json:"new_version,omitempty"`
	Strategy        string            `json:"strategy,omitempty"`
	Steps           []StepResult      `json:"steps,omitempty"`
}

// StepResult represents the result of a single rotation step
type StepResult struct {
	Name        string        `json:"name"`
	Status      string        `json:"status"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt time.Time     `json:"completed_at"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}