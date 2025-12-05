// Package rollback provides integration tests for the manual rollback flow.
package rollback

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/rotation/notifications"
	"github.com/systmms/dsops/internal/rotation/rollback"
	"github.com/systmms/dsops/internal/rotation/storage"
)

// TestManualRollbackFlow tests the complete manual rollback workflow
func TestManualRollbackFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create test storage
	tempDir := t.TempDir()
	t.Setenv("DSOPS_ROTATION_DIR", tempDir)
	store := storage.NewFileStorage(tempDir)

	// Setup: Create rotation status and history
	status := &storage.RotationStatus{
		ServiceName:   "test-postgres",
		Status:        "failed",
		LastRotation:  time.Now().Add(-1 * time.Hour),
		LastResult:    "failed",
		RotationCount: 5,
		Metadata: map[string]string{
			"current_version": "v2.0.0",
		},
	}
	require.NoError(t, store.SaveStatus(status))

	// Add history entry with version info
	historyEntry := &storage.HistoryEntry{
		ID:             "rotation-123",
		Timestamp:      time.Now().Add(-1 * time.Hour),
		ServiceName:    "test-postgres",
		CredentialType: "password",
		Action:         "rotate",
		Status:         "failed",
		OldVersion:     "v1.0.0",
		NewVersion:     "v2.0.0",
		Strategy:       "two-key",
		User:           "rotation-scheduler",
		Error:          "Connection timeout during verification",
	}
	require.NoError(t, store.SaveHistory(historyEntry))

	t.Run("successful manual rollback", func(t *testing.T) {
		// Initialize notification manager (capture notifications)
		notifier := notifications.NewManager(100)
		notifier.Start(context.Background())
		defer notifier.Stop()

		// Create rollback manager
		config := rollback.DefaultConfig()
		manager := rollback.NewManager(config, notifier)

		// Track if restore and verify were called
		restoreCalled := false
		verifyCalled := false

		// Execute manual rollback
		req := rollback.RollbackRequest{
			Service:         "test-postgres",
			Environment:     "integration-test",
			Reason:          "Testing manual rollback flow",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test",
			RestoreFunc: func(ctx context.Context) error {
				restoreCalled = true
				return nil
			},
			VerifyFunc: func(ctx context.Context) error {
				verifyCalled = true
				return nil
			},
		}

		result, err := manager.ManualRollback(context.Background(), req)
		require.NoError(t, err)

		// Verify result
		assert.True(t, result.Success)
		assert.Equal(t, rollback.StateCompleted, result.State)
		assert.True(t, restoreCalled, "RestoreFunc should have been called")
		assert.True(t, verifyCalled, "VerifyFunc should have been called")
		assert.Greater(t, result.Duration, time.Duration(0))
		assert.Equal(t, 1, result.Attempts)
	})

	t.Run("rollback with restore failure triggers retry", func(t *testing.T) {
		notifier := notifications.NewManager(100)
		notifier.Start(context.Background())
		defer notifier.Stop()

		config := rollback.DefaultConfig()
		config.MaxRetries = 2
		manager := rollback.NewManager(config, notifier)

		attemptCount := 0
		req := rollback.RollbackRequest{
			Service:         "test-postgres-retry",
			Environment:     "integration-test",
			Reason:          "Testing retry on restore failure",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test",
			RestoreFunc: func(ctx context.Context) error {
				attemptCount++
				// Fail first 2 attempts, succeed on 3rd
				if attemptCount <= 2 {
					return assert.AnError
				}
				return nil
			},
			VerifyFunc: func(ctx context.Context) error {
				return nil
			},
		}

		result, err := manager.ManualRollback(context.Background(), req)
		require.NoError(t, err)

		// Should succeed after retries
		assert.True(t, result.Success)
		assert.Equal(t, 3, attemptCount, "Should have attempted 3 times (1 original + 2 retries)")
	})

	t.Run("rollback with all retries exhausted", func(t *testing.T) {
		notifier := notifications.NewManager(100)
		notifier.Start(context.Background())
		defer notifier.Stop()

		config := rollback.DefaultConfig()
		config.MaxRetries = 1
		manager := rollback.NewManager(config, notifier)

		req := rollback.RollbackRequest{
			Service:         "test-postgres-fail",
			Environment:     "integration-test",
			Reason:          "Testing exhausted retries",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test",
			RestoreFunc: func(ctx context.Context) error {
				return assert.AnError // Always fail
			},
			VerifyFunc: func(ctx context.Context) error {
				return nil
			},
		}

		result, err := manager.ManualRollback(context.Background(), req)
		require.Error(t, err)

		assert.False(t, result.Success)
		assert.Equal(t, rollback.StateFailed, result.State)
		assert.NotNil(t, result.Error)
	})

	t.Run("rollback state tracking", func(t *testing.T) {
		notifier := notifications.NewManager(100)
		notifier.Start(context.Background())
		defer notifier.Stop()

		config := rollback.DefaultConfig()
		manager := rollback.NewManager(config, notifier)

		statesObserved := []rollback.State{}
		req := rollback.RollbackRequest{
			Service:         "test-postgres-states",
			Environment:     "integration-test",
			Reason:          "Testing state tracking",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test",
			RestoreFunc: func(ctx context.Context) error {
				state := manager.GetState("test-postgres-states", "integration-test")
				if state != nil {
					statesObserved = append(statesObserved, state.GetCurrent())
				}
				return nil
			},
			VerifyFunc: func(ctx context.Context) error {
				state := manager.GetState("test-postgres-states", "integration-test")
				if state != nil {
					statesObserved = append(statesObserved, state.GetCurrent())
				}
				return nil
			},
		}

		_, err := manager.ManualRollback(context.Background(), req)
		require.NoError(t, err)

		// Verify we observed state transitions
		assert.NotEmpty(t, statesObserved)
		// During RestoreFunc we should be in StateInProgress
		assert.Contains(t, statesObserved, rollback.StateInProgress)
		// During VerifyFunc we should be in StateVerifying
		assert.Contains(t, statesObserved, rollback.StateVerifying)
	})

	t.Run("concurrent rollback prevention", func(t *testing.T) {
		notifier := notifications.NewManager(100)
		notifier.Start(context.Background())
		defer notifier.Stop()

		config := rollback.DefaultConfig()
		manager := rollback.NewManager(config, notifier)

		blockChan := make(chan struct{})
		doneChan := make(chan struct{})

		// Start first rollback that blocks
		go func() {
			req := rollback.RollbackRequest{
				Service:         "test-postgres-concurrent",
				Environment:     "integration-test",
				Reason:          "First rollback",
				PreviousVersion: "v1.0.0",
				FailedVersion:   "v2.0.0",
				InitiatedBy:     "integration-test-1",
				RestoreFunc: func(ctx context.Context) error {
					<-blockChan // Block until signaled
					return nil
				},
				VerifyFunc: func(ctx context.Context) error {
					return nil
				},
			}
			_, _ = manager.ManualRollback(context.Background(), req)
			close(doneChan)
		}()

		// Wait for first rollback to start
		time.Sleep(100 * time.Millisecond)

		// Try second rollback - should fail
		req2 := rollback.RollbackRequest{
			Service:         "test-postgres-concurrent",
			Environment:     "integration-test",
			Reason:          "Second rollback (should fail)",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test-2",
			RestoreFunc: func(ctx context.Context) error {
				return nil
			},
			VerifyFunc: func(ctx context.Context) error {
				return nil
			},
		}

		_, err := manager.ManualRollback(context.Background(), req2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already in progress")

		// Unblock first rollback
		close(blockChan)
		<-doneChan
	})
}

