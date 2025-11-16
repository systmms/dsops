package dsopsdata

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoader(t *testing.T) {
	loader := NewLoader("/tmp/test-data")
	assert.NotNil(t, loader)
	assert.Equal(t, "/tmp/test-data", loader.dataDir)
	assert.Equal(t, "/tmp/test-data/schemas", loader.schemasDir)
	assert.True(t, loader.enableValidation)
}

func TestNewLoaderWithoutValidation(t *testing.T) {
	loader := NewLoaderWithoutValidation("/tmp/test-data")
	assert.NotNil(t, loader)
	assert.Equal(t, "/tmp/test-data", loader.dataDir)
	assert.False(t, loader.enableValidation)
}

func TestLoader_LoadServiceTypes(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	serviceTypesDir := filepath.Join(tmpDir, "service-types")
	err := os.MkdirAll(serviceTypesDir, 0755)
	require.NoError(t, err)

	// Create a valid service type YAML file
	serviceTypeYAML := `apiVersion: dsops-data/v1
kind: ServiceType
metadata:
  name: postgresql
  description: PostgreSQL database
  category: database
spec:
  credentialKinds:
    - name: password
      description: Database password
      capabilities:
        - create
        - rotate
        - revoke
        - verify
      constraints:
        maxActive: 2
        ttl: "90d"
        format: "alphanumeric"
  defaults:
    rateLimit: "10/minute"
    rotationStrategy: "two-key"
`
	err = os.WriteFile(filepath.Join(serviceTypesDir, "postgresql.yaml"), []byte(serviceTypeYAML), 0644)
	require.NoError(t, err)

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	serviceTypes, err := loader.LoadServiceTypes(ctx)
	require.NoError(t, err)
	assert.Len(t, serviceTypes, 1)

	pgType, exists := serviceTypes["postgresql"]
	assert.True(t, exists)
	assert.Equal(t, "dsops-data/v1", pgType.APIVersion)
	assert.Equal(t, "ServiceType", pgType.Kind)
	assert.Equal(t, "postgresql", pgType.Metadata.Name)
	assert.Equal(t, "PostgreSQL database", pgType.Metadata.Description)
	assert.Equal(t, "database", pgType.Metadata.Category)
	assert.Len(t, pgType.Spec.CredentialKinds, 1)
	assert.Equal(t, "password", pgType.Spec.CredentialKinds[0].Name)
	assert.Contains(t, pgType.Spec.CredentialKinds[0].Capabilities, "create")
	assert.Contains(t, pgType.Spec.CredentialKinds[0].Capabilities, "rotate")
	assert.Equal(t, "two-key", pgType.Spec.Defaults.RotationStrategy)
}

