# SPEC-002: Configuration Parsing

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Related**:
- VISION.md Section 4 (Configuration `dsops.yaml`)
- VISION.md Section 5 (Secret Stores & Services)
- ADR-001: Terminology and dsops-data

## Summary

dsops parses YAML configuration files (`dsops.yaml`) that define secret stores, services, environments, variables, transforms, templates, and policies. The parser supports both modern (`secretStores` + `services`) and legacy (`providers`) formats for backward compatibility. Configuration is loaded at runtime, validated for required fields and format correctness, and provides detailed error messages for malformed configs.

## User Stories (As Built)

### User Story 1: Load Configuration from YAML (P1)

Users provide a `dsops.yaml` file that dsops parses into a structured configuration object.

**Why this priority**: Foundation for all dsops operations. Without configuration parsing, no other features work.

**Acceptance Criteria** (✅ Validated by tests):
1. Test: `TestConfigLoad_ValidYAML` in `internal/config/config_new_format_test.go`
2. Test: Validates version field, secret stores, services, environments
3. **Given** valid `dsops.yaml`, **Then** `Config.Load()` succeeds and populates `Definition`
4. **Given** missing file, **Then** returns `FileNotFoundError` with helpful message
5. **Given** invalid YAML syntax, **Then** returns parse error with line number

### User Story 2: Support Dual Reference Format (P1)

Users can reference secrets using modern URI format (`store://`, `svc://`) or legacy format (`provider` + `key`).

**Why this priority**: Backward compatibility ensures existing configs continue working during migration.

**Acceptance Criteria** (✅ Validated by tests):
1. **Given** variable with `from.store: store://aws/secret`, **Then** parsed as store reference
2. **Given** variable with `from.provider` + `from.key`, **Then** parsed as legacy reference
3. **Given** both formats present, **Then** modern format takes precedence
4. Test: `TestConfigLoad_MixedFormat` validates dual format support

### User Story 3: Validate Configuration Structure (P2)

Users receive clear validation errors for malformed configurations.

**Why this priority**: Early validation prevents runtime failures and improves debugging experience.

**Acceptance Criteria** (✅ Validated):
1. **Given** missing `version` field, **Then** error suggests adding version
2. **Given** unknown provider/store type, **Then** error lists valid types
3. **Given** invalid environment reference, **Then** error shows available environments

## Implementation

### Architecture

Configuration parsing follows a three-stage pipeline:

1. **File Reading** → Read YAML bytes from disk
2. **YAML Parsing** → Unmarshal into Go structs using `gopkg.in/yaml.v3`
3. **Validation** → Check required fields, format correctness, reference validity

**Key Files**:
- `internal/config/config.go:1-200` - Core config structures and loading
- `internal/config/config_new_format_test.go` - Test coverage for new format
- `internal/errors/errors.go` - Custom error types

### Design Decisions

- **YAML over JSON**: Chose YAML for human-friendliness (comments, multiline strings, no trailing commas)
- **Struct Tags**: Use `yaml` tags for field mapping and `omitempty` for optional fields
- **Inline Config**: Provider-specific fields embedded via `,inline` for flexibility
- **Version Field**: Required `version: 1` enables future breaking changes without migration scripts
- **Dual Format Support**: `secretStores` + `services` is primary; `providers` maintained for compatibility
- **Reference Union Type**: `Reference` struct supports both modern and legacy formats

**Trade-offs**:
- **Pro**: YAML readability improves adoption
- **Pro**: Inline config allows provider-specific fields without predefined schemas
- **Con**: YAML parser errors can be cryptic for beginners
- **Con**: Dual format support adds complexity to resolution logic

### Configuration Structure

```yaml
version: 1  # Required

# Modern format (primary)
secretStores:
  store-name:
    type: provider-type
    timeout_ms: 30000  # Optional
    # Provider-specific fields inline

services:
  service-name:
    type: service-type  # Maps to dsops-data
    # Service-specific fields inline

# Legacy format (deprecated but supported)
providers:
  provider-name:
    type: provider-type
    # Provider-specific fields

# Optional sections
transforms:
  named-transform: [step1, step2]

envs:
  env-name:
    VAR_NAME:
      from:
        store: store://store-name/path  # Modern
        service: svc://service-name?kind=credential_kind  # Optional
      transform: named-transform  # Optional
      optional: false  # Default: false

templates:
  - name: template-name
    format: dotenv|json|yaml|template
    env: env-name
    out: output-path

policies:
  gitignore:
    enforce: true
```

## Security Considerations

- **No Secrets in Config**: Configuration file should never contain raw secret values (only references)
- **File Permissions**: Configuration file can contain sensitive metadata (provider names, paths); recommend `chmod 600`
- **Validation Errors**: Error messages redact potential secret values from validation failures
- **Legacy Format Warning**: Log warning when legacy `providers:` format detected, encourage migration

## Testing

**Test Coverage**: 85% (from `internal/config/config_new_format_test.go`)

**Test Files**:
- `internal/config/config_new_format_test.go` - Comprehensive format testing
  - `TestConfigLoad_ValidYAML` - Happy path validation
  - `TestConfigLoad_MixedFormat` - Dual format support
  - `TestConfigLoad_InvalidYAML` - Error handling
  - `TestReference_Validation` - Reference format validation

**Future Enhancement**: Add property-based testing for YAML parsing edge cases

## Documentation

- **User Guide**: `docs/content/getting-started/configuration.md` - Config file structure
- **CLI Reference**: `docs/content/reference/configuration-reference.md` - Complete field reference
- **Examples**: `examples/*.yaml` - 40+ working configurations

## Lessons Learned

**What Went Well**:
- Inline config pattern provides flexibility without sacrificing type safety
- `yaml.v3` library handles complex YAML features (anchors, aliases) automatically
- Test-driven approach caught many edge cases early

**What Could Be Improved**:
- **Schema Validation**: Add JSON Schema or similar for config validation before parsing
- **Migration Tool**: Create `dsops migrate config` to auto-upgrade legacy format
- **Error Messages**: Improve YAML parse error messages with more context

## Future Enhancements (v0.2+)

1. **Config Validation Command**: `dsops config validate` with detailed error reporting
2. **Config Generation**: `dsops config generate` interactive wizard
3. **Secrets in Config**: Support `config://` references for config-level secrets (e.g., provider credentials)
4. **Include Files**: `includes:` section for splitting large configs
5. **Environment Overrides**: Environment variable substitution in config (e.g., `${AWS_REGION}`)
6. **Config Profiles**: Multiple configs with inheritance (e.g., `base.yaml` + `dev.yaml`)

## Related Specifications

- **SPEC-001**: CLI Framework (`--config` flag integration)
- **SPEC-003**: Secret Resolution Engine (consumes parsed config)
- **SPEC-004**: Transform Pipeline (uses `transforms:` section)
- **SPEC-080-093**: Provider Specs (config schema for each provider)
