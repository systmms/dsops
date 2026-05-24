# Feature Specification: psst Provider Integration

**Feature Branch**: `025-psst-provider`
**Created**: 2026-01-10
**Status**: Draft
**Input**: User description: "Add a psst provider that allows dsops to retrieve secrets from psst's local encrypted vault, enabling integration with dsops's multi-provider ecosystem."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Use Existing psst Secrets (Priority: P1)

As a developer who already uses psst for local secret management, I want to reference my existing psst secrets in dsops configuration files so that I can use dsops's unified workflow without migrating my secrets.

**Why this priority**: This is the core value proposition - zero-migration integration with existing psst users.

**Independent Test**: Can be fully tested by configuring a psst provider in dsops.yaml, referencing an existing psst secret, and running `dsops plan` to verify resolution.

**Acceptance Scenarios**:

1. **Given** a secret `API_KEY` exists in psst vault, **When** configuring `store://local/API_KEY` in dsops.yaml with a psst provider, **Then** `dsops plan` shows the secret can be resolved.

2. **Given** a secret does not exist in psst vault, **When** referencing it in dsops.yaml, **Then** dsops returns a clear "secret not found" error message.

3. **Given** psst vault exists but is locked, **When** running dsops plan, **Then** dsops returns a clear authentication/unlock error.

---

### User Story 2 - Multi-Provider Configuration (Priority: P1)

As a developer, I want to use psst for local development secrets alongside 1Password for production secrets so that I can manage different environments appropriately.

**Why this priority**: The multi-provider story is essential - psst alone isn't sufficient for production, but combined with other providers it enables a complete workflow.

**Independent Test**: Can be tested by configuring both psst and 1Password providers, with different environments referencing different stores.

**Acceptance Scenarios**:

1. **Given** psst and 1Password providers are configured, **When** running `dsops exec --env development`, **Then** secrets are resolved from psst.

2. **Given** psst and 1Password providers are configured, **When** running `dsops exec --env production`, **Then** secrets are resolved from 1Password.

---

### User Story 3 - Environment-Specific Secrets (Priority: P2)

As a developer with multiple psst environments (dev, staging), I want to specify which psst environment to use in my dsops configuration so that I can target the correct set of secrets.

**Why this priority**: Environment separation is important but secondary to basic functionality.

**Independent Test**: Can be tested by configuring psst provider with `env: staging` and verifying secrets from the staging environment are resolved.

**Acceptance Scenarios**:

1. **Given** psst has secrets in `development` and `staging` environments, **When** configuring psst provider with `env: staging`, **Then** secrets from staging environment are resolved.

2. **Given** no environment is specified in config, **When** resolving secrets, **Then** psst's default environment is used.

---

### Edge Cases

- **psst CLI not installed**: FR-005 validates CLI availability during provider init, returns clear error
- **Vault doesn't exist**: FR-006 validates vault existence during provider init
- **Vault requires authentication**: FR-008 surfaces psst's auth errors directly (keychain failure, locked vault)
- **Environment fallback behavior**: FR-011 surfaces resolution source in doctor output (via the optional `DoctorInfoProvider` interface); documented that psst checks local vault → global vault → env vars
- **Special characters in secret names**: FR-012 passes secret names as discrete argv arguments (no shell), so names with spaces, quotes, or other special characters are handled as a single literal argument without escaping

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support `psst` as a provider type in secretStores configuration
- **FR-002**: System MUST resolve secrets from psst vaults using `store://` reference syntax
- **FR-003**: System MUST use psst's default vault precedence (local `.psst/` → global `~/.psst/`) without exposing custom path configuration
- **FR-004**: System MUST support optional `env` configuration to specify psst environment
- **FR-005**: System MUST validate psst CLI availability during provider initialization
- **FR-006**: System MUST validate vault existence during provider initialization
- **FR-007**: System MUST return `NotFoundError` when requested secret doesn't exist in vault
- **FR-008**: System MUST surface psst CLI auth errors (keychain access failure, locked vault) without pre-validation - delegate authentication entirely to psst
- **FR-009**: System MUST implement the `Describe()` method as an existence check (via `psst list`) that reports whether the secret exists WITHOUT retrieving its value, per the `provider.Provider` contract
- **FR-010**: System MUST report accurate capabilities (no versioning, no rotation, requires auth)
- **FR-011**: System MUST surface resolution source (vault vs env var fallback) in `dsops doctor` output to aid debugging, delivered via an optional `DoctorInfoProvider` interface that `dsops doctor` type-asserts and renders (the core `Provider` interface is unchanged)
- **FR-012**: System MUST pass secret names to the psst CLI as discrete argv arguments via `pkg/exec.CommandExecutor` (no shell, no manual quoting); this is injection-safe by design and avoids the literal-quote corruption that manual escaping (e.g. `%q`) would introduce

### Key Entities

- **PsstProvider**: The provider implementation that wraps psst CLI interaction
- **PsstConfig**: Configuration options (only `env` for environment selection; vault path uses psst defaults)
- **SecretReference**: The `store://provider/secret-name` format for referencing psst secrets

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users with existing psst vaults can resolve secrets in dsops without modifying their psst setup
- **SC-002**: Secret resolution completes within 2 seconds for individual secrets
- **SC-003**: Error messages clearly indicate the cause when resolution fails (CLI missing, vault missing, secret missing, auth required)
- **SC-004**: Provider follows existing dsops provider patterns making it familiar to contributors
- **SC-005**: All existing dsops commands (plan, exec, doctor) work correctly with psst provider

## Assumptions

- psst CLI is installed separately by the user (dsops doesn't bundle or install psst)
- psst vaults use the standard `.psst` directory structure
- `psst get <name>` returns plain secret value; `--json` flag used only for list/metadata operations
- Users have already authenticated with psst (vault unlocked) before running dsops
- psst's environment concept maps cleanly to dsops's multi-environment model

## Clarifications

### Session 2026-01-11

- Q: Should dsops pre-validate psst auth state or delegate fully to psst CLI? → A: Delegate fully to psst CLI - invoke commands and surface any auth errors as-is. psst handles its own authentication via OS keychain (or PSST_PASSWORD env var for CI).
- Q: How should the provider retrieve secrets - JSON parsing or plain output? → A: Use `psst get <name>` (plain output) for retrieval, `--json` only for list/metadata operations. Simpler and less brittle than parsing JSON for every secret.
- Q: How should dsops handle psst's automatic env var fallback behavior? → A: Document behavior AND show resolution source in `dsops doctor` output. Users should understand the resolution chain (local vault → global vault → env var fallback).
- Q: Should dsops expose vault path configuration or use psst's default precedence? → A: Use psst's default precedence (local → global), only expose `env` option. Keeps configuration simple and matches developer expectations.
- Q: How should dsops handle secret names with special characters? → A: Pass the secret name as a discrete argv argument via `pkg/exec.CommandExecutor` (no shell, no manual quoting). This is injection-safe by design and correct for names with spaces/quotes. (Corrected per PR #54 review: the original answer called for `%q` "shell-escaping", which is wrong for `os/exec` — there is no shell, and `%q` would pass literal quotes to psst and break lookups.)
