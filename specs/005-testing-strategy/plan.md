# Implementation Plan: Testing Strategy & Infrastructure

**Branch**: `005-testing-strategy` | **Date**: 2025-11-14 | **Spec**: [SPEC-005](./spec.md)
**Input**: Feature specification from `/specs/005-testing-strategy/spec.md`

## Summary

Establish comprehensive testing infrastructure to achieve 80% test coverage across dsops codebase. Implementation includes Docker-based integration test environments, provider contract tests, security validation tests, and CI/CD coverage gates. Approach follows TDD principles with phased rollout prioritizing critical packages first (providers, resolution, config, rotation).

**Key Deliverables**:
- Docker Compose test infrastructure (Vault, PostgreSQL, LocalStack)
- Provider contract test framework
- Integration test suite with automated setup
- CI/CD coverage enforcement (≥80% gate)
- Test utilities and fixtures library
- TDD workflow documentation

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**:
- Standard `go test` framework (no external test frameworks)
- Docker & Docker Compose (test infrastructure)
- HashiCorp Vault (secret store testing)
- LocalStack (AWS service emulation)
- PostgreSQL 15 (database rotation testing)
- mockgen (interface mocking)

**Storage**: Test fixtures in `testdata/` directories, ephemeral Docker volumes
**Testing**: Native Go testing with table-driven patterns, Docker-based integration tests
**Target Platform**: Cross-platform (macOS, Linux, Windows) - CI runs on Linux
**Project Type**: CLI tool with provider-based architecture
**Performance Goals**:
- Unit tests: <5 minutes total execution
- Integration tests: <10 minutes with Docker startup
- Parallel test execution where possible
- CI pipeline: <15 minutes end-to-end

**Constraints**:
- No real cloud credentials in tests (Docker emulation only)
- All test secrets must be fake/mock values
- Tests must cleanup temporary files
- Integration tests gracefully skip if Docker unavailable
- Race detector must pass (`go test -race`)

**Scale/Scope**:
- 20+ packages to test
- Target: 80% overall coverage (current: ~20%)
- ~15 providers to contract-test
- 3-phase implementation (10 weeks)
- Critical packages: providers, resolve, config, rotation (85% target)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### ✅ Principle VII: Test-Driven Development (DIRECTLY ADDRESSES)
**Status**: COMPLIANT - This spec implements the TDD mandate
- Establishes TDD workflow and infrastructure
- Requires tests-first approach for all new code
- Provides test patterns and examples for developers
- Enforces 80% coverage via CI/CD gates

**Implementation Notes**:
- Red-Green-Refactor cycle documented
- Test patterns provided for all testing categories
- CI enforces coverage on every PR
- TDD developer guide will be created

### ✅ Principle II: Security by Default
**Status**: COMPLIANT - Security tests included
- Validates `logging.Secret()` redaction across all packages
- Tests prevent secret leakage in logs and errors
- Race detection tests for concurrent access safety
- Memory safety tests for secret cleanup after GC

**Implementation Notes**:
- Security test category defined in spec
- Redaction tests for all secret-handling code
- CI runs with `-race` flag automatically
- Test fixtures use only fake/mock secrets

### ✅ Principle III: Provider-Agnostic Interfaces
**Status**: COMPLIANT - Provider contract tests ensure interface compliance
- Generic contract test suite validates all providers
- Tests enforce `provider.Provider` interface compliance
- New providers automatically validated by contract tests
- Prevents interface drift and breaking changes

**Implementation Notes**:
- Provider contract test framework (Phase 1)
- Each provider tested against same contract
- Validates Resolve(), Describe(), Validate(), Capabilities()

### ✅ Principle V: Developer Experience First
**Status**: COMPLIANT - Developer-friendly testing infrastructure
- Docker-based tests require no manual setup
- Test utilities abstract common patterns
- Clear test patterns and examples
- Fast feedback (tests complete in <10 minutes)

**Implementation Notes**:
- `make test`, `make test-integration`, `make test-coverage` commands
- Docker Compose auto-configures test services
- Test fixtures and utilities in `tests/testutil/`
- TDD workflow guide for developers

