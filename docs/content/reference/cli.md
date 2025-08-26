---
title: "CLI Reference"
description: "Complete command-line interface reference for dsops"
lead: "Comprehensive reference for all dsops commands, flags, and options. Master the command-line interface for efficient secret management workflows."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 10
---

## Overview

dsops is a command-line tool for developer secret operations. It provides secure, ephemeral access to secrets from various providers with a focus on security-first principles.

## Global Options

All dsops commands support these global flags:

| Flag | Description | Default | Environment Variable |
|------|-------------|---------|----------------------|
| `--config` | Config file path | `dsops.yaml` | `DSOPS_CONFIG` |
| `--debug` | Enable debug logging | `false` | `DSOPS_DEBUG=true` |
| `--no-color` | Disable colored output | `false` | `DSOPS_NO_COLOR=true` |
| `--non-interactive` | Non-interactive mode | `false` | - |

### Examples

```bash
# Use custom config file
dsops --config production.yaml exec --env prod -- npm start

# Enable debug logging
dsops --debug doctor

# Disable colors (useful for scripts)
dsops --no-color render --env prod --out .env
```

## Command Categories

### Core Commands

Essential commands for everyday secret operations.

#### `dsops init`

Initialize a new dsops configuration file.

```bash
dsops init [flags]
```

**Description**: Creates a new `dsops.yaml` configuration file with example providers and environments. Interactive prompts guide you through initial setup.

**Flags**:
- `--force` - Overwrite existing configuration file
- `--provider <type>` - Include specific provider in template
- `--minimal` - Generate minimal configuration

**Examples**:
```bash
# Interactive setup
dsops init

# Force overwrite existing config
dsops init --force

# Minimal config with 1Password
dsops init --provider onepassword --minimal
```

**Output**: Creates `dsops.yaml` with sample configuration.

---

#### `dsops plan`

Show resolution plan without fetching secret values.

```bash
dsops plan [flags]
```

**Description**: Displays what secrets will be resolved and from which providers, without accessing actual values. Useful for debugging configuration and verifying provider connectivity.

**Flags**:
- `--env <name>` - Environment to plan (required)
- `--format <format>` - Output format: `table`, `json` (default: `table`)
- `--data-dir <path>` - Path to dsops-data directory

**Examples**:
```bash
# Plan production environment
dsops plan --env production

# JSON output for automation
dsops plan --env staging --format json

# With custom data directory
dsops plan --env dev --data-dir ./custom-dsops-data
```

**Output**: Table showing variables, sources, and resolution status.

---

#### `dsops exec`

Execute commands with ephemeral environment variables.

```bash
dsops exec [flags] -- <command> [args...]
```

**Description**: Resolves secrets and injects them as environment variables into a child process. Secrets never touch disk and are only visible to the executed command.

**Flags**:
- `--env <name>` - Environment to use (required)
- `--print` - Print environment variables instead of executing
- `--allow-override` - Allow existing env vars to be overridden
- `--working-dir <path>` - Working directory for command
- `--timeout <seconds>` - Command timeout (default: no timeout)

**Examples**:
```bash
# Execute npm with production secrets
dsops exec --env production -- npm start

# Run Docker Compose with staging secrets
dsops exec --env staging -- docker compose up

# Print variables (debug mode)
dsops exec --env dev --print

# Execute with timeout
dsops exec --env prod --timeout 300 -- python deploy.py
```

**Security**: Child process inherits environment variables, parent process never sees secret values.

---

#### `dsops render`

Generate files from resolved secrets.

```bash
dsops render [flags]
```

**Description**: Renders secrets to various output formats. Explicit `--out` flag required for security (prevents accidental file writes).

**Flags**:
- `--env <name>` - Environment to render (required)
- `--out <file>` - Output file path (required)
- `--format <format>` - Output format: `dotenv`, `json`, `yaml`, `template`
- `--template <file>` - Custom template file (Go templates)
- `--ttl <duration>` - File TTL for automatic cleanup
- `--permissions <mode>` - File permissions (default: 0600)

**Supported Formats**:
- **dotenv**: `.env` file format
- **json**: JSON object with key-value pairs
- **yaml**: YAML object with key-value pairs
- **template**: Custom Go template

**Examples**:
```bash
# Render to .env file
dsops render --env development --out .env.development

# Render to JSON
dsops render --env production --out secrets.json --format json

# Use custom template
dsops render --env staging --out k8s-secret.yaml --template secret.tmpl

# With TTL and custom permissions
dsops render --env prod --out /tmp/secrets.env --ttl 1h --permissions 0640
```

