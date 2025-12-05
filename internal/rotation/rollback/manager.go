package rollback

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/systmms/dsops/internal/rotation/notifications"
)

// Manager orchestrates rollback operations for rotation failures.
type Manager struct {
	config   Config
	notifier *notifications.Manager

	// states tracks rollback state per service/environment
	states   map[string]*StateInfo
	statesMu sync.RWMutex
}

// NewManager creates a new rollback manager with the given configuration.
func NewManager(config Config, notifier *notifications.Manager) *Manager {
	return &Manager{
		config:   config,
		notifier: notifier,
		states:   make(map[string]*StateInfo),
	}
}

// RollbackRequest contains information needed to perform a rollback.
type RollbackRequest struct {
	// Service is the service to rollback.
	Service string

	// Environment is the environment.
	Environment string

	// Reason explains why rollback was triggered.
	Reason string

	// PreviousVersion is the version to rollback to.
	PreviousVersion string

	// FailedVersion is the version that failed.
	FailedVersion string

	// RestoreFunc is the function that restores the previous secret.
	// It should return nil on success or an error on failure.
	RestoreFunc func(ctx context.Context) error

	// VerifyFunc is the function that verifies the restored secret works.
	// It should return nil on success or an error on failure.
	VerifyFunc func(ctx context.Context) error

	// InitiatedBy indicates who or what initiated the rollback.
	InitiatedBy string
}

// RollbackResult contains the outcome of a rollback operation.
type RollbackResult struct {
	// Success indicates whether the rollback succeeded.
	Success bool

	// State is the final rollback state.
	State State

	// Duration is how long the rollback took.
	Duration time.Duration

	// Attempts is the number of attempts made.
	Attempts int

	// Error is the error if rollback failed.
	Error error
}

// stateKey generates a unique key for service/environment combination.
func stateKey(service, environment string) string {
	return fmt.Sprintf("%s/%s", service, environment)
}

// GetState returns the current rollback state for a service/environment.
func (m *Manager) GetState(service, environment string) *StateInfo {
	m.statesMu.RLock()
	defer m.statesMu.RUnlock()

	key := stateKey(service, environment)
	if state, ok := m.states[key]; ok {
		return state
	}
	return nil
}

// TriggerRollback initiates an automatic rollback operation.
// This is called when verification fails after rotation.
func (m *Manager) TriggerRollback(ctx context.Context, req RollbackRequest) (*RollbackResult, error) {
	if !m.config.Automatic {
		return nil, fmt.Errorf("automatic rollback is disabled")
	}

	return m.executeRollback(ctx, req)
}

// ManualRollback initiates a manual rollback operation.
// This bypasses the automatic rollback check.
func (m *Manager) ManualRollback(ctx context.Context, req RollbackRequest) (*RollbackResult, error) {
	return m.executeRollback(ctx, req)
}

// executeRollback performs the actual rollback operation with retries.
func (m *Manager) executeRollback(ctx context.Context, req RollbackRequest) (*RollbackResult, error) {
	key := stateKey(req.Service, req.Environment)

	// Initialize or get existing state with lock held during check
	m.statesMu.Lock()
	state, exists := m.states[key]
	if !exists {
		state = NewStateInfo(req.Service, req.Environment)
		m.states[key] = state
	}

	// Check if rollback is already in progress (while still holding lock)
	current := state.GetCurrent()
	if current != StateIdle && !current.IsTerminal() {
		m.statesMu.Unlock()
		return nil, fmt.Errorf("rollback already in progress for %s", key)
	}

	// Set up state info (while still holding lock)
	state.SetVersionInfo(req.Reason, req.PreviousVersion, req.FailedVersion)
	m.statesMu.Unlock()

	result := &RollbackResult{}

	// Retry loop
	for attempt := 0; attempt <= m.config.MaxRetries; attempt++ {
		// Transition to triggered
		if err := state.TransitionTo(StateTriggered, req.Reason, nil); err != nil {
			result.Error = err
			return result, err
		}

		// Create timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, m.config.Timeout)

		// Execute rollback
		err := m.doRollback(timeoutCtx, state, req)
		cancel()

		if err == nil {
			// Success
			result.Success = true
			result.State = StateCompleted
			result.Duration = state.Duration()
			result.Attempts = state.Attempts

			// Send success notification
			m.sendRollbackNotification(req, result, nil)

			return result, nil
		}

		// Failed - check if we should retry
		if attempt < m.config.MaxRetries {
			// Reset state for retry
			_ = state.TransitionTo(StateIdle, "preparing for retry", nil)
			continue
		}

		// Final failure
		result.Success = false
		result.State = StateFailed
		result.Duration = state.Duration()
		result.Attempts = state.Attempts
		result.Error = err

		// Send failure notification
		m.sendRollbackNotification(req, result, err)

		return result, err
	}

	return result, fmt.Errorf("rollback failed after %d attempts", m.config.MaxRetries+1)
}