### ✅ Principle VIII: Explicit Over Implicit
**STATUS**: COMPLIANT - Explicit test categorization and execution
- Tests categorized by type (unit, integration, security, e2e)
- Integration tests explicitly skipped if Docker unavailable
- Coverage gates explicitly fail PR if threshold unmet
- Short flag honors (`go test -short` skips integration tests)

**No Constitutional Violations**: All principles aligned

## Project Structure

### Documentation (this feature)

```text
specs/005-testing-strategy/
├── spec.md              # Feature specification (existing)
├── plan.md              # This file (/speckit.plan output)
├── research.md          # Phase 0: Test framework decisions (TO BE CREATED)
├── data-model.md        # Phase 1: Test infrastructure models (TO BE CREATED)
├── contracts/           # Phase 1: Test utilities API (TO BE CREATED)
│   ├── testutil.md      # Test helper function contracts
│   └── providers.md     # Provider contract test spec
└── tasks.md             # Phase 2: Task breakdown (/speckit.tasks)
```

### Source Code (repository root)

```text
# Testing Infrastructure (NEW)
tests/
├── integration/              # Integration tests
│   ├── docker-compose.yml    # Test service definitions
│   ├── providers/            # Provider integration tests
│   │   ├── vault_test.go
│   │   ├── aws_test.go
│   │   └── postgres_test.go
│   ├── rotation/             # Rotation workflow tests
│   │   ├── immediate_test.go
│   │   ├── twokey_test.go
│   │   └── overlap_test.go
│   └── e2e/                  # End-to-end scenario tests
│       ├── exec_workflow_test.go
│       └── rotation_workflow_test.go
├── fixtures/                 # Test data and fixtures
│   ├── configs/              # Test dsops.yaml files
│   │   ├── simple.yaml
│   │   ├── multi-provider.yaml
│   │   └── rotation.yaml
│   ├── secrets/              # Mock secret data (JSON)
│   └── services/             # Service definitions
├── mocks/                    # Generated mocks (mockgen)
│   ├── provider_mock.go
│   └── service_mock.go
├── fakes/                    # Manual fakes
│   ├── provider_fake.go      # Fake provider.Provider
│   └── secretstore_fake.go
└── testutil/                 # Test helpers and utilities
    ├── config.go             # Config helpers
    ├── logger.go             # Logger helpers
    ├── provider.go           # Provider helpers
    └── docker.go             # Docker helpers

# Existing Code (with new test files)
internal/
├── providers/
│   ├── bitwarden.go
│   ├── bitwarden_test.go     # NEW: Provider unit tests
│   ├── contract_test.go      # NEW: Provider contract tests
│   └── ...
├── resolve/
│   ├── resolver.go
│   ├── resolver_test.go      # NEW: Resolution tests
│   ├── transforms.go
│   └── transforms_test.go    # NEW: Transform tests
├── config/
│   ├── config.go
│   └── config_test.go        # EXPAND: More coverage
├── rotation/
│   ├── engine.go
│   ├── engine_test.go        # NEW: Rotation tests
│   └── ...
└── ...

# CI/CD (NEW)
.github/
└── workflows/
    ├── test.yml              # NEW: Test workflow
    ├── coverage.yml          # NEW: Coverage reporting
    └── integration.yml       # NEW: Integration test workflow

# Makefile additions
Makefile                      # EXPAND: Add test targets
```

**Structure Decision**: Single project with dedicated `tests/` directory for integration tests and shared test utilities. Unit tests co-located with source code following Go conventions (`*_test.go`). This approach:
- Keeps unit tests close to implementation (Go best practice)
- Centralizes integration tests and test infrastructure
- Provides shared test utilities via `tests/testutil/` package
- Separates test fixtures from implementation code

## Complexity Tracking

> **No constitutional violations**

| Aspect | Justification |
|--------|---------------|
| Docker dependency | Required for provider integration tests without real cloud credentials |
| Test infrastructure | Necessary to achieve 80% coverage goal and test provider integrations |
| Phased approach | Manages complexity by prioritizing critical packages first |

## Phase 0: Research & Design Decisions

### Research Questions

1. **Test Framework Choice**:
   - Should we use standard `go test` or external framework (Ginkgo, Testify)?
   - **Decision Needed**: Evaluate Go native testing vs. alternatives
   - **Output**: research.md with framework comparison