**Security**: Files created with restrictive permissions (600) by default.

---

#### `dsops get`

Retrieve individual secret values.

```bash
dsops get [flags] <variable>
```

**Description**: Fetches and displays a single secret value. Useful for scripts and debugging.

**Flags**:
- `--env <name>` - Environment to use (required)
- `--raw` - Output raw value without formatting
- `--json` - Output as JSON with metadata

**Examples**:
```bash
# Get database password
dsops get --env production DATABASE_PASSWORD

# Raw output (no decorations)
dsops get --env staging --raw API_KEY

# JSON with metadata
dsops get --env dev --json DATABASE_URL
```

**Output**: Secret value or JSON object with metadata.

---

#### `dsops doctor`

Check provider connectivity and configuration health.

```bash
dsops doctor [flags]
```

**Description**: Validates configuration file, tests provider authentication, and checks system dependencies. Essential for troubleshooting setup issues.

**Flags**:
- `--env <name>` - Also validate specific environment
- `--verbose` - Show detailed diagnostic information
- `--data-dir <path>` - Path to dsops-data directory

**Examples**:
```bash
# Basic health check
dsops doctor

# Check specific environment
dsops doctor --env production

# Verbose diagnostics
dsops doctor --verbose
```

**Checks**:
- Configuration file syntax
- Provider authentication
- Required dependencies
- Network connectivity
- Environment variable definitions

---

### Information Commands

Commands for exploring and understanding your configuration.

#### `dsops providers`

List available secret store providers.

```bash
dsops providers [flags]
```

**Description**: Shows all supported secret store providers with descriptions and current status.

**Flags**:
- `--format <format>` - Output format: `table`, `json` (default: `table`)

**Examples**:
```bash
# List all providers
dsops providers

# JSON output
dsops providers --format json
```

**Output**: Table or JSON with provider types, descriptions, and status.

---

### Authentication Commands

Manage authentication for various providers.

#### `dsops login`

Authenticate with supported providers.

```bash
dsops login [provider] [flags]
```

**Description**: Interactive authentication flow for providers that support it (1Password, Bitwarden, etc.).

**Arguments**:
- `provider` - Provider to authenticate with (optional, prompts if not specified)

**Examples**:
```bash
# Interactive provider selection
dsops login

# Login to 1Password
dsops login onepassword

# Login to Bitwarden
dsops login bitwarden
```

**Behavior**: Opens browser or prompts for credentials as appropriate for the provider.

---

### Security Commands

Commands for security analysis and protection.

#### `dsops leak`

Scan for potentially leaked secrets.

```bash
dsops leak [flags] [paths...]
```

**Description**: Scans files for patterns that might indicate leaked secrets. Helps prevent accidental secret commits.

**Flags**:
- `--recursive` - Scan directories recursively
- `--include <pattern>` - Include files matching pattern
- `--exclude <pattern>` - Exclude files matching pattern
- `--format <format>` - Output format: `table`, `json`

**Examples**:
```bash
# Scan current directory
dsops leak

# Scan specific files
dsops leak src/config.js .env

# Recursive scan with pattern
dsops leak --recursive --include "*.js,*.py" src/
```

**Patterns Detected**:
- API keys
- Passwords
- Private keys
- Database URLs
- JWT tokens

---

#### `dsops shred`

Securely delete secret files.

```bash
dsops shred [flags] <files...>
```

**Description**: Securely overwrites and deletes files containing secrets. Uses multiple passes to prevent recovery.

**Flags**:
- `--passes <n>` - Number of overwrite passes (default: 3)
- `--force` - Don't prompt for confirmation

**Examples**:
```bash
# Shred secret files
dsops shred .env secrets.json

# Force shred without confirmation
dsops shred --force temp-secrets.yaml

# More secure deletion
dsops shred --passes 7 old-passwords.txt
```

**Warning**: This permanently destroys data. Use with caution.

---

#### `dsops guard`

Monitor file system for secret exposure.

```bash
dsops guard [flags] [paths...]
```

**Description**: Watches directories for files that might contain secrets and alerts when detected.

**Flags**:
- `--watch` - Continuous monitoring mode
- `--alert <method>` - Alert method: `log`, `email`, `webhook`
- `--exclude <pattern>` - Exclude paths from monitoring

