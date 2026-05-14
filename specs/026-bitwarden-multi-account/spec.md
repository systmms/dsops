# Feature Specification: Bitwarden Multi-Account Support

**Feature Branch**: `claude/multi-account-desktop-support-eS7zI`
**Created**: 2026-05-11
**Status**: Draft
**Input**: User description: "do we support multi account desktop gui bitwarden installations?"

## Summary

Bitwarden's desktop application supports keeping multiple accounts logged in at
once (e.g. a personal account alongside a work account, or a self-hosted
Vaultwarden alongside the cloud vault). Today a dsops configuration cannot
faithfully mirror that setup: declaring two `type: bitwarden` providers shares a
single underlying `bw` CLI state on disk, so only one account stays
authenticated at a time. This feature lets each provider instance pin itself to
an isolated account by introducing three optional fields — an account-scoped
state directory, a server URL, and an expected user email — so multiple
accounts can be resolved within a single `dsops exec` invocation.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Two accounts side-by-side in one run (Priority: P1)

A developer keeps a `personal` Bitwarden account and a `work` Bitwarden account
logged in via the desktop app. They want to read secrets from both within a
single `dsops exec` invocation without one account's CLI login displacing the
other's.

**Why this priority**: This is the core multi-account scenario and a
prerequisite for every other story. Without it, users cannot use dsops
alongside the desktop GUI's headline feature.

**Independent Test**: Configure two `type: bitwarden` providers in a single
`dsops.yaml`, each pinned to a different account-scoped state directory; run
`dsops plan` and `dsops exec -- env` against an environment that references one
variable from each provider. Both values resolve.

**Acceptance Scenarios**:

1. **Given** two Bitwarden providers configured with distinct account state
   directories and both already authenticated, **When** the user runs
   `dsops exec --env dev -- env`, **Then** both providers resolve their
   secrets in the same process without re-prompting and without invalidating
   each other's session.
2. **Given** two providers configured against the same account state directory,
   **When** the user runs `dsops doctor`, **Then** dsops emits a clear
   diagnostic warning that the two providers will race for the same session.
3. **Given** a single provider with no account-scoped fields set, **When** the
   user runs `dsops exec`, **Then** behavior is identical to the pre-feature
   baseline (no surprise change to existing configs).

---

### User Story 2 - Self-hosted Vaultwarden alongside cloud (Priority: P2)

An operator has a personal vault on `vault.bitwarden.com` and a corporate vault
on a self-hosted Vaultwarden instance. They want each provider instance to
target the correct server without manual `bw config server` invocations between
runs.

**Why this priority**: This is the second most common multi-account shape and
the only way to use dsops in shops that run their own Vaultwarden. It is a
strict superset of US1 in configuration terms but adds server reconciliation
logic.

**Independent Test**: Configure two providers with distinct state directories
and distinct `server` URLs; run `dsops plan`. Each provider's state directory
reports the configured server when inspected with `bw status`.

**Acceptance Scenarios**:

1. **Given** a provider configured with a `server` URL that differs from the
   value currently saved in its state directory, **When** dsops first uses that
   provider in a process, **Then** dsops reconciles the server setting before
   the first secret lookup and the operation succeeds.
2. **Given** a provider configured with a `server` URL that matches the saved
   value, **When** dsops uses it, **Then** dsops does not re-issue server
   configuration commands.
3. **Given** a provider configured with a malformed or unreachable `server`
   URL, **When** dsops uses it, **Then** dsops returns a clear error naming
   the provider instance and the offending URL.

---

### User Story 3 - Multi-account headless CI (Priority: P3)

A CI pipeline needs to read secrets from two Bitwarden accounts (e.g. a
shared "infra" account and a service-specific account) without operator
interaction.

**Why this priority**: This combines the existing `headless: true` flow with
multi-account isolation. It is valuable but smaller in audience than US1/US2
and depends on both.

**Independent Test**: In a unit test, drive two `headless: true` providers
with distinct `appDataDir`s through a mock executor and confirm each
provider's `bw login --apikey` and `bw unlock --passwordenv` invocations
carry the matching `BITWARDENCLI_APPDATA_DIR` and the credential env vars
present at call time. In a real CI run, split the secret resolutions across
two dsops invocations (one per account) so each invocation can carry its
own `BW_CLIENTID` / `BW_CLIENTSECRET` / `BW_PASSWORD` env values.

**Note on credentials in a single CI process**: `bw` reads its API-key and
master-password env values (`BW_CLIENTID`, `BW_CLIENTSECRET`, `BW_PASSWORD`)
from its own environment, which is process-wide. dsops mirrors that
contract in v1: running two `headless: true` providers with **different
identities** inside a single `dsops` process is not supported. The
realistic CI shape is one of:
- Two CI steps, each invoking dsops once with its own credentials.
- Same-identity multi-vault: both providers headless, same credentials,
  different `appDataDir`s — useful for separating org vaults.

Adding per-instance credential-env-var overrides (so one process can hold
two identities) is out of scope for SPEC-026 and would be a follow-up.

