# Tasks: New Secret Store Providers

**Input**: Design documents from `/specs/021-new-providers/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Included - dsops follows TDD (Constitution Principle VII)

**Organization**: Tasks are grouped by user story (P1 providers → configuration → doctor integration)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add dependencies, create shared types and test infrastructure

- [X] T001 Add Akeyless SDK dependency to go.mod: `github.com/akeylesslabs/akeyless-go/v3`
- [X] T002 [P] Create provider error types in internal/providers/errors.go (KeychainError, InfisicalError, AkeylessError)
- [X] T003 [P] Create token cache utility in internal/providers/token_cache.go (per-process caching per FR-017)
- [X] T004 [P] Create test fakes directory structure: tests/fakes/fake_keychain.go, fake_infisical.go, fake_akeyless.go

**Checkpoint**: ✅ Dependencies installed, shared utilities ready

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Contracts and base interfaces that all providers need

**⚠️ CRITICAL**: No provider implementation can begin until this phase is complete

- [X] T005 Copy contract interfaces from specs/021-new-providers/contracts/ to internal/providers/contracts/
- [X] T006 [P] Create fake keychain client in tests/fakes/fake_keychain.go implementing contracts.KeychainClient
- [X] T007 [P] Create fake infisical client in tests/fakes/fake_infisical.go implementing contracts.InfisicalClient
- [X] T008 [P] Create fake akeyless client in tests/fakes/fake_akeyless.go implementing contracts.AkeylessClient
- [X] T009 Create provider factory type definitions in internal/providers/factory.go

**Checkpoint**: ✅ Foundation ready - provider implementation can now begin in parallel

---

## Phase 3: User Story 1 - OS Keychain Provider (Priority: P1) 🎯 MVP

**Goal**: Retrieve secrets from macOS Keychain and Linux Secret Service

**Independent Test**: Store test credential in OS keychain, configure dsops, verify resolution

### Tests for User Story 1

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation (TDD)**

- [X] T010 [P] [US1] Contract test for keychain provider in tests/contract/keychain_contract_test.go (tests in internal/providers/keychain_test.go)
- [X] T011 [P] [US1] Unit test for keychain.go in internal/providers/keychain_test.go (use fake client)
- [X] T012 [P] [US1] Unit test for reference parsing in internal/providers/keychain_test.go

### Implementation for User Story 1

- [X] T013 [US1] Create keychain provider struct in internal/providers/keychain.go
- [X] T014 [US1] Implement Resolve() method in internal/providers/keychain.go (FR-001, FR-002)
- [X] T015 [US1] Implement Describe() method in internal/providers/keychain.go
- [X] T016 [US1] Implement Validate() method in internal/providers/keychain.go
- [X] T017 [US1] Implement Capabilities() method in internal/providers/keychain.go
- [X] T018 [P] [US1] Create macOS-specific client in internal/providers/keychain_darwin.go (FR-003)
- [X] T019 [P] [US1] Create Linux Secret Service client in internal/providers/keychain_linux.go (FR-002)
- [X] T020 [P] [US1] Create unsupported platform stub in internal/providers/keychain_unsupported.go (FR-004)
- [X] T021 [US1] Add platform detection for headless environments in internal/providers/keychain.go
- [X] T022 [US1] Implement parseKeychainReference() in internal/providers/keychain.go
- [X] T023 [US1] Add keychain factory function and register in internal/providers/registry.go

**Checkpoint**: ✅ `store://keychain/service/account` works on macOS and Linux

---

## Phase 4: User Story 2 - Infisical Provider (Priority: P1)

**Goal**: Retrieve secrets from Infisical (cloud or self-hosted)

**Independent Test**: Configure Infisical project with test secrets, verify dsops retrieval

### Tests for User Story 2

- [X] T024 [P] [US2] Contract test for infisical provider in tests/contract/infisical_contract_test.go (tests in internal/providers/infisical_test.go)
- [X] T025 [P] [US2] Unit test for infisical.go in internal/providers/infisical_test.go (use fake client)
- [X] T026 [P] [US2] Unit test for HTTP client wrapper in internal/providers/infisical_client_test.go

