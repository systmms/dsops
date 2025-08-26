package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/systmms/dsops/internal/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefinition_Load(t *testing.T) {
	// Create a temporary config file with new format
	configContent := `version: 0

secretStores:
  bitwarden:
    type: bitwarden
    
  vault:
    type: vault
    address: https://vault.company.com
    auth_method: token
    timeout_ms: 5000

services:
  postgres-prod:
    type: postgres
    host: db.prod.company.com
    database: app

envs:
  development:
    DATABASE_PASSWORD:
      from: { store: "store://bitwarden/Platform/Database#password" }
    
    API_KEY:
      from: { store: "store://vault/secrets/api-keys?version=latest" }
    
    NODE_ENV:
      literal: "development"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load the configuration
	logger := logging.New(false, false)
	config := &Config{
		Path:   configPath,
		Logger: logger,
	}

	err = config.Load()
	require.NoError(t, err)

	// Test Definition parsing
	require.NotNil(t, config.Definition)
	assert.Equal(t, 0, config.Definition.Version)
	
	// Test secret stores
	assert.Len(t, config.Definition.SecretStores, 2)
	
	bitwardenStore, err := config.GetSecretStore("bitwarden")
	require.NoError(t, err)
	assert.Equal(t, "bitwarden", bitwardenStore.Type)
	
	vaultStore, err := config.GetSecretStore("vault")
	require.NoError(t, err)
	assert.Equal(t, "vault", vaultStore.Type)
	assert.Equal(t, 5000, vaultStore.TimeoutMs)
	assert.Equal(t, "https://vault.company.com", vaultStore.Config["address"])

	// Test services
	assert.Len(t, config.Definition.Services, 1)
	
	postgresService, err := config.GetService("postgres-prod")
	require.NoError(t, err)
	assert.Equal(t, "postgres", postgresService.Type)
	assert.Equal(t, "db.prod.company.com", postgresService.Config["host"])

	// Test environments
	assert.Len(t, config.Definition.Envs, 1)
	
	devEnv := config.Definition.Envs["development"]
	assert.Len(t, devEnv, 3)
	
	// Test store reference
	dbPassword := devEnv["DATABASE_PASSWORD"]
	require.NotNil(t, dbPassword.From)
	assert.True(t, dbPassword.From.IsStoreReference())
	assert.Equal(t, "store://bitwarden/Platform/Database#password", dbPassword.From.Store)
	
	// Test literal value
	nodeEnv := devEnv["NODE_ENV"]
	assert.Equal(t, "development", nodeEnv.Literal)

	// Test provider compatibility - should work with GetProvider method
	bitwardenProvider, err := config.GetProvider("bitwarden")
	require.NoError(t, err)
	assert.Equal(t, "bitwarden", bitwardenProvider.Type)
	
	postgresProvider, err := config.GetProvider("postgres-prod")
	require.NoError(t, err)
	assert.Equal(t, "postgres", postgresProvider.Type)
	
	env, err := config.GetEnvironment("development")
	require.NoError(t, err)
	assert.Len(t, env, 3)
}

func TestReference_Conversion(t *testing.T) {
	tests := []struct {
		name     string
		ref      Reference
		expected string
	}{
		{
			name: "store reference",
			ref: Reference{
				Store: "store://bitwarden/path/to/secret#field",
			},
			expected: "bitwarden",
		},
		{
			name: "service reference",
			ref: Reference{
				Service: "svc://postgres/prod-db?kind=password&host=example.com",
			},
			expected: "postgres",
		},
		{
			name: "legacy reference",
			ref: Reference{
				Provider: "vault",
				Key:      "secret/data/mykey",
				Version:  "v1",
			},
			expected: "vault",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			effective := tt.ref.GetEffectiveProvider()
			assert.Equal(t, tt.expected, effective)
		})
	}
}

func TestReference_ToSecretRef(t *testing.T) {
	// Test store reference conversion
	ref := Reference{
		Store: "store://bitwarden/Platform/Database#password?version=latest",
	}
	
	secretRef, err := ref.ToSecretRef()
	require.NoError(t, err)
	assert.Equal(t, "bitwarden", secretRef.Store)
	assert.Equal(t, "Platform/Database", secretRef.Path)
	assert.Equal(t, "password", secretRef.Field)
	assert.Equal(t, "latest", secretRef.Version)

	// Test legacy format conversion
	legacyRef := Reference{
		Provider: "vault",
		Key:      "secret/data/mykey",
		Version:  "v1",
	}
	
	legacySecretRef, err := legacyRef.ToSecretRef()
	require.NoError(t, err)
	assert.Equal(t, "vault", legacySecretRef.Store)
	assert.Equal(t, "secret/data/mykey", legacySecretRef.Path)
	assert.Equal(t, "v1", legacySecretRef.Version)

	// Test service reference should fail
	serviceRef := Reference{
		Service: "svc://postgres/prod-db",
	}
	
	_, err = serviceRef.ToSecretRef()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a secret store reference")
}

func TestConvertLegacyReference(t *testing.T) {
	tests := []struct {
		name     string
		ref      *ProviderRef
		expected string
	}{
		{
			name: "simple reference",
			ref: &ProviderRef{
				Provider: "bitwarden",
				Key:      "mykey",
			},
			expected: "store://bitwarden/mykey",
		},
		{
			name: "reference with version",
			ref: &ProviderRef{
				Provider: "vault",
				Key:      "secret/data/mykey",
				Version:  "v1",
			},
			expected: "store://vault/secret/data/mykey?version=v1",
		},
		{
			name:     "nil reference",
			ref:      nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertLegacyReference(tt.ref)
			assert.Equal(t, tt.expected, result)
		})
	}
}