package validation

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	
	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
)

// CredentialValidator validates credentials against schema constraints
type CredentialValidator struct {
	repository *dsopsdata.Repository
	logger     *logging.Logger
}

// NewCredentialValidator creates a new credential validator
func NewCredentialValidator(repository *dsopsdata.Repository, logger *logging.Logger) *CredentialValidator {
	return &CredentialValidator{
		repository: repository,
		logger:     logger,
	}
}

// ValidationResult contains the result of a validation
type ValidationResult struct {
	Valid       bool     `json:"valid"`
	Errors      []string `json:"errors,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	TTLSeconds  int64    `json:"ttl_seconds,omitempty"`
}

// ValidateCredential validates a credential value against schema constraints
func (v *CredentialValidator) ValidateCredential(serviceType, credentialKind, value string) *ValidationResult {
	result := &ValidationResult{Valid: true}
	
	if v.repository == nil {
		v.logger.Debug("No repository available, skipping validation")
		return result
	}
	
	// Get service type
	svcType, exists := v.repository.GetServiceType(serviceType)
	if !exists {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Unknown service type '%s', cannot validate", serviceType))
		return result
	}
	
	// Find credential kind
	var credKind *dsopsdata.CredentialKind
	for _, ck := range svcType.Spec.CredentialKinds {
		if ck.Name == credentialKind {
			credKind = &ck
			break
		}
	}
	
	if credKind == nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Unknown credential kind '%s' for service '%s'", credentialKind, serviceType))
		return result
	}
	
	// Validate format if specified
	if credKind.Constraints.Format != "" {
		if err := v.validateFormat(value, credKind.Constraints.Format); err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("Format validation failed: %v", err))
		}
	}
	
	// Parse TTL if specified
	if credKind.Constraints.TTL != "" {
		ttl, err := time.ParseDuration(credKind.Constraints.TTL)
		if err != nil {
			// Try parsing as days (e.g., "365d")
			if strings.HasSuffix(credKind.Constraints.TTL, "d") {
				days := strings.TrimSuffix(credKind.Constraints.TTL, "d")
				var d int
				if _, err := fmt.Sscanf(days, "%d", &d); err == nil {
					ttl = time.Duration(d) * 24 * time.Hour
				} else {
					result.Warnings = append(result.Warnings, fmt.Sprintf("Invalid TTL format '%s': %v", credKind.Constraints.TTL, err))
				}
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Invalid TTL format '%s': %v", credKind.Constraints.TTL, err))
			}
		}
		
		if ttl > 0 {
			result.TTLSeconds = int64(ttl.Seconds())
			v.logger.Debug("Credential TTL constraint: %v", ttl)
		}
	}
	
	return result
}

// validateFormat validates a value against a regular expression
func (v *CredentialValidator) validateFormat(value, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
	}
	
	if !re.MatchString(value) {
		// Mask the value for security in error messages
		maskedValue := maskValue(value)
		return fmt.Errorf("value '%s' does not match required format '%s'", maskedValue, pattern)
	}
	
	return nil
}

// ValidateNewCredential validates a new credential before rotation
func (v *CredentialValidator) ValidateNewCredential(serviceType, credentialKind string, newValue string, currentValue string) *ValidationResult {
	result := v.ValidateCredential(serviceType, credentialKind, newValue)
	
	// Additional validation: ensure new value is different from current
	if newValue == currentValue {
		result.Valid = false
		result.Errors = append(result.Errors, "New credential value must be different from current value")
	}
	
	return result
}

// maskValue masks a credential value for safe logging
func maskValue(value string) string {
	if len(value) <= 8 {
		return "***"
	}
	
	// Show first 3 and last 3 characters
	return value[:3] + "***" + value[len(value)-3:]
}