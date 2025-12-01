// Package e2e provides end-to-end workflow tests for dsops.
package e2e

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/rotation"
)

// FakeRotationStrategy implements SecretValueRotator for testing
type FakeRotationStrategy struct {
	name         string
	supportedTypes []rotation.SecretType
	rotateFunc   func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error)
	callCount    int64 // Use int64 for atomic operations
}

func NewFakeRotationStrategy(name string, supportedTypes []rotation.SecretType) *FakeRotationStrategy {
	return &FakeRotationStrategy{
		name:         name,
		supportedTypes: supportedTypes,
	}
}

func (f *FakeRotationStrategy) Name() string {
	return f.name
}

func (f *FakeRotationStrategy) Description() string {
	return "Fake rotation strategy for testing"
}

func (f *FakeRotationStrategy) SupportsSecret(ctx context.Context, secret rotation.SecretInfo) bool {
	for _, t := range f.supportedTypes {
		if secret.SecretType == t {
			return true
		}
	}
	return false
}

func (f *FakeRotationStrategy) Rotate(ctx context.Context, request rotation.RotationRequest) (*rotation.RotationResult, error) {
	atomic.AddInt64(&f.callCount, 1) // Thread-safe increment

	if f.rotateFunc != nil {
		return f.rotateFunc(ctx, request)
	}

	// Default successful rotation
	now := time.Now()
	return &rotation.RotationResult{
		Secret:     request.Secret,
		Status:     rotation.StatusCompleted,
		NewSecretRef: &rotation.SecretReference{
			Provider:   request.Secret.Provider,
			Key:        request.Secret.Key,
			Identifier: "v2",
		},
		RotatedAt:  &now,
		AuditTrail: []rotation.AuditEntry{
			{
				Timestamp: time.Now(),
				Action:    "rotation_completed",
				Component: "fake_strategy",
				Status:    "success",
				Message:   "Rotation completed successfully",
			},
		},
	}, nil
}

func (f *FakeRotationStrategy) Verify(ctx context.Context, request rotation.VerificationRequest) error {
	return nil
}

func (f *FakeRotationStrategy) Rollback(ctx context.Context, request rotation.RollbackRequest) error {
	return nil
}

func (f *FakeRotationStrategy) GetStatus(ctx context.Context, secret rotation.SecretInfo) (*rotation.RotationStatusInfo, error) {
	return &rotation.RotationStatusInfo{
		Status: rotation.StatusCompleted,
	}, nil
}

func (f *FakeRotationStrategy) GetCallCount() int {
	return int(atomic.LoadInt64(&f.callCount)) // Thread-safe read
}

func (f *FakeRotationStrategy) SetRotateFunc(fn func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error)) {
	f.rotateFunc = fn
}

