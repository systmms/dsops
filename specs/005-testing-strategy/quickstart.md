# Quick Start: Writing Tests for dsops

**Audience**: dsops developers writing unit, integration, and end-to-end tests
**Prerequisites**: Go 1.25+, Docker (for integration tests)
**Status**: Reference Guide

## Overview

This guide provides quick-start examples for writing different types of tests in dsops. All examples follow the TDD (Test-Driven Development) workflow and use our testing infrastructure.

## Test Categories

### 1. Unit Tests (Pure Logic)

**When to use**: Testing functions with no external dependencies (pure logic, transforms, validation).

**Pattern**: Table-driven tests with testify assertions

**Example**: Testing a transform function

```go
// internal/resolve/transforms_test.go
package resolve

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestTransformJSONExtract(t *testing.T) {
    t.Parallel()  // Safe for pure logic tests

    tests := []struct {
        name     string
        input    string
        path     string
        expected string
        wantErr  bool
    }{
        {
            name:     "simple_key",
            input:    `{"key":"value"}`,
            path:     "key",
            expected: "value",
            wantErr:  false,
        },
        {
            name:     "nested_path",
            input:    `{"database":{"password":"secret123"}}`,
            path:     "database.password",
            expected: "secret123",
            wantErr:  false,
        },
        {
            name:     "invalid_json",
            input:    "not json",
            path:     "key",
            expected: "",
            wantErr:  true,
        },
        {
            name:     "missing_key",
            input:    `{"key":"value"}`,
            path:     "nonexistent",
            expected: "",
            wantErr:  true,
        },
    }

    for _, tt := range tests {
        tt := tt  // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            result, err := TransformJSONExtract(tt.input, tt.path)

            if tt.wantErr {
                assert.Error(t, err)
                return
            }

            assert.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**Key Points**:
- Use `t.Parallel()` for independent tests
- Table-driven pattern for multiple test cases
- Clear test names (`simple_key`, `nested_path`)
- Test both success and error paths

---

### 2. Provider Unit Tests (with Fakes)

**When to use**: Testing code that depends on providers, without Docker/real providers.

**Pattern**: Use FakeProvider from `tests/fakes/`

**Example**: Testing resolver with fake provider

```go
// internal/resolve/resolver_test.go
package resolve_test

