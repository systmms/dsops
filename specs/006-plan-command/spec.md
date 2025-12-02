# SPEC-006: Plan Command

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Related**:
- SPEC-001: CLI Framework
- SPEC-002: Configuration Parsing
- SPEC-003: Secret Resolution Engine

## Summary

The `plan` command is a dry-run diagnostic tool that shows what secrets will be resolved from which sources **without fetching actual values**. It enables users to verify their configuration is correct before executing commands that need secrets, with helpful error reporting for troubleshooting. Safe to run in CI/CD environments without credential exposure.

## User Stories (As Built)

### User Story 1: Preview Secret Resolution (P1)

Users can see what variables will be resolved and from which providers before actual execution.

**Why this priority**: Essential for debugging and validating configuration without risking credential exposure or provider rate limits.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops plan --env production`, **Then** table shows all variables with their sources, transforms, and optional status
2. **Given** configuration references non-existent provider, **Then** error is shown with helpful suggestion
3. **Given** user runs `dsops plan --env dev --json`, **Then** output is machine-readable JSON

### User Story 2: Identify Configuration Errors (P1)

Users receive clear error messages identifying configuration issues before attempting secret resolution.

**Why this priority**: Fail-fast approach saves time and prevents partial failures during actual secret operations.

**Acceptance Criteria** (✅ Validated):
1. **Given** variable references unregistered provider, **Then** plan shows error with suggested fix
2. **Given** variable uses service reference (`svc://`) instead of store reference, **Then** plan detects and reports the error
3. **Given** environment has multiple errors, **Then** all errors are reported together (not fail-fast)

### User Story 3: Validate Environment Before Execution (P2)

Users can integrate plan into CI/CD pipelines as a validation gate.

**Why this priority**: Enables shift-left validation of secret configurations in deployment pipelines.

**Acceptance Criteria** (✅ Validated):
1. **Given** plan finds errors, **Then** exit code is non-zero (1)
2. **Given** plan succeeds, **Then** exit code is 0
3. **Given** `--json` flag used, **Then** output includes summary with error counts

## Implementation

### Architecture

The plan command follows a three-phase workflow:

1. **Configuration Loading**: Parse `dsops.yaml` and validate structure
2. **Provider Registration**: Register all secret stores without connecting
3. **Planning**: Analyze each variable's resolution path without fetching values

**Key Files**:
- `cmd/dsops/commands/plan.go` - Command implementation
- `internal/resolve/resolver.go:123-173` - `Plan()` method
- `internal/resolve/resolver.go:114-120` - `PlannedVariable` struct
- `internal/resolve/resolver.go:108-111` - `PlanResult` struct

### Command Signature

```bash
dsops plan --env <name> [--json] [--data-dir <path>]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--env` | string | (required) | Environment name to plan |
| `--json` | bool | false | Output results in JSON format |
| `--data-dir` | string | "./dsops-data" | Path to dsops-data repository |

### Data Structures

```go
type PlannedVariable struct {
    Name      string  // Variable name (e.g., "DATABASE_URL")
    Source    string  // "literal", "provider:X key:Y", or service reference
    Transform string  // Transform to apply (e.g., "json_extract:.password")
    Optional  bool    // Whether variable is optional
    Error     error   // Planning error (if any)
}

type PlanResult struct {
    Variables []PlannedVariable
    Errors    []error  // Collected planning errors
}
```

### Output Formats

**Table Output** (default):
```
VARIABLE        SOURCE                    TRANSFORM           OPTIONAL  STATUS
DATABASE_URL    provider:aws key:db/url   json_extract:.url   false     ✓ ready
API_KEY         provider:bw item:api-key                      true      ✓ ready
BAD_REF         (unknown)                                     false     ✗ error: provider not registered

Summary: 2 variables ready, 1 error
```

**JSON Output** (`--json`):
```json
{
  "variables": [...],
  "errors": [...],
  "summary": {
    "total": 3,
    "ready": 2,
    "errors": 1
  }
}
```

### Design Decisions

- **No Provider Connection**: Plan never connects to actual providers, ensuring no credential exposure
- **Two-Registry Support**: Handles both new `secretStores` format and legacy `providers` for backward compatibility
- **Error Aggregation**: Collects all errors before reporting (doesn't fail on first error)
- **Smart Suggestions**: Output suggests next steps (`dsops exec`, `dsops doctor`) based on result

**Trade-offs**:
- **Pro**: Safe for CI/CD pipelines (no secret exposure)
- **Pro**: Fast execution (no network calls)
- **Con**: Cannot detect runtime errors (invalid credentials, missing secrets)
- **Con**: Does not validate transform syntax (only validated at resolution time)

## Testing

**Test File**: `cmd/dsops/commands/plan_test.go`

**Test Cases**:
- `TestPlanCommand_BasicUsage` - Table and JSON output formats
- `TestPlanCommand_MissingEnvFlag` - Required flag validation
- `TestPlanCommand_InvalidEnvironment` - Non-existent environment handling
- `TestPlanCommand_VariableWithTransform` - Transform display
- `TestPlanCommand_OptionalVariable` - Optional flag display
- `TestPlanCommand_EmptyEnvironment` - Empty environment handling

## Lessons Learned

**What Went Well**:
- Reusing resolver infrastructure made implementation straightforward
- Table output format provides quick visual feedback
- JSON output enables scripting and tooling integration

**What Could Be Improved**:
- Add transform syntax validation during planning (catch errors earlier)
- Add `--quiet` flag for CI/CD (exit code only, no output)
- Consider caching plan results for large configurations

## Future Enhancements (v0.2+)

1. **Transform Validation**: Validate transform syntax during planning
2. **Diff Mode**: Compare plan against previous run (`dsops plan --diff`)
3. **Export Format**: Export plan as GitHub Actions env file (`dsops plan --export-github`)
4. **Dry-run Transforms**: Show transformed value shapes without real data

## Related Specifications

- **SPEC-001**: CLI Framework (command registration, global flags)
- **SPEC-002**: Configuration Parsing (config file loading)
- **SPEC-003**: Secret Resolution Engine (resolver infrastructure)
- **SPEC-007**: Exec Command (uses plan internally before execution)