// TestRollbackHistoryRecording tests that rollback events are recorded in history
func TestRollbackHistoryRecording(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	t.Setenv("DSOPS_ROTATION_DIR", tempDir)
	store := storage.NewFileStorage(tempDir)

	// Setup initial rotation history
	initialEntry := &storage.HistoryEntry{
		ID:             "rotation-initial",
		Timestamp:      time.Now().Add(-1 * time.Hour),
		ServiceName:    "history-test-service",
		CredentialType: "password",
		Action:         "rotate",
		Status:         "failed",
		OldVersion:     "v1.0.0",
		NewVersion:     "v2.0.0",
	}
	require.NoError(t, store.SaveHistory(initialEntry))

	// Setup status
	status := &storage.RotationStatus{
		ServiceName:  "history-test-service",
		Status:       "failed",
		LastRotation: time.Now().Add(-1 * time.Hour),
		Metadata:     map[string]string{"current_version": "v2.0.0"},
	}
	require.NoError(t, store.SaveStatus(status))

	t.Run("rollback creates history entry", func(t *testing.T) {
		// Perform rollback (simulated via storage)
		rollbackEntry := &storage.HistoryEntry{
			ID:             "rollback-test-1",
			Timestamp:      time.Now(),
			ServiceName:    "history-test-service",
			CredentialType: "password",
			Action:         "rollback",
			Status:         "success",
			Duration:       500 * time.Millisecond,
			User:           "test-user",
			OldVersion:     "v2.0.0",
			NewVersion:     "v1.0.0",
			Metadata: map[string]string{
				"reason":  "Integration test",
				"trigger": "manual",
			},
		}
		require.NoError(t, store.SaveHistory(rollbackEntry))

		// Verify history contains rollback entry
		history, err := store.GetHistory("history-test-service", 10)
		require.NoError(t, err)

		// Find the rollback entry
		var foundRollback bool
		for _, entry := range history {
			if entry.Action == "rollback" {
				foundRollback = true
				assert.Equal(t, "success", entry.Status)
				assert.Equal(t, "test-user", entry.User)
				assert.Equal(t, "Integration test", entry.Metadata["reason"])
				assert.Equal(t, "manual", entry.Metadata["trigger"])
			}
		}
		assert.True(t, foundRollback, "Rollback entry should be in history")
	})
}