func TestRotationWorkflow(t *testing.T) {
	t.Parallel()

	t.Run("simple_rotation_workflow", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		// Register a fake strategy
		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		err := engine.RegisterStrategy(strategy)
		require.NoError(t, err)

		// Create rotation request
		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "database/password",
				SecretType: "database",
				Metadata:   map[string]string{"environment": "test"},
			},
			Strategy: "immediate",
			DryRun:   false,
			Force:    false,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Perform rotation
		result, err := engine.Rotate(ctx, request)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, rotation.StatusCompleted, result.Status)
		assert.Equal(t, "database/password", result.Secret.Key)
		assert.NotNil(t, result.NewSecretRef)
		assert.NotEmpty(t, result.AuditTrail)
		assert.Equal(t, 1, strategy.GetCallCount())
	})

	t.Run("strategy_selection", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		// Register multiple strategies
		dbStrategy := NewFakeRotationStrategy("db-rotation", []rotation.SecretType{"database", "postgresql"})
		apiStrategy := NewFakeRotationStrategy("api-rotation", []rotation.SecretType{"api_key", "oauth"})

		err := engine.RegisterStrategy(dbStrategy)
		require.NoError(t, err)
		err = engine.RegisterStrategy(apiStrategy)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Test auto-selection for database
		dbSecret := rotation.SecretInfo{
			Key:        "db/password",
			SecretType: "database",
		}
		selectedStrategy, err := engine.AutoSelectStrategy(ctx, dbSecret)
		require.NoError(t, err)
		assert.Equal(t, "db-rotation", selectedStrategy)

		// Test auto-selection for API key
		apiSecret := rotation.SecretInfo{
			Key:        "api/key",
			SecretType: "api_key",
		}
		selectedStrategy, err = engine.AutoSelectStrategy(ctx, apiSecret)
		require.NoError(t, err)
		assert.Equal(t, "api-rotation", selectedStrategy)
	})

	t.Run("rotation_with_dry_run", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		// Track that dry run was respected
		strategy.SetRotateFunc(func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error) {
			now := time.Now()
			return &rotation.RotationResult{
				Secret:       req.Secret,
				Status:       rotation.StatusCompleted,
				NewSecretRef: &rotation.SecretReference{Identifier: "dry-run-v2"},
				RotatedAt:    &now,
				AuditTrail: []rotation.AuditEntry{
					{
						Timestamp: time.Now(),
						Action:    "dry_run",
						Component: "fake_strategy",
						Status:    "info",
						Message:   "Dry run completed",
						Details: map[string]interface{}{
							"dry_run": req.DryRun,
						},
					},
				},
			}, nil
		})

		err := engine.RegisterStrategy(strategy)
		require.NoError(t, err)

		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "database/password",
				SecretType: "database",
			},
			Strategy: "immediate",
			DryRun:   true,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := engine.Rotate(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, rotation.StatusCompleted, result.Status)
		assert.Equal(t, 1, strategy.GetCallCount())
	})

	t.Run("rotation_with_force_flag", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		forceFlagUsed := false
		strategy.SetRotateFunc(func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error) {
			forceFlagUsed = req.Force
			now := time.Now()
			return &rotation.RotationResult{
				Secret:       req.Secret,
				Status:       rotation.StatusCompleted,
				NewSecretRef: &rotation.SecretReference{Identifier: "v2"},
				RotatedAt:    &now,
			}, nil
		})

		err := engine.RegisterStrategy(strategy)
		require.NoError(t, err)

		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "database/password",
				SecretType: "database",
			},
			Strategy: "immediate",
			Force:    true,
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := engine.Rotate(ctx, request)
		require.NoError(t, err)
		assert.Equal(t, rotation.StatusCompleted, result.Status)
		assert.True(t, forceFlagUsed)
	})

	t.Run("list_available_strategies", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		// Initially empty
		strategies := engine.ListStrategies()
		assert.Empty(t, strategies)

		// Add strategies
		_ = engine.RegisterStrategy(NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"}))
		_ = engine.RegisterStrategy(NewFakeRotationStrategy("two-secret", []rotation.SecretType{"api_key"}))
		_ = engine.RegisterStrategy(NewFakeRotationStrategy("overlap", []rotation.SecretType{"oauth"}))

		strategies = engine.ListStrategies()
		assert.Len(t, strategies, 3)
		assert.Contains(t, strategies, "immediate")
		assert.Contains(t, strategies, "two-secret")
		assert.Contains(t, strategies, "overlap")
	})

	t.Run("get_strategy_by_name", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		expectedStrategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		_ = engine.RegisterStrategy(expectedStrategy)

		// Get existing strategy
		strategy, err := engine.GetStrategy("immediate")
		require.NoError(t, err)
		assert.Equal(t, "immediate", strategy.Name())

		// Get non-existent strategy
		_, err = engine.GetStrategy("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("rotation_with_audit_trail", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		strategy.SetRotateFunc(func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error) {
			now := time.Now()
			return &rotation.RotationResult{
				Secret:       req.Secret,
				Status:       rotation.StatusCompleted,
				NewSecretRef: &rotation.SecretReference{Identifier: "v2"},
				RotatedAt:    &now,
				AuditTrail: []rotation.AuditEntry{
					{
						Timestamp: time.Now(),
						Action:    "rotation_started",
						Component: "test_strategy",
						Status:    "info",
						Message:   "Starting rotation",
					},
					{
						Timestamp: time.Now(),
						Action:    "generate_new_secret",
						Component: "test_strategy",
						Status:    "success",
						Message:   "Generated new secret value",
					},
					{
						Timestamp: time.Now(),
						Action:    "update_provider",
						Component: "test_strategy",
						Status:    "success",
						Message:   "Updated secret in provider",
					},
					{
						Timestamp: time.Now(),
						Action:    "rotation_completed",
						Component: "test_strategy",
						Status:    "success",
						Message:   "Rotation completed successfully",
					},
				},
			}, nil
		})

		err := engine.RegisterStrategy(strategy)
		require.NoError(t, err)

		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "database/password",
				SecretType: "database",
			},
			Strategy: "immediate",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := engine.Rotate(ctx, request)
		require.NoError(t, err)

		// Verify audit trail is comprehensive
		// Engine adds its own entry, strategy adds 4 more
		assert.GreaterOrEqual(t, len(result.AuditTrail), 4)

		// Check audit entries have required fields
		for _, entry := range result.AuditTrail {
			assert.False(t, entry.Timestamp.IsZero())
			assert.NotEmpty(t, entry.Action)
			assert.NotEmpty(t, entry.Component)
			assert.NotEmpty(t, entry.Status)
			assert.NotEmpty(t, entry.Message)
		}
	})

	t.Run("rotation_with_config", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		receivedConfig := make(map[string]interface{})
		strategy.SetRotateFunc(func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error) {
			receivedConfig = req.Config
			now := time.Now()
			return &rotation.RotationResult{
				Secret:       req.Secret,
				Status:       rotation.StatusCompleted,
				NewSecretRef: &rotation.SecretReference{Identifier: "v2"},
				RotatedAt:    &now,
			}, nil
		})

		err := engine.RegisterStrategy(strategy)
		require.NoError(t, err)

		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "database/password",
				SecretType: "database",
			},
			Strategy: "immediate",
			Config: map[string]interface{}{
				"password_length":    32,
				"include_symbols":    true,
				"rotation_window_ms": 5000,
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err = engine.Rotate(ctx, request)
		require.NoError(t, err)

		// Config should be passed through
		assert.Equal(t, 32, receivedConfig["password_length"])
		assert.Equal(t, true, receivedConfig["include_symbols"])
		assert.Equal(t, 5000, receivedConfig["rotation_window_ms"])
	})
}

