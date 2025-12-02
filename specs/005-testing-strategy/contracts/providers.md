# Provider Contract Test Specification

**Purpose**: Define contract tests that ALL providers must pass
**Package**: `internal/providers` (contract_test.go)
**Status**: Design Complete

## Overview

Provider contract tests ensure that all implementations of `provider.Provider` interface behave consistently and correctly. These tests are generic and run against every provider implementation (Bitwarden, 1Password, Vault, AWS, etc.).

## Contract Test Suite

### Test Structure

```go
// internal/providers/contract_test.go
package providers_test

import (
    "context"
    "testing"
    "time"

    "github.com/systmms/dsops/pkg/provider"
    "github.com/systmms/dsops/tests/testutil"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

// ProviderTestCase defines a provider to test
type ProviderTestCase struct {
    Name         string
    Provider     provider.Provider
    TestData     map[string]provider.SecretValue  // key -> expected value
    Setup        func(t *testing.T) error          // Setup test data
    Teardown     func(t *testing.T) error          // Cleanup
    SkipTests    []string                          // Tests to skip (with reason)
}

// TestAllProviders runs contract tests against all providers
func TestAllProviders(t *testing.T) {
    testCases := GetProviderTestCases(t)

    for _, tc := range testCases {
        tc := tc  // Capture range variable
        t.Run(tc.Name, func(t *testing.T) {
            // Setup
            if tc.Setup != nil {
                require.NoError(t, tc.Setup(t))
            }
            defer func() {
                if tc.Teardown != nil {
                    tc.Teardown(t)
                }
            }()

            // Run contract tests
            RunProviderContractTests(t, tc)
        })
    }
}
```

---

## Contract Tests

### 1. Test Name() Method

**Purpose**: Verify provider returns consistent, non-empty name.

**Test**: `TestProviderName`

```go
func testProviderName(t *testing.T, tc ProviderTestCase) {
    name := tc.Provider.Name()

    assert.NotEmpty(t, name, "Provider name must not be empty")
    assert.Equal(t, tc.Name, name, "Provider name must match test case name")

    // Name should be stable across calls
    name2 := tc.Provider.Name()
    assert.Equal(t, name, name2, "Provider name must be consistent")
}
```

**Acceptance Criteria**:
- Name is non-empty string
- Name matches expected provider name
- Name is consistent across multiple calls

---

### 2. Test Resolve() Method

**Purpose**: Verify provider can resolve secrets correctly.

**Test**: `TestProviderResolve`

```go
func testProviderResolve(t *testing.T, tc ProviderTestCase) {
    ctx := context.Background()

    for key, expectedValue := range tc.TestData {
        t.Run(key, func(t *testing.T) {
            // Resolve secret
            ref := provider.Reference{Key: key}
            secret, err := tc.Provider.Resolve(ctx, ref)

            // Assertions
            require.NoError(t, err, "Resolve should not error for valid key")
            assert.NotNil(t, secret.Value, "Secret value must not be nil")
            assert.Equal(t, expectedValue.Value, secret.Value, "Secret value must match expected")

            // Metadata assertions
            assert.NotEmpty(t, secret.CreatedAt, "CreatedAt should be populated")
            assert.NotEmpty(t, secret.Source, "Source should identify provider")
        })
    }
}
```

**Sub-tests**:

#### Test Resolve with Context Cancellation

```go
func testResolveContextCancellation(t *testing.T, tc ProviderTestCase) {
    ctx, cancel := context.WithCancel(context.Background())
    cancel()  // Cancel immediately

    ref := provider.Reference{Key: firstKey(tc.TestData)}
    _, err := tc.Provider.Resolve(ctx, ref)

    assert.Error(t, err, "Resolve should respect context cancellation")
    assert.Contains(t, err.Error(), "context canceled")
}
```

#### Test Resolve with Timeout

```go
func testResolveTimeout(t *testing.T, tc ProviderTestCase) {
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
    defer cancel()

    time.Sleep(10 * time.Millisecond)  // Ensure timeout

    ref := provider.Reference{Key: firstKey(tc.TestData)}
    _, err := tc.Provider.Resolve(ctx, ref)

    assert.Error(t, err, "Resolve should respect context timeout")
}
```

#### Test Resolve Non-Existent Key

```go
func testResolveNonExistent(t *testing.T, tc ProviderTestCase) {
    ctx := context.Background()
    ref := provider.Reference{Key: "nonexistent/key/that/does/not/exist"}

    _, err := tc.Provider.Resolve(ctx, ref)

    assert.Error(t, err, "Resolve should error for non-existent key")
    assert.Contains(t, err.Error(), "not found", "Error should indicate key not found")
}
```

**Acceptance Criteria**:
- Resolves valid secrets without error
- Returns correct secret values
- Respects context cancellation
- Respects context timeout
- Returns meaningful error for non-existent keys

---

### 3. Test Describe() Method