// TestRollbackNotifications tests that notifications are sent during rollback
func TestRollbackNotifications(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("notification sent on successful rollback", func(t *testing.T) {
		// Create notification manager with test provider
		notifier := notifications.NewManager(100)

		// Create a test provider that captures events
		capturedEvents := make(chan notifications.RotationEvent, 10)
		testProvider := &testNotificationProvider{
			name:   "test-capture",
			events: capturedEvents,
		}
		notifier.RegisterProvider(testProvider)
		notifier.Start(context.Background())
		defer notifier.Stop()

		// Create rollback manager
		config := rollback.DefaultConfig()
		manager := rollback.NewManager(config, notifier)

		// Execute rollback
		req := rollback.RollbackRequest{
			Service:         "notify-test-service",
			Environment:     "integration-test",
			Reason:          "Testing notification dispatch",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test",
			RestoreFunc:     func(ctx context.Context) error { return nil },
			VerifyFunc:      func(ctx context.Context) error { return nil },
		}

		_, err := manager.ManualRollback(context.Background(), req)
		require.NoError(t, err)

		// Wait for notification
		select {
		case event := <-capturedEvents:
			assert.Equal(t, notifications.EventTypeRollback, event.Type)
			assert.Equal(t, "notify-test-service", event.Service)
			assert.Equal(t, "integration-test", event.Environment)
			assert.Equal(t, notifications.StatusRolledBack, event.Status)
			assert.Equal(t, "integration-test", event.InitiatedBy)
			assert.Equal(t, "v1.0.0", event.PreviousVersion)
			assert.Equal(t, "v2.0.0", event.NewVersion)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for notification")
		}
	})

	t.Run("notification sent on failed rollback", func(t *testing.T) {
		notifier := notifications.NewManager(100)
		capturedEvents := make(chan notifications.RotationEvent, 10)
		testProvider := &testNotificationProvider{
			name:   "test-capture-fail",
			events: capturedEvents,
		}
		notifier.RegisterProvider(testProvider)
		notifier.Start(context.Background())
		defer notifier.Stop()

		config := rollback.DefaultConfig()
		config.MaxRetries = 0 // No retries
		manager := rollback.NewManager(config, notifier)

		req := rollback.RollbackRequest{
			Service:         "notify-fail-service",
			Environment:     "integration-test",
			Reason:          "Testing failure notification",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test",
			RestoreFunc:     func(ctx context.Context) error { return assert.AnError },
			VerifyFunc:      func(ctx context.Context) error { return nil },
		}

		_, err := manager.ManualRollback(context.Background(), req)
		require.Error(t, err)

		// Wait for notification
		select {
		case event := <-capturedEvents:
			assert.Equal(t, notifications.EventTypeRollback, event.Type)
			assert.Equal(t, notifications.StatusFailure, event.Status)
			assert.NotNil(t, event.Error)
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for failure notification")
		}
	})
}

// testNotificationProvider is a test notification provider that captures events
type testNotificationProvider struct {
	name   string
	events chan notifications.RotationEvent
}

func (p *testNotificationProvider) Name() string {
	return p.name
}

func (p *testNotificationProvider) Send(ctx context.Context, event notifications.RotationEvent) error {
	select {
	case p.events <- event:
	default:
		// Channel full, drop event
	}
	return nil
}

func (p *testNotificationProvider) SupportsEvent(eventType notifications.EventType) bool {
	return true
}

func (p *testNotificationProvider) Validate(ctx context.Context) error {
	return nil
}

