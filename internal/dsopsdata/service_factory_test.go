package dsopsdata

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/pkg/service"
)

func createTestServiceType() *ServiceType {
	return &ServiceType{
		APIVersion: "dsops-data/v1",
		Kind:       "ServiceType",
		Metadata: struct {
			Name        string `yaml:"name" json:"name"`
			Description string `yaml:"description,omitempty" json:"description,omitempty"`
			Category    string `yaml:"category,omitempty" json:"category,omitempty"`
		}{
			Name:        "postgresql",
			Description: "PostgreSQL database",
			Category:    "database",
		},
		Spec: struct {
			CredentialKinds []CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
			Defaults        struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
		}{
			CredentialKinds: []CredentialKind{
				{
					Name:        "password",
					Description: "Database password",
					Capabilities: []string{
						"create",
						"rotate",
						"revoke",
						"verify",
					},
					Constraints: struct {
						MaxActive interface{} `yaml:"maxActive,omitempty" json:"maxActive,omitempty"`
						TTL       string      `yaml:"ttl,omitempty" json:"ttl,omitempty"`
						Format    string      `yaml:"format,omitempty" json:"format,omitempty"`
					}{
						MaxActive: 2,
						TTL:       "90d",
					},
				},
			},
			Defaults: struct {
				RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
				RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
			}{
				RotationStrategy: "two-key",
			},
		},
	}
}

func createTestRepository() *Repository {
	return &Repository{
		ServiceTypes: map[string]*ServiceType{
			"postgresql": createTestServiceType(),
		},
		ServiceInstances: map[string]*ServiceInstance{},
		RotationPolicies: map[string]*RotationPolicy{},
		Principals:       map[string]*Principal{},
	}
}

func TestNewDataDrivenServiceFactory(t *testing.T) {
	t.Parallel()

	repo := createTestRepository()
	factory := NewDataDrivenServiceFactory(repo)

	assert.NotNil(t, factory)
	assert.Equal(t, repo, factory.repository)
	assert.NotNil(t, factory.registry)
}

func TestDataDrivenServiceFactory_CreateService(t *testing.T) {
	t.Parallel()

	repo := createTestRepository()
	factory := NewDataDrivenServiceFactory(repo)

	cfg := config.ServiceConfig{
		Type: "postgresql",
		Config: map[string]interface{}{
			"host":     "localhost",
			"port":     5432,
			"database": "testdb",
		},
	}

	svc, err := factory.CreateService("test-db", cfg)
	require.NoError(t, err)
	assert.NotNil(t, svc)
	assert.Equal(t, "test-db", svc.Name())
}

func TestDataDrivenServiceFactory_CreateService_UnknownType(t *testing.T) {
	t.Parallel()

	repo := createTestRepository()
	factory := NewDataDrivenServiceFactory(repo)

	cfg := config.ServiceConfig{
		Type: "unknown-service",
	}

	_, err := factory.CreateService("test", cfg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown service type")
}

func TestDataDrivenServiceFactory_GetSupportedTypes(t *testing.T) {
	t.Parallel()

	repo := createTestRepository()
	factory := NewDataDrivenServiceFactory(repo)

	types := factory.GetSupportedTypes()
	assert.Contains(t, types, "postgresql")
}

func TestDataDrivenServiceFactory_IsSupported(t *testing.T) {
	t.Parallel()

	repo := createTestRepository()
	factory := NewDataDrivenServiceFactory(repo)

	assert.True(t, factory.IsSupported("postgresql"))
	assert.False(t, factory.IsSupported("mysql"))
}

func TestDataDrivenService_Name(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	assert.Equal(t, "prod-db", svc.Name())
}

func TestDataDrivenService_Plan_TwoKeyStrategy(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	req := service.RotationRequest{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "password",
		},
		Strategy: "two-key",
	}

	ctx := context.Background()
	plan, err := svc.Plan(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, "two-key", plan.Strategy)
	assert.Len(t, plan.Steps, 4) // create, verify, promote, revoke
	assert.Equal(t, "create_new", plan.Steps[0].Name)
	assert.Equal(t, "create", plan.Steps[0].Action)
	assert.Equal(t, "verify_new", plan.Steps[1].Name)
	assert.Equal(t, "verify", plan.Steps[1].Action)
	assert.Equal(t, "promote_new", plan.Steps[2].Name)
	assert.Equal(t, "promote", plan.Steps[2].Action)
	assert.Equal(t, "revoke_old", plan.Steps[3].Name)
	assert.Equal(t, "delete", plan.Steps[3].Action)
}

func TestDataDrivenService_Plan_ImmediateStrategy(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	req := service.RotationRequest{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "password",
		},
		Strategy: "immediate",
	}

	ctx := context.Background()
	plan, err := svc.Plan(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, "immediate", plan.Strategy)
	assert.Len(t, plan.Steps, 2) // rotate, verify
	assert.Equal(t, "rotate_immediate", plan.Steps[0].Name)
	assert.Equal(t, "verify_rotated", plan.Steps[1].Name)
}

