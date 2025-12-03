package rollback

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()

	assert.True(t, config.Automatic)
	assert.True(t, config.OnVerificationFailure)
	assert.True(t, config.OnHealthCheckFailure)
	assert.Equal(t, DefaultTimeout, config.Timeout)
	assert.Equal(t, DefaultMaxRetries, config.MaxRetries)
}

func TestStateInfo_NewStateInfo(t *testing.T) {
	t.Parallel()
	info := NewStateInfo("test-service", "production")

	assert.Equal(t, StateIdle, info.Current)
	assert.Equal(t, "test-service", info.Service)
	assert.Equal(t, "production", info.Environment)
	assert.Empty(t, info.Transitions)
}

func TestStateInfo_TransitionTo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		fromState   State
		toState     State
		shouldError bool
	}{
		{"idle to triggered", StateIdle, StateTriggered, false},
		{"triggered to in_progress", StateTriggered, StateInProgress, false},
		{"in_progress to verifying", StateInProgress, StateVerifying, false},
		{"verifying to completed", StateVerifying, StateCompleted, false},
		{"verifying to failed", StateVerifying, StateFailed, false},
		{"idle to completed (invalid)", StateIdle, StateCompleted, true},
		{"completed to in_progress (invalid)", StateCompleted, StateInProgress, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := NewStateInfo("test", "prod")
			info.Current = tt.fromState

			err := info.TransitionTo(tt.toState, "test transition", nil)
			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.toState, info.Current)
			}
		})
	}
}

func TestStateInfo_TransitionTracking(t *testing.T) {
	t.Parallel()
	info := NewStateInfo("test", "prod")

	// Transition through states
	_ = info.TransitionTo(StateTriggered, "starting rollback", nil)
	_ = info.TransitionTo(StateInProgress, "executing", nil)
	_ = info.TransitionTo(StateVerifying, "verifying", nil)
	_ = info.TransitionTo(StateCompleted, "done", nil)

	assert.Len(t, info.Transitions, 4)
	assert.Equal(t, StateIdle, info.Transitions[0].FromState)
	assert.Equal(t, StateTriggered, info.Transitions[0].ToState)
	assert.NotZero(t, info.StartedAt)
	assert.NotZero(t, info.CompletedAt)
}

func TestState_IsTerminal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state      State
		isTerminal bool
	}{
		{StateIdle, false},
		{StateTriggered, false},
		{StateInProgress, false},
		{StateVerifying, false},
		{StateCompleted, true},
		{StateFailed, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.isTerminal, tt.state.IsTerminal())
		})
	}
}

func TestManager_NewManager(t *testing.T) {
	t.Parallel()
	config := DefaultConfig()
	m := NewManager(config, nil)

	assert.NotNil(t, m)
}

func TestManager_TriggerRollback_Success(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.Timeout = 5 * time.Second
	m := NewManager(config, nil)

	restoreCalled := false
	verifyCalled := false

	req := RollbackRequest{
		Service:         "test-service",
		Environment:     "production",
		Reason:          "verification failed",
		PreviousVersion: "v1",
		FailedVersion:   "v2",
		RestoreFunc: func(ctx context.Context) error {
			restoreCalled = true
			return nil
		},
		VerifyFunc: func(ctx context.Context) error {
			verifyCalled = true
			return nil
		},
	}

	result, err := m.TriggerRollback(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, StateCompleted, result.State)
	assert.True(t, restoreCalled)
	assert.True(t, verifyCalled)
	assert.Greater(t, result.Duration, time.Duration(0))
}

func TestManager_TriggerRollback_RestoreFailure(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.MaxRetries = 0 // No retries
	m := NewManager(config, nil)

	req := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "verification failed",
		RestoreFunc: func(ctx context.Context) error {
			return errors.New("restore failed")
		},
	}

	result, err := m.TriggerRollback(context.Background(), req)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, StateFailed, result.State)
	assert.Contains(t, err.Error(), "restore failed")
}

func TestManager_TriggerRollback_VerifyFailure(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.MaxRetries = 0 // No retries
	m := NewManager(config, nil)

	req := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "verification failed",
		RestoreFunc: func(ctx context.Context) error {
			return nil
		},
		VerifyFunc: func(ctx context.Context) error {
			return errors.New("verification failed")
		},
	}

	result, err := m.TriggerRollback(context.Background(), req)
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, err.Error(), "verification failed")
}

func TestManager_TriggerRollback_Retries(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.MaxRetries = 2
	m := NewManager(config, nil)

	attempts := 0
	req := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "verification failed",
		RestoreFunc: func(ctx context.Context) error {
			attempts++
			if attempts < 3 {
				return errors.New("transient error")
			}
			return nil
		},
		VerifyFunc: func(ctx context.Context) error {
			return nil
		},
	}

	result, err := m.TriggerRollback(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 3, attempts) // Initial + 2 retries
}

func TestManager_TriggerRollback_Timeout(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.Timeout = 100 * time.Millisecond
	config.MaxRetries = 0
	m := NewManager(config, nil)

	req := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "verification failed",
		RestoreFunc: func(ctx context.Context) error {
			select {
			case <-time.After(5 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	result, err := m.TriggerRollback(context.Background(), req)
	assert.Error(t, err)
	assert.False(t, result.Success)
}

func TestManager_TriggerRollback_DisabledAutomatic(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.Automatic = false
	m := NewManager(config, nil)

	req := RollbackRequest{
		Service: "test-service",
	}

	_, err := m.TriggerRollback(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "automatic rollback is disabled")
}

func TestManager_ManualRollback_BypassesAutoCheck(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	config.Automatic = false // Disable automatic
	m := NewManager(config, nil)

	req := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "manual request",
		RestoreFunc: func(ctx context.Context) error {
			return nil
		},
	}

	// Manual rollback should work even with Automatic=false
	result, err := m.ManualRollback(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestManager_GetState(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	m := NewManager(config, nil)

	// Initially no state
	state := m.GetState("test-service", "production")
	assert.Nil(t, state)

	// After a rollback, state should exist
	req := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "test",
		RestoreFunc: func(ctx context.Context) error { return nil },
	}
	_, _ = m.ManualRollback(context.Background(), req)

	state = m.GetState("test-service", "production")
	assert.NotNil(t, state)
	assert.Equal(t, "test-service", state.Service)
}

func TestManager_Reset(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	m := NewManager(config, nil)

	// Trigger a rollback to create state
	req := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "test",
		RestoreFunc: func(ctx context.Context) error { return nil },
	}
	_, _ = m.ManualRollback(context.Background(), req)

	// Verify state exists
	state := m.GetState("test-service", "production")
	assert.NotNil(t, state)

	// Reset the state
	m.Reset("test-service", "production")

	// State should be gone
	state = m.GetState("test-service", "production")
	assert.Nil(t, state)
}

func TestManager_ConcurrentRollbacks(t *testing.T) {
	t.Parallel()

	config := DefaultConfig()
	m := NewManager(config, nil)

	// First rollback in progress
	req1 := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "test1",
		RestoreFunc: func(ctx context.Context) error {
			time.Sleep(100 * time.Millisecond)
			return nil
		},
	}

	// Start first rollback in goroutine
	done := make(chan struct{})
	go func() {
		_, _ = m.ManualRollback(context.Background(), req1)
		close(done)
	}()

	// Give it time to start
	time.Sleep(10 * time.Millisecond)

	// Try to start a second rollback for same service
	req2 := RollbackRequest{
		Service:     "test-service",
		Environment: "production",
		Reason:      "test2",
	}

	_, err := m.ManualRollback(context.Background(), req2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in progress")

	<-done // Wait for first to complete
}
