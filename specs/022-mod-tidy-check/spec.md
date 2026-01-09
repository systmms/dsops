# Feature Specification: Go Module Tidy Check

**Feature Branch**: `022-mod-tidy-check`
**Created**: 2026-01-09
**Status**: Implemented
**Input**: User description: "Add go mod tidy check to development workflow - Add automated checks to ensure go.mod and go.sum are always tidy, preventing stale dependency files. Components: (1) Makefile mod-tidy-check target integrated into make check, (2) CI workflow step in unit-tests job to block PRs with stale deps, (3) Lefthook pre-commit hook using npx lefthook for local development. Use npx lefthook instead of raw git hooks for cross-platform support."

## Overview

This feature adds automated validation to ensure Go module dependency files (`go.mod` and `go.sum`) are always in a tidy state. This prevents stale dependency declarations from being committed, which can cause confusion between local and CI environments during releases.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - CI Blocks Stale Dependencies (Priority: P1)

As a project maintainer, I want CI to automatically detect and block pull requests that have un-tidy go.mod/go.sum files, so that stale dependencies never reach the main branch.

**Why this priority**: This is the most critical check because it prevents bad state from ever reaching main. All other checks are convenience; this is the safety net.

**Independent Test**: Can be tested by creating a PR with modified import statements but without running `go mod tidy`, and verifying CI fails with a clear error message.

**Acceptance Scenarios**:

1. **Given** a PR with stale go.mod (e.g., removed import but dependency still listed), **When** CI runs, **Then** the unit-tests job fails with message indicating "go.mod or go.sum is not tidy" and instructions to fix
2. **Given** a PR with correctly tidied go.mod/go.sum, **When** CI runs, **Then** the tidy check passes and CI proceeds to other checks
3. **Given** a PR with stale go.sum (missing new transitive dependency), **When** CI runs, **Then** the check fails and shows the specific differences

---

### User Story 2 - Makefile Check Target (Priority: P2)

As a developer, I want a `make mod-tidy-check` target that verifies my dependency files are tidy before I commit, so I can catch issues locally before pushing.

**Why this priority**: Enables developers to self-check before committing. Less critical than CI (which is the safety net) but important for developer experience.

**Independent Test**: Can be tested by running `make mod-tidy-check` with tidy files (should pass) and with stale files (should fail with clear message).

**Acceptance Scenarios**:

1. **Given** go.mod and go.sum are already tidy, **When** I run `make mod-tidy-check`, **Then** the command succeeds with confirmation message
2. **Given** go.mod has unused dependencies, **When** I run `make mod-tidy-check`, **Then** the command fails with error message and the original files remain unchanged
3. **Given** I run `make check`, **Then** the mod-tidy-check is included as part of the standard check suite

---

### User Story 3 - Pre-commit Hook via Lefthook (Priority: P3)

As a developer, I want an optional pre-commit hook that automatically checks dependency tidiness before each commit, so I never accidentally commit stale files.

**Why this priority**: Optional convenience feature. Some developers prefer automated hooks while others find them intrusive. Should be opt-in via `make install-hooks`.

**Independent Test**: Can be tested by installing hooks with `make install-hooks`, then attempting to commit with stale go.mod (should be blocked).

**Acceptance Scenarios**:

1. **Given** I have run `make install-hooks`, **When** I attempt to commit with stale go.mod, **Then** the commit is blocked with a message explaining the issue
2. **Given** I have run `make install-hooks`, **When** I commit with tidy go.mod/go.sum, **Then** the commit proceeds normally
3. **Given** I have NOT installed hooks, **When** I commit, **Then** no hook runs (hooks are opt-in)
4. **Given** I want to remove hooks, **When** I run appropriate command, **Then** hooks are disabled

---

### Edge Cases

- What happens when go.mod doesn't exist (not a Go project)? Check should skip gracefully or be skipped for non-Go directories.
- What happens when `go mod tidy` itself fails (e.g., network issues, invalid module)? Error should be surfaced clearly, not confused with "not tidy" state.
- What happens when developer has local `replace` directives? These should be preserved and not cause false positives.
- What happens on Windows vs macOS vs Linux? Lefthook provides cross-platform support; verify behavior is consistent.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: CI workflow MUST include a step that validates go.mod and go.sum are tidy before running other checks
- **FR-002**: CI tidy check MUST fail the build if files would be modified by `go mod tidy`
- **FR-003**: CI tidy check MUST display clear error message with instructions to run `go mod tidy`
- **FR-004**: CI tidy check MUST NOT modify the actual files in the repository
- **FR-005**: Makefile MUST include `mod-tidy-check` target that validates dependency files
- **FR-006**: Makefile `mod-tidy-check` target MUST preserve original files if check fails (no side effects)
- **FR-007**: Makefile `check` target MUST include `mod-tidy-check` as a dependency
- **FR-008**: Project MUST include Lefthook configuration for pre-commit hooks
- **FR-009**: Makefile MUST include `install-hooks` target to enable Lefthook hooks via `npx lefthook install`
- **FR-010**: Pre-commit hook MUST run go mod tidy check before allowing commit
- **FR-011**: All checks MUST work without requiring global tool installation (use npx for Lefthook)

### Key Entities

- **Lefthook Configuration** (`lefthook.yml`): Defines pre-commit hooks and their behavior
- **Makefile Targets**: New targets for tidy checking and hook installation
- **CI Workflow Step**: New step in `.github/workflows/ci.yml` for tidy validation

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of PRs with stale go.mod/go.sum are blocked before merge
- **SC-002**: Developers can verify dependency tidiness locally in under 5 seconds via `make mod-tidy-check`
- **SC-003**: New developers can enable pre-commit hooks with a single command (`make install-hooks`)
- **SC-004**: Zero false positives - tidy check passes when files are genuinely tidy
- **SC-005**: Clear, actionable error messages guide developers to fix issues within 30 seconds

## Assumptions

- Lefthook is available via npx (no global installation required)
- Developers have Node.js/npm installed (standard for most development environments)
- CI runners have Go installed and configured
- The project is a Go module project with go.mod at the root

## Out of Scope

- Automatic fixing of go.mod/go.sum (checks only, no auto-fix)
- Integration with other Git hooks beyond pre-commit
- IDE/editor integration
