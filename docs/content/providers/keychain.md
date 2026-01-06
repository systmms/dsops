---
title: "OS Keychain"
description: "Use native OS credential storage with dsops"
lead: "Integrate with macOS Keychain and Linux Secret Service for secure local secret storage."
date: 2025-01-04T12:00:00-07:00
lastmod: 2025-01-04T12:00:00-07:00
draft: false
weight: 16
---

## Overview

The keychain provider retrieves secrets from your operating system's native credential storage:

- **macOS**: Keychain Services (hardware-backed on Apple Silicon)
- **Linux**: Secret Service D-Bus API (gnome-keyring, KWallet)

## Features

- **OS-Native Security**: Uses the operating system's secure credential storage
- **Hardware Backing**: On Apple Silicon Macs, secrets can be protected by the Secure Enclave
- **No External Dependencies**: Works completely offline
- **Touch ID Support**: On macOS, can require biometric authentication
- **Session Integration**: On Linux, integrates with desktop session

## Prerequisites

### macOS

Keychain is built into macOS. No additional installation required.

### Linux

Install a Secret Service-compatible keyring:

{{< tabs >}}
{{< tab "GNOME/Ubuntu" >}}
```bash
# Usually pre-installed with GNOME desktop
sudo apt-get install gnome-keyring libsecret-tools
```
{{< /tab >}}
{{< tab "KDE" >}}
```bash
# KWallet provides Secret Service support
sudo apt-get install kwalletd5
```
{{< /tab >}}
{{< tab "Headless" >}}
```bash
# For headless servers, use gnome-keyring-daemon
sudo apt-get install gnome-keyring

# Start the daemon
eval $(gnome-keyring-daemon --start)
export SSH_AUTH_SOCK
```
{{< /tab >}}
{{< /tabs >}}

## Configuration

Add keychain to your `dsops.yaml`:

```yaml
version: 1

secretStores:
  keychain:
    type: keychain
    # Optional: prefix for service names
    service_prefix: com.mycompany.myapp
    # Optional: macOS access group for app sharing
    access_group: TEAM_ID.com.mycompany.shared

envs:
  development:
    DATABASE_PASSWORD:
      from:
        store: keychain/myapp/database-password

    API_KEY:
      from:
        store: keychain/github.com/personal-token
```

## Key Format

Secrets are referenced using `service/account` format:

```
keychain/service-name/account-name
```

- **service-name**: The service or application identifier
- **account-name**: The specific credential name

## Adding Secrets

### macOS

{{< tabs >}}
{{< tab "Keychain Access" >}}
1. Open **Keychain Access** (Applications > Utilities)
2. File > **New Password Item**
3. Set **Keychain Item Name** as the service (e.g., `myapp`)
4. Set **Account Name** as the key (e.g., `database-password`)
5. Enter the password and save
{{< /tab >}}
{{< tab "Command Line" >}}
```bash
# Add a generic password
security add-generic-password \
  -a "database-password" \
  -s "myapp" \
  -w "your-secret-value" \
  -T "/usr/bin/security"

# Retrieve it
security find-generic-password \
  -a "database-password" \
  -s "myapp" \
  -w
```
{{< /tab >}}
{{< /tabs >}}

### Linux

```bash
# Store a secret
secret-tool store --label="My App Database Password" \
  service myapp \
  account database-password

# Retrieve a secret
secret-tool lookup service myapp account database-password
```

## Usage Examples

### Basic Usage

```yaml
version: 1

secretStores:
  keychain:
    type: keychain

envs:
  development:
    # Simple retrieval
    DATABASE_PASSWORD:
      from:
        store: keychain/postgres/dev-password

    # AWS credentials
    AWS_ACCESS_KEY_ID:
      from:
        store: keychain/aws/access-key-id

    AWS_SECRET_ACCESS_KEY:
      from:
        store: keychain/aws/secret-access-key

    # SSH passphrase
    SSH_PASSPHRASE:
      from:
        store: keychain/ssh/id_rsa-passphrase
      optional: true
```

### With Service Prefix

