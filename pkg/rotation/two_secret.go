package rotation

import (
	"context"
	"fmt"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

// TwoSecretStrategy implements zero-downtime rotation using the two-secret approach
// pioneered by Doppler. It maintains two instances of each secret (active/inactive)
// and alternates between them during rotation.
type TwoSecretStrategy struct {
	baseStrategy SecretValueRotator
	logger       *logging.Logger
}

// NewTwoSecretStrategy wraps any SecretValueRotator with two-secret capabilities
func NewTwoSecretStrategy(baseStrategy SecretValueRotator, logger *logging.Logger) *TwoSecretStrategy {
	return &TwoSecretStrategy{
		baseStrategy: baseStrategy,
		logger:       logger,
	}
}

// Name returns the strategy name with two-secret prefix
func (s *TwoSecretStrategy) Name() string {
	return fmt.Sprintf("two-secret-%s", s.baseStrategy.Name())
}

// SupportsSecret checks if the base strategy supports this secret
func (s *TwoSecretStrategy) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	// Check if base strategy supports it
	if !s.baseStrategy.SupportsSecret(ctx, secret) {
		return false
	}

	// Check if the secret type is compatible with two-secret approach
	switch secret.SecretType {
	case SecretTypePassword, SecretTypeAPIKey, SecretTypeOAuth:
		return true
	case SecretTypeCertificate:
		// Certificates can work but need special handling for domains
		return true
	case SecretTypeEncryption:
		// Application encryption keys can use two-secret if properly designed
		return true
	default:
		return false
	}
}

// Rotate implements zero-downtime rotation using the two-secret pattern
func (s *TwoSecretStrategy) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	auditTrail := []AuditEntry{
		createAuditEntry("two_secret_rotation_started", "two_secret_strategy", "info",
			"Starting zero-downtime rotation", map[string]interface{}{
				"secret_key": logging.Secret(request.Secret.Key),
				"strategy":   s.baseStrategy.Name(),
			}),
	}

	s.logger.Info("Starting two-secret rotation for %s", logging.Secret(request.Secret.Key))

	// Step 1: Check current status
	status, err := s.GetStatus(ctx, request.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to get rotation status: %w", err)
	}

	if !request.Force && status != nil && status.LastRotated != nil {
		if time.Since(*status.LastRotated) < request.Secret.Constraints.MinRotationInterval {
			return &RotationResult{
				Secret:     request.Secret,
				Status:     StatusPending,
				Error:      fmt.Sprintf("rotation too recent, last rotated %v ago", time.Since(*status.LastRotated)),
				AuditTrail: auditTrail,
			}, nil
		}
	}

	// Step 2: Create secondary secret (inactive)
	secondaryReq := SecondarySecretRequest{
		Secret:   request.Secret,
		NewValue: request.NewValue,
		Config:   request.Config,
	}

	if request.DryRun {
		auditTrail = append(auditTrail, createAuditEntry("dry_run_secondary_creation", "two_secret_strategy", "info",
			"Would create secondary secret", nil))
		
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusPending,
			AuditTrail: auditTrail,
		}, nil
	}

	// Create the secondary secret
	twoSecretRotator, ok := s.baseStrategy.(TwoSecretRotator)
	if !ok {
		// Fallback to regular rotation if base strategy doesn't support two-secret
		s.logger.Warn("Base strategy doesn't support two-secret, falling back to regular rotation")
		return s.baseStrategy.Rotate(ctx, request)
	}

	secondaryRef, err := twoSecretRotator.CreateSecondarySecret(ctx, secondaryReq)
	if err != nil {
		auditTrail = append(auditTrail, createAuditEntry("secondary_creation_failed", "two_secret_strategy", "error",
			"Failed to create secondary secret", map[string]interface{}{"error": err.Error()}))
		
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("failed to create secondary secret: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	auditTrail = append(auditTrail, createAuditEntry("secondary_created", "two_secret_strategy", "info",
		"Created secondary secret", map[string]interface{}{
			"secondary_ref": secondaryRef.Identifier,
		}))

	s.logger.Info("Created secondary secret %s for %s", secondaryRef.Identifier, logging.Secret(request.Secret.Key))

	// Step 3: Verify the secondary secret works
	verifyReq := VerificationRequest{
		Secret:       request.Secret,
		NewSecretRef: *secondaryRef,
		Tests:        request.Secret.Constraints.RequiredTests,
		Timeout:      30 * time.Second,
	}

	if err := s.Verify(ctx, verifyReq); err != nil {
		// Verification failed, clean up secondary secret
		s.logger.Error("Secondary secret verification failed: %v", err)
		
		// Attempt cleanup (best effort)
		deprecateReq := DeprecateRequest{
			Secret:      request.Secret,
			OldRef:      *secondaryRef,
			GracePeriod: 0, // Immediate cleanup
			HardDelete:  true,
		}
		if cleanupErr := twoSecretRotator.DeprecatePrimarySecret(ctx, deprecateReq); cleanupErr != nil {
			s.logger.Error("Failed to cleanup secondary secret after verification failure: %v", cleanupErr)
		}

		auditTrail = append(auditTrail, createAuditEntry("verification_failed", "two_secret_strategy", "error",
			"Secondary secret verification failed", map[string]interface{}{"error": err.Error()}))

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("secondary secret verification failed: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	auditTrail = append(auditTrail, createAuditEntry("verification_passed", "two_secret_strategy", "info",
		"Secondary secret verification passed", nil))

	// Step 4: Promote secondary to primary
	promoteReq := PromoteRequest{
		Secret:       request.Secret,
		SecondaryRef: *secondaryRef,
		GracePeriod:  request.Secret.Constraints.GracePeriod,
		VerifyFirst:  false, // Already verified
	}

	if err := twoSecretRotator.PromoteSecondarySecret(ctx, promoteReq); err != nil {
		auditTrail = append(auditTrail, createAuditEntry("promotion_failed", "two_secret_strategy", "error",
			"Failed to promote secondary secret", map[string]interface{}{"error": err.Error()}))

		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("failed to promote secondary secret: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	auditTrail = append(auditTrail, createAuditEntry("promotion_completed", "two_secret_strategy", "info",
		"Promoted secondary secret to primary", nil))

	s.logger.Info("Promoted secondary secret to primary for %s", logging.Secret(request.Secret.Key))

	// Step 5: Schedule deprecation of old primary (after grace period)
	// This would typically be done asynchronously
	rotatedAt := time.Now()
	expiresAt := rotatedAt.Add(promoteReq.GracePeriod)

	auditTrail = append(auditTrail, createAuditEntry("rotation_completed", "two_secret_strategy", "info",
		fmt.Sprintf("Zero-downtime rotation completed, old secret expires at %v", expiresAt), nil))

	return &RotationResult{
		Secret:       request.Secret,
		Status:       StatusCompleted,
		NewSecretRef: secondaryRef,
		RotatedAt:    &rotatedAt,
		ExpiresAt:    &expiresAt,
		AuditTrail:   auditTrail,
	}, nil
}