// doRollback performs a single rollback attempt.
func (m *Manager) doRollback(ctx context.Context, state *StateInfo, req RollbackRequest) error {
	// Transition to in progress
	if err := state.TransitionTo(StateInProgress, "starting rollback", nil); err != nil {
		return fmt.Errorf("failed to transition to in_progress: %w", err)
	}

	// Execute restore function
	if req.RestoreFunc != nil {
		if err := req.RestoreFunc(ctx); err != nil {
			_ = state.TransitionTo(StateFailed, "restore failed", err)
			return fmt.Errorf("restore failed: %w", err)
		}
	}

	// Transition to verifying
	if err := state.TransitionTo(StateVerifying, "restore complete, verifying", nil); err != nil {
		return fmt.Errorf("failed to transition to verifying: %w", err)
	}

	// Execute verify function
	if req.VerifyFunc != nil {
		if err := req.VerifyFunc(ctx); err != nil {
			_ = state.TransitionTo(StateFailed, "verification failed", err)
			return fmt.Errorf("verification failed: %w", err)
		}
	}

	// Transition to completed
	if err := state.TransitionTo(StateCompleted, "rollback complete", nil); err != nil {
		return fmt.Errorf("failed to transition to completed: %w", err)
	}

	return nil
}

// sendRollbackNotification sends a notification about the rollback result.
func (m *Manager) sendRollbackNotification(req RollbackRequest, result *RollbackResult, err error) {
	if m.notifier == nil {
		return
	}

	status := notifications.StatusRolledBack
	if !result.Success {
		status = notifications.StatusFailure
	}

	// Determine trigger type
	trigger := "automatic"
	if req.InitiatedBy != "" && req.InitiatedBy != "rotation-engine" && req.InitiatedBy != "verification-failure" {
		trigger = "manual"
	}

	// Build enhanced metadata
	metadata := map[string]string{
		"reason":          req.Reason,
		"attempts":        fmt.Sprintf("%d", result.Attempts),
		"trigger":         trigger,
		"target_version":  req.PreviousVersion,
		"failed_version":  req.FailedVersion,
		"final_state":     string(result.State),
		"duration_ms":     fmt.Sprintf("%d", result.Duration.Milliseconds()),
	}

	// Add user info for manual rollbacks
	if trigger == "manual" && req.InitiatedBy != "" {
		metadata["user"] = req.InitiatedBy
	}

	// Add error message to metadata if present
	if err != nil {
		metadata["error_message"] = err.Error()
	}

	event := notifications.RotationEvent{
		Type:            notifications.EventTypeRollback,
		Service:         req.Service,
		Environment:     req.Environment,
		Status:          status,
		Error:           err,
		Duration:        result.Duration,
		Timestamp:       time.Now(),
		PreviousVersion: req.PreviousVersion,
		NewVersion:      req.FailedVersion,
		InitiatedBy:     req.InitiatedBy,
		Metadata:        metadata,
	}

	m.notifier.Send(event)
}

// Reset clears the rollback state for a service/environment.
// This should be called after a successful rotation.
func (m *Manager) Reset(service, environment string) {
	m.statesMu.Lock()
	defer m.statesMu.Unlock()

	key := stateKey(service, environment)
	delete(m.states, key)
}
