# SPEC-008: Doctor Command

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Related**:
- SPEC-001: CLI Framework
- SPEC-002: Configuration Parsing
- SPEC-010 through SPEC-019: Provider Specifications

## Summary

The `doctor` command is a diagnostic utility for validating that dsops configuration, secret store providers, and their connectivity are properly configured and accessible. It provides actionable troubleshooting suggestions when issues are detected, making it the first tool to run when debugging secret resolution problems. Supports both the new `secretStores` format and legacy `providers` for backward compatibility.

## User Stories (As Built)

### User Story 1: Validate Provider Connectivity (P1)

Users can verify all configured providers are accessible and properly authenticated.

**Why this priority**: Provider validation is essential before attempting secret operations - fail-fast approach saves debugging time.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops doctor`, **Then** table shows status of all configured providers
2. **Given** provider is healthy, **Then** status shows `✓ healthy`
3. **Given** provider authentication fails, **Then** status shows `✗ error` with message

### User Story 2: Receive Actionable Troubleshooting (P1)

Users receive provider-specific suggestions when validation fails.

**Why this priority**: Generic error messages are unhelpful; provider-specific guidance accelerates problem resolution.

**Acceptance Criteria** (✅ Validated):
1. **Given** Bitwarden provider fails with "locked", **Then** suggests `bw unlock`
2. **Given** AWS provider fails with "credentials", **Then** suggests `aws configure`
3. **Given** 1Password provider fails with "not found", **Then** suggests installation URL

### User Story 3: Inspect Provider Capabilities (P2)

Users can see what features each provider supports.

**Why this priority**: Understanding capabilities helps users choose appropriate providers and features.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops doctor --verbose`, **Then** healthy providers show capabilities
2. **Given** provider supports versioning, **Then** capabilities show `Versioning: true`
3. **Given** unhealthy provider, **Then** verbose mode shows troubleshooting suggestions

### User Story 4: Validate Environment Configuration (P2)

Users can validate a specific environment's variable definitions.

**Why this priority**: Catching environment configuration errors early prevents runtime failures.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops doctor --env production`, **Then** environment variables are validated
2. **Given** environment has errors, **Then** specific variable errors are listed
3. **Given** environment is valid, **Then** shows "Environment 'X' is ready"

## Implementation

### Architecture

The doctor command performs sequential validation phases:

1. **Configuration Loading**: Parse and validate `dsops.yaml`
2. **Provider Registration**: Register all configured providers
3. **Secret Store Validation**: Call `Validate()` on each secret store
4. **Legacy Provider Validation**: Validate legacy `providers` section
5. **Environment Validation** (optional): Run plan on specified environment

**Key Files**:
- `cmd/dsops/commands/doctor.go` - Command implementation
- `pkg/provider/provider.go` - Provider interface with `Validate()` method
- `internal/resolve/resolver.go` - `ValidateProvider()` method

### Command Signature

```bash
dsops doctor [--verbose] [--env <name>] [--data-dir <path>]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--verbose` | bool | false | Show detailed provider information and suggestions |
| `--env` | string | "" | Also validate specific environment configuration |
| `--data-dir` | string | "./dsops-data" | Path to dsops-data repository |

### Data Structures

```go
type ProviderHealth struct {
    Name         string                 // Provider instance name
    Type         string                 // Provider type (e.g., "bitwarden")
    Status       string                 // "healthy", "error", "checking"
    Error        string                 // Error message if status is error
    Message      string                 // Success message if healthy
    Capabilities provider.Capabilities  // Provider capability flags
    Suggestions  []string              // Troubleshooting suggestions
}
```

### Output Formats

**Standard Output**:
```
PROVIDER        TYPE                   STATUS      MESSAGE
aws-prod        aws.secretsmanager     ✓ healthy   Provider is accessible
bitwarden-dev   bitwarden              ✗ error     vault is locked
literal         literal                ✓ healthy   Literal provider ready

