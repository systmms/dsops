# dsops Test Infrastructure

**Last Updated**: 2025-11-15
**Purpose**: Test utilities, fixtures, and Docker-based integration test infrastructure
**For**: Contributors writing tests for dsops

## Overview

This directory contains shared test infrastructure used by unit tests, integration tests, and end-to-end tests throughout the dsops codebase. The test utilities provide consistent patterns for building test configurations, managing Docker environments, capturing logs, and validating behavior.

## Directory Structure

```
tests/
├── README.md                  # This file
├── integration/               # Integration tests with Docker
│   ├── docker-compose.yml     # Test service definitions
│   ├── providers/             # Provider integration tests
│   ├── rotation/              # Rotation workflow tests
│   └── e2e/                   # End-to-end CLI tests
├── fixtures/                  # Test data and configurations
│   ├── configs/               # Test dsops.yaml files
│   ├── secrets/               # Mock secret data (JSON)
│   └── services/              # Service definitions
├── fakes/                     # Manual test doubles
│   └── provider_fake.go       # Fake provider.Provider implementation
├── mocks/                     # Generated mocks (mockgen)
│   └── (generated files)
└── testutil/                  # Test utilities and helpers
    ├── assert.go              # Custom assertions (AssertSecretRedacted, etc.)
    ├── config.go              # Config builders (TestConfigBuilder)
    ├── contract.go            # Provider contract tests
    ├── docker.go              # Docker environment management
    ├── env.go                 # Environment variable helpers
    ├── fixtures.go            # Fixture loading
    └── logger.go              # Log capture (TestLogger)
```

## Test Utilities

### testutil Package

Import test utilities:
```go
import "github.com/systmms/dsops/tests/testutil"
```

### TestConfigBuilder

Programmatically build test configurations without manual YAML:

```go
func TestMyFeature(t *testing.T) {
    // Build config programmatically
    builder := testutil.NewTestConfig(t).
        WithSecretStore("vault", "vault", map[string]any{
            "addr":  "http://localhost:8200",
            "token": "test-token",
        }).
        WithEnv("test", map[string]config.Variable{
            "DATABASE_URL": {
                From: "store://vault/database/url",
            },
        })
    defer builder.Cleanup()  // Removes temp files

    // Get in-memory config
    cfg := builder.Build()

    // Or write to temp file
    configPath := builder.Write()

    // Use in tests
    assert.NotNil(t, cfg.SecretStores["vault"])
}
```

**Methods**:
- `NewTestConfig(t)` - Create new builder
- `WithSecretStore(name, type, config)` - Add secret store
- `WithService(name, type, config)` - Add service
- `WithEnv(name, variables)` - Add environment
- `WithProvider(name, type, config)` - Add legacy provider
- `Build()` - Get in-memory config
- `Write()` - Write to temp file, return path
- `Cleanup()` - Remove temp files (automatic via t.Cleanup)

### FakeProvider

Manual fake implementation of `provider.Provider` interface:

```go
import "github.com/systmms/dsops/tests/fakes"

func TestResolution(t *testing.T) {
    // Create fake provider with test data
    fake := fakes.NewFakeProvider("test").
        WithSecret("db/password", provider.SecretValue{
            Value: map[string]string{"password": "test-123"},
        }).
        WithSecret("api/key", provider.SecretValue{
            Value: map[string]string{"key": "test-key-456"},
        }).
        WithError("bad/secret", errors.New("not found"))

    // Use fake in tests
    resolver := resolve.NewResolver(fake)
    secret, err := resolver.Resolve(ctx, "store://test/db/password")

    assert.NoError(t, err)
    assert.Equal(t, "test-123", secret.Value["password"])

    // Verify call count
    assert.Equal(t, 1, fake.GetCallCount("Resolve"))
}
```

**Methods**:
- `NewFakeProvider(name)` - Create fake
- `WithSecret(key, value)` - Add secret data
- `WithMetadata(key, metadata)` - Add metadata
- `WithError(key, err)` - Make key return error
- `WithDelay(duration)` - Simulate network latency
- `WithCapability(cap, supported)` - Set capability flag
- `GetCallCount(method)` - Get call count for inspection
- `ResetCallCount()` - Reset counters

### DockerTestEnv

Manage Docker Compose services for integration tests:

```go
func TestVaultIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Start Docker services
    env := testutil.StartDockerEnv(t, []string{"vault", "postgres"})
    defer env.Stop()  // Automatic cleanup

    // Wait for health checks
    require.NoError(t, env.WaitForHealthy(30*time.Second))

    // Get service clients
    vaultClient := env.VaultClient()
    pgClient := env.PostgresClient()

    // Seed test data
    err := vaultClient.Write("secret/test", map[string]any{
        "password": "test-secret-123",
    })
    require.NoError(t, err)

    // Get provider config
    vaultConfig := env.VaultConfig()

    // Create and test provider
    provider := providers.NewVaultProvider(vaultConfig)
    secret, err := provider.Resolve(ctx, ref)

    assert.NoError(t, err)
}
```