**Examples**:
```bash
# Monitor current directory
dsops guard

# Watch mode with webhook alerts
dsops guard --watch --alert webhook --url https://alerts.example.com

# Monitor with exclusions
dsops guard --exclude "node_modules,*.log" src/
```

---

#### `dsops install-hook`

Install Git hooks for secret protection.

```bash
dsops install-hook [flags]
```

**Description**: Installs pre-commit hooks to prevent secrets from being committed to Git repositories.

**Flags**:
- `--type <type>` - Hook type: `pre-commit`, `pre-push` (default: `pre-commit`)
- `--force` - Overwrite existing hooks

**Examples**:
```bash
# Install pre-commit hook
dsops install-hook

# Install pre-push hook
dsops install-hook --type pre-push

# Force overwrite existing hook
dsops install-hook --force
```

**Behavior**: Creates `.git/hooks/pre-commit` that scans staged files for secrets.

---

### Secret Management Commands

Advanced secret lifecycle management.

#### `dsops secrets`

Parent command for secret management operations.

```bash
dsops secrets <subcommand> [flags]
```

**Description**: Manages the lifecycle of secret values including rotation, validation, and history tracking.

**Subcommands**:
- `rotate` - Rotate secret values
- `status` - Show secret rotation status
- `history` - View secret change history

---

#### `dsops secrets rotate`

Rotate secret values using configured strategies.

```bash
dsops secrets rotate [flags]
```

**Description**: Performs secret rotation using configured rotation strategies. Supports immediate, two-key, overlap, and gradual rotation patterns.

**Flags**:
- `--env <name>` - Environment to rotate
- `--key <name>` - Specific secret to rotate
- `--service <name>` - Rotate secrets for specific service
- `--strategy <strategy>` - Override configured strategy
- `--dry-run` - Show what would be rotated without doing it
- `--force` - Force rotation even if not due

**Strategies**:
- `immediate` - Replace secret instantly (brief downtime)
- `two-key` - Maintain two valid secrets (zero downtime)
- `overlap` - Gradual transition with overlap period
- `gradual` - Percentage-based rollout

**Examples**:
```bash
# Rotate all due secrets in production
dsops secrets rotate --env production

# Rotate specific database password
dsops secrets rotate --env prod --key DATABASE_PASSWORD

# Dry run to see what would happen
dsops secrets rotate --env staging --dry-run

# Force immediate rotation
dsops secrets rotate --env dev --service postgres --strategy immediate --force
```

**Output**: Progress report with rotation results and timing.

---

#### `dsops secrets status`

Show current rotation status for secrets.

```bash
dsops secrets status [flags]
```

**Description**: Displays rotation status, last rotation times, and next scheduled rotations for all secrets.

**Flags**:
- `--env <name>` - Environment to check
- `--service <name>` - Filter by service
- `--format <format>` - Output format: `table`, `json`, `yaml`
- `--verbose` - Include additional details

**Examples**:
```bash
# Show all secret status
dsops secrets status --env production

# Service-specific status
dsops secrets status --env prod --service postgres

# JSON for automation
dsops secrets status --env staging --format json
```

**Status Indicators**:
- ✅ Active - Recently rotated, healthy
- 🟡 Due - Rotation needed soon
- 🔄 Rotating - Currently rotating
- ❌ Failed - Last rotation failed
- ⚪ Never - Never been rotated

---

#### `dsops secrets history`

View secret rotation history.

```bash
dsops secrets history [flags] [service]
```

**Description**: Shows historical rotation events for audit trails and troubleshooting.

**Arguments**:
- `service` - Service name to filter history (optional)

**Flags**:
- `--limit <n>` - Limit number of entries (default: 50)
- `--since <date>` - Show history since date (YYYY-MM-DD)
- `--until <date>` - Show history until date
- `--status <status>` - Filter by status: `success`, `failed`
- `--format <format>` - Output format: `table`, `json`

**Examples**:
```bash
# Recent rotation history
dsops secrets history

# History for specific service
dsops secrets history postgres-prod

# Filter by date range
dsops secrets history --since 2024-01-01 --until 2024-12-31

# Only failed rotations
dsops secrets history --status failed --limit 10
```

**Output**: Chronological list with timestamps, services, results, and durations.

---

### Rotation Management Commands

Monitor and manage rotation operations.

#### `dsops rotation`

Parent command for rotation management.

```bash
dsops rotation <subcommand> [flags]
```

**Description**: Provides visibility into rotation operations, history, and compliance status.

**Subcommands**:
- `status` - Current rotation status across services
- `history` - Historical rotation events

---

