# SPEC-081: OnePassword Provider

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Provider Type**: `onepassword`
**Related**:
- VISION.md Section 5 (Secret Stores & Services)
- SPEC-002: Configuration Parsing
- SPEC-003: Secret Resolution Engine
- SPEC-005: Provider Registry

## Summary

The OnePassword provider enables secret retrieval from OnePasswordProvider implements the provider.Provider interface for 1Password CLI. Implementation uses CLI wrapper (executes onepassword command-line tool) for authentication and secret fetching. This provider supports [TODO: List key capabilities].

## User Stories (As Built)

### User Story 1: Authenticate with OnePassword (P1)

Users authenticate with OnePassword using CLI authentication (requires onepassword CLI to be logged in).

**Why this priority**: Authentication is prerequisite for all secret operations. Without auth, provider cannot function.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid credentials, **Then** authentication succeeds
2. **Given** invalid credentials, **Then** clear error message with remediation steps
3. **Given** network failure, **Then** timeout with retry suggestion


### User Story 2: Fetch Secrets from OnePassword (P1)

Users reference secrets using `store://onepassword/path` URI format.

**Why this priority**: Core functionality. Enables secret resolution from OnePassword.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid secret reference, **Then** secret value returned
2. **Given** non-existent secret, **Then** error indicates secret not found
3. **Given** insufficient permissions, **Then** error explains permission issue


### User Story 3: Handle OnePassword-Specific Features (P2)

Users leverage OnePassword-specific capabilities ([Provider-specific features]).

**Acceptance Criteria** (✅ Validated):


## Implementation

### Architecture

**Key Files**:
- `internal/providers/onepassword.go` - Provider implementation
- `internal/providers/onepassword_test.go` - Test suite
- `internal/providers/registry.go` - Provider registration
- `pkg/provider/provider.go` - Provider interface

### Provider Interface Implementation

```go
type OnePasswordProvider struct {
    [TODO: Add struct fields]
}

func (p *OnePasswordProvider) Name() string {
    return "onepassword"
}

func (p *OnePasswordProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    [TODO: Describe resolution logic]
}

func (p *OnePasswordProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
    [TODO: Describe metadata logic]
}

func (p *OnePasswordProvider) Capabilities() provider.Capabilities {
    return provider.Capabilities{
        [TODO: List capabilities]
    }
}

func (p *OnePasswordProvider) Validate(ctx context.Context) error {
    [TODO: Describe validation logic]
}
```

### Configuration Schema

```yaml
secretStores:
  onepassword-dev:
    type: onepassword
    # Provider-specific configuration fields
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
1. Parse reference URI (`store://onepassword/path/to/secret`)
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
| Versioning | ❌ | [TODO] |
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

**Test Coverage**: 0.0%

**Test Files**:
- `internal/providers/onepassword_test.go` - Unit and integration tests


**Test Categories**:
- Authentication tests (valid/invalid credentials)
- Secret resolution tests (existing/missing secrets)
- Error handling tests (network failures, timeouts)
- 

**Integration Testing**:
[TODO: Add integration test notes]

## Documentation

- **Provider Guide**: `docs/content/providers/onepassword.md`
- **Configuration Reference**: `docs/content/reference/providers.md#onepassword`
- **Examples**: 

**Example Configuration**:
[TODO: Add example config]

## Lessons Learned

**What Went Well**:
[TODO: What went well]

**What Could Be Improved**:
[TODO: What could improve]

**OnePassword-Specific Notes**:
[TODO: Provider-specific notes]

## Future Enhancements (v0.2+)

[TODO: Future enhancements]

## Related Specifications

- **SPEC-001**: CLI Framework (provider commands)
- **SPEC-002**: Configuration Parsing (provider config schema)
- **SPEC-003**: Secret Resolution Engine (resolution pipeline)
- **SPEC-005**: Provider Registry (provider registration)
- **SPEC-010**: Doctor Command (provider validation)