**Methods**:
- `StartDockerEnv(t, services)` - Start Docker Compose with specified services
- `SkipIfDockerUnavailable(t)` - Skip test if Docker not available
- `IsDockerAvailable()` - Check if Docker is installed
- `WaitForHealthy(timeout)` - Wait for service health checks
- `VaultClient()` - Get Vault test client
- `PostgresClient()` - Get PostgreSQL connection
- `LocalStackClient()` - Get LocalStack AWS client
- `MongoClient()` - Get MongoDB client
- `VaultConfig()` - Get provider config map
- `PostgresConfig()` - Get provider config map
- `Stop()` - Stop Docker services

### TestLogger

Capture log output for redaction validation:

```go
func TestSecretRedaction(t *testing.T) {
    logger := testutil.NewTestLogger(t)

    // Log a secret
    secretValue := "super-secret-password"
    logger.Logger().Info("Retrieved secret: %s", logging.Secret(secretValue))

    // Validate redaction
    output := logger.GetOutput()
    logger.AssertContains(t, "[REDACTED]")
    logger.AssertNotContains(t, secretValue)
    logger.AssertRedacted(t, secretValue)
}
```

**Methods**:
- `NewTestLogger(t)` - Create logger with default level
- `NewTestLoggerWithLevel(t, level)` - Create with specific level
- `Capture(fn)` - Capture logs from function
- `GetOutput()` - Get captured log output
- `Clear()` - Clear captured logs
- `AssertContains(t, substr)` - Assert substring present
- `AssertNotContains(t, substr)` - Assert substring absent
- `AssertRedacted(t, secretValue)` - Assert secret redacted
- `Logger()` - Get underlying logger instance

### Provider Contract Tests

Validate provider implements interface correctly:

```go
func TestMyProviderContract(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup test environment (Docker, etc.)
    env := testutil.StartDockerEnv(t, []string{"myprovider"})
    defer env.Stop()

    // Seed test data
    testData := map[string]provider.SecretValue{
        "test/secret1": {
            Value: map[string]string{"password": "test-123"},
        },
        "test/secret2": {
            Value: map[string]string{"api_key": "key-456"},
        },
    }

    for key, secret := range testData {
        require.NoError(t, env.MyProviderClient().CreateSecret(key, secret.Value))
    }

    // Create provider
    provider := providers.NewMyProvider(env.MyProviderConfig())

    // Run ALL contract tests
    tc := testutil.ProviderTestCase{
        Name:     "myprovider",
        Provider: provider,
        TestData: testData,
    }

    testutil.RunProviderContractTests(t, tc)
}
```

**Contract tests verify**:
- `Name()` returns non-empty string
- `Resolve()` retrieves correct secret values
- `Describe()` returns metadata (not secret values)
- `Capabilities()` returns valid capability flags
- `Validate()` checks provider configuration
- Error handling for missing secrets
- Concurrent access is thread-safe

## Fixtures

### Test Configurations

Pre-built configuration files for common test scenarios:

```go
func TestConfigLoading(t *testing.T) {
    fixtures := testutil.NewTestFixture(t)

    // Load pre-defined config
    cfg := fixtures.LoadConfig("simple.yaml")

    assert.NotNil(t, cfg)
}
```

**Available fixtures**:
- `simple.yaml` - Basic single-provider config
- `multi-provider.yaml` - Multiple secret stores
- `rotation.yaml` - Rotation-enabled config

### Custom Fixtures

Add new fixtures to `tests/fixtures/configs/`:

```yaml
# tests/fixtures/configs/my-test.yaml
version: 1

secretStores:
  test:
    type: literal
    values:
      DATABASE_URL: "postgres://localhost/testdb"
      API_KEY: "test-api-key-123"

envs:
  test:
    DATABASE_URL:
      from: store://test/DATABASE_URL
    API_KEY:
      from: store://test/API_KEY
```

Load in tests:
```go
cfg := fixtures.LoadConfig("my-test.yaml")
```

## Docker Integration

### Docker Compose Services

Integration tests use Docker Compose to run real service implementations:

**Available services**:
- `vault` - HashiCorp Vault (secret storage)
- `postgres` - PostgreSQL (database rotation testing)
- `localstack` - LocalStack (AWS service emulation)
- `mongodb` - MongoDB (database rotation testing)

### Starting Services

**Start specific services**:
```go
env := testutil.StartDockerEnv(t, []string{"vault"})
defer env.Stop()
```

**Start multiple services**:
```go
env := testutil.StartDockerEnv(t, []string{"vault", "postgres", "localstack"})
defer env.Stop()
```

### Service Configuration

Services are defined in `tests/integration/docker-compose.yml` with:
- Health checks (automatic readiness detection)
- Port mappings (localhost access)
- Environment variables (test credentials)
- Volumes (data persistence across tests)