func TestLoader_LoadServiceTypes_InvalidKind(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	serviceTypesDir := filepath.Join(tmpDir, "service-types")
	err := os.MkdirAll(serviceTypesDir, 0755)
	require.NoError(t, err)

	// Create a YAML file with wrong kind
	invalidYAML := `apiVersion: dsops-data/v1
kind: WrongKind
metadata:
  name: test
spec:
  credentialKinds: []
`
	err = os.WriteFile(filepath.Join(serviceTypesDir, "invalid.yaml"), []byte(invalidYAML), 0644)
	require.NoError(t, err)

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	_, err = loader.LoadServiceTypes(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid kind")
	assert.Contains(t, err.Error(), "expected ServiceType, got WrongKind")
}

func TestLoader_LoadServiceInstances(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	instancesDir := filepath.Join(tmpDir, "service-instances")
	err := os.MkdirAll(instancesDir, 0755)
	require.NoError(t, err)

	instanceYAML := `apiVersion: dsops-data/v1
kind: ServiceInstance
metadata:
  type: postgresql
  id: prod-db-01
  name: Production Database
  description: Main production database
  tags:
    - production
    - critical
spec:
  endpoint: "postgres://db.example.com:5432/main"
  auth: "password"
  credentialKinds:
    - name: password
      policy: standard-rotation
      principals:
        - app-server
        - backup-service
      config:
        minLength: 32
  config:
    sslMode: "require"
    maxConnections: 100
`
	err = os.WriteFile(filepath.Join(instancesDir, "prod-db.yaml"), []byte(instanceYAML), 0644)
	require.NoError(t, err)

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	instances, err := loader.LoadServiceInstances(ctx)
	require.NoError(t, err)
	assert.Len(t, instances, 1)

	instance, exists := instances["postgresql/prod-db-01"]
	assert.True(t, exists)
	assert.Equal(t, "postgresql", instance.Metadata.Type)
	assert.Equal(t, "prod-db-01", instance.Metadata.ID)
	assert.Equal(t, "Production Database", instance.Metadata.Name)
	assert.Contains(t, instance.Metadata.Tags, "production")
	assert.Contains(t, instance.Metadata.Tags, "critical")
	assert.Equal(t, "postgres://db.example.com:5432/main", instance.Spec.Endpoint)
	assert.Len(t, instance.Spec.CredentialKinds, 1)
	assert.Equal(t, "password", instance.Spec.CredentialKinds[0].Name)
	assert.Equal(t, "standard-rotation", instance.Spec.CredentialKinds[0].Policy)
}

func TestLoader_LoadRotationPolicies(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	policiesDir := filepath.Join(tmpDir, "rotation-policies")
	err := os.MkdirAll(policiesDir, 0755)
	require.NoError(t, err)

	policyYAML := `apiVersion: dsops-data/v1
kind: RotationPolicy
metadata:
  name: standard-rotation
  description: Standard 90-day rotation policy
spec:
  strategy: "two-key"
  schedule: "0 0 * * 0"
  verification:
    method: "connection"
    timeout: "30s"
    retries: 3
  cutover:
    requireCheck: true
    gracePeriod: "1h"
    rollbackWindow: "24h"
  notifications:
    onSuccess:
      - security-team
    onFailure:
      - security-team
      - on-call
    beforeExpiry:
      targets:
        - credential-owner
      advance: "7d"
  constraints:
    requireApproval: false
    maintenanceWindows:
      - cron: "0 2 * * 6"
        duration: "4h"
        timezone: "UTC"
    excludeEnvironments:
      - emergency
`
	err = os.WriteFile(filepath.Join(policiesDir, "standard.yaml"), []byte(policyYAML), 0644)
	require.NoError(t, err)

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	policies, err := loader.LoadRotationPolicies(ctx)
	require.NoError(t, err)
	assert.Len(t, policies, 1)

	policy, exists := policies["standard-rotation"]
	assert.True(t, exists)
	assert.Equal(t, "two-key", policy.Spec.Strategy)
	assert.Equal(t, "0 0 * * 0", policy.Spec.Schedule)
	assert.NotNil(t, policy.Spec.Verification)
	assert.Equal(t, "connection", policy.Spec.Verification.Method)
	assert.Equal(t, "30s", policy.Spec.Verification.Timeout)
	assert.Equal(t, 3, policy.Spec.Verification.Retries)
	assert.NotNil(t, policy.Spec.Cutover)
	assert.True(t, policy.Spec.Cutover.RequireCheck)
	assert.Equal(t, "1h", policy.Spec.Cutover.GracePeriod)
	assert.NotNil(t, policy.Spec.Notifications)
	assert.Contains(t, policy.Spec.Notifications.OnSuccess, "security-team")
}

func TestLoader_LoadPrincipals(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	principalsDir := filepath.Join(tmpDir, "principals")
	err := os.MkdirAll(principalsDir, 0755)
	require.NoError(t, err)

	principalYAML := `apiVersion: dsops-data/v1
kind: Principal
metadata:
  name: app-server
  description: Application server service account
  labels:
    tier: application
    environment: production
spec:
  type: service
  team: platform
  environment: production
  permissions:
    allowedServices:
      - postgresql
      - redis
    allowedCredentialKinds:
      - password
      - connection_string
    maxCredentialTTL: "90d"
  contact:
    email: platform@example.com
    slack: "#platform-team"
    oncall: "platform-oncall"
  metadata:
    created_by: admin
    approved: true
`
	err = os.WriteFile(filepath.Join(principalsDir, "app-server.yaml"), []byte(principalYAML), 0644)
	require.NoError(t, err)

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	principals, err := loader.LoadPrincipals(ctx)
	require.NoError(t, err)
	assert.Len(t, principals, 1)

	principal, exists := principals["app-server"]
	assert.True(t, exists)
	assert.Equal(t, "service", principal.Spec.Type)
	assert.Equal(t, "platform", principal.Spec.Team)
	assert.Equal(t, "production", principal.Spec.Environment)
	assert.NotNil(t, principal.Spec.Permissions)
	assert.Contains(t, principal.Spec.Permissions.AllowedServices, "postgresql")
	assert.Equal(t, "90d", principal.Spec.Permissions.MaxCredentialTTL)
	assert.NotNil(t, principal.Spec.Contact)
	assert.Equal(t, "platform@example.com", principal.Spec.Contact.Email)
	assert.Equal(t, "production", principal.Metadata.Labels["environment"])
}

func TestLoader_LoadAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create all directory structures
	dirs := []string{
		filepath.Join(tmpDir, "service-types"),
		filepath.Join(tmpDir, "service-instances"),
		filepath.Join(tmpDir, "rotation-policies"),
		filepath.Join(tmpDir, "principals"),
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
	}

	// Create minimal valid files
	serviceTypeYAML := `apiVersion: dsops-data/v1
kind: ServiceType
metadata:
  name: test-service
spec:
  credentialKinds:
    - name: password
      capabilities:
        - create
`
	err := os.WriteFile(filepath.Join(tmpDir, "service-types", "test.yaml"), []byte(serviceTypeYAML), 0644)
	require.NoError(t, err)

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	repo, err := loader.LoadAll(ctx)
	require.NoError(t, err)
	assert.NotNil(t, repo)
	assert.Len(t, repo.ServiceTypes, 1)
	assert.Len(t, repo.ServiceInstances, 0)
	assert.Len(t, repo.RotationPolicies, 0)
	assert.Len(t, repo.Principals, 0)
}

