# Testing Strategy for dsops

**Last Updated**: 2025-11-15
**Status**: Active - Required for all contributions
**Spec**: [SPEC-005 Testing Strategy](/specs/005-testing-strategy/spec.md)

## Overview

This document outlines the comprehensive testing strategy for dsops, including test categories, coverage requirements, tooling, and best practices. All contributors must follow this strategy to ensure code quality and security.

## Testing Philosophy

dsops follows these testing principles:

1. **Test-Driven Development**: Tests written before implementation (see [TDD Workflow](./tdd-workflow.md))
2. **Security First**: Secret redaction tested before handling real credentials
3. **Provider Contracts**: All providers validate against same contract tests
4. **Comprehensive Coverage**: ≥80% overall, ≥85% for critical packages
5. **Fast Feedback**: Unit tests complete in <5 minutes

## Test Categories

### 1. Unit Tests

**Purpose**: Test pure logic and business rules in isolation

**Characteristics**:
- No external dependencies (databases, APIs, Docker)
- Fast execution (<1 second per test)
- Deterministic results
- Can run in parallel

**When to use**:
- Pure functions (transforms, validation, parsing)
- Business logic (resolution engine, strategy selection)
- Error handling and edge cases

**Example**:
```go
func TestTransformJSONExtract(t *testing.T) {
    t.Parallel()

    result, err := TransformJSONExtract(`{"key":"value"}`, "key")

    assert.NoError(t, err)
    assert.Equal(t, "value", result)
}
```

**Coverage Target**: ≥80% for all packages

---

### 2. Integration Tests

**Purpose**: Test components working together with real external systems

**Characteristics**:
- Uses Docker for external services (Vault, PostgreSQL, LocalStack)
- Slower execution (5-10 seconds per test)
- Requires Docker availability
- May run sequentially to avoid port conflicts

**When to use**:
- Provider implementations (Vault, AWS Secrets Manager)
- Database rotation workflows
- End-to-end CLI commands

**Example**:
```go
func TestVaultProviderIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    // Test with real Vault instance
    provider := providers.NewVaultProvider(env.VaultConfig())
    secret, err := provider.Resolve(ctx, ref)

    assert.NoError(t, err)
}
```

**Coverage Target**: Critical integration paths covered (not measured in %)

---

### 3. Provider Contract Tests

**Purpose**: Ensure all providers implement the `provider.Provider` interface consistently

**Characteristics**:
- Generic tests applied to all providers
- Validates interface compliance
- Tests common error conditions
- Includes concurrency testing

**When to use**:
- Adding a new provider
- Modifying provider interface
- Debugging provider-specific issues

**Example**:
```go
func TestMyProviderContract(t *testing.T) {
    provider := providers.NewMyProvider(config)

    tc := testutil.ProviderTestCase{
        Name:     "myprovider",
        Provider: provider,
        TestData: testData,
    }

    testutil.RunProviderContractTests(t, tc)
}
```

**Contract tests verify**:
- `Name()` returns consistent value
- `Resolve()` retrieves secrets correctly
- `Describe()` returns metadata (not secret values)
- `Capabilities()` returns valid flags
- `Validate()` checks configuration
- Concurrent access is thread-safe

---

### 4. Security Tests

**Purpose**: Validate secret redaction and prevent credential leakage

**Characteristics**:
- Tests logging behavior
- Validates error messages don't leak secrets
- Uses race detector for concurrency safety
- Tests memory cleanup

**When to use**:
- Any code that handles secrets
- Logging code
- Error handling code
- Concurrent secret access

**Example**:
```go
func TestSecretRedaction(t *testing.T) {
    logger := testutil.NewTestLogger(t)

    secretValue := "super-secret-password"
    logger.Logger().Info("Secret: %s", logging.Secret(secretValue))

    output := logger.GetOutput()
    logger.AssertContains(t, "[REDACTED]")
    logger.AssertNotContains(t, secretValue)
}
```

**Security test requirements**:
- All secret-handling code must have redaction tests
- Error messages must not contain secrets
- Race detector must pass (`go test -race`)

---

### 5. End-to-End (E2E) Tests

**Purpose**: Test complete user workflows from CLI invocation to output

**Characteristics**:
- Tests entire application stack
- Executes binary as subprocess
- Validates CLI output and exit codes
- Slowest test category (10+ seconds)