// Verify uses the base strategy's verification
func (s *TwoSecretStrategy) Verify(ctx context.Context, request VerificationRequest) error {
	return s.baseStrategy.Verify(ctx, request)
}

// Rollback implements rollback by promoting the old secret back to primary
func (s *TwoSecretStrategy) Rollback(ctx context.Context, request RollbackRequest) error {
	s.logger.Info("Rolling back two-secret rotation for %s", logging.Secret(request.Secret.Key))
	
	twoSecretRotator, ok := s.baseStrategy.(TwoSecretRotator)
	if !ok {
		return s.baseStrategy.Rollback(ctx, request)
	}

	// Promote the old secret back to primary
	promoteReq := PromoteRequest{
		Secret:       request.Secret,
		SecondaryRef: request.OldSecretRef,
		GracePeriod:  5 * time.Minute, // Short grace period for rollback
		VerifyFirst:  true,
	}

	return twoSecretRotator.PromoteSecondarySecret(ctx, promoteReq)
}

// GetStatus delegates to the base strategy
func (s *TwoSecretStrategy) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	return s.baseStrategy.GetStatus(ctx, secret)
}

// TwoSecretMetadata contains information about the two-secret setup
type TwoSecretMetadata struct {
	ActiveSecret   SecretReference `json:"active_secret"`
	InactiveSecret *SecretReference `json:"inactive_secret,omitempty"`
	LastRotated    *time.Time      `json:"last_rotated,omitempty"`
	NextRotation   *time.Time      `json:"next_rotation,omitempty"`
	GracePeriod    time.Duration   `json:"grace_period"`
}

// GetTwoSecretStatus returns detailed status for two-secret rotation
func (s *TwoSecretStrategy) GetTwoSecretStatus(ctx context.Context, secret SecretInfo) (*TwoSecretMetadata, error) {
	// This would query the provider for both active and inactive secrets
	// Implementation would depend on the specific provider
	return nil, fmt.Errorf("two-secret status not yet implemented")
}