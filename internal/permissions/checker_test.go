package permissions

import (
	"context"
	"testing"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/rotation"
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

func TestCredentialKindValidation(t *testing.T) {
	logger := logging.New(false, true)

	repo := &dsopsdata.Repository{
		Principals: map[string]*dsopsdata.Principal{
			"api-user": {
				Spec: struct {
					Type        string                          `yaml:"type" json:"type"`
					Email       string                          `yaml:"email,omitempty" json:"email,omitempty"`
					Team        string                          `yaml:"team,omitempty" json:"team,omitempty"`
					Environment string                          `yaml:"environment,omitempty" json:"environment,omitempty"`
					Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
					Contact     *dsopsdata.PrincipalContact     `yaml:"contact,omitempty" json:"contact,omitempty"`
					Metadata    map[string]interface{}          `yaml:"metadata,omitempty" json:"metadata,omitempty"`
				}{
					Type: "user",
					Permissions: &dsopsdata.PrincipalPermissions{
						AllowedServices:        []string{"api"},
						AllowedCredentialKinds: []string{"api_key", "oauth_token"},
					},
				},
			},
		},
	}

	checker := NewPermissionChecker(repo, logger)

	// Test allowed credential kind
	allowedRequest := RotationRequest{
		Principal:      "api-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "test",
		SecretKey:      "API_KEY",
	}

	result := checker.CheckRotationPermission(context.Background(), allowedRequest)
	if !result.Allowed {
		t.Errorf("Expected allowed credential kind to be permitted: %s", result.Reason)
	}

	// Test denied credential kind
	deniedRequest := RotationRequest{
		Principal:      "api-user",
		ServiceType:    "api",
		CredentialKind: "certificate", // Not in allowed kinds
		Environment:    "test",
		SecretKey:      "API_CERT",
	}

	result = checker.CheckRotationPermission(context.Background(), deniedRequest)
	if result.Allowed {
		t.Error("Expected denied credential kind to be rejected")
	}
	if !containsString(result.Reason, "not in allowed kinds") {
		t.Errorf("Expected credential kind error, got: %s", result.Reason)
	}
}

func TestInvalidTTLFormat(t *testing.T) {
	logger := logging.New(false, true)

	repo := &dsopsdata.Repository{
		Principals: map[string]*dsopsdata.Principal{
			"bad-ttl-user": {
				Spec: struct {
					Type        string                          `yaml:"type" json:"type"`
					Email       string                          `yaml:"email,omitempty" json:"email,omitempty"`
					Team        string                          `yaml:"team,omitempty" json:"team,omitempty"`
					Environment string                          `yaml:"environment,omitempty" json:"environment,omitempty"`
					Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
					Contact     *dsopsdata.PrincipalContact     `yaml:"contact,omitempty" json:"contact,omitempty"`
					Metadata    map[string]interface{}          `yaml:"metadata,omitempty" json:"metadata,omitempty"`
				}{
					Type: "user",
					Permissions: &dsopsdata.PrincipalPermissions{
						MaxCredentialTTL: "invalid-duration", // Bad format
					},
				},
			},
		},
	}

	checker := NewPermissionChecker(repo, logger)

	request := RotationRequest{
		Principal:      "bad-ttl-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "test",
		SecretKey:      "API_KEY",
		RequestedTTL:   30 * time.Minute,
	}

	result := checker.CheckRotationPermission(context.Background(), request)
	// Should still be allowed (invalid format is logged as constraint, not blocker)
	if !result.Allowed {
		t.Errorf("Expected request with invalid TTL format to be allowed: %s", result.Reason)
	}
	// Should have constraint about invalid format
	foundConstraint := false
	for _, c := range result.Constraints {
		if containsString(c, "Invalid maxCredentialTTL format") {
			foundConstraint = true
			break
		}
	}
	if !foundConstraint {
		t.Error("Expected constraint about invalid TTL format")
	}
}

func TestEnvironmentRestrictions(t *testing.T) {
	logger := logging.New(false, true)

	repo := &dsopsdata.Repository{
		Principals: map[string]*dsopsdata.Principal{
			"prod-only-user": {
				Spec: struct {
					Type        string                          `yaml:"type" json:"type"`
					Email       string                          `yaml:"email,omitempty" json:"email,omitempty"`
					Team        string                          `yaml:"team,omitempty" json:"team,omitempty"`
					Environment string                          `yaml:"environment,omitempty" json:"environment,omitempty"`
					Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
					Contact     *dsopsdata.PrincipalContact     `yaml:"contact,omitempty" json:"contact,omitempty"`
					Metadata    map[string]interface{}          `yaml:"metadata,omitempty" json:"metadata,omitempty"`
				}{
					Type:        "user",
					Environment: "production", // Only allowed in production
					Permissions: &dsopsdata.PrincipalPermissions{
						// Empty but not nil - environment check will run
					},
				},
			},
			"multi-env-user": {
				Spec: struct {
					Type        string                          `yaml:"type" json:"type"`
					Email       string                          `yaml:"email,omitempty" json:"email,omitempty"`
					Team        string                          `yaml:"team,omitempty" json:"team,omitempty"`
					Environment string                          `yaml:"environment,omitempty" json:"environment,omitempty"`
					Permissions *dsopsdata.PrincipalPermissions `yaml:"permissions,omitempty" json:"permissions,omitempty"`
					Contact     *dsopsdata.PrincipalContact     `yaml:"contact,omitempty" json:"contact,omitempty"`
					Metadata    map[string]interface{}          `yaml:"metadata,omitempty" json:"metadata,omitempty"`
				}{
					Type:        "user",
					Environment: "production",
					Permissions: &dsopsdata.PrincipalPermissions{
						// Empty but not nil - environment check will run
					},
					Metadata: map[string]interface{}{
						"environments": []interface{}{"production", "staging"},
					},
				},
			},
		},
	}

	checker := NewPermissionChecker(repo, logger)

	// Test prod-only user in prod environment (allowed)
	prodRequest := RotationRequest{
		Principal:      "prod-only-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "production",
		SecretKey:      "API_KEY",
	}

	result := checker.CheckRotationPermission(context.Background(), prodRequest)
	if !result.Allowed {
		t.Errorf("Expected prod user in prod environment to be allowed: %s", result.Reason)
	}

	// Test prod-only user in dev environment (denied)
	devRequest := RotationRequest{
		Principal:      "prod-only-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "development",
		SecretKey:      "API_KEY",
	}

	result = checker.CheckRotationPermission(context.Background(), devRequest)
	if result.Allowed {
		t.Error("Expected prod-only user in dev environment to be denied")
	}
	if !containsString(result.Reason, "Environment mismatch") {
		t.Errorf("Expected environment mismatch error, got: %s", result.Reason)
	}

	// Test multi-env user in staging (allowed via metadata)
	stagingRequest := RotationRequest{
		Principal:      "multi-env-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "staging",
		SecretKey:      "API_KEY",
	}

	result = checker.CheckRotationPermission(context.Background(), stagingRequest)
	if !result.Allowed {
		t.Errorf("Expected multi-env user in staging to be allowed via metadata: %s", result.Reason)
	}

	// Test multi-env user in dev (denied - not in metadata list)
	multiEnvDevRequest := RotationRequest{
		Principal:      "multi-env-user",
		ServiceType:    "api",
		CredentialKind: "api_key",
		Environment:    "development", // Not in metadata list (only prod, staging)
		SecretKey:      "API_KEY",
	}
	result = checker.CheckRotationPermission(context.Background(), multiEnvDevRequest)
	if result.Allowed {
		t.Error("Expected multi-env user in dev environment to be denied (not in metadata list)")
	}
}

func TestGetPrincipalForRotation(t *testing.T) {
	logger := logging.New(false, true)
	checker := NewPermissionChecker(nil, logger)

	tests := []struct {
		name     string
		secret   rotation.SecretInfo
		expected string
	}{
		{
			name: "principal in metadata",
			secret: rotation.SecretInfo{
				Key:      "test-secret",
				Metadata: map[string]string{"principal": "test-user"},
			},
			expected: "test-user",
		},
		{
			name: "no principal in metadata",
			secret: rotation.SecretInfo{
				Key:      "test-secret",
				Metadata: map[string]string{"other": "value"},
			},
			expected: "",
		},
		{
			name: "nil metadata",
			secret: rotation.SecretInfo{
				Key:      "test-secret",
				Metadata: nil,
			},
			expected: "",
		},
		{
			name: "empty metadata",
			secret: rotation.SecretInfo{
				Key:      "test-secret",
				Metadata: map[string]string{},
			},
			expected: "",
		},
		{
			name: "principal with empty value",
			secret: rotation.SecretInfo{
				Key:      "test-secret",
				Metadata: map[string]string{"principal": ""},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checker.GetPrincipalForRotation(context.Background(), tt.secret)
			if result != tt.expected {
				t.Errorf("GetPrincipalForRotation() = %q, expected %q", result, tt.expected)
			}
		})
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