Summary: 2/3 providers healthy
```

**Verbose Output** (`--verbose`):
```
PROVIDER        TYPE                   STATUS      MESSAGE
aws-prod        aws.secretsmanager     ✓ healthy   Provider is accessible

aws-prod capabilities:
  • Versioning: true
  • Metadata: true
  • Auth required: true
  • Auth methods: [iam, access_key]

bitwarden-dev   bitwarden              ✗ error     vault is locked

bitwarden-dev suggestions:
  • Run: bw unlock
  • Export session: export BW_SESSION="session-key"
```

**Environment Check Output** (`--env`):
```
Environment 'production': 5 variables, 1 error

Variable errors:
  ✗ DATABASE_URL: provider 'mysql' not registered

✓ Environment 'production' is ready (4/5 variables valid)
```

### Provider-Specific Suggestions

| Provider | Error Contains | Suggestions |
|----------|---------------|-------------|
| bitwarden | "not found" | Install CLI: `npm install -g @bitwarden/cli` |
| bitwarden | "unauthenticated" | Run: `bw login your-email@example.com` |
| bitwarden | "locked" | Run: `bw unlock`, Export `BW_SESSION` |
| 1password | "not found" | Install from: developer.1password.com |
| aws | "credentials" | Run: `aws configure`, Set env vars |
| aws | "region" | Set `AWS_REGION` or configure in dsops.yaml |

### Design Decisions

- **Secret Stores Only**: Validates secret stores, not services (services are rotation targets)
- **Timeout Handling**: Each provider has configurable timeout (default 30s)
- **Backward Compatibility**: Supports both `secretStores` and legacy `providers` sections
- **Non-Destructive**: Only reads, never modifies provider state

**Trade-offs**:
- **Pro**: Provider-specific suggestions dramatically reduce debugging time
- **Pro**: Timeout prevents hanging on unresponsive providers
- **Con**: Cannot detect all issues (e.g., missing permissions for specific secrets)
- **Con**: Suggestions are hardcoded, may become outdated

## Testing

**Test File**: `cmd/dsops/commands/doctor_test.go`

**Test Cases** (11 tests):
- `TestDoctorCommand_BasicExecution` - Basic execution with literal provider
- `TestDoctorCommand_NoProviders` - Behavior with no configured providers
- `TestDoctorCommand_WithSecretStores` - New secretStores format
- `TestDoctorCommand_UnimplementedProvider` - Unsupported provider types
- `TestDoctorCommand_WithEnvironmentCheck` - `--env` flag validation
- `TestDoctorCommand_VerboseFlag` - Verbose output format
- `TestDoctorCommand_FlagDefinitions` - Flag parsing
- `TestDoctorCommand_InvalidConfig` - Malformed YAML handling
- `TestDoctorCommand_MixedSecretStoresAndLegacyProviders` - Mixed format
- `TestGetSuggestions` - Provider-specific suggestion generation
- `TestContainsHelper` - String matching helper

## Lessons Learned

**What Went Well**:
- Provider-specific suggestions receive positive user feedback
- Timeout handling prevents frustrating hangs
- Table output format provides quick visual status

**What Could Be Improved**:
- Add `--json` flag for machine-readable output
- Make suggestions configurable (allow updates without code changes)
- Add `--fix` flag to attempt automatic remediation

## Future Enhancements (v0.2+)

1. **JSON Output**: Add `--json` flag for CI/CD integration
2. **Auto-Fix Mode**: `--fix` flag to attempt automatic remediation
3. **Health History**: Track provider health over time for trend analysis
4. **Custom Suggestions**: User-configurable suggestion overrides
5. **Parallel Validation**: Validate providers concurrently for faster results
6. **Deep Validation**: Optionally test fetching a known secret

## Related Specifications

- **SPEC-001**: CLI Framework (command registration, global flags)
- **SPEC-002**: Configuration Parsing (config file loading)
- **SPEC-006**: Plan Command (environment validation logic shared)
- **SPEC-010 through SPEC-019**: Provider implementations that doctor validates
