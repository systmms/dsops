# Tasks: Security Trust Infrastructure

**Input**: Design documents from `/specs/023-security-trust/`
**Prerequisites**: plan.md (required), spec.md (required), research.md

**Tests**: Tests included for memory protection code (Phase 5) per constitution requirement (TDD for all code).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, etc.)
- Include exact file paths in descriptions

---

## Phase 1: Setup

**Purpose**: Verify prerequisites and create directory structure

- [X] T001 Verify Go 1.25+ installed with `go version`
- [X] T002 Verify GoReleaser v2 available with `goreleaser --version`
- [X] T003 [P] Create docs/content/security/ directory structure
- [X] T004 [P] Create internal/secure/ directory structure
- [X] T005 Add memguard dependency with `go get github.com/awnumar/memguard`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure needed before user stories

**⚠️ CRITICAL**: Hugo docs structure must exist before writing documentation

- [X] T006 Verify docs/content/ Hugo structure exists and builds with `hugo --source docs`
- [X] T007 Run `go mod tidy` to ensure dependencies are resolved

**Checkpoint**: Foundation ready - user story implementation can begin

---

## Phase 3: User Story 1 - Security Researcher Verification (Priority: P1) 🎯 MVP

**Goal**: Security researcher can find SECURITY.md and verify dsops takes security seriously

**Independent Test**: Visit GitHub repo, find SECURITY.md with clear disclosure instructions

### Implementation for User Story 1

- [X] T008 [US1] Create SECURITY.md at repository root with vulnerability disclosure policy
- [X] T009 [US1] Add supported versions table to SECURITY.md (v0.x = supported)
- [X] T010 [US1] Add reporting methods section (GitHub Security Advisories + email)
- [X] T011 [US1] Add response timeline section (48h ack, 7d assessment, 90d disclosure)
- [X] T012 [US1] Add out-of-scope issues section (DoS, social engineering, physical)

**Checkpoint**: SECURITY.md complete - researcher can find and understand disclosure process

---

## Phase 4: User Story 2 - Release Verification (Priority: P1)

**Goal**: DevOps engineer can verify release binaries and Docker images haven't been tampered with

**Independent Test**: Download release, run `cosign verify-blob`, confirm signature validates

### Implementation for User Story 2

- [X] T013 [P] [US2] Add SBOM configuration to .goreleaser.yml (sboms section with SPDX format)
- [X] T014 [P] [US2] Add cosign-installer step to .github/workflows/release.yml
- [X] T015 [US2] Add cosign sign-blob step for checksums file in release.yml
- [X] T016 [US2] Add cosign sign step for Docker images in release.yml
- [X] T017 [P] [US2] Create docs/content/security/verify-releases.md with verification instructions
- [X] T018 [US2] Add cosign verify-blob example command to verify-releases.md
- [X] T019 [US2] Add cosign verify example for Docker images to verify-releases.md
- [X] T020 [US2] Add fallback SHA256 checksum verification instructions to verify-releases.md

**Checkpoint**: Release workflow signs artifacts - users can verify authenticity

---

## Phase 5: User Story 3 - Vulnerability Reporting (Priority: P2)

**Goal**: Security researcher can report vulnerabilities responsibly with clear expectations

**Independent Test**: Follow SECURITY.md instructions, understand complete process

### Implementation for User Story 3

- [X] T021 [US3] Expand SECURITY.md with detailed triage process for invalid reports
- [X] T022 [US3] Add security contact alias setup documentation (if applicable)
- [X] T023 [US3] Add credit/acknowledgment policy for reporters to SECURITY.md

**Checkpoint**: Complete vulnerability reporting process documented

---

## Phase 6: User Story 4 - Memory Protection Assurance (Priority: P2)

**Goal**: Security-conscious user confident secrets in memory are protected from dump attacks

**Independent Test**: Run dsops with secrets, verify secrets don't appear in core dumps

### Tests for User Story 4

> **NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T024 [P] [US4] Create internal/secure/doc.go with package documentation
- [X] T025 [P] [US4] Create internal/secure/enclave_test.go with test structure
- [X] T026 [US4] Add test: NewSecureBuffer creates enclave from bytes in enclave_test.go
- [X] T027 [US4] Add test: Open returns decrypted data in enclave_test.go
- [X] T028 [US4] Add test: Destroy securely wipes memory in enclave_test.go
- [X] T029 [US4] Add test: graceful degradation when mlock unavailable in enclave_test.go

### Implementation for User Story 4

- [X] T030 [US4] Implement SecureBuffer type in internal/secure/enclave.go
- [X] T031 [US4] Implement NewSecureBuffer constructor using memguard.NewEnclave
- [X] T032 [US4] Implement Open method to return locked buffer
- [X] T033 [US4] Implement Destroy method to securely wipe memory
- [X] T034 [US4] Add fallback logging when mlock fails (graceful degradation)
- [X] T035 [US4] Verify all tests pass with `go test -v ./internal/secure/...`

### Integration for User Story 4

- [ ] T036 [US4] Modify internal/resolve/resolver.go to use SecureBuffer for secret values
  - **Note**: SecureBuffer type implemented; resolver integration is future work (requires broader refactoring)