func TestLoader_EmptyDirectories(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create empty directory structure
	dirs := []string{
		filepath.Join(tmpDir, "service-types"),
		filepath.Join(tmpDir, "service-instances"),
		filepath.Join(tmpDir, "rotation-policies"),
		filepath.Join(tmpDir, "principals"),
	}
	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0755)
		require.NoError(t, err)
	}

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	repo, err := loader.LoadAll(ctx)
	require.NoError(t, err)
	assert.NotNil(t, repo)
	assert.Empty(t, repo.ServiceTypes)
	assert.Empty(t, repo.ServiceInstances)
	assert.Empty(t, repo.RotationPolicies)
	assert.Empty(t, repo.Principals)
}

func TestLoader_InvalidYAMLSyntax(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	serviceTypesDir := filepath.Join(tmpDir, "service-types")
	err := os.MkdirAll(serviceTypesDir, 0755)
	require.NoError(t, err)

	// Create invalid YAML (syntax error)
	invalidYAML := `apiVersion: dsops-data/v1
kind: ServiceType
metadata:
  name: test
  invalid yaml syntax here [[[
`
	err = os.WriteFile(filepath.Join(serviceTypesDir, "invalid.yaml"), []byte(invalidYAML), 0644)
	require.NoError(t, err)

	loader := NewLoaderWithoutValidation(tmpDir)
	ctx := context.Background()

	_, err = loader.LoadServiceTypes(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal")
}

func TestRepository_GetServiceType(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceTypes: map[string]*ServiceType{
			"postgresql": {
				Metadata: struct {
					Name        string `yaml:"name" json:"name"`
					Description string `yaml:"description,omitempty" json:"description,omitempty"`
					Category    string `yaml:"category,omitempty" json:"category,omitempty"`
				}{
					Name: "postgresql",
				},
			},
		},
	}

	st, exists := repo.GetServiceType("postgresql")
	assert.True(t, exists)
	assert.Equal(t, "postgresql", st.Metadata.Name)

	_, exists = repo.GetServiceType("nonexistent")
	assert.False(t, exists)
}