### Implementation for User Story 2

- [X] T027 [US2] Create HTTP client wrapper in internal/providers/infisical_client.go (uses net/http)
- [X] T028 [US2] Implement Authenticate() in infisical_client.go (FR-005: machine_identity, service_token, api_key)
- [X] T029 [US2] Implement GetSecret() in infisical_client.go (FR-006)
- [X] T030 [US2] Implement ListSecrets() in infisical_client.go
- [X] T031 [US2] Create infisical provider struct in internal/providers/infisical.go
- [X] T032 [US2] Implement Resolve() method in internal/providers/infisical.go (FR-006, FR-008)
- [X] T033 [US2] Implement Describe() method in internal/providers/infisical.go
- [X] T034 [US2] Implement Validate() method in internal/providers/infisical.go (FR-007)
- [X] T035 [US2] Implement Capabilities() method in internal/providers/infisical.go
- [X] T036 [US2] Add token caching with 30s timeout in internal/providers/infisical.go (FR-017)
- [X] T037 [US2] Implement parseInfisicalReference() in internal/providers/infisical.go
- [X] T038 [US2] Support custom CA certificates in infisical_client.go (self-hosted TLS)
- [X] T039 [US2] Add infisical factory function and register in internal/providers/registry.go

**Checkpoint**: ✅ `store://infisical/SECRET_NAME` works with cloud and self-hosted instances

---

## Phase 5: User Story 3 - Akeyless Provider (Priority: P1)

**Goal**: Retrieve secrets from Akeyless enterprise platform

**Independent Test**: Configure Akeyless with test secrets, verify dsops retrieval

### Tests for User Story 3

- [X] T040 [P] [US3] Contract test for akeyless provider in tests/contract/akeyless_contract_test.go (tests in internal/providers/akeyless_test.go)
- [X] T041 [P] [US3] Unit test for akeyless.go in internal/providers/akeyless_test.go (use fake client)
- [X] T042 [P] [US3] Unit test for authentication methods in internal/providers/akeyless_test.go

### Implementation for User Story 3

- [X] T043 [US3] Create akeyless SDK wrapper in internal/providers/akeyless_client.go
- [X] T044 [US3] Implement Authenticate() with API key method in akeyless_client.go (FR-009)
- [X] T045 [US3] Implement Authenticate() with AWS IAM method in akeyless_client.go (FR-009)
- [X] T046 [P] [US3] Implement Authenticate() with Azure AD method in akeyless_client.go (FR-009)
- [X] T047 [P] [US3] Implement Authenticate() with GCP method in akeyless_client.go (FR-009)
- [X] T048 [US3] Implement GetSecret() in akeyless_client.go (FR-010)
- [X] T049 [US3] Implement DescribeItem() in akeyless_client.go
- [X] T050 [US3] Implement ListItems() in akeyless_client.go
- [X] T051 [US3] Create akeyless provider struct in internal/providers/akeyless.go
- [X] T052 [US3] Implement Resolve() method in internal/providers/akeyless.go (FR-010, FR-012)
- [X] T053 [US3] Implement Describe() method in internal/providers/akeyless.go
- [X] T054 [US3] Implement Validate() method in internal/providers/akeyless.go (FR-011)
- [X] T055 [US3] Implement Capabilities() method in internal/providers/akeyless.go
- [X] T056 [US3] Add token caching with 30s timeout in internal/providers/akeyless.go (FR-017)
- [X] T057 [US3] Implement parseAkeylessReference() in internal/providers/akeyless.go
- [X] T058 [US3] Add akeyless factory function and register in internal/providers/registry.go

**Checkpoint**: ✅ `store://akeyless/path/to/secret` works with all auth methods

---

## Phase 6: User Story 4 - Configuration (Priority: P1)

**Goal**: Parse and validate provider configurations in dsops.yaml

**Independent Test**: Create valid/invalid configs, verify parsing and validation errors

