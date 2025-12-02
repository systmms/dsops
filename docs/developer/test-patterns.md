# Common Test Patterns for dsops

**Last Updated**: 2025-11-15
**Audience**: dsops contributors
**Purpose**: Reference guide for common test patterns and examples

## Overview

This document provides ready-to-use test patterns for common scenarios in dsops development. Each pattern includes a complete example that you can adapt for your use case.

## Table of Contents

- [Unit Test Patterns](#unit-test-patterns)
  - [Table-Driven Tests](#table-driven-tests)
  - [Subtest Organization](#subtest-organization)
  - [Testing Error Conditions](#testing-error-conditions)
- [Provider Test Patterns](#provider-test-patterns)
  - [Testing with FakeProvider](#testing-with-fakeprovider)
  - [Provider Contract Tests](#provider-contract-tests)
  - [Testing Provider Errors](#testing-provider-errors)
- [Integration Test Patterns](#integration-test-patterns)
  - [Docker-Based Integration Tests](#docker-based-integration-tests)
  - [Multi-Service Integration](#multi-service-integration)
- [Security Test Patterns](#security-test-patterns)
  - [Secret Redaction Tests](#secret-redaction-tests)
  - [Concurrent Access Tests](#concurrent-access-tests)
- [Configuration Test Patterns](#configuration-test-patterns)
  - [Programmatic Config Building](#programmatic-config-building)
  - [Config Validation Tests](#config-validation-tests)
- [CLI Command Test Patterns](#cli-command-test-patterns)
  - [Command Execution Tests](#command-execution-tests)
  - [Output Validation Tests](#output-validation-tests)

---

## Unit Test Patterns

### Table-Driven Tests

**Use case**: Testing multiple inputs/outputs for the same function

**Pattern**:
```go
func TestTransformJSONExtract(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name     string        // Test case name
        input    string        // Input parameters
        path     string
        expected string        // Expected output
        wantErr  bool         // Should error occur?
        errMsg   string       // Expected error message (optional)
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
            errMsg:   "invalid JSON",
        },
        {
            name:     "missing_key",
            input:    `{"key":"value"}`,
            path:     "nonexistent",
            expected: "",
            wantErr:  true,
            errMsg:   "key not found",
        },
    }

    for _, tt := range tests {
        tt := tt  // Capture range variable for parallel tests
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            result, err := TransformJSONExtract(tt.input, tt.path)

            if tt.wantErr {
                assert.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
                return
            }

            assert.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**Key elements**:
- Struct fields document test inputs and expectations
- `tt := tt` captures range variable for parallel execution
- Clear test names using `snake_case`
- Separate error path validation
- Optional error message matching

---

### Subtest Organization

**Use case**: Grouping related test scenarios

**Pattern**:
```go
func TestConfigValidation(t *testing.T) {
    t.Parallel()

    t.Run("version_validation", func(t *testing.T) {
        t.Parallel()

        t.Run("missing_version", func(t *testing.T) {
            t.Parallel()

            cfg := &config.Config{
                SecretStores: map[string]config.SecretStore{
                    "vault": {Type: "vault"},
                },
            }

            err := ValidateConfig(cfg)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), "version required")
        })

        t.Run("unsupported_version", func(t *testing.T) {
            t.Parallel()

            cfg := &config.Config{
                Version: 999,
            }

            err := ValidateConfig(cfg)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), "unsupported version")
        })
    })

    t.Run("provider_validation", func(t *testing.T) {
        t.Parallel()

        t.Run("unknown_provider_type", func(t *testing.T) {
            t.Parallel()

            cfg := &config.Config{
                Version: 1,
                SecretStores: map[string]config.SecretStore{
                    "test": {Type: "nonexistent"},
                },
            }

            err := ValidateConfig(cfg)
            assert.Error(t, err)
            assert.Contains(t, err.Error(), "unknown provider type")
        })
    })
}
```

**Benefits**:
- Logical grouping of related tests
- Clear test hierarchy
- Easy to run specific groups: `go test -run TestConfigValidation/version_validation`

---

### Testing Error Conditions

**Use case**: Validating error handling and messages

**Pattern**:
```go
func TestResolveSecret_ErrorHandling(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name          string
        ref           string
        setupProvider func() provider.Provider
        wantErr       bool
        errContains   string
        errType       error  // Optional: check error type
    }{
        {
            name:    "provider_not_found",
            ref:     "store://unknown/secret",
            setupProvider: func() provider.Provider {
                return nil
            },
            wantErr:     true,
            errContains: "provider not found",
            errType:     ErrProviderNotFound,
        },
        {
            name: "secret_not_found",
            ref:  "store://vault/nonexistent",
            setupProvider: func() provider.Provider {
                return fakes.NewFakeProvider("vault").
                    WithError("nonexistent", ErrSecretNotFound)
            },
            wantErr:     true,
            errContains: "secret not found",
            errType:     ErrSecretNotFound,
        },
        {
            name: "network_timeout",
            ref:  "store://vault/secret",
            setupProvider: func() provider.Provider {
                return fakes.NewFakeProvider("vault").
                    WithError("secret", context.DeadlineExceeded)
            },
            wantErr:     true,
            errContains: "deadline exceeded",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            provider := tt.setupProvider()
            resolver := resolve.NewResolver(provider)

            _, err := resolver.Resolve(context.Background(), tt.ref)

            require.Error(t, err)
            assert.Contains(t, err.Error(), tt.errContains)

            if tt.errType != nil {
                assert.ErrorIs(t, err, tt.errType)
            }
        })
    }
}
```

**Best practices**:
- Test all error paths, not just happy paths
- Validate error messages contain useful context
- Use `errors.Is()` for error type checking
- Use `errors.As()` for error type extraction

---

## Provider Test Patterns

### Testing with FakeProvider

**Use case**: Unit testing code that depends on providers without Docker

**Pattern**:
```go
func TestResolverDependencyChain(t *testing.T) {
    t.Parallel()

    ctx := context.Background()

    // Setup fake provider with multiple secrets
    fake := fakes.NewFakeProvider("test").
        WithSecret("db/host", provider.SecretValue{
            Value: map[string]string{"host": "localhost"},
        }).
        WithSecret("db/port", provider.SecretValue{
            Value: map[string]string{"port": "5432"},
        }).
        WithSecret("db/password", provider.SecretValue{
            Value: map[string]string{"password": "test-pass-123"},
        })

    resolver := resolve.NewResolver(fake)

    // Test resolution
    secrets := []string{
        "store://test/db/host",
        "store://test/db/port",
        "store://test/db/password",
    }

    results := make(map[string]string)
    for _, ref := range secrets {
        secret, err := resolver.Resolve(ctx, ref)
        require.NoError(t, err)

        // Extract first value from map
        for _, v := range secret.Value {
            results[ref] = v
            break
        }
    }

    // Validate all secrets resolved
    assert.Equal(t, "localhost", results["store://test/db/host"])
    assert.Equal(t, "5432", results["store://test/db/port"])
    assert.Equal(t, "test-pass-123", results["store://test/db/password"])

    // Verify call counts
    assert.Equal(t, 3, fake.GetCallCount("Resolve"))
}
```

**When to use FakeProvider**:
- ✅ Unit testing resolution logic
- ✅ Testing transform pipelines
- ✅ Testing error handling
- ✅ Fast feedback during TDD
- ❌ Testing actual provider implementation (use integration tests)

---

### Provider Contract Tests

**Use case**: Validating a provider implementation follows the interface contract

**Pattern**:
```go
func TestVaultProviderContract(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Start Docker environment
    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    ctx := context.Background()

    // Seed test data
    testData := map[string]provider.SecretValue{
        "secret/data/test1": {
            Value: map[string]string{
                "password": "test-password-123",
                "username": "testuser",
            },
        },
        "secret/data/test2": {
            Value: map[string]string{
                "api_key": "test-key-456",
            },
        },
    }

    vault := env.VaultClient()
    for key, secret := range testData {
        err := vault.Write(key, map[string]any{
            "data": secret.Value,
        })
        require.NoError(t, err)
    }

    // Create provider instance
    vaultProvider := providers.NewVaultProvider(env.VaultConfig())

    // Run contract tests
    tc := testutil.ProviderTestCase{
        Name:     "vault",
        Provider: vaultProvider,
        TestData: testData,
    }

    testutil.RunProviderContractTests(t, tc)
}
```

**Contract tests automatically verify**:
- Name() returns non-empty string
- Resolve() retrieves correct values
- Describe() returns metadata without secrets
- Capabilities() returns valid flags
- Validate() checks configuration
- Thread-safe concurrent access

---

### Testing Provider Errors

**Use case**: Validating provider error handling

**Pattern**:
```go
func TestProviderErrorHandling(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name        string
        providerCfg map[string]any
        secretRef   string
        wantErr     bool
        errContains string
    }{
        {
            name: "invalid_address",
            providerCfg: map[string]any{
                "addr":  "invalid://url",
                "token": "test-token",
            },
            wantErr:     true,
            errContains: "invalid address",
        },
        {
            name: "missing_token",
            providerCfg: map[string]any{
                "addr": "http://localhost:8200",
            },
            wantErr:     true,
            errContains: "token required",
        },
        {
            name: "secret_not_found",
            providerCfg: map[string]any{
                "addr":  "http://localhost:8200",
                "token": "test-token",
            },
            secretRef:   "secret/nonexistent",
            wantErr:     true,
            errContains: "not found",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            provider := providers.NewVaultProvider(tt.providerCfg)

            if tt.secretRef != "" {
                _, err := provider.Resolve(context.Background(), provider.Reference{
                    Key: tt.secretRef,
                })
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errContains)
            } else {
                err := provider.Validate(context.Background())
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.errContains)
            }
        })
    }
}
```

---

## Integration Test Patterns

### Docker-Based Integration Tests

**Use case**: Testing with real external services

**Pattern**:
```go
func TestVaultProviderIntegration(t *testing.T) {
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
    err := vault.Write("secret/data/integration-test", map[string]any{
        "data": map[string]any{
            "password": "test-secret-123",
            "username": "testuser",
            "port":     "5432",
        },
    })
    require.NoError(t, err)

    // Create provider with test config
    provider := providers.NewVaultProvider(env.VaultConfig())

    // Validate provider
    err = provider.Validate(ctx)
    require.NoError(t, err)

    // Test resolution
    secret, err := provider.Resolve(ctx, provider.Reference{
        Key: "secret/data/integration-test",
    })
    require.NoError(t, err)

    // Verify secret values
    assert.Equal(t, "test-secret-123", secret.Value["password"])
    assert.Equal(t, "testuser", secret.Value["username"])
    assert.Equal(t, "5432", secret.Value["port"])

    // Test metadata retrieval
    meta, err := provider.Describe(ctx, provider.Reference{
        Key: "secret/data/integration-test",
    })
    require.NoError(t, err)
    assert.NotEmpty(t, meta.Version)

    // Verify metadata doesn't contain secret values
    metaJSON, _ := json.Marshal(meta)
    assert.NotContains(t, string(metaJSON), "test-secret-123")
}
```

**Key patterns**:
- Always check `testing.Short()` to skip in fast mode
- Use `defer env.Stop()` for cleanup
- Wait for health checks before testing
- Test both resolve and metadata operations
- Verify metadata doesn't leak secrets

---

### Multi-Service Integration

**Use case**: Testing workflows that span multiple services

**Pattern**:
```go
func TestRotationWorkflow_PostgreSQL(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Start multiple services
    env := testutil.StartDockerEnv(t, []string{"vault", "postgres"})
    defer env.Stop()

    require.NoError(t, env.WaitForHealthy(30*time.Second))

    ctx := context.Background()

    // Setup: Store initial password in Vault
    vault := env.VaultClient()
    initialPassword := "initial-password-123"
    err := vault.Write("secret/data/postgres/password", map[string]any{
        "data": map[string]any{"password": initialPassword},
    })
    require.NoError(t, err)

    // Setup: Configure PostgreSQL service
    pgClient := env.PostgresClient()
    _, err = pgClient.Exec("CREATE USER testuser WITH PASSWORD $1", initialPassword)
    require.NoError(t, err)

    // Test: Rotate password
    rotator := rotation.NewRotator(
        providers.NewVaultProvider(env.VaultConfig()),
        services.NewPostgreSQLService(env.PostgresConfig()),
    )

    newPassword, err := rotator.Rotate(ctx, rotation.RotateRequest{
        SecretRef:  "secret/data/postgres/password",
        ServiceRef: "svc://postgres",
        Strategy:   "two-key",
    })
    require.NoError(t, err)
    assert.NotEqual(t, initialPassword, newPassword)

    // Verify: New password stored in Vault
    vaultSecret, err := vault.Read("secret/data/postgres/password")
    require.NoError(t, err)
    assert.Equal(t, newPassword, vaultSecret["data"].(map[string]any)["password"])

    // Verify: New password works in PostgreSQL
    testConn, err := sql.Open("postgres", fmt.Sprintf(
        "host=%s user=testuser password=%s dbname=testdb",
        env.PostgresConfig()["host"], newPassword,
    ))
    require.NoError(t, err)
    defer testConn.Close()

    err = testConn.Ping()
    assert.NoError(t, err, "New password should work in PostgreSQL")

    // Verify: Old password no longer works
    oldConn, err := sql.Open("postgres", fmt.Sprintf(
        "host=%s user=testuser password=%s dbname=testdb",
        env.PostgresConfig()["host"], initialPassword,
    ))
    require.NoError(t, err)
    defer oldConn.Close()

    err = oldConn.Ping()
    assert.Error(t, err, "Old password should not work after rotation")
}
```

---

## Security Test Patterns

### Secret Redaction Tests

**Use case**: Ensuring secrets never appear in logs

**Pattern**:
```go
func TestSecretRedactionInLogs(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name       string
        logLevel   logging.Level
        secretVal  string
        logFunc    func(*logging.Logger, logging.SecretString)
    }{
        {
            name:      "info_level",
            logLevel:  logging.InfoLevel,
            secretVal: "super-secret-password-123",
            logFunc: func(l *logging.Logger, s logging.SecretString) {
                l.Info("Retrieved secret: %s", s)
            },
        },
        {
            name:      "debug_level",
            logLevel:  logging.DebugLevel,
            secretVal: "api-key-abc-def-789",
            logFunc: func(l *logging.Logger, s logging.SecretString) {
                l.Debug("Processing secret: %s", s)
            },
        },
        {
            name:      "error_level",
            logLevel:  logging.ErrorLevel,
            secretVal: "database-password-xyz",
            logFunc: func(l *logging.Logger, s logging.SecretString) {
                l.Error("Failed to store secret: %s", s)
            },
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            // Create test logger
            logger := testutil.NewTestLoggerWithLevel(t, tt.logLevel)

            // Log secret value
            secret := logging.Secret(tt.secretVal)
            tt.logFunc(logger.Logger(), secret)

            // Validate redaction
            output := logger.GetOutput()

            logger.AssertContains(t, "[REDACTED]")
            logger.AssertNotContains(t, tt.secretVal)
            logger.AssertRedacted(t, tt.secretVal)
        })
    }
}

func TestErrorMessagesDoNotLeakSecrets(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name      string
        secretVal string
        errFunc   func(logging.SecretString) error
    }{
        {
            name:      "validation_error",
            secretVal: "secret-password-123",
            errFunc: func(s logging.SecretString) error {
                return fmt.Errorf("failed to validate secret: %s", s)
            },
        },
        {
            name:      "connection_error",
            secretVal: "api-key-xyz-789",
            errFunc: func(s logging.SecretString) error {
                return fmt.Errorf("connection failed with key: %s", s)
            },
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            secret := logging.Secret(tt.secretVal)
            err := tt.errFunc(secret)

            errMsg := err.Error()
            assert.Contains(t, errMsg, "[REDACTED]")
            assert.NotContains(t, errMsg, tt.secretVal)
        })
    }
}
```

---

### Concurrent Access Tests

**Use case**: Validating thread-safety with race detector

**Pattern**:
```go
func TestConcurrentProviderAccess(t *testing.T) {
    t.Parallel()

    fake := fakes.NewFakeProvider("test").
        WithSecret("secret1", provider.SecretValue{
            Value: map[string]string{"key": "value1"},
        }).
        WithSecret("secret2", provider.SecretValue{
            Value: map[string]string{"key": "value2"},
        }).
        WithSecret("secret3", provider.SecretValue{
            Value: map[string]string{"key": "value3"},
        })

    // Run concurrent goroutines
    const numGoroutines = 100
    var wg sync.WaitGroup
    wg.Add(numGoroutines)

    errors := make(chan error, numGoroutines)

    for i := 0; i < numGoroutines; i++ {
        go func(id int) {
            defer wg.Done()

            secretKey := fmt.Sprintf("secret%d", (id%3)+1)
            _, err := fake.Resolve(context.Background(), provider.Reference{
                Key: secretKey,
            })

            if err != nil {
                errors <- err
            }
        }(i)
    }

    wg.Wait()
    close(errors)

    // Verify no errors occurred
    for err := range errors {
        t.Errorf("Concurrent access error: %v", err)
    }

    // Run with race detector: go test -race
}
```

**Note**: This test is most valuable when run with `-race` flag:
```bash
go test -race ./internal/providers
```

---

## Configuration Test Patterns

### Programmatic Config Building

**Use case**: Building test configs without YAML files

**Pattern**:
```go
func TestConfigBasedResolution(t *testing.T) {
    t.Parallel()

    // Build config programmatically
    builder := testutil.NewTestConfig(t).
        WithSecretStore("vault", "vault", map[string]any{
            "addr":  "http://localhost:8200",
            "token": "test-token",
        }).
        WithSecretStore("aws", "aws.secretsmanager", map[string]any{
            "region": "us-east-1",
        }).
        WithEnv("production", map[string]config.Variable{
            "DATABASE_URL": {
                From: "store://vault/database/url",
            },
            "API_KEY": {
                From: "store://aws/api/key",
            },
        })
    defer builder.Cleanup()

    cfg := builder.Build()

    // Verify config structure
    assert.Contains(t, cfg.SecretStores, "vault")
    assert.Contains(t, cfg.SecretStores, "aws")
    assert.Contains(t, cfg.Envs, "production")
    assert.Len(t, cfg.Envs["production"], 2)

    // Use config in tests
    assert.Equal(t, "vault", cfg.SecretStores["vault"].Type)
    assert.Equal(t, "aws.secretsmanager", cfg.SecretStores["aws"].Type)
}
```

---

### Config Validation Tests

**Use case**: Testing configuration validation rules

**Pattern**:
```go
func TestConfigValidation(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name       string
        configYAML string
        wantErr    bool
        errMsg     string
    }{
        {
            name: "valid_config",
            configYAML: `
version: 1
secretStores:
  vault:
    type: vault
    addr: http://localhost:8200
envs:
  test:
    VAR: {from: "store://vault/secret"}
`,
            wantErr: false,
        },
        {
            name: "missing_version",
            configYAML: `
secretStores:
  vault:
    type: vault
`,
            wantErr: true,
            errMsg:  "version required",
        },
        {
            name: "unknown_provider_type",
            configYAML: `
version: 1
secretStores:
  test:
    type: nonexistent
`,
            wantErr: true,
            errMsg:  "unknown provider type",
        },
        {
            name: "invalid_reference",
            configYAML: `
version: 1
secretStores:
  vault:
    type: vault
envs:
  test:
    VAR: {from: "invalid-reference"}
`,
            wantErr: true,
            errMsg:  "invalid reference format",
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            configPath := testutil.WriteTestConfig(t, tt.configYAML)
            cfg, err := config.Load(configPath)

            if tt.wantErr {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errMsg)
                return
            }

            assert.NoError(t, err)
            assert.NotNil(t, cfg)
        })
    }
}
```

---

## CLI Command Test Patterns

### Command Execution Tests

**Use case**: Testing CLI commands end-to-end

**Pattern**:
```go
func TestPlanCommand(t *testing.T) {
    t.Parallel()

    tests := []struct {
        name           string
        configYAML     string
        args           []string
        wantErr        bool
        stdoutContains []string
        stderrContains []string
    }{
        {
            name: "simple_plan",
            configYAML: `
version: 1
secretStores:
  test:
    type: literal
    values:
      DATABASE_URL: "postgres://localhost/test"
envs:
  test:
    DATABASE_URL: {from: "store://test/DATABASE_URL"}
`,
            args:           []string{"plan", "--env", "test"},
            wantErr:        false,
            stdoutContains: []string{"DATABASE_URL", "postgres://localhost/test"},
        },
        {
            name: "missing_env",
            configYAML: `
version: 1
secretStores:
  test:
    type: literal
    values:
      KEY: "value"
envs:
  production:
    KEY: {from: "store://test/KEY"}
`,
            args:           []string{"plan", "--env", "nonexistent"},
            wantErr:        true,
            stderrContains: []string{"environment not found"},
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            // Write config file
            configPath := testutil.WriteTestConfig(t, tt.configYAML)

            // Build command
            args := append([]string{"--config", configPath}, tt.args...)
            cmd := exec.Command("dsops", args...)

            var stdout, stderr bytes.Buffer
            cmd.Stdout = &stdout
            cmd.Stderr = &stderr

            // Execute
            err := cmd.Run()

            // Validate exit code
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }

            // Validate stdout
            stdoutStr := stdout.String()
            for _, expected := range tt.stdoutContains {
                assert.Contains(t, stdoutStr, expected)
            }

            // Validate stderr
            stderrStr := stderr.String()
            for _, expected := range tt.stderrContains {
                assert.Contains(t, stderrStr, expected)
            }
        })
    }
}
```

---

### Output Validation Tests

**Use case**: Validating command output format

**Pattern**:
```go
func TestRenderCommand_OutputFormats(t *testing.T) {
    t.Parallel()

    configYAML := `
version: 1
secretStores:
  test:
    type: literal
    values:
      DATABASE_URL: "postgres://localhost/db"
      API_KEY: "test-api-key-123"
envs:
  test:
    DATABASE_URL: {from: "store://test/DATABASE_URL"}
    API_KEY: {from: "store://test/API_KEY"}
`

    tests := []struct {
        name       string
        format     string
        validator  func(*testing.T, string)
    }{
        {
            name:   "dotenv_format",
            format: "dotenv",
            validator: func(t *testing.T, output string) {
                assert.Contains(t, output, "DATABASE_URL=postgres://localhost/db")
                assert.Contains(t, output, "API_KEY=test-api-key-123")
                // Verify format
                lines := strings.Split(output, "\n")
                for _, line := range lines {
                    if line == "" {
                        continue
                    }
                    assert.Regexp(t, `^[A-Z_]+=.+$`, line)
                }
            },
        },
        {
            name:   "json_format",
            format: "json",
            validator: func(t *testing.T, output string) {
                var data map[string]string
                err := json.Unmarshal([]byte(output), &data)
                require.NoError(t, err)

                assert.Equal(t, "postgres://localhost/db", data["DATABASE_URL"])
                assert.Equal(t, "test-api-key-123", data["API_KEY"])
            },
        },
        {
            name:   "yaml_format",
            format: "yaml",
            validator: func(t *testing.T, output string) {
                var data map[string]string
                err := yaml.Unmarshal([]byte(output), &data)
                require.NoError(t, err)

                assert.Equal(t, "postgres://localhost/db", data["DATABASE_URL"])
                assert.Equal(t, "test-api-key-123", data["API_KEY"])
            },
        },
    }

    for _, tt := range tests {
        tt := tt
        t.Run(tt.name, func(t *testing.T) {
            t.Parallel()

            configPath := testutil.WriteTestConfig(t, configYAML)

            cmd := exec.Command("dsops", "render",
                "--config", configPath,
                "--env", "test",
                "--format", tt.format,
            )

            output, err := cmd.Output()
            require.NoError(t, err)

            tt.validator(t, string(output))
        })
    }
}
```

---

## Summary

These patterns cover the most common testing scenarios in dsops:

1. **Unit tests**: Table-driven, subtests, error handling
2. **Provider tests**: Fakes, contracts, error conditions
3. **Integration tests**: Docker, multi-service workflows
4. **Security tests**: Redaction, concurrency
5. **Configuration tests**: Building, validation
6. **CLI tests**: Execution, output validation

**Best Practices**:
- Use table-driven tests for multiple cases
- Start with unit tests (fast), add integration tests later
- Always test error paths
- Use `t.Parallel()` for independent tests
- Check `testing.Short()` for slow tests
- Validate both behavior and error messages

---

**Further Reading**:
- [TDD Workflow](./tdd-workflow.md) - Red-Green-Refactor cycle
- [Testing Strategy](./testing.md) - Test categories and coverage
- [Test Infrastructure](../../tests/README.md) - Test utilities guide
- [Quick Start](../../specs/005-testing-strategy/quickstart.md) - Quick reference

**Questions?** See [SPEC-005](/specs/005-testing-strategy/spec.md) or GitHub Discussions.