func TestRepository_GetServiceInstance(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceInstances: map[string]*ServiceInstance{
			"postgresql/prod-db": {
				Metadata: struct {
					Type        string   `yaml:"type" json:"type"`
					ID          string   `yaml:"id" json:"id"`
					Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
					Description string   `yaml:"description,omitempty" json:"description,omitempty"`
					Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
				}{
					Type: "postgresql",
					ID:   "prod-db",
				},
			},
		},
	}

	instance, exists := repo.GetServiceInstance("postgresql", "prod-db")
	assert.True(t, exists)
	assert.Equal(t, "prod-db", instance.Metadata.ID)

	_, exists = repo.GetServiceInstance("mysql", "prod-db")
	assert.False(t, exists)
}

func TestRepository_GetRotationPolicy(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		RotationPolicies: map[string]*RotationPolicy{
			"standard": {
				Metadata: struct {
					Name        string `yaml:"name" json:"name"`
					Description string `yaml:"description,omitempty" json:"description,omitempty"`
				}{
					Name: "standard",
				},
			},
		},
	}

	policy, exists := repo.GetRotationPolicy("standard")
	assert.True(t, exists)
	assert.Equal(t, "standard", policy.Metadata.Name)

	_, exists = repo.GetRotationPolicy("nonexistent")
	assert.False(t, exists)
}

func TestRepository_GetPrincipal(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		Principals: map[string]*Principal{
			"app-server": {
				Metadata: struct {
					Name        string            `yaml:"name" json:"name"`
					Description string            `yaml:"description,omitempty" json:"description,omitempty"`
					Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
				}{
					Name: "app-server",
				},
			},
		},
	}

	principal, exists := repo.GetPrincipal("app-server")
	assert.True(t, exists)
	assert.Equal(t, "app-server", principal.Metadata.Name)

	_, exists = repo.GetPrincipal("nonexistent")
	assert.False(t, exists)
}

func TestRepository_ListServiceTypes(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceTypes: map[string]*ServiceType{
			"postgresql": {},
			"mysql":      {},
			"redis":      {},
		},
	}

	names := repo.ListServiceTypes()
	assert.Len(t, names, 3)
	assert.Contains(t, names, "postgresql")
	assert.Contains(t, names, "mysql")
	assert.Contains(t, names, "redis")
}

func TestRepository_ListServiceInstancesByType(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceInstances: map[string]*ServiceInstance{
			"postgresql/prod-db":   {Metadata: struct {
				Type        string   `yaml:"type" json:"type"`
				ID          string   `yaml:"id" json:"id"`
				Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
				Description string   `yaml:"description,omitempty" json:"description,omitempty"`
				Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
			}{Type: "postgresql", ID: "prod-db"}},
			"postgresql/staging-db": {Metadata: struct {
				Type        string   `yaml:"type" json:"type"`
				ID          string   `yaml:"id" json:"id"`
				Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
				Description string   `yaml:"description,omitempty" json:"description,omitempty"`
				Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
			}{Type: "postgresql", ID: "staging-db"}},
			"mysql/prod-db":         {Metadata: struct {
				Type        string   `yaml:"type" json:"type"`
				ID          string   `yaml:"id" json:"id"`
				Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
				Description string   `yaml:"description,omitempty" json:"description,omitempty"`
				Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
			}{Type: "mysql", ID: "prod-db"}},
		},
	}

	pgInstances := repo.ListServiceInstancesByType("postgresql")
	assert.Len(t, pgInstances, 2)

	mysqlInstances := repo.ListServiceInstancesByType("mysql")
	assert.Len(t, mysqlInstances, 1)

	redisInstances := repo.ListServiceInstancesByType("redis")
	assert.Len(t, redisInstances, 0)
}

