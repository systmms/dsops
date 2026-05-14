---
description: "Task list for SPEC-026: Bitwarden Multi-Account Support"
---

# Tasks: Bitwarden Multi-Account Support

**Input**: Design documents from `/home/user/dsops/specs/026-bitwarden-multi-account/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅
**Tests**: Required (constitution Principle VII — TDD non-negotiable). Tests for each story are written FIRST and MUST fail before implementation.
**Working branch**: `claude/multi-account-desktop-support-eS7zI`

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Different file, no dependency on incomplete tasks in this phase — safe to run in parallel
- **[USn]**: Maps to user story n from spec.md
- All paths are absolute or repo-rooted

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Confirm a clean baseline before changing code so regressions are unambiguous.

- [X] T001 Run `make check` from repo root and capture the passing baseline (lint, vet, race, coverage); record current `internal/providers` coverage in a working note so post-feature coverage can be compared against it.

> **T001 result (2026-05-14)**: `make check` fails before tests because the `gosec` binary is missing in this environment; lint passes (`0 issues.`). Substituted `go test -race -count=1 -cover ./...` for the baseline: all packages PASS, `internal/providers` at **41.0%**, `pkg/exec` at **100.0%**. The 85% constitution target for `internal/providers` is a pre-existing gap, not caused by this feature; will re-measure after Phase 6.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Extend `pkg/exec` and refactor `internal/providers/bitwarden.go` so per-call env injection and the three new config fields are in place. Every user story below depends on these.

**⚠️ CRITICAL**: No user story tasks may start until this phase is complete.

- [X] T002 [P] Write failing test `TestRealCommandExecutor_ExecuteWithEnv` in `pkg/exec/executor_test.go` asserting that an entry like `FOO=bar` reaches a `printenv FOO` child and that omitting `env` leaves the child env equal to `os.Environ()`.
- [X] T003 Add `EnvCommandExecutor` interface and implement `ExecuteWithEnv(ctx, env, name, args...)` on `RealCommandExecutor` in `pkg/exec/executor.go`, per `specs/026-bitwarden-multi-account/contracts/executor-interface.md`. T002 must pass.
- [X] T004 [P] Write failing test `TestApplyBitwardenConfig_NewFields` in `internal/providers/bitwarden_internal_test.go` covering `appDataDir`, `server`, `email` parsing (presence, absence, `~` expansion for `appDataDir`).
- [X] T005 Extend `BitwardenProvider` struct with `appDataDir`, `server`, `email` fields plus mutex-guarded `observedEmail` / `observedServer` / `observedStatus` and an `accountOnce sync.Once`, and update `applyBitwardenConfig` in `internal/providers/bitwarden.go` to populate them. T004 must pass.
- [X] T006 Introduce private helper `func (bw *BitwardenProvider) run(ctx context.Context, args ...string) ([]byte, []byte, error)` in `internal/providers/bitwarden.go` and route every existing `bw.executor.Execute(ctx, "bw", ...)` call site through it. Initial body delegates to `bw.executor.Execute` unchanged so the refactor is behavior-preserving.
- [X] T007 [P] Write failing test `TestBwEnv` in `internal/providers/bitwarden_internal_test.go` asserting (a) empty slice/nil when `appDataDir` is unset, (b) exactly one `BITWARDENCLI_APPDATA_DIR=<abs path>` entry when set, with `~` expanded.
- [X] T008 Implement `func (bw *BitwardenProvider) bwEnv() []string` in `internal/providers/bitwarden.go` and update `run(...)` to call `ExecuteWithEnv` when `bwEnv()` returns a non-empty slice; if the executor does not satisfy `pkgexec.EnvCommandExecutor` in that case, `run(...)` MUST return an error rather than silently falling back (per `contracts/executor-interface.md`). T007 must pass; pre-existing Bitwarden tests must still pass.

**Checkpoint**: Foundation ready — every `bw` subprocess can carry per-instance env, struct holds the new fields, and config parses them.

---

## Phase 3: User Story 1 — Two accounts side-by-side (Priority: P1) 🎯 MVP

**Goal**: Two `type: bitwarden` providers in one `dsops.yaml` resolve secrets from distinct accounts in the same process via isolated `appDataDir`s. Email verification guards against misconfiguration.

**Independent Test**: With two BitwardenProvider instances bound to a mock executor, distinct `appDataDir` values, and a stubbed `bw status`/`bw get item` script, `Resolve` calls on each instance produce the correct secret and the mock executor records the matching `BITWARDENCLI_APPDATA_DIR=...` for every `bw` invocation. `dsops doctor` flags two instances sharing one `appDataDir`.

### Tests for User Story 1 (write first; must fail before implementation)

- [X] T009 [P] [US1] Write failing test `TestBitwarden_MultipleInstances_EnvIsolated` in `internal/providers/bitwarden_mock_test.go`: construct two providers with distinct `appDataDir`s, resolve one secret from each via mock executor, assert each `bw` call carries the correct `BITWARDENCLI_APPDATA_DIR=...` and no cross-contamination.
- [X] T010 [P] [US1] Write failing test `TestEnsureAccount_VerifyEmailMismatch` in `internal/providers/bitwarden_internal_test.go`: configure `email: alice@x.com`, mock `bw status` returning `bob@x.com`, assert `ensureAccount` returns an error naming the provider instance and both emails, and that no `bw get item` follows.
- [X] T011 [P] [US1] Write failing test `TestEnsureAccount_VerifyEmailMatch_CaseInsensitive` in `internal/providers/bitwarden_internal_test.go`: configure `email: Alice@X.com`, mock `bw status` returning `alice@x.com`, assert no error.
- [X] T012 [P] [US1] Write failing test `TestValidateAppDataDir` in `internal/providers/bitwarden_internal_test.go`: covers absolute-path requirement, `~` expansion, and parent-directory-must-exist check; rejects non-absolute relative paths.
- [X] T013 [P] [US1] Write failing test `TestDoctor_BitwardenSharedAppDataDir_Warns` in `cmd/dsops/commands/doctor_test.go`: two providers with the same `appDataDir` produce a `⚠ shared with:` line under each.

### Implementation for User Story 1

- [X] T014 [US1] Implement `func (bw *BitwardenProvider) ensureAccount(ctx context.Context) error` in `internal/providers/bitwarden.go`, gated by `bw.accountOnce`: runs `bw status` via `bw.run`, unmarshals into the existing `bitwardenStatus` shape (extend if necessary), records `observedEmail`/`observedServer`/`observedStatus`, and — if `bw.email != ""` — case-insensitively compares against `observedEmail` returning a clear error on mismatch. T010 and T011 must pass.
- [X] T015 [US1] Wire `ensureAccount` into the `Resolve` and `Describe` entry points in `internal/providers/bitwarden.go`, ordered as: `ensureHeadlessAuth` → `ensureAccount` → `ensureSync` → existing body.
- [X] T016 [US1] Add `appDataDir` validation (absolute-path coercion via `filepath.Abs`, `~` expansion, parent-exists check) inside `applyBitwardenConfig` in `internal/providers/bitwarden.go`. T012 must pass.
- [X] T017 [US1] Extend `cmd/dsops/commands/doctor.go` to print a per-Bitwarden-provider block (`appDataDir`, `server`, `email`, observed status) using the format in `research.md` §R6, and to detect cross-instance `appDataDir` collisions and emit the warning line. T013 must pass; T009 must pass (env routing already covered by Phase 2).

**Checkpoint**: User Story 1 is independently demonstrable — two accounts resolve side-by-side, mis-email fails fast, doctor surfaces the right info.

---

## Phase 4: User Story 2 — Self-hosted Vaultwarden alongside cloud (Priority: P2)

**Goal**: Each provider instance reconciles its configured `server` against the state directory at most once per process, so a Vaultwarden instance and `vault.bitwarden.com` can coexist without manual `bw config server` calls.

**Independent Test**: A provider configured with `server: https://vw.example.com` against a mock `bw status` that reports a different `serverUrl` triggers exactly one `bw config server https://vw.example.com` invocation before the first `bw get item`, even across multiple `Resolve` calls.

