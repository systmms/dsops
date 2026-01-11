# Tasks: psst Provider Integration

**Input**: Design documents from `/specs/025-psst-provider/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

**Tests**: Required per Constitution Principle VII (TDD). Tests written BEFORE implementation.

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- All paths relative to repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Provider skeleton and registration

- [ ] T001 Create PsstConfig struct in internal/providers/psst.go
- [ ] T002 Create PsstProvider struct with config, logger, executor fields in internal/providers/psst.go
- [ ] T003 Add NewPsstProvider and NewPsstProviderWithExecutor constructors in internal/providers/psst.go
- [ ] T004 Register psst provider factory in internal/providers/registry.go

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core provider interface implementation that ALL user stories depend on

**⚠️ CRITICAL**: User stories cannot be tested until these methods are implemented

### Tests for Foundational (TDD - Write First, Must Fail)

- [ ] T005 [P] Write contract test for PsstProvider implementing provider.Provider in internal/providers/psst_test.go
- [ ] T006 [P] Write unit test for Name() returning "psst" in internal/providers/psst_test.go
- [ ] T007 [P] Write unit test for Capabilities() returning correct struct in internal/providers/psst_test.go

### Implementation for Foundational

- [ ] T008 Implement Name() method returning "psst" in internal/providers/psst.go
- [ ] T009 Implement Capabilities() method with SupportsVersioning=false, RequiresAuth=true in internal/providers/psst.go
- [ ] T010 Implement executePsst() helper for CLI invocation with shell escaping (FR-012) in internal/providers/psst.go

**Checkpoint**: Provider compiles and is registered; contract test passes

---

## Phase 3: User Story 1 - Use Existing psst Secrets (Priority: P1) 🎯 MVP

**Goal**: Developers can reference existing psst secrets in dsops configuration and resolve them

**Independent Test**: Configure psst provider in dsops.yaml, reference a secret, run `dsops plan` to verify resolution

### Tests for User Story 1 (TDD - Write First, Must Fail)

- [ ] T011 [P] [US1] Write unit test for Validate() detecting missing psst CLI in internal/providers/psst_test.go
- [ ] T012 [P] [US1] Write unit test for Validate() detecting inaccessible vault in internal/providers/psst_test.go
- [ ] T013 [P] [US1] Write unit test for Validate() success case in internal/providers/psst_test.go
- [ ] T014 [P] [US1] Write unit test for Resolve() returning secret value in internal/providers/psst_test.go
- [ ] T015 [P] [US1] Write unit test for Resolve() returning NotFoundError when secret missing in internal/providers/psst_test.go
- [ ] T016 [P] [US1] Write unit test for Resolve() surfacing auth errors from psst CLI in internal/providers/psst_test.go
- [ ] T017 [P] [US1] Write unit test for Describe() returning metadata in internal/providers/psst_test.go

### Implementation for User Story 1

- [ ] T018 [US1] Implement Validate() checking CLI availability with exec.LookPath in internal/providers/psst.go
- [ ] T019 [US1] Implement Validate() testing vault access with `psst list` in internal/providers/psst.go
- [ ] T020 [US1] Implement Resolve() invoking `psst get <key>` and returning SecretValue in internal/providers/psst.go (Note: FR-003 vault precedence handled by psst CLI - no dsops code needed)
- [ ] T021 [US1] Implement error mapping in Resolve() for not-found and auth errors in internal/providers/psst.go
- [ ] T022 [US1] Implement Describe() returning Metadata with exists and size in internal/providers/psst.go
- [ ] T023 [US1] Add logging with logging.Secret() wrapper for all secret operations in internal/providers/psst.go
- [ ] T024 [US1] Implement GetDoctorInfo() method returning resolution source (vault vs env fallback) per FR-011 in internal/providers/psst.go

**Checkpoint**: User Story 1 complete - can resolve secrets from default psst vault

---

## Phase 4: User Story 2 - Multi-Provider Configuration (Priority: P1)

**Goal**: Use psst for development alongside other providers (1Password) for production

**Independent Test**: Configure both psst and another provider, verify correct resolution per environment

**Note**: This story primarily validates that psst integrates correctly with existing dsops multi-provider infrastructure. Most functionality is already in dsops core.

### Tests for User Story 2 (TDD - Write First, Must Fail)

- [ ] T025 [P] [US2] Write integration test with psst + mock provider in same config in internal/providers/psst_test.go

### Implementation for User Story 2

- [ ] T026 [US2] Create example multi-provider configuration in examples/psst.yaml
- [ ] T027 [US2] Verify dsops plan works with psst provider in multi-provider config (manual test documented)

**Checkpoint**: User Story 2 complete - psst works alongside other providers

---

## Phase 5: User Story 3 - Environment-Specific Secrets (Priority: P2)

**Goal**: Specify which psst environment to use via `env` config option

**Independent Test**: Configure psst provider with `env: staging`, verify secrets from staging environment are resolved

### Tests for User Story 3 (TDD - Write First, Must Fail)

- [ ] T028 [P] [US3] Write unit test for executePsst() including --env flag when config.Env set in internal/providers/psst_test.go
- [ ] T029 [P] [US3] Write unit test for Resolve() with env config using correct psst command in internal/providers/psst_test.go
- [ ] T030 [P] [US3] Write unit test for Validate() with env config using --env flag in internal/providers/psst_test.go

### Implementation for User Story 3

- [ ] T031 [US3] Update executePsst() to include --env flag when config.Env is set in internal/providers/psst.go
- [ ] T032 [US3] Update Validate() to use --env flag when config.Env is set in internal/providers/psst.go
- [ ] T033 [US3] Add environment-specific example to examples/psst.yaml

**Checkpoint**: User Story 3 complete - can target specific psst environments

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, edge cases, and quality improvements

### Tests for Edge Cases

- [ ] T034 [P] Write unit test for shell escaping of secret names with special characters in internal/providers/psst_test.go
- [ ] T035 [P] Write unit test for handling secrets with whitespace in values in internal/providers/psst_test.go

### Implementation for Edge Cases

- [ ] T036 Verify shell escaping handles quotes, spaces, and special characters in internal/providers/psst.go
- [ ] T037 Verify stdout trimming handles trailing newlines correctly in internal/providers/psst.go

### Documentation

- [ ] T038 [P] Create user documentation in docs/content/reference/providers/psst.md
- [ ] T039 [P] Add psst to provider list in docs/content/reference/providers/_index.md
- [ ] T040 Update examples/README.md to mention psst example

### Validation

- [ ] T041 Run make check to verify lint, vet, and tests pass
- [ ] T042 Run contract tests to verify provider interface compliance
- [ ] T043 Validate quickstart.md scenarios work end-to-end (requires psst installed)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational - core functionality
- **User Story 2 (Phase 4)**: Depends on US1 - validates multi-provider integration
- **User Story 3 (Phase 5)**: Depends on US1 - adds environment support
- **Polish (Phase 6)**: Depends on all user stories complete

### User Story Dependencies

```text
Setup (Phase 1)
    │
    ▼
