# Feature Specification: Machine-level Secret Store Declarations

**Feature Branch**: `claude/machine-level-secret-refs-g533q7`
**Created**: 2026-09-06
**Status**: Implemented
**Input**: User description: "can you help me figure out how to make machine level references for secret stores for projects? i also want to figure out the pros/cons of that setup; for example this could be an open source project (useable by anyone) but i use nix to manage my machine."

## Summary

dsops configuration has always been strictly project-local: `dsops.yaml` must
declare every secret store it references, including machine-specific details
such as a Bitwarden `appDataDir`, a Vault address, or an AWS profile. That
forces an open-source project to bake one maintainer's machine layout into a
file every contributor shares.

This feature adds a **machine-level (user) config file** that declares secret
stores only. A project `dsops.yaml` can then reference `store://work-vault/...`
without defining `work-vault`; each contributor binds that name to their own
backend in `~/.config/dsops/config.yaml` (or wherever `DSOPS_USER_CONFIG`
points). The project file always wins on a name collision, the user file may
not declare anything but stores, and `dsops doctor` / `dsops providers` show
where every store came from.

The split is: **credentials and bindings are machine-level, references are
project-level.** Both files hold only references, never secret values.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Maintainer declares stores once, per machine (Priority: P1)

A maintainer manages their laptop with nix (home-manager / nix-darwin). They
want to declare `work-vault` and `bw-work` once in their nix configuration and
have every project on that machine resolve those names, with the project
`dsops.yaml` staying free of home-directory paths, emails, or server URLs.

**Why this priority**: This is the core scenario and the reason the feature
exists. Without it, portable project configs are impossible.

**Independent Test**: Write a project `dsops.yaml` with no `secretStores:`
section referencing `store://work-vault/x`; write
`$XDG_CONFIG_HOME/dsops/config.yaml` declaring `work-vault: {type: literal}`;
run `dsops plan --env dev`. The variable resolves and `dsops providers` shows
`work-vault` with source `user`.

**Acceptance Scenarios**:

1. **Given** a user config declaring `work-vault` and a project referencing it
   without declaring it, **When** the user runs any command that loads config,
   **Then** the store resolves exactly as if it had been declared in the
   project.
2. **Given** a user config file that is a symlink into `/nix/store` (mode
   0444), **When** dsops loads it, **Then** no permission warning is emitted.
3. **Given** no user config file at the default location, **When** dsops runs,
   **Then** behaviour is byte-identical to the pre-feature baseline.

---

### User Story 2 - Contributor without nix onboards (Priority: P2)

A contributor clones the open-source project. They do not use nix. They need
to discover which stores the project expects and where to declare them.

**Why this priority**: Portability is only real if the non-nix path is
obvious.

**Independent Test**: With no user config, run `dsops doctor` or `dsops plan`
against the project. The error names the missing store, the project file, and
the exact user-config path to create. After copying
`examples/user-config.yaml` there, the same command succeeds and the store
shows source `user`.

**Acceptance Scenarios**:

1. **Given** a missing store, **When** resolution fails, **Then** the
   suggestion reads "Add provider 'x' to the 'secretStores:' section of
   <project path>, or to your user config at <user path>".
2. **Given** `dsops doctor`, **When** it runs, **Then** it prints a
   "Configuration sources" block with the project path and the user path
   (loaded, "none found at <default>", or "disabled"), and a `SOURCE` column
   per store.
3. **Given** the same store name in both files, **When** doctor runs, **Then**
   the project definition is used and the user entry is reported as shadowed.

---

### User Story 3 - CI and scripts pin or disable the user config (Priority: P3)

A CI job must not accidentally pick up a runner's home directory, and a script
may want to point at a specific machine file.

**Why this priority**: Determinism for automation; prevents "works on the
runner" surprises.

**Independent Test**: `DSOPS_USER_CONFIG=none dsops plan` ignores any file at
the default location; `DSOPS_USER_CONFIG=/ci/stores.yaml dsops plan` errors
if that file does not exist.

**Acceptance Scenarios**:

1. **Given** `--user-config none` or `DSOPS_USER_CONFIG=none`, **When** dsops
   runs, **Then** no user config is consulted.
2. **Given** an explicit `--user-config` or `DSOPS_USER_CONFIG` path that does
   not exist, **When** dsops loads config, **Then** it fails with a
   configuration error naming the path.