**Acceptance Scenarios**:

1. **Given** two providers configured with `headless: true`, distinct
   `appDataDir`s, and credential env vars set at invocation time, **When**
   the providers run under a mock executor (unit test) or in their own CI
   step (production), **Then** each provider's `bw login` / `bw unlock` is
   isolated to its own state directory.
2. **Given** two providers configured with `headless: true` that share the
   same `appDataDir`, **When** dsops starts, **Then** dsops fails fast with
   a configuration error naming both providers rather than silently
   overwriting the first login.

---

### Edge Cases

- The configured account state directory does not yet exist on disk.
- Two provider instances reference the same account state directory.
- A provider's configured `server` URL differs from the value already saved in
  its state directory.
- A provider's configured `email` differs from the email reported by the
  authenticated session (e.g. someone re-logged-in the desktop GUI to a
  different account that happens to share the same state directory).
- The Bitwarden desktop GUI is running and unlocked while dsops operates.
  dsops must not depend on or disturb that session.
- The bw CLI is missing or unauthenticated in one provider's state directory
  while another provider is fully authenticated; failure must be localized.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The Bitwarden provider MUST accept an optional account state
  directory setting per provider instance. When omitted, behavior matches the
  current single-account default.
- **FR-002**: When an account state directory is set, every Bitwarden CLI
  invocation made by that provider instance MUST operate against that
  directory and MUST NOT read or write any other provider instance's state.
- **FR-003**: The Bitwarden provider MUST accept an optional server URL per
  provider instance. When set, dsops MUST reconcile the server setting in
  the account state directory at most once per process before the first
  secret operation.
- **FR-004**: The Bitwarden provider MUST accept an optional expected email
  per provider instance. When set, dsops MUST verify that the authenticated
  session in the account state directory matches the expected email before
  the first secret operation, and MUST fail fast on mismatch.
- **FR-005**: `dsops doctor` MUST report, per Bitwarden provider instance: the
  resolved account state directory (absolute path), the configured server,
  and the authenticated email.
- **FR-006**: dsops MUST treat the configured and reported email addresses as
  potentially sensitive and MUST redact them from logs at the standard log
  levels, surfacing them only in `dsops doctor` output and in opt-in debug
  logging.
- **FR-007**: dsops MUST detect when two provider instances reference the
  same account state directory and surface this as a diagnostic
  (warning at `doctor` time; error if both are also marked headless because
  of guaranteed login races).
- **FR-008**: Omitting all three new fields MUST behave identically to the
  pre-feature provider, with no observable difference in `plan`, `exec`,
  `doctor`, or log output.
- **FR-009**: Multi-account configuration MUST be compatible with the
  existing `profile`, `sync`, and `headless` options; the new fields layer on
  top rather than replace them.

### Key Entities *(include if feature involves data)*

- **Account profile**: The unique identity of a single Bitwarden account
  inside dsops. Conceptually a triple of (state directory, server, expected
  email). Each `type: bitwarden` provider instance corresponds to at most one
  account profile.

### Assumptions

- The user has already authenticated each account with the Bitwarden CLI for
  the configured state directory before running dsops (or has provided the
  necessary credentials via the headless flow). dsops does not perform
  first-time interactive login on the user's behalf.
- Each account's state directory persists between runs; dsops does not own
  its lifecycle and does not delete it.
- The Bitwarden desktop GUI and the Bitwarden CLI maintain independent state
  stores. dsops integrates only with the CLI's state and does not read,
  write, or signal the desktop GUI's data.

### Out of Scope (Explicit Non-Goals)

- Reading the Bitwarden desktop app's local data file or any GUI-internal
  storage to discover accounts or reuse unlocked sessions.
- Native messaging / IPC integration with a running Bitwarden desktop process.
- A REST-API integration with a background `bw serve` process. This is a
  plausible future spec but is not in scope here.
- Per-variable account override (selecting a Bitwarden account at the
  individual `store://` reference level rather than the provider level).
- Automatic discovery of accounts the user has logged into elsewhere.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A `dsops.yaml` declaring two `type: bitwarden` providers pinned
  to different accounts resolves a variable from each in a single
  `dsops exec` invocation, with neither provider's session being invalidated
  by the other, demonstrated by a passing acceptance test.
- **SC-002**: `dsops doctor` output unambiguously identifies which Bitwarden
  account is associated with each provider instance, such that a new operator
  can determine which provider talks to which account without reading the
  configuration file.
- **SC-003**: All pre-feature Bitwarden tests pass unchanged; the
  pre-feature configuration shape (no new fields) produces byte-identical
  external behavior in `plan`, `exec`, and `doctor`.
- **SC-004**: A configuration error (shared state directory under headless
  mode, mismatched email, missing prerequisite login) is reported in under
  one second after dsops loads the config, with the offending provider
  instance named in the error message.
- **SC-005**: Combined critical-package coverage for the Bitwarden provider
  remains at or above 85% after the feature lands.