Foundational (Phase 2) ─────────┬─────────────────────────┐
    │                           │                         │
    ▼                           ▼                         ▼
User Story 1 (P1) ────► User Story 2 (P1) ────► User Story 3 (P2)
    │                           │                         │
    └───────────────────────────┴─────────────────────────┘
                                │
                                ▼
                        Polish (Phase 6)
```

### Within Each User Story (TDD Cycle)

1. Write tests (marked [P] can run in parallel)
2. Verify tests FAIL (red)
3. Implement code to make tests pass (green)
4. Refactor if needed
5. Story checkpoint - validate independently

### Parallel Opportunities

**Phase 1**: T001-T004 are sequential (same file, interdependent)

**Phase 2**: T005-T007 tests can run in parallel

**Phase 3 (US1)**: T011-T017 tests can run in parallel; implementation is sequential

**Phase 4 (US2)**: T024 standalone; T025-T026 sequential

**Phase 5 (US3)**: T027-T029 tests can run in parallel; implementation is sequential

**Phase 6**: T033-T034 tests parallel; T037-T038 docs parallel

---

## Parallel Example: User Story 1 Tests

```bash
# Launch all US1 tests in parallel (they test different behaviors):
Task: "Write unit test for Validate() detecting missing psst CLI"
Task: "Write unit test for Validate() detecting inaccessible vault"
Task: "Write unit test for Validate() success case"
Task: "Write unit test for Resolve() returning secret value"
Task: "Write unit test for Resolve() returning NotFoundError"
Task: "Write unit test for Resolve() surfacing auth errors"
Task: "Write unit test for Describe() returning metadata"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T010)
3. Complete Phase 3: User Story 1 (T011-T023)
4. **STOP and VALIDATE**: Run `dsops plan` with psst provider
5. Deploy/demo if ready - basic psst integration works

### Incremental Delivery

1. MVP: Setup + Foundational + US1 → Basic secret resolution
2. +US2: Multi-provider validation → Production-ready
3. +US3: Environment support → Full feature parity
4. +Polish: Documentation and edge cases → Release-ready

### Estimated Scope

- **Total tasks**: 43
- **Phase 1 (Setup)**: 4 tasks
- **Phase 2 (Foundational)**: 6 tasks
- **Phase 3 (US1)**: 14 tasks (7 tests + 7 implementation, includes FR-011 doctor integration)
- **Phase 4 (US2)**: 3 tasks (1 test + 2 implementation)
- **Phase 5 (US3)**: 6 tasks (3 tests + 3 implementation)
- **Phase 6 (Polish)**: 10 tasks

---

## Notes

- [P] tasks = different behaviors/files, no dependencies on incomplete tasks
- [Story] label required for user story phases only
- TDD: Write test → verify fails → implement → verify passes
- Commit after each task or logical group
- Use FakeExecutor for unit tests (inject via NewPsstProviderWithExecutor)
- Shell escaping critical for security - test with special characters
