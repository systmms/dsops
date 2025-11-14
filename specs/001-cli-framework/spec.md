# SPEC-001: CLI Framework

**Status**: Implemented (Retrospective)
**Feature Branch**: `main` (merged)
**Implementation Date**: 2025-08-26
**Related**:
- VISION.md Section 9 (CLI spec)
- VISION.md Section 3 (High-level UX)

## Summary

dsops uses a Cobra-based CLI framework providing a consistent command-line interface with global flags, subcommands, and built-in help. The framework enables ephemeral-first secret operations through intuitive commands like `exec`, `render`, `plan`, and `doctor`. All commands share global configuration (config file path, debug mode, color output) and follow Unix conventions for exit codes and error reporting.

## User Stories (As Built)

### User Story 1: Execute Commands with Global Flags (P1)

Users can invoke any dsops command with consistent global flags that control behavior across the entire CLI.

**Why this priority**: Foundation for all CLI interactions. Without consistent global flags, every command would need its own configuration mechanism.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops --config custom.yaml plan`, **Then** custom config file is loaded for plan command
2. **Given** user runs `dsops --debug exec -- env`, **Then** debug logging is enabled during execution
3. **Given** user runs `dsops --no-color plan`, **Then** output is rendered without ANSI color codes
4. **Given** user runs `dsops --non-interactive render`, **Then** no interactive prompts are shown

### User Story 2: Discover Available Commands (P1)

Users can explore available commands through built-in help and version information.

**Why this priority**: Discoverability is critical for onboarding and self-service troubleshooting.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs `dsops --help`, **Then** list of all commands with descriptions is shown
2. **Given** user runs `dsops version`, **Then** version, commit hash, and build date are displayed
3. **Given** user runs `dsops <command> --help`, **Then** command-specific help and flags are shown

### User Story 3: Consistent Error Handling (P2)

Users receive clear error messages with non-zero exit codes on failures.

**Why this priority**: Enables reliable scripting and CI/CD integration.

**Acceptance Criteria** (✅ Validated):
1. **Given** user runs invalid command, **Then** error is printed to stderr and exit code is 1
2. **Given** command fails, **Then** error message explains problem and suggests solution
3. **Given** `PersistentPreRun` fails, **Then** error propagates and prevents command execution

## Implementation

### Architecture

The CLI framework consists of three layers:

1. **Entry Point** (`cmd/dsops/main.go`)
   - Application initialization
   - Global flag parsing
   - Command registration
   - Error handling and exit codes

2. **Command Layer** (`cmd/dsops/commands/*.go`)
   - Individual command implementations
   - Command-specific flags
   - Business logic delegation to internal packages

3. **Shared Context** (`config.Config`)
   - Logger instance
   - Configuration file path
   - Interactive mode flag
   - Propagated to all commands via `cfg` parameter

**Key Files**:
- `cmd/dsops/main.go:1-80` - Main entry point and root command setup
- `cmd/dsops/commands/` - Command implementations (15 files)
- `internal/config/config.go` - Configuration structure
- `internal/logging/logger.go` - Logging implementation

### Design Decisions

- **Cobra Framework**: Chosen for rich CLI features (subcommands, flags, help generation) and widespread Go community adoption
- **Persistent Flags**: Global flags defined on root command propagate to all subcommands
- **PersistentPreRun**: Initializes logger and config before any command executes, ensuring consistent setup
- **Config Struct Sharing**: Single `config.Config` instance passed to all commands via constructor pattern
- **Version Injection**: Build-time variables (`version`, `commit`, `date`) injected via ldflags during compilation

**Trade-offs**:
- **Pro**: Cobra provides excellent UX out-of-the-box (help text, completions, etc.)
- **Pro**: Persistent flags eliminate repetitive flag definitions
- **Con**: Cobra adds ~3MB to binary size
- **Con**: Global state in `cfg` requires careful handling in tests

### Command Structure

```go
rootCmd (dsops)
├── init       - Bootstrap new project
├── plan       - Dry-run secret resolution
├── render     - Write secrets to files
├── exec       - Execute command with secrets
├── get        - Fetch single secret value
├── doctor     - Validate provider credentials
├── providers  - List available providers
├── login      - Authenticate with providers
├── shred      - Securely delete generated files
├── guard      - Repository hygiene checks
├── install-hook - Install git pre-commit hooks
├── leak       - Record security incidents
├── secrets    - Secret management (subcommand)
│   ├── rotate - Rotate secrets
│   ├── status - Check rotation status
│   └── history - View rotation history
└── rotation   - Rotation metadata (subcommand)
    ├── status - Overall rotation status
    └── history - Rotation audit log
```

## Security Considerations

- **No Secrets in Flags**: CLI design prohibits passing secret values via command-line flags (visible in `ps`)
- **Debug Mode Safety**: `--debug` flag enables verbose logging but never prints raw secret values (enforced in `internal/logging`)
- **Error Message Sanitization**: Errors sanitize paths and values to prevent accidental secret leakage
- **Non-Interactive Mode**: `--non-interactive` flag disables prompts for CI/CD environments where stdin is unavailable

## Testing

**Test Coverage**: No dedicated CLI framework tests (integration tests validate command behavior)

**Testing Approach**:
- CLI framework tested implicitly through command integration tests
- Each command in `cmd/dsops/commands/` has corresponding `*_test.go`
- Example: `cmd/dsops/commands/rotation_test.go` validates command registration and flag parsing

**Future Enhancement**: Add dedicated `cmd/dsops/main_test.go` for root command behavior

## Documentation

- **User Guide**: `docs/content/getting-started/` - CLI usage examples
- **CLI Reference**: `docs/content/reference/cli.md` - Complete command reference
- **Built-in Help**: `dsops --help` and `dsops <command> --help`

## Lessons Learned

**What Went Well**:
- Cobra's `PersistentPreRun` hook simplified global state initialization
- Single config struct pattern made dependency injection straightforward
- Version injection via ldflags works reliably across build systems

**What Could Be Improved**:
- **Testing**: Add dedicated CLI framework tests for flag parsing and error handling
- **Configuration**: Consider splitting `config.Config` into separate concerns (LoggerConfig, ProviderConfig, etc.)
- **Subcommand Depth**: Two-level subcommands (`dsops secrets rotate`) work well; avoid deeper nesting

## Future Enhancements (v0.2+)

1. **Shell Completions**: Generate Bash/Zsh/Fish completions via `dsops completion`
2. **Command Aliases**: Support `dsops r` as alias for `dsops render`
3. **Config Profiles**: Multiple config files with `--profile` flag (e.g., `dsops --profile=staging plan`)
4. **Plugin Commands**: Dynamic command registration from exec-plugins
5. **JSON Output Mode**: Global `--json` flag for machine-readable output
6. **Progress Indicators**: Spinner for long-running operations (plan, doctor, rotation)

## Related Specifications

- **SPEC-002**: Configuration Parsing (how `--config` flag is processed)
- **SPEC-006**: Plan Command (first major command implementation)
- **SPEC-007**: Exec Command (ephemeral execution workflow)
- **SPEC-010**: Doctor Command (credential validation)
