# Data Model: Test Infrastructure Components

**Feature**: SPEC-005 Testing Strategy & Infrastructure
**Date**: 2025-11-14
**Status**: Design Complete

## Overview

This document defines the data structures and APIs for dsops test infrastructure. These models enable consistent, maintainable, and reusable test code across unit tests, integration tests, and end-to-end tests.

## Core Test Infrastructure Models

### 1. FakeProvider (Manual Fake)

**Purpose**: Predictable, configurable fake implementation of `provider.Provider` interface for unit testing.

**Location**: `tests/fakes/provider_fake.go`

**Interface Compliance**: Implements `pkg/provider/provider.go:Provider`

```go
type FakeProvider struct {
    // Configuration
    name          string
    capabilities  provider.Capabilities

    // Test data
    secrets       map[string]provider.SecretValue  // key -> secret value
    metadata      map[string]provider.Metadata     // key -> metadata

    // Behavior control
    failOn        map[string]error                 // key -> error to return
    resolveDelay  time.Duration                    // simulate network latency
    callCount     map[string]int                   // method call tracking

    // Thread safety
    mu            sync.RWMutex
}
```

**Methods**:
```go
// Constructor
func NewFakeProvider(name string) *FakeProvider

// Builder methods (fluent API)
func (f *FakeProvider) WithSecret(key string, value provider.SecretValue) *FakeProvider
func (f *FakeProvider) WithMetadata(key string, meta provider.Metadata) *FakeProvider
func (f *FakeProvider) WithError(key string, err error) *FakeProvider
func (f *FakeProvider) WithDelay(d time.Duration) *FakeProvider
func (f *FakeProvider) WithCapability(cap provider.Capability, supported bool) *FakeProvider

// Provider interface implementation
func (f *FakeProvider) Name() string
func (f *FakeProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error)
func (f *FakeProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error)
func (f *FakeProvider) Capabilities() provider.Capabilities
func (f *FakeProvider) Validate(ctx context.Context) error

// Test inspection methods
func (f *FakeProvider) GetCallCount(method string) int
func (f *FakeProvider) ResetCallCount()
```

**Usage Example**:
```go
func TestResolverWithFakeProvider(t *testing.T) {
    // Setup fake provider with test data
    fake := fakes.NewFakeProvider("test").
        WithSecret("db/password", provider.SecretValue{
            Value: map[string]string{"password": "test-secret-123"},
        }).
        WithMetadata("db/password", provider.Metadata{
            Version: "v1",
            Tags:    []string{"test"},
        })

    // Use in test
    resolver := resolve.NewResolver(fake)
    secret, err := resolver.Resolve(ctx, "store://test/db/password")

    assert.NoError(t, err)
    assert.Equal(t, "test-secret-123", secret.Value["password"])
    assert.Equal(t, 1, fake.GetCallCount("Resolve"))
}
```

---

### 2. TestConfigBuilder (Configuration Builder)

**Purpose**: Programmatically build dsops.yaml configurations for testing without manual YAML writing.

**Location**: `tests/testutil/config.go`

```go
type TestConfigBuilder struct {
    config        *config.Config
    tempDir       string
    cleanupFuncs  []func()
    t             *testing.T
}
```

**Methods**:
```go
// Constructor
func NewTestConfig(t *testing.T) *TestConfigBuilder

// Builder methods
func (b *TestConfigBuilder) WithSecretStore(name, storeType string, cfg map[string]any) *TestConfigBuilder
func (b *TestConfigBuilder) WithService(name, serviceType string, cfg map[string]any) *TestConfigBuilder
func (b *TestConfigBuilder) WithEnv(name string, vars map[string]config.Variable) *TestConfigBuilder
func (b *TestConfigBuilder) WithProvider(name, providerType string, cfg map[string]any) *TestConfigBuilder  // Legacy

// Output methods
func (b *TestConfigBuilder) Build() *config.Config                    // Returns in-memory config
func (b *TestConfigBuilder) Write() string                            // Writes to temp file, returns path
func (b *TestConfigBuilder) WriteYAML(path string) error              // Writes to specific path
func (b *TestConfigBuilder) Cleanup()                                 // Cleans up temp files
```

