# SPEC-015: Vault Provider

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Provider Type**: `vault/vault`
**Related**:
- VISION.md Section 5 (Secret Stores & Services)
- SPEC-002: Configuration Parsing
- SPEC-003: Secret Resolution Engine
- SPEC-005: Provider Registry

## Summary

The Vault provider enables secret retrieval from vault/vault secret management system. Implementation uses Direct integration for authentication and secret fetching. This provider supports [TODO: List key capabilities].

## User Stories (As Built)

### User Story 1: Authenticate with Vault (P1)

Users authenticate with Vault using HashiCorp Vault token (from VAULT_TOKEN environment variable).

**Why this priority**: Authentication is prerequisite for all secret operations. Without auth, provider cannot function.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid credentials, **Then** authentication succeeds
2. **Given** invalid credentials, **Then** clear error message with remediation steps
3. **Given** network failure, **Then** timeout with retry suggestion


### User Story 2: Fetch Secrets from Vault (P1)

Users reference secrets using `store://vault/vault/path` URI format.

**Why this priority**: Core functionality. Enables secret resolution from Vault.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid secret reference, **Then** secret value returned
2. **Given** non-existent secret, **Then** error indicates secret not found
3. **Given** insufficient permissions, **Then** error explains permission issue


### User Story 3: Handle Vault-Specific Features (P2)

Users leverage Vault-specific capabilities ([Provider-specific features]).

**Acceptance Criteria** (✅ Validated):


## Implementation

### Architecture

**Key Files**:
- `internal/providers/vault/vault.go` - Provider implementation
- `internal/providers/vault/vault_test.go` - Test suite
- `internal/providers/registry.go` - Provider registration
- `pkg/provider/provider.go` - Provider interface

### Provider Interface Implementation

```go
type VaultProvider struct {
    [TODO: Add struct fields]
}

func (p *VaultProvider) Name() string {
    return "vault/vault"
}

func (p *VaultProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    [TODO: Describe resolution logic]
}

func (p *VaultProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
    [TODO: Describe metadata logic]
}

func (p *VaultProvider) Capabilities() provider.Capabilities {
    return provider.Capabilities{
        [TODO: List capabilities]
    }
}

func (p *VaultProvider) Validate(ctx context.Context) error {
    [TODO: Describe validation logic]
}
```

### Configuration Schema

```yaml
secretStores:
  vault/vault-dev:
    type: vault/vault
    address: value  # Vault server address
    token: value  # Vault token (discouraged, use env var)
    auth_method: value  # Authentication method: token, userpass, ldap, aws, k8s
    namespace: value  # Vault namespace (Vault Enterprise)
    userpass_username: value  # For userpass auth
    userpass_password: value  # For userpass auth (discouraged)
    ldap_username: value  # For LDAP auth
    ldap_password: value  # For LDAP auth (discouraged)
    aws_role: value  # For AWS auth
    k8s_role: value  # For Kubernetes auth
    ca_cert: value  # Path to CA certificate
    client_cert: value  # Path to client certificate
    client_key: value  # Path to client key
    tls_skip: value  # Skip TLS verification (not recommended)
```

**Configuration Fields**:
[TODO: Add config fields table]

### Authentication Method

[TODO: Add auth details]

**Authentication Flow**:
1. [TODO]
2. [TODO]
3. [TODO]

### Secret Resolution

**Resolution Process**:
1. Parse reference URI (`store://vault/vault/path/to/secret`)
2. Extract path/key components
3. [TODO]
4. [TODO]
5. Return secret value with metadata

**Path Format**: [TODO: Describe path format]

**Examples**:
[TODO: Add path examples]

### Capabilities

| Capability | Supported | Notes |
|------------|-----------|-------|
| Versioning | ✅ | [TODO] |
| Metadata | ✅ | [TODO] |
| List Secrets | ❌ | [TODO] |
| Rotation | ❌ | [TODO] |
| Encryption at Rest | ✅ | [TODO] |

## Design Decisions

[TODO: Add design decisions]

**Trade-offs**:
[TODO: Add tradeoffs]

## Security Considerations

- **Credential Storage**: [TODO]
- **Network Security**: [TODO]
- **Secret Lifetime**: [TODO]
- **Audit Trail**: [TODO]


## Testing

**Test Coverage**: N/A%

**Test Files**:
- `internal/providers/vault/vault_test.go` - Unit and integration tests


**Test Categories**:
- Authentication tests (valid/invalid credentials)
- Secret resolution tests (existing/missing secrets)
- Error handling tests (network failures, timeouts)
- 

**Integration Testing**:
[TODO: Add integration test notes]

## Documentation

- **Provider Guide**: `docs/content/providers/vault/vault.md`
- **Configuration Reference**: `docs/content/reference/providers.md#vault/vault`
- **Examples**: 

**Example Configuration**:
[TODO: Add example config]

## Lessons Learned

**What Went Well**:
[TODO: What went well]

**What Could Be Improved**:
[TODO: What could improve]

**Vault-Specific Notes**:
[TODO: Provider-specific notes]

## Future Enhancements (v0.2+)

[TODO: Future enhancements]

## Related Specifications

- **SPEC-001**: CLI Framework (provider commands)
- **SPEC-002**: Configuration Parsing (provider config schema)
- **SPEC-003**: Secret Resolution Engine (resolution pipeline)
- **SPEC-005**: Provider Registry (provider registration)
- **SPEC-008**: Doctor Command (provider validation)

