package rotation

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

// fakeSecretValueRotator is a test double for SecretValueRotator
type fakeSecretValueRotator struct {
	mu sync.Mutex

	StrategyName     string
	SupportedTypes   []SecretType
	SupportsAllTypes bool

	RotateFunc   func(ctx context.Context, req RotationRequest) (*RotationResult, error)
	VerifyFunc   func(ctx context.Context, req VerificationRequest) error
	RollbackFunc func(ctx context.Context, req RollbackRequest) error
	StatusFunc   func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error)

	RotateCalls   []RotationRequest
	VerifyCalls   []VerificationRequest
	RollbackCalls []RollbackRequest
	StatusCalls   []SecretInfo
}

func newFakeSecretValueRotator(name string) *fakeSecretValueRotator {
	return &fakeSecretValueRotator{
		StrategyName:   name,
		SupportedTypes: []SecretType{SecretTypePassword},
		RotateCalls:    make([]RotationRequest, 0),
		VerifyCalls:    make([]VerificationRequest, 0),
		RollbackCalls:  make([]RollbackRequest, 0),
		StatusCalls:    make([]SecretInfo, 0),
	}
}

func (f *fakeSecretValueRotator) Name() string { return f.StrategyName }

func (f *fakeSecretValueRotator) SupportsSecret(_ context.Context, secret SecretInfo) bool {
	if f.SupportsAllTypes {
		return true
	}
	for _, t := range f.SupportedTypes {
		if t == secret.SecretType {
			return true
		}
	}
	return false
}

func (f *fakeSecretValueRotator) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	f.mu.Lock()
	f.RotateCalls = append(f.RotateCalls, request)
	f.mu.Unlock()

	if f.RotateFunc != nil {
		return f.RotateFunc(ctx, request)
	}

	now := time.Now()
	return &RotationResult{
		Secret:       request.Secret,
		Status:       StatusCompleted,
		NewSecretRef: &SecretReference{Provider: request.Secret.Provider, Key: request.Secret.Key + "_new", Version: "v2"},
		OldSecretRef: &SecretReference{Provider: request.Secret.Provider, Key: request.Secret.Key, Version: "v1"},
		RotatedAt:    &now,
		AuditTrail:   []AuditEntry{},
	}, nil
}

func (f *fakeSecretValueRotator) Verify(ctx context.Context, request VerificationRequest) error {
	f.mu.Lock()
	f.VerifyCalls = append(f.VerifyCalls, request)
	f.mu.Unlock()
	if f.VerifyFunc != nil {
		return f.VerifyFunc(ctx, request)
	}
	return nil
}

func (f *fakeSecretValueRotator) Rollback(ctx context.Context, request RollbackRequest) error {
	f.mu.Lock()
	f.RollbackCalls = append(f.RollbackCalls, request)
	f.mu.Unlock()
	if f.RollbackFunc != nil {
		return f.RollbackFunc(ctx, request)
	}
	return nil
}

func (f *fakeSecretValueRotator) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	f.mu.Lock()
	f.StatusCalls = append(f.StatusCalls, secret)
	f.mu.Unlock()
	if f.StatusFunc != nil {
		return f.StatusFunc(ctx, secret)
	}
	lastRotated := time.Now().Add(-24 * time.Hour)
	nextRotation := time.Now().Add(30 * 24 * time.Hour)
	return &RotationStatusInfo{
		Status:       StatusCompleted,
		LastRotated:  &lastRotated,
		NextRotation: &nextRotation,
		CanRotate:    true,
	}, nil
}

// fakeTwoSecretRotator implements TwoSecretRotator
type fakeTwoSecretRotator struct {
	fakeSecretValueRotator

	CreateSecondaryFunc  func(ctx context.Context, req SecondarySecretRequest) (*SecretReference, error)
	PromoteSecondaryFunc func(ctx context.Context, req PromoteRequest) error
	DeprecatePrimaryFunc func(ctx context.Context, req DeprecateRequest) error

	CreateSecondaryCalls  []SecondarySecretRequest
	PromoteSecondaryCalls []PromoteRequest
	DeprecatePrimaryCalls []DeprecateRequest
}

func newFakeTwoSecretRotator(name string) *fakeTwoSecretRotator {
	return &fakeTwoSecretRotator{
		fakeSecretValueRotator: *newFakeSecretValueRotator(name),
		CreateSecondaryCalls:   make([]SecondarySecretRequest, 0),
		PromoteSecondaryCalls:  make([]PromoteRequest, 0),
		DeprecatePrimaryCalls:  make([]DeprecateRequest, 0),
	}
}

