package providers_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
)

// TestRegistryCreation validates registry initialization
func TestRegistryCreation(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()
	assert.NotNil(t, registry)

	// Check that built-in providers are registered
	supportedTypes := registry.GetSupportedTypes()
	assert.NotEmpty(t, supportedTypes)
	assert.GreaterOrEqual(t, len(supportedTypes), 10, "Should have multiple built-in providers")
}

// TestRegistryIsSupported validates provider type checking
func TestRegistryIsSupported(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()

	tests := []struct {
		name         string
		providerType string
		wantSupported bool
	}{
		{"literal", "literal", true},
		{"bitwarden", "bitwarden", true},
		{"onepassword", "onepassword", true},
		{"aws_secretsmanager", "aws.secretsmanager", true},
		{"aws_ssm", "aws.ssm", true},
		{"gcp_secretmanager", "gcp.secretmanager", true},
		{"azure_keyvault", "azure.keyvault", true},
		{"vault", "vault", true},
		{"doppler", "doppler", true},
		{"pass", "pass", true},
		{"unknown", "unknown-provider", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			supported := registry.IsSupported(tt.providerType)
			assert.Equal(t, tt.wantSupported, supported,
				"Provider type '%s' support check failed", tt.providerType)
		})
	}
}

// TestRegistryCreateProvider validates provider creation
func TestRegistryCreateProvider(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()

	tests := []struct {
		name         string
		providerName string
		providerType string
		config       map[string]interface{}
		wantErr      bool
	}{
		{
			name:         "literal_provider",
			providerName: "my-literal",
			providerType: "literal",
			config:       map[string]interface{}{"test": "value"},
			wantErr:      false,
		},
		{
			name:         "mock_provider",
			providerName: "my-mock",
			providerType: "mock",
			config:       map[string]interface{}{},
			wantErr:      false,
		},
		{
			name:         "unknown_provider",
			providerName: "unknown",
			providerType: "unknown-type",
			config:       map[string]interface{}{},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := config.ProviderConfig{
				Type:   tt.providerType,
				Config: tt.config,
			}

			provider, err := registry.CreateProvider(tt.providerName, cfg)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, provider)
				assert.Contains(t, err.Error(), "unknown provider type")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, provider)
				assert.Equal(t, tt.providerName, provider.Name())
			}
		})
	}
}

// TestRegistryGetSupportedTypes validates listing supported types
func TestRegistryGetSupportedTypes(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()
	types := registry.GetSupportedTypes()

	// Should have all built-in providers
	expectedTypes := []string{
		"literal",
		"mock",
		"json",
		"bitwarden",
		"onepassword",
		"aws.secretsmanager",
		"aws.ssm",
		"aws",
		"gcp.secretmanager",
		"gcp",
		"azure.keyvault",
		"azure",
		"vault",
		"doppler",
		"pass",
	}

	for _, expectedType := range expectedTypes {
		assert.Contains(t, types, expectedType,
			"Expected provider type '%s' to be registered", expectedType)
	}
}

// TestRegistryRegisterFactory validates custom factory registration
func TestRegistryRegisterFactory(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()

	// Register a custom factory
	customFactoryCalled := false
	customFactory := func(name string, config map[string]interface{}) (provider.Provider, error) {
		customFactoryCalled = true
		return providers.NewLiteralProvider(name, nil), nil
	}

	registry.RegisterFactory("custom", customFactory)

	// Verify it's registered
	assert.True(t, registry.IsSupported("custom"))

	// Create a provider using the custom factory
	cfg := config.ProviderConfig{
		Type:   "custom",
		Config: map[string]interface{}{},
	}

	provider, err := registry.CreateProvider("test-custom", cfg)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.True(t, customFactoryCalled, "Custom factory should have been called")
}

// TestRegistryFactoryOverride validates factory replacement
func TestRegistryFactoryOverride(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()

	// Override an existing factory
	overrideCalled := false
	overrideFactory := func(name string, config map[string]interface{}) (provider.Provider, error) {
		overrideCalled = true
		return providers.NewLiteralProvider(name, nil), nil
	}

	registry.RegisterFactory("literal", overrideFactory)

	cfg := config.ProviderConfig{
		Type:   "literal",
		Config: map[string]interface{}{},
	}

	provider, err := registry.CreateProvider("test-override", cfg)
	assert.NoError(t, err)
	assert.NotNil(t, provider)
	assert.True(t, overrideCalled, "Override factory should have been called")
}

// TestRegistryMultipleProviders validates creating multiple provider instances
func TestRegistryMultipleProviders(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()

	// Create multiple providers of different types
	createdProviders := make(map[string]provider.Provider)

	configs := map[string]config.ProviderConfig{
		"literal-1": {Type: "literal", Config: map[string]interface{}{}},
		"literal-2": {Type: "literal", Config: map[string]interface{}{}},
		"mock-1":    {Type: "mock", Config: map[string]interface{}{}},
	}

	for name, cfg := range configs {
		p, err := registry.CreateProvider(name, cfg)
		require.NoError(t, err)
		require.NotNil(t, p)
		createdProviders[name] = p
	}

	// Verify all providers were created with correct names
	assert.Equal(t, "literal-1", createdProviders["literal-1"].Name())
	assert.Equal(t, "literal-2", createdProviders["literal-2"].Name())
	assert.Equal(t, "mock-1", createdProviders["mock-1"].Name())

	// Verify they are independent instances
	assert.NotSame(t, createdProviders["literal-1"], createdProviders["literal-2"])
}

// TestRegistryErrorHandling validates error scenarios
func TestRegistryErrorHandling(t *testing.T) {
	t.Parallel()

	registry := providers.NewRegistry()

	tests := []struct {
		name        string
		providerCfg config.ProviderConfig
		wantErr     string
	}{
		{
			name:        "unknown_type",
			providerCfg: config.ProviderConfig{Type: "nonexistent", Config: nil},
			wantErr:     "unknown provider type",
		},
		{
			name:        "empty_type",
			providerCfg: config.ProviderConfig{Type: "", Config: nil},
			wantErr:     "unknown provider type",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := registry.CreateProvider("test", tt.providerCfg)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
