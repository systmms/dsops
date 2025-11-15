# SPEC-005: Testing Strategy & Plan

**Status**: In Progress
**Feature Branch**: feature/005-testing-strategy
**Target Milestone**: v0.2
**Related**:
- Constitution Principle VII (Test-Driven Development)
- docs/content/reference/status.md (Project Roadmap - v0.2 Testing Goal)

## Summary

Comprehensive testing strategy to achieve 80% test coverage across the dsops codebase while establishing TDD (Test-Driven Development) practices for future development. This spec defines the testing infrastructure, coverage goals, testing categories, and migration path from the current 20% average coverage to production-ready quality standards.

**Current State**: Mixed coverage across packages (0-94%, ~20% average)
**Target State**: 80% minimum coverage with robust CI/CD gates
**Approach**: Phased implementation prioritizing critical paths, with test infrastructure for provider integration testing

## User Stories

### User Story 1: Achieve 80% Test Coverage (P0)

**As a** maintainer, **I want** comprehensive test coverage across all critical code paths, **so that** we can confidently release dsops to production users without fear of regressions.

**Why this priority**: Production readiness depends on test quality. Without adequate coverage, bugs slip through and user trust erodes.

**Acceptance Criteria**:
1. **Given** the codebase is built, **When** tests run with coverage enabled, **Then** overall coverage is ≥80%
2. **Given** any package contains business logic, **When** coverage is measured, **Then** package coverage is ≥70% (no packages <50%)
3. **Given** critical packages (providers, resolution, config), **When** coverage is measured, **Then** package coverage is ≥85%
4. **Given** CI runs on pull requests, **When** coverage drops below threshold, **Then** CI fails with clear error message

**Current Baseline** (from investigation):
```
High Coverage (≥80%):
✅ internal/secretstores: 94.1%
✅ internal/validation: 90.0%
✅ pkg/adapter: 83.0%
✅ internal/logging: 80.8%

Medium Coverage (50-79%):
🟡 internal/permissions: 66.1%
🟡 internal/services: 53.1% (HAS FAILING TESTS - priority fix)
🟡 internal/config: 44.9%

Low/No Coverage (<50%):
❌ cmd/dsops/commands: 7.9%
❌ internal/providers: 0.8%
❌ Zero coverage (0%):
   - internal/dsopsdata
   - internal/execenv
   - internal/errors
   - internal/policy
   - internal/incident
   - internal/vault
   - internal/resolve
   - internal/rotation/storage
   - internal/rotation
   - internal/template
```

### User Story 2: Establish TDD Workflow (P1)

**As a** developer, **I want** clear TDD practices and infrastructure, **so that** I write tests before code and avoid technical debt accumulation.

**Why this priority**: Constitution Principle VII mandates TDD. Without established patterns, developers won't adopt it consistently.

**Acceptance Criteria**:
1. **Given** a new feature is planned, **When** implementation begins, **Then** tests are written first (Red-Green-Refactor)
2. **Given** a pull request is submitted, **When** CI runs, **Then** new code has ≥80% test coverage
3. **Given** developer needs to write tests, **When** they reference examples, **Then** test patterns exist for all common scenarios
4. **Given** TDD is followed, **When** code changes, **Then** refactoring is safe due to comprehensive test suite

**TDD Workflow**:
```
1. RED:   Write failing test for new functionality
2. GREEN: Implement minimal code to pass test
3. REFACTOR: Improve code while tests remain green
4. REPEAT: For each acceptance criterion
```

### User Story 3: Integration Test Infrastructure (P1)

**As a** developer, **I want** Docker-based integration test environments, **so that** I can test providers without requiring real credentials or cloud accounts.

**Why this priority**: Provider tests are currently 0.8% covered because no test infrastructure exists. Can't reach 80% without solving this.