**Purpose**: Verify provider returns metadata without retrieving secret value.

**Test**: `TestProviderDescribe`

```go
func testProviderDescribe(t *testing.T, tc ProviderTestCase) {
    ctx := context.Background()

    for key := range tc.TestData {
        t.Run(key, func(t *testing.T) {
            // Describe secret
            ref := provider.Reference{Key: key}
            metadata, err := tc.Provider.Describe(ctx, ref)

            // Assertions
            require.NoError(t, err, "Describe should not error for valid key")
            assert.NotEmpty(t, metadata.Key, "Key should be populated")
            assert.Equal(t, key, metadata.Key, "Key should match requested key")

            // Metadata should NOT contain secret value
            assert.Empty(t, metadata.Value, "Describe must not return secret value")

            // Metadata fields
            assert.NotEmpty(t, metadata.CreatedAt, "CreatedAt should be populated")
            assert.NotEmpty(t, metadata.UpdatedAt, "UpdatedAt should be populated")
        })
    }
}
```

**Sub-tests**:

#### Test Describe Non-Existent Key

```go
func testDescribeNonExistent(t *testing.T, tc ProviderTestCase) {
    ctx := context.Background()
    ref := provider.Reference{Key: "nonexistent/key"}

    _, err := tc.Provider.Describe(ctx, ref)

    assert.Error(t, err, "Describe should error for non-existent key")
}
```

**Acceptance Criteria**:
- Returns metadata for valid keys
- Does NOT return secret value
- Populates standard metadata fields (Key, CreatedAt, UpdatedAt)
- Errors for non-existent keys

---

### 4. Test Capabilities() Method

**Purpose**: Verify provider declares capabilities correctly.

**Test**: `TestProviderCapabilities`

```go
func testProviderCapabilities(t *testing.T, tc ProviderTestCase) {
    caps := tc.Provider.Capabilities()

    // All providers must support basic resolution
    assert.True(t, caps.Supports(provider.CapabilityResolve), "Provider must support Resolve")

    // Capabilities should be consistent
    caps2 := tc.Provider.Capabilities()
    assert.Equal(t, caps, caps2, "Capabilities must be consistent")

    // Capabilities should be sensible
    if caps.Supports(provider.CapabilityRotate) {
        assert.True(t, caps.Supports(provider.CapabilityWrite), "Rotation requires write capability")
    }
}
```

**Capability Flags**:
- `CapabilityResolve`: Read secrets (REQUIRED for all providers)
- `CapabilityDescribe`: Retrieve metadata without value
- `CapabilityWrite`: Write/update secrets
- `CapabilityDelete`: Delete secrets
- `CapabilityList`: List available secrets
- `CapabilityRotate`: Support secret rotation
- `CapabilityVersioning`: Support secret versioning

**Acceptance Criteria**:
- `CapabilityResolve` is always true (required)
- Capabilities are consistent across calls
- Capability dependencies are logical (e.g., rotate requires write)

---

### 5. Test Validate() Method

**Purpose**: Verify provider validates configuration and connectivity.

**Test**: `TestProviderValidate`

```go
func testProviderValidate(t *testing.T, tc ProviderTestCase) {
    ctx := context.Background()

    // Valid provider should validate successfully
    err := tc.Provider.Validate(ctx)
    assert.NoError(t, err, "Valid provider configuration should pass validation")

    // Validate should be idempotent
    err2 := tc.Provider.Validate(ctx)
    assert.NoError(t, err2, "Validate should be idempotent")
}
```

**Sub-tests**:

#### Test Validate with Invalid Configuration

```go
func testValidateInvalidConfig(t *testing.T, tc ProviderTestCase) {
    // Create provider with invalid config (wrong address, invalid token, etc.)
    invalidProvider := tc.CreateInvalidProvider(t)

    ctx := context.Background()
    err := invalidProvider.Validate(ctx)

    assert.Error(t, err, "Invalid configuration should fail validation")
    assert.Contains(t, err.Error(), "validation failed", "Error should indicate validation failure")
}
```

**Acceptance Criteria**:
- Valid configuration passes validation
- Invalid configuration fails validation with meaningful error
- Validate is idempotent (can be called multiple times)
- Validate checks connectivity (not just config syntax)

---

### 6. Test Error Handling

**Purpose**: Verify provider returns consistent, helpful errors.

**Test**: `TestProviderErrorHandling`

