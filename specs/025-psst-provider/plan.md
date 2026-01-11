# Implementation Plan: psst Provider Integration

**Branch**: `025-psst-provider` | **Date**: 2026-01-11 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/025-psst-provider/spec.md`

## Summary

Add a psst secret store provider to dsops, enabling developers who use psst for local secret management to integrate their existing vaults with dsops's multi-provider ecosystem. The provider wraps the psst CLI, uses plain text output for retrieval, and delegates all authentication to psst (OS keychain or PSST_PASSWORD env var).

## Technical Context

**Language/Version**: Go 1.25 (matches existing project)
**Primary Dependencies**: `pkg/exec` (CommandExecutor), `pkg/provider` (Provider interface), `internal/errors` (UserError)
**Storage**: N/A (psst manages its own `.psst/` vault storage)
**Testing**: `go test` with table-driven tests, `FakeExecutor` for unit tests, contract tests
**Target Platform**: macOS, Linux, Windows (cross-platform)
**Project Type**: Single project - new provider in `internal/providers/`
**Performance Goals**: <2 seconds per secret resolution (SC-002)
**Constraints**: CLI-based (requires psst installed), no custom vault path config
**Scale/Scope**: Single provider, ~300 LOC implementation + ~400 LOC tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Ephemeral-First | ✅ Pass | No file writes; secrets exist only in memory during resolution |
| II. Security by Default | ✅ Pass | Uses `logging.Secret()` wrapper; shell-escapes secret names (FR-012) |
| III. Provider-Agnostic Interfaces | ✅ Pass | Implements `provider.Provider` interface exactly |
| IV. Data-Driven Service Architecture | N/A | Secret store provider, not service integration |
| V. Developer Experience First | ✅ Pass | Clear error messages with suggestions (dserrors.UserError) |
| VI. Cross-Platform Support | ✅ Pass | Pure Go with CLI abstraction via pkgexec |
| VII. Test-Driven Development | ✅ Required | TDD cycle: tests before implementation |
| VIII. Explicit Over Implicit | ✅ Pass | Uses psst's default vault precedence (explicit design choice) |
| IX. Deterministic and Reproducible | ✅ Pass | Same config → same results (psst CLI behavior) |

**Gate Result**: ✅ All applicable gates pass. Proceed to Phase 0.

## Project Structure

### Documentation (this feature)

```text
specs/025-psst-provider/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (N/A for CLI provider)
└── tasks.md             # Phase 2 output (/speckit.tasks command)
```

### Source Code (repository root)

```text
internal/providers/
├── psst.go                    # Provider implementation
├── psst_test.go               # Unit tests with FakeExecutor
└── registry.go                # Add PsstProviderFactory registration

pkg/exec/
└── executor.go                # CommandExecutor interface (existing)

examples/
└── psst.yaml                  # Example configuration

docs/content/reference/providers/
└── psst.md                    # User documentation
```

**Structure Decision**: Single project pattern - new provider files in `internal/providers/` following established patterns (PassProvider, BitwardenProvider, etc.)

## Complexity Tracking

> No violations requiring justification.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | N/A | N/A |

## Implementation Phases

### Phase 0: Research (Complete via /speckit.clarify)

All technical unknowns resolved during clarification session:
- ✅ Authentication: Delegate to psst CLI (OS keychain / PSST_PASSWORD)
- ✅ Secret retrieval: `psst get <name>` plain output (no JSON parsing)
- ✅ Vault precedence: Local `.psst/` → Global `~/.psst/` (psst default)
- ✅ Environment fallback: Document in doctor output (FR-011)
- ✅ Shell escaping: Required for security (FR-012)

### Phase 1: Design

See:
- [data-model.md](./data-model.md) - Entity definitions
- [quickstart.md](./quickstart.md) - Getting started guide
- [research.md](./research.md) - Research findings

### Phase 2: Tasks

Generated via `/speckit.tasks` command after Phase 1 approval.