**Acceptance Criteria**:
1. **Given** developer runs `make test-integration`, **When** tests execute, **Then** Docker containers start for Vault, PostgreSQL, LocalStack (AWS emulation)
2. **Given** integration tests run, **When** provider needs authentication, **Then** test credentials are auto-configured (no manual setup)
3. **Given** provider contract tests exist, **When** new provider is added, **Then** contract tests validate interface compliance automatically
4. **Given** CI runs integration tests, **When** containers fail to start, **Then** tests are skipped gracefully (not failed)

**Infrastructure Requirements**:
- Docker Compose configuration for test services
- Vault (HashiCorp): Test secret storage
- PostgreSQL: Test database rotation
- LocalStack: AWS service emulation (Secrets Manager, SSM)
- MongoDB: Test NoSQL rotation
- Test data fixtures in `testdata/` directories

### User Story 4: Security Test Validation (P2)

**As a** security engineer, **I want** automated tests for secret redaction and leak prevention, **so that** users can trust dsops with sensitive data.

**Why this priority**: Security is a core value proposition. Without tests, redaction bugs could leak secrets.

**Acceptance Criteria**:
1. **Given** secret value is logged, **When** logs are captured, **Then** secret is always `[REDACTED]`
2. **Given** error message contains secret path, **When** error is formatted, **Then** path components are sanitized
3. **Given** command crashes, **When** stack trace is generated, **Then** no secret values appear in trace
4. **Given** race detector runs (`-race`), **When** tests execute concurrently, **Then** no race conditions detected

**Test Categories**:
- Redaction validation (logging.Secret() wrapper)
- Error sanitization (paths, references)
- Memory safety (no secret persistence after GC)
- Concurrent access safety (race detector)

### User Story 5: CI/CD Coverage Gates (P2)

**As a** project maintainer, **I want** automated coverage enforcement in CI, **so that** code quality never regresses.

**Why this priority**: Manual coverage checks are forgotten. Automation ensures consistency.

**Acceptance Criteria**:
1. **Given** PR is opened, **When** CI runs, **Then** coverage report is generated and posted as comment
2. **Given** PR decreases coverage, **When** CI evaluates, **Then** PR is blocked with clear instructions
3. **Given** coverage goal is unmet, **When** PR is merged, **Then** coverage debt is tracked in issue
4. **Given** release is cut, **When** version is tagged, **Then** coverage report is included in release notes

**CI Integration**:
- GitHub Actions workflow: `.github/workflows/test.yml`
- Coverage reporting: codecov.io or coveralls.io
- PR comments: Automated coverage diff reporting
- Badge: Coverage badge in README.md

## Implementation

### Phase 1: Critical Path Coverage (Target: 60% overall) - 4 weeks

**Priority Packages** (current → target):
1. **internal/providers** (0.8% → 85%)
   - Provider contract tests (interface compliance)
   - Mock provider for testing
   - Per-provider unit tests (Bitwarden, 1Password, AWS, etc.)
   - Integration tests with Docker (Vault, LocalStack)

2. **internal/resolve** (0% → 85%)
   - Dependency graph resolution tests
   - Transform pipeline tests (json_extract, base64_decode, etc.)
   - Error aggregation tests
   - Circular dependency detection tests

3. **internal/config** (44.9% → 80%)
   - YAML parsing tests (v0 and v1 formats)
   - Validation tests (missing providers, invalid refs)
   - Migration tests (v0 → v1 format conversion)
   - Schema validation tests

4. **internal/rotation** (0% → 75%)
   - Strategy tests (immediate, two-key, overlap)
   - Verification tests (connection validation)
   - Grace period tests
   - Rotation state machine tests

**Deliverables**:
- Provider fake/mock implementations
- Docker Compose for test infrastructure
- Test data fixtures
- 60% overall coverage

### Phase 2: Command & Integration Coverage (Target: 70% overall) - 3 weeks

**Priority Packages** (current → target):
1. **cmd/dsops/commands** (7.9% → 70%)
   - Command execution tests (plan, exec, render, get)
   - Flag parsing tests
   - Error handling tests
   - Exit code validation tests

2. **internal/execenv** (0% → 80%)
   - Process execution tests
   - Environment injection tests
   - Exit code propagation tests
   - Signal handling tests

