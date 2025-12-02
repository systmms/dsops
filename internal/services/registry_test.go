package services

import (
	"context"
	"testing"

	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/pkg/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceRegistry(t *testing.T) {
	// Create a mock repository with test services
	postgresType := &dsopsdata.ServiceType{
		APIVersion: "v1",
		Kind:       "ServiceType",
	}
	postgresType.Metadata.Name = "postgres"
	postgresType.Metadata.Description = "PostgreSQL Database"
	postgresType.Metadata.Category = "database"

	mysqlType := &dsopsdata.ServiceType{
		APIVersion: "v1",
		Kind:       "ServiceType",
	}
	mysqlType.Metadata.Name = "mysql"
	mysqlType.Metadata.Description = "MySQL Database"
	mysqlType.Metadata.Category = "database"

	githubType := &dsopsdata.ServiceType{
		APIVersion: "v1",
		Kind:       "ServiceType",
	}
	githubType.Metadata.Name = "github"
	githubType.Metadata.Description = "GitHub API"
	githubType.Metadata.Category = "api"

	stripeType := &dsopsdata.ServiceType{
		APIVersion: "v1",
		Kind:       "ServiceType",
	}
	stripeType.Metadata.Name = "stripe"
	stripeType.Metadata.Description = "Stripe API"
	stripeType.Metadata.Category = "api"

	mockRepo := &dsopsdata.Repository{
		ServiceTypes: map[string]*dsopsdata.ServiceType{
			"postgres": postgresType,
			"mysql":    mysqlType,
			"github":   githubType,
			"stripe":   stripeType,
		},
		ServiceInstances: make(map[string]*dsopsdata.ServiceInstance),
		RotationPolicies: make(map[string]*dsopsdata.RotationPolicy),
		Principals:       make(map[string]*dsopsdata.Principal),
	}

	registry := NewRegistryWithDataDriven(mockRepo)

	// Register a factory for postgres to test HasImplementation
	registry.RegisterFactory("postgres", func(name string, config map[string]interface{}) (service.Service, error) {
		return &mockService{name: name}, nil
	})

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
		// All services in the data-driven repository are implemented
		assert.True(t, registry.HasImplementation("postgres"))
		assert.True(t, registry.HasImplementation("mysql"))
		assert.True(t, registry.HasImplementation("github"))

		// Unknown services are not implemented
		assert.False(t, registry.HasImplementation("unknown"))
	})

	t.Run("CreateService_Implemented", func(t *testing.T) {
		cfg := config.ServiceConfig{
			Type: "postgres",
			Config: map[string]interface{}{
				"host":     "localhost",
				"database": "testdb",
			},
		}

		// Service should be created via hardcoded factory
		service, err := registry.CreateService("test-postgres", cfg)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, "test-postgres", service.Name())
	})

	t.Run("CreateService_DataDriven", func(t *testing.T) {
		cfg := config.ServiceConfig{
			Type: "mysql",
			Config: map[string]interface{}{
				"host":     "localhost",
				"database": "testdb",
			},
		}

		// MySQL should be created via data-driven factory
		service, err := registry.CreateService("test-mysql", cfg)
		require.NoError(t, err)
		assert.NotNil(t, service)
		assert.Equal(t, "test-mysql", service.Name())
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

// mockService is a simple mock service for testing
type mockService struct {
	name string
}

func (m *mockService) Name() string {
	return m.name
}

func (m *mockService) Plan(ctx context.Context, req service.RotationRequest) (service.RotationPlan, error) {
	return service.RotationPlan{}, nil
}

func (m *mockService) Execute(ctx context.Context, plan service.RotationPlan) (service.RotationResult, error) {
	return service.RotationResult{}, nil
}

func (m *mockService) Verify(ctx context.Context, result service.RotationResult) error {
	return nil
}

func (m *mockService) Rollback(ctx context.Context, result service.RotationResult) error {
	return nil
}

func (m *mockService) GetStatus(ctx context.Context, ref service.ServiceRef) (service.RotationStatus, error) {
	return service.RotationStatus{}, nil
}

func (m *mockService) Capabilities() service.ServiceCapabilities {
	return service.ServiceCapabilities{}
}

func (m *mockService) Validate(ctx context.Context) error {
	return nil
}