# Test-Driven Development (TDD) Workflow

**Audience**: dsops contributors and maintainers
**Status**: Active - Required for all new code
**Last Updated**: 2025-11-15

## Overview

dsops follows Test-Driven Development (TDD) principles as mandated by [Constitution Principle VII](/docs/TERMINOLOGY.md#principle-vii-test-driven-development). This guide explains the TDD workflow, best practices, and provides practical examples for implementing features using TDD.

## Why TDD?

TDD provides several critical benefits for dsops:

1. **Security by Design**: Tests verify secret redaction before code handles real credentials
2. **Provider Reliability**: Contract tests ensure all providers behave consistently
3. **Refactoring Confidence**: Comprehensive tests allow safe refactoring without breaking changes
4. **Living Documentation**: Tests document expected behavior and edge cases
5. **Quality Gates**: CI enforces 80% coverage minimum, ensuring standards don't slip

## The Red-Green-Refactor Cycle

TDD follows a simple three-step cycle:

```
┌──────────┐
│   RED    │  Write a failing test that defines desired behavior
└────┬─────┘
     │
     ▼
┌──────────┐
│  GREEN   │  Write minimal code to make the test pass
└────┬─────┘
     │
     ▼
┌──────────┐
│ REFACTOR │  Improve code quality while keeping tests green
└────┬─────┘
     │
     └──────── (repeat)
```

### Step 1: RED - Write a Failing Test

**Goal**: Define what you want to build through test assertions.

**Example**: Implementing a new transform function

```go
// internal/resolve/transforms_test.go
func TestTransformTrim(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {
            name:     "leading_whitespace",
            input:    "  secret-value",
            expected: "secret-value",
        },
        {
            name:     "trailing_whitespace",
            input:    "secret-value  ",
            expected: "secret-value",
        },
        {
            name:     "both_sides",
            input:    "  secret-value  ",
            expected: "secret-value",
        },
        {
            name:     "no_whitespace",
            input:    "secret-value",
            expected: "secret-value",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            result, err := TransformTrim(tt.input)

            assert.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**Run the test**:
```bash
go test -v ./internal/resolve -run TestTransformTrim
# FAIL: undefined: TransformTrim
```

✅ **Test fails** as expected (RED phase complete)

### Step 2: GREEN - Make the Test Pass

**Goal**: Write the simplest code that makes the test pass.

```go
// internal/resolve/transforms.go
func TransformTrim(input string) (string, error) {
    return strings.TrimSpace(input), nil
}
```

**Run the test**:
```bash
go test -v ./internal/resolve -run TestTransformTrim
# PASS
```

✅ **Test passes** (GREEN phase complete)

### Step 3: REFACTOR - Improve Code Quality

**Goal**: Clean up implementation while keeping tests green.

```go
// internal/resolve/transforms.go

// TransformTrim removes leading and trailing whitespace from secret values.
// This is useful when secrets are stored with accidental whitespace padding.
//
// Example:
//   input:  "  my-secret-key  "
//   output: "my-secret-key"
func TransformTrim(input string) (string, error) {
    if input == "" {
        return "", fmt.Errorf("cannot trim empty string")
    }

    trimmed := strings.TrimSpace(input)

    if trimmed == "" {
        return "", fmt.Errorf("input contains only whitespace")
    }

    return trimmed, nil
}
```

**Add tests for new error cases**:
```go
{
    name:     "empty_string",
    input:    "",
    wantErr:  true,
},
{
    name:     "only_whitespace",
    input:    "   ",
    wantErr:  true,
},
```

**Run tests again**:
```bash
go test -v ./internal/resolve -run TestTransformTrim
# PASS (all cases including error handling)
```

✅ **Refactoring complete** - tests still pass, code is cleaner

## TDD for Different Component Types

### Adding a New Provider

**Step 1: RED - Write Contract Tests**

```go
// internal/providers/newprovider_test.go
func TestNewProviderContract(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Setup test environment
    env := testutil.StartDockerEnv(t, []string{"newprovider"})
    defer env.Stop()

    // Seed test data
    testData := map[string]provider.SecretValue{
        "test/secret": {
            Value: map[string]string{"password": "test-123"},
        },
    }

    for key, secret := range testData {
        require.NoError(t, env.NewProviderClient().CreateSecret(key, secret.Value))
    }

    // Create provider instance
    p := providers.NewMyProvider(env.NewProviderConfig())

    // Run all contract tests
    tc := testutil.ProviderTestCase{
        Name:     "newprovider",
        Provider: p,
        TestData: testData,
    }

    testutil.RunProviderContractTests(t, tc)
}
```

**Run test**: Fails because `NewMyProvider` doesn't exist yet (RED)

**Step 2: GREEN - Implement Provider Interface**

```go
// internal/providers/newprovider.go
type NewProvider struct {
    name   string
    client *newproviderapi.Client
}

func NewMyProvider(config map[string]any) *NewProvider {
    return &NewProvider{
        name: "newprovider",
        client: newproviderapi.NewClient(config),
    }
}

func (p *NewProvider) Name() string {
    return p.name
}

func (p *NewProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    // Minimal implementation
    return p.client.GetSecret(ctx, ref.Key)
}

// ... implement other provider.Provider methods
```

**Run test**: Passes (GREEN)

**Step 3: REFACTOR - Add Error Handling, Retries, etc.**

```go
func (p *NewProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    if ref.Key == "" {
        return provider.SecretValue{}, fmt.Errorf("secret key cannot be empty")
    }

    // Add retry logic
    var secret provider.SecretValue
    err := retry.Do(func() error {
        var retryErr error
        secret, retryErr = p.client.GetSecret(ctx, ref.Key)
        return retryErr
    }, retry.Attempts(3))

    if err != nil {
        return provider.SecretValue{}, fmt.Errorf("failed to resolve %s: %w", ref.Key, err)
    }

    return secret, nil
}
```

### Adding a CLI Command

**Step 1: RED - Write Command Test**

```go
// cmd/dsops/commands/inspect_test.go
func TestInspectCommand(t *testing.T) {
    t.Parallel()

    configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  test:
    type: literal
    values:
      API_KEY: "test-key-123"
`)

    cmd := exec.Command("dsops", "inspect", "--config", configPath, "--secret", "store://test/API_KEY")
    var stdout bytes.Buffer
    cmd.Stdout = &stdout

    err := cmd.Run()

    require.NoError(t, err)
    assert.Contains(t, stdout.String(), "Secret: store://test/API_KEY")
    assert.Contains(t, stdout.String(), "Provider: literal")
    assert.NotContains(t, stdout.String(), "test-key-123") // Should not show actual value
}
```

**Run test**: Fails because `inspect` command doesn't exist (RED)

**Step 2: GREEN - Implement Command**

```go
// cmd/dsops/commands/inspect.go
var inspectCmd = &cobra.Command{
    Use:   "inspect",
    Short: "Inspect secret metadata without revealing values",
    RunE:  runInspect,
}

func runInspect(cmd *cobra.Command, args []string) error {
    secretRef := cmd.Flag("secret").Value.String()

    // Load config, resolve metadata
    meta, err := resolveMeta(secretRef)
    if err != nil {
        return err
    }

    fmt.Printf("Secret: %s\n", secretRef)
    fmt.Printf("Provider: %s\n", meta.Provider)
    // Don't print actual value
    return nil
}
```

**Run test**: Passes (GREEN)

**Step 3: REFACTOR - Add Output Formatting, Flags, etc.**

## TDD Best Practices for dsops

### 1. Security-First Testing

**Always test secret redaction BEFORE handling real secrets**:

```go
func TestNewFeatureRedactsSecrets(t *testing.T) {
    logger := testutil.NewTestLogger(t)

    secretValue := "actual-secret-password"

    // Call feature with secret
    NewFeature(logging.Secret(secretValue))

    // Verify redaction
    output := logger.GetOutput()
    logger.AssertRedacted(t, secretValue)
    logger.AssertContains(t, "[REDACTED]")
}
```

### 2. Test Error Paths, Not Just Happy Paths

```go
tests := []struct {
    name    string
    input   string
    wantErr bool
    errMsg  string
}{
    {
        name:    "success",
        input:   "valid-input",
        wantErr: false,
    },
    {
        name:    "empty_input",
        input:   "",
        wantErr: true,
        errMsg:  "input cannot be empty",
    },
    {
        name:    "invalid_format",
        input:   "bad-format",
        wantErr: true,
        errMsg:  "invalid format",
    },
}
```

### 3. Use Table-Driven Tests for Multiple Cases

**Benefits**: Less code duplication, easier to add new test cases

```go
func TestValidateConfig(t *testing.T) {
    tests := []struct {
        name    string
        config  *config.Config
        wantErr bool
    }{
        {
            name: "valid_config",
            config: &config.Config{
                Version: 1,
                SecretStores: map[string]config.SecretStore{
                    "vault": {Type: "vault"},
                },
            },
            wantErr: false,
        },
        {
            name: "missing_version",
            config: &config.Config{
                SecretStores: map[string]config.SecretStore{
                    "vault": {Type: "vault"},
                },
            },
            wantErr: true,
        },
    }

    for _, tt := range tests {
        tt := tt  // Capture range variable
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            err := ValidateConfig(tt.config)

            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### 4. Start with Unit Tests, Add Integration Tests Later

**Phase 1**: Unit tests with fakes

```go
func TestResolverWithFake(t *testing.T) {
    fake := fakes.NewFakeProvider("test").
        WithSecret("db/password", provider.SecretValue{
            Value: map[string]string{"password": "test-123"},
        })

    resolver := resolve.NewResolver(fake)
    secret, err := resolver.Resolve(ctx, "store://test/db/password")

    assert.NoError(t, err)
    assert.Equal(t, "test-123", secret.Value["password"])
}
```

**Phase 2**: Integration tests with real providers

```go
func TestResolverWithVault(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    // ... test with real Vault
}
```

### 5. Write Tests That Document Behavior

```go
func TestRotationStrategy_TwoKey(t *testing.T) {
    t.Run("creates_new_key_before_deleting_old", func(t *testing.T) {
        // Documents that two-key strategy has overlap period
    })

    t.Run("deletes_old_key_after_new_key_active", func(t *testing.T) {
        // Documents cleanup behavior
    })

    t.Run("fails_if_provider_does_not_support_multiple_keys", func(t *testing.T) {
        // Documents validation requirements
    })
}
```

## Common TDD Pitfalls

### ❌ DON'T: Write Implementation First

```go
// Bad: Implement without tests
func NewFeature(input string) string {
    // 50 lines of untested code
    return result
}
```

### ✅ DO: Write Test First

```go
// Good: Test defines expected behavior
func TestNewFeature(t *testing.T) {
    result := NewFeature("input")
    assert.Equal(t, "expected", result)
}

// Then implement
func NewFeature(input string) string {
    return "expected"  // Start simple
}
```

### ❌ DON'T: Test Implementation Details

```go
// Bad: Tests internal structure
func TestParserInternals(t *testing.T) {
    p := NewParser()
    assert.Equal(t, 3, len(p.internalCache)) // Fragile!
}
```

### ✅ DO: Test Public Behavior

```go
// Good: Tests observable behavior
func TestParser(t *testing.T) {
    p := NewParser()
    result, err := p.Parse("input")
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### ❌ DON'T: Skip Refactor Step

```go
// Bad: Leave messy code after GREEN
func NewFeature(input string) string {
    // Lots of copy-paste
    // No error handling
    // No documentation
    return result
}
```

### ✅ DO: Clean Up After Tests Pass

```go
// Good: Refactor while tests are green
// NewFeature processes input according to spec XYZ.
//
// Returns error if input is invalid or processing fails.
func NewFeature(input string) (string, error) {
    if err := validate(input); err != nil {
        return "", fmt.Errorf("invalid input: %w", err)
    }

    return process(input), nil
}
```

## TDD Workflow Checklist

When implementing a new feature:

- [ ] **Read the spec** - Understand acceptance criteria
- [ ] **Write failing test** - RED phase (test what you want to exist)
- [ ] **Run test** - Verify it fails for the right reason
- [ ] **Write minimal code** - GREEN phase (make test pass)
- [ ] **Run test** - Verify it passes
- [ ] **Refactor code** - Clean up while tests stay green
- [ ] **Run all tests** - Ensure no regressions
- [ ] **Check coverage** - Verify new code is tested
- [ ] **Commit** - Save working, tested code

## Running Tests During Development

**Fast feedback loop** (unit tests only):
```bash
go test -short ./...
```

**Watch mode** (requires entr):
```bash
find . -name '*.go' | entr -c go test -short ./...
```

**Specific package**:
```bash
go test -v ./internal/providers -run TestVault
```

**With coverage**:
```bash
go test -coverprofile=coverage.txt ./internal/resolve
go tool cover -html=coverage.txt
```

**Race detection**:
```bash
go test -race ./...
```

## Integration with CI/CD

All pull requests must:

1. ✅ Have tests for new code
2. ✅ Pass all existing tests
3. ✅ Maintain ≥80% overall coverage
4. ✅ Pass race detector
5. ✅ Include security tests for secret-handling code

**CI automatically enforces** these requirements - PRs that don't meet standards are blocked.

## Further Reading

- [Testing Strategy Guide](./testing.md) - Overview of test categories
- [Test Patterns](./test-patterns.md) - Common test patterns and examples
- [Test Infrastructure Guide](../../tests/README.md) - Docker setup and test utilities
- [Quick Start Guide](../../specs/005-testing-strategy/quickstart.md) - Quick reference for writing tests

---

**Questions?** See [Testing Strategy Spec](/specs/005-testing-strategy/spec.md) or ask in GitHub Discussions.
