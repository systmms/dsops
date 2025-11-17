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

func TestNewOverlapRotationStrategy(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")

	t.Run("default_values", func(t *testing.T) {
		strategy := NewOverlapRotationStrategy(baseRotator, logger, 0, 0)
		assert.Equal(t, 7*24*time.Hour, strategy.overlapPeriod)
		assert.Equal(t, 90*24*time.Hour, strategy.totalValidity)
	})

	t.Run("custom_values", func(t *testing.T) {
		strategy := NewOverlapRotationStrategy(baseRotator, logger, 2*24*time.Hour, 30*24*time.Hour)
		assert.Equal(t, 2*24*time.Hour, strategy.overlapPeriod)
		assert.Equal(t, 30*24*time.Hour, strategy.totalValidity)
	})
}

func TestOverlapRotationStrategyName(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewOverlapRotationStrategy(baseRotator, logger, time.Hour, 24*time.Hour)

	assert.Equal(t, "overlap-base", strategy.Name())
}

func TestOverlapRotationStrategySupportsSecret(t *testing.T) {
	logger := logging.New(false, true)

	tests := []struct {
		name           string
		secretType     SecretType
		metadata       map[string]string
		baseSupport    bool
		expectedResult bool
	}{
		{
			name:           "supports_certificate",
			secretType:     SecretTypeCertificate,
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:       "supports_api_key_with_expiration",
			secretType: SecretTypeAPIKey,
			metadata: map[string]string{
				"supports_expiration": "true",
			},
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:       "supports_oauth_with_expiration",
			secretType: SecretTypeOAuth,
			metadata: map[string]string{
				"supports_expiration": "true",
			},
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:           "no_support_for_api_key_without_expiration",
			secretType:     SecretTypeAPIKey,
			metadata:       map[string]string{},
			baseSupport:    true,
			expectedResult: false,
		},
		{
			name:           "no_support_for_password",
			secretType:     SecretTypePassword,
			baseSupport:    true,
			expectedResult: false,
		},
		{
			name:           "base_not_supporting",
			secretType:     SecretTypeCertificate,
			baseSupport:    false,
			expectedResult: false,
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

			strategy := NewOverlapRotationStrategy(baseRotator, logger, time.Hour, 24*time.Hour)
			secret := SecretInfo{
				Key:        "test-secret",
				SecretType: tt.secretType,
				Metadata:   tt.metadata,
			}

			result := strategy.SupportsSecret(context.Background(), secret)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestOverlapRotationStrategyRotate(t *testing.T) {
	logger := logging.New(false, true)
	overlapPeriod := 7 * 24 * time.Hour
	totalValidity := 90 * 24 * time.Hour

	t.Run("successful_rotation", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		// Override status to not block rotation
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return nil, nil
		}
		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				Provider:   "test-provider",
				SecretType: SecretTypeCertificate,
			},
			NewValue: &NewSecretValue{Type: "certificate", Value: "new-cert"},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusCompleted, result.Status)
		assert.NotNil(t, result.ExpiresAt)
		require.True(t, len(result.Warnings) > 0)
		assert.Contains(t, result.Warnings[0], "Overlap period active")
		assert.True(t, len(result.AuditTrail) > 0)

		// Verify base rotator was called with proper config
		assert.Equal(t, 1, len(baseRotator.RotateCalls))
		rotateRequest := baseRotator.RotateCalls[0]
		assert.NotNil(t, rotateRequest.Config["valid_from"])
		assert.NotNil(t, rotateRequest.Config["valid_until"])
	})

	t.Run("dry_run_mode", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return nil, nil
		}
		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypeCertificate,
			},
			DryRun: true,
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, result.Status)
		assert.NotNil(t, result.ExpiresAt)
		assert.Equal(t, 0, len(baseRotator.RotateCalls))
	})

	t.Run("respects_rotation_schedule", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		nextRotation := time.Now().Add(24 * time.Hour) // Next rotation is tomorrow
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return &RotationStatusInfo{
				Status:       StatusCompleted,
				NextRotation: &nextRotation,
			}, nil
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypeCertificate,
			},
			Force: false,
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, result.Status)
		assert.Contains(t, result.Error, "Too early to rotate")
	})

	t.Run("force_overrides_schedule", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		nextRotation := time.Now().Add(24 * time.Hour)
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return &RotationStatusInfo{
				Status:       StatusCompleted,
				NextRotation: &nextRotation,
			}, nil
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypeCertificate,
			},
			Force: true,
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusCompleted, result.Status)
	})

	t.Run("rotation_failure", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return nil, nil
		}
		baseRotator.RotateFunc = func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			return nil, fmt.Errorf("rotation failed")
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypeCertificate,
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.Error(t, err)
		assert.Equal(t, StatusFailed, result.Status)
		assert.Contains(t, result.Error, "overlap rotation failed")
	})

	t.Run("nil_config_is_initialized", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return nil, nil
		}
		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypeCertificate,
			},
			Config: nil, // Will be initialized
		}

		_, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		require.Greater(t, len(baseRotator.RotateCalls), 0)
		assert.NotNil(t, baseRotator.RotateCalls[0].Config)
	})

	t.Run("with_old_secret_ref_in_result", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return nil, nil
		}
		baseRotator.RotateFunc = func(ctx context.Context, req RotationRequest) (*RotationResult, error) {
			now := time.Now()
			return &RotationResult{
				Secret:       req.Secret,
				Status:       StatusCompleted,
				OldSecretRef: &SecretReference{Key: "old-key"},
				NewSecretRef: &SecretReference{Key: "new-key", Metadata: make(map[string]string)},
				RotatedAt:    &now,
				AuditTrail:   []AuditEntry{},
			}, nil
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypeCertificate,
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusCompleted, result.Status)
		// Should have audit entry for scheduling expiration
		hasSchedulingEntry := false
		for _, entry := range result.AuditTrail {
			if entry.Action == "scheduling_expiration" {
				hasSchedulingEntry = true
				break
			}
		}
		assert.True(t, hasSchedulingEntry)
	})
}