import (
    "context"
    "testing"

    "github.com/systmms/dsops/internal/resolve"
    "github.com/systmms/dsops/pkg/provider"
    "github.com/systmms/dsops/tests/fakes"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestResolverBasicResolution(t *testing.T) {
    t.Parallel()

    ctx := context.Background()

    // Setup fake provider with test data
    fake := fakes.NewFakeProvider("test").
        WithSecret("db/password", provider.SecretValue{
            Value: map[string]string{"password": "test-secret-123"},
        }).
        WithSecret("api/key", provider.SecretValue{
            Value: map[string]string{"key": "test-api-key-456"},
        })

    // Create resolver
    resolver := resolve.NewResolver(fake)

    // Test resolution
    secret, err := resolver.Resolve(ctx, "store://test/db/password")

    require.NoError(t, err)
    assert.Equal(t, "test-secret-123", secret.Value["password"])
    assert.Equal(t, 1, fake.GetCallCount("Resolve"))
}

func TestResolverErrorHandling(t *testing.T) {
    t.Parallel()

    ctx := context.Background()

    // Setup fake provider to return error
    fake := fakes.NewFakeProvider("test").
        WithError("db/password", errors.New("connection failed"))

    resolver := resolve.NewResolver(fake)

    // Test error propagation
    _, err := resolver.Resolve(ctx, "store://test/db/password")

    assert.Error(t, err)
    assert.Contains(t, err.Error(), "connection failed")
}
```

**Key Points**:
- Use `fakes.NewFakeProvider()` for test data
- Fluent API: `.WithSecret()`, `.WithError()`
- Inspect call counts: `fake.GetCallCount()`
- No Docker required (fast tests)

---

### 3. Integration Tests (with Docker)

**When to use**: Testing with real provider implementations (Vault, PostgreSQL, LocalStack).

**Pattern**: Use `testutil.StartDockerEnv()` for Docker services

**Example**: Vault provider integration test

```go
// internal/providers/vault_test.go
package providers_test

import (
    "context"
    "testing"

    "github.com/systmms/dsops/internal/providers"
    "github.com/systmms/dsops/pkg/provider"
    "github.com/systmms/dsops/tests/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestVaultProviderIntegration(t *testing.T) {
    // Skip if short mode (no Docker)
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Start Docker Compose services
    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    // Wait for services to be healthy
    require.NoError(t, env.WaitForHealthy(30*time.Second))

    ctx := context.Background()

    // Seed test data
    vault := env.VaultClient()
    err := vault.Write("secret/data/test", map[string]any{
        "data": map[string]any{
            "password": "test-secret-123",
            "username": "testuser",
        },
    })
    require.NoError(t, err)

    // Create provider
    vaultProvider := providers.NewVaultProvider(env.VaultConfig())

    // Test resolution
    secret, err := vaultProvider.Resolve(ctx, provider.Reference{
        Key: "secret/data/test",
    })

    require.NoError(t, err)
    assert.Equal(t, "test-secret-123", secret.Value["password"])
    assert.Equal(t, "testuser", secret.Value["username"])
}
```

**Key Points**:
- Check `testing.Short()` to skip in fast mode
- `testutil.StartDockerEnv()` manages Docker lifecycle
- `defer env.Stop()` ensures cleanup
- Use `.VaultClient()` for test data seeding
- Real provider behavior (not faked)

---

### 4. Security Tests (Redaction Validation)

**When to use**: Ensuring secrets are never leaked in logs or errors.

**Pattern**: Use `testutil.NewTestLogger()` to capture logs

**Example**: Testing secret redaction

```go
// internal/logging/redaction_test.go
package logging_test

import (
    "testing"

    "github.com/systmms/dsops/internal/logging"
    "github.com/systmms/dsops/tests/testutil"
    "github.com/stretchr/testify/assert"
)

func TestSecretRedactionInLogs(t *testing.T) {
    t.Parallel()

    logger := testutil.NewTestLogger(t)

    // Log a secret value
    secretValue := "super-secret-password-12345"
    secret := logging.Secret(secretValue)

    logger.Logger().Info("Retrieved secret: %s", secret)
    logger.Logger().Debug("Processing secret: %s", secret)

    // Validate redaction
    output := logger.GetOutput()

    logger.AssertContains(t, "[REDACTED]")
    logger.AssertNotContains(t, secretValue)

    // Ensure multiple log lines both redacted
    assert.Equal(t, 2, strings.Count(output, "[REDACTED]"))
}

func TestErrorMessagesDoNotLeakSecrets(t *testing.T) {
    t.Parallel()

    secretValue := "api-key-abc123"

    // Create error with secret value
    err := fmt.Errorf("failed to validate secret: %s", logging.Secret(secretValue))

    // Error message must not contain secret
    errMsg := err.Error()
    assert.Contains(t, errMsg, "[REDACTED]")
    assert.NotContains(t, errMsg, secretValue)
}
```

**Key Points**:
- Always use `logging.Secret()` wrapper
- Capture logs with `testutil.NewTestLogger()`
- Assert `[REDACTED]` present and secret absent
- Test both Info and Debug levels

---

### 5. Provider Contract Tests

**When to use**: Adding a new provider implementation.

**Pattern**: Use `testutil.RunProviderContractTests()`

**Example**: Running contract tests for new provider

```go
// internal/providers/myapp_test.go
package providers_test

import (
    "testing"

    "github.com/systmms/dsops/internal/providers"
    "github.com/systmms/dsops/pkg/provider"
    "github.com/systmms/dsops/tests/testutil"
    "github.com/stretchr/testify/require"
)

func TestMyAppProviderContract(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Start test environment (if needed)
    env := testutil.StartDockerEnv(t, []string{"myapp"})
    defer env.Stop()

    // Seed test data
    testData := map[string]provider.SecretValue{
        "secret/test1": {
            Value: map[string]string{"password": "test-pass-123"},
        },
        "secret/test2": {
            Value: map[string]string{"api_key": "test-key-456"},
        },
    }

    for key, secret := range testData {
        err := env.MyAppClient().CreateSecret(key, secret.Value)
        require.NoError(t, err)
    }

    // Create provider
    myAppProvider := providers.NewMyAppProvider(env.MyAppConfig())

    // Run ALL contract tests
    tc := testutil.ProviderTestCase{
        Name:     "myapp",
        Provider: myAppProvider,
        TestData: testData,
    }

    testutil.RunProviderContractTests(t, tc)
}
```

**Contract tests automatically verify**:
- `Name()` returns consistent value
- `Resolve()` retrieves secrets correctly
- `Describe()` returns metadata (not values)
- `Capabilities()` returns valid flags
- `Validate()` checks configuration
- Error handling is consistent
- Concurrency safety (no race conditions)

---

### 6. Configuration Tests

**When to use**: Testing config parsing and validation.

**Pattern**: Use `testutil.NewTestConfig()` or `testutil.WriteTestConfig()`

**Example**: Testing config loading

```go
// internal/config/config_test.go
package config_test

import (
    "testing"

    "github.com/systmms/dsops/internal/config"
    "github.com/systmms/dsops/tests/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestConfigParsing(t *testing.T) {
    t.Parallel()

    // Option 1: Programmatic config
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
    defer builder.Cleanup()

    cfg := builder.Build()

    assert.NotNil(t, cfg.SecretStores["vault"])
    assert.Contains(t, cfg.Envs, "test")

    // Option 2: YAML string
    configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  vault:
    type: vault
    addr: http://localhost:8200
`)

    cfg2, err := config.Load(configPath)
    require.NoError(t, err)
    assert.Contains(t, cfg2.SecretStores, "vault")
}

func TestConfigValidation(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name    string
        yaml    string
        wantErr bool
        errMsg  string
    }{
        {
            name: "missing_version",
            yaml: `
secretStores:
  vault:
    type: vault
`,
            wantErr: true,
            errMsg:  "version required",
        },
        {
            name: "invalid_provider_type",
            yaml: `
version: 1
secretStores:
  vault:
    type: nonexistent
`,
            wantErr: true,
            errMsg:  "unknown provider type",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            configPath := testutil.WriteTestConfig(t, tt.yaml)
            _, err := config.Load(configPath)

            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

---

### 7. Command Tests (CLI Testing)

**When to use**: Testing CLI commands end-to-end.

**Pattern**: Execute binary, capture output

**Example**: Testing `dsops plan` command

```go
// cmd/dsops/commands/plan_test.go
package commands_test

import (
    "bytes"
    "context"
    "os/exec"
    "testing"

    "github.com/systmms/dsops/tests/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestPlanCommand(t *testing.T) {
    t.Parallel()

    // Create test config
    configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: literal
    values:
      DATABASE_URL: "postgres://localhost/test"
envs:
  test:
    DATABASE_URL:
      from: store://test/DATABASE_URL
`)

    // Run command
    cmd := exec.Command("dsops", "plan", "--config", configPath, "--env", "test")
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr

    err := cmd.Run()

    // Assertions
    require.NoError(t, err)
    assert.Contains(t, stdout.String(), "DATABASE_URL")
    assert.Contains(t, stdout.String(), "postgres://localhost/test")

    // Should not show secrets redacted in plan output
    assert.NotContains(t, stdout.String(), "[REDACTED]")
}
```

---

## TDD Workflow

### Red-Green-Refactor Cycle

**Step 1: RED - Write failing test**
```go
func TestNewFeature(t *testing.T) {
    result := NewFeature("input")
    assert.Equal(t, "expected", result)
}
// Test fails: NewFeature undefined
```

**Step 2: GREEN - Minimal implementation**
```go
func NewFeature(input string) string {
    return "expected"  // Hardcoded to pass test
}
// Test passes
```

**Step 3: REFACTOR - Improve implementation**
```go
func NewFeature(input string) string {
    // Real implementation
    return processInput(input)
}
// Test still passes, code is clean
```

**Repeat for each acceptance criterion**

---

## Running Tests

**Run all tests** (unit + integration):
```bash
make test-all
```

**Run unit tests only** (fast, no Docker):
```bash
make test
# or
go test -short ./...
```

**Run integration tests** (requires Docker):
```bash
make test-integration
```

**Run with race detection**:
```bash
make test-race
# or
go test -race ./...
```

**Run specific package**:
```bash
go test -v ./internal/providers
```

**Run specific test**:
```bash
go test -v -run TestVaultProvider ./internal/providers
```

**Generate coverage report**:
```bash
make test-coverage
# Opens coverage.html in browser
```

---

## Best Practices

### DO:
- ✅ Write tests BEFORE implementation (TDD)
- ✅ Use table-driven tests for multiple cases
- ✅ Use `t.Parallel()` for independent tests
- ✅ Use `testing.Short()` to skip slow tests
- ✅ Use `t.Cleanup()` for resource cleanup
- ✅ Test both success and error paths
- ✅ Use descriptive test names (`snake_case`)
- ✅ Assert on specific values, not just non-nil

### DON'T:
- ❌ Skip writing tests ("I'll add them later")
- ❌ Test implementation details (test behavior)
- ❌ Use `time.Sleep()` in tests (use channels/sync)
- ❌ Leave commented-out test code
- ❌ Ignore test failures locally
- ❌ Commit failing tests
- ❌ Write tests that depend on execution order
- ❌ Leak secrets in test fixtures

---

## Quick Reference

| Test Type | Pattern | Speed | Docker? |
|-----------|---------|-------|---------|
| Unit Tests | Table-driven + testify | Fast (<1s) | No |
| Provider Unit | FakeProvider | Fast (<1s) | No |
| Integration | DockerTestEnv | Slow (5-10s) | Yes |
| Security | TestLogger | Fast (<1s) | No |
| Contract | RunContractTests | Slow (5-10s) | Yes |
| E2E | Binary exec | Slow (10s+) | Optional |

---

**Quick Start Complete**: 2025-11-14
**Full Documentation**: See `docs/developer/testing.md` (after implementation)
**More Examples**: Browse existing `*_test.go` files in codebase