**Usage Example**:
```go
func TestConfigParsing(t *testing.T) {
    // Build config programmatically
    builder := testutil.NewTestConfig(t).
        WithSecretStore("vault", "vault", map[string]any{
            "addr": "http://localhost:8200",
            "token": "test-token",
        }).
        WithEnv("test", map[string]config.Variable{
            "DATABASE_URL": {
                From: "store://vault/database/url",
            },
        })
    defer builder.Cleanup()

    // Get config object
    cfg := builder.Build()

    // Or write to file
    configPath := builder.Write()

    // Use in test
    assert.NotNil(t, cfg.SecretStores["vault"])
    assert.FileExists(t, configPath)
}
```

---

### 3. DockerTestEnv (Integration Test Environment)

**Purpose**: Manage Docker Compose lifecycle for integration tests.

**Location**: `tests/testutil/docker.go`

```go
type DockerTestEnv struct {
    composePath   string
    services      []string
    started       bool
    clients       map[string]interface{}  // service name -> client
    cleanupFuncs  []func()
    t             *testing.T
}
```

**Methods**:
```go
// Constructor
func StartDockerEnv(t *testing.T, services []string) *DockerTestEnv
func SkipIfDockerUnavailable(t *testing.T)
func IsDockerAvailable() bool

// Lifecycle
func (e *DockerTestEnv) Stop()
func (e *DockerTestEnv) WaitForHealthy(timeout time.Duration) error

// Client accessors
func (e *DockerTestEnv) VaultClient() *VaultTestClient
func (e *DockerTestEnv) PostgresClient() *sql.DB
func (e *DockerTestEnv) LocalStackClient() *LocalStackTestClient
func (e *DockerTestEnv) MongoClient() *mongo.Client

// Configuration getters
func (e *DockerTestEnv) VaultConfig() map[string]any
func (e *DockerTestEnv) PostgresConfig() map[string]any
func (e *DockerTestEnv) LocalStackConfig() map[string]any
```

**Sub-Types**:

```go
// VaultTestClient wraps Vault API for testing
type VaultTestClient struct {
    client *api.Client
    token  string
}

func (v *VaultTestClient) Write(path string, data map[string]any) error
func (v *VaultTestClient) Read(path string) (map[string]any, error)
func (v *VaultTestClient) Delete(path string) error
func (v *VaultTestClient) ListSecrets(path string) ([]string, error)

// LocalStackTestClient wraps AWS SDK for LocalStack
type LocalStackTestClient struct {
    secretsManager *secretsmanager.SecretsManager
    ssm            *ssm.SSM
}

func (l *LocalStackTestClient) CreateSecret(name string, value map[string]any) error
func (l *LocalStackTestClient) PutParameter(name, value string) error
```

**Usage Example**:
```go
func TestVaultIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Start Docker environment
    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    // Seed test data
    vault := env.VaultClient()
    err := vault.Write("secret/test", map[string]any{
        "password": "test-secret-123",
    })
    require.NoError(t, err)

    // Test provider
    provider := providers.NewVaultProvider(env.VaultConfig())
    secret, err := provider.Resolve(ctx, provider.Reference{Key: "secret/test"})

    assert.NoError(t, err)
    assert.Equal(t, "test-secret-123", secret.Value["password"])
}
```

---

### 4. TestLogger (Log Capture & Validation)

**Purpose**: Capture log output for validation (especially redaction tests).

**Location**: `tests/testutil/logger.go`

```go
type TestLogger struct {
    buffer    *bytes.Buffer
    logger    *logging.Logger
    level     logging.Level
    mu        sync.Mutex
}
```

