package rotation

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/internal/logging"
)

func TestScriptRotator_SupportsSecret(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	tests := []struct {
		name     string
		secret   SecretInfo
		expected bool
	}{
		{
			name: "supports_with_script_path",
			secret: SecretInfo{
				Metadata: map[string]string{
					"script_path": "/path/to/script.sh",
				},
			},
			expected: true,
		},
		{
			name: "no_script_path",
			secret: SecretInfo{
				Metadata: map[string]string{},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rotator.SupportsSecret(context.Background(), tt.secret)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestScriptRotator_WithRepository(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	// Create mock repository
	repo := &dsopsdata.Repository{
		ServiceTypes: map[string]*dsopsdata.ServiceType{
			"postgresql": {
				Metadata: struct {
					Name        string `yaml:"name" json:"name"`
					Description string `yaml:"description,omitempty" json:"description,omitempty"`
					Category    string `yaml:"category,omitempty" json:"category,omitempty"`
				}{
					Name: "postgresql",
				},
				Spec: struct {
					CredentialKinds []dsopsdata.CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
					Defaults        struct {
						RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
						RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
					} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
				}{
					CredentialKinds: []dsopsdata.CredentialKind{
						{
							Name:         "password",
							Description:  "Database password",
							Capabilities: []string{"rotate", "verify", "revoke"},
							Constraints: struct {
								MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
								TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
								Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
							}{
								MaxActive: 2,
								TTL:       "30d",
								Format:    "alphanumeric",
							},
						},
					},
				},
			},
		},
	}

	rotator.SetRepository(repo)

	// Test with capability support
	secret := SecretInfo{
		SecretType: "postgresql",
		Metadata: map[string]string{
			"script_path":      "/path/to/script.sh",
			"service_type":     "postgresql",
			"credential_kind":  "password",
		},
	}

	supports := rotator.SupportsSecret(context.Background(), secret)
	if !supports {
		t.Error("expected rotator to support secret with valid capabilities")
	}

	// Test without rotate capability
	repo.ServiceTypes["postgresql"].Spec.CredentialKinds[0].Capabilities = []string{"verify", "revoke"}
	supports = rotator.SupportsSecret(context.Background(), secret)
	if supports {
		t.Error("expected rotator to not support secret without rotate capability")
	}
}

func TestScriptRotator_ExecuteScript(t *testing.T) {
	// Create a test script
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-rotate.sh")

	scriptContent := `#!/bin/bash
read input
action=$(echo "$input" | jq -r '.action')
if [ "$action" = "rotate" ]; then
  echo '{
    "success": true,
    "message": "Rotation successful",
    "new_secret_ref": {
      "provider": "test-store",
      "key": "new/secret/path"
    }
  }'
elif [ "$action" = "verify" ]; then
  echo '{"success": true, "message": "Verification successful"}'
elif [ "$action" = "status" ]; then
  echo '{
    "success": true,
    "metadata": {
      "status": "completed",
      "can_rotate": true,
      "reason": "Ready for rotation"
    }
  }'
else
  echo '{"success": false, "error": "Unknown action"}'
fi
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	// Test rotation
	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "test-secret",
			SecretType: "api-key",
			Provider:   "script",
			Metadata: map[string]string{
				"script_path": scriptPath,
			},
		},
		DryRun: false,
		Force:  false,
	}

	ctx := context.Background()
	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, result.Status)
	}
	if result.NewSecretRef == nil {
		t.Error("expected new secret ref")
	} else {
		if result.NewSecretRef.Provider != "test-store" {
			t.Errorf("expected provider test-store, got %s", result.NewSecretRef.Provider)
		}
		if result.NewSecretRef.Key != "new/secret/path" {
			t.Errorf("expected key new/secret/path, got %s", result.NewSecretRef.Key)
		}
	}
}

func TestScriptRotator_ErrorHandling(t *testing.T) {
	// Create a test script that returns error
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-error.sh")

	scriptContent := `#!/bin/bash
echo '{"success": false, "error": "Script execution failed"}'
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "test-secret",
			SecretType: "api-key",
			Provider:   "script",
			Metadata: map[string]string{
				"script_path": scriptPath,
			},
		},
	}

	ctx := context.Background()
	result, err := rotator.Rotate(ctx, request)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if result.Status != StatusFailed {
		t.Errorf("expected status %s, got %s", StatusFailed, result.Status)
	}
	if result.Error != "Script execution failed" {
		t.Errorf("expected error 'Script execution failed', got %s", result.Error)
	}
}

func TestScriptRotator_EnvironmentVariables(t *testing.T) {
	// Create a test script that checks environment variables
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-env.sh")

	scriptContent := `#!/bin/bash
if [ "$DSOPS_ACTION" = "rotate" ] && [ "$DSOPS_SECRET_KEY" = "test-secret" ] && [ "$DSOPS_DRY_RUN" = "false" ]; then
  echo '{"success": true, "message": "Environment variables verified"}'
else
  echo '{"success": false, "error": "Environment variables not set correctly"}'
fi
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "test-secret",
			SecretType: "api-key",
			Provider:   "script",
			Metadata: map[string]string{
				"script_path": scriptPath,
			},
		},
		DryRun: false,
	}

	ctx := context.Background()
	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, result.Status)
	}
}

func TestScriptRotator_SchemaMetadata(t *testing.T) {
	// Create a test script that validates schema metadata
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-schema.sh")

	scriptContent := `#!/bin/bash
read input
schema=$(echo "$input" | jq -r '.schema_metadata')
if [ "$schema" != "null" ]; then
  service_type=$(echo "$input" | jq -r '.schema_metadata.service_type')
  if [ "$service_type" = "postgresql" ]; then
    echo '{"success": true, "message": "Schema metadata validated"}'
  else
    echo '{"success": false, "error": "Unexpected service type"}'
  fi
else
  echo '{"success": false, "error": "No schema metadata"}'
fi
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	// Set up repository
	repo := &dsopsdata.Repository{
		ServiceTypes: map[string]*dsopsdata.ServiceType{
			"postgresql": {
				Metadata: struct {
					Name        string `yaml:"name" json:"name"`
					Description string `yaml:"description,omitempty" json:"description,omitempty"`
					Category    string `yaml:"category,omitempty" json:"category,omitempty"`
				}{
					Name: "postgresql",
				},
				Spec: struct {
					CredentialKinds []dsopsdata.CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
					Defaults        struct {
						RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
						RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
					} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
				}{
					CredentialKinds: []dsopsdata.CredentialKind{
						{
							Name:         "password",
							Capabilities: []string{"rotate"},
						},
					},
				},
			},
		},
	}
	rotator.SetRepository(repo)

	request := RotationRequest{
		Secret: SecretInfo{
			Key:        "test-secret",
			SecretType: "postgresql",
			Provider:   "script",
			Metadata: map[string]string{
				"script_path":     scriptPath,
				"service_type":    "postgresql",
				"credential_kind": "password",
			},
		},
	}

	ctx := context.Background()
	result, err := rotator.Rotate(ctx, request)
	if err != nil {
		t.Fatalf("rotation failed: %v", err)
	}

	if result.Status != StatusCompleted {
		t.Errorf("expected status %s, got %s", StatusCompleted, result.Status)
	}
}

func TestScriptRotator_InvalidJSON(t *testing.T) {
	// Create a test script that returns invalid JSON
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test-invalid.sh")

	scriptContent := `#!/bin/bash
echo "This is not JSON"
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to create test script: %v", err)
	}

	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	request := RotationRequest{
		Secret: SecretInfo{
			Key: "test-secret",
			Metadata: map[string]string{
				"script_path": scriptPath,
			},
		},
	}

	ctx := context.Background()
	_, err := rotator.Rotate(ctx, request)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}

	if !contains(err.Error(), "failed to parse script output as JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

func TestScriptRotator_ScriptNotFound(t *testing.T) {
	logger := logging.New(false, true)
	rotator := NewScriptRotator(logger)

	request := RotationRequest{
		Secret: SecretInfo{
			Key: "test-secret",
			Metadata: map[string]string{
				"script_path": "/nonexistent/script.sh",
			},
		},
	}

	ctx := context.Background()
	_, err := rotator.Rotate(ctx, request)
	if err == nil {
		t.Fatal("expected error for missing script, got nil")
	}

	if !contains(err.Error(), "script not found") {
		t.Errorf("expected script not found error, got: %v", err)
	}
}

// Test helper to check if error contains substring
func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}