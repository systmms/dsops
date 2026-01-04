# Feature Specification: New Secret Store Providers

**Feature Branch**: `021-new-providers`
**Created**: 2026-01-03
**Status**: Draft
**Provider Types**: `keychain`, `infisical`, `akeyless`

## Summary

Add three new secret store providers to dsops, expanding coverage across local development, open-source SaaS, and enterprise use cases:

| Provider | Type | Score | Primary Use Case |
|----------|------|-------|------------------|
| OS Keychain | `keychain` | 8.25 | Local dev, offline workflows |
| Infisical | `infisical` | 8.5 | Open-source SaaS alternative to Doppler |
| Akeyless | `akeyless` | 8.2 | Enterprise zero-knowledge platform |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Retrieve Secrets from OS Keychain (Priority: P1)

A developer wants to use credentials stored in their operating system's native keychain within dsops workflows, enabling local development without external secret services.

**Why this priority**: Enables offline development and provides zero-dependency secret storage for all macOS/Linux users.

**Independent Test**: Can be fully tested by storing a test credential in the OS keychain, configuring dsops to reference it, and verifying resolution.

**Acceptance Scenarios**:

1. **Given** a secret exists in the OS keychain with service "myapp" and account "api-key", **When** dsops resolves `store://keychain/myapp/api-key`, **Then** the secret value is returned
2. **Given** the secret does not exist in the keychain, **When** dsops attempts to resolve, **Then** a clear error indicates the secret was not found
3. **Given** the user denies keychain access, **When** dsops attempts to resolve, **Then** the error indicates access was denied with remediation steps
4. **Given** dsops runs on an unsupported platform (Windows), **When** configuration loads, **Then** a clear error indicates the platform is not supported

---

### User Story 2 - Retrieve Secrets from Infisical (Priority: P1)

A developer wants to retrieve secrets from Infisical (self-hosted or cloud) to use in their dsops workflows, providing an open-source alternative to proprietary secret management services.

**Why this priority**: Fills strategic gap between self-hosted Vault and proprietary Doppler; high community adoption (12k+ GitHub stars).

**Independent Test**: Can be fully tested by configuring an Infisical project with test secrets and verifying dsops retrieves them correctly.

**Acceptance Scenarios**:

1. **Given** valid Infisical credentials and a secret exists at path "DATABASE_URL", **When** dsops resolves `store://infisical/DATABASE_URL`, **Then** the secret value is returned
2. **Given** valid credentials but the secret does not exist, **When** dsops attempts to resolve, **Then** a clear error indicates the secret was not found
3. **Given** invalid or expired credentials, **When** dsops attempts to resolve, **Then** the error indicates authentication failed with remediation steps
4. **Given** a self-hosted Infisical instance, **When** configured with custom host URL, **Then** dsops connects to the self-hosted instance

---

### User Story 3 - Retrieve Secrets from Akeyless (Priority: P1)

A developer wants to retrieve secrets from Akeyless to use in dsops workflows, providing enterprise-grade zero-knowledge secret management for regulated environments.

**Why this priority**: Addresses enterprise market with FIPS 140-2 certification and multiple authentication methods.

**Independent Test**: Can be fully tested by configuring Akeyless with test secrets and verifying dsops retrieves them correctly.

**Acceptance Scenarios**:

1. **Given** valid Akeyless credentials and a secret exists at "/prod/database/password", **When** dsops resolves `store://akeyless/prod/database/password`, **Then** the secret value is returned
2. **Given** valid credentials but the secret path does not exist, **When** dsops attempts to resolve, **Then** a clear error indicates the secret was not found
3. **Given** invalid credentials, **When** dsops attempts to resolve, **Then** the error indicates authentication failed with remediation steps
4. **Given** an Akeyless configuration using cloud IAM authentication, **When** running in that cloud environment, **Then** authentication succeeds without explicit credentials

---

### User Story 4 - Configure Providers in dsops.yaml (Priority: P1)

A developer configures one or more of the new providers in their dsops configuration file with appropriate settings for their environment.

**Why this priority**: Configuration is required for all providers to function.

**Independent Test**: Can be tested by creating dsops.yaml configurations and validating they parse correctly.

**Acceptance Scenarios**:

1. **Given** a valid keychain provider configuration, **When** dsops loads the configuration, **Then** the provider is registered and ready for use
2. **Given** a valid Infisical provider configuration with project_id and environment, **When** dsops loads the configuration, **Then** the provider is registered
3. **Given** a valid Akeyless provider configuration with access_id, **When** dsops loads the configuration, **Then** the provider is registered
4. **Given** an invalid configuration (missing required fields), **When** dsops loads the configuration, **Then** a clear validation error identifies what is missing

---

### User Story 5 - Validate Provider Health with Doctor Command (Priority: P2)

A developer uses `dsops doctor` to verify their new providers are properly configured and accessible.

**Why this priority**: Diagnostics help users troubleshoot configuration issues before they cause workflow failures.

**Independent Test**: Can be tested by running `dsops doctor` with each provider configured.

**Acceptance Scenarios**:

1. **Given** a keychain provider configuration, **When** running `dsops doctor`, **Then** the provider shows platform compatibility and access status
2. **Given** an Infisical provider configuration, **When** running `dsops doctor`, **Then** the provider shows connection status and authentication result
3. **Given** an Akeyless provider configuration, **When** running `dsops doctor`, **Then** the provider shows connection status and authentication method
4. **Given** a provider with invalid configuration, **When** running `dsops doctor`, **Then** specific remediation steps are provided

---

### Edge Cases