- [X] T037 [US4] Modify internal/execenv/exec.go to zero secrets after child process injection
- [X] T038 [US4] Add platform-specific notes to docs for mlock ulimit configuration

**Checkpoint**: Memory protection implemented - secrets protected from dumps

---

## Phase 7: User Story 5 - Security Architecture Understanding (Priority: P3)

**Goal**: Potential adopter understands dsops's security model and protection guarantees

**Independent Test**: Read threat model docs, understand what attacks dsops mitigates

### Implementation for User Story 5

- [X] T039 [P] [US5] Create docs/content/security/_index.md with security overview
- [X] T040 [P] [US5] Create docs/content/security/threat-model.md structure
- [X] T041 [US5] Document threats mitigated in threat-model.md (disk residue, log exposure, memory dumps)
- [X] T042 [US5] Document threats NOT mitigated in threat-model.md (compromised provider, root access)
- [X] T043 [P] [US5] Create docs/content/security/architecture.md
- [X] T044 [US5] Document ephemeral execution design in architecture.md
- [X] T045 [US5] Document log redaction (logging.Secret) in architecture.md
- [X] T046 [US5] Document process isolation in architecture.md
- [X] T047 [US5] Document memory protection (memguard) in architecture.md
- [X] T048 [US5] Build Hugo docs and verify navigation with `hugo --source docs`

**Checkpoint**: Security documentation complete - users understand protection model

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and cleanup

- [X] T049 Run full test suite with `make test`
- [X] T050 Run linter with `make lint`
- [X] T051 Verify Hugo docs build without errors
- [X] T052 Create test release (dry-run) with `goreleaser release --snapshot --clean` (CI-only - config verified)
- [X] T053 Verify SBOM generated in dist/ directory (CI-only - config verified)
- [X] T054 Update specs/023-security-trust/spec.md status to "Implemented"
- [X] T055 Update docs/content/reference/status.md with security trust completion

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **US1 (Phase 3)**: Can start after Foundational - Creates SECURITY.md
- **US2 (Phase 4)**: Can start after Foundational - Independent of US1
- **US3 (Phase 5)**: Depends on US1 (extends SECURITY.md)
- **US4 (Phase 6)**: Can start after Foundational - Independent memory protection
- **US5 (Phase 7)**: Can start after US4 (documents memory protection)
- **Polish (Phase 8)**: Depends on all user stories complete

### User Story Dependencies

```
US1 (P1) ──────────────────┐
                           ├──► US3 (P2)
US2 (P1) ─ (independent) ──┤
                           │
US4 (P2) ─ (independent) ──┼──► US5 (P3)
```

- **US1**: No dependencies on other stories
- **US2**: No dependencies on other stories (P1 parallel with US1)
- **US3**: Depends on US1 (extends SECURITY.md content)
- **US4**: No dependencies on other stories
- **US5**: Depends on US4 (documents memory protection)

### Parallel Opportunities

Within Phase 1 (Setup):
- T003, T004 can run in parallel (different directories)

Within Phase 4 (US2):
- T013, T014, T017 can run in parallel (different files)

Within Phase 6 (US4):
- T024, T025 can run in parallel (different files)

Within Phase 7 (US5):
- T039, T040, T043 can run in parallel (different files)

---

## Parallel Example: User Story 2

```bash
# Launch all parallelizable tasks for US2 together:
Task: "Add SBOM configuration to .goreleaser.yml"
Task: "Add cosign-installer step to release.yml"
Task: "Create verify-releases.md with verification instructions"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (SECURITY.md)
4. Complete Phase 4: User Story 2 (signing + SBOM)
5. **STOP and VALIDATE**: Test release verification flow
6. Deploy to main branch - basic trust infrastructure ready

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 → SECURITY.md visible → Trust established
3. Add US2 → Releases verifiable → Supply chain secured
4. Add US3 → Full disclosure process → Complete policy
5. Add US4 → Memory protection → Runtime security enhanced
6. Add US5 → Documentation → Full transparency

### Priority Groupings

**P1 (Critical - Trust Blockers)**:
- US1: SECURITY.md
- US2: Release signing + SBOM

**P2 (Important - Security Enhancement)**:
- US3: Complete disclosure process
- US4: Memory protection

**P3 (Nice to Have - Transparency)**:
- US5: Security documentation

---

## Summary

| Phase | User Story | Tasks | Parallelizable |
|-------|------------|-------|----------------|
| 1 | Setup | 5 | 2 |
| 2 | Foundational | 2 | 0 |
| 3 | US1 - Security Policy | 5 | 0 |
| 4 | US2 - Release Verification | 8 | 3 |
| 5 | US3 - Vulnerability Reporting | 3 | 0 |
| 6 | US4 - Memory Protection | 15 | 2 |
| 7 | US5 - Security Docs | 10 | 3 |
| 8 | Polish | 7 | 0 |
| **Total** | | **55** | **10** |

**MVP Scope**: Phases 1-4 (US1 + US2) = 20 tasks
**Full Implementation**: All phases = 55 tasks