func TestOverlapRotationStrategyVerify(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewOverlapRotationStrategy(baseRotator, logger, time.Hour, 24*time.Hour)

	request := VerificationRequest{
		Secret: SecretInfo{
			Key: "test-secret",
		},
	}

	err := strategy.Verify(context.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(baseRotator.VerifyCalls))
}

func TestOverlapRotationStrategyRollback(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewOverlapRotationStrategy(baseRotator, logger, time.Hour, 24*time.Hour)

	request := RollbackRequest{
		Secret: SecretInfo{
			Key: "test-secret",
		},
	}

	err := strategy.Rollback(context.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(baseRotator.RollbackCalls))
}

func TestOverlapRotationStrategyGetStatus(t *testing.T) {
	logger := logging.New(false, true)
	overlapPeriod := 7 * 24 * time.Hour
	totalValidity := 90 * 24 * time.Hour

	t.Run("enhances_status_with_overlap_info", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		lastRotated := time.Now().Add(-24 * time.Hour)
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return &RotationStatusInfo{
				Status:      StatusCompleted,
				LastRotated: &lastRotated,
				Reason:      "",
			}, nil
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)
		secret := SecretInfo{
			Key: "test-secret",
		}

		status, err := strategy.GetStatus(context.Background(), secret)
		require.NoError(t, err)
		assert.NotNil(t, status.NextRotation)
		assert.Contains(t, status.Reason, "Overlap rotation")
	})

	t.Run("preserves_existing_reason", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		lastRotated := time.Now().Add(-24 * time.Hour)
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return &RotationStatusInfo{
				Status:      StatusCompleted,
				LastRotated: &lastRotated,
				Reason:      "Custom reason",
			}, nil
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)
		secret := SecretInfo{
			Key: "test-secret",
		}

		status, err := strategy.GetStatus(context.Background(), secret)
		require.NoError(t, err)
		assert.Equal(t, "Custom reason", status.Reason)
	})

	t.Run("handles_nil_status", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return nil, nil
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)
		secret := SecretInfo{
			Key: "test-secret",
		}

		status, err := strategy.GetStatus(context.Background(), secret)
		require.NoError(t, err)
		assert.Nil(t, status)
	})

	t.Run("handles_error", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return nil, fmt.Errorf("status error")
		}

		strategy := NewOverlapRotationStrategy(baseRotator, logger, overlapPeriod, totalValidity)
		secret := SecretInfo{
			Key: "test-secret",
		}

		status, err := strategy.GetStatus(context.Background(), secret)
		assert.Error(t, err)
		assert.Nil(t, status)
	})
}