**When to use**:
- Testing complete workflows (init → plan → exec)
- Multi-provider scenarios
- Rotation workflows with multiple steps

**Example**:
```go
func TestExecWorkflow(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping E2E test")
    }

    configPath := testutil.WriteTestConfig(t, configYAML)

    cmd := exec.Command("dsops", "exec", "--config", configPath, "--env", "test", "--", "env")
    output, err := cmd.CombinedOutput()

    require.NoError(t, err)
    assert.Contains(t, string(output), "DATABASE_URL=")
}
```

**Coverage Target**: Major workflows covered (init, plan, exec, rotate)

---

## Test Infrastructure

### Directory Structure

```
tests/
├── integration/              # Integration tests
│   ├── docker-compose.yml    # Test service definitions
│   ├── providers/            # Provider integration tests
│   ├── rotation/             # Rotation workflow tests
│   └── e2e/                  # End-to-end tests
├── fixtures/                 # Test data
│   ├── configs/              # Test dsops.yaml files
│   ├── secrets/              # Mock secret data (JSON)
│   └── services/             # Service definitions
├── fakes/                    # Manual fakes (FakeProvider, etc.)
├── mocks/                    # Generated mocks (mockgen)
└── testutil/                 # Test utilities and helpers
    ├── config.go             # Config helpers (TestConfigBuilder)
    ├── logger.go             # Logger helpers (TestLogger)
    ├── docker.go             # Docker helpers (DockerTestEnv)
    ├── fixtures.go           # Fixture loaders
    └── contract.go           # Provider contract tests
```

### Test Utilities

#### TestConfigBuilder

Programmatically build test configurations:

```go
builder := testutil.NewTestConfig(t).
    WithSecretStore("vault", "vault", map[string]any{
        "addr":  "http://localhost:8200",
        "token": "test-token",
    }).
    WithEnv("test", map[string]config.Variable{
        "DATABASE_URL": {From: "store://vault/db/url"},
    })
defer builder.Cleanup()

cfg := builder.Build()
```

#### FakeProvider

Manual fake for testing without Docker:

```go
fake := fakes.NewFakeProvider("test").
    WithSecret("db/password", provider.SecretValue{
        Value: map[string]string{"password": "test-123"},
    }).
    WithError("bad/secret", errors.New("not found"))

resolver := resolve.NewResolver(fake)
```

#### DockerTestEnv

Manage Docker services for integration tests:

```go
env := testutil.StartDockerEnv(t, []string{"vault", "postgres"})
defer env.Stop()

vaultClient := env.VaultClient()
vaultClient.Write("secret/test", map[string]any{"key": "value"})
```

#### TestLogger

Capture logs for redaction validation:

```go
logger := testutil.NewTestLogger(t)

logger.Logger().Info("Secret: %s", logging.Secret("password"))

logger.AssertContains(t, "[REDACTED]")
logger.AssertNotContains(t, "password")
```

---

## Coverage Requirements

### Overall Coverage Target: ≥80%

**Critical Packages** (≥85% coverage required):
- `internal/providers` - Secret store provider implementations
- `internal/resolve` - Secret resolution engine
- `internal/config` - Configuration parsing and validation
- `internal/rotation` - Rotation engine

**Standard Packages** (≥70% coverage required):
- `cmd/dsops/commands` - CLI commands
- `internal/template` - Template rendering
- `internal/execenv` - Process execution
- `pkg/*` - Public packages

**Infrastructure Packages** (≥60% coverage acceptable):
- `internal/logging` - Logging infrastructure
- `internal/errors` - Error handling utilities

### Coverage Measurement

**Run coverage locally**:
```bash
make test-coverage
# Opens coverage.html in browser
```

**Check coverage percentage**:
```bash
go tool cover -func=coverage.txt | grep total
```

**CI enforcement**:
- Pull requests must maintain ≥80% overall coverage
- CI fails if coverage drops below threshold
- Coverage diff shown in PR comments (via codecov.io)

---

## Running Tests

### Local Development

**Fast unit tests** (recommended for TDD):
```bash
make test
# or
go test -short ./...
```

**With race detection**:
```bash
make test-race
# or
go test -race -short ./...
```

**Integration tests** (requires Docker):
```bash
make test-integration
```

