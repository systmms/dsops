# dsops — Developer Secret Operations

[![Go Report Card](https://goreportcard.com/badge/github.com/systmms/dsops)](https://goreportcard.com/report/github.com/systmms/dsops)
[![codecov](https://codecov.io/gh/systmms/dsops/branch/main/graph/badge.svg)](https://codecov.io/gh/systmms/dsops)
[![Test Status](https://github.com/systmms/dsops/actions/workflows/test.yml/badge.svg)](https://github.com/systmms/dsops/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

> A fast, cross-platform CLI that pulls secrets from your vault(s) and renders `.env*` files or launches commands with ephemeral environment variables.

## Quick Start

```bash
# Initialize a new project
dsops init

# Preview what secrets will be resolved (no values shown)
dsops plan --env development

# Run your app with ephemeral environment variables (no files on disk)
dsops exec --env development -- npm start

# Optionally render a .env file (explicit opt-in)
dsops render --env production --out .env.production
```

## Features

- **Ephemeral First**: Secrets are injected into process environment, not written to disk by default
- **Provider Agnostic**: Works with password managers (1Password, Bitwarden) and cloud secret stores (AWS, GCP, Azure)
- **Safe by Default**: All logs redact sensitive values; no secrets in crash dumps
- **Flexible Output**: Generate `.env` files, JSON, YAML, or custom templates
- **Transform Pipeline**: Built-in transforms for JSON extraction, base64 encoding/decoding, and more
- **Cross Platform**: Works on macOS, Linux, and Windows

## Installation

### Homebrew (macOS/Linux)
```bash
brew install systmms/tap/dsops
```

### Go Install
```bash
go install github.com/systmms/dsops/cmd/dsops@latest
```

### Download Binary
Download the latest release from [GitHub Releases](https://github.com/systmms/dsops/releases).

## Configuration

Create a `dsops.yaml` file in your project root:

```yaml
version: 1

secretStores:
  onepassword:
    type: onepassword
  aws:
    type: aws.secretsmanager
    region: us-east-1

envs:
  development:
    DATABASE_URL:
      from: { store: onepassword, key: "op://Dev/MyApp/DATABASE_URL" }
    API_SECRET:
      from: { store: aws, key: "myapp/dev/api" }
      transform: json_extract:.secret
    DEBUG:
      literal: "true"
```

The legacy `providers:` format is still supported for backward compatibility.

## Commands

### Core Commands
| Command | Description |
|---------|-------------|
| `dsops init` | Initialize a new dsops configuration |
| `dsops plan --env <name>` | Preview which secrets will be resolved |
| `dsops exec --env <name> -- <command>` | Run command with ephemeral environment |
| `dsops render --env <name> --out <file>` | Generate environment file |
| `dsops get --key <var>` | Get a single secret value |
| `dsops doctor` | Check provider connectivity |
| `dsops providers` | List available providers |
| `dsops login <provider>` | Authenticate with a provider |
| `dsops completion <shell>` | Generate shell completions (bash, fish, zsh) |

### Secret Rotation Commands
| Command | Description |
|---------|-------------|
| `dsops secrets rotate` | Rotate secrets with configured strategy |
| `dsops secrets status` | Check rotation status |
| `dsops secrets history` | View rotation history |
| `dsops rotation rollback` | Rollback a failed rotation |

### Security Commands
| Command | Description |
|---------|-------------|
| `dsops guard` | Access control and security checks |
| `dsops leak` | Detect potential secret leaks |
| `dsops shred` | Securely wipe sensitive data |
| `dsops install-hook` | Install Git hooks for leak prevention |

## Supported Providers

### Password Managers
- **1Password** (`onepassword`) - via `op` CLI
- **Bitwarden** (`bitwarden`) - via `bw` CLI
- **Pass** (`pass`) - Unix password manager (zx2c4)
- **OS Keychain** (`keychain`) - macOS Keychain / Linux Secret Service

### Cloud Secret Stores
- **AWS Secrets Manager** (`aws.secretsmanager`)
- **AWS SSM Parameter Store** (`aws.ssm`)
- **AWS STS** (`aws.sts`) - temporary credentials with role assumption
- **AWS SSO** (`aws.sso`) - IAM Identity Center
- **AWS Unified** (`aws`) - intelligent routing across all AWS services
- **Google Cloud Secret Manager** (`gcp.secretmanager`)
- **GCP Unified** (`gcp`) - intelligent routing
- **Azure Key Vault** (`azure.keyvault`)
- **Azure Identity** (`azure.identity`) - Managed Identity / Service Principal
- **Azure Unified** (`azure`) - intelligent routing
- **HashiCorp Vault** (`vault`)

### Configuration Management
- **Doppler** (`doppler`) - centralized secrets management
- **Infisical** (`infisical`) - open-source secret management
- **Akeyless** (`akeyless`) - enterprise zero-knowledge vault

## Transforms

Built-in transforms for processing secret values:

```yaml
envs:
  production:
    DATABASE_URL:
      from: { store: aws, key: "db-config" }
      transform: json_extract:.url  # Extract JSON field

    JWT_KEY:
      from: { store: onepassword, key: "op://Prod/JWT/private_key" }
      transform: multiline_to_single  # Convert multiline to single line
```

Available transforms:
- `json_extract:.path` - Extract value from JSON
- `yaml_extract:.path` - Extract value from YAML
- `base64_decode` / `base64_encode` - Base64 operations
- `trim` - Remove whitespace
- `multiline_to_single` - Convert multiline strings
- `join:separator` - Join array values with separator
- Custom transform chains supported

## Secret Rotation

dsops includes a full-featured secret rotation engine:

- **Rotation Strategies**: Canary (single instance first), percentage rollout (progressive waves), service group coordination
- **Notifications**: Slack, email (SMTP), PagerDuty, and generic webhooks for rotation events
- **Rollback**: Automatic rollback on verification failure, manual rollback command
- **Health Monitoring**: SQL, HTTP, and custom script health checks to validate rotations
- **Metrics**: Prometheus metrics for success rate, duration, and health status

```yaml
services:
  postgres-prod:
    type: postgresql
    rotation:
      strategy: canary
      schedule: "0 2 * * 0"  # Weekly at 2am Sunday
      notifications:
        - type: slack
          channel: "#ops-alerts"
```

## Security

dsops is designed with security as the top priority:

- **No Disk Residue**: Secrets exist only in memory by default
- **Process Isolation**: Child processes get secrets; parent process never sees them
- **Redacted Logging**: All logs automatically redact sensitive values
- **Crash Safety**: Panic handler prevents secrets from appearing in crash dumps
- **Minimal Cache**: Optional encrypted keychain storage only

## Development

```bash
# Set up development environment
make setup

# Run tests
make test

# Build binary
make build

# Run with debug logging
make dev
```

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## Documentation

For detailed documentation, see the [docs](docs/) directory or visit our [documentation site](https://systmms.github.io/dsops).