3. **internal/template** (0% → 80%)
   - Template rendering tests (dotenv, JSON, YAML, Go templates)
   - Format detection tests
   - Error handling tests

4. **Integration Tests** (0% → 60% of use cases)
   - End-to-end workflow tests (init → plan → exec)
   - Multi-provider tests (AWS + Bitwarden + Vault)
   - Rotation workflow tests
   - CI/CD scenario tests

**Deliverables**:
- Command integration test suite
- Process execution tests
- Template rendering tests
- 70% overall coverage

### Phase 3: Full Coverage & Edge Cases (Target: 80% overall) - 3 weeks

**Priority Packages** (current → target):
1. **Remaining 0% packages** (→ 70% each):
   - internal/dsopsdata (data loading, validation)
   - internal/errors (error formatting, suggestions)
   - internal/policy (guardrail enforcement)
   - internal/incident (leak reporting)
   - internal/vault (Vault-specific logic)
   - internal/rotation/storage (rotation state persistence)

2. **Security Tests** (comprehensive):
   - Redaction validation across all packages
   - Race condition tests (`go test -race ./...`)
   - Memory leak tests
   - Concurrent access tests

3. **Edge Case Coverage**:
   - Error path coverage (failure modes)
   - Boundary condition tests
   - Invalid input handling
   - Provider authentication failure scenarios

**Deliverables**:
- Complete test suite for all packages
- Security test harness
- Edge case test matrix
- 80% overall coverage

### Architecture: Test Infrastructure

**Directory Structure**:
```
tests/
├── integration/          # Integration tests
│   ├── docker-compose.yml
│   ├── providers/       # Provider integration tests
│   ├── rotation/        # Rotation workflow tests
│   └── e2e/             # End-to-end scenarios
├── fixtures/            # Test data and fixtures
│   ├── configs/         # Test dsops.yaml files
│   ├── secrets/         # Mock secret data
│   └── services/        # Service definitions
├── mocks/               # Generated mocks (mockgen)
├── fakes/               # Manual fakes (provider.Provider)
└── testutil/            # Test helpers and utilities
```

**Test Utilities**:
- `testutil.NewTestConfig()` - Create test configuration
- `testutil.NewMockProvider()` - Create mock provider
- `testutil.NewTestLogger()` - Create test logger (buffer output)
- `testutil.SetupTestEnv()` - Setup test environment variables
- `testutil.CleanupTestFiles()` - Cleanup test artifacts

**Docker Test Infrastructure**:
```yaml
# tests/integration/docker-compose.yml
services:
  vault:
    image: hashicorp/vault:latest
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: test-token

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_PASSWORD: test-password

  localstack:
    image: localstack/localstack:latest
    environment:
      SERVICES: secretsmanager,ssm
```

### Design Decisions

**Testing Framework**: Standard `go test` with table-driven tests
- **Rationale**: No external dependencies, native Go tooling, excellent IDE support
- **Alternative considered**: Ginkgo/Gomega (rejected: adds complexity)

**Mocking Strategy**: Combination of manual fakes and generated mocks
- **Provider fakes**: Manual implementations for realistic behavior
- **Interface mocks**: mockgen for internal interfaces
- **Rationale**: Fakes for complex behavior, mocks for simple interfaces

**Integration Tests**: Docker-based ephemeral environments
- **Rationale**: Reproducible, no cloud credentials needed, fast parallel execution
- **Alternative considered**: Testcontainers (rejected: Go support immature)

**Coverage Tool**: Built-in `go test -cover` with codecov.io reporting
- **Rationale**: Native tooling, free for open source, GitHub integration
- **Alternative considered**: Coveralls (rejected: codecov has better UX)

**CI Platform**: GitHub Actions
- **Rationale**: Native GitHub integration, free for open source, excellent Docker support
- **Alternative considered**: CircleCI (rejected: GitHub Actions is simpler)

### Testing Categories