func (f *fakeTwoSecretRotator) CreateSecondarySecret(ctx context.Context, request SecondarySecretRequest) (*SecretReference, error) {
	f.mu.Lock()
	f.CreateSecondaryCalls = append(f.CreateSecondaryCalls, request)
	f.mu.Unlock()
	if f.CreateSecondaryFunc != nil {
		return f.CreateSecondaryFunc(ctx, request)
	}
	return &SecretReference{
		Provider:   request.Secret.Provider,
		Key:        request.Secret.Key + "_secondary",
		Version:    "v2",
		Identifier: "secondary_" + request.Secret.Key,
	}, nil
}

func (f *fakeTwoSecretRotator) PromoteSecondarySecret(ctx context.Context, request PromoteRequest) error {
	f.mu.Lock()
	f.PromoteSecondaryCalls = append(f.PromoteSecondaryCalls, request)
	f.mu.Unlock()
	if f.PromoteSecondaryFunc != nil {
		return f.PromoteSecondaryFunc(ctx, request)
	}
	return nil
}

func (f *fakeTwoSecretRotator) DeprecatePrimarySecret(ctx context.Context, request DeprecateRequest) error {
	f.mu.Lock()
	f.DeprecatePrimaryCalls = append(f.DeprecatePrimaryCalls, request)
	f.mu.Unlock()
	if f.DeprecatePrimaryFunc != nil {
		return f.DeprecatePrimaryFunc(ctx, request)
	}
	return nil
}

func TestTwoSecretStrategyName(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeSecretValueRotator("base")
	strategy := NewTwoSecretStrategy(baseRotator, logger)

	assert.Equal(t, "two-secret-base", strategy.Name())
}

func TestTwoSecretStrategySupportsSecret(t *testing.T) {
	logger := logging.New(false, true)

	tests := []struct {
		name           string
		secretType     SecretType
		baseSupport    bool
		expectedResult bool
	}{
		{
			name:           "supports_password",
			secretType:     SecretTypePassword,
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:           "supports_api_key",
			secretType:     SecretTypeAPIKey,
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:           "supports_oauth",
			secretType:     SecretTypeOAuth,
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:           "supports_certificate",
			secretType:     SecretTypeCertificate,
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:           "supports_encryption",
			secretType:     SecretTypeEncryption,
			baseSupport:    true,
			expectedResult: true,
		},
		{
			name:           "unsupported_type",
			secretType:     SecretType("unsupported"),
			baseSupport:    true,
			expectedResult: false,
		},
		{
			name:           "base_not_supporting",
			secretType:     SecretTypePassword,
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
				baseRotator.SupportedTypes = []SecretType{} // Empty = no support
			}

			strategy := NewTwoSecretStrategy(baseRotator, logger)
			secret := SecretInfo{
				Key:        "test-secret",
				SecretType: tt.secretType,
			}

			result := strategy.SupportsSecret(context.Background(), secret)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestTwoSecretStrategyRotate(t *testing.T) {
	logger := logging.New(false, true)

	t.Run("successful_rotation_with_two_secret_support", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				Provider:   "test-provider",
				SecretType: SecretTypePassword,
				Constraints: &RotationConstraints{
					MinRotationInterval: 1 * time.Hour,
					GracePeriod:         30 * time.Minute,
				},
			},
			NewValue: &NewSecretValue{Type: "password", Value: "new-password"},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, StatusCompleted, result.Status)
		assert.NotNil(t, result.NewSecretRef)
		assert.NotNil(t, result.RotatedAt)
		assert.NotNil(t, result.ExpiresAt)
		assert.True(t, len(result.AuditTrail) > 0)

		// Verify the calls were made
		assert.Equal(t, 1, len(baseRotator.CreateSecondaryCalls))
		assert.Equal(t, 1, len(baseRotator.PromoteSecondaryCalls))
		assert.Equal(t, 1, len(baseRotator.VerifyCalls))
	})

	t.Run("dry_run_mode", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				Provider:   "test-provider",
				SecretType: SecretTypePassword,
				Constraints: &RotationConstraints{},
			},
			DryRun: true,
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, result.Status)
		assert.Equal(t, 0, len(baseRotator.CreateSecondaryCalls))
	})

	t.Run("fallback_to_regular_rotation", func(t *testing.T) {
		// Base rotator doesn't implement TwoSecretRotator
		baseRotator := newFakeSecretValueRotator("base")
		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				Provider:   "test-provider",
				SecretType: SecretTypePassword,
				Constraints: &RotationConstraints{},
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusCompleted, result.Status)
		assert.Equal(t, 1, len(baseRotator.RotateCalls))
	})

	t.Run("respects_minimum_rotation_interval", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		// Set status to indicate recent rotation
		lastRotated := time.Now().Add(-10 * time.Minute) // 10 mins ago
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return &RotationStatusInfo{
				Status:      StatusCompleted,
				LastRotated: &lastRotated,
			}, nil
		}

		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key: "test-secret",
				Constraints: &RotationConstraints{
					MinRotationInterval: 1 * time.Hour, // Requires 1 hour between rotations
				},
			},
			Force: false,
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusPending, result.Status)
		assert.Contains(t, result.Error, "rotation too recent")
	})

	t.Run("force_overrides_minimum_interval", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		lastRotated := time.Now().Add(-10 * time.Minute)
		baseRotator.StatusFunc = func(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
			return &RotationStatusInfo{
				Status:      StatusCompleted,
				LastRotated: &lastRotated,
			}, nil
		}

		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key: "test-secret",
				Constraints: &RotationConstraints{
					MinRotationInterval: 1 * time.Hour,
				},
			},
			Force: true,
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.NoError(t, err)
		assert.Equal(t, StatusCompleted, result.Status)
	})

	t.Run("secondary_creation_failure", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		baseRotator.CreateSecondaryFunc = func(ctx context.Context, req SecondarySecretRequest) (*SecretReference, error) {
			return nil, fmt.Errorf("failed to create secondary")
		}

		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
				Constraints: &RotationConstraints{},
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.Error(t, err)
		assert.Equal(t, StatusFailed, result.Status)
		assert.Contains(t, result.Error, "failed to create secondary")
	})

	t.Run("verification_failure_with_cleanup", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		baseRotator.VerifyFunc = func(ctx context.Context, req VerificationRequest) error {
			return fmt.Errorf("verification failed")
		}

		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
				Constraints: &RotationConstraints{},
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.Error(t, err)
		assert.Equal(t, StatusFailed, result.Status)
		assert.Contains(t, result.Error, "verification failed")
		// Should have attempted cleanup
		assert.Equal(t, 1, len(baseRotator.DeprecatePrimaryCalls))
	})

	t.Run("promotion_failure", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		baseRotator.PromoteSecondaryFunc = func(ctx context.Context, req PromoteRequest) error {
			return fmt.Errorf("promotion failed")
		}

		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RotationRequest{
			Secret: SecretInfo{
				Key:        "test-secret",
				SecretType: SecretTypePassword,
				Constraints: &RotationConstraints{},
			},
		}

		result, err := strategy.Rotate(context.Background(), request)
		require.Error(t, err)
		assert.Equal(t, StatusFailed, result.Status)
		assert.Contains(t, result.Error, "failed to promote")
	})
}

