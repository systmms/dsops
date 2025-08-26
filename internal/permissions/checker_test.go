package permissions

import (
	"context"
	"testing"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
)

func TestPermissionCheckerBasics(t *testing.T) {
	logger := logging.New(false, true)
	
	// Test with nil repository (should allow all)
	checker := NewPermissionChecker(nil, logger)
	request := RotationRequest{
		Principal:      "anyone",
		ServiceType:    "any-service",
		CredentialKind: "any-credential",
		Environment:    "any-env",
		SecretKey:      "any-key",
	}
	
	result := checker.CheckRotationPermission(context.Background(), request)
	if !result.Allowed {
		t.Error("Expected permission to be allowed when no repository configured")
	}
	if result.Reason != "No permission system configured" {
		t.Errorf("Expected 'No permission system configured', got: %s", result.Reason)
	}
}

func TestPermissionCheckerWithSimpleRepo(t *testing.T) {
	logger := logging.New(false, true)
	
	// Create minimal repository for testing
	repo := &dsopsdata.Repository{
		Principals: map[string]*dsopsdata.Principal{
			"test-principal": {
				Spec: struct {
					Type        string                        `yaml:"type" json:"type"`
					Email       string                        `yaml:"email,omitempty" json:"email,omitempty"`
					Team        string                        `yaml:"team,omitempty" json:"team,omitempty"`
					Environment string                        `yaml:"environment,omitempty" json:"environment,omitempty"`
					Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
					Contact     *dsopsdata.PrincipalContact     `yaml:"contact,omitempty" json:"contact,omitempty"`
					Metadata    map[string]interface{}        `yaml:"metadata,omitempty" json:"metadata,omitempty"`
				}{
					Type: "user",
					Permissions: &dsopsdata.PrincipalPermissions{
						AllowedServices:        []string{"postgresql"},
						AllowedCredentialKinds: []string{"password"},
						MaxCredentialTTL:       "30d",
					},
				},
			},
		},
	}
	
	checker := NewPermissionChecker(repo, logger)
	
	// Test allowed request
	allowedRequest := RotationRequest{
		Principal:      "test-principal",
		ServiceType:    "postgresql",
		CredentialKind: "password",
		Environment:    "test",
		SecretKey:      "DB_PASSWORD",
	}
	
	result := checker.CheckRotationPermission(context.Background(), allowedRequest)
	if !result.Allowed {
		t.Errorf("Expected permission to be allowed, got denied: %s", result.Reason)
	}
	
	// Test denied request (wrong service)
	deniedRequest := RotationRequest{
		Principal:      "test-principal",
		ServiceType:    "mysql", // Not in allowed services
		CredentialKind: "password",
		Environment:    "test",
		SecretKey:      "MYSQL_PASSWORD",
	}
	
	result = checker.CheckRotationPermission(context.Background(), deniedRequest)
	if result.Allowed {
		t.Error("Expected permission to be denied for wrong service type")
	}
	
	// Test unknown principal
	unknownRequest := RotationRequest{
		Principal:      "unknown-user",
		ServiceType:    "postgresql",
		CredentialKind: "password",
		Environment:    "test",
		SecretKey:      "DB_PASSWORD",
	}
	
	result = checker.CheckRotationPermission(context.Background(), unknownRequest)
	if result.Allowed {
		t.Error("Expected permission to be denied for unknown principal")
	}
	if result.Reason != "Unknown principal: unknown-user" {
		t.Errorf("Expected unknown principal error, got: %s", result.Reason)
	}
}

func TestTTLValidation(t *testing.T) {
	logger := logging.New(false, true)
	
	// Create repository with TTL limits
	repo := &dsopsdata.Repository{
		Principals: map[string]*dsopsdata.Principal{
			"limited-user": {
				Spec: struct {
					Type        string                        `yaml:"type" json:"type"`
					Email       string                        `yaml:"email,omitempty" json:"email,omitempty"`
					Team        string                        `yaml:"team,omitempty" json:"team,omitempty"`
					Environment string                        `yaml:"environment,omitempty" json:"environment,omitempty"`
					Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
					Contact     *dsopsdata.PrincipalContact     `yaml:"contact,omitempty" json:"contact,omitempty"`
					Metadata    map[string]interface{}        `yaml:"metadata,omitempty" json:"metadata,omitempty"`
				}{
					Type: "user",
					Permissions: &dsopsdata.PrincipalPermissions{
						AllowedServices:        []string{"api"},
						AllowedCredentialKinds: []string{"api_key"},
						MaxCredentialTTL:       "1h", // Short TTL limit
					},
				},
			},
		},
	}
	
	checker := NewPermissionChecker(repo, logger)
	
	// Test within TTL limit
	validRequest := RotationRequest{
		Principal:      "limited-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "test",
		SecretKey:      "API_KEY",
		RequestedTTL:   30 * time.Minute, // Under 1h limit
	}
	
	result := checker.CheckRotationPermission(context.Background(), validRequest)
	if !result.Allowed {
		t.Errorf("Expected permission within TTL limit to be allowed: %s", result.Reason)
	}
	
	// Test exceeding TTL limit
	invalidRequest := RotationRequest{
		Principal:      "limited-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "test",
		SecretKey:      "API_KEY",
		RequestedTTL:   2 * time.Hour, // Over 1h limit
	}
	
	result = checker.CheckRotationPermission(context.Background(), invalidRequest)
	if result.Allowed {
		t.Error("Expected permission exceeding TTL limit to be denied")
	}
	if !containsString(result.Reason, "exceeds maximum allowed") {
		t.Errorf("Expected TTL limit error, got: %s", result.Reason)
	}
}

func TestPrincipalWithoutPermissions(t *testing.T) {
	logger := logging.New(false, true)
	
	// Create repository with principal that has no specific permissions
	repo := &dsopsdata.Repository{
		Principals: map[string]*dsopsdata.Principal{
			"no-permissions": {
				Spec: struct {
					Type        string                        `yaml:"type" json:"type"`
					Email       string                        `yaml:"email,omitempty" json:"email,omitempty"`
					Team        string                        `yaml:"team,omitempty" json:"team,omitempty"`
					Environment string                        `yaml:"environment,omitempty" json:"environment,omitempty"`
					Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
					Contact     *dsopsdata.PrincipalContact     `yaml:"contact,omitempty" json:"contact,omitempty"`
					Metadata    map[string]interface{}        `yaml:"metadata,omitempty" json:"metadata,omitempty"`
				}{
					Type: "user",
					// No permissions specified - should allow all
				},
			},
		},
	}
	
	checker := NewPermissionChecker(repo, logger)
	
	request := RotationRequest{
		Principal:      "no-permissions",
		ServiceType:    "any-service",
		CredentialKind: "any-credential",
		Environment:    "any",
		SecretKey:      "ANY_KEY",
	}
	
	result := checker.CheckRotationPermission(context.Background(), request)
	if !result.Allowed {
		t.Errorf("Expected principal without permissions to be allowed: %s", result.Reason)
	}
	if result.Reason != "No specific permissions configured for principal" {
		t.Errorf("Expected no permissions configured message, got: %s", result.Reason)
	}
}

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && 
		   (len(substr) == 0 || (len(s) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}