```go
func testProviderErrorHandling(t *testing.T, tc ProviderTestCase) {
    ctx := context.Background()

    testCases := []struct {
        name      string
        operation func() error
        wantErr   string
    }{
        {
            name: "resolve_not_found",
            operation: func() error {
                _, err := tc.Provider.Resolve(ctx, provider.Reference{Key: "nonexistent"})
                return err
            },
            wantErr: "not found",
        },
        {
            name: "resolve_invalid_key",
            operation: func() error {
                _, err := tc.Provider.Resolve(ctx, provider.Reference{Key: ""})
                return err
            },
            wantErr: "invalid",
        },
        {
            name: "describe_not_found",
            operation: func() error {
                _, err := tc.Provider.Describe(ctx, provider.Reference{Key: "nonexistent"})
                return err
            },
            wantErr: "not found",
        },
    }

    for _, tt := range testCases {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.operation()

            require.Error(t, err, "Operation should return error")
            assert.Contains(t, err.Error(), tt.wantErr, "Error message should be helpful")

            // Error should not contain secrets
            for _, secret := range tc.TestData {
                for _, value := range secret.Value {
                    assert.NotContains(t, err.Error(), value, "Error must not leak secret values")
                }
            }
        })
    }
}
```

**Acceptance Criteria**:
- Errors are non-nil for failure cases
- Error messages are descriptive and actionable
- Errors do NOT contain secret values (redacted)
- Error types are consistent across providers

---

### 7. Test Concurrency Safety

**Purpose**: Verify provider is safe for concurrent use.

**Test**: `TestProviderConcurrency`

```go
func testProviderConcurrency(t *testing.T, tc ProviderTestCase) {
    if testing.Short() {
        t.Skip("Skipping concurrency test in short mode")
    }

    ctx := context.Background()
    key := firstKey(tc.TestData)
    ref := provider.Reference{Key: key}

    // Run 100 concurrent Resolve calls
    const numGoroutines = 100
    errors := make(chan error, numGoroutines)

    for i := 0; i < numGoroutines; i++ {
        go func() {
            _, err := tc.Provider.Resolve(ctx, ref)
            errors <- err
        }()
    }

    // Collect results
    for i := 0; i < numGoroutines; i++ {
        err := <-errors
        assert.NoError(t, err, "Concurrent Resolve should not error")
    }
}
```

**Note**: Run with `-race` flag to detect race conditions.

**Acceptance Criteria**:
- Provider handles concurrent Resolve calls safely
- No race conditions detected by `-race` flag
- No data corruption or crashes under concurrent load

---

## Provider Test Case Implementation

### Example: Vault Provider

```go
// internal/providers/vault_test.go
package providers_test

func TestVaultProviderContract(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping Vault integration test")
    }

    // Start Vault via Docker
    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    // Seed test data
    vault := env.VaultClient()
    testData := map[string]provider.SecretValue{
        "secret/test/db": {
            Value: map[string]string{
                "password": "test-password-123",
                "username": "testuser",
            },
        },
        "secret/test/api": {
            Value: map[string]string{
                "key": "test-api-key-456",
            },
        },
    }

    for key, secret := range testData {
        err := vault.Write(key, secret.Value)
        require.NoError(t, err)
    }

    // Create provider
    provider := providers.NewVaultProvider(env.VaultConfig())

    // Run contract tests
    tc := ProviderTestCase{
        Name:     "vault",
        Provider: provider,
        TestData: testData,
    }

    RunProviderContractTests(t, tc)
}
```

### Example: Bitwarden Provider

```go
// internal/providers/bitwarden_test.go
func TestBitwardenProviderContract(t *testing.T) {
    // Bitwarden requires real CLI, skip if not available
    if !bwCLIAvailable() {
        t.Skip("Bitwarden CLI not available")
    }

    // Setup: Create test vault items (requires BW_SESSION)
    testData := setupBitwardenTestData(t)

    provider := providers.NewBitwardenProvider(config.BitwardenConfig{
        CLI: "bw",
    })

    tc := ProviderTestCase{
        Name:     "bitwarden",
        Provider: provider,
        TestData: testData,
        Teardown: func(t *testing.T) error {
            return cleanupBitwardenTestData(t, testData)
        },
    }

    RunProviderContractTests(t, tc)
}
```

---

## Test Execution

**Run all contract tests**:
```bash
go test -v -run TestAllProviders ./internal/providers
```

**Run specific provider**:
```bash
go test -v -run TestAllProviders/vault ./internal/providers
```

**Run with race detection**:
```bash
go test -race -v -run TestAllProviders ./internal/providers
```

**Skip integration tests** (use fakes):
```bash
go test -short -v -run TestAllProviders ./internal/providers
```

---

## Success Criteria

**Contract tests must**:
1. ✅ Run against ALL provider implementations
2. ✅ Pass for every provider (no exceptions)
3. ✅ Detect interface violations automatically
4. ✅ Run in CI on every PR
5. ✅ Complete in <10 minutes (all providers)

**When adding new provider**:
1. Implement `provider.Provider` interface
2. Create `<provider>_test.go` with contract test
3. All contract tests must pass
4. No provider-specific test exceptions (unless documented)

---

**Contract Complete**: 2025-11-14
**Implementation**: `internal/providers/contract_test.go`
**Usage**: Every provider test file (`*_test.go`) must run `RunProviderContractTests()`
