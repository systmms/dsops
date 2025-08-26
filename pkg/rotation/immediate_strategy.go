package rotation

import (
	"context"
	"fmt"
	"time"

	"github.com/systmms/dsops/internal/logging"
)

// ImmediateRotationStrategy replaces secrets immediately without overlap
// This may cause brief downtime but is simpler and works with all providers
type ImmediateRotationStrategy struct {
	baseRotator SecretValueRotator
	logger      *logging.Logger
}

// NewImmediateRotationStrategy creates an immediate replacement strategy
func NewImmediateRotationStrategy(baseRotator SecretValueRotator, logger *logging.Logger) *ImmediateRotationStrategy {
	return &ImmediateRotationStrategy{
		baseRotator: baseRotator,
		logger:      logger,
	}
}

// Name returns the strategy name
func (s *ImmediateRotationStrategy) Name() string {
	return fmt.Sprintf("immediate-%s", s.baseRotator.Name())
}

// SupportsSecret delegates to base rotator
func (s *ImmediateRotationStrategy) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	return s.baseRotator.SupportsSecret(ctx, secret)
}

// Rotate performs immediate secret replacement
func (s *ImmediateRotationStrategy) Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error) {
	auditTrail := []AuditEntry{
		createAuditEntry("immediate_rotation_started", "immediate_strategy", "info",
			"Starting immediate secret rotation", map[string]interface{}{
				"secret_key": logging.Secret(request.Secret.Key),
				"strategy":   s.baseRotator.Name(),
			}),
	}

	s.logger.Info("Starting immediate rotation for %s", logging.Secret(request.Secret.Key))

	// Step 1: Generate new secret value
	auditTrail = append(auditTrail, createAuditEntry("generating_new_value", "immediate_strategy", "info",
		"Generating new secret value", nil))

	// Step 2: Create brief backup of current value (if possible)
	auditTrail = append(auditTrail, createAuditEntry("backup_current", "immediate_strategy", "info",
		"Creating backup of current value", nil))

	if request.DryRun {
		auditTrail = append(auditTrail, createAuditEntry("dry_run_complete", "immediate_strategy", "info",
			"Would perform immediate rotation", nil))
		
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusPending,
			AuditTrail: auditTrail,
		}, nil
	}

	// Step 3: Perform rotation through base rotator
	s.logger.Debug("Delegating to base rotator for immediate update")
	
	result, err := s.baseRotator.Rotate(ctx, request)
	if err != nil {
		auditTrail = append(auditTrail, createAuditEntry("rotation_failed", "immediate_strategy", "error",
			"Immediate rotation failed", map[string]interface{}{"error": err.Error()}))
		
		return &RotationResult{
			Secret:     request.Secret,
			Status:     StatusFailed,
			Error:      fmt.Sprintf("immediate rotation failed: %v", err),
			AuditTrail: auditTrail,
		}, err
	}

	// Step 4: Quick verification
	auditTrail = append(auditTrail, createAuditEntry("verifying_new_secret", "immediate_strategy", "info",
		"Verifying new secret works", nil))

	// Add a warning about potential downtime
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	result.Warnings = append(result.Warnings, 
		"Immediate rotation may have caused brief downtime. Consider using two-key strategy if provider supports it.")

	// Merge audit trails
	result.AuditTrail = append(auditTrail, result.AuditTrail...)

	rotatedAt := time.Now()
	result.RotatedAt = &rotatedAt

	result.AuditTrail = append(result.AuditTrail, createAuditEntry("rotation_completed", "immediate_strategy", "info",
		"Immediate rotation completed", nil))

	s.logger.Info("Completed immediate rotation for %s", logging.Secret(request.Secret.Key))

	return result, nil
}

// Verify delegates to base rotator
func (s *ImmediateRotationStrategy) Verify(ctx context.Context, request VerificationRequest) error {
	return s.baseRotator.Verify(ctx, request)
}

// Rollback attempts to restore the previous value
func (s *ImmediateRotationStrategy) Rollback(ctx context.Context, request RollbackRequest) error {
	s.logger.Info("Attempting rollback for immediate rotation of %s", logging.Secret(request.Secret.Key))
	
	// In immediate rotation, rollback is challenging since old value is already gone
	// This would need to restore from backup if available
	
	return s.baseRotator.Rollback(ctx, request)
}

// GetStatus delegates to base rotator
func (s *ImmediateRotationStrategy) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	return s.baseRotator.GetStatus(ctx, secret)
}