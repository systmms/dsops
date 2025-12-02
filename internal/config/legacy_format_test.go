package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

// T038: Test YAML parsing (v0 legacy format with providers section)

func TestConfig_LegacyFormat_ProvidersSection(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	// Legacy v0 format with providers section instead of secretStores
	configContent := `version: 0
providers:
  vault:
    type: vault
    address: https://vault.example.com
    timeout_ms: 5000

  bitwarden:
    type: bitwarden

envs:
  production:
    DATABASE_PASSWORD:
      from:
        provider: vault
        key: secret/data/db/password
        version: latest
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

	// Test legacy providers section is loaded
	require.NotNil(t, config.Definition)
	assert.Len(t, config.Definition.Providers, 2)

	// Test GetProvider works with legacy format
	vaultProvider, err := config.GetProvider("vault")
	require.NoError(t, err)
	assert.Equal(t, "vault", vaultProvider.Type)
	assert.Equal(t, 5000, vaultProvider.TimeoutMs)

	bitwardenProvider, err := config.GetProvider("bitwarden")
	require.NoError(t, err)
	assert.Equal(t, "bitwarden", bitwardenProvider.Type)

	// Test environment parsing with legacy references
	env, err := config.GetEnvironment("production")
	require.NoError(t, err)

	dbPassword := env["DATABASE_PASSWORD"]
	require.NotNil(t, dbPassword.From)
	assert.True(t, dbPassword.From.IsLegacyFormat())
	assert.Equal(t, "vault", dbPassword.From.Provider)
	assert.Equal(t, "secret/data/db/password", dbPassword.From.Key)
	assert.Equal(t, "latest", dbPassword.From.Version)
}

func TestConfig_LegacyFormat_MixedProviders(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	// Mix of legacy providers and new secret stores
	configContent := `version: 0
providers:
  legacy-vault:
    type: vault

secretStores:
  new-bitwarden:
    type: bitwarden

services:
  postgres:
    type: postgres
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

	// All provider types should be accessible via GetProvider
	legacyVault, err := config.GetProvider("legacy-vault")
	require.NoError(t, err)
	assert.Equal(t, "vault", legacyVault.Type)

	newBitwarden, err := config.GetProvider("new-bitwarden")
	require.NoError(t, err)
	assert.Equal(t, "bitwarden", newBitwarden.Type)

	postgres, err := config.GetProvider("postgres")
	require.NoError(t, err)
	assert.Equal(t, "postgres", postgres.Type)

	// ListAllProviders should return all three
	allProviders := config.ListAllProviders()
	assert.Len(t, allProviders, 3)
}

// T040: Test v0 → v1 migration (backward compatibility)
// Note: There's no actual migration code, just backward compatibility support

func TestConfig_BackwardCompatibility_LegacyReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		legacyRef     Reference
		expectedStore string
		expectedPath  string
		expectedVer   string
	}{
		{
			name: "simple legacy reference",
			legacyRef: Reference{
				Provider: "vault",
				Key:      "secret/data/mykey",
			},
			expectedStore: "vault",
			expectedPath:  "secret/data/mykey",
			expectedVer:   "",
		},
		{
			name: "legacy reference with version",
			legacyRef: Reference{
				Provider: "bitwarden",
				Key:      "Platform/Database#password",
				Version:  "v123",
			},
			expectedStore: "bitwarden",
			expectedPath:  "Platform/Database#password",
			expectedVer:   "v123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test ToSecretRef conversion
			secretRef, err := tt.legacyRef.ToSecretRef()
			require.NoError(t, err)
			assert.Equal(t, tt.expectedStore, secretRef.Store)
			assert.Equal(t, tt.expectedPath, secretRef.Path)
			assert.Equal(t, tt.expectedVer, secretRef.Version)

			// Test ToLegacyProviderRef (round-trip)
			providerRef := tt.legacyRef.ToLegacyProviderRef()
			assert.Equal(t, tt.legacyRef.Provider, providerRef.Provider)
			assert.Equal(t, tt.legacyRef.Key, providerRef.Key)
			assert.Equal(t, tt.legacyRef.Version, providerRef.Version)
		})
	}
}

func TestConfig_BackwardCompatibility_ConvertLegacyReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerRef  *ProviderRef
		expectedURI  string
	}{
		{
			name: "simple conversion",
			providerRef: &ProviderRef{
				Provider: "vault",
				Key:      "secret/data/mykey",
			},
			expectedURI: "store://vault/secret/data/mykey",
		},
		{
			name: "with version",
			providerRef: &ProviderRef{
				Provider: "bitwarden",
				Key:      "Platform/Database",
				Version:  "latest",
			},
			expectedURI: "store://bitwarden/Platform/Database?version=latest",
		},
		{
			name:        "nil reference",
			providerRef: nil,
			expectedURI: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri := ConvertLegacyReference(tt.providerRef)
			assert.Equal(t, tt.expectedURI, uri)
		})
	}
}