func TestRepository_ListServiceInstancesByTag(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceInstances: map[string]*ServiceInstance{
			"postgresql/prod-db": {Metadata: struct {
				Type        string   `yaml:"type" json:"type"`
				ID          string   `yaml:"id" json:"id"`
				Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
				Description string   `yaml:"description,omitempty" json:"description,omitempty"`
				Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
			}{Tags: []string{"production", "critical"}}},
			"postgresql/staging-db": {Metadata: struct {
				Type        string   `yaml:"type" json:"type"`
				ID          string   `yaml:"id" json:"id"`
				Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
				Description string   `yaml:"description,omitempty" json:"description,omitempty"`
				Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
			}{Tags: []string{"staging"}}},
			"redis/cache": {Metadata: struct {
				Type        string   `yaml:"type" json:"type"`
				ID          string   `yaml:"id" json:"id"`
				Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
				Description string   `yaml:"description,omitempty" json:"description,omitempty"`
				Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
			}{Tags: []string{"production"}}},
		},
	}

	prodInstances := repo.ListServiceInstancesByTag([]string{"production"})
	assert.Len(t, prodInstances, 2)

	criticalInstances := repo.ListServiceInstancesByTag([]string{"critical"})
	assert.Len(t, criticalInstances, 1)

	stagingInstances := repo.ListServiceInstancesByTag([]string{"staging", "production"})
	assert.Len(t, stagingInstances, 3) // All instances match at least one tag
}

func TestRepository_Validate_Success(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceTypes: map[string]*ServiceType{
			"postgresql": {
				Spec: struct {
					CredentialKinds []CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
					Defaults        struct {
						RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
						RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
					} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
				}{
					CredentialKinds: []CredentialKind{
						{Name: "password"},
					},
				},
			},
		},
		ServiceInstances: map[string]*ServiceInstance{
			"postgresql/prod-db": {
				Metadata: struct {
					Type        string   `yaml:"type" json:"type"`
					ID          string   `yaml:"id" json:"id"`
					Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
					Description string   `yaml:"description,omitempty" json:"description,omitempty"`
					Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
				}{Type: "postgresql", ID: "prod-db"},
				Spec: struct {
					Endpoint        string                 `yaml:"endpoint" json:"endpoint"`
					Auth            string                 `yaml:"auth" json:"auth"`
					CredentialKinds []InstanceCredential   `yaml:"credentialKinds" json:"credentialKinds"`
					Config          map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
				}{
					CredentialKinds: []InstanceCredential{
						{
							Name:       "password",
							Policy:     "standard",
							Principals: []string{"app-server"},
						},
					},
				},
			},
		},
		RotationPolicies: map[string]*RotationPolicy{
			"standard": {},
		},
		Principals: map[string]*Principal{
			"app-server": {},
		},
	}

	err := repo.Validate()
	assert.NoError(t, err)
}

func TestRepository_Validate_UnknownServiceType(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceTypes: map[string]*ServiceType{},
		ServiceInstances: map[string]*ServiceInstance{
			"unknown-type/instance": {
				Metadata: struct {
					Type        string   `yaml:"type" json:"type"`
					ID          string   `yaml:"id" json:"id"`
					Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
					Description string   `yaml:"description,omitempty" json:"description,omitempty"`
					Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
				}{Type: "unknown-type", ID: "instance"},
			},
		},
	}

	err := repo.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown service type")
}

func TestRepository_Validate_UnknownCredentialKind(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceTypes: map[string]*ServiceType{
			"postgresql": {
				Spec: struct {
					CredentialKinds []CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
					Defaults        struct {
						RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
						RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
					} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
				}{
					CredentialKinds: []CredentialKind{
						{Name: "password"},
					},
				},
			},
		},
		ServiceInstances: map[string]*ServiceInstance{
			"postgresql/prod-db": {
				Metadata: struct {
					Type        string   `yaml:"type" json:"type"`
					ID          string   `yaml:"id" json:"id"`
					Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
					Description string   `yaml:"description,omitempty" json:"description,omitempty"`
					Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
				}{Type: "postgresql", ID: "prod-db"},
				Spec: struct {
					Endpoint        string                 `yaml:"endpoint" json:"endpoint"`
					Auth            string                 `yaml:"auth" json:"auth"`
					CredentialKinds []InstanceCredential   `yaml:"credentialKinds" json:"credentialKinds"`
					Config          map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
				}{
					CredentialKinds: []InstanceCredential{
						{
							Name:   "api_key", // Unknown credential kind
							Policy: "standard",
						},
					},
				},
			},
		},
		RotationPolicies: map[string]*RotationPolicy{
			"standard": {},
		},
	}

	err := repo.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown credential kind api_key")
}