func TestTwoSecretStrategyVerify(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeTwoSecretRotator("base")
	strategy := NewTwoSecretStrategy(baseRotator, logger)

	request := VerificationRequest{
		Secret: SecretInfo{
			Key: "test-secret",
		},
	}

	err := strategy.Verify(context.Background(), request)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(baseRotator.VerifyCalls))
}

func TestTwoSecretStrategyRollback(t *testing.T) {
	logger := logging.New(false, true)

	t.Run("with_two_secret_support", func(t *testing.T) {
		baseRotator := newFakeTwoSecretRotator("base")
		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RollbackRequest{
			Secret: SecretInfo{
				Key: "test-secret",
			},
			OldSecretRef: SecretReference{
				Key: "old-secret",
			},
		}

		err := strategy.Rollback(context.Background(), request)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(baseRotator.PromoteSecondaryCalls))
	})

	t.Run("without_two_secret_support", func(t *testing.T) {
		baseRotator := newFakeSecretValueRotator("base")
		strategy := NewTwoSecretStrategy(baseRotator, logger)

		request := RollbackRequest{
			Secret: SecretInfo{
				Key: "test-secret",
			},
		}

		err := strategy.Rollback(context.Background(), request)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(baseRotator.RollbackCalls))
	})
}

func TestTwoSecretStrategyGetStatus(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeTwoSecretRotator("base")
	strategy := NewTwoSecretStrategy(baseRotator, logger)

	secret := SecretInfo{
		Key: "test-secret",
	}

	status, err := strategy.GetStatus(context.Background(), secret)
	assert.NoError(t, err)
	assert.NotNil(t, status)
	assert.Equal(t, 1, len(baseRotator.StatusCalls))
}

func TestTwoSecretStrategyGetTwoSecretStatus(t *testing.T) {
	logger := logging.New(false, true)
	baseRotator := newFakeTwoSecretRotator("base")
	strategy := NewTwoSecretStrategy(baseRotator, logger)

	secret := SecretInfo{
		Key: "test-secret",
	}

	status, err := strategy.GetTwoSecretStatus(context.Background(), secret)
	assert.Error(t, err) // Not yet implemented
	assert.Nil(t, status)
}
