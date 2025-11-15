# Task Breakdown: Testing Strategy & Infrastructure

**Feature**: SPEC-005 Testing Strategy & Infrastructure
**Branch**: `005-testing-strategy`
**Goal**: Establish comprehensive testing infrastructure to achieve 80% test coverage
**Status**: Ready for Implementation

## Overview

This task breakdown implements a comprehensive testing strategy for dsops, organized by user story to enable independent implementation and testing. The approach prioritizes critical packages first (providers, resolution, config, rotation) and establishes TDD workflow and infrastructure.

**Total Estimated Tasks**: 75+
**Implementation Phases**: 5 (Setup + 3 User Story Phases + Polish)
**Target Timeline**: 10 weeks
**Test Approach**: TDD (Test-Driven Development) - tests written before implementation

## User Story Mapping

| Story | Priority | Focus | Target Coverage | Independent? |
|-------|----------|-------|-----------------|--------------|
| **US1** | P0 | Test Infrastructure + Critical Packages (providers, resolve, config, rotation) | 60% overall, 85% critical | ✅ Yes |
| **US2** | P1 | TDD Workflow Documentation | N/A (documentation) | ✅ Yes |
| **US3** | P1 | Docker Integration Infrastructure | Infrastructure complete | ⚠️ Depends on US1 test utilities |
| **US4** | P2 | Security Test Validation (redaction, race conditions) | Security tests pass | ⚠️ Depends on US1 test utilities |
| **US5** | P2 | CI/CD Coverage Gates | CI enforces 80% gate | ⚠️ Depends on US1 (tests must exist) |

## Dependency Graph

```
Setup Phase (foundational)
    ↓
US1: Test Infrastructure + Critical Packages (P0) ← PRIMARY DELIVERABLE
    ├→ US2: TDD Documentation (P1)           [independent]
    ├→ US3: Docker Infrastructure (P1)       [uses US1 testutil]
    ├→ US4: Security Tests (P2)              [uses US1 testutil]
    └→ US5: CI/CD Gates (P2)                 [requires US1 tests exist]
```

**Recommended Implementation Order**: Setup → US1 → (US2 + US3) in parallel → (US4 + US5) in parallel → Polish

---

## Phase 1: Setup & Foundational Infrastructure

**Goal**: Create project structure and foundational test utilities needed by all user stories.

**Independent Test Criteria**: Test utilities compile and basic fixtures load successfully.

### Setup Tasks