// TestRollbackWithTimeout tests timeout behavior
func TestRollbackWithTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Run("rollback times out", func(t *testing.T) {
		notifier := notifications.NewManager(100)
		notifier.Start(context.Background())
		defer notifier.Stop()

		config := rollback.DefaultConfig()
		config.Timeout = 100 * time.Millisecond // Very short timeout
		config.MaxRetries = 0
		manager := rollback.NewManager(config, notifier)

		req := rollback.RollbackRequest{
			Service:         "timeout-test-service",
			Environment:     "integration-test",
			Reason:          "Testing timeout",
			PreviousVersion: "v1.0.0",
			FailedVersion:   "v2.0.0",
			InitiatedBy:     "integration-test",
			RestoreFunc: func(ctx context.Context) error {
				// Sleep longer than timeout
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(500 * time.Millisecond):
					return nil
				}
			},
			VerifyFunc: func(ctx context.Context) error {
				return nil
			},
		}

		result, err := manager.ManualRollback(context.Background(), req)
		require.Error(t, err)
		assert.False(t, result.Success)
		assert.Contains(t, err.Error(), "context deadline exceeded")
	})
}

// TestStorageIntegration tests the full integration with file storage
func TestStorageIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tempDir := t.TempDir()
	storageDir := filepath.Join(tempDir, "rotation")
	require.NoError(t, os.MkdirAll(storageDir, 0755))
	t.Setenv("DSOPS_ROTATION_DIR", storageDir)

	store := storage.NewFileStorage(storageDir)

	t.Run("full rotation and rollback cycle", func(t *testing.T) {
		serviceName := "cycle-test-service"

		// 1. Initial rotation
		rotateEntry := &storage.HistoryEntry{
			ID:             "rotate-1",
			Timestamp:      time.Now(),
			ServiceName:    serviceName,
			CredentialType: "password",
			Action:         "rotate",
			Status:         "success",
			OldVersion:     "v1.0.0",
			NewVersion:     "v2.0.0",
		}
		require.NoError(t, store.SaveHistory(rotateEntry))

		status := &storage.RotationStatus{
			ServiceName:   serviceName,
			Status:        "active",
			LastRotation:  time.Now(),
			LastResult:    "success",
			RotationCount: 1,
			Metadata:      map[string]string{"current_version": "v2.0.0"},
		}
		require.NoError(t, store.SaveStatus(status))

		// 2. Failed rotation
		failedEntry := &storage.HistoryEntry{
			ID:             "rotate-2",
			Timestamp:      time.Now().Add(1 * time.Hour),
			ServiceName:    serviceName,
			CredentialType: "password",
			Action:         "rotate",
			Status:         "failed",
			OldVersion:     "v2.0.0",
			NewVersion:     "v3.0.0",
			Error:          "Verification failed",
		}
		require.NoError(t, store.SaveHistory(failedEntry))

		status.Status = "failed"
		status.LastResult = "failed"
		status.LastError = "Verification failed"
		status.Metadata["current_version"] = "v3.0.0"
		require.NoError(t, store.SaveStatus(status))

		// 3. Rollback
		rollbackEntry := &storage.HistoryEntry{
			ID:             "rollback-1",
			Timestamp:      time.Now().Add(2 * time.Hour),
			ServiceName:    serviceName,
			CredentialType: "password",
			Action:         "rollback",
			Status:         "success",
			OldVersion:     "v3.0.0",
			NewVersion:     "v2.0.0",
			User:           "operator",
			Metadata: map[string]string{
				"reason":  "Rolling back failed rotation",
				"trigger": "manual",
			},
		}
		require.NoError(t, store.SaveHistory(rollbackEntry))

		status.Status = "active"
		status.LastResult = "rolled_back"
		status.LastError = ""
		status.Metadata["current_version"] = "v2.0.0"
		status.Metadata["last_rollback_reason"] = "Rolling back failed rotation"
		require.NoError(t, store.SaveStatus(status))

		// 4. Verify final state
		finalStatus, err := store.GetStatus(serviceName)
		require.NoError(t, err)
		assert.Equal(t, "active", finalStatus.Status)
		assert.Equal(t, "rolled_back", finalStatus.LastResult)
		assert.Equal(t, "v2.0.0", finalStatus.Metadata["current_version"])

		// 5. Verify history
		history, err := store.GetHistory(serviceName, 10)
		require.NoError(t, err)
		assert.Len(t, history, 3)

		// Find actions in history
		actions := make(map[string]int)
		for _, entry := range history {
			actions[entry.Action]++
		}
		assert.Equal(t, 2, actions["rotate"])
		assert.Equal(t, 1, actions["rollback"])
	})
}