2. **Mocking Strategy**:
   - Manual fakes vs. generated mocks (mockgen, gomock)?
   - When to use each approach?
   - **Decision Needed**: Define mocking strategy per component type
   - **Output**: research.md with mocking guidelines

3. **Integration Test Infrastructure**:
   - Docker Compose vs. Testcontainers-go?
   - Which services needed (Vault, LocalStack, PostgreSQL, etc.)?
   - **Decision Needed**: Integration test architecture
   - **Output**: research.md with infrastructure design

4. **Coverage Tooling**:
   - codecov.io vs. coveralls.io vs. self-hosted?
   - How to enforce coverage gates in CI?
   - **Decision Needed**: Coverage reporting and enforcement approach
   - **Output**: research.md with tooling selection

5. **Test Execution Strategy**:
   - Parallel test execution approach?
   - How to handle Docker container lifecycle?
   - Short flag usage for skipping integration tests?
   - **Decision Needed**: Test execution patterns
   - **Output**: research.md with execution guidelines

### Research Tasks

**Task 1**: Evaluate Go testing frameworks
- Compare `go test`, `testify/suite`, `ginkgo/gomega`
- Assess: simplicity, IDE support, community adoption
- Recommendation: Framework choice with rationale

**Task 2**: Design mocking strategy
- When to use manual fakes vs. generated mocks
- Evaluate mockgen, gomock, testify/mock
- Define patterns for provider mocking
- Recommendation: Mocking approach per component type

**Task 3**: Design integration test infrastructure
- Docker Compose service definitions
- Container lifecycle management (setup/teardown)
- Test data seeding approach
- Recommendation: Infrastructure architecture

**Task 4**: Select coverage tooling
- Evaluate codecov.io, coveralls.io, self-hosted
- CI integration approach
- PR comment automation
- Recommendation: Coverage tool with CI integration plan

**Task 5**: Define test execution patterns
- Parallel execution where safe
- Integration test skipping (`-short` flag)
- Race detection enforcement
- Recommendation: Test execution guidelines

**Output**: `research.md` with decisions, rationale, and alternatives considered

## Phase 1: Design & Contracts

### Phase 1.1: Data Models

**File**: `data-model.md`

**Test Infrastructure Models**:

1. **TestProvider** (Fake provider.Provider):
   - Fields: `name`, `secrets` (map), `metadata` (map), `shouldFail` (bool)
   - Methods: Implement full `provider.Provider` interface
   - Purpose: Predictable provider for unit tests

2. **TestConfig** (Test configuration builder):
   - Fields: `providers`, `services`, `envs`, `tempDir`
   - Methods: `AddProvider()`, `AddEnv()`, `Write()`, `Cleanup()`
   - Purpose: Programmatically build test configurations

3. **DockerTestEnv** (Integration test environment):
   - Fields: `compose`, `services`, `cleanup` (function)
   - Methods: `Start()`, `Stop()`, `VaultClient()`, `PostgresClient()`
   - Purpose: Manage Docker test infrastructure

4. **TestLogger** (Log capture):
   - Fields: `buffer`, `level`
   - Methods: `Capture()`, `AssertContains()`, `AssertNotContains()`
   - Purpose: Validate logging and redaction

### Phase 1.2: API Contracts

**Directory**: `contracts/`

**File**: `contracts/testutil.md`

Test utility API specification:

```go
package testutil

// Configuration helpers
func NewTestConfig(t *testing.T) *TestConfigBuilder
func WriteTestConfig(t *testing.T, yaml string) string
func LoadTestConfig(t *testing.T, path string) *config.Config

// Provider helpers
func NewMockProvider(name string) *TestProvider
func NewFakeProvider(name string, secrets map[string]string) *TestProvider

// Logger helpers
func NewTestLogger(t *testing.T) *TestLogger
func CaptureLog(fn func()) string

// Environment helpers
func SetupTestEnv(t *testing.T, vars map[string]string)
func CleanupTestFiles(t *testing.T)

// Docker helpers (integration tests only)
func StartDockerEnv(t *testing.T, services []string) *DockerTestEnv
func SkipIfDockerUnavailable(t *testing.T)
```

