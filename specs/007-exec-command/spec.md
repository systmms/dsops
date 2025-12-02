# SPEC-007: Exec Command

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Related**:
- SPEC-001: CLI Framework
- SPEC-003: Secret Resolution Engine
- SPEC-006: Plan Command

## Summary

The `exec` command resolves all environment variables from configured secret providers and executes a child process with secrets injected into its environment. **No secrets are written to disk** - they exist only in the child process's environment (ephemeral execution). This is the primary workflow for running applications with secrets in dsops, following the security-first principle that secrets should never touch disk.

## User Stories (As Built)

### User Story 1: Execute Commands with Injected Secrets (P1)

Users can run any command with secrets automatically injected as environment variables.

**Why this priority**: Core functionality of dsops - ephemeral secret injection is the primary use case.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops exec --env prod -- npm start`, **Then** child process receives resolved secrets as env vars
2. **Given** command requires `--` separator, **Then** dsops flags are parsed separately from command flags
3. **Given** child process exits with code N, **Then** dsops exits with same code N

### User Story 2: Debug Secret Resolution (P1)

Users can inspect which variables were resolved without exposing actual values.

**Why this priority**: Essential for debugging in production-like environments without leaking secrets.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops exec --env prod --print -- env`, **Then** resolved variable names are shown with masked values
2. **Given** `--print` flag used with short value, **Then** value shows as `***`
3. **Given** `--print` flag used with long value, **Then** value shows as `abc********xy` (partial masking)

### User Story 3: Control Environment Merging (P2)

Users can control how dsops secrets interact with existing environment variables.

**Why this priority**: Backward compatibility with existing deployment patterns and override flexibility.

**Acceptance Criteria** (✅ Validated):
1. **Given** dsops secret conflicts with existing env var, **Then** dsops value takes precedence (default)
2. **Given** `--allow-override` flag used, **Then** existing env vars are preserved (dsops values only added if missing)
3. **Given** no conflicts, **Then** both dsops secrets and existing env vars are available

### User Story 4: Safe Command Execution (P2)

Users are protected from accidentally running dangerous commands with elevated privileges.

**Why this priority**: Defense in depth - prevent accidental destructive operations.

**Acceptance Criteria** (✅ Validated):
1. **Given** user attempts `dsops exec -- rm -rf /`, **Then** warning is logged (command blocked)
2. **Given** user provides non-existent command, **Then** error is returned before secret resolution
3. **Given** user sets timeout, **Then** command is terminated after timeout expires

## Implementation

### Architecture

The exec command follows a four-phase workflow:

1. **Validation**: Validate command exists and is not blocked
2. **Resolution**: Resolve all secrets from configured providers
3. **Environment Building**: Merge resolved secrets with process environment
4. **Execution**: Launch child process with injected environment

**Key Files**:
- `cmd/dsops/commands/exec.go` - Command implementation
- `internal/execenv/exec.go` - Executor implementation
- `internal/resolve/resolver.go:99-105` - `ResolvedVariable` struct

### Command Signature

```bash
dsops exec --env <name> [flags] -- <command> [args...]
```

**Flags**:
| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--env` | string | (required) | Environment name to resolve |
| `--print` | bool | false | Print resolved variable names (values masked) |
| `--allow-override` | bool | false | Allow existing env vars to override dsops values |
| `--working-dir` | string | "" | Working directory for executed command |
| `--timeout` | int | 0 | Command timeout in seconds (0 = no timeout) |

**Important**: The `--` separator is required to distinguish dsops flags from command arguments.

### Data Structures

```go
type ResolvedVariable struct {
    Name        string  // Variable name
    Value       string  // Resolved secret value
    Source      string  // Provider that provided the value
    Transformed bool    // Whether value was transformed
    Error       error   // Resolution error (if any)
}