func TestConfig_BackwardCompatibility_GetEffectiveProvider(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		ref              Reference
		expectedProvider string
	}{
		{
			name: "legacy format",
			ref: Reference{
				Provider: "vault",
				Key:      "secret/path",
			},
			expectedProvider: "vault",
		},
		{
			name: "store reference",
			ref: Reference{
				Store: "store://bitwarden/path/to/secret",
			},
			expectedProvider: "bitwarden",
		},
		// Note: Service reference parsing may not be fully implemented yet
		// Commenting out until service.ParseServiceRef is available
		// {
		// 	name: "service reference",
		// 	ref: Reference{
		// 		Service: "svc://postgres/prod-db",
		// 	},
		// 	expectedProvider: "postgres",
		// },
		{
			name:             "empty reference",
			ref:              Reference{},
			expectedProvider: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.ref.GetEffectiveProvider()
			assert.Equal(t, tt.expectedProvider, provider)
		})
	}
}

func TestConfig_BackwardCompatibility_MixedReferenceFormats(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "dsops.yaml")

	// Config with both legacy and new reference formats
	configContent := `version: 0
providers:
  vault:
    type: vault

secretStores:
  bitwarden:
    type: bitwarden

envs:
  test:
    # Legacy format reference
    LEGACY_VAR:
      from:
        provider: vault
        key: secret/data/legacy
        version: v1

    # New store format reference
    NEW_VAR:
      from:
        store: "store://bitwarden/Platform/Secret#field"
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

	// Test legacy format
	legacyVar := env["LEGACY_VAR"]
	require.NotNil(t, legacyVar.From)
	assert.True(t, legacyVar.From.IsLegacyFormat())
	assert.False(t, legacyVar.From.IsStoreReference())
	assert.Equal(t, "vault", legacyVar.From.Provider)
	assert.Equal(t, "secret/data/legacy", legacyVar.From.Key)

	// Test new format
	newVar := env["NEW_VAR"]
	require.NotNil(t, newVar.From)
	assert.True(t, newVar.From.IsStoreReference())
	assert.False(t, newVar.From.IsLegacyFormat())
	assert.Equal(t, "store://bitwarden/Platform/Secret#field", newVar.From.Store)
}

func TestConfig_LegacyFormat_ReferenceTypeDetection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		ref             Reference
		isLegacy        bool
		isStore         bool
		isService       bool
	}{
		{
			name: "legacy with provider and key",
			ref: Reference{
				Provider: "vault",
				Key:      "secret/path",
			},
			isLegacy:  true,
			isStore:   false,
			isService: false,
		},
		{
			name: "new store reference",
			ref: Reference{
				Store: "store://vault/secret/path",
			},
			isLegacy:  false,
			isStore:   true,
			isService: false,
		},
		{
			name: "new service reference",
			ref: Reference{
				Service: "svc://postgres/db",
			},
			isLegacy:  false,
			isStore:   false,
			isService: true,
		},
		{
			name:      "empty reference",
			ref:       Reference{},
			isLegacy:  false,
			isStore:   false,
			isService: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.isLegacy, tt.ref.IsLegacyFormat())
			assert.Equal(t, tt.isStore, tt.ref.IsStoreReference())
			assert.Equal(t, tt.isService, tt.ref.IsServiceReference())
		})
	}
}

func TestConfig_LegacyFormat_ToLegacyProviderRef_Conversion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ref      Reference
		expected ProviderRef
	}{
		{
			name: "legacy format unchanged",
			ref: Reference{
				Provider: "vault",
				Key:      "secret/path",
				Version:  "v1",
			},
			expected: ProviderRef{
				Provider: "vault",
				Key:      "secret/path",
				Version:  "v1",
			},
		},
		{
			name: "store reference converted",
			ref: Reference{
				Store: "store://bitwarden/Platform/Secret?version=v2",
			},
			expected: ProviderRef{
				Provider: "bitwarden",
				Key:      "Platform/Secret",
				Version:  "v2",
			},
		},
		{
			name:     "service reference returns empty",
			ref:      Reference{
				Service: "svc://postgres/db",
			},
			expected: ProviderRef{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ref.ToLegacyProviderRef()
			assert.Equal(t, tt.expected.Provider, result.Provider)
			assert.Equal(t, tt.expected.Key, result.Key)
			assert.Equal(t, tt.expected.Version, result.Version)
		})
	}
}