#### `dsops rotation status`

Display rotation status across all services.

```bash
dsops rotation status [flags] [service]
```

**Description**: Shows current rotation state, timing, and health for configured services.

**Arguments**:
- `service` - Specific service to check (optional)

**Flags**:
- `--format <format>` - Output format: `table`, `json`, `yaml`
- `--verbose` - Show additional details

**Examples**:
```bash
# All services status
dsops rotation status

# Specific service
dsops rotation status postgres-prod

# JSON for monitoring
dsops rotation status --format json
```

**Output**: Service status with last rotation, next rotation, and current state.

---

#### `dsops rotation history`

View rotation event history.

```bash
dsops rotation history [flags] [service]
```

**Description**: Historical view of all rotation events for compliance and troubleshooting.

**Arguments**:
- `service` - Filter by service name (optional)

**Flags**:
- `--limit <n>` - Number of entries to show
- `--since <date>` - Events since date
- `--until <date>` - Events until date
- `--status <status>` - Filter by result status
- `--format <format>` - Output format

**Examples**:
```bash
# Recent rotation events
dsops rotation history --limit 20

# Service-specific history
dsops rotation history postgres-prod

# Compliance report
dsops rotation history --since 2024-Q1 --format json > q1-rotations.json
```

---

## Environment Variables

dsops supports these environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `DSOPS_CONFIG` | Configuration file path | `dsops.yaml` |
| `DSOPS_ENV` | Default environment name | - |
| `DSOPS_DEBUG` | Enable debug logging | `false` |
| `DSOPS_NO_COLOR` | Disable colored output | `false` |
| `DSOPS_ROTATION_DIR` | Rotation metadata storage | Platform default |
| `DSOPS_DATA_DIR` | dsops-data directory | `./dsops-data` |

## Exit Codes

dsops uses these exit codes:

| Code | Meaning | Description |
|------|---------|-------------|
| 0 | Success | Operation completed successfully |
| 1 | General Error | Unspecified error occurred |
| 2 | Configuration Error | Invalid configuration file |
| 3 | Provider Error | Provider authentication or access failed |
| 4 | Resolution Error | Secret resolution failed |
| 5 | Rotation Error | Secret rotation operation failed |
| 6 | Validation Error | Input validation failed |

## Common Patterns

### Development Workflow

```bash
# Initialize configuration
dsops init

# Check provider setup
dsops doctor

# Plan development environment
dsops plan --env development

# Execute development server
dsops exec --env development -- npm start
```

### Production Deployment

```bash
# Validate production configuration
dsops doctor --env production

# Check rotation status
dsops rotation status

# Deploy with production secrets
dsops exec --env production -- ./deploy.sh
```

### CI/CD Pipeline

```bash
# Non-interactive mode for automation
dsops --non-interactive exec --env staging -- npm test

# Generate config for containerized apps
dsops render --env production --out /app/secrets.env

# Rotate secrets on schedule
dsops secrets rotate --env prod --dry-run
```

### Debugging and Troubleshooting

```bash
# Debug configuration
dsops --debug doctor --verbose

# Inspect resolution plan
dsops plan --env staging --format json

# Check specific secret
dsops get --env prod --json DATABASE_URL

# Scan for leaks before commit
dsops leak --recursive src/
```

## Best Practices

### Security

1. **Use `exec` by default**: Prefer ephemeral injection over file rendering
2. **Enable git hooks**: Use `dsops install-hook` to prevent secret commits
3. **Regular rotation**: Set up automated rotation schedules
4. **Monitor access**: Track secret usage and rotation compliance

### Configuration

1. **Environment separation**: Use distinct configurations for dev/staging/prod
2. **Provider authentication**: Use managed identities when possible
3. **Validate regularly**: Run `dsops doctor` in CI/CD pipelines
4. **Version control**: Track dsops.yaml in version control (no secrets!)

### Operations

1. **Plan before execute**: Always use `dsops plan` to verify configuration
2. **Use non-interactive mode**: Set `--non-interactive` for automation
3. **Monitor rotation**: Set up alerts for failed rotations
4. **Audit history**: Regularly export rotation history for compliance

## Related Documentation

- [Configuration Reference](/reference/configuration/) - Complete dsops.yaml reference
- [Provider Documentation](/providers/) - Provider-specific guides
- [Rotation Strategies](/rotation/strategies/) - Secret rotation patterns
- [Security Guide](/security/) - Security best practices
- [Getting Started](/getting-started/) - Initial setup guides