**Methods**:
```go
// Constructor
func NewTestLogger(t *testing.T) *TestLogger
func NewTestLoggerWithLevel(t *testing.T, level logging.Level) *TestLogger

// Capture methods
func (l *TestLogger) Capture(fn func()) string
func (l *TestLogger) GetOutput() string
func (l *TestLogger) Clear()

// Assertion helpers
func (l *TestLogger) AssertContains(t *testing.T, substr string)
func (l *TestLogger) AssertNotContains(t *testing.T, substr string)
func (l *TestLogger) AssertRedacted(t *testing.T, secretValue string)
func (l *TestLogger) AssertLogCount(t *testing.T, level logging.Level, count int)

// Get underlying logger
func (l *TestLogger) Logger() *logging.Logger
```

**Usage Example**:
```go
func TestSecretRedaction(t *testing.T) {
    logger := testutil.NewTestLogger(t)

    secret := logging.Secret("super-secret-password")
    logger.Logger().Info("Retrieved secret: %s", secret)

    output := logger.GetOutput()
    logger.AssertContains(t, "[REDACTED]")
    logger.AssertNotContains(t, "super-secret-password")
    logger.AssertRedacted(t, "super-secret-password")
}
```

---

### 5. TestFixture (Test Data Management)

**Purpose**: Load and manage test fixtures (configs, secrets, service definitions).

**Location**: `tests/testutil/fixtures.go`

```go
type TestFixture struct {
    baseDir   string
    cache     map[string][]byte
    t         *testing.T
}
```

**Methods**:
```go
// Constructor
func NewTestFixture(t *testing.T) *TestFixture

// Fixture loading
func (f *TestFixture) LoadConfig(name string) *config.Config
func (f *TestFixture) LoadYAML(name string) (map[string]any, error)
func (f *TestFixture) LoadJSON(name string) (map[string]any, error)
func (f *TestFixture) LoadFile(path string) ([]byte, error)

// Fixture paths
func (f *TestFixture) ConfigPath(name string) string
func (f *TestFixture) SecretPath(name string) string
func (f *TestFixture) ServicePath(name string) string
```

**Fixture Directory Structure**:
```text
tests/fixtures/
├── configs/
│   ├── simple.yaml
│   ├── multi-provider.yaml
│   └── rotation.yaml
├── secrets/
│   ├── vault-secrets.json
│   └── aws-secrets.json
└── services/
    ├── postgresql.yaml
    └── mongodb.yaml
```

**Usage Example**:
```go
func TestConfigLoading(t *testing.T) {
    fixtures := testutil.NewTestFixture(t)

    // Load pre-defined test config
    cfg := fixtures.LoadConfig("simple.yaml")

    assert.NotNil(t, cfg)
    assert.Contains(t, cfg.SecretStores, "vault")
}
```

---

## Provider Contract Test Models

### 6. ProviderContractTest (Contract Test Suite)

**Purpose**: Generic contract tests that all providers must pass.

**Location**: `tests/testutil/contract.go`

```go
type ProviderContractTest struct {
    provider      provider.Provider
    testData      map[string]provider.SecretValue
    requiredCaps  []provider.Capability
}
```

**Methods**:
```go
// Constructor
func NewProviderContractTest(p provider.Provider, testData map[string]provider.SecretValue) *ProviderContractTest

// Contract test methods (called by provider tests)
func (c *ProviderContractTest) TestResolve(t *testing.T)
func (c *ProviderContractTest) TestDescribe(t *testing.T)
func (c *ProviderContractTest) TestValidate(t *testing.T)
func (c *ProviderContractTest) TestCapabilities(t *testing.T)
func (c *ProviderContractTest) TestErrorHandling(t *testing.T)
func (c *ProviderContractTest) TestConcurrency(t *testing.T)

// Run all contract tests
func (c *ProviderContractTest) RunAll(t *testing.T)
```

**Usage Example** (per provider):
```go
// internal/providers/vault_test.go
func TestVaultProviderContract(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    // Seed test data
    env.VaultClient().Write("secret/test", map[string]any{"key": "value"})

    // Create provider
    provider := providers.NewVaultProvider(env.VaultConfig())

    // Run contract tests
    contractTest := testutil.NewProviderContractTest(provider, map[string]provider.SecretValue{
        "secret/test": {Value: map[string]string{"key": "value"}},
    })
    contractTest.RunAll(t)
}
```