func TestDataDrivenService_Plan_OverlapStrategy(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	req := service.RotationRequest{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "password",
		},
		Strategy: "overlap",
	}

	ctx := context.Background()
	plan, err := svc.Plan(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, "overlap", plan.Strategy)
	assert.Len(t, plan.Steps, 3) // create, verify, activate (no revoke)
	assert.Equal(t, "create_overlapping", plan.Steps[0].Name)
	assert.Equal(t, "verify_overlapping", plan.Steps[1].Name)
	assert.Equal(t, "activate_overlapping", plan.Steps[2].Name)
}

func TestDataDrivenService_Plan_DefaultStrategy(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	req := service.RotationRequest{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "password",
		},
		Strategy: "", // Empty strategy should use default
	}

	ctx := context.Background()
	plan, err := svc.Plan(ctx, req)
	require.NoError(t, err)

	assert.Equal(t, "two-key", plan.Strategy) // Default from service type
}

func TestDataDrivenService_Plan_UnknownCredentialKind(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	req := service.RotationRequest{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "unknown_kind",
		},
		Strategy: "immediate",
	}

	ctx := context.Background()
	_, err := svc.Plan(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credential kind unknown_kind not supported")
}

func TestDataDrivenService_Plan_UnsupportedStrategy(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	req := service.RotationRequest{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "password",
		},
		Strategy: "invalid-strategy",
	}

	ctx := context.Background()
	_, err := svc.Plan(ctx, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported rotation strategy")
}

func TestDataDrivenService_Plan_Fingerprint(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	req := service.RotationRequest{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "password",
		},
		Strategy: "immediate",
	}

	ctx := context.Background()
	plan, err := svc.Plan(ctx, req)
	require.NoError(t, err)

	assert.NotEmpty(t, plan.Fingerprint)
	assert.NotZero(t, plan.CreatedAt)
}

func TestDataDrivenService_Capabilities(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	caps := svc.Capabilities()

	assert.Equal(t, 2, caps.MaxActiveKeys) // From maxActive constraint
	assert.True(t, caps.SupportsExpiration) // Has TTL
	assert.Contains(t, caps.SupportedStrategies, "two-key")
	assert.Contains(t, caps.SupportedStrategies, "overlap")
	assert.Contains(t, caps.SupportedStrategies, "immediate")
}

func TestDataDrivenService_Capabilities_UnlimitedMaxActive(t *testing.T) {
	t.Parallel()

	serviceType := createTestServiceType()
	serviceType.Spec.CredentialKinds[0].Constraints.MaxActive = "unlimited"

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: serviceType,
	}

	caps := svc.Capabilities()
	assert.Equal(t, -1, caps.MaxActiveKeys) // Unlimited
}

func TestDataDrivenService_Capabilities_StringMaxActive(t *testing.T) {
	t.Parallel()

	serviceType := createTestServiceType()
	serviceType.Spec.CredentialKinds[0].Constraints.MaxActive = "10"

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: serviceType,
	}

	caps := svc.Capabilities()
	assert.Equal(t, 10, caps.MaxActiveKeys)
}

func TestDataDrivenService_Capabilities_NoExpiration(t *testing.T) {
	t.Parallel()

	serviceType := createTestServiceType()
	serviceType.Spec.CredentialKinds[0].Constraints.TTL = "" // No TTL

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: serviceType,
	}

	caps := svc.Capabilities()
	assert.False(t, caps.SupportsExpiration)
}

func TestDataDrivenService_Validate_Success(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	ctx := context.Background()
	err := svc.Validate(ctx)
	assert.NoError(t, err)
}

func TestDataDrivenService_Validate_NilServiceType(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: nil,
	}

	ctx := context.Background()
	err := svc.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service type definition is nil")
}

func TestDataDrivenService_Validate_EmptyName(t *testing.T) {
	t.Parallel()

	serviceType := createTestServiceType()
	serviceType.Metadata.Name = ""

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: serviceType,
	}

	ctx := context.Background()
	err := svc.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "service type name is empty")
}

func TestDataDrivenService_Validate_NoCredentialKinds(t *testing.T) {
	t.Parallel()

	serviceType := createTestServiceType()
	serviceType.Spec.CredentialKinds = []CredentialKind{}

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: serviceType,
	}

	ctx := context.Background()
	err := svc.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no credential kinds defined")
}

func TestDataDrivenService_Validate_NoCapabilities(t *testing.T) {
	t.Parallel()

	serviceType := createTestServiceType()
	serviceType.Spec.CredentialKinds[0].Capabilities = []string{}

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: serviceType,
	}

	ctx := context.Background()
	err := svc.Validate(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no capabilities defined")
}

