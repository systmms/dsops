# Test Utilities API Contract

**Package**: `tests/testutil`
**Purpose**: Provide reusable test utilities for dsops testing
**Status**: Design Complete

## Package Overview

The `testutil` package provides helper functions and utilities for writing tests across dsops. All utilities follow Go testing best practices and integrate seamlessly with standard `go test`.

## API Surface

### Configuration Helpers

#### NewTestConfig

```go
func NewTestConfig(t *testing.T) *TestConfigBuilder
```

**Purpose**: Create a new configuration builder for programmatic config creation.

**Parameters**:
- `t *testing.T`: Test context for automatic cleanup

**Returns**: `*TestConfigBuilder` - Fluent builder for configurations

**Example**:
```go
func TestConfigParsing(t *testing.T) {
    builder := testutil.NewTestConfig(t).
        WithSecretStore("vault", "vault", map[string]any{
            "addr": "http://localhost:8200",
        })
    defer builder.Cleanup()

    cfg := builder.Build()
    assert.NotNil(t, cfg)
}
```

---

####WriteTestConfig

```go
func WriteTestConfig(t *testing.T, yaml string) string
```

**Purpose**: Write a YAML configuration string to a temporary file.

**Parameters**:
- `t *testing.T`: Test context
- `yaml string`: YAML configuration content

**Returns**: `string` - Path to temporary config file (auto-cleaned)

