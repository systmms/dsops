package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

// T039: Test config validation

func TestConfig_Validation_MissingFile(t *testing.T) {
	t.Parallel()

	logger := logging.New(false, false)
	config := &Config{
		Path:   "/nonexistent/path/to/config.yaml",
		Logger: logger,
	}

	err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "configuration file not found")
}

func TestConfig_Validation_InvalidYAML(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	// Invalid YAML with syntax error
	invalidYAML := `version: 0
secretStores:
  vault:
    type: vault
    bad syntax here [[[
`

	err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid YAML syntax")
}

func TestConfig_Validation_UnsupportedVersion(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	// Unsupported version number
	configContent := `version: 999
secretStores:
  vault:
    type: vault
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported configuration version")
}

func TestConfig_Validation_MissingRequiredFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		content     string
		shouldError bool
		errorMsg    string
	}{
		{
			name: "secret store missing type",
			content: `version: 0
secretStores:
  vault:
    address: https://vault.example.com
`,
			shouldError: false, // Type is optional, will be empty string
		},
		{
			name: "valid minimal config",
			content: `version: 0
secretStores:
  vault:
    type: vault
`,
			shouldError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "dsops.yaml")

			err := os.WriteFile(configPath, []byte(tt.content), 0644)
			require.NoError(t, err)

			logger := logging.New(false, false)
			config := &Config{
				Path:   configPath,
				Logger: logger,
			}

			err = config.Load()
			if tt.shouldError {
				require.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Validation_InvalidReferenceTypes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	// Config with both store and service references (testing parsing, not validation)
	configContent := `version: 0
secretStores:
  vault:
    type: vault

services:
  postgres:
    type: postgres

envs:
  test:
    DATABASE_PASSWORD:
      from:
        store: "store://vault/db/password"
    CONNECTION_STRING:
      from:
        service: "svc://postgres?kind=connection_string"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	env, err := config.GetEnvironment("test")
	require.NoError(t, err)

	dbPassword := env["DATABASE_PASSWORD"]
	assert.True(t, dbPassword.From.IsStoreReference())
	assert.False(t, dbPassword.From.IsServiceReference())

	connString := env["CONNECTION_STRING"]
	assert.True(t, connString.From.IsServiceReference())
	assert.False(t, connString.From.IsStoreReference())
}

func TestConfig_GetEnvironment_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	configContent := `version: 0
secretStores:
  vault:
    type: vault

envs:
  production:
    VAR1:
      literal: "value"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	_, err = config.GetEnvironment("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "environment not found")
	assert.Contains(t, err.Error(), "production") // Should suggest available env
}

func TestConfig_GetProvider_NotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	configContent := `version: 0
secretStores:
  vault:
    type: vault
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	_, err = config.GetProvider("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found")
	assert.Contains(t, err.Error(), "vault") // Should suggest available provider
}

func TestConfig_ListAllProviders(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	configContent := `version: 0
secretStores:
  vault:
    type: vault
  bitwarden:
    type: bitwarden

services:
  postgres:
    type: postgres

providers:  # Legacy format
  legacy-aws:
    type: aws.secretsmanager
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	providers := config.ListAllProviders()
	assert.Len(t, providers, 4)
	assert.Contains(t, providers, "vault")
	assert.Contains(t, providers, "bitwarden")
	assert.Contains(t, providers, "postgres")
	assert.Contains(t, providers, "legacy-aws")
}

// T041: Test schema validation

func TestConfig_Schema_SecretStoreConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	configContent := `version: 0
secretStores:
  vault:
    type: vault
    address: https://vault.example.com
    auth_method: token
    timeout_ms: 5000
    namespace: prod
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	store, err := config.GetSecretStore("vault")
	require.NoError(t, err)
	assert.Equal(t, "vault", store.Type)
	assert.Equal(t, 5000, store.TimeoutMs)
	assert.Equal(t, "https://vault.example.com", store.Config["address"])
	assert.Equal(t, "token", store.Config["auth_method"])
	assert.Equal(t, "prod", store.Config["namespace"])
}

func TestConfig_Schema_ServiceConfig(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	configContent := `version: 0
services:
  postgres-prod:
    type: postgres
    host: db.prod.example.com
    port: 5432
    database: appdb
    timeout_ms: 3000
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	service, err := config.GetService("postgres-prod")
	require.NoError(t, err)
	assert.Equal(t, "postgres", service.Type)
	assert.Equal(t, 3000, service.TimeoutMs)
	assert.Equal(t, "db.prod.example.com", service.Config["host"])
	assert.Equal(t, 5432, service.Config["port"]) // YAML parses numbers as int
	assert.Equal(t, "appdb", service.Config["database"])
}

func TestConfig_Schema_EnvironmentVariables(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	configContent := `version: 0
secretStores:
  vault:
    type: vault

envs:
  test:
    # Store reference
    VAR1:
      from:
        store: "store://vault/path/to/secret#field"
      optional: false

    # Literal value
    VAR2:
      literal: "static-value"

    # Optional variable
    VAR3:
      from:
        store: "store://vault/optional/secret"
      optional: true
      metadata:
        description: "Optional test variable"
        source: "vault"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	env, err := config.GetEnvironment("test")
	require.NoError(t, err)
	assert.Len(t, env, 3)

	// Check VAR1 (required store reference)
	var1 := env["VAR1"]
	require.NotNil(t, var1.From)
	assert.True(t, var1.From.IsStoreReference())
	assert.False(t, var1.Optional)

	// Check VAR2 (literal)
	var2 := env["VAR2"]
	assert.Equal(t, "static-value", var2.Literal)
	assert.Nil(t, var2.From)

	// Check VAR3 (optional with metadata)
	var3 := env["VAR3"]
	require.NotNil(t, var3.From)
	assert.True(t, var3.Optional)
	assert.Equal(t, "Optional test variable", var3.Metadata["description"])
	assert.Equal(t, "vault", var3.Metadata["source"])
}

func TestConfig_Schema_ProviderTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		timeoutMs       int
		expectedTimeout int
	}{
		{
			name:            "custom timeout",
			timeoutMs:       5000,
			expectedTimeout: 5000,
		},
		{
			name:            "default timeout",
			timeoutMs:       0,
			expectedTimeout: 30000, // Default 30 seconds
		},
		{
			name:            "negative timeout defaults",
			timeoutMs:       -100,
			expectedTimeout: 30000, // Should default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := ProviderConfig{
				Type:      "vault",
				TimeoutMs: tt.timeoutMs,
			}

			timeout := provider.GetProviderTimeout()
			assert.Equal(t, tt.expectedTimeout, timeout)
		})
	}
}