type ExecOptions struct {
    Command       []string          // Command and arguments
    Environment   map[string]string // Variables to inject
    AllowOverride bool              // Override existing env vars?
    PrintVars     bool              // Print resolved vars?
    WorkingDir    string            // Working directory
    Timeout       int               // Timeout in seconds
}
```

### Value Masking Logic

When `--print` is used, values are masked for security:

| Value Length | Display |
|--------------|---------|
| Empty | `(empty)` |
| 1-3 chars | `***` |
| 4-8 chars | `a******z` (first + asterisks + last) |
| 9+ chars | `abc********xy` (first 3 + asterisks + last 2) |

### Environment Merging

```
┌─────────────────┐     ┌──────────────────┐
│ dsops secrets   │  +  │ existing env     │
│ (resolved)      │     │ (from parent)    │
└────────┬────────┘     └────────┬─────────┘
         │                       │
         └───────────┬───────────┘
                     │
         ┌───────────▼───────────┐
         │ allow-override=false  │ → dsops wins conflicts
         │ allow-override=true   │ → existing wins conflicts
         └───────────────────────┘
```

### Blocked Commands

The executor blocks these dangerous commands:
- File system: `rm`, `rmdir`, `del`, `format`, `fdisk`, `dd`, `mkfs`, `parted`
- System: `shutdown`, `reboot`

### Design Decisions

- **Exit Code Preservation**: Child process exit code propagates exactly to parent (critical for CI/CD)
- **Ephemeral Only**: Secrets never written to disk, only exist in child process memory
- **Concurrent Resolution**: Uses semaphore (max 10) for concurrent provider calls
- **Error Aggregation**: Collects all resolution errors before failing (shows complete picture)

**Trade-offs**:
- **Pro**: True ephemeral execution - secrets cleaned up when process exits
- **Pro**: Exit code preservation enables reliable CI/CD integration
- **Con**: Cannot inspect secrets after execution (by design)
- **Con**: Blocked commands list is basic safety check, not comprehensive security

## Security Considerations

- **No Disk Writes**: Secrets exist only in child process environment
- **Parent Isolation**: Parent process never sees resolved secret values
- **Masked Printing**: `--print` flag shows names but masks values
- **Process Boundary**: Secrets are cleaned up when child process terminates
- **No Shell Expansion**: Command executed directly, not through shell (prevents injection)

## Testing

**Test File**: `cmd/dsops/commands/exec_test.go`

**Test Cases**:
- `TestExecCommand_MissingCommand` - No command after `--`
- `TestExecCommand_MissingEnvFlag` - Required `--env` flag validation
- `TestExecCommand_NonexistentEnvironment` - Invalid environment handling
- `TestExecCommand_FlagParsing` - All flags properly defined
- `TestExecCommand_SimpleExecution` - Actual script execution
- `TestExecCommand_ResolutionError` - Provider resolution failures
- `TestExecCommand_EmptyEnvironment` - Execution with no variables

## Lessons Learned

**What Went Well**:
- Exit code preservation was straightforward with `syscall.WaitStatus`
- Environment merging logic handles edge cases cleanly
- Value masking provides good debugging UX without security risks

**What Could Be Improved**:
- Add signal forwarding (SIGTERM, SIGINT) to child process
- Add `--shell` flag for users who need shell expansion
- Consider streaming output vs buffering for long-running commands

## Future Enhancements (v0.2+)

1. **Signal Forwarding**: Forward signals (SIGTERM, SIGINT) to child process
2. **Shell Mode**: Optional `--shell` flag to execute via shell
3. **Retry Logic**: Automatic retry for transient provider failures
4. **Secret Rotation Detection**: Warn if secrets changed during long-running execution
5. **Audit Logging**: Log which secrets were accessed (without values)

## Related Specifications

- **SPEC-001**: CLI Framework (command registration, global flags)
- **SPEC-003**: Secret Resolution Engine (resolver infrastructure)
- **SPEC-006**: Plan Command (dry-run validation before exec)
- **SPEC-008**: Doctor Command (validate providers before exec)