### Tests for User Story 2

- [X] T018 [P] [US2] Write failing test `TestEnsureServer_MismatchTriggersConfig` in `internal/providers/bitwarden_mock_test.go`: configure `server: https://vw.example.com`, mock `bw status` with `serverUrl: https://vault.bitwarden.com`, run two Resolves, assert exactly one `bw config server https://vw.example.com` call (and that it carries the correct env from Phase 2).
- [X] T019 [P] [US2] Write failing test `TestEnsureServer_MatchSkipsConfig` in `internal/providers/bitwarden_mock_test.go`: configure `server: https://vw.example.com`, mock `bw status` already at that URL, assert zero `bw config server` calls.
- [X] T020 [P] [US2] Write failing test `TestEnsureServer_OncePerProcess` in `internal/providers/bitwarden_mock_test.go`: even with five concurrent `Resolve` calls (goroutines), at most one `bw config server` is invoked.
- [X] T021 [P] [US2] Write failing test `TestApplyBitwardenConfig_RejectsMalformedServer` in `internal/providers/bitwarden_internal_test.go`: empty scheme, missing host, and non-http schemes are rejected at config-load time with an error naming the provider instance.

### Implementation for User Story 2

- [X] T022 [US2] Implement `func (bw *BitwardenProvider) ensureServer(ctx context.Context) error` in `internal/providers/bitwarden.go`: when `bw.server != ""` and `bw.observedServer` (populated by `ensureAccount`) differs from `bw.server`, run `bw.run(ctx, "config", "server", bw.server)` and update `bw.observedServer`. Call it from `ensureAccount` immediately after parsing `bw status` and before email verification. T018, T019, T020 must pass.
- [X] T023 [US2] Add URL parse + scheme check to `applyBitwardenConfig` in `internal/providers/bitwarden.go`. T021 must pass.