**Full test suite** (unit + integration + race):
```bash
make test-all
```

**Specific package**:
```bash
go test -v ./internal/providers
```

**Specific test**:
```bash
go test -v -run TestVaultProvider ./internal/providers
```

**Coverage report**:
```bash
make test-coverage
# Generates coverage.html
```

### Continuous Integration

**GitHub Actions automatically runs**:
1. Unit tests with race detector
2. Integration tests (Docker-based)
3. Coverage measurement and reporting
4. Coverage gate enforcement (≥80%)

**PR requirements**:
- ✅ All tests pass
- ✅ No race conditions detected
- ✅ Coverage ≥80% overall
- ✅ New code has tests

---

## Test Patterns

### Table-Driven Tests

**Best for**: Multiple test cases with similar structure

```go
func TestValidation(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid",
            input:   "valid-input",
            wantErr: false,
        },
        {
            name:    "empty",
            input:   "",
            wantErr: true,
            errMsg:  "cannot be empty",
        },
    }

    for _, tt := range tests {
        tt := tt  // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            err := Validate(tt.input)

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

### Subtests

**Best for**: Grouping related test cases

```go
func TestRotation(t *testing.T) {
    t.Run("two_key_strategy", func(t *testing.T) {
        // Test two-key specific behavior
    })

    t.Run("immediate_strategy", func(t *testing.T) {
        // Test immediate strategy behavior
    })

    t.Run("overlap_strategy", func(t *testing.T) {
        // Test overlap strategy behavior
    })
}
```

### Parallel Tests

**Best for**: Independent unit tests

```go
func TestIndependentFunction(t *testing.T) {
    t.Parallel()  // Safe because no shared state

    result := Process("input")
    assert.Equal(t, "output", result)
}
```

**Don't use for**:
- Tests that modify global state
- Integration tests sharing Docker containers
- Tests with file system side effects

---

## Best Practices

### DO

✅ Write tests before implementation (TDD)
✅ Test both success and error paths
✅ Use descriptive test names (`snake_case`)
✅ Use table-driven tests for multiple cases
✅ Use `t.Parallel()` for independent tests
✅ Use `testing.Short()` to skip slow tests
✅ Clean up resources with `t.Cleanup()`
✅ Test public behavior, not internals
✅ Use `assert.Equal(t, expected, actual)` (not vice versa)

### DON'T

❌ Skip writing tests ("I'll add them later")
❌ Test implementation details
❌ Use `time.Sleep()` for synchronization
❌ Leave commented-out test code
❌ Ignore test failures locally
❌ Commit failing tests
❌ Write tests that depend on execution order
❌ Leak secrets in test fixtures
❌ Mock everything (use fakes for complex interfaces)

---

## Troubleshooting

### Tests Pass Locally But Fail in CI

**Possible causes**:
- Race condition (run `go test -race ./...` locally)
- Docker not available (integration tests)
- Environment variable differences
- File path assumptions (use `t.TempDir()`)

**Solution**:
```bash
# Replicate CI environment locally
go test -race -v ./...
make test-integration
```

### Slow Test Suite

**Optimizations**:
- Use `-short` flag to skip integration tests during development
- Run specific package instead of entire suite
- Ensure `t.Parallel()` used for unit tests
- Share Docker containers between integration tests

**Example**:
```bash
# Fast iteration during development
go test -short -v ./internal/resolve

# Full suite before committing
make test-all
```

### Coverage Not Updating

**Common issues**:
- Coverage file not being generated
- Tests not actually running (skipped due to `-short`)
- Code not being imported anywhere

**Solution**:
```bash
# Force coverage regeneration
rm coverage.txt coverage.html
go test -coverprofile=coverage.txt ./...
go tool cover -html=coverage.txt -o coverage.html
```

---

## Further Reading

- [TDD Workflow Guide](./tdd-workflow.md) - Red-Green-Refactor cycle
- [Test Patterns](./test-patterns.md) - Common test patterns and examples
- [Test Infrastructure Guide](../../tests/README.md) - Docker setup and utilities
- [Quick Start Guide](../../specs/005-testing-strategy/quickstart.md) - Quick reference

---

**Questions?** See [SPEC-005](/specs/005-testing-strategy/spec.md) or ask in GitHub Discussions.