**Example**:
```yaml
vault:
  image: hashicorp/vault:1.15
  ports:
    - "8200:8200"
  environment:
    VAULT_DEV_ROOT_TOKEN_ID: test-root-token
  healthcheck:
    test: ["CMD", "vault", "status"]
    interval: 2s
    retries: 10
```

### Running Integration Tests

**Local development**:
```bash
# Run with Docker
make test-integration

# Skip integration tests (fast)
go test -short ./...
```

**CI/CD**:
- Integration tests automatically run in GitHub Actions
- Docker services started via docker-compose
- Tests skip if Docker unavailable

### PostgreSQL Test Limitations

The PostgreSQL integration tests use the lib/pq driver, which has known limitations with concurrent DDL operations on system catalogs. Specifically:

- ❌ **Avoid**: Concurrent CREATE/DROP/ALTER USER operations
- ✅ **OK**: Concurrent SELECT/INSERT/UPDATE queries
- ✅ **OK**: Sequential user management operations

If you encounter "pq: invalid message format" errors in tests involving concurrent user creation, this is a driver limitation. Use sequential tests or the `connection_pool_compatibility` test as a reference for concurrent query patterns.

## Best Practices

### Use Fakes for Unit Tests

✅ **DO**: Use `FakeProvider` for unit tests
```go
fake := fakes.NewFakeProvider("test").WithSecret(...)
resolver := resolve.NewResolver(fake)
```

❌ **DON'T**: Use Docker for pure logic tests
```go
// Slow and unnecessary
env := testutil.StartDockerEnv(t, []string{"vault"})
```

### Skip Integration Tests in Short Mode

✅ **DO**: Check `testing.Short()`
```go
func TestIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    env := testutil.StartDockerEnv(t, []string{"vault"})
    // ...
}
```

### Clean Up Resources

✅ **DO**: Use `defer` for cleanup
```go
env := testutil.StartDockerEnv(t, []string{"vault"})
defer env.Stop()  // Guarantees cleanup even if test fails

builder := testutil.NewTestConfig(t)
defer builder.Cleanup()  // Automatic via t.Cleanup()
```

### Don't Leak Secrets in Fixtures

✅ **DO**: Use fake/mock secrets
```yaml
# tests/fixtures/secrets/test-secrets.json
{
  "password": "test-fake-password-123",
  "api_key": "test-fake-key-456"
}
```

❌ **DON'T**: Use real credentials
```yaml
# BAD - never commit real secrets
{
  "password": "MyActualPassword123!",
  "api_key": "sk-real-openai-key"
}
```

### Parallel Unit Tests

✅ **DO**: Use `t.Parallel()` for independent tests
```go
func TestPureLogic(t *testing.T) {
    t.Parallel()  // Safe - no shared state

    result := Process("input")
    assert.Equal(t, "output", result)
}
```

❌ **DON'T**: Parallelize integration tests
```go
func TestVault(t *testing.T) {
    // NO t.Parallel() - Docker ports conflict
    env := testutil.StartDockerEnv(t, []string{"vault"})
    // ...
}
```

## Troubleshooting

### Docker Services Not Starting

**Symptom**: Integration tests fail with connection errors

**Solution**:
```bash
# Check Docker is running
docker ps

# Manually start services
cd tests/integration
docker-compose up -d

# Check service health
docker-compose ps
docker-compose logs vault

# Stop services
docker-compose down
```

### Port Conflicts

**Symptom**: `bind: address already in use`

**Solution**:
```bash
# Find process using port
lsof -i :8200

# Stop conflicting service
docker-compose down

# Or kill process
kill -9 <PID>
```

### Tests Fail Only in CI

**Symptom**: Tests pass locally but fail in GitHub Actions

**Common causes**:
- Race condition (run `go test -race ./...` locally)
- Docker not available (check `testing.Short()`)
- Timing differences (use health checks, not `time.Sleep()`)
- File path assumptions (use `t.TempDir()`)

## Performance Tips

**Optimize test execution**:
1. Use `-short` flag during development
2. Share Docker containers between tests
3. Run unit tests in parallel (`t.Parallel()`)
4. Cache Docker images locally
5. Use test fixtures instead of rebuilding configs

**Fast iteration**:
```bash
# Unit tests only (fast)
go test -short -v ./internal/resolve

# Watch mode (requires entr)
find . -name '*.go' | entr -c go test -short ./...
```

## Further Reading

- [TDD Workflow Guide](/docs/developer/tdd-workflow.md) - Red-Green-Refactor cycle
- [Testing Strategy](/docs/developer/testing.md) - Overview of test categories
- [Test Patterns](/docs/developer/test-patterns.md) - Common patterns
- [Quick Start](/specs/005-testing-strategy/quickstart.md) - Quick reference

---

**Questions?** See [SPEC-005](/specs/005-testing-strategy/spec.md) or ask in GitHub Discussions.
