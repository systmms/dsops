# GitHub Issues to Create

This file documents issues that should be created manually in the GitHub repository. Delete this file after issues are created.

## Issue #1: Repository needs initial git commit

**Title**: Initialize git repository with first commit

**Labels**: `devops`, `setup`

**Description**:
The dsops repository has been developed but never had an initial git commit. This needs to be done to establish version control history.

**Tasks**:
- [x] Complete all v0.1 MVP features
- [x] Complete documentation
- [ ] Create initial commit with all v0.1 code
- [ ] Tag as v0.1.0
- [ ] Push to GitHub

**Context**:
From STATUS.md (archived): "No Git History - Repository needs initial commit"

---

## Issue #2: Achieve 80% unit test coverage

**Title**: Implement comprehensive test suite (20% → 80% coverage)

**Labels**: `testing`, `quality`, `v0.2`

**Milestone**: v0.2

**Description**:
Current unit test coverage is ~20%, but production readiness requires 80% coverage. See [SPEC-005: Testing Strategy & Plan](../specs/features/005-testing-strategy.md) for detailed implementation plan.

**Current Baseline**:
```
High Coverage (≥80%):
✅ internal/secretstores: 94.1%
✅ internal/validation: 90.0%
✅ pkg/adapter: 83.0%
✅ internal/logging: 80.8%

Medium Coverage (50-79%):
🟡 internal/permissions: 66.1%
🟡 internal/services: 53.1% (HAS FAILING TESTS)
🟡 internal/config: 44.9%

Low/No Coverage (<50%):
❌ cmd/dsops/commands: 7.9%
❌ internal/providers: 0.8%
❌ Zero coverage (0%): internal/dsopsdata, internal/execenv, internal/errors, internal/policy, internal/incident, internal/vault, internal/resolve, internal/rotation/storage, internal/rotation, internal/template
```

**Implementation Plan**:
See [SPEC-005](../specs/features/005-testing-strategy.md) for phased approach:
- Phase 1 (4 weeks): Critical paths to 60% (providers, resolve, config, rotation)
- Phase 2 (3 weeks): Commands & integration to 70%
- Phase 3 (3 weeks): Full coverage & edge cases to 80%

**Related Specifications**:
- [SPEC-005: Testing Strategy & Plan](../specs/features/005-testing-strategy.md)

---

## Issue #3: Add integration test suite

**Title**: Implement Docker-based integration test infrastructure

**Labels**: `testing`, `infrastructure`, `v0.2`

**Milestone**: v0.2

**Description**:
Integration tests are currently at 0% (target: 60%). Need to implement Docker-based test infrastructure for testing with real provider CLIs (Vault, LocalStack for AWS, PostgreSQL, etc.).

**Requirements**:
- Docker Compose configuration for test services:
  - HashiCorp Vault (test secret storage)
  - PostgreSQL (test database rotation)
  - LocalStack (AWS service emulation)
  - MongoDB (test NoSQL rotation)
- Test data fixtures in `testdata/` directories
- CI integration (run in GitHub Actions)
- Graceful degradation when Docker unavailable (skip, don't fail)

**Tasks**:
- [ ] Create `tests/integration/docker-compose.yml`
- [ ] Implement provider integration tests
- [ ] Implement rotation workflow integration tests
- [ ] Add E2E test scenarios (init → plan → exec)
- [ ] Configure GitHub Actions to run integration tests
- [ ] Document test infrastructure setup in README

**Related Specifications**:
- [SPEC-005: Testing Strategy & Plan](../specs/features/005-testing-strategy.md) - Section "Integration Test Infrastructure"

---

## Issue #4: Configure GitHub Actions CI/CD workflows

**Title**: Set up GitHub Actions for CI/CD pipeline

**Labels**: `devops`, `ci-cd`, `v0.2`

**Milestone**: v0.2

**Description**:
GitHub Actions workflows are not configured. Need CI/CD pipeline for automated testing, linting, and releases.

**Required Workflows**:

### `.github/workflows/test.yml` - Test & Coverage
- Run on: pull requests, push to main
- Steps:
  - Checkout code
  - Setup Go
  - Run unit tests with coverage (`go test -race -coverprofile=coverage.txt ./...`)
  - Run integration tests (`make test-integration`)
  - Upload coverage to codecov.io
  - Coverage gate (fail if <80%)

### `.github/workflows/lint.yml` - Code Quality
- Run on: pull requests
- Steps:
  - Checkout code
  - Setup Go
  - Run golangci-lint
  - Check code formatting (`gofmt`)

### `.github/workflows/release.yml` - Releases
- Run on: git tags (v*)
- Steps:
  - Build binaries for multiple platforms (Linux, macOS, Windows)
  - Create GitHub Release
  - Upload binaries as release assets
  - Generate CHANGELOG excerpt for release notes

**Related**:
- [SPEC-005](../specs/features/005-testing-strategy.md) - Section "CI/CD Coverage Gates"

**Tasks**:
- [ ] Create `.github/workflows/test.yml`
- [ ] Create `.github/workflows/lint.yml`
- [ ] Create `.github/workflows/release.yml`
- [ ] Configure codecov.io integration
- [ ] Add status badges to README.md
- [ ] Document CI/CD pipeline in developer docs

---

**Note**: After creating these issues in GitHub, delete this file.