**1. Unit Tests** (Pure Logic)
- **Scope**: Functions with no external dependencies
- **Pattern**: Table-driven tests
- **Example**:
```go
func TestTransformJSONExtract(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        path     string
        expected string
        wantErr  bool
    }{
        {"simple path", `{"key":"value"}`, "key", "value", false},
        {"nested path", `{"a":{"b":"c"}}`, "a.b", "c", false},
        {"invalid JSON", "invalid", "key", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

**2. Provider Contract Tests** (Interface Compliance)
- **Scope**: Validate all providers implement provider.Provider correctly
- **Pattern**: Generic test suite run against each provider
- **Example**:
```go
func TestProviderContract(t *testing.T) {
    providers := []struct {
        name     string
        provider provider.Provider
    }{
        {"bitwarden", providers.NewBitwardenProvider(cfg)},
        {"onepassword", providers.NewOnePasswordProvider(cfg)},
        // ... all providers
    }
    for _, tt := range providers {
        t.Run(tt.name, func(t *testing.T) {
            testProviderResolve(t, tt.provider)
            testProviderDescribe(t, tt.provider)
            testProviderValidate(t, tt.provider)
        })
    }
}
```

**3. Integration Tests** (Real Provider CLIs)
- **Scope**: Test with actual provider implementations (Docker-based)
- **Pattern**: Docker Compose + test fixtures
- **Example**: Vault integration test
```go
func TestVaultIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }

    // Start Vault via Docker
    vault := testutil.StartVault(t)
    defer vault.Stop()

    // Write test secret
    vault.Write("secret/test", map[string]string{"key": "value"})

    // Test provider
    provider := providers.NewVaultProvider(vault.Config())
    result, err := provider.Resolve(ctx, provider.Reference{Key: "secret/test"})
    assert.NoError(t, err)
    assert.Equal(t, "value", result.Value)
}
```

**4. Security Tests** (Redaction & Safety)
- **Scope**: Validate no secret leakage
- **Pattern**: Log capture + assertion
- **Example**:
```go
func TestSecretRedaction(t *testing.T) {
    buf := &bytes.Buffer{}
    logger := logging.NewLogger(buf)

    secret := logging.Secret("super-secret-value")
    logger.Info("Retrieved secret: %s", secret)

    output := buf.String()
    assert.Contains(t, output, "[REDACTED]")
    assert.NotContains(t, output, "super-secret-value")
}
```

**5. End-to-End Tests** (Complete Workflows)
- **Scope**: Full user scenarios (init → plan → exec)
- **Pattern**: Binary execution + output validation
- **Example**:
```go
func TestE2EExecWorkflow(t *testing.T) {
    // Setup test config
    configFile := testutil.WriteTestConfig(t, testConfigYAML)

    // Run dsops exec
    cmd := exec.Command("dsops", "exec", "--config", configFile,
                       "--env", "test", "--", "env")
    output, err := cmd.CombinedOutput()

    assert.NoError(t, err)
    assert.Contains(t, string(output), "DATABASE_URL=")
    assert.NotContains(t, string(output), "[REDACTED]") // Secret should be in env
}
```

## Security Considerations

**Test Data Security**:
- **No real secrets**: All test data uses fake/mock values
- **Fixture isolation**: Test fixtures in dedicated `testdata/` directories
- **Cleanup enforcement**: All tests must cleanup temporary files
- **CI secret handling**: Test credentials are ephemeral (Docker containers destroyed after tests)

**Race Condition Detection**:
- All tests run with `-race` flag in CI
- Concurrent provider access tests
- Rotation state machine concurrency tests

**Memory Safety**:
- Secret values not retained after GC
- Buffer overflow tests for transforms
- Nil pointer dereference tests

## Testing

**Coverage Measurement**:
```bash
# Unit tests with coverage
make test-coverage

# Integration tests (requires Docker)
make test-integration

# All tests with race detection
make test-all