```yaml
secretStores:
  work:
    type: keychain
    service_prefix: com.company.devtools

envs:
  work:
    # Stored as "com.company.devtools.postgres/password"
    DB_PASSWORD:
      from:
        store: work/postgres/password

    SLACK_TOKEN:
      from:
        store: work/slack/bot-token
```

### Multiple Keystores

```yaml
secretStores:
  personal:
    type: keychain

  work:
    type: keychain
    service_prefix: com.company

envs:
  development:
    PERSONAL_TOKEN:
      from:
        store: personal/github.com/token

    WORK_TOKEN:
      from:
        store: work/github-enterprise/token
```

## Security Features

### macOS Security

- **Keychain Access Control**: Secrets can require password or Touch ID
- **Secure Enclave**: On M1/M2 Macs, keys can be hardware-protected
- **Access Groups**: Share secrets between related applications
- **ACLs**: Fine-grained access control lists

### Linux Security

- **Session-based**: Keyring unlocked with desktop session login
- **D-Bus Encryption**: Secure communication between applications
- **Multiple Keyrings**: Separate keyrings for different purposes

## Access Control (macOS)

### Setting Access Permissions

```bash
# Create item with specific app access
security add-generic-password \
  -a "api-key" \
  -s "myapp" \
  -w "secret-value" \
  -T "/Applications/MyApp.app/Contents/MacOS/MyApp" \
  -T "/usr/local/bin/dsops"

# View access list
security dump-keychain -a
```

### Requiring Password/Touch ID

Items can be configured to:
- Always require password
- Require Touch ID
- Allow access without prompt (default for dsops)

## Troubleshooting

### Common Issues

| Error | Cause | Solution |
|-------|-------|----------|
| `keychain not available` | Running on unsupported OS | Use macOS or Linux with Secret Service |
| `secret not found` | Item doesn't exist | Add the secret to your keychain |
| `access denied` | Permission denied | Update access list or re-add secret |
| `headless environment` | No GUI session | Set up keyring daemon for headless use |

### macOS Troubleshooting

```bash
# List all keychain items
security dump-keychain

# Find specific item
security find-generic-password -s "myapp" -a "api-key"

# Reset keychain (CAUTION: deletes all items)
security delete-keychain ~/Library/Keychains/login.keychain-db

# Check keychain status
security show-keychain-info
```

### Linux Troubleshooting

```bash
# Check if Secret Service is running
dbus-send --session --print-reply --dest=org.freedesktop.secrets \
  /org/freedesktop/secrets org.freedesktop.DBus.Introspectable.Introspect

# List all stored secrets
secret-tool search --all

# Unlock the keyring
gnome-keyring-daemon --unlock

# Check keyring status
secret-tool search --unlock service "*"
```

## Headless Server Usage

For CI/CD or headless servers on Linux:

```bash
# Start gnome-keyring-daemon
export $(gnome-keyring-daemon --start --components=secrets)

# Set password for the keyring (can be automated)
echo "keyring-password" | gnome-keyring-daemon --unlock

# Now dsops can access secrets
dsops exec --env production -- your-command
```

**Note**: Keychain is primarily designed for interactive use. For CI/CD pipelines, consider using cloud-based secret managers (AWS Secrets Manager, Vault, etc.) instead.

## Best Practices

### 1. Organize by Service

```
# Good organization
myapp/database-password
myapp/api-key
myapp/jwt-secret

github.com/personal-token
github.com/work-token
```

### 2. Use Descriptive Names

```bash
# Good
security add-generic-password -s "myapp-production" -a "database-password"

# Less clear
security add-generic-password -s "app" -a "pwd"
```

### 3. Limit Access

On macOS, restrict which applications can access secrets:

```bash
security add-generic-password \
  -T "/usr/local/bin/dsops" \
  -s "myapp" \
  -a "secret" \
  -w "value"
```

### 4. Regular Cleanup

Remove unused secrets:

```bash
# macOS
security delete-generic-password -s "old-app" -a "old-secret"

# Linux
secret-tool clear service old-app account old-secret
```

## Related Documentation

- [macOS Keychain Documentation](https://developer.apple.com/documentation/security/keychain_services)
- [GNOME Keyring](https://wiki.gnome.org/Projects/GnomeKeyring)
- [Secret Service API](https://specifications.freedesktop.org/secret-service/latest/)