**Checkpoint**: User Story 2 is independently demonstrable — self-hosted and cloud Bitwarden accounts coexist in one config; doctor output reflects the configured `server` per instance.

---

## Phase 5: User Story 3 — Multi-account headless CI (Priority: P3)

**Goal**: Two `headless: true` Bitwarden providers can log in and unlock against distinct accounts in the same process without sharing state. The illegal case of two headless providers sharing an `appDataDir` fails fast.

**Independent Test**: Two providers with `headless: true` and distinct `appDataDir`s drive `bw login --apikey` / `bw unlock --passwordenv BW_PASSWORD --raw` via the mock executor, each carrying the right env; two headless providers with a shared `appDataDir` cause `Validate()` (or first `Resolve`) to return a configuration error naming both providers.

### Tests for User Story 3

- [ ] T024 [P] [US3] Write failing test `TestHeadless_TwoInstances_EnvIsolated` in `internal/providers/bitwarden_mock_test.go`: two providers with `headless: true`, distinct `appDataDir`s, distinct credential env values set via the mock; assert each provider's `bw login --apikey` and `bw unlock` calls carry the matching `BITWARDENCLI_APPDATA_DIR=...`.
- [ ] T025 [P] [US3] Write failing test `TestHeadless_SharedAppDataDir_Errors` in `internal/providers/bitwarden_mock_test.go` (and a sibling at the registry/`Validate` layer): two providers configured with `headless: true` and the same `appDataDir` fail at construction/validation with an error naming both providers.
- [ ] T026 [P] [US3] Write failing test `TestHeadlessLogin_RoutesThroughExecuteWithEnv` in `internal/providers/bitwarden_mock_test.go`: confirms `bw login --apikey` and `bw unlock` are invoked through `ExecuteWithEnv` when `appDataDir` is set.

### Implementation for User Story 3

- [ ] T027 [US3] Update `ensureHeadlessAuth` in `internal/providers/bitwarden.go` to invoke `bw.run(...)` for `bw login --apikey` and `bw unlock --passwordenv BW_PASSWORD --raw`, so env propagates. T024 and T026 must pass.
- [ ] T028 [US3] Implement cross-instance shared-`appDataDir` + `headless` detection. Preferred location: a one-shot validation pass in `internal/providers/registry.go` (or a helper called from there) that inspects the assembled provider set after construction and returns a configuration error naming both providers. T025 must pass.

**Checkpoint**: All three stories are independently functional — single-account configs are byte-identical to pre-feature behavior.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Examples, docs, and final verification.

- [ ] T029 [P] Create `examples/bitwarden-multi-account.yaml` with two providers (one cloud, one self-hosted Vaultwarden) mirroring the example in `specs/026-bitwarden-multi-account/contracts/config-schema.md`.
- [ ] T030 [P] Add a "Multiple accounts" subsection to `docs/content/providers/bitwarden.md` covering `appDataDir` / `server` / `email`, the multi-instance pattern, and a pointer to the quickstart.
- [ ] T031 [P] Add a SPEC-026 row to `docs/content/reference/status.md` reflecting Implemented status when the feature lands.
- [ ] T032 [P] Update the frontmatter of `specs/026-bitwarden-multi-account/spec.md` from `Status: Draft` → `Status: In Progress` at start of implementation, then `Implemented` once merged.
- [ ] T033 Run `make check` (lint + vet + race + coverage) from repo root; confirm `internal/providers` coverage is ≥85% (constitution VII / SC-005) and that the pre-feature baseline captured in T001 has not regressed.
- [ ] T034 Walk through `specs/026-bitwarden-multi-account/quickstart.md` against a real `bw` install (or a `Vaultwarden`-in-Docker fixture) end-to-end: Steps 1–6, including the negative tests. Record any deviation from documented behavior as a follow-up issue rather than silently editing the spec.