# Coverage report (HTML)
make coverage-report
```

**CI/CD Pipeline**:
```yaml
# .github/workflows/test.yml
name: Test
on: [pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - name: Unit Tests
        run: go test -race -coverprofile=coverage.txt ./...
      - name: Integration Tests
        run: make test-integration
      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.txt
      - name: Coverage Gate
        run: |
          COVERAGE=$(go tool cover -func=coverage.txt | grep total | awk '{print $3}' | sed 's/%//')
          if (( $(echo "$COVERAGE < 80" | bc -l) )); then
            echo "Coverage $COVERAGE% is below 80% threshold"
            exit 1
          fi
```

**Success Metrics**:
- **Coverage**: ≥80% overall, ≥70% per package
- **CI Green**: All tests pass on every PR
- **Test Execution Time**: <5 minutes for unit tests, <10 minutes for integration tests
- **Flakiness**: <1% flaky test rate (retry failed tests 3x to detect flakiness)

## Documentation

**Developer Guide**:
- `docs/developer/testing.md` - Testing strategy and guidelines
- `docs/developer/tdd-workflow.md` - TDD process and examples
- `tests/README.md` - Test infrastructure setup

**Test Documentation**:
- Each test file includes package-level comment explaining test scope
- Complex tests include inline comments for non-obvious assertions
- Integration tests document Docker dependencies

**Coverage Reports**:
- CI generates HTML coverage report as artifact
- Coverage badge in README.md (via codecov.io)
- Release notes include coverage metrics

## Milestones & Timeline

**Phase 1: Critical Path (Weeks 1-4)**
- Week 1: Provider contract tests + mocks/fakes
- Week 2: Provider integration tests (Docker setup)
- Week 3: Resolution engine tests
- Week 4: Config parsing & rotation tests
- **Checkpoint**: 60% coverage, integration test infrastructure ready

**Phase 2: Commands & Integration (Weeks 5-7)**
- Week 5: Command tests (plan, exec, render, get)
- Week 6: Execution environment & template tests
- Week 7: End-to-end workflow tests
- **Checkpoint**: 70% coverage, full integration test suite

**Phase 3: Full Coverage (Weeks 8-10)**
- Week 8: Remaining 0% packages
- Week 9: Security tests + race detection
- Week 10: Edge cases + documentation
- **Checkpoint**: 80% coverage, production-ready test suite

**Total Timeline**: 10 weeks (2.5 months)

## Success Criteria

**Definition of Done**:
1. ✅ Overall test coverage ≥80%
2. ✅ All packages ≥70% coverage (no packages <50%)
3. ✅ Critical packages (providers, resolve, config, rotation) ≥85%
4. ✅ Integration test infrastructure complete (Docker Compose)
5. ✅ CI/CD coverage gates enforced
6. ✅ TDD documentation published
7. ✅ Zero failing tests in main branch
8. ✅ Race detector passes (`go test -race ./...`)
9. ✅ Test execution time <10 minutes (full suite)
10. ✅ Coverage report in README and release notes

## Future Enhancements (v0.3+)

1. **Property-Based Testing**: Use `gopter` for fuzz testing transforms and resolution logic
2. **Mutation Testing**: Use `go-mutesting` to validate test quality
3. **Performance Tests**: Benchmark suite for resolution performance
4. **Contract Testing**: Consumer-driven contract tests for provider APIs
5. **Chaos Testing**: Failure injection for rotation resilience testing
6. **Visual Regression Tests**: For CLI output formatting
7. **Load Testing**: Concurrent resolution performance tests
8. **Compliance Tests**: Automated compliance validation (PCI-DSS, SOC2)

## Related Specifications

- **SPEC-001**: CLI Framework (command testing patterns)
- **SPEC-002**: Configuration Parsing (config validation tests)
- **SPEC-003**: Secret Resolution Engine (resolution testing)
- **SPEC-050**: Rotation Phase 5 Completion (rotation testing requirements)
- **SPEC-080-089**: Provider Specifications (provider contract tests)

## References

- Constitution Principle VII: Test-Driven Development
- Go Testing Best Practices: https://go.dev/doc/tutorial/add-a-test
- Table-Driven Tests: https://github.com/golang/go/wiki/TableDrivenTests
- Testcontainers (future): https://golang.testcontainers.org/