func TestRepository_Validate_UnknownRotationPolicy(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceTypes: map[string]*ServiceType{
			"postgresql": {
				Spec: struct {
					CredentialKinds []CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
					Defaults        struct {
						RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
						RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
					} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
				}{
					CredentialKinds: []CredentialKind{
						{Name: "password"},
					},
				},
			},
		},
		ServiceInstances: map[string]*ServiceInstance{
			"postgresql/prod-db": {
				Metadata: struct {
					Type        string   `yaml:"type" json:"type"`
					ID          string   `yaml:"id" json:"id"`
					Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
					Description string   `yaml:"description,omitempty" json:"description,omitempty"`
					Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
				}{Type: "postgresql", ID: "prod-db"},
				Spec: struct {
					Endpoint        string                 `yaml:"endpoint" json:"endpoint"`
					Auth            string                 `yaml:"auth" json:"auth"`
					CredentialKinds []InstanceCredential   `yaml:"credentialKinds" json:"credentialKinds"`
					Config          map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
				}{
					CredentialKinds: []InstanceCredential{
						{
							Name:   "password",
							Policy: "nonexistent-policy",
						},
					},
				},
			},
		},
		RotationPolicies: map[string]*RotationPolicy{},
	}

	err := repo.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown rotation policy nonexistent-policy")
}

func TestRepository_Validate_UnknownPrincipal(t *testing.T) {
	t.Parallel()

	repo := &Repository{
		ServiceTypes: map[string]*ServiceType{
			"postgresql": {
				Spec: struct {
					CredentialKinds []CredentialKind `yaml:"credentialKinds" json:"credentialKinds"`
					Defaults        struct {
						RateLimit        string `yaml:"rateLimit,omitempty" json:"rateLimit,omitempty"`
						RotationStrategy string `yaml:"rotationStrategy,omitempty" json:"rotationStrategy,omitempty"`
					} `yaml:"defaults,omitempty" json:"defaults,omitempty"`
				}{
					CredentialKinds: []CredentialKind{
						{Name: "password"},
					},
				},
			},
		},
		ServiceInstances: map[string]*ServiceInstance{
			"postgresql/prod-db": {
				Metadata: struct {
					Type        string   `yaml:"type" json:"type"`
					ID          string   `yaml:"id" json:"id"`
					Name        string   `yaml:"name,omitempty" json:"name,omitempty"`
					Description string   `yaml:"description,omitempty" json:"description,omitempty"`
					Tags        []string `yaml:"tags,omitempty" json:"tags,omitempty"`
				}{Type: "postgresql", ID: "prod-db"},
				Spec: struct {
					Endpoint        string                 `yaml:"endpoint" json:"endpoint"`
					Auth            string                 `yaml:"auth" json:"auth"`
					CredentialKinds []InstanceCredential   `yaml:"credentialKinds" json:"credentialKinds"`
					Config          map[string]interface{} `yaml:"config,omitempty" json:"config,omitempty"`
				}{
					CredentialKinds: []InstanceCredential{
						{
							Name:       "password",
							Policy:     "standard",
							Principals: []string{"unknown-principal"},
						},
					},
				},
			},
		},
		RotationPolicies: map[string]*RotationPolicy{
			"standard": {},
		},
		Principals: map[string]*Principal{},
	}

	err := repo.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown principal unknown-principal")
}

func TestContains(t *testing.T) {
	t.Parallel()

	slice := []string{"create", "rotate", "verify"}

	assert.True(t, contains(slice, "create"))
	assert.True(t, contains(slice, "rotate"))
	assert.True(t, contains(slice, "verify"))
	assert.False(t, contains(slice, "delete"))
	assert.False(t, contains(slice, ""))

	emptySlice := []string{}
	assert.False(t, contains(emptySlice, "anything"))
}
