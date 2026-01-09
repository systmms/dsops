# Tasks: Go Module Tidy Check

**Input**: Design documents from `/specs/022-mod-tidy-check/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md
**Status**: ✅ ALL TASKS COMPLETE

**Tests**: Not required for this build tooling feature (manual verification via Makefile and CI).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

This is a build tooling feature - changes are to configuration files at the repository root:
- `Makefile` - Build automation
- `.github/workflows/ci.yml` - CI configuration
- `lefthook.yml` - Git hooks configuration

---

## Phase 1: Setup

**Purpose**: No setup required - modifying existing project infrastructure

- [x] T001 Verify existing Makefile structure and `check` target in `Makefile`
- [x] T002 Verify existing CI workflow structure in `.github/workflows/ci.yml`

---

## Phase 2: User Story 1 - CI Blocks Stale Dependencies (Priority: P1) 🎯 MVP

**Goal**: CI automatically detects and blocks PRs with un-tidy go.mod/go.sum files

**Independent Test**: Create PR with stale go.mod → CI should fail with clear message

### Implementation for User Story 1

- [x] T003 [US1] Add "Check go.mod/go.sum are tidy" step to `unit-tests` job in `.github/workflows/ci.yml`
  - Place after `go mod download`, before linting
  - Implement backup-compare-restore pattern
  - Display clear error message with fix instructions

**Acceptance Criteria Met**:
- ✅ FR-001: CI includes tidy validation step
- ✅ FR-002: Fails build if files would be modified
- ✅ FR-003: Displays clear error message with instructions
- ✅ FR-004: Does not modify actual repository files

**Checkpoint**: User Story 1 complete - CI will block stale PRs

---

## Phase 3: User Story 2 - Makefile Check Target (Priority: P2)

**Goal**: Developers can verify dependency tidiness locally via `make mod-tidy-check`

**Independent Test**: Run `make mod-tidy-check` with tidy files (pass) and stale files (fail)

### Implementation for User Story 2

- [x] T004 [US2] Add `mod-tidy-check` target to `Makefile`
  - Implement backup-compare-restore pattern
  - Clear error message with fix instructions
  - Preserve original files on failure (no side effects)
- [x] T005 [US2] Update `check` target to include `mod-tidy-check` as first dependency in `Makefile`

**Acceptance Criteria Met**:
- ✅ FR-005: Makefile includes `mod-tidy-check` target
- ✅ FR-006: Preserves original files if check fails
- ✅ FR-007: `check` target includes `mod-tidy-check`

**Checkpoint**: User Story 2 complete - `make mod-tidy-check` works independently

---

## Phase 4: User Story 3 - Pre-commit Hook via Lefthook (Priority: P3)

**Goal**: Optional pre-commit hook automatically checks dependency tidiness before each commit

**Independent Test**: Run `make install-hooks`, attempt commit with stale go.mod → should be blocked

### Implementation for User Story 3

- [x] T006 [P] [US3] Create `lefthook.yml` at repository root
  - Configure pre-commit hook for mod-tidy-check
  - Trigger on staged `.go`, `go.mod`, or `go.sum` file changes
  - Set parallel: false for sequential execution
- [x] T007 [P] [US3] Add `install-hooks` target to `Makefile`
  - Run `npx lefthook install`
  - Clear success message
- [x] T008 [P] [US3] Add `uninstall-hooks` target to `Makefile`
  - Run `npx lefthook uninstall`
  - Allow developers to disable hooks

**Acceptance Criteria Met**:
- ✅ FR-008: Project includes Lefthook configuration
- ✅ FR-009: `install-hooks` target uses `npx lefthook install`
- ✅ FR-010: Pre-commit hook runs tidy check
- ✅ FR-011: Works without global tool installation (npx)

**Checkpoint**: User Story 3 complete - pre-commit hooks available via `make install-hooks`

---

## Phase 5: Polish & Verification

**Purpose**: Final verification and documentation

- [x] T009 Test `make mod-tidy-check` with tidy files (should pass)
- [x] T010 Verify `make help` shows new targets
- [x] T011 Commit all changes with conventional commit message

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Read-only verification
- **User Story 1 (Phase 2)**: No dependencies - can implement CI check first
- **User Story 2 (Phase 3)**: No dependencies - Makefile target independent
- **User Story 3 (Phase 4)**: Depends on T004 (reuses tidy check logic in hook)
- **Polish (Phase 5)**: Depends on all stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent - CI check
- **User Story 2 (P2)**: Independent - Makefile target
- **User Story 3 (P3)**: Hooks call the tidy check, but can be implemented in parallel

### Parallel Opportunities

Tasks T006, T007, T008 can run in parallel (different files):
```bash
# Can run simultaneously:
Task: "Create lefthook.yml at repository root"
Task: "Add install-hooks target to Makefile"
Task: "Add uninstall-hooks target to Makefile"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. ~~Complete Phase 1: Setup~~ ✅
2. ~~Complete Phase 2: User Story 1 (CI)~~ ✅
3. **STOP and VALIDATE**: CI blocks stale PRs
4. MVP delivered - main branch protected

### Incremental Delivery

1. ~~User Story 1 (CI)~~ → Safety net in place ✅
2. ~~User Story 2 (Makefile)~~ → Local validation available ✅
3. ~~User Story 3 (Hooks)~~ → Optional convenience layer ✅

---

## Summary

| Metric | Value |
|--------|-------|
| Total Tasks | 11 |
| Tasks Complete | 11 (100%) |
| User Story 1 (P1) | 1 task - Complete |
| User Story 2 (P2) | 2 tasks - Complete |
| User Story 3 (P3) | 3 tasks - Complete |
| Parallel Tasks | 3 (T006, T007, T008) |

**Implementation Status**: ✅ ALL COMPLETE

Commit: `f423a8d` - "build: add go mod tidy check to Makefile and CI"

---

## Notes

- All tasks marked [x] are complete
- No tests required - manual verification via Makefile targets and CI runs
- Feature is ready for PR and merge