**File**: `contracts/providers.md`

Provider contract test specification:

```go
package providers_test

// Contract test suite - all providers must pass
func TestProviderContract(t *testing.T) {
    providers := GetAllProviders(t)
    for name, provider := range providers {
        t.Run(name, func(t *testing.T) {
            testProviderResolve(t, provider)
            testProviderDescribe(t, provider)
            testProviderValidate(t, provider)
            testProviderCapabilities(t, provider)
        })
    }
}

// Individual contract tests
func testProviderResolve(t *testing.T, p provider.Provider)
func testProviderDescribe(t *testing.T, p provider.Provider)
func testProviderValidate(t *testing.T, p provider.Provider)
func testProviderCapabilities(t *testing.T, p provider.Provider)
```

### Phase 1.3: Integration Scenarios

**File**: `quickstart.md`

Quick start guide for writing tests:

**Unit Test Example**:
```go
func TestTransformJSONExtract(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        path     string
        expected string
        wantErr  bool
    }{
        {"simple", `{"key":"val"}`, "key", "val", false},
        {"nested", `{"a":{"b":"c"}}`, "a.b", "c", false},
        {"invalid", "bad json", "key", "", true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
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

**Integration Test Example**:
```go
func TestVaultIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }

    // Start Docker environment
    env := testutil.StartDockerEnv(t, []string{"vault"})
    defer env.Stop()

    // Seed test data
    env.VaultClient().Write("secret/test", map[string]any{
        "password": "test-secret-123",
    })

    // Test provider
    provider := providers.NewVaultProvider(env.VaultConfig())
    secret, err := provider.Resolve(ctx, provider.Reference{
        Key: "secret/test",
    })

    assert.NoError(t, err)
    assert.Equal(t, "test-secret-123", secret.Value["password"])
}
```

**Security Test Example**:
```go
func TestSecretRedaction(t *testing.T) {
    logger := testutil.NewTestLogger(t)

    secret := logging.Secret("super-secret-password")
    logger.Info("Retrieved secret: %s", secret)

    output := logger.Output()
    assert.Contains(t, output, "[REDACTED]")
    assert.NotContains(t, output, "super-secret-password")
}
```

### Phase 1.4: Docker Infrastructure Design

**File**: `tests/integration/docker-compose.yml` (design)

```yaml
version: '3.8'

services:
  vault:
    image: hashicorp/vault:1.15
    ports:
      - "8200:8200"
    environment:
      VAULT_DEV_ROOT_TOKEN_ID: test-root-token
      VAULT_DEV_LISTEN_ADDRESS: 0.0.0.0:8200
    cap_add:
      - IPC_LOCK
    healthcheck:
      test: ["CMD", "vault", "status"]
      interval: 2s
      timeout: 1s
      retries: 10

  postgres:
    image: postgres:15-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_USER: test
      POSTGRES_PASSWORD: test-password
      POSTGRES_DB: testdb
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U test"]
      interval: 2s
      timeout: 1s
      retries: 10

  localstack:
    image: localstack/localstack:latest
    ports:
      - "4566:4566"
    environment:
      SERVICES: secretsmanager,ssm
      DEBUG: 1
    volumes:
      - ./localstack-init:/etc/localstack/init/ready.d
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:4566/_localstack/health"]
      interval: 2s
      timeout: 1s
      retries: 10

  mongodb:
    image: mongo:7
    ports:
      - "27017:27017"
    environment:
      MONGO_INITDB_ROOT_USERNAME: test
      MONGO_INITDB_ROOT_PASSWORD: test-password
    healthcheck:
      test: ["CMD", "mongosh", "--eval", "db.adminCommand('ping')"]
      interval: 2s
      timeout: 1s
      retries: 10
