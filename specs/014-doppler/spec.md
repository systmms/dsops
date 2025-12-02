# SPEC-014: Doppler Provider

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Provider Type**: `doppler`
**Related**:
- VISION.md Section 5 (Secret Stores & Services)
- SPEC-002: Configuration Parsing
- SPEC-003: Secret Resolution Engine
- SPEC-005: Provider Registry

## Summary

The Doppler provider enables secret retrieval from Doppler. Implementation uses CLI wrapper (executes doppler command-line tool) for authentication and secret fetching. This provider supports [TODO: List key capabilities].

## User Stories (As Built)

### User Story 1: Authenticate with Doppler (P1)

Users authenticate with Doppler using Provider-specific authentication method.

**Why this priority**: Authentication is prerequisite for all secret operations. Without auth, provider cannot function.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid credentials, **Then** authentication succeeds
2. **Given** invalid credentials, **Then** clear error message with remediation steps
3. **Given** network failure, **Then** timeout with retry suggestion


### User Story 2: Fetch Secrets from Doppler (P1)

Users reference secrets using `store://doppler/path` URI format.

**Why this priority**: Core functionality. Enables secret resolution from Doppler.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid secret reference, **Then** secret value returned
2. **Given** non-existent secret, **Then** error indicates secret not found
3. **Given** insufficient permissions, **Then** error explains permission issue


### User Story 3: Handle Doppler-Specific Features (P2)

Users leverage Doppler-specific capabilities ([Provider-specific features]).

**Acceptance Criteria** (✅ Validated):


## Implementation

### Architecture

**Key Files**:
- `internal/providers/doppler.go` - Provider implementation
- `internal/providers/doppler_test.go` - Test suite
- `internal/providers/registry.go` - Provider registration
- `pkg/provider/provider.go` - Provider interface

### Provider Interface Implementation

```go
type DopplerProvider struct {
    [TODO: Add struct fields]
}

func (p *DopplerProvider) Name() string {
    return "doppler"
}

func (p *DopplerProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    [TODO: Describe resolution logic]
}

func (p *DopplerProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
    [TODO: Describe metadata logic]
}

func (p *DopplerProvider) Capabilities() provider.Capabilities {
    return provider.Capabilities{
        [TODO: List capabilities]
    }
}

func (p *DopplerProvider) Validate(ctx context.Context) error {
    [TODO: Describe validation logic]
}
```

### Configuration Schema

```yaml
secretStores:
  doppler-dev:
    type: doppler
    token,omitempty: value  # Service token
    project,omitempty: value  # Project name
    config,omitempty: value  # Config/environment name
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
1. Parse reference URI (`store://doppler/path/to/secret`)
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

**Test Coverage**: N/A%

**Test Files**:
- `internal/providers/doppler_test.go` - Unit and integration tests


**Test Categories**:
- Authentication tests (valid/invalid credentials)
- Secret resolution tests (existing/missing secrets)
- Error handling tests (network failures, timeouts)
- 

**Integration Testing**:
[TODO: Add integration test notes]

## Documentation

- **Provider Guide**: `docs/content/providers/doppler.md`
- **Configuration Reference**: `docs/content/reference/providers.md#doppler`
- **Examples**: examples/doppler.yaml,examples/doppler-copy.yaml,examples/doppler-simple.yaml,examples/doppler-test.yaml

**Example Configuration**:
[TODO: Add example config]

## Lessons Learned

**What Went Well**:
[TODO: What went well]

**What Could Be Improved**:
[TODO: What could improve]

**Doppler-Specific Notes**:
[TODO: Provider-specific notes]

## Future Enhancements (v0.2+)

[TODO: Future enhancements]

## Related Specifications

- **SPEC-001**: CLI Framework (provider commands)
- **SPEC-002**: Configuration Parsing (provider config schema)
- **SPEC-003**: Secret Resolution Engine (resolution pipeline)
- **SPEC-005**: Provider Registry (provider registration)
- **SPEC-008**: Doctor Command (provider validation)

