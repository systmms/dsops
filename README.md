# dsops — Developer Secret Operations

[![Go Report Card](https://goreportcard.com/badge/github.com/systmms/dsops)](https://goreportcard.com/report/github.com/systmms/dsops)
[![codecov](https://codecov.io/gh/systmms/dsops/branch/main/graph/badge.svg)](https://codecov.io/gh/systmms/dsops)
[![Test Status](https://github.com/systmms/dsops/actions/workflows/test.yml/badge.svg)](https://github.com/systmms/dsops/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

> A fast, cross-platform CLI that pulls secrets from your vault(s) and renders `.env*` files or launches commands with ephemeral environment variables.

## 🚀 Quick Start

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

## ✨ Features

- **Ephemeral First**: Secrets are injected into process environment, not written to disk by default
- **Provider Agnostic**: Works with password managers (1Password, Bitwarden) and cloud secret stores (AWS, GCP, Azure)
- **Safe by Default**: All logs redact sensitive values; no secrets in crash dumps
- **Flexible Output**: Generate `.env` files, JSON, YAML, or custom templates
- **Transform Pipeline**: Built-in transforms for JSON extraction, base64 encoding/decoding, and more
- **Cross Platform**: Works on macOS, Linux, and Windows

## 📦 Installation

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

## 🏗️ Configuration

Create a `dsops.yaml` file in your project root:

```yaml
version: 0

providers:
  onepassword:
    type: onepassword
  aws_sm:
    type: aws.secretsmanager
    region: us-east-1

envs:
  development:
    DATABASE_URL:
      from: { provider: onepassword, key: "op://Dev/MyApp/DATABASE_URL" }
    API_SECRET:
      from: { provider: aws_sm, key: "myapp/dev/api" }
      transform: json_extract:.secret
    DEBUG:
      literal: "true"
```

## 🔧 Commands

| Command | Description |
|---------|-------------|
| `dsops init` | Initialize a new dsops configuration |
| `dsops plan --env <name>` | Preview which secrets will be resolved |
| `dsops exec --env <name> -- <command>` | Run command with ephemeral environment |
| `dsops render --env <name> --out <file>` | Generate environment file |
| `dsops get --key <var>` | Get a single secret value |
| `dsops doctor` | Check provider connectivity |
| `dsops providers` | List available providers |

## 🔐 Supported Providers

### Password Managers
- **1Password** (`onepassword`) - via `op` CLI
- **Bitwarden** (`bitwarden`) - via `bw` CLI

### Cloud Secret Stores
- **AWS Secrets Manager** (`aws.secretsmanager`)
- **AWS Systems Manager Parameter Store** (`aws.ssm`)
- **Google Cloud Secret Manager** (`gcp.secretmanager`)
- **Azure Key Vault** (`azure.keyvault`)
- **HashiCorp Vault** (`hashicorp.vault`)

## 🔄 Transforms

Built-in transforms for processing secret values:

```yaml
envs:
  production:
    DATABASE_URL:
      from: { provider: aws_sm, key: "db-config" }
      transform: json_extract:.url  # Extract JSON field
    
    JWT_KEY:
      from: { provider: onepassword, key: "op://Prod/JWT/private_key" }
      transform: multiline_to_single  # Convert multiline to single line
```

Available transforms:
- `json_extract:.path` - Extract value from JSON
- `base64_decode` / `base64_encode` - Base64 operations
- `trim` - Remove whitespace
- `multiline_to_single` - Convert multiline strings
- Custom transform chains supported

## 🛡️ Security

dsops is designed with security as the top priority:

- **No Disk Residue**: Secrets exist only in memory by default
- **Process Isolation**: Child processes get secrets; parent process never sees them
- **Redacted Logging**: All logs automatically redact sensitive values
- **Crash Safety**: Panic handler prevents secrets from appearing in crash dumps
- **Minimal Cache**: Optional encrypted keychain storage only

## 🏃‍♂️ Development

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

## 📄 License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## 🤝 Contributing

We welcome contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## 📚 Documentation

For detailed documentation, see the [docs](docs/) directory or visit our [documentation site](https://systmms.github.io/dsops).