- **Keychain locked**: Should trigger unlock prompt or return "keychain locked" error with unlock instructions
- **Headless environment**: Keychain provider should detect headless mode and return appropriate error
- **Network timeout**: Infisical/Akeyless should timeout after 30 seconds (configurable) with retry suggestions
- **Token expiration**: Should provide clear error when tokens expire with refresh instructions
- **Self-signed certificates**: Infisical/Akeyless should support custom CA certificates for self-hosted instances
- **Rate limiting**: Should handle rate limit responses gracefully with backoff suggestions
- **Secret versioning**: Infisical/Akeyless should support requesting specific secret versions

## Requirements *(mandatory)*

### Functional Requirements

**OS Keychain Provider:**
- **FR-001**: System MUST retrieve secret values from macOS Keychain using service name and account name
- **FR-002**: System MUST retrieve secret values from Linux Secret Service using service and account identifiers
- **FR-003**: System MUST support OS-native authentication prompts (Touch ID, password) for protected items
- **FR-004**: System MUST detect unsupported platforms and return clear error messages

**Infisical Provider:**
- **FR-005**: System MUST authenticate with Infisical using machine identity, service token, or API key
- **FR-006**: System MUST retrieve secrets from Infisical by secret name within project/environment scope
- **FR-007**: System MUST support both cloud-hosted and self-hosted Infisical instances
- **FR-008**: System MUST support retrieving specific secret versions when requested

**Akeyless Provider:**
- **FR-009**: System MUST authenticate with Akeyless using API key, SAML, OIDC, or cloud IAM methods
- **FR-010**: System MUST retrieve secrets from Akeyless using path-based references
- **FR-011**: System MUST support custom gateway URLs for enterprise deployments
- **FR-012**: System MUST support retrieving specific secret versions when requested

**Shared Requirements:**
- **FR-013**: All providers MUST return clear error messages with actionable remediation steps
- **FR-014**: All providers MUST report health status through the doctor command
- **FR-015**: All providers MUST validate configuration at load time and fail fast on invalid config
- **FR-016**: Users MUST be able to reference secrets using standard URI format `store://provider/path`
- **FR-017**: All providers MUST cache authentication tokens in memory only (per-process); no disk-based credential persistence

### Configuration Schema

**OS Keychain:**
```yaml
secretStores:
  local:
    type: keychain
    service_prefix: "com.mycompany"      # Optional: default service name prefix
    access_group: "TEAMID.shared"        # Optional: macOS keychain access group
```

**Infisical:**
```yaml
secretStores:
  infisical-prod:
    type: infisical
    host: "https://app.infisical.com"    # Optional: defaults to cloud
    project_id: "proj_abc123"            # Required
    environment: "production"            # Required
    auth:
      method: machine_identity           # or: service_token, api_key
      client_id: "${INFISICAL_CLIENT_ID}"
      client_secret: "${INFISICAL_CLIENT_SECRET}"
```

**Akeyless:**
```yaml
secretStores:
  akeyless-prod:
    type: akeyless
    access_id: "p-abc123"                # Required
    gateway_url: "https://gw.akeyless.io" # Optional: custom gateway
    auth:
      method: api_key                    # or: aws_iam, azure_ad, gcp, oidc, saml
      access_key: "${AKEYLESS_ACCESS_KEY}"
```

### Key Entities

- **Keychain Item**: Secret stored in OS credential store, identified by service name and account name
- **Infisical Secret**: Secret stored in Infisical, scoped to project and environment
- **Akeyless Secret**: Secret stored in Akeyless, identified by hierarchical path
- **Provider Configuration**: Settings defining how to connect to and authenticate with each secret store

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Users can retrieve secrets from all three providers in under 2 seconds (excluding auth prompt time)
- **SC-002**: Configuration validation catches 100% of invalid configurations before runtime
- **SC-003**: Error messages include actionable remediation steps for all common failure scenarios
- **SC-004**: `dsops doctor` correctly identifies provider health status for all three providers
- **SC-005**: Zero crashes or hangs when providers encounter network failures, auth failures, or unsupported platforms
- **SC-006**: All three providers pass the standard provider contract test suite

## Assumptions

- Users have accounts/access configured for Infisical and Akeyless before using those providers
- Users have keychain items already stored via OS-native tools (Keychain Access, seahorse)
- Linux systems have a Secret Service implementation running (gnome-keyring, KWallet) for keychain provider
- Network connectivity is available for Infisical and Akeyless (keychain works offline)
- Existing dsops configuration patterns apply (environment variables for sensitive values)

## Out of Scope

- Creating or modifying secrets (dsops is read-only for secrets)
- Windows Credential Manager support (may be added in future spec)
- Keychain iCloud synchronization
- Infisical/Akeyless secret rotation triggers (dsops reads only; rotation is managed by the platforms)
- Dynamic secret generation (Akeyless feature - may be added later)

## Dependencies

- Existing provider interface (`pkg/provider/provider.go`)
- Provider registry (`internal/providers/registry.go`)
- Doctor command integration (`cmd/dsops/commands/doctor.go`)
- Configuration parsing (`internal/config/`)

## Clarifications

### Session 2026-01-03

- Q: What should the default network timeout be for Infisical/Akeyless providers? → A: 30 seconds (standard API timeout, matches AWS SDK defaults)
- Q: Should providers cache authentication tokens between invocations? → A: Per-process only (cache tokens in memory during single dsops run, no disk persistence)

## Related Specifications

- **SPEC-001**: CLI Framework (provider commands)
- **SPEC-002**: Configuration Parsing (provider config schema)
- **SPEC-003**: Secret Resolution Engine (resolution pipeline)
- **SPEC-008**: Doctor Command (provider validation)
- **SPEC-015**: Vault Provider (similar enterprise pattern)
- **SPEC-016**: AWS Secrets Manager (similar cloud pattern)
