package rotation

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

// NOTE: fakeSecretValueRotator is defined in two_secret_test.go

func TestImmediateRotationStrategyName(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewImmediateRotationStrategy(baseRotator, logger)

	assert.Equal(t, "immediate-base", strategy.Name())
}

func TestImmediateRotationStrategySupportsSecret(t *testing.T) {
	logger := logging.New(false, true)

	tests := []struct {
		name           string
		secretType     SecretType
		baseSupport    bool
		expectedResult bool
	}{
		{
			name:           "base_supports",
			secretType:     SecretTypePassword,
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:           "base_not_supporting",
			secretType:     SecretTypePassword,
			baseSupport:    false,
			expectedResult: false,
		},
		{
			name:           "api_key_support",
			secretType:     SecretTypeAPIKey,
			baseSupport:    true,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseRotator := newFakeSecretValueRotator("base")
			if tt.baseSupport {
				baseRotator.SupportsAllTypes = true
			} else {
				baseRotator.SupportedTypes = []SecretType{}
			}

			strategy := NewImmediateRotationStrategy(baseRotator, logger)
			secret := SecretInfo{
				Key:        "test-secret",
				SecretType: tt.secretType,
			}

			result := strategy.SupportsSecret(context.Background(), secret)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestImmediateRotationStrategyRotate(t *testing.T) {
	logger := logging.New(false, true)

	t.Run("successful_rotation", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		strategy := NewImmediateRotationStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				Provider:   "test-provider",
				SecretType: SecretTypePassword,
			},
			NewValue: &NewSecretValue{Type: "password", Value: "new-password"},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusCompleted, result.Status)
		assert.NotNil(t, result.RotatedAt)
		assert.True(t, len(result.Warnings) > 0)
		assert.Contains(t, result.Warnings[0], "brief downtime")
		assert.True(t, len(result.AuditTrail) > 0)

		// Verify base rotator was called
		assert.Equal(t, 1, len(baseRotator.RotateCalls))
	})

	t.Run("dry_run_mode", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		strategy := NewImmediateRotationStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
			},
			DryRun: true,
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, result.Status)
		assert.Equal(t, 0, len(baseRotator.RotateCalls))
	})

	t.Run("rotation_failure", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.RotateFunc = func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			return nil, fmt.Errorf("rotation failed")
		}

		strategy := NewImmediateRotationStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.Error(t, err)
		assert.Equal(t, StatusFailed, result.Status)
		assert.Contains(t, result.Error, "immediate rotation failed")
	})

	t.Run("adds_warning_to_existing_warnings", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.RotateFunc = func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			now := time.Now()
			return &RotationResult{
				Secret:     req.Secret,
				Status:     StatusCompleted,
				Warnings:   []string{"existing warning"},
				RotatedAt:  &now,
				AuditTrail: []AuditEntry{},
			}, nil
		}

		strategy := NewImmediateRotationStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, 2, len(result.Warnings))
		assert.Equal(t, "existing warning", result.Warnings[0])
		assert.Contains(t, result.Warnings[1], "brief downtime")
	})

	t.Run("nil_warnings_initialized", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.RotateFunc = func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			now := time.Now()
			return &RotationResult{
				Secret:     req.Secret,
				Status:     StatusCompleted,
				Warnings:   nil, // nil slice
				RotatedAt:  &now,
				AuditTrail: []AuditEntry{},
			}, nil
		}

		strategy := NewImmediateRotationStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.NotNil(t, result.Warnings)
		assert.Equal(t, 1, len(result.Warnings))
	})

	t.Run("audit_trail_is_merged", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.RotateFunc = func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			now := time.Now()
			return &RotationResult{
				Secret:    req.Secret,
				Status:    StatusCompleted,
				RotatedAt: &now,
				AuditTrail: []AuditEntry{
					{Action: "base_action", Component: "base"},
				},
			}, nil
		}

		strategy := NewImmediateRotationStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)

		// Should have strategy entries first, then base entries, then completion
		hasStarted := false
		hasCompleted := false
		hasBaseAction := false
		for _, entry := range result.AuditTrail {
			if entry.Action == "immediate_rotation_started" {
				hasStarted = true
			}
			if entry.Action == "rotation_completed" {
				hasCompleted = true
			}
			if entry.Action == "base_action" {
				hasBaseAction = true
			}
		}
		assert.True(t, hasStarted)
		assert.True(t, hasCompleted)
		assert.True(t, hasBaseAction)
	})

	t.Run("rotated_at_is_set", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.RotateFunc = func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			return &RotationResult{
				Secret:     req.Secret,
				Status:     StatusCompleted,
				RotatedAt:  nil, // Base doesn't set this
				AuditTrail: []AuditEntry{},
			}, nil
		}

		strategy := NewImmediateRotationStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
			},
		}

		before := time.Now()
		result, err := strategy.Rotate(context.Background(), request)
		after := time.Now()

		require.NoError(t, err)
		require.NotNil(t, result.RotatedAt)
		assert.True(t, result.RotatedAt.After(before) || result.RotatedAt.Equal(before))
		assert.True(t, result.RotatedAt.Before(after) || result.RotatedAt.Equal(after))
	})
}

func TestImmediateRotationStrategyVerify(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewImmediateRotationStrategy(baseRotator, logger)

	request := VerificationRequest{
		Secret: SecretInfo{
			Key: "test-secret",
		},
	}

	err := strategy.Verify(context.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(baseRotator.VerifyCalls))
}

func TestImmediateRotationStrategyRollback(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewImmediateRotationStrategy(baseRotator, logger)

	request := RollbackRequest{
		Secret: SecretInfo{
			Key: "test-secret",
		},
	}

	err := strategy.Rollback(context.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(baseRotator.RollbackCalls))
}

func TestImmediateRotationStrategyGetStatus(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewImmediateRotationStrategy(baseRotator, logger)

	secret := SecretInfo{
		Key: "test-secret",
	}

	status, err := strategy.GetStatus(context.Background(), secret)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, 1, len(baseRotator.StatusCalls))
}
