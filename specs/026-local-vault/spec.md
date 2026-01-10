# Feature Specification: Local Encrypted Vault Provider

**Feature Branch**: `026-local-vault`
**Created**: 2026-01-10
**Status**: Draft
**Input**: User description: "Add a native local encrypted vault provider that stores secrets locally with AES-256-GCM encryption and OS keychain integration for air-gapped, offline-capable secret storage."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Air-Gapped Secret Storage (Priority: P1)

As a developer working in an air-gapped environment without network access to cloud secret managers, I want to store and retrieve secrets from an encrypted local vault so that I can securely manage credentials offline.

**Why this priority**: Air-gapped environments are a primary use case - users literally cannot use cloud providers.

**Independent Test**: Can be fully tested by initializing a vault, storing a secret, disconnecting from network, and verifying `dsops plan` still resolves the secret.

**Acceptance Scenarios**:

1. **Given** a local vault is initialized, **When** storing a secret with `dsops vault set API_KEY`, **Then** the secret is encrypted and saved to the vault file.

2. **Given** a secret exists in the local vault, **When** running `dsops plan --env dev` with no network access, **Then** the secret is resolved successfully.

3. **Given** the vault file doesn't exist, **When** trying to resolve secrets, **Then** a clear error message indicates the vault needs initialization.

---

### User Story 2 - Quick Local Setup (Priority: P1)

As a solo developer starting a new project, I want to quickly set up local secrets without configuring cloud services so that I can begin development immediately.

**Why this priority**: Reduces friction for new users - no external account setup required.

**Independent Test**: Can be tested by running `dsops vault init` and `dsops vault set` in under 1 minute with no prior configuration.

**Acceptance Scenarios**:

1. **Given** a new project with no dsops configuration, **When** running `dsops vault init`, **Then** a vault is created with encryption key stored in OS keychain.

2. **Given** an initialized vault, **When** running `dsops vault set DATABASE_URL`, **Then** an interactive prompt securely captures and encrypts the value.

3. **Given** multiple secrets stored, **When** running `dsops vault list`, **Then** all secret names are displayed (values remain encrypted).

---

### User Story 3 - Migration Path to Enterprise (Priority: P2)

As a team lead, I want to bootstrap projects with local secrets and later migrate to 1Password so that we can start development immediately while enterprise tooling is being provisioned.

**Why this priority**: Supports gradual adoption - teams can start with local vault and upgrade later.

**Independent Test**: Can be tested by configuring both local and 1Password providers in the same dsops.yaml and switching environment references.

**Acceptance Scenarios**:

1. **Given** secrets in local vault, **When** configuring the same secret names in 1Password, **Then** switching `store://` references migrates without code changes.

2. **Given** both local and cloud providers configured, **When** running `dsops exec --env dev`, **Then** local secrets are used for development while production uses cloud.

---

### User Story 4 - Password Fallback (Priority: P2)

As a developer on a system without OS keychain support (headless server, container), I want to use password-based encryption so that I can still use local vault functionality.

**Why this priority**: Expands compatibility to environments without keychain access.

**Independent Test**: Can be tested by initializing vault with `--no-keychain` flag and verifying password prompt works.

**Acceptance Scenarios**:

1. **Given** OS keychain is unavailable, **When** running `dsops vault init`, **Then** user is prompted for a master password.

2. **Given** vault uses password-based encryption, **When** resolving secrets, **Then** password is requested (or read from environment variable).

---

### Edge Cases

- What happens when keychain access is denied by the OS?
- How does the system handle concurrent write operations to the vault?
- What happens when the vault file is corrupted or tampered with?
- How does password-based encryption work in non-interactive CI environments?
- What happens when migrating vault between different operating systems?

## Requirements *(mandatory)*

### Functional Requirements

#### Provider Requirements
- **FR-001**: System MUST support `local` as a provider type in secretStores configuration
- **FR-002**: System MUST resolve secrets from local vault using `store://` reference syntax
- **FR-003**: System MUST support optional `path` configuration for vault file location (default: `.dsops/vault.enc`)
- **FR-004**: System MUST support optional `keychain` configuration (default: true)

#### Encryption Requirements
- **FR-005**: System MUST encrypt all secrets at rest using authenticated encryption
- **FR-006**: System MUST store encryption key in OS keychain when available and enabled
- **FR-007**: System MUST support password-based key derivation when keychain is unavailable or disabled
- **FR-008**: System MUST validate vault integrity on read (detect tampering)

#### CLI Requirements
- **FR-009**: System MUST provide `dsops vault init` command to create a new vault
- **FR-010**: System MUST provide `dsops vault set <name>` command to add/update secrets
- **FR-011**: System MUST provide `dsops vault list` command to show secret names
- **FR-012**: System MUST provide `dsops vault delete <name>` command to remove secrets
- **FR-013**: System MUST use secure interactive prompts for secret input (no echo)

#### Operational Requirements
- **FR-014**: System MUST work completely offline without network access
- **FR-015**: System MUST handle concurrent read access safely
- **FR-016**: System MUST use file locking for write operations to prevent corruption

### Key Entities

- **LocalVaultProvider**: The provider implementation for local encrypted storage
- **Vault**: The encrypted container holding multiple secrets with integrity verification
- **VaultEntry**: A single secret with name, encrypted value, and metadata
- **KeychainKey**: The encryption key stored in OS keychain (or derived from password)

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can initialize a vault and store their first secret in under 60 seconds
- **SC-002**: Secret operations (read, write) complete within 500ms for vaults with up to 100 secrets
- **SC-003**: Vault works identically on macOS, Linux, and Windows (cross-platform compatibility)
- **SC-004**: All dsops commands (plan, exec, doctor) work correctly with local provider
- **SC-005**: Encrypted vault files are unreadable without the encryption key
- **SC-006**: Tampering with vault file is detected and reported as an error

## Assumptions

- OS keychain is available on most developer workstations (macOS Keychain, GNOME Keyring, Windows Credential Manager)
- Users in environments without keychain (CI, containers) can use password-based encryption
- Vault file format is dsops-specific and not intended to be compatible with other tools
- Single-user access is the primary use case (team sharing requires different solutions)
- Password-based encryption in CI can use `DSOPS_VAULT_PASSWORD` environment variable
