package validation

import (
	"strings"
	"testing"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
)

func TestCredentialValidatorBasics(t *testing.T) {
	logger := logging.New(false, true)
	
	// Test with nil repository (should allow all)
	validator := NewCredentialValidator(nil, logger)
	
	result := validator.ValidateNewCredential("any", "any", "any-value", "")
	if !result.Valid {
		t.Error("Expected validation to pass when no repository configured")
	}
	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}
}

func TestCredentialValidatorSimple(t *testing.T) {
	logger := logging.New(false, true)
	
	// Create simple repository structure for testing
	// This avoids complex struct initialization by using a minimal setup
	repo := &dsopsdata.Repository{
		ServiceTypes: make(map[string]*dsopsdata.ServiceType),
	}
	
	// Create a simple ServiceType manually
	serviceType := &dsopsdata.ServiceType{}
	serviceType.Metadata.Name = "test-service"
	
	// Add credential kind structure
	credKind := dsopsdata.CredentialKind{
		Name:         "test-password",
		Capabilities: []string{"create", "rotate", "verify"},
	}
	// Set constraints manually to avoid struct field issues
	credKind.Constraints.Format = "^[A-Za-z0-9!@#$%^&*()]+$"
	credKind.Constraints.TTL = "30d"
	
	serviceType.Spec.CredentialKinds = []dsopsdata.CredentialKind{credKind}
	repo.ServiceTypes["test-service"] = serviceType
	
	validator := NewCredentialValidator(repo, logger)
	
	// Test valid credential
	result := validator.ValidateNewCredential("test-service", "test-password", "ValidPassword123!", "")
	if !result.Valid {
		t.Errorf("Expected valid credential to pass validation, got errors: %v", result.Errors)
	}
	
	// Test invalid format
	result = validator.ValidateNewCredential("test-service", "test-password", "invalid-unicode-€", "")
	if result.Valid {
		t.Error("Expected invalid format to fail validation")
	}
	if len(result.Errors) == 0 {
		t.Error("Expected validation errors for invalid format")
	}
	
	// Test unknown service (should be valid with warnings, not errors)
	result = validator.ValidateNewCredential("unknown-service", "any", "any-value", "")
	if !result.Valid {
		t.Errorf("Expected unknown service to be valid with warnings, got errors: %v", result.Errors)
	}
	if len(result.Warnings) == 0 || !containsString(result.Warnings[0], "Unknown service type") {
		t.Errorf("Expected unknown service warning, got: %v", result.Warnings)
	}
}

func TestTTLParsing(t *testing.T) {
	logger := logging.New(false, true)
	
	tests := []struct {
		name        string
		ttlString   string
		expectedTTL time.Duration
	}{
		{
			name:        "hours",
			ttlString:   "2h",
			expectedTTL: 2 * time.Hour,
		},
		{
			name:        "minutes",
			ttlString:   "30m",
			expectedTTL: 30 * time.Minute,
		},
		{
			name:        "days",
			ttlString:   "7d",
			expectedTTL: 7 * 24 * time.Hour,
		},
		{
			name:        "seconds",
			ttlString:   "300s",
			expectedTTL: 300 * time.Second,
		},
		{
			name:        "invalid",
			ttlString:   "invalid",
			expectedTTL: 0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create simple repository for TTL testing
			repo := &dsopsdata.Repository{
				ServiceTypes: make(map[string]*dsopsdata.ServiceType),
			}
			
			serviceType := &dsopsdata.ServiceType{}
			serviceType.Metadata.Name = "test"
			
			credKind := dsopsdata.CredentialKind{
				Name: "test-cred",
			}
			credKind.Constraints.TTL = tt.ttlString
			
			serviceType.Spec.CredentialKinds = []dsopsdata.CredentialKind{credKind}
			repo.ServiceTypes["test"] = serviceType
			
			validator := NewCredentialValidator(repo, logger)
			result := validator.ValidateNewCredential("test", "test-cred", "valid-value", "")
			
			expectedSeconds := int64(tt.expectedTTL.Seconds())
			if result.TTLSeconds != expectedSeconds {
				t.Errorf("Expected TTL %d seconds, got %d", expectedSeconds, result.TTLSeconds)
			}
		})
	}
}

func TestSameValueValidation(t *testing.T) {
	logger := logging.New(false, true)
	
	// Create simple repository
	repo := &dsopsdata.Repository{
		ServiceTypes: make(map[string]*dsopsdata.ServiceType),
	}
	
	serviceType := &dsopsdata.ServiceType{}
	serviceType.Metadata.Name = "test"
	
	credKind := dsopsdata.CredentialKind{
		Name: "password",
	}
	
	serviceType.Spec.CredentialKinds = []dsopsdata.CredentialKind{credKind}
	repo.ServiceTypes["test"] = serviceType
	
	validator := NewCredentialValidator(repo, logger)
	
	// Test same value as current
	result := validator.ValidateNewCredential("test", "password", "same-password", "same-password")
	if result.Valid {
		t.Error("Expected validation to fail when new value equals current value")
	}
	if len(result.Errors) == 0 || !containsString(result.Errors[0], "must be different") {
		t.Errorf("Expected 'must be different' error, got: %v", result.Errors)
	}
	
	// Test different value
	result = validator.ValidateNewCredential("test", "password", "new-password", "old-password")
	if !result.Valid {
		t.Errorf("Expected validation to pass with different values, got errors: %v", result.Errors)
	}
}

func TestEmptyValueValidation(t *testing.T) {
	logger := logging.New(false, true)
	
	// Create simple repository
	repo := &dsopsdata.Repository{
		ServiceTypes: make(map[string]*dsopsdata.ServiceType),
	}
	
	serviceType := &dsopsdata.ServiceType{}
	serviceType.Metadata.Name = "test"
	
	credKind := dsopsdata.CredentialKind{
		Name: "password",
	}
	
	serviceType.Spec.CredentialKinds = []dsopsdata.CredentialKind{credKind}
	repo.ServiceTypes["test"] = serviceType
	
	validator := NewCredentialValidator(repo, logger)
	
	// Test empty value (will fail because it equals current empty value)
	result := validator.ValidateNewCredential("test", "password", "", "")
	if result.Valid {
		t.Error("Expected validation to fail for empty value")
	}
	if len(result.Errors) == 0 || !containsString(result.Errors[0], "must be different") {
		t.Errorf("Expected 'must be different' error for same empty values, got: %v", result.Errors)
	}
	
	// Test empty value with different current value
	result = validator.ValidateNewCredential("test", "password", "", "not-empty")
	if !result.Valid {
		t.Errorf("Expected validation to pass for empty new value with non-empty current, got errors: %v", result.Errors)
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}