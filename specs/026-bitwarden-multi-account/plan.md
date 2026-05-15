# Implementation Plan: Bitwarden Multi-Account Support

**Branch**: `claude/multi-account-desktop-support-eS7zI`
**Spec dir**: `026-bitwarden-multi-account`
**Date**: 2026-05-11
**Spec**: [./spec.md](./spec.md)

## Summary

Add per-instance account isolation to the Bitwarden Password Manager provider by
introducing three optional fields — `appDataDir`, `server`, `email` — that
together pin a provider instance to one account-scoped `bw` CLI state. The work
is contained in `internal/providers/bitwarden.go` plus a thin extension to the
`pkg/exec` command-executor abstraction so per-call environment variables can
be injected without leaking via `os.Setenv`. No changes to the `provider.Provider`
interface or config-parsing layer; new fields are read from the existing
`map[string]interface{}` config bag.

## Technical Context

**Language/Version**: Go 1.25 (existing project minimum)
**Primary Dependencies**: Bitwarden CLI `bw` (executed via `pkg/exec.CommandExecutor`); standard library only inside the provider.
**Storage**: Per-provider account-state directory on disk, owned by `bw` (we only set `BITWARDENCLI_APPDATA_DIR` for the child process).
**Testing**: Go `testing` + table-driven unit tests; mock `CommandExecutor` from `tests/fakes`; opt-in integration test gated on `bw` presence and `DSOPS_BW_INTEGRATION=1`.
**Target Platform**: Linux, macOS, Windows (matches existing provider).
**Project Type**: Single-binary Go CLI (`bin/dsops`); no new packages.
**Performance Goals**: No regression on existing single-account hot path; per-instance `bw config server`/`bw status` calls run at most once per process via `sync.Once`.
**Constraints**: No `os.Setenv` (concurrency-hostile); no reading desktop GUI state; backwards-compatible default behavior; ≥85% coverage in `internal/providers` (constitution VII).
**Scale/Scope**: Realistic upper bound 3–5 Bitwarden accounts per dsops.yaml. Code change: one .go file + one .go file in `pkg/exec` + tests + docs.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Notes |
|---|---|---|
| I. Ephemeral-First | ✅ | No secrets written to disk by dsops. We only point `bw` at a state dir it already owns. Session token still lives in memory only. |
| II. Security by Default | ✅ | Email values flow through `logging.Secret()` (FR-006). Server URLs are logged plainly (not secret). No new on-disk artifacts. |
| III. Provider-Agnostic Interfaces | ✅ | `provider.Provider` interface unchanged. New config keys live inside the existing Bitwarden config map. |
| IV. Data-Driven Service Architecture | N/A | Bitwarden is a secret store, not a service. |
| V. Developer Experience First | ✅ | `dsops doctor` adds per-instance account identity (FR-005). Errors name the offending provider instance (FR-007, SC-004). Examples added. |
| VI. Cross-Platform Support | ✅ | `BITWARDENCLI_APPDATA_DIR` is documented and supported by `bw` on macOS, Linux, Windows. Filesystem path handling uses `filepath.Abs`. |
| VII. Test-Driven Development | ✅ | Tests written before implementation. New mock-executor cases for env injection, `ensureServer` once-only, `verifyEmail` mismatch, doctor reporting. |
| VIII. Explicit Over Implicit | ✅ | All three fields are opt-in; defaults match pre-feature behavior exactly (FR-008). Multi-account headless collisions error fast (FR-007). |
| IX. Deterministic and Reproducible | ✅ | Server reconciliation is idempotent (no-op when current value matches). Same config → same `bw` invocations. |

**Initial gate: PASS.** No principle violations; no Complexity Tracking entries needed.

**Post-design re-check (after Phase 1)**: PASS. The design adds one optional
interface to `pkg/exec` (no breaking change), keeps `provider.Provider`
untouched, and contains all multi-account state inside the existing
`BitwardenProvider` struct. No new persistent storage, no new processes, no
new dependencies. Email handling routes through `logging.Secret()` per
principle II. No principle now fails that previously passed.

## Project Structure

### Documentation (this feature)

```text
specs/026-bitwarden-multi-account/
├── plan.md              # This file
├── research.md          # Phase 0 — bw env vars, status JSON, executor extension
├── data-model.md        # Phase 1 — account profile entity
├── quickstart.md        # Phase 1 — two-account walkthrough
├── contracts/           # Phase 1 — YAML schema delta + Go executor contract
│   ├── config-schema.md
│   └── executor-interface.md
├── checklists/
│   └── requirements.md  # already passed
└── tasks.md             # /speckit.tasks output (not in this command)
```

### Source Code (repository root)

```text
internal/providers/
├── bitwarden.go                       # extend struct + config; bwEnv(); ensureServer(); verifyEmail(); thread env through all bw invocations
├── bitwarden_test.go                  # capability assertions (unchanged)
├── bitwarden_internal_test.go         # parseKey/config tests; add appDataDir/server/email config-parsing cases
├── bitwarden_mock_test.go             # add: env injection, ensureServer once, verifyEmail mismatch, headless collision detection
└── bitwarden_status_test.go           # add: serverUrl + userEmail fields on status parse

pkg/exec/
├── executor.go                        # add EnvCommandExecutor (optional interface) + DefaultExecutor still satisfies it
└── executor_test.go                   # tests for env-injecting variant

cmd/dsops/commands/
└── doctor.go                          # surface per-instance Bitwarden account identity in output

examples/
└── bitwarden-multi-account.yaml       # new — two providers, two accounts

docs/content/providers/bitwarden.md   # new "Multiple accounts" subsection
docs/content/reference/status.md      # SPEC-026 entry
```

**Structure Decision**: Single-project Go CLI; no new packages. All
changes are localized to the Bitwarden provider, the executor abstraction it
uses, and the doctor command. No changes to `internal/config`,
`internal/resolve`, or `pkg/provider`.

## Phase Outputs

Phase 0 → `research.md`: resolves how `bw` discovers its state directory, how
to inject env without `os.Setenv`, what `bw status` returns when the vault is
locked/unauthenticated, and what `bw config server` does when the vault is
already authenticated.

Phase 1 → `data-model.md` (the account-profile entity), `contracts/` (config
schema delta + executor-interface contract), `quickstart.md` (an operator
walkthrough using two real-ish accounts), plus an agent-context refresh.

Phase 2 (separate `/speckit.tasks` command) → `tasks.md`.

## Complexity Tracking

> Fill ONLY if Constitution Check has violations that must be justified.

None.