---

## Dependencies & Execution Order

### Phase dependencies

- Phase 1 (Setup) has no dependencies.
- Phase 2 (Foundational) depends on Phase 1; **blocks every user-story phase**.
- Phase 3, 4, 5 (US1, US2, US3) all depend on Phase 2.
  - US1 is the MVP and should land first.
  - US2 and US3 can proceed in parallel after US1 lands (US2 strictly extends `ensureAccount`; US3 strictly extends the headless code path). They share no code paths.
- Phase 6 (Polish) depends on whichever user-story phases are slated for release.

### Within each phase

- TDD: tests precede their matching implementation task and must FAIL before that implementation begins.
- Models / config-parsing changes precede service-layer changes.
- `bitwarden.go` is the hot file across nearly every phase, so two tasks editing it cannot run in parallel even if both are otherwise standalone — observe the [P] markers strictly.

### Parallel opportunities

- All [P] tasks within a phase are safe to run concurrently because they edit different files (or different test functions in the same file *only when explicitly allowed*).
- Phase 2 tests T002 / T004 / T007 are all [P] (different test functions, separate concerns).
- Phase 3 tests T009–T013 are all [P] across `bitwarden_mock_test.go`, `bitwarden_internal_test.go`, and `doctor_test.go`.
- Phase 4 tests T018–T021 are all [P] but T018–T020 share `bitwarden_mock_test.go`; treat the file as a critical section if your tooling serializes file edits, otherwise split into separate Go test functions and run concurrently.
- Phase 6 docs/examples tasks T029–T032 are all [P].

### Cross-story dependencies

- US2's `ensureServer` must call into the `ensureAccount` flow introduced in US1 (it relies on `observedServer` populated there). Sequence: US1 → US2.
- US3 reuses the env-routing helper from Phase 2; no dependency on US1/US2 code paths beyond the shared helper.

---

## Parallel Example — User Story 1

```bash
# Write all US1 tests in parallel (different test files / different test functions):
Task: "TestBitwarden_MultipleInstances_EnvIsolated in internal/providers/bitwarden_mock_test.go"  # T009
Task: "TestEnsureAccount_VerifyEmailMismatch in internal/providers/bitwarden_internal_test.go"   # T010
Task: "TestEnsureAccount_VerifyEmailMatch_CaseInsensitive in same file"                          # T011
Task: "TestValidateAppDataDir in same file"                                                       # T012
Task: "TestDoctor_BitwardenSharedAppDataDir_Warns in cmd/dsops/commands/doctor_test.go"          # T013

# Then implement sequentially in bitwarden.go (single file → serialize):
Task: "ensureAccount() — T014"
Task: "wire into Resolve/Describe — T015"
Task: "appDataDir validation — T016"
Task: "doctor block — T017"   # different file, could run in parallel with T014–T016 once each individual test fail is captured
```

---

## Implementation Strategy

### MVP first (User Story 1 only)

1. Complete Phase 1 (T001).
2. Complete Phase 2 (T002–T008) — foundation done.
3. Complete Phase 3 (T009–T017).
4. Stop, run `dsops exec` against a real two-account config, validate end-to-end manually.
5. Land US1 as a reviewable PR; this delivers ~80% of the user-visible value of the spec.

### Incremental delivery

1. MVP merged → status.md and bitwarden.md updated for US1 (subset of T030/T031).
2. Add US2 (T018–T023) → land independently.
3. Add US3 (T024–T028) → land independently.
4. Final Polish pass (T029, T033, T034) after the last user story merges.

### Parallel team strategy

With two engineers:

- Engineer A: Phase 2 → US1 → US2.
- Engineer B: starts Phase 6 polish tasks T029/T030 in parallel with Engineer A's US1, then takes US3 once Phase 2 lands.

---

## Notes

- [P] means different file (or different non-overlapping test function in the same file) AND no dependency on incomplete in-phase tasks.
- The Bitwarden provider currently lives in a single `internal/providers/bitwarden.go`. Treat the file as a serialization point: two implementation tasks editing it cannot truly parallel-execute even if their concerns are independent.
- Constitution Principle VII mandates TDD; do not skip the failing-test step.
- Constitution Principle II mandates `logging.Secret()` around the configured/observed email values — re-verify during T017 and again during T033's coverage check.
- The Bitwarden desktop GUI is **not touched** by any task in this list; that boundary is part of the spec's explicit out-of-scope guarantees.
