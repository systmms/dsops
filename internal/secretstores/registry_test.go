package secretstores

import (
	"testing"

	"github.com/systmms/dsops/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecretStoreRegistry(t *testing.T) {
	registry := NewRegistry()

	t.Run("GetSupportedTypes", func(t *testing.T) {
		types := registry.GetSupportedTypes()
		assert.Contains(t, types, "bitwarden")
		assert.Contains(t, types, "vault")
		assert.Contains(t, types, "aws.secretsmanager")
		assert.Contains(t, types, "onepassword")
		
		// Should not contain service types
		assert.NotContains(t, types, "postgres")
		assert.NotContains(t, types, "mysql")
	})

	t.Run("IsSupported", func(t *testing.T) {
		// Secret store types should be supported
		assert.True(t, registry.IsSupported("bitwarden"))
		assert.True(t, registry.IsSupported("vault"))
		assert.True(t, registry.IsSupported("aws.secretsmanager"))
		
		// Service types should not be supported
		assert.False(t, registry.IsSupported("postgres"))
		assert.False(t, registry.IsSupported("mysql"))
		assert.False(t, registry.IsSupported("github"))
		
		// Unknown types should not be supported
		assert.False(t, registry.IsSupported("unknown"))
	})

	t.Run("CreateSecretStore", func(t *testing.T) {
		cfg := config.SecretStoreConfig{
			Type: "literal",
			Config: map[string]interface{}{
				"values": map[string]interface{}{
					"test-key": "test-value",
				},
			},
		}

		store, err := registry.CreateSecretStore("test-literal", cfg)
		require.NoError(t, err)
		assert.NotNil(t, store)
		assert.Equal(t, "test-literal", store.Name())
	})

	t.Run("CreateSecretStore_UnsupportedType", func(t *testing.T) {
		cfg := config.SecretStoreConfig{
			Type:   "postgres", // Service type, not secret store
			Config: map[string]interface{}{},
		}

		_, err := registry.CreateSecretStore("test-postgres", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown secret store type: postgres")
	})
}