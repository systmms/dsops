---
title: "pass"
description: "Use the standard Unix password store with dsops"
lead: "Integrate with pass (password-store), the standard Unix password manager that uses GPG and Git."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 15
---

## Overview

[pass](https://www.passwordstore.org/) is a simple password store that keeps passwords inside GPG-encrypted files organized in a directory hierarchy. It's a lightweight, command-line based password manager that follows the Unix philosophy.

## Features

- **GPG Encryption**: Each password is encrypted with your GPG key
- **Git Integration**: Optional version control for password history
- **Simple Structure**: Passwords stored as files in `~/.password-store/`
- **Unix Philosophy**: Does one thing well, integrates with shell
- **No Database**: Plain text files, easy to backup and sync

## Prerequisites

1. **GPG Key**: A valid GPG key pair for encryption/decryption
2. **pass CLI**: The pass command-line tool installed
3. **Git** (optional): For version control of your password store

### Installation

{{< tabs >}}
{{< tab "macOS" >}}
```bash
brew install pass
```
{{< /tab >}}
{{< tab "Linux" >}}
```bash
# Debian/Ubuntu
sudo apt-get install pass

# Fedora
sudo dnf install pass

# Arch
sudo pacman -S pass
```
{{< /tab >}}
{{< tab "From Source" >}}
```bash
git clone https://git.zx2c4.com/password-store
cd password-store
sudo make install
```
{{< /tab >}}
{{< /tabs >}}

### Initial Setup

```bash
# Initialize pass with your GPG key
pass init "your-gpg-id@example.com"

# Optional: Initialize git
pass git init

# Add your first password
pass insert development/database
# Enter password when prompted

# Or generate a password
pass generate production/api-key 32
```

## Configuration

Add pass to your `dsops.yaml`:

```yaml
version: 1

secretStores:
  pass:
    type: pass
    # Optional: custom password store location
    path: ~/.password-store
    # Optional: specific GPG key
    gpg_key: your-gpg-id@example.com

envs:
  development:
    DATABASE_PASSWORD:
      from:
        store: development/database
    
    API_KEY:
      from:
        store: api/development/key
    
    # Multi-line secrets (first line is password)
    PRIVATE_KEY:
      from:
        store: keys/app-private
      transform: trim
```

## Secret Organization

### Directory Structure

pass uses a hierarchical directory structure:

```
~/.password-store/
├── development/
│   ├── database.gpg
│   ├── redis.gpg
│   └── api/
│       ├── stripe.gpg
│       └── github.gpg
├── production/
│   ├── database.gpg
│   └── api/
│       └── stripe.gpg
└── personal/
    └── github.gpg
```

### Naming Conventions

Use a consistent naming scheme:

```yaml
# Environment-based
development/service/credential
staging/service/credential
production/service/credential

# Service-based
database/development/password
database/production/password
api/stripe/development/key
api/stripe/production/key

# Project-based
projectA/development/database
projectA/production/database
projectB/development/api-key
```

## Usage Examples

### Basic Secrets

```yaml
envs:
  development:
    # Simple password
    DB_PASSWORD:
      from: development/postgres

    # Nested path
    STRIPE_KEY:
      from: api/stripe/development
    
    # With transforms
    REDIS_URL:
      from: development/redis-connection
      transform: trim
```

### Multi-line Secrets

pass stores additional data after the first line:

```bash
# Create multi-line secret
pass insert -m certificates/app
# Enter password: mysecret
# Enter additional data:
# -----BEGIN PRIVATE KEY-----
# MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEA...
# -----END PRIVATE KEY-----
# ^D
```

Access multi-line content:

```yaml
envs:
  production:
    # First line only (password)
    CERT_PASSWORD:
      from: certificates/app
      transform: trim
    
    # Full content (including first line)
    CERT_PRIVATE_KEY:
      from: certificates/app
      # Use custom script to extract certificate portion
```

### Team Sharing

Share passwords with team using GPG:

```bash
# Add team member's GPG key
pass init -p team-project "gpg-id-1@example.com" "gpg-id-2@example.com"

# Re-encrypt existing passwords
pass init "$(cat ~/.password-store/.gpg-id | tr '\n' ' ')"
```

## GPG Configuration

### Key Management

```bash
# List GPG keys
gpg --list-secret-keys

# Generate new GPG key
gpg --full-generate-key

# Export public key for sharing
gpg --export --armor your-email@example.com > public.key

# Import team member's key
gpg --import colleague-public.key
```

### GPG Agent Configuration

Configure GPG agent for better usability:

```bash
# ~/.gnupg/gpg-agent.conf
default-cache-ttl 3600
max-cache-ttl 86400
pinentry-program /usr/local/bin/pinentry-mac
```

## Git Integration

### Enable Version Control

```bash
# Initialize git repository
pass git init

# Add remote repository
pass git remote add origin git@github.com:team/password-store.git

# Push to remote
pass git push -u origin main

# Configure automatic commits
pass git config --bool pass.signcommits true
```

### Useful Git Commands

```bash
# View password history
pass git log development/database

# Revert password change
pass git revert HEAD

# Show who changed what
pass git blame production/api-key
```

## Best Practices

### 1. Secure Your GPG Key

- Use a strong passphrase
- Keep secure backups of your private key
- Consider using a hardware security key (YubiKey)

### 2. Organize Hierarchically

```
environment/
  └── service/
      └── credential
      
project/
  └── environment/
      └── service
```

### 3. Regular Backups

```bash
# Backup password store
tar -czf pass-backup-$(date +%Y%m%d).tar.gz ~/.password-store

# Backup GPG keys
gpg --export-secret-keys --armor > gpg-backup.asc
```

### 4. Use Generated Passwords

```bash
# Generate secure passwords
pass generate development/new-service 32

# Generate without symbols
pass generate -n production/api-key 40
```

## Security Considerations

### File Permissions

pass automatically sets secure permissions:

```bash
~/.password-store/: 700 (drwx------)
*.gpg files: 600 (-rw-------)
```

### Clipboard Security

```bash
# Copy password to clipboard (clears after 45 seconds)
pass -c development/database

# Adjust clipboard timeout
PASSWORD_STORE_CLIP_TIME=10 pass -c production/api
```

### Secure Deletion

When removing passwords:

```bash
# Secure removal
pass rm production/old-api-key

# Remove from git history (if using git)
pass git filter-branch --tree-filter 'rm -f production/old-api-key.gpg' HEAD
```

## Troubleshooting

### GPG Issues

```bash
# GPG key not found
export GPG_TTY=$(tty)
echo "test" | gpg --clearsign

# Permission denied
chmod 700 ~/.gnupg
chmod 600 ~/.gnupg/*

# Agent not running
gpg-agent --daemon
```

### pass Command Errors

| Error | Solution |
|-------|----------|
| `gpg: decryption failed: No secret key` | Import your private GPG key |
| `Password store not initialized` | Run `pass init your-gpg-id` |
| `gpg: public key decryption failed: Bad passphrase` | Check GPG passphrase, restart agent |
| `.password-store is not a git repository` | Run `pass git init` |

### Integration Issues

```bash
# Test pass is working
pass list

# Test specific password
pass show development/database

# Check dsops can access pass
dsops doctor
```

## Advanced Usage

### Custom Password Store Location

```yaml
secretStores:
  team-pass:
    type: pass
    path: /shared/team-passwords
  
  personal-pass:
    type: pass
    path: ~/.password-store-personal
```

### Multiple GPG Recipients

```bash
# Initialize with multiple recipients
pass init -p project "key1@example.com" "key2@example.com" "key3@example.com"

# Add new recipient
echo "new-key@example.com" >> .gpg-id
pass init $(cat .gpg-id)
```

### Integration with Other Tools

```bash
# Use with dmenu for GUI selection
passmenu

# Browser integration
# Firefox: PassFF extension
# Chrome: browserpass extension

# Mobile access
# Android: Password Store app
# iOS: Pass for iOS
```

## Migration Guide

### From Other Password Managers

```bash
# From KeePass
keepass2pass.py keepass.xml

# From LastPass
lastpass2pass.rb

# From 1Password
1password2pass.rb

# Manual import
echo "mypassword" | pass insert -e migration/service
```

## Related Documentation

- [pass Official Documentation](https://www.passwordstore.org/)
- [pass Git Repository](https://git.zx2c4.com/password-store/)
- [GPG Best Practices](https://riseup.net/en/security/message-security/openpgp/gpg-best-practices)
- [pass Extensions](https://github.com/roddhjav/pass-extensions)