**Example**:
```go
func TestConfigLoading(t *testing.T) {
    configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  vault:
    type: vault
    addr: http://localhost:8200
`)
    cfg, err := config.Load(configPath)
    assert.NoError(t, err)
}
```

---

#### LoadTestConfig

```go
func LoadTestConfig(t *testing.T, path string) *config.Config
```

**Purpose**: Load a config file and fail test on error.

**Parameters**:
- `t *testing.T`: Test context
- `path string`: Path to config file (absolute or relative to tests/fixtures/)

**Returns**: `*config.Config` - Parsed configuration

**Example**:
```go
func TestWithFixture(t *testing.T) {
    cfg := testutil.LoadTestConfig(t, "configs/simple.yaml")
    assert.Contains(t, cfg.SecretStores, "vault")
}
```

---

### Provider Helpers

#### NewMockProvider

```go
func NewMockProvider(name string) *FakeProvider
```

**Purpose**: Create an empty mock provider for testing.

**Parameters**:
- `name string`: Provider name

**Returns**: `*FakeProvider` - Configurable fake provider

**Example**:
```go
func TestResolver(t *testing.T) {
    provider := testutil.NewMockProvider("test")
    // Provider has no secrets by default (returns errors)
}
```

---

#### NewFakeProvider

```go
func NewFakeProvider(name string, secrets map[string]string) *FakeProvider
```

**Purpose**: Create a fake provider with predefined secrets.

**Parameters**:
- `name string`: Provider name
- `secrets map[string]string`: Initial secrets (key -> value)

**Returns**: `*FakeProvider` - Pre-populated fake provider

**Example**:
```go
func TestResolveSecret(t *testing.T) {
    provider := testutil.NewFakeProvider("test", map[string]string{
        "db/password": "secret-123",
    })

    secret, err := provider.Resolve(ctx, provider.Reference{Key: "db/password"})
    assert.NoError(t, err)
    assert.Equal(t, "secret-123", secret.Value["password"])
}
```

---

### Logger Helpers

#### NewTestLogger

```go
func NewTestLogger(t *testing.T) *TestLogger
```

**Purpose**: Create a logger that captures output for validation.

**Parameters**:
- `t *testing.T`: Test context

**Returns**: `*TestLogger` - Logger with buffered output

**Example**:
```go
func TestLogging(t *testing.T) {
    logger := testutil.NewTestLogger(t)
    logger.Logger().Info("test message")

    output := logger.GetOutput()
    assert.Contains(t, output, "test message")
}
```

---

#### CaptureLog

```go
func CaptureLog(fn func()) string
```

**Purpose**: Capture log output from a function.

**Parameters**:
- `fn func()`: Function to execute while capturing logs

**Returns**: `string` - Captured log output

**Example**:
```go
func TestSecretRedaction(t *testing.T) {
    output := testutil.CaptureLog(func() {
        secret := logging.Secret("super-secret")
        logging.Info("Secret: %s", secret)
    })

    assert.Contains(t, output, "[REDACTED]")
    assert.NotContains(t, output, "super-secret")
}
```

---

### Environment Helpers

#### SetupTestEnv

```go
func SetupTestEnv(t *testing.T, vars map[string]string)
```

**Purpose**: Set environment variables for test duration (auto-restored).

**Parameters**:
- `t *testing.T`: Test context
- `vars map[string]string`: Environment variables to set

**Example**:
```go
func TestEnvVars(t *testing.T) {
    testutil.SetupTestEnv(t, map[string]string{
        "DSOPS_DEBUG": "true",
        "VAULT_ADDR": "http://localhost:8200",
    })

    // Environment variables available during test
    assert.Equal(t, "true", os.Getenv("DSOPS_DEBUG"))
}
// Environment automatically restored after test
```

---

#### CleanupTestFiles

```go
func CleanupTestFiles(t *testing.T, patterns ...string)
```

**Purpose**: Remove test files matching glob patterns.

**Parameters**:
- `t *testing.T`: Test context
- `patterns ...string`: Glob patterns to delete

**Example**:
```go
func TestFileGeneration(t *testing.T) {
    testutil.CleanupTestFiles(t, "*.test.yaml", "tmp/*")

    // Generate test files
    // ...

    // Cleanup happens automatically via t.Cleanup()
}
```

---

### Docker Helpers (Integration Tests)

#### StartDockerEnv

```go
func StartDockerEnv(t *testing.T, services []string) *DockerTestEnv
```

**Purpose**: Start Docker Compose services for integration testing.

**Parameters**:
- `t *testing.T`: Test context
- `services []string`: Service names to start (from docker-compose.yml)

**Returns**: `*DockerTestEnv` - Environment with client accessors

**Example**:
```go
func TestVaultIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    vault := env.VaultClient()
    vault.Write("secret/test", map[string]any{"key": "value"})
}
```

---

#### SkipIfDockerUnavailable

```go
func SkipIfDockerUnavailable(t *testing.T)
```

**Purpose**: Skip test if Docker is not available.

**Parameters**:
- `t *testing.T`: Test context

**Example**:
```go
func TestDockerRequired(t *testing.T) {
    testutil.SkipIfDockerUnavailable(t)

    // Docker is available, continue test
}
```

---

#### IsDockerAvailable

```go
func IsDockerAvailable() bool
```

**Purpose**: Check if Docker is available without skipping test.

**Returns**: `bool` - True if Docker daemon is accessible

**Example**:
```go
func TestConditionalDocker(t *testing.T) {
    if testutil.IsDockerAvailable() {
        // Run Docker-based test
    } else {
        // Run mock-based test
    }
}
```

---

### Assertion Helpers (extends testify)

#### AssertSecretRedacted

```go
func AssertSecretRedacted(t *testing.T, output, secretValue string)
```

**Purpose**: Assert that secret value is redacted in output.

**Parameters**:
- `t *testing.T`: Test context
- `output string`: Log or error output to check
- `secretValue string`: Secret value that should be redacted

**Example**:
```go
func TestRedaction(t *testing.T) {
    secret := "super-secret-password"
    output := fmt.Sprintf("Secret: %s", logging.Secret(secret))

    testutil.AssertSecretRedacted(t, output, secret)
    // Checks: output contains "[REDACTED]" AND does not contain "super-secret-password"
}
```

---

#### AssertFileContents

```go
func AssertFileContents(t *testing.T, path, expected string)
```

**Purpose**: Assert file exists and contains expected content.

**Parameters**:
- `t *testing.T`: Test context
- `path string`: File path
- `expected string`: Expected file contents

**Example**:
```go
func TestFileGeneration(t *testing.T) {
    // Generate file
    err := render.RenderToFile("output.env", vars)
    assert.NoError(t, err)

    testutil.AssertFileContents(t, "output.env", "DATABASE_URL=postgres://...")
}
```

---

### Test Fixtures

#### LoadFixture

```go
func LoadFixture(t *testing.T, path string) []byte
```

**Purpose**: Load test fixture file from tests/fixtures/.

**Parameters**:
- `t *testing.T`: Test context
- `path string`: Relative path within fixtures/ directory

**Returns**: `[]byte` - File contents

**Example**:
```go
func TestWithFixture(t *testing.T) {
    yamlData := testutil.LoadFixture(t, "configs/simple.yaml")
    cfg, err := config.ParseYAML(yamlData)
    assert.NoError(t, err)
}
```

---

#### LoadFixtureJSON

```go
func LoadFixtureJSON(t *testing.T, path string) map[string]any
```

**Purpose**: Load and parse JSON fixture.

**Parameters**:
- `t *testing.T`: Test context
- `path string`: Relative path to JSON file

**Returns**: `map[string]any` - Parsed JSON

**Example**:
```go
func TestSecretData(t *testing.T) {
    secrets := testutil.LoadFixtureJSON(t, "secrets/vault-secrets.json")
    assert.Contains(t, secrets, "database")
}
```

---

## Error Handling

All testutil functions follow these conventions:

1. **Fatal errors** (configuration issues): Call `t.Fatal()` to stop test
2. **Expected errors** (test assertions): Return error for assertion
3. **Cleanup**: All resources auto-cleanup via `t.Cleanup()`

## Thread Safety

- All utilities are safe for parallel test execution (`t.Parallel()`)
- Docker environment uses singleton pattern (shared across parallel tests)
- Test loggers use buffered output (no race conditions)

## Performance Considerations

- Docker environment starts services once (reused across tests)
- Config builder caches parsed configs
- Fixture loader caches file reads

## Example Test Suite

```go
package mypackage_test

import (
    "testing"
    "github.com/systmms/dsops/tests/testutil"
    "github.com/stretchr/testify/assert"
)

func TestCompleteWorkflow(t *testing.T) {
    // Setup environment
    testutil.SetupTestEnv(t, map[string]string{
        "DSOPS_DEBUG": "true",
    })

    // Create configuration
    builder := testutil.NewTestConfig(t).
        WithSecretStore("vault", "vault", map[string]any{
            "addr": "http://localhost:8200",
        })
    defer builder.Cleanup()

    configPath := builder.Write()

    // Run command
    output := testutil.CaptureLog(func() {
        cmd := exec.Command("dsops", "plan", "--config", configPath)
        err := cmd.Run()
        assert.NoError(t, err)
    })

    // Validate output
    assert.Contains(t, output, "Planning environment")
}

func TestIntegrationWithDocker(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Start Docker services
    env := testutil.StartDockerEnv(t, []string{"vault", "postgres"})
    defer env.Stop()

    // Seed test data
    vault := env.VaultClient()
    err := vault.Write("secret/db", map[string]any{
        "password": "test-password-123",
    })
    assert.NoError(t, err)

    // Test provider
    provider := providers.NewVaultProvider(env.VaultConfig())
    secret, err := provider.Resolve(ctx, provider.Reference{Key: "secret/db"})

    assert.NoError(t, err)
    assert.Equal(t, "test-password-123", secret.Value["password"])
}
```

---

**Contract Complete**: 2025-11-14
**Implementation**: `tests/testutil/*.go`
**Next**: See `contracts/providers.md` for provider contract specification
