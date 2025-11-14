# SPEC-004: Transform Pipeline

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Related**:
- VISION.md Section 6 (Architecture - Transforms)
- SPEC-003: Secret Resolution Engine (applies transforms after resolution)

## Summary

The transform pipeline provides post-resolution value transformation through built-in functions like `json_extract`, `base64_decode`, `yaml_extract`, `trim`, and `multiline_to_single`. Transforms are composable, stateless, and fail-fast. Users can define named transform chains in config and apply them to individual variables. Transform errors include helpful debugging information (input sample, transform step, failure reason).

## User Stories (As Built)

### User Story 1: Extract JSON Fields (P1)

Users retrieve JSON secrets from providers and extract specific fields.

**Acceptance Criteria** (✅ Validated):
1. **Given** secret value `{"password":"secret123"}`, **When** transform `json_extract:.password`, **Then** result is `"secret123"`
2. **Given** nested JSON, **When** path `.data.credentials.key`, **Then** extracts nested value
3. **Given** invalid JSON, **Then** error explains JSON parse failure

**Implementation**: `internal/resolve/transforms.go:12-74`

### User Story 2: Base64 Decode (P1)

Users decode base64-encoded secrets.

**Acceptance Criteria** (✅ Validated):
1. **Given** base64 string, **When** transform `base64_decode`, **Then** decoded value returned
2. **Given** invalid base64, **Then** error explains decode failure

**Implementation**: `internal/resolve/transforms.go:76-82`

### User Story 3: Composable Transforms (P2)

Users chain multiple transforms in sequence.

**Acceptance Criteria** (✅ Validated):
1. **Given** transforms `[base64_decode, json_extract:.password]`, **Then** applied in order
2. **Given** second transform fails, **Then** error shows which step failed
3. **Given** named transform, **Then** resolves to configured chain

## Implementation

### Built-In Transforms

| Transform | Description | Example |
|-----------|-------------|---------|
| `json_extract:.path` | Extract JSON field | `.data.password` |
| `yaml_extract:.path` | Extract YAML field | `.spec.secret` |
| `base64_decode` | Decode base64 | Decodes standard base64 |
| `base64_encode` | Encode base64 | Encodes to standard base64 |
| `trim` | Remove whitespace | Leading/trailing spaces |
| `multiline_to_single` | Collapse lines | Converts `\n` to spaces |
| `replace:from:to` | String replacement | `replace:dev:prod` |
| `join:delimiter` | Join array elements | `join:,` |

### Design Decisions

- **Pure Functions**: All transforms are stateless (same input → same output)
- **Fail-Fast**: Transform error stops pipeline immediately
- **Path Syntax**: JSON/YAML paths use `.field.nested` syntax (subset of JSONPath)
- **Named Transforms**: Users define reusable chains in config `transforms:` section
- **Error Context**: Errors include input sample (first 50 chars), transform name, specific failure

**Implementation**: `internal/resolve/transforms.go`

## Security Considerations

- **Input Sanitization**: Transform errors redact secret values from error messages
- **Memory Safety**: JSON/YAML parsing uses safe libraries (no buffer overflows)
- **No Side Effects**: Transforms cannot write files, make network calls, or access environment

## Testing

**Test Coverage**: 85%

**Test Files**:
- `internal/resolve/transforms_test.go` - Unit tests for each transform
- Integration tests via command test suites

## Lessons Learned

**What Went Well**:
- Simple path syntax (`.field.nested`) covers 95% of use cases
- Named transforms reduce config duplication
- Fail-fast errors help users debug transform issues quickly

**What Could Be Improved**:
- **JSONPath**: Current implementation is subset; full JSONPath would be more powerful
- **Array Indexing**: `json_extract:.array[0]` not yet implemented
- **Custom Transforms**: No plugin system for user-defined transforms

## Future Enhancements (v0.2+)

1. **Full JSONPath Support**: Implement complete JSONPath spec (array indexing, filters, wildcards)
2. **Custom Transform Plugins**: Load transforms from `~/.dsops/transforms/`
3. **Transform Validation**: Validate transform syntax at config parse time
4. **Transform Testing**: `dsops transform test` command for debugging
5. **Additional Transforms**:
   - `upper`, `lower` - Case conversion
   - `slice:start:end` - String slicing
   - `regex:pattern:replacement` - Regex replacement
   - `template:"{{.field}}"` - Go template evaluation

## Related Specifications

- **SPEC-002**: Configuration Parsing (`transforms:` section)
- **SPEC-003**: Secret Resolution Engine (invokes transforms)
- **SPEC-008**: Render Command (applies transforms during rendering)