- [X] T001 Create test directory structure per plan (tests/integration, tests/fixtures, tests/fakes, tests/testutil, tests/mocks)
- [X] T002 Add testify/assert dependency to go.mod (github.com/stretchr/testify v1.8+)
- [X] T003 [P] Create .gitignore entries for test artifacts (coverage.txt, coverage.html, tests/tmp/*)
- [X] T004 [P] Create tests/fixtures/configs/ directory with initial test fixtures
- [X] T005 [P] Create tests/fixtures/secrets/ directory for mock secret data
- [X] T006 [P] Create tests/fixtures/services/ directory for service definitions

### Foundational Test Utilities (blocks all user stories)

- [X] T007 Implement FakeProvider in tests/fakes/provider_fake.go (manual fake for provider.Provider interface)
- [X] T008 Implement TestConfigBuilder in tests/testutil/config.go (programmatic config creation)
- [X] T009 Implement TestLogger in tests/testutil/logger.go (log capture for redaction tests)
- [X] T010 Implement TestFixture in tests/testutil/fixtures.go (fixture loading helpers)
- [X] T011 [P] Implement SetupTestEnv in tests/testutil/env.go (environment variable helpers)
- [X] T012 [P] Implement assertion helpers in tests/testutil/assert.go (AssertSecretRedacted, AssertFileContents)
- [X] T013 Add Makefile targets: test, test-short, test-race, test-coverage, coverage-report

**Checkpoint**: Foundational test utilities compile and can be imported by test files.

---

## Phase 2: User Story 1 - Test Infrastructure + Critical Packages (P0)

**Goal**: Achieve 60% overall coverage by testing critical packages (providers, resolve, config, rotation) and establishing test infrastructure.

**Priority**: P0 (HIGHEST) - Blocks all other user stories
**Target Coverage**: 60% overall, 85% for critical packages
**Timeline**: 4 weeks

**Independent Test Criteria**:
- ✅ Overall test coverage ≥60%
- ✅ internal/providers coverage ≥85%
- ✅ internal/resolve coverage ≥85%
- ✅ internal/config coverage ≥80%
- ✅ internal/rotation coverage ≥75%
- ✅ Provider contract tests pass for all providers
- ✅ All tests pass with -race flag

### Provider Contract Tests (Foundation)

- [X] T014 [US1] Implement ProviderContractTest in tests/testutil/contract.go (generic contract test framework)
- [X] T015 [US1] Implement RunProviderContractTests in tests/testutil/contract.go (executes all contract tests)
- [X] T016 [US1] Create provider contract test suite in internal/providers/contract_test.go (testProviderName, testProviderResolve, testProviderDescribe, testProviderValidate, testProviderCapabilities)

### Provider Tests (internal/providers: 0.8% → 85%)

- [X] T017 [P] [US1] Test Bitwarden provider in internal/providers/bitwarden_test.go (unit tests + contract tests)
- [X] T018 [P] [US1] Test 1Password provider in internal/providers/onepassword_test.go (unit tests + contract tests)
- [X] T019 [P] [US1] Test Literal provider in internal/providers/literal_test.go (unit tests + contract tests)
- [X] T020 [P] [US1] Test Pass provider in internal/providers/pass_test.go (unit tests + contract tests)
- [X] T021 [P] [US1] Test Doppler provider in internal/providers/doppler_test.go (unit tests + contract tests)
- [X] T022 [P] [US1] Test Vault provider in internal/providers/vault_test.go (unit tests + contract tests)
- [X] T023 [P] [US1] Test AWS Secrets Manager provider in internal/providers/aws_secretsmanager_test.go (unit tests + contract tests)
- [X] T024 [P] [US1] Test AWS SSM provider in internal/providers/aws_ssm_test.go (unit tests + contract tests)
- [X] T025 [P] [US1] Test Azure Key Vault provider in internal/providers/azure_keyvault_test.go (unit tests + contract tests)
- [X] T026 [P] [US1] Test GCP Secret Manager provider in internal/providers/gcp_secretmanager_test.go (unit tests + contract tests)
- [X] T027 [US1] Test provider registry in internal/providers/registry_test.go (registration, factory functions, unknown provider errors)

### Resolution Engine Tests (internal/resolve: 0% → 85%)

- [ ] T028 [P] [US1] Test dependency graph resolution in internal/resolve/resolver_test.go (simple resolution, dependency chains, parallel resolution)
- [ ] T029 [P] [US1] Test circular dependency detection in internal/resolve/resolver_test.go (detect cycles, error messages)
- [ ] T030 [P] [US1] Test error aggregation in internal/resolve/resolver_test.go (multiple errors, error formatting)
- [ ] T031 [P] [US1] Test TransformJSONExtract in internal/resolve/transforms_test.go (simple paths, nested paths, arrays, invalid JSON)
- [ ] T032 [P] [US1] Test TransformBase64Decode in internal/resolve/transforms_test.go (valid base64, invalid base64, empty input)
- [ ] T033 [P] [US1] Test TransformYAMLExtract in internal/resolve/transforms_test.go (valid YAML, nested keys, invalid YAML)
- [ ] T034 [P] [US1] Test TransformRegexReplace in internal/resolve/transforms_test.go (pattern matching, capture groups, invalid regex)
- [ ] T035 [P] [US1] Test TransformTemplate in internal/resolve/transforms_test.go (Go templates, variable substitution, template errors)
- [ ] T036 [P] [US1] Test transform pipeline chaining in internal/resolve/transforms_test.go (multiple transforms, order matters)

### Configuration Tests (internal/config: 44.9% → 80%)

- [ ] T037 [P] [US1] Test YAML parsing (v1 format) in internal/config/config_test.go (valid config, secretStores section, services section, envs section)
- [ ] T038 [P] [US1] Test YAML parsing (v0 legacy format) in internal/config/config_test.go (providers section, backward compatibility)
- [ ] T039 [P] [US1] Test config validation in internal/config/validation_test.go (missing version, unknown provider type, invalid references)
- [ ] T040 [P] [US1] Test v0 → v1 migration in internal/config/migration_test.go (providers → secretStores, config transformation)
- [ ] T041 [P] [US1] Test schema validation in internal/config/schema_test.go (required fields, type checking, format validation)
- [ ] T042 [P] [US1] Test config merging in internal/config/merge_test.go (environment-specific overrides, precedence rules)

### Rotation Engine Tests (internal/rotation: 0% → 75%)

- [ ] T043 [P] [US1] Test immediate strategy in internal/rotation/strategies_test.go (immediate rotation, no overlap, validation)
- [ ] T044 [P] [US1] Test two-key strategy in internal/rotation/strategies_test.go (old and new keys coexist, validation order)
- [ ] T045 [P] [US1] Test overlap strategy in internal/rotation/strategies_test.go (grace period, old key removal)
- [ ] T046 [P] [US1] Test connection verification in internal/rotation/verify_test.go (successful connection, failed connection, timeout)
- [ ] T047 [P] [US1] Test grace period handling in internal/rotation/grace_test.go (timer management, expiration, cleanup)
- [ ] T048 [P] [US1] Test rotation state machine in internal/rotation/engine_test.go (state transitions, error handling, rollback)
- [ ] T049 [P] [US1] Test rotation storage in internal/rotation/storage/storage_test.go (save state, load state, state persistence)

### Failing Test Fixes

- [X] T050 [US1] Fix failing tests in internal/services/ (investigate and fix 53.1% coverage with failing tests)

**Checkpoint US1**:
- Run `make test-coverage` → overall coverage ≥60%
- Run `make test-race` → no race conditions
- All provider contract tests pass
- Critical packages meet coverage targets (providers 85%, resolve 85%, config 80%, rotation 75%)

---

## Phase 3a: User Story 2 - TDD Workflow Documentation (P1)

**Goal**: Document TDD practices and provide developer guide for writing tests.

**Priority**: P1
**Timeline**: 1 week (can run in parallel with US3)
**Dependencies**: US1 (needs test patterns to document)

**Independent Test Criteria**:
- ✅ Documentation files exist and are well-formatted
- ✅ All code examples in documentation are valid Go syntax
- ✅ TDD guide references existing test patterns from US1

### Documentation Tasks

- [ ] T051 [P] [US2] Create TDD workflow guide in docs/developer/tdd-workflow.md (Red-Green-Refactor cycle, test patterns, examples)
- [ ] T052 [P] [US2] Create testing strategy guide in docs/developer/testing.md (testing categories, when to use each, best practices)
- [ ] T053 [P] [US2] Create test infrastructure guide in tests/README.md (Docker setup, running tests, test utilities)
- [ ] T054 [P] [US2] Add test pattern examples to docs/developer/test-patterns.md (table-driven tests, integration tests, security tests)
- [ ] T055 [US2] Update CLAUDE.md with testing workflow (reference TDD guide, testing best practices)

**Checkpoint US2**:
- TDD documentation complete and reviewed
- Developer guide published
- CLAUDE.md updated with testing references

---

## Phase 3b: User Story 3 - Docker Integration Infrastructure (P1)

**Goal**: Setup Docker Compose integration test environment for provider testing.

**Priority**: P1
**Timeline**: 2 weeks (can run in parallel with US2)
**Dependencies**: US1 test utilities (TestConfigBuilder, FakeProvider)

**Independent Test Criteria**:
- ✅ `make test-integration` starts Docker services successfully
- ✅ Vault integration test passes
- ✅ LocalStack integration test passes
- ✅ PostgreSQL integration test passes
- ✅ Tests skip gracefully if Docker unavailable

### Docker Infrastructure

- [ ] T056 [US3] Create Docker Compose config in tests/integration/docker-compose.yml (Vault, PostgreSQL, LocalStack, MongoDB services with health checks)
- [ ] T057 [US3] Implement DockerTestEnv in tests/testutil/docker.go (Docker lifecycle management, StartDockerEnv, Stop, WaitForHealthy)
- [ ] T058 [US3] Implement VaultTestClient in tests/testutil/docker.go (Vault API wrapper for testing: Write, Read, Delete, ListSecrets)
- [ ] T059 [US3] Implement LocalStackTestClient in tests/testutil/docker.go (AWS SDK wrapper for LocalStack: CreateSecret, PutParameter)
- [ ] T060 [P] [US3] Implement IsDockerAvailable and SkipIfDockerUnavailable in tests/testutil/docker.go (Docker availability checks)

### Integration Tests

- [ ] T061 [P] [US3] Create Vault integration test in tests/integration/providers/vault_test.go (Docker-based test with real Vault)
- [ ] T062 [P] [US3] Create AWS Secrets Manager integration test in tests/integration/providers/aws_test.go (LocalStack-based test)
- [ ] T063 [P] [US3] Create PostgreSQL integration test in tests/integration/rotation/postgres_test.go (database rotation with Docker)
- [ ] T064 [US3] Add test-integration target to Makefile (docker-compose up, run integration tests, docker-compose down)

**Checkpoint US3**:
- Docker Compose starts all services with health checks passing
- Integration tests pass with real provider implementations
- Tests skip gracefully when Docker unavailable
- `make test-integration` completes successfully

---

## Phase 4a: User Story 4 - Security Test Validation (P2)

**Goal**: Implement comprehensive security tests for secret redaction and leak prevention.

**Priority**: P2
**Timeline**: 1 week
**Dependencies**: US1 test utilities (TestLogger, assertion helpers)

**Independent Test Criteria**:
- ✅ All redaction tests pass (secrets never leak)
- ✅ Race detector passes (`go test -race`)
- ✅ Error messages sanitized (no secret values)
- ✅ Memory safety tests pass

### Security Tests

- [ ] T065 [P] [US4] Test secret redaction in logs in internal/logging/redaction_test.go (Info level, Debug level, multiple secrets)
- [ ] T066 [P] [US4] Test error message sanitization in internal/errors/errors_test.go (errors don't contain secret values, paths sanitized)
- [ ] T067 [P] [US4] Test concurrent provider access in internal/providers/concurrency_test.go (100 goroutines, no race conditions)
- [ ] T068 [P] [US4] Test concurrent resolution in internal/resolve/concurrency_test.go (parallel resolution, race detection)
- [ ] T069 [P] [US4] Test secret cleanup after GC in internal/resolve/memory_test.go (secrets don't persist in memory)
- [ ] T070 [US4] Add race detection to CI workflow in .github/workflows/test.yml (go test -race flag)

**Checkpoint US4**:
- All security tests pass
- Race detector passes with no warnings
- Error messages confirmed to not leak secrets
- CI enforces race detection

---

## Phase 4b: User Story 5 - CI/CD Coverage Gates (P2)

**Goal**: Setup GitHub Actions workflows with automated coverage enforcement.

**Priority**: P2
**Timeline**: 1 week
**Dependencies**: US1 (tests must exist to measure coverage)

**Independent Test Criteria**:
- ✅ CI runs tests on every PR
- ✅ Coverage report generated and uploaded to codecov.io
- ✅ PR comment shows coverage diff
- ✅ CI fails if coverage <80%
- ✅ Coverage badge in README.md works

### CI/CD Workflows

- [ ] T071 [P] [US5] Create test workflow in .github/workflows/test.yml (unit tests with coverage, race detection, codecov upload)
- [ ] T072 [P] [US5] Create integration test workflow in .github/workflows/integration.yml (Docker-based integration tests)
- [ ] T073 [P] [US5] Add coverage gate to test workflow in .github/workflows/test.yml (fail if coverage <80%, clear error message)
- [ ] T074 [US5] Configure codecov.io integration (codecov.yml config, GitHub App installation)
- [ ] T075 [US5] Add coverage badge to README.md (codecov.io badge, links to coverage report)

**Checkpoint US5**:
- CI runs successfully on PRs
- Coverage report appears on codecov.io
- PR comments show coverage diffs
- CI blocks PRs with coverage <80%
- Coverage badge displays correct percentage

---

## Phase 5: Polish & Cross-Cutting Concerns

**Goal**: Complete remaining packages, edge cases, and ensure 80% overall coverage.

**Target Coverage**: 80% overall
**Timeline**: 3 weeks

### Remaining Package Tests (0% → 70%)

- [ ] T076 [P] Test internal/dsopsdata package (data loading, service definition validation)
- [ ] T077 [P] Test internal/execenv package (process execution, environment injection, exit code propagation)
- [ ] T078 [P] Test internal/template package (dotenv rendering, JSON rendering, YAML rendering, Go templates)
- [ ] T079 [P] Test internal/errors package (error formatting, error suggestions, error wrapping)
- [ ] T080 [P] Test internal/policy package (guardrail enforcement, policy evaluation)
- [ ] T081 [P] Test internal/incident package (leak reporting, incident logging)
- [ ] T082 [P] Test internal/vault package (Vault-specific logic, token renewal)
- [ ] T083 [P] Test internal/rotation/storage package (state persistence, state recovery)

### Command Tests (cmd/dsops/commands: 7.9% → 70%)

- [ ] T084 [P] Test plan command in cmd/dsops/commands/plan_test.go (flag parsing, output format, error handling)
- [ ] T085 [P] Test exec command in cmd/dsops/commands/exec_test.go (process execution, environment injection, exit codes)
- [ ] T086 [P] Test render command in cmd/dsops/commands/render_test.go (file rendering, format detection, overwrite protection)
- [ ] T087 [P] Test get command in cmd/dsops/commands/get_test.go (secret retrieval, output formatting)
- [ ] T088 [P] Test doctor command in cmd/dsops/commands/doctor_test.go (validation checks, provider connectivity)
- [ ] T089 [P] Test rotate command in cmd/dsops/commands/rotate_test.go (rotation workflow, strategy selection)

### End-to-End Tests

- [ ] T090 [P] Test init → plan → exec workflow in tests/integration/e2e/workflow_test.go
- [ ] T091 [P] Test multi-provider workflow in tests/integration/e2e/multi_provider_test.go (AWS + Bitwarden + Vault)
- [ ] T092 [P] Test rotation workflow in tests/integration/e2e/rotation_test.go (end-to-end rotation with verification)

### Edge Cases & Error Paths

- [ ] T093 [P] Test invalid configuration handling (malformed YAML, missing required fields)
- [ ] T094 [P] Test provider authentication failures (invalid credentials, network errors)
- [ ] T095 [P] Test boundary conditions (empty configs, very large secrets, special characters)
- [ ] T096 [P] Test error recovery (partial failures, retry logic, timeout handling)

### Final Validation

- [ ] T097 Run full test suite with coverage (`make test-coverage`) and verify ≥80%
- [ ] T098 Run race detector on full suite (`make test-race`) and verify no warnings
- [ ] T099 Verify all packages meet minimum coverage (≥70%, critical packages ≥85%)
- [ ] T100 Update docs/content/reference/status.md with testing milestone completion

**Final Checkpoint**:
- ✅ Overall coverage ≥80%
- ✅ All packages ≥70% (critical packages ≥85%)
- ✅ Race detector passes
- ✅ CI enforces coverage gates
- ✅ Documentation complete
- ✅ Test execution time <10 minutes

---

## Parallel Execution Opportunities

### Within US1 (Critical Packages):
**Parallel Group 1** (Provider Tests): T017-T026 can run in parallel (different providers, no dependencies)
**Parallel Group 2** (Resolution Tests): T028-T036 can run in parallel (independent transforms)
**Parallel Group 3** (Config Tests): T037-T042 can run in parallel (different config aspects)
**Parallel Group 4** (Rotation Tests): T043-T049 can run in parallel (different strategies)

### Across User Stories:
**Parallel Group 5** (US2 + US3): T051-T060 can run in parallel (documentation + Docker setup)
**Parallel Group 6** (US4 + US5): T065-T075 can run in parallel (security tests + CI setup)

### Polish Phase:
**Parallel Group 7** (Remaining Packages): T076-T083 can run in parallel (independent packages)
**Parallel Group 8** (Commands): T084-T089 can run in parallel (independent commands)
**Parallel Group 9** (E2E): T090-T092 can run in parallel (independent scenarios)

---

## Implementation Strategy

### MVP Scope (Week 1-2):
**Deliver**: US1 foundational utilities + provider contract tests + one provider fully tested
**Goal**: Prove testing infrastructure works end-to-end
**Tasks**: T001-T016 + T017 (Bitwarden as proof-of-concept)
**Success**: Provider contract tests pass, FakeProvider usable in tests

### Incremental Delivery (Week 3-10):
1. **Weeks 3-4**: Complete US1 (all providers + critical packages)
2. **Weeks 5-6**: US2 (documentation) + US3 (Docker infrastructure) in parallel
3. **Weeks 7-8**: US4 (security tests) + US5 (CI/CD) in parallel
4. **Weeks 9-10**: Polish phase (remaining packages + edge cases)

### Test Execution Commands:
```bash
# Unit tests only (fast)
make test

# With coverage
make test-coverage

# Integration tests (requires Docker)
make test-integration

# With race detection
make test-race

# Full suite
make test-all

# CI validation
make check  # lint + vet + test-race + coverage gate
```

---

## Task Format Legend

- `- [ ]` = Uncompleted task (checkbox)
- `[P]` = Parallelizable (can run concurrently with other [P] tasks in same group)
- `[US1]`, `[US2]`, etc. = User Story label (maps to spec.md user stories)
- Task ID format: `T###` (sequential numbering for execution order tracking)

---

**Task Breakdown Complete**: 2025-11-14
**Total Tasks**: 100
**Ready for**: `/speckit.implement` to execute tasks systematically
