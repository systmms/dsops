# SPEC-{{SPEC_NUMBER}}: {{PROVIDER_NAME}} Provider

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: {{IMPL_DATE}}
**Provider Type**: `{{PROVIDER_TYPE}}`
**Related**:
- VISION.md Section 5 (Secret Stores & Services)
- SPEC-002: Configuration Parsing
- SPEC-003: Secret Resolution Engine
- SPEC-005: Provider Registry

## Summary

The {{PROVIDER_NAME}} provider enables secret retrieval from {{PROVIDER_DESCRIPTION}}. Implementation uses {{INTEGRATION_METHOD}} for authentication and secret fetching. This provider supports {{CAPABILITIES_SUMMARY}}.

## User Stories (As Built)

### User Story 1: Authenticate with {{PROVIDER_NAME}} (P1)

Users authenticate with {{PROVIDER_NAME}} using {{AUTH_METHOD}}.

**Why this priority**: Authentication is prerequisite for all secret operations. Without auth, provider cannot function.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid {{AUTH_CREDENTIAL}}, **Then** authentication succeeds
2. **Given** invalid credentials, **Then** clear error message with remediation steps
3. **Given** network failure, **Then** timeout with retry suggestion
{{AUTH_TESTS}}

### User Story 2: Fetch Secrets from {{PROVIDER_NAME}} (P1)

Users reference secrets using `store://{{PROVIDER_TYPE}}/path` URI format.

**Why this priority**: Core functionality. Enables secret resolution from {{PROVIDER_NAME}}.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** valid secret reference, **Then** secret value returned
2. **Given** non-existent secret, **Then** error indicates secret not found
3. **Given** insufficient permissions, **Then** error explains permission issue
{{FETCH_TESTS}}

### User Story 3: Handle {{PROVIDER_NAME}}-Specific Features (P2)

Users leverage {{PROVIDER_NAME}}-specific capabilities ({{SPECIAL_FEATURES}}).

**Acceptance Criteria** (✅ Validated):
{{SPECIAL_FEATURES_TESTS}}

## Implementation

### Architecture

**Key Files**:
- `{{MAIN_FILE}}` - Provider implementation
- `{{MAIN_FILE_TEST}}` - Test suite
- `{{REGISTRY_ENTRY}}` - Provider registration
- `pkg/provider/provider.go` - Provider interface

### Provider Interface Implementation

```go
type {{PROVIDER_STRUCT_NAME}} struct {
    {{STRUCT_FIELDS}}
}

func (p *{{PROVIDER_STRUCT_NAME}}) Name() string {
    return "{{PROVIDER_TYPE}}"
}

func (p *{{PROVIDER_STRUCT_NAME}}) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    {{RESOLVE_IMPLEMENTATION}}
}

func (p *{{PROVIDER_STRUCT_NAME}}) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
    {{DESCRIBE_IMPLEMENTATION}}
}

func (p *{{PROVIDER_STRUCT_NAME}}) Capabilities() provider.Capabilities {
    return provider.Capabilities{
        {{CAPABILITIES}}
    }
}

func (p *{{PROVIDER_STRUCT_NAME}}) Validate(ctx context.Context) error {
    {{VALIDATE_IMPLEMENTATION}}
}
```

### Configuration Schema

```yaml
secretStores:
  {{EXAMPLE_STORE_NAME}}:
    type: {{PROVIDER_TYPE}}
{{CONFIG_FIELDS_YAML}}
```

**Configuration Fields**:
{{CONFIG_FIELDS_TABLE}}

### Authentication Method

{{AUTH_DETAILS}}

**Authentication Flow**:
1. {{AUTH_STEP_1}}
2. {{AUTH_STEP_2}}
3. {{AUTH_STEP_3}}

### Secret Resolution

**Resolution Process**:
1. Parse reference URI (`store://{{PROVIDER_TYPE}}/{{PATH_EXAMPLE}}`)
2. Extract path/key components
3. {{RESOLUTION_STEP_3}}
4. {{RESOLUTION_STEP_4}}
5. Return secret value with metadata

**Path Format**: {{PATH_FORMAT_DESCRIPTION}}

**Examples**:
{{PATH_EXAMPLES}}

### Capabilities

| Capability | Supported | Notes |
|------------|-----------|-------|
| Versioning | {{CAP_VERSIONING}} | {{CAP_VERSIONING_NOTES}} |
| Metadata | {{CAP_METADATA}} | {{CAP_METADATA_NOTES}} |
| List Secrets | {{CAP_LIST}} | {{CAP_LIST_NOTES}} |
| Rotation | {{CAP_ROTATION}} | {{CAP_ROTATION_NOTES}} |
| Encryption at Rest | {{CAP_ENCRYPTION}} | {{CAP_ENCRYPTION_NOTES}} |

## Design Decisions

{{DESIGN_DECISIONS}}

**Trade-offs**:
{{TRADEOFFS}}

## Security Considerations

- **Credential Storage**: {{CREDENTIAL_STORAGE}}
- **Network Security**: {{NETWORK_SECURITY}}
- **Secret Lifetime**: {{SECRET_LIFETIME}}
- **Audit Trail**: {{AUDIT_TRAIL}}
{{SECURITY_NOTES}}

## Testing

**Test Coverage**: {{TEST_COVERAGE}}%

**Test Files**:
- `{{MAIN_FILE_TEST}}` - {{TEST_DESCRIPTION}}
{{ADDITIONAL_TEST_FILES}}

**Test Categories**:
- Authentication tests (valid/invalid credentials)
- Secret resolution tests (existing/missing secrets)
- Error handling tests (network failures, timeouts)
- {{PROVIDER_SPECIFIC_TESTS}}

**Integration Testing**:
{{INTEGRATION_TEST_NOTES}}

## Documentation

- **Provider Guide**: `docs/content/providers/{{PROVIDER_SLUG}}.md`
- **Configuration Reference**: `docs/content/reference/providers.md#{{PROVIDER_TYPE}}`
- **Examples**: {{EXAMPLE_FILES}}

**Example Configuration**:
{{EXAMPLE_CONFIG_SNIPPET}}

## Lessons Learned

**What Went Well**:
{{LESSONS_GOOD}}

**What Could Be Improved**:
{{LESSONS_IMPROVE}}

**{{PROVIDER_NAME}}-Specific Notes**:
{{PROVIDER_NOTES}}

## Future Enhancements (v0.2+)

{{FUTURE_ENHANCEMENTS}}

## Related Specifications

- **SPEC-001**: CLI Framework (provider commands)
- **SPEC-002**: Configuration Parsing (provider config schema)
- **SPEC-003**: Secret Resolution Engine (resolution pipeline)
- **SPEC-005**: Provider Registry (provider registration)
- **SPEC-010**: Doctor Command (provider validation)
{{RELATED_SPECS}}
