package rollback

import (
	"fmt"
	"sync"
	"time"
)

// State represents the current state of a rollback operation.
type State string

const (
	// StateIdle indicates no rollback is in progress.
	StateIdle State = "idle"

	// StateTriggered indicates a rollback has been triggered but not yet started.
	StateTriggered State = "triggered"

	// StateInProgress indicates rollback is currently executing.
	StateInProgress State = "in_progress"

	// StateVerifying indicates rollback is verifying the restored secret.
	StateVerifying State = "verifying"

	// StateCompleted indicates rollback completed successfully.
	StateCompleted State = "completed"

	// StateFailed indicates rollback failed.
	StateFailed State = "failed"
)

// String returns the string representation of the state.
func (s State) String() string {
	return string(s)
}

// IsTerminal returns true if this is a terminal state (completed or failed).
func (s State) IsTerminal() bool {
	return s == StateCompleted || s == StateFailed
}

// ValidTransitions defines allowed state transitions.
var ValidTransitions = map[State][]State{
	StateIdle:       {StateTriggered},
	StateTriggered:  {StateInProgress, StateFailed},
	StateInProgress: {StateVerifying, StateFailed},
	StateVerifying:  {StateCompleted, StateFailed},
	StateCompleted:  {StateIdle},
	StateFailed:     {StateIdle, StateTriggered}, // Can retry after failure
}

// CanTransitionTo checks if a transition from current state to new state is valid.
func (s State) CanTransitionTo(newState State) bool {
	validStates, ok := ValidTransitions[s]
	if !ok {
		return false
	}
	for _, valid := range validStates {
		if valid == newState {
			return true
		}
	}
	return false
}

// Transition represents a state transition with metadata.
type Transition struct {
	FromState State
	ToState   State
	Reason    string
	Error     error
	Timestamp time.Time
}

// StateInfo holds information about the current rollback state.
type StateInfo struct {
	mu sync.RWMutex

	// Current is the current state.
	Current State

	// Service is the service being rolled back.
	Service string

	// Environment is the environment.
	Environment string

	// StartedAt is when the rollback was triggered.
	StartedAt time.Time

	// CompletedAt is when the rollback completed (success or failure).
	CompletedAt time.Time

	// Reason is why the rollback was triggered.
	Reason string

	// Error is the error if rollback failed.
	Error error

	// PreviousVersion is the version being rolled back to.
	PreviousVersion string

	// FailedVersion is the version that failed.
	FailedVersion string

	// Transitions is the history of state transitions.
	Transitions []Transition

	// Attempts is the number of rollback attempts.
	Attempts int
}

// NewStateInfo creates a new StateInfo with initial values.
func NewStateInfo(service, environment string) *StateInfo {
	return &StateInfo{
		Current:     StateIdle,
		Service:     service,
		Environment: environment,
		Transitions: make([]Transition, 0),
	}
}

// TransitionTo attempts to transition to a new state.
// Returns an error if the transition is not allowed.
func (si *StateInfo) TransitionTo(newState State, reason string, err error) error {
	si.mu.Lock()
	defer si.mu.Unlock()

	if !si.Current.CanTransitionTo(newState) {
		return fmt.Errorf("invalid state transition from %s to %s", si.Current, newState)
	}

	transition := Transition{
		FromState: si.Current,
		ToState:   newState,
		Reason:    reason,
		Error:     err,
		Timestamp: time.Now(),
	}
	si.Transitions = append(si.Transitions, transition)
	si.Current = newState

	if newState == StateTriggered {
		si.StartedAt = time.Now()
		si.Attempts++
	}

	if newState.IsTerminal() {
		si.CompletedAt = time.Now()
		if err != nil {
			si.Error = err
		}
	}

	return nil
}

// Duration returns the duration of the rollback operation.
// Returns 0 if the rollback has not completed.
func (si *StateInfo) Duration() time.Duration {
	si.mu.RLock()
	defer si.mu.RUnlock()

	if si.StartedAt.IsZero() {
		return 0
	}
	if si.CompletedAt.IsZero() {
		return time.Since(si.StartedAt)
	}
	return si.CompletedAt.Sub(si.StartedAt)
}

// GetCurrent returns the current state (thread-safe).
func (si *StateInfo) GetCurrent() State {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.Current
}

// GetAttempts returns the number of attempts (thread-safe).
func (si *StateInfo) GetAttempts() int {
	si.mu.RLock()
	defer si.mu.RUnlock()
	return si.Attempts
}

// SetVersionInfo sets version-related fields (thread-safe).
func (si *StateInfo) SetVersionInfo(reason, previousVersion, failedVersion string) {
	si.mu.Lock()
	defer si.mu.Unlock()
	si.Reason = reason
	si.PreviousVersion = previousVersion
	si.FailedVersion = failedVersion
}