func TestDataDrivenService_Verify_NotImplemented(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	ctx := context.Background()
	result := service.RotationResult{}
	err := svc.Verify(ctx, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestDataDrivenService_Rollback_NotImplemented(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	ctx := context.Background()
	result := service.RotationResult{}
	err := svc.Rollback(ctx, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
}

func TestDataDrivenService_GetStatus_NotImplemented(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	ctx := context.Background()
	ref := service.ServiceRef{
		Type:     "postgresql",
		Instance: "prod-db",
	}
	status, err := svc.GetStatus(ctx, ref)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not yet implemented")
	assert.Equal(t, "unknown", status.Status)
	assert.NotEmpty(t, status.Warnings)
}

func TestDataDrivenService_GetProtocolType(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		category     string
		serviceName  string
		expectedType string
	}{
		{
			name:         "SQL database",
			category:     "database",
			serviceName:  "postgresql",
			expectedType: "sql",
		},
		{
			name:         "MongoDB (NoSQL)",
			category:     "database",
			serviceName:  "mongodb",
			expectedType: "nosql",
		},
		{
			name:         "Redis (NoSQL)",
			category:     "database",
			serviceName:  "redis",
			expectedType: "nosql",
		},
		{
			name:         "DynamoDB (NoSQL)",
			category:     "database",
			serviceName:  "dynamodb",
			expectedType: "nosql",
		},
		{
			name:         "API service",
			category:     "api-service",
			serviceName:  "stripe",
			expectedType: "http-api",
		},
		{
			name:         "API category",
			category:     "api",
			serviceName:  "github",
			expectedType: "http-api",
		},
		{
			name:         "Certificate",
			category:     "certificate",
			serviceName:  "letsencrypt",
			expectedType: "certificate",
		},
		{
			name:         "Certificates plural",
			category:     "certificates",
			serviceName:  "acme",
			expectedType: "certificate",
		},
		{
			name:         "Unknown category defaults to HTTP API",
			category:     "unknown",
			serviceName:  "custom",
			expectedType: "http-api",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			serviceType := createTestServiceType()
			serviceType.Metadata.Category = tc.category
			serviceType.Metadata.Name = tc.serviceName

			svc := &DataDrivenService{
				name:        "test",
				serviceType: serviceType,
			}

			protocolType := svc.getProtocolType()
			assert.Equal(t, tc.expectedType, protocolType)
		})
	}
}

func TestDataDrivenService_BuildAdapterConfig(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
		config: config.ServiceConfig{
			Type: "postgresql",
			Config: map[string]interface{}{
				"host":     "db.example.com",
				"port":     float64(5432),
				"database": "production",
				"username": "admin",
				"password": "secret123",
				"timeout":  float64(30),
			},
		},
	}

	adapterConfig := svc.buildAdapterConfig()

	assert.Equal(t, "db.example.com", adapterConfig.Connection["host"])
	assert.Equal(t, "5432", adapterConfig.Connection["port"])
	assert.Equal(t, "production", adapterConfig.Connection["database"])
	assert.Equal(t, "postgresql", adapterConfig.Connection["type"])
	assert.Equal(t, "admin", adapterConfig.Auth["username"])
	assert.Equal(t, "secret123", adapterConfig.Auth["password"])
	assert.Equal(t, 30, adapterConfig.Timeout)
}

func TestDataDrivenService_BuildAdapterConfig_IntegerPort(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
		config: config.ServiceConfig{
			Type: "postgresql",
			Config: map[string]interface{}{
				"host": "db.example.com",
				"port": "5432", // String port
			},
		},
	}

	adapterConfig := svc.buildAdapterConfig()
	assert.Equal(t, "5432", adapterConfig.Connection["port"])
}

func TestDataDrivenService_BuildAdapterConfig_APIKey(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "api-service",
		serviceType: createTestServiceType(),
		config: config.ServiceConfig{
			Type: "api",
			Config: map[string]interface{}{
				"base_url": "https://api.example.com",
				"api_key":  "sk_test_123",
			},
		},
	}

	adapterConfig := svc.buildAdapterConfig()
	assert.Equal(t, "https://api.example.com", adapterConfig.Connection["base_url"])
	assert.Equal(t, "api_key", adapterConfig.Auth["type"])
	assert.Equal(t, "sk_test_123", adapterConfig.Auth["value"])
}

func TestDataDrivenService_BuildAdapterConfig_BearerToken(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "api-service",
		serviceType: createTestServiceType(),
		config: config.ServiceConfig{
			Type: "api",
			Config: map[string]interface{}{
				"token": "bearer_token_123",
			},
		},
	}

	adapterConfig := svc.buildAdapterConfig()
	assert.Equal(t, "bearer", adapterConfig.Auth["type"])
	assert.Equal(t, "bearer_token_123", adapterConfig.Auth["value"])
}