```

### Phase 1.5: Agent Context Update

Run agent context update script:
```bash
.specify/scripts/bash/update-agent-context.sh claude
```

**Technologies to add**:
- Go testing framework
- Docker & Docker Compose
- HashiCorp Vault
- LocalStack
- PostgreSQL
- mockgen

**Output**: Updated `.claude/context.md` with testing infrastructure technology stack

## Phase 2: Implementation Plan

**Note**: Detailed task breakdown will be generated by `/speckit.tasks` command. This section provides high-level implementation structure.

### Implementation Phases

**Phase 1: Test Infrastructure & Critical Packages (Weeks 1-4)**
- Setup Docker Compose infrastructure
- Create test utilities (`tests/testutil/`)
- Implement provider contract tests
- Add tests for `internal/providers` (0.8% → 85%)
- Add tests for `internal/resolve` (0% → 85%)
- Add tests for `internal/config` (44.9% → 80%)
- Add tests for `internal/rotation` (0% → 75%)
- **Checkpoint**: 60% overall coverage

**Phase 2: Command & Integration Coverage (Weeks 5-7)**
- Add tests for `cmd/dsops/commands` (7.9% → 70%)
- Add tests for `internal/execenv` (0% → 80%)
- Add tests for `internal/template` (0% → 80%)
- Implement end-to-end workflow tests
- **Checkpoint**: 70% overall coverage

**Phase 3: Full Coverage & CI/CD (Weeks 8-10)**
- Add tests for remaining 0% packages
- Implement security test suite (redaction, race conditions)
- Setup CI/CD workflows (`.github/workflows/test.yml`)
- Add coverage gates and reporting
- Documentation: TDD workflow guide
- **Checkpoint**: 80% overall coverage, CI enforced

### Key Files to Create/Modify

**New Test Infrastructure**:
- `tests/integration/docker-compose.yml`
- `tests/testutil/*.go` (10+ helper files)
- `tests/fixtures/**/*` (test data)
- `tests/fakes/provider_fake.go`

**New Test Files** (co-located with source):
- `internal/providers/*_test.go` (15+ provider test files)
- `internal/resolve/*_test.go` (5+ resolution test files)
- `internal/config/*_test.go` (expand existing)
- `internal/rotation/*_test.go` (8+ rotation test files)
- `cmd/dsops/commands/*_test.go` (9+ command test files)

**CI/CD Workflows**:
- `.github/workflows/test.yml` (main test workflow)
- `.github/workflows/coverage.yml` (coverage reporting)
- `.github/workflows/integration.yml` (integration tests)

**Documentation**:
- `docs/developer/testing.md` (testing guide)
- `docs/developer/tdd-workflow.md` (TDD process)
- `tests/README.md` (test infrastructure setup)

**Makefile Targets** (additions):
```makefile
test:              # Run unit tests
test-coverage:     # Run tests with coverage report
test-integration:  # Run integration tests (requires Docker)
test-race:         # Run tests with race detector
test-all:          # Run all tests (unit + integration + race)
coverage-report:   # Generate HTML coverage report
```

## Success Criteria

**Definition of Done**:
1. ✅ Overall test coverage ≥80%
2. ✅ All packages ≥70% coverage (no packages <50%)
3. ✅ Critical packages (providers, resolve, config, rotation) ≥85%
4. ✅ Docker Compose integration test infrastructure complete
5. ✅ CI/CD workflows enforce coverage gates
6. ✅ All tests pass with race detector (`-race`)
7. ✅ Test execution time <10 minutes (full suite)
8. ✅ TDD documentation published
9. ✅ Provider contract tests validate all 14+ providers
10. ✅ Security tests validate redaction across all packages

**Validation**:
```bash
# Coverage check
make test-coverage
# Should show: overall coverage ≥80%

# Integration tests
make test-integration
# Should pass without requiring manual setup

# Race detection
make test-race
# Should pass with no race conditions detected

# CI workflow
git push origin 005-testing-strategy
# CI should run tests and enforce coverage gates
```

## Notes

**TDD Workflow Enforcement**:
- All new code must have tests written first
- PR reviews check for test coverage
- CI blocks merges if coverage drops below 80%

**Test Maintenance**:
- Integration tests auto-cleanup Docker containers
- Test fixtures stored in `testdata/` (Go convention)
- Mock/fake implementations in `tests/mocks/` and `tests/fakes/`

**Performance Considerations**:
- Unit tests run in parallel (`t.Parallel()`)
- Integration tests skip in short mode (`-short`)
- Docker containers cached between test runs when possible