func TestRotationErrorHandling(t *testing.T) {
	t.Parallel()

	t.Run("unsupported_secret_type", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		// Strategy only supports database
		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		_ = engine.RegisterStrategy(strategy)

		// Try to rotate API key (unsupported)
		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "api/key",
				SecretType: "api_key", // Not supported by strategy
			},
			Strategy: "immediate",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := engine.Rotate(ctx, request)
		require.NoError(t, err) // No error, but status should be failed
		assert.Equal(t, rotation.StatusFailed, result.Status)
		assert.Contains(t, result.Error, "does not support")
	})

	t.Run("strategy_not_found", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "database/password",
				SecretType: "database",
			},
			Strategy: "nonexistent",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := engine.Rotate(ctx, request)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("rotation_failure", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		strategy.SetRotateFunc(func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error) {
			return &rotation.RotationResult{
				Secret: req.Secret,
				Status: rotation.StatusFailed,
				Error:  "Provider connection failed",
				AuditTrail: []rotation.AuditEntry{
					{
						Timestamp: time.Now(),
						Action:    "rotation_failed",
						Component: "fake_strategy",
						Status:    "error",
						Message:   "Provider connection failed",
					},
				},
			}, nil
		})

		err := engine.RegisterStrategy(strategy)
		require.NoError(t, err)

		request := rotation.RotationRequest{
			Secret: rotation.SecretInfo{
				Key:        "database/password",
				SecretType: "database",
			},
			Strategy: "immediate",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := engine.Rotate(ctx, request)
		require.NoError(t, err) // No error returned, but status is failed
		assert.Equal(t, rotation.StatusFailed, result.Status)
		assert.Contains(t, result.Error, "connection failed")
	})

	t.Run("duplicate_strategy_registration", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		strategy1 := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		strategy2 := NewFakeRotationStrategy("immediate", []rotation.SecretType{"api_key"})

		err := engine.RegisterStrategy(strategy1)
		require.NoError(t, err)

		// Second registration should fail
		err = engine.RegisterStrategy(strategy2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already registered")
	})

	t.Run("no_suitable_strategy_found", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		// Register strategy that doesn't support the secret type
		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		_ = engine.RegisterStrategy(strategy)

		// Try to auto-select for unsupported type
		secret := rotation.SecretInfo{
			Key:        "ssh/private_key",
			SecretType: "ssh_key",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := engine.AutoSelectStrategy(ctx, secret)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no suitable")
	})
}

func TestRotationConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("concurrent_rotations", func(t *testing.T) {
		t.Parallel()

		logger := logging.New(false, false)
		engine := rotation.NewRotationEngine(logger)

		strategy := NewFakeRotationStrategy("immediate", []rotation.SecretType{"database"})
		strategy.SetRotateFunc(func(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error) {
			// Small delay to simulate work
			time.Sleep(10 * time.Millisecond)
			now := time.Now()
			return &rotation.RotationResult{
				Secret:       req.Secret,
				Status:       rotation.StatusCompleted,
				NewSecretRef: &rotation.SecretReference{Identifier: "v2"},
				RotatedAt:    &now,
			}, nil
		})

		err := engine.RegisterStrategy(strategy)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Run multiple rotations concurrently
		numRotations := 10
		results := make(chan *rotation.RotationResult, numRotations)
		errors := make(chan error, numRotations)

		for i := 0; i < numRotations; i++ {
			i := i
			go func() {
				request := rotation.RotationRequest{
					Secret: rotation.SecretInfo{
						Key:        "database/password",
						SecretType: "database",
						Metadata:   map[string]string{"rotation_id": string(rune('0' + i))},
					},
					Strategy: "immediate",
				}

				result, err := engine.Rotate(ctx, request)
				if err != nil {
					errors <- err
				} else {
					results <- result
				}
			}()
		}

		// Collect results
		successCount := 0
		for i := 0; i < numRotations; i++ {
			select {
			case result := <-results:
				assert.Equal(t, rotation.StatusCompleted, result.Status)
				successCount++
			case err := <-errors:
				t.Errorf("Concurrent rotation failed: %v", err)
			case <-time.After(5 * time.Second):
				t.Error("Timeout waiting for rotation results")
			}
		}

		assert.Equal(t, numRotations, successCount)
		assert.Equal(t, numRotations, strategy.GetCallCount())
	})
}