3. **Given** `DSOPS_CONFIG=prod.yaml` (long documented, previously ignored),
   **When** `--config` is not passed, **Then** `prod.yaml` is the project
   config.

---

### Edge Cases

- User file exists but is empty: accepted, contributes nothing.
- User file declares `envs:`, `services:`, `templates:`, `transforms:`,
  `policies:`, `notifications:`, `metrics:` or an unknown/typo key: rejected
  with an error listing every offending key.
- User file `version` is not `0`: rejected.
- User file path is a directory: rejected.
- User file or its parent directory is group/world-writable: warning (not
  error), because whoever can edit it can redirect secret resolution.
- Same name in the user file's `secretStores:` and `providers:`: rejected.
- Name collides with a project *service* or legacy *provider*: project wins
  (all three share the resolver namespace).
- Relative `XDG_CONFIG_HOME`: ignored per the XDG spec.
- Windows: `%APPDATA%\dsops\config.yaml`; permission check skipped.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: dsops MUST discover a user config in this order: `--user-config`
  flag, `DSOPS_USER_CONFIG`, `$XDG_CONFIG_HOME/dsops/config.yaml`,
  `~/.config/dsops/config.yaml` on non-Windows systems (including macOS),
  `%APPDATA%\dsops\config.yaml` on Windows.
- **FR-002**: The value `none` (case-insensitive) for the flag or env var MUST
  disable user config loading.
- **FR-003**: A missing file at the default location MUST be silent; a missing
  file at an explicit (flag/env) path MUST be a configuration error.
- **FR-004**: The user file MUST accept only `version`, `secretStores:` and
  legacy `providers:`; any other top-level key MUST be rejected with an error
  naming the key(s) and the file.
- **FR-005**: On a name collision with any project `secretStores`, `services`
  or `providers` entry, the project definition MUST win and the user entry
  MUST be recorded as shadowed.
- **FR-006**: dsops MUST record provenance (`project`/`user` + file path) for
  every configured store name and expose it in `dsops doctor` and
  `dsops providers`.
- **FR-007**: Missing-store errors MUST name both the project file and, when
  user config is enabled, the user file path (even if not yet created).
- **FR-008**: dsops MUST warn when the user file (symlink target) or its parent
  directory is writable by group or others; it MUST NOT check ownership.
- **FR-009**: The project `dsops.yaml` MUST remain required; the user file is
  never a substitute for it.
- **FR-010**: With no user config present, behaviour MUST be unchanged.
- **FR-011**: `--config` MUST default to `DSOPS_CONFIG` when set.
- **FR-012**: Configuration discovery MUST happen in the CLI layer, not in
  `config.Load()`, so a `Config` built directly (tests, embedding) never reads
  the real home directory.

### Key Entities

- **UserConfigSpec**: `{Path, Origin}` input to `Load()`; `Origin` is one of
  none/flag/env/default and decides whether absence is an error.
- **StoreSource**: `{Scope: project|user, Path}` provenance recorded per store
  name.
- **userConfigFile**: the restricted schema (`version`, `secretStores`,
  `providers`).

### Assumptions

- Both config files hold references and connection details only; secret
  values never appear in them (constitution Principle I).
- Provider CLIs (`bw`, `vault`, `aws`, `op`) keep their own machine-level auth
  state; dsops inherits it as before.

### Out of Scope (Explicit Non-Goals)

- `requires:` / type expectations in the project file (a project cannot yet
  assert "`work-vault` must be a `vault`"). Reserved shape: a separate
  top-level `requires:` key so older loaders ignore it.
- `dsops init --user` scaffolding (must respect nix-store symlinks).
- Deep merge of individual store fields between files.
- `${VAR}` substitution inside config values.
- Widening the `secretStores` type allowlist (`keychain`, `infisical`,
  `akeyless`, `bitwarden.secretsmanager` still need legacy `providers:`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: All pre-existing tests pass unchanged.
- **SC-002**: `internal/config` coverage stays ≥85% (was 83.0% before; 88%+
  after).
- **SC-003**: A project with zero declared stores resolves every reference via
  the user config (`examples/portable-project.yaml` +
  `examples/user-config.yaml`).
- **SC-004**: `dsops doctor` identifies each store's source without the user
  opening either file.
- **SC-005**: A contributor with no user config gets an error that names the
  file to create.
