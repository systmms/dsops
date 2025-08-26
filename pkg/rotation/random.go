package rotation

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

// RandomRotator implements a simple random value rotation for testing and generic use
type RandomRotator struct {
	logger *logging.Logger
}

// NewRandomRotator creates a new random rotation strategy
func NewRandomRotator(logger *logging.Logger) *RandomRotator {
	return &RandomRotator{
		logger: logger,
	}
}

// Name returns the strategy name
func (r *RandomRotator) Name() string {
	return "random"
}

// SupportsSecret checks if this strategy can rotate the given secret
func (r *RandomRotator) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	// Random strategy supports all secret types for testing purposes
	return true
}

// Rotate generates a new random value and simulates rotation
func (r *RandomRotator) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	auditTrail := []AuditEntry{
		{
			Timestamp: time.Now(),
			Action:    "random_rotation_started",
			Component: "random_rotator",
			Status:    "info",
			Message:   "Starting random value rotation",
			Details: map[string]interface{}{
				"secret_key": logging.Secret(request.Secret.Key),
				"dry_run":    request.DryRun,
			},
		},
	}

	r.logger.Info("Starting random rotation for %s", logging.Secret(request.Secret.Key))

	// Generate new random value
	newValue, err := r.generateRandomValue(request.NewValue)
	if err != nil {
		auditTrail = append(auditTrail, AuditEntry{
			Timestamp: time.Now(),
			Action:    "random_generation_failed",
			Component: "random_rotator",
			Status:    "error",
			Message:   "Failed to generate random value",
			Error:     err.Error(),
		})

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("failed to generate random value: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	if request.DryRun {
		auditTrail = append(auditTrail, AuditEntry{
			Timestamp: time.Now(),
			Action:    "dry_run_simulation",
			Component: "random_rotator",
			Status:    "info",
			Message:   fmt.Sprintf("Would generate new random value of length %d", len(newValue)),
		})

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusPending,
			AuditTrail: auditTrail,
		}, nil
	}

	// Simulate successful rotation
	newSecretRef := &SecretReference{
		Provider:   request.Secret.Provider,
		Key:        request.Secret.ProviderRef.Key,
		Version:    fmt.Sprintf("random-%d", time.Now().Unix()),
		Identifier: "random-generated",
		Metadata: map[string]string{
			"rotated_at": time.Now().UTC().Format(time.RFC3339),
			"strategy":   "random",
			"length":     fmt.Sprintf("%d", len(newValue)),
		},
	}

	rotatedAt := time.Now()
	auditTrail = append(auditTrail, AuditEntry{
		Timestamp: time.Now(),
		Action:    "rotation_completed",
		Component: "random_rotator",
		Status:    "info",
		Message:   "Random value rotation completed successfully",
	})

	r.logger.Info("Successfully rotated random value for %s", logging.Secret(request.Secret.Key))

	return &RotationResult{
		Secret:       request.Secret,
		Status:       StatusCompleted,
		NewSecretRef: newSecretRef,
		RotatedAt:    &rotatedAt,
		AuditTrail:   auditTrail,
	}, nil
}

// Verify simulates verification of the random value
func (r *RandomRotator) Verify(ctx context.Context, request VerificationRequest) error {
	r.logger.Debug("Verifying random value for %s", logging.Secret(request.Secret.Key))
	// Random values are always "valid" for testing purposes
	return nil
}

// Rollback simulates rollback for random values
func (r *RandomRotator) Rollback(ctx context.Context, request RollbackRequest) error {
	r.logger.Info("Rolling back random value for %s", logging.Secret(request.Secret.Key))
	// For random values, we can't really rollback, so we just log it
	return nil
}

// GetStatus returns the current rotation status for random values
func (r *RandomRotator) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	// Random values don't have persistent state
	return &RotationStatusInfo{
		Status:    StatusPending,
		CanRotate: true,
		Reason:    "Random values can always be rotated",
	}, nil
}

// generateRandomValue creates a new random value based on the specification
func (r *RandomRotator) generateRandomValue(newValue *NewSecretValue) ([]byte, error) {
	if newValue != nil {
		switch newValue.Type {
		case ValueTypeLiteral:
			return []byte(newValue.Value), nil
		case ValueTypeGenerated, ValueTypeRandom:
			// Use config to determine length
			if lengthVal, ok := newValue.Config["length"]; ok {
				if length, ok := lengthVal.(int); ok {
					return r.generateRandomBytes(length)
				}
			}
		}
	}

	// Default: generate 32-character random string
	return r.generateRandomBytes(32)
}

// generateRandomBytes creates cryptographically secure random bytes
func (r *RandomRotator) generateRandomBytes(length int) ([]byte, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	
	randomBytes := make([]byte, length)
	charsetBytes := make([]byte, length)
	
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	for i := 0; i < length; i++ {
		charsetBytes[i] = charset[randomBytes[i]%byte(len(charset))]
	}
	
	return charsetBytes, nil
}