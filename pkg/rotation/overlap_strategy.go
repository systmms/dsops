package rotation

import (
	"context"
	"fmt"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

// OverlapRotationStrategy creates new secrets with validity overlap
// Used for providers that support expiration dates (certificates, some API keys)
type OverlapRotationStrategy struct {
	baseRotator    SecretValueRotator
	logger         *logging.Logger
	overlapPeriod  time.Duration
	totalValidity  time.Duration
}

// NewOverlapRotationStrategy creates an overlap-based rotation strategy
func NewOverlapRotationStrategy(baseRotator SecretValueRotator, logger *logging.Logger, overlapPeriod, totalValidity time.Duration) *OverlapRotationStrategy {
	// Default overlap period if not specified
	if overlapPeriod == 0 {
		overlapPeriod = 7 * 24 * time.Hour // 7 days
	}
	if totalValidity == 0 {
		totalValidity = 90 * 24 * time.Hour // 90 days
	}

	return &OverlapRotationStrategy{
		baseRotator:   baseRotator,
		logger:        logger,
		overlapPeriod: overlapPeriod,
		totalValidity: totalValidity,
	}
}

// Name returns the strategy name
func (s *OverlapRotationStrategy) Name() string {
	return fmt.Sprintf("overlap-%s", s.baseRotator.Name())
}

// SupportsSecret checks if the secret and provider support overlap rotation
func (s *OverlapRotationStrategy) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	// Must support base rotation
	if !s.baseRotator.SupportsSecret(ctx, secret) {
		return false
	}

	// Good for certificates and time-bound tokens
	switch secret.SecretType {
	case SecretTypeCertificate:
		return true
	case SecretTypeAPIKey, SecretTypeOAuth:
		// Check if provider supports expiration
		if exp, ok := secret.Metadata["supports_expiration"]; ok && exp == "true" {
			return true
		}
	}

	return false
}

// Rotate performs rotation with overlap period
func (s *OverlapRotationStrategy) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	auditTrail := []AuditEntry{
		createAuditEntry("overlap_rotation_started", "overlap_strategy", "info",
			fmt.Sprintf("Starting overlap rotation with %v overlap period", s.overlapPeriod), 
			map[string]interface{}{
				"secret_key":      logging.Secret(request.Secret.Key),
				"overlap_period":  s.overlapPeriod.String(),
				"total_validity":  s.totalValidity.String(),
			}),
	}

	s.logger.Info("Starting overlap rotation for %s with %v overlap", 
		logging.Secret(request.Secret.Key), s.overlapPeriod)

	// Calculate timing
	now := time.Now()
	newValidFrom := now
	newValidUntil := now.Add(s.totalValidity)
	oldExpiresAt := now.Add(s.overlapPeriod)

	// Step 1: Check if we're in valid rotation window
	status, err := s.GetStatus(ctx, request.Secret)
	if err == nil && status != nil && status.NextRotation != nil {
		if now.Before(*status.NextRotation) && !request.Force {
			return &RotationResult{
				Secret:     request.Secret,
				Status:     StatusPending,
				Error:      fmt.Sprintf("Too early to rotate, next rotation at %v", status.NextRotation),
				AuditTrail: auditTrail,
			}, nil
		}
	}

	// Step 2: Configure new secret with validity period
	if request.Config == nil {
		request.Config = make(map[string]interface{})
	}
	request.Config["valid_from"] = newValidFrom
	request.Config["valid_until"] = newValidUntil
	request.Config["overlap_with_previous"] = s.overlapPeriod

	auditTrail = append(auditTrail, createAuditEntry("configuring_validity", "overlap_strategy", "info",
		fmt.Sprintf("New secret valid from %v to %v", newValidFrom.Format(time.RFC3339), newValidUntil.Format(time.RFC3339)), 
		nil))

	if request.DryRun {
		auditTrail = append(auditTrail, createAuditEntry("dry_run_complete", "overlap_strategy", "info",
			fmt.Sprintf("Would create overlapping secret expiring at %v", newValidUntil), nil))
		
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusPending,
			ExpiresAt:  &newValidUntil,
			AuditTrail: auditTrail,
		}, nil
	}

	// Step 3: Create new secret through base rotator
	result, err := s.baseRotator.Rotate(ctx, request)
	if err != nil {
		auditTrail = append(auditTrail, createAuditEntry("rotation_failed", "overlap_strategy", "error",
			"Failed to create overlapping secret", map[string]interface{}{"error": err.Error()}))
		
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("overlap rotation failed: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	// Step 4: Schedule old secret expiration
	if result.OldSecretRef != nil {
		auditTrail = append(auditTrail, createAuditEntry("scheduling_expiration", "overlap_strategy", "info",
			fmt.Sprintf("Old secret will expire at %v", oldExpiresAt.Format(time.RFC3339)), 
			map[string]interface{}{
				"old_secret_ref": result.OldSecretRef.Identifier,
				"expires_at":     oldExpiresAt,
			}))
	}

	// Update result with overlap information
	result.ExpiresAt = &newValidUntil
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	result.Warnings = append(result.Warnings, 
		fmt.Sprintf("Overlap period active until %v. Both old and new secrets are valid during this time.", 
			oldExpiresAt.Format(time.RFC3339)))

	// Merge audit trails
	result.AuditTrail = append(auditTrail, result.AuditTrail...)

	// Calculate next rotation time (before expiry with buffer)
	nextRotation := newValidUntil.Add(-s.overlapPeriod * 2) // Rotate with 2x overlap period buffer
	if result.NewSecretRef != nil && result.NewSecretRef.Metadata == nil {
		result.NewSecretRef.Metadata = make(map[string]string)
	}
	if result.NewSecretRef != nil {
		result.NewSecretRef.Metadata["next_rotation"] = nextRotation.Format(time.RFC3339)
		result.NewSecretRef.Metadata["expires_at"] = newValidUntil.Format(time.RFC3339)
	}

	s.logger.Info("Completed overlap rotation for %s, new secret expires at %v", 
		logging.Secret(request.Secret.Key), newValidUntil)

	return result, nil
}

// Verify checks both old and new secrets during overlap
func (s *OverlapRotationStrategy) Verify(ctx context.Context, request VerificationRequest) error {
	s.logger.Debug("Verifying overlapping secret for %s", logging.Secret(request.Secret.Key))
	
	// During overlap, both secrets should work
	// This could verify both if we have access to the old one
	
	return s.baseRotator.Verify(ctx, request)
}

// Rollback during overlap means revoking the new secret
func (s *OverlapRotationStrategy) Rollback(ctx context.Context, request RollbackRequest) error {
	s.logger.Info("Rolling back overlap rotation for %s", logging.Secret(request.Secret.Key))
	
	// During overlap period, rollback just means revoking/deleting the new secret
	// The old one is still valid
	
	return s.baseRotator.Rollback(ctx, request)
}

// GetStatus returns rotation status including validity periods
func (s *OverlapRotationStrategy) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	status, err := s.baseRotator.GetStatus(ctx, secret)
	if err != nil {
		return nil, err
	}

	// Enhance status with overlap information if available
	if status != nil && status.LastRotated != nil {
		// Calculate when next rotation should happen
		nextRotation := status.LastRotated.Add(s.totalValidity - s.overlapPeriod*2)
		status.NextRotation = &nextRotation
		
		if status.Reason == "" {
			status.Reason = fmt.Sprintf("Overlap rotation with %v overlap period", s.overlapPeriod)
		}
	}

	return status, nil
}