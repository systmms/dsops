# Phase 0 — Research: Bitwarden Multi-Account Support

All "NEEDS CLARIFICATION" placeholders from the Technical Context resolve below.

## R1. How does `bw` locate its on-disk state?

**Decision**: Use the `BITWARDENCLI_APPDATA_DIR` environment variable to
override `bw`'s default state directory per provider instance.

**Rationale**:
- Documented behavior of the Bitwarden CLI: when set, `bw` reads and writes
  `data.json`, server config, session metadata, etc. under that directory
  instead of the platform default.
- Per-call scope: setting it on the child process only (via `exec.Cmd.Env`)
  means dsops itself never mutates its own environment, so concurrent calls
  from different providers cannot race.
- It is the same mechanism Bitwarden's own docs recommend for multi-account
  CLI usage and is what tools like `rbw` simulate.

**Alternatives considered**:
- `bw login --profile` — no such flag exists.
- `bw config server` alone — sets server URL but doesn't isolate session
  state.
- Reading `~/.config/Bitwarden/data.json` directly (desktop GUI state) —
  rejected: undocumented schema, encrypted at rest, no documented unlock API.
- Running a per-account `bw serve` and talking HTTP — possible future spec
  but adds a background-process dependency. Out of scope for SPEC-026.

## R2. How do we inject env into the `bw` child process without `os.Setenv`?

**Decision**: Extend the `pkg/exec` abstraction with an optional
`EnvCommandExecutor` interface; `RealCommandExecutor` will implement it by
setting `cmd.Env = append(os.Environ(), extras...)`. The Bitwarden provider
type-asserts to the optional interface and falls back to the env-free
`Execute` when extras are empty.

**Rationale**:
- `os.Setenv` is process-global and not safe under concurrent provider
  resolutions (constitution IX: deterministic and reproducible).
- `exec.Cmd.Env` is the standard, documented way to pass per-child env in
  Go. Setting it explicitly does **not** strip the parent env when we
  prepend `os.Environ()`.
- Optional-interface pattern preserves backwards compatibility for any
  existing callers/tests that mock `CommandExecutor`.

**Alternatives considered**:
- Add `env` parameter to `Execute` directly — breaks every existing call
  site and test mock for all providers. Rejected as unnecessarily invasive.
- Per-provider wrapper executor — works, but duplicates the env-merging
  logic for any future provider that needs it. Rejected.
- A separate `BitwardenExecutor` interface inside the provider package —
  works, but the env-injection capability is generic; lives more naturally
  in `pkg/exec`.

## R3. What does `bw status` return, and which fields do we need?

**Decision**: Parse the JSON output of `bw status --raw` (or just
`bw status`, which is JSON-by-default) and read `serverUrl`, `userEmail`,
`status` fields.

**Rationale**:
- `bw status` returns a JSON object across all auth states. Relevant fields
  (verified empirically against `bw 2025.x`):
  - `serverUrl` (string, may be empty when no custom server configured)
  - `userEmail` (string, present when authenticated)
  - `userId` (string)
  - `status` (one of `"unauthenticated"`, `"locked"`, `"unlocked"`)
  - `lastSync` (ISO timestamp string)
- This is the same call existing code uses for the headless flow
  (`bitwarden_status_test.go` already covers the basic shape).
- No extra `bw` calls needed; one `bw status` invocation per provider per
  process gives us everything for both `ensureServer` and `verifyEmail`.

**Alternatives considered**:
- Parse `data.json` directly — fragile across `bw` versions; reuse the
  documented CLI output instead.
- Separate `bw config server --raw` to read the current server — extra
  process spawn for data already in `bw status`. Rejected.

## R4. How does `bw config server` behave?

**Decision**: Call `bw config server <url>` once per process when the
configured `server` differs from the current `serverUrl` in `bw status`.
Treat empty configured value as "use default cloud" → call
`bw config server https://bitwarden.com` (the documented default) if and
only if `bw status` reports a non-empty `serverUrl` other than the cloud
default.

**Rationale**:
- `bw config server <url>` mutates the state directory's `data.json` to
  point future logins/syncs at the given server. It does not require an
  authenticated session and is safe to run when locked or unauthenticated.
- Idempotency check via `bw status` before mutation prevents needless
  writes and side effects on disk-watched setups.
- Reconciling at most once per process (via `sync.Once`) avoids racing
  multiple Resolves into the same `data.json`.

**Edge case**: User has a configured `server` URL but the current state dir
was never initialized (no `data.json`). In that case `bw status` returns
`status: "unauthenticated"` with empty `serverUrl`, and we run
`bw config server <url>` as a one-time setup.

## R5. What does `verifyEmail` actually compare?

**Decision**: `verifyEmail` is a case-insensitive equality check between the
configured `email` and `bw status`'s `userEmail`. Mismatch yields a
`provider.AuthError` (or the closest existing error type) naming the
provider instance, the expected email (redacted with `logging.Secret()` for
logs but shown plainly in the error message returned to the user), and the
observed email.

**Rationale**:
- Bitwarden treats emails as case-insensitive for login.
- The check is a guardrail for the "two providers accidentally point at the
  same state dir, and the desktop user re-logged that dir into a different
  account" scenario (edge case in spec.md).
- Returning the values in the error message is necessary for the user to
  diagnose; only logging is redacted (FR-006).

**Alternatives considered**:
- Skip verification entirely and trust the user — rejected; the failure
  mode is silent and produces wrong secret values, which is a security
  hazard.
- Verify against `userId` instead of `userEmail` — rejected; users
  configure emails, not UUIDs.

## R6. Doctor output format

**Decision**: `dsops doctor` adds a new line per Bitwarden provider:
```
provider <name> (bitwarden):
  appDataDir: /home/op/.config/dsops/bw-personal
  server:     https://vault.bitwarden.com
  email:      alice@example.com   (status: unlocked)
```
When values are unset, render as `<unset>`. When two providers share an
`appDataDir`, append a `⚠ shared with: <other-name>` line under both.

**Rationale**:
- Consistent with the existing doctor section style.
- Surfaces the three fields the user configured plus the live auth status.
- The collision warning satisfies FR-007.

**Alternatives considered**:
- A single-line dump — rejected, hard to read with three fields.
- JSON-only output — covered by `dsops doctor --json` flag (if present)
  but the human-readable form is the primary surface here.

## R7. Test strategy

**Decision**:
- Unit: extend `bitwarden_mock_test.go` and `bitwarden_internal_test.go`
  with the new cases. All assertions go through the mock executor, no real
  `bw` needed.
- Status: extend `bitwarden_status_test.go` with `serverUrl` / `userEmail`
  parse cases (already exercised lightly today).
- Executor: add `pkg/exec/executor_test.go` cases for the env-injecting
  variant (presence of vars in child process; absence when extras empty).
- Integration: a single opt-in test under `tests/integration/bitwarden/`
  guarded by `os.Getenv("DSOPS_BW_INTEGRATION") == "1"` and a present `bw`
  binary, exercising two real `BITWARDENCLI_APPDATA_DIR`s.

**Rationale**: TDD (constitution VII); keeps the default `make test` fast
and dependency-free.

## Open questions

None. All planning unknowns resolved.
