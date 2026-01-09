# Implementation Plan: Go Module Tidy Check

**Branch**: `022-mod-tidy-check` | **Date**: 2026-01-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/022-mod-tidy-check/spec.md`

## Summary

Add automated validation to ensure `go.mod` and `go.sum` are always tidy. This is a build/tooling feature that adds:
1. CI workflow step to block PRs with stale dependency files
2. Makefile target for local validation
3. Lefthook pre-commit hooks for optional automatic checking

## Technical Context

**Language/Version**: Go 1.25 (existing project), Bash (Makefile/CI), YAML (Lefthook config)
**Primary Dependencies**: Lefthook (via npx - no global install required)
**Storage**: N/A (no data persistence)
**Testing**: Manual verification via Makefile targets and CI runs
**Target Platform**: macOS, Linux, Windows (cross-platform via Lefthook)
**Project Type**: Build tooling addition to existing Go CLI project
**Performance Goals**: Tidy check completes in < 5 seconds
**Constraints**: Must not require global tool installation; must preserve original files on failure
**Scale/Scope**: Single project, 3 files to create/modify

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Ephemeral-First | N/A | Build tooling, not secret handling |
| II. Security by Default | N/A | No secrets involved |
| III. Provider-Agnostic | N/A | Not a provider feature |
| IV. Data-Driven Service | N/A | Not a service feature |
| V. Developer Experience First | ✅ PASS | Clear commands (`make mod-tidy-check`, `make install-hooks`), helpful error messages |
| VI. Cross-Platform Support | ✅ PASS | Lefthook provides cross-platform hooks via npx |
| VII. Test-Driven Development | ✅ PASS | Verification via acceptance scenarios in spec |
| VIII. Explicit Over Implicit | ✅ PASS | Hooks are opt-in via `make install-hooks` |
| IX. Deterministic | ✅ PASS | Same go.mod/go.sum produces same tidy check result |

**Gate Result**: ✅ PASS - No violations

## Project Structure

### Documentation (this feature)

```text
specs/022-mod-tidy-check/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 output (minimal - simple feature)
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
# Files to modify/create:
Makefile                           # Add mod-tidy-check, install-hooks, uninstall-hooks targets
.github/workflows/ci.yml           # Add tidy check step to unit-tests job
lefthook.yml                       # New file - pre-commit hook configuration
```

**Structure Decision**: This is a build tooling feature, not application code. Changes are to existing build/CI files plus one new configuration file.

## Complexity Tracking

No constitution violations - no complexity justification needed.

## Implementation Summary

### Phase 1: Makefile Targets (P2 - Implemented)

1. **`mod-tidy-check` target**: Validates go.mod/go.sum are tidy
   - Backs up files, runs `go mod tidy`, compares, restores on failure
   - Clear error message with fix instructions

2. **`install-hooks` target**: Enables Lefthook via `npx lefthook install`

3. **`uninstall-hooks` target**: Disables hooks via `npx lefthook uninstall`

4. **`check` target update**: Add `mod-tidy-check` as first dependency

### Phase 2: CI Workflow (P1 - Implemented)

1. Add "Check go.mod/go.sum are tidy" step to `unit-tests` job
2. Place after `go mod download`, before linting
3. Same backup-compare-restore logic as Makefile

### Phase 3: Lefthook Configuration (P3 - Implemented)

1. Create `lefthook.yml` with pre-commit hook
2. Hook runs tidy check on `.go` file changes
3. Parallel: false (ensure sequential execution)

## Verification

### Acceptance Testing

1. **CI Check (P1)**:
   - Create PR with stale go.mod → CI should fail
   - Create PR with tidy go.mod → CI should pass

2. **Makefile Check (P2)**:
   ```bash
   # Should pass
   make mod-tidy-check

   # Simulate stale state, should fail
   echo "// test" >> go.mod
   make mod-tidy-check  # Should fail with clear message
   git restore go.mod
   ```

3. **Pre-commit Hook (P3)**:
   ```bash
   make install-hooks
   # Attempt commit with stale go.mod → should be blocked
   make uninstall-hooks
   ```

## Status

**Implementation**: ✅ COMPLETE

All three components have been implemented and committed to branch `022-mod-tidy-check`:
- Makefile targets: `mod-tidy-check`, `install-hooks`, `uninstall-hooks`
- CI workflow step added to `.github/workflows/ci.yml`
- `lefthook.yml` created with pre-commit hook configuration

Commit: `f423a8d` - "build: add go mod tidy check to Makefile and CI"
