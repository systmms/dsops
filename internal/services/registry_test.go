package services

import (
	"testing"

	"github.com/systmms/dsops/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRegistry(t *testing.T) {
	registry := NewRegistry()

	t.Run("GetSupportedTypes", func(t *testing.T) {
		types := registry.GetSupportedTypes()
		assert.Contains(t, types, "postgres")
		assert.Contains(t, types, "mysql")
		assert.Contains(t, types, "github")
		assert.Contains(t, types, "stripe")
		
		// Should not contain secret store types
		assert.NotContains(t, types, "bitwarden")
		assert.NotContains(t, types, "vault")
	})

	t.Run("IsSupported", func(t *testing.T) {
		// Service types should be supported
		assert.True(t, registry.IsSupported("postgres"))
		assert.True(t, registry.IsSupported("mysql"))
		assert.True(t, registry.IsSupported("github"))
		
		// Secret store types should not be supported
		assert.False(t, registry.IsSupported("bitwarden"))
		assert.False(t, registry.IsSupported("vault"))
		assert.False(t, registry.IsSupported("aws.secretsmanager"))
		
		// Unknown types should not be supported
		assert.False(t, registry.IsSupported("unknown"))
	})

	t.Run("HasImplementation", func(t *testing.T) {
		// Postgres has a factory (even if it's not implemented)
		assert.True(t, registry.HasImplementation("postgres"))
		
		// MySQL is supported but not implemented yet
		assert.False(t, registry.HasImplementation("mysql"))
		assert.False(t, registry.HasImplementation("github"))
	})

	t.Run("CreateService_Implemented", func(t *testing.T) {
		cfg := config.ServiceConfig{
			Type: "postgres",
			Config: map[string]interface{}{
				"host":     "localhost",
				"database": "testdb",
			},
		}

		service, err := registry.CreateService("test-postgres", cfg)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, "test-postgres", service.Name())
	})

	t.Run("CreateService_SupportedButNotImplemented", func(t *testing.T) {
		cfg := config.ServiceConfig{
			Type: "mysql",
			Config: map[string]interface{}{
				"host":     "localhost",
				"database": "testdb",
			},
		}

		_, err := registry.CreateService("test-mysql", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "service type 'mysql' is supported but not yet implemented")
	})

	t.Run("CreateService_UnsupportedType", func(t *testing.T) {
		cfg := config.ServiceConfig{
			Type:   "bitwarden", // Secret store type, not service
			Config: map[string]interface{}{},
		}

		_, err := registry.CreateService("test-bitwarden", cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown service type: bitwarden")
	})
}