### Tests for User Story 4

- [X] T059 [P] [US4] Unit test for keychain config parsing in internal/config/keychain_test.go (config parsing integrated in provider tests)
- [X] T060 [P] [US4] Unit test for infisical config parsing in internal/config/infisical_test.go (config parsing integrated in provider tests)
- [X] T061 [P] [US4] Unit test for akeyless config parsing in internal/config/akeyless_test.go (config parsing integrated in provider tests)
- [X] T062 [P] [US4] Integration test for config validation in internal/config/config_test.go

### Implementation for User Story 4

- [X] T063 [P] [US4] Add keychain config struct to internal/config/provider_config.go (FR-015)
- [X] T064 [P] [US4] Add infisical config struct to internal/config/provider_config.go (FR-015)
- [X] T065 [P] [US4] Add akeyless config struct to internal/config/provider_config.go (FR-015)
- [X] T066 [US4] Add config validation for keychain (service_prefix, access_group optional)
- [X] T067 [US4] Add config validation for infisical (project_id, environment required; FR-015)
- [X] T068 [US4] Add config validation for akeyless (access_id required; FR-015)
- [X] T069 [US4] Map config structs to provider factory in internal/config/loader.go
- [X] T070 [US4] Add clear validation error messages per FR-013

**Checkpoint**: ✅ Configuration loads and validates correctly for all three providers

---

## Phase 7: User Story 5 - Doctor Integration (Priority: P2)

**Goal**: Providers report health status via `dsops doctor`

**Independent Test**: Run `dsops doctor` with each provider configured

### Tests for User Story 5

- [X] T071 [P] [US5] Unit test for keychain doctor checks in cmd/dsops/commands/doctor_test.go (Validate() tests in provider tests)
- [X] T072 [P] [US5] Unit test for infisical doctor checks in cmd/dsops/commands/doctor_test.go (Validate() tests in provider tests)
- [X] T073 [P] [US5] Unit test for akeyless doctor checks in cmd/dsops/commands/doctor_test.go (Validate() tests in provider tests)

### Implementation for User Story 5

- [X] T074 [US5] Add keychain health check (platform detection, access check) to Validate() (FR-014)
- [X] T075 [US5] Add infisical health check (auth test, project access) to Validate() (FR-014)
- [X] T076 [US5] Add akeyless health check (auth test, gateway connectivity) to Validate() (FR-014)
- [X] T077 [US5] Ensure doctor output includes remediation steps per FR-013
- [X] T078 [US5] Add provider types to cmd/dsops/commands/providers.go descriptions

**Checkpoint**: ✅ `dsops doctor` shows health status for all three providers

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, examples, and integration tests