func TestDataDrivenService_BuildProtocolOperation(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	step := service.RotationStep{
		Name:        "create_new",
		Description: "Create new password",
		Action:      "create",
		Target:      "password:new",
	}

	plan := service.RotationPlan{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "prod-db",
			Kind:     "password",
		},
		Strategy: "two-key",
		Metadata: map[string]string{
			"owner": "platform-team",
		},
	}

	operation := svc.buildProtocolOperation(step, plan)

	assert.Equal(t, "create", operation.Action)
	assert.Equal(t, "password:new", operation.Target)
	assert.Equal(t, "platform-team", operation.Parameters["owner"])
	assert.Equal(t, true, operation.Parameters["generate"])
	assert.Equal(t, "password", operation.Parameters["credential_kind"])
	assert.Equal(t, "postgresql", operation.Metadata["service_type"])
	assert.Equal(t, "prod-db", operation.Metadata["service_instance"])
	assert.Equal(t, "password", operation.Metadata["credential_kind"])
}

func TestDataDrivenService_BuildProtocolOperation_WithNewValue(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	step := service.RotationStep{
		Name:   "create_new",
		Action: "create",
		Target: "password:new",
	}

	plan := service.RotationPlan{
		ServiceRef: service.ServiceRef{
			Instance: "prod-db",
		},
		Metadata: map[string]string{
			"new_value": "new_password_123",
		},
	}

	operation := svc.buildProtocolOperation(step, plan)
	assert.Equal(t, "new_password_123", operation.Parameters["value"])
}

func TestDataDrivenService_BuildProtocolOperation_Verify(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	step := service.RotationStep{
		Name:   "verify_new",
		Action: "verify",
		Target: "password:new",
	}

	plan := service.RotationPlan{
		ServiceRef: service.ServiceRef{
			Instance: "prod-db",
		},
		Metadata: map[string]string{
			"verify_value": "test_password",
		},
	}

	operation := svc.buildProtocolOperation(step, plan)
	assert.Equal(t, "verify", operation.Action)
	assert.Equal(t, "test_password", operation.Parameters["value"])
}

func TestDataDrivenService_BuildProtocolOperation_Revoke(t *testing.T) {
	t.Parallel()

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: createTestServiceType(),
	}

	step := service.RotationStep{
		Name:   "revoke_old",
		Action: "delete",
		Target: "password:old",
	}

	plan := service.RotationPlan{
		ServiceRef: service.ServiceRef{
			Instance: "prod-db",
		},
		Metadata: map[string]string{
			"old_value":     "old_password_123",
			"serial_number": "abc123",
		},
	}

	operation := svc.buildProtocolOperation(step, plan)
	assert.Equal(t, "delete", operation.Action)
	assert.Equal(t, "old_password_123", operation.Parameters["value"])
	assert.Equal(t, "abc123", operation.Parameters["serial_number"])
}

func TestDataDrivenService_ExtractServiceConfig(t *testing.T) {
	t.Parallel()

	serviceType := createTestServiceType()
	serviceType.Spec.Defaults.RateLimit = "10/minute"

	svc := &DataDrivenService{
		name:        "prod-db",
		serviceType: serviceType,
	}

	config := svc.extractServiceConfig()

	assert.Equal(t, "10/minute", config["rate_limit"])
	assert.Equal(t, "postgresql", config["service_type"])
	assert.Equal(t, "database", config["category"])
	assert.NotNil(t, config["commands"])
}

func TestDataDrivenService_Execute(t *testing.T) {
	t.Parallel()

	repo := createTestRepository()
	factory := NewDataDrivenServiceFactory(repo)

	cfg := config.ServiceConfig{
		Type: "postgresql",
		Config: map[string]interface{}{
			"host":     "localhost",
			"port":     5432,
			"database": "testdb",
		},
	}

	svc, err := factory.CreateService("test-db", cfg)
	require.NoError(t, err)

	plan := service.RotationPlan{
		ServiceRef: service.ServiceRef{
			Type:     "postgresql",
			Instance: "test-db",
			Kind:     "password",
		},
		Strategy: "immediate",
		Steps: []service.RotationStep{
			{
				Name:        "rotate_immediate",
				Description: "Rotate password immediately",
				Action:      "create",
				Target:      "password",
			},
		},
		EstimatedTime: 60 * time.Second,
		Fingerprint:   "test-fingerprint",
		CreatedAt:     time.Now(),
		Metadata:      map[string]string{},
	}

	ctx := context.Background()
	result, _ := svc.Execute(ctx, plan)
	// Execute may fail due to missing actual database, but should not panic
	// We're testing that the code path works correctly
	assert.NotNil(t, result)
	assert.NotZero(t, result.StartedAt)
	assert.NotZero(t, result.CompletedAt)
}