---

## Test Execution Models

### 7. TestRunner (Parallel Test Execution)

**Purpose**: Coordinate parallel test execution with shared resources.

**Location**: `tests/testutil/runner.go`

```go
type TestRunner struct {
    tests     []TestCase
    parallel  bool
    timeout   time.Duration
    resources *SharedResources
}

type TestCase struct {
    Name      string
    Run       func(t *testing.T)
    Parallel  bool
    Timeout   time.Duration
}

type SharedResources struct {
    DockerEnv *DockerTestEnv
    Fixtures  *TestFixture
    mu        sync.RWMutex
}
```

**Methods**:
```go
// Constructor
func NewTestRunner(parallel bool) *TestRunner

// Add tests
func (r *TestRunner) AddTest(name string, fn func(*testing.T)) *TestRunner
func (r *TestRunner) AddParallelTest(name string, fn func(*testing.T)) *TestRunner

// Shared resources
func (r *TestRunner) WithSharedDocker(services []string) *TestRunner
func (r *TestRunner) WithSharedFixtures() *TestRunner

// Execute
func (r *TestRunner) Run(t *testing.T)
```

**Usage Example**:
```go
func TestProviderSuite(t *testing.T) {
    runner := testutil.NewTestRunner(true).
        WithSharedDocker([]string{"vault", "postgres"})

    runner.
        AddParallelTest("vault_resolve", testVaultResolve).
        AddParallelTest("vault_describe", testVaultDescribe).
        AddTest("vault_rotation", testVaultRotation).  // Sequential (modifies state)
        Run(t)
}
```

---

## Entity Relationships

```
┌─────────────────────┐
│  TestConfigBuilder  │
│  (builds configs)   │
└──────────┬──────────┘
           │
           │ produces
           ▼
     ┌──────────┐
     │  Config  │
     └──────────┘
           │
           │ used by
           ▼
     ┌──────────┐
     │ Provider │◄──────┐
     └──────────┘       │
           │            │ implements
           │ tested by  │
           ▼            │
┌──────────────────┐   │
│ ProviderContract │   │
│      Test        │   │
└──────────────────┘   │
                       │
              ┌────────┴────────┐
              │  FakeProvider   │
              │  (test double)  │
              └─────────────────┘

┌──────────────────┐
│  DockerTestEnv   │
│ (integration)    │
└────────┬─────────┘
         │
         │ manages
         ▼
   ┌──────────┐
   │  Docker  │
   │ Services │
   └──────────┘
         │
         │ provides
         ▼
   ┌──────────┐
   │  Clients │
   │ (Vault,  │
   │ Postgres)│
   └──────────┘

┌──────────────┐
│ TestLogger   │
│ (captures)   │
└──────┬───────┘
       │
       │ validates
       ▼
  ┌─────────┐
  │  Logs   │
  └─────────┘
```

---

## File Organization

**Test Infrastructure**:
```
tests/
├── fakes/
│   ├── provider_fake.go        # FakeProvider
│   └── secretstore_fake.go     # Other fakes
├── testutil/
│   ├── config.go               # TestConfigBuilder
│   ├── docker.go               # DockerTestEnv
│   ├── logger.go               # TestLogger
│   ├── fixtures.go             # TestFixture
│   ├── contract.go             # ProviderContractTest
│   └── runner.go               # TestRunner
├── fixtures/
│   ├── configs/                # YAML configs
│   ├── secrets/                # Mock secrets
│   └── services/               # Service definitions
└── mocks/                      # mockgen generated mocks
```

---

## Implementation Notes

1. **Thread Safety**: All test utilities are thread-safe for parallel execution
2. **Cleanup**: All resources have automatic cleanup via `t.Cleanup()`
3. **Isolation**: Each test gets isolated resources (unless explicitly shared)
4. **Debugging**: Test utilities provide detailed error messages with context
5. **Performance**: Shared Docker environments reduce test execution time

---

**Data Model Complete**: 2025-11-14
**Next Step**: Phase 1.2 - API Contracts (contracts/)