- [X] T079 [P] Create example configuration in examples/keychain.yaml
- [X] T080 [P] Create example configuration in examples/infisical.yaml
- [X] T081 [P] Create example configuration in examples/akeyless.yaml
- [X] T082 [P] Create user documentation in docs/content/providers/keychain.md
- [X] T083 [P] Create user documentation in docs/content/providers/infisical.md
- [X] T084 [P] Create user documentation in docs/content/providers/akeyless.md
- [X] T085 Update docs/PROVIDERS.md with new provider entries (provider docs in docs/content/providers/)
- [ ] T086 [P] Create integration test for keychain in tests/integration/keychain_integration_test.go (deferred - requires OS keychain access, can't mock HTTP)
- [X] T087 [P] Create integration test for infisical in tests/integration/providers/infisical_integration_test.go (uses mock HTTP server)
- [X] T088 [P] Create integration test for akeyless in tests/integration/providers/akeyless_integration_test.go (uses mock HTTP server)
- [X] T089 Run all provider contract tests to verify interface compliance (SC-006)
- [X] T090 Run `make lint` and fix any linting issues
- [X] T091 Run `make test-race` to verify no race conditions (SC-005)
- [X] T092 Update spec.md status from Draft to Implemented

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all provider implementation
- **User Stories 1-3 (Phases 3-5)**: All depend on Foundational; can run in parallel
- **User Story 4 (Phase 6)**: Depends on provider structs from US1-3 being defined
- **User Story 5 (Phase 7)**: Depends on US1-4 completion
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

| Story | Depends On | Can Parallelize With |
|-------|------------|---------------------|
| US1 (Keychain) | Phase 2 | US2, US3 |
| US2 (Infisical) | Phase 2 | US1, US3 |
| US3 (Akeyless) | Phase 2 | US1, US2 |
| US4 (Config) | US1-3 structs defined | None |
| US5 (Doctor) | US1-4 | None |

### Within Each User Story

1. Tests MUST be written and FAIL before implementation (TDD)
2. Client wrapper before provider struct
3. Core methods (Resolve, Validate) before auxiliary (Describe, Capabilities)
4. Reference parsing before resolution
5. Factory registration last

### Parallel Opportunities

**Phase 2 (Foundational)**:
```
T006, T007, T008 - All fake clients can be written in parallel
```

**Phases 3-5 (Provider Implementation)**:
```
# All three providers can be implemented in parallel after Phase 2:
Developer A: US1 (Keychain) - T010-T023
Developer B: US2 (Infisical) - T024-T039
Developer C: US3 (Akeyless) - T040-T058
```

**Within US1 (Keychain)**:
```
# Platform-specific files can be written in parallel:
T018 (darwin), T019 (linux), T020 (unsupported)
```

**Phase 8 (Polish)**:
```
# All documentation and integration tests can run in parallel:
T079-T088 all marked [P]
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T009)
3. Complete Phase 3: User Story 1 - Keychain (T010-T023)
4. **STOP and VALIDATE**: Test keychain provider independently
5. Demo: `dsops exec --config examples/keychain.yaml -- env | grep MY_SECRET`

### Incremental Delivery

1. **MVP**: Setup + Foundational + US1 (Keychain) → Test → Deploy
2. **Increment 2**: Add US2 (Infisical) → Test → Deploy
3. **Increment 3**: Add US3 (Akeyless) → Test → Deploy
4. **Increment 4**: Add US4 (Config validation) → Test → Deploy
5. **Increment 5**: Add US5 (Doctor) → Test → Deploy
6. **Final**: Polish phase (docs, integration tests) → Release

### Parallel Team Strategy

With 3 developers after Phase 2:
- Developer A: Keychain (US1) → Config keychain parts (US4 partial) → Doctor keychain (US5 partial)
- Developer B: Infisical (US2) → Config infisical parts (US4 partial) → Doctor infisical (US5 partial)
- Developer C: Akeyless (US3) → Config akeyless parts (US4 partial) → Doctor akeyless (US5 partial)

---

## Summary

| Metric | Count | Completed |
|--------|-------|-----------|
| Total Tasks | 92 | 91 |
| Phase 1 (Setup) | 4 | 4 ✅ |
| Phase 2 (Foundational) | 5 | 5 ✅ |
| Phase 3 (US1 Keychain) | 14 | 14 ✅ |
| Phase 4 (US2 Infisical) | 16 | 16 ✅ |
| Phase 5 (US3 Akeyless) | 19 | 19 ✅ |
| Phase 6 (US4 Config) | 12 | 12 ✅ |
| Phase 7 (US5 Doctor) | 8 | 8 ✅ |
| Phase 8 (Polish) | 14 | 13 |
| Parallelizable [P] | 38 | - |

**Implementation Status**: ✅ **COMPLETE** (99% - 91/92 tasks)

**Deferred Tasks** (1 remaining):
- T086: Keychain integration test requires OS-level keychain access (macOS Keychain or Linux Secret Service) - can't be mocked via HTTP

**Completed via Mock HTTP Servers**:
- T087: Infisical integration test using httptest.NewServer() mock
- T088: Akeyless integration test using httptest.NewServer() mock

**Notes**:
- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each provider can be independently completed and tested
- All unit tests pass with race detection
- Integration tests for Infisical/Akeyless use mock HTTP servers
- Spec status updated to Implemented
