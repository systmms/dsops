---
title: "1Password"
description: "Integrate with 1Password for secure team password management"
lead: "Use 1Password as a secret provider for your development workflows with dsops."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 10
---

## Overview

1Password is a popular password manager for teams and enterprises. dsops integrates with 1Password through the official `op` CLI, providing secure access to your vaults and items.

## Prerequisites

1. **1Password Account**: Personal, team, or business account
2. **1Password CLI**: Install the `op` CLI tool
3. **Authentication**: Sign in to your account

### Installing 1Password CLI

{{< tabs >}}
{{< tab "macOS" >}}
```bash
brew install 1password-cli
```
{{< /tab >}}
{{< tab "Linux" >}}
```bash
# Download from 1Password
curl -sS https://downloads.1password.com/linux/cli/stable/op_linux_amd64_v2.20.0.zip -o op.zip
unzip op.zip && sudo mv op /usr/local/bin/
```
{{< /tab >}}
{{< tab "Windows" >}}
```powershell
# Download from https://1password.com/downloads/command-line/
# Or use Chocolatey:
choco install 1password-cli
```
{{< /tab >}}
{{< /tabs >}}

## Configuration

Add 1Password to your `dsops.yaml`:

```yaml
version: 1

secretStores:
  op:
    type: onepassword
    # Optional: specify account
    account: my-team.1password.com
    
envs:
  development:
    DATABASE_URL:
      from:
        store: op://Development/Database/url
    
    API_KEY:
      from:
        store: op://Development/API/key
      
    # Using item UUID (more stable)
    AWS_SECRET:
      from:
        store: op://vault_uuid/item_uuid/field
```

## Authentication

### Interactive Login

```bash
# Sign in to 1Password CLI
op signin

# Or with specific account
op signin --account my-team.1password.com

# dsops will use your active session
dsops exec --env development -- npm start
```

### Environment Variables

For CI/CD environments, use service accounts:

```bash
# Set service account token
export OP_SERVICE_ACCOUNT_TOKEN="ops_eyJhbGc..."

# dsops will automatically use the token
dsops render --env production --out .env
```

### Biometric Unlock

On supported systems, enable biometric unlock:

```bash
op signin --biometric
```

## Secret Reference Format

1Password uses the URI format for secret references:

### Standard Format
```
op://vault/item/field
```

### Examples
```yaml
# By vault and item name
PASSWORD: 
  from: op://Development/Database/password

# By UUID (more stable)
API_KEY:
  from: op://vault_uuid/item_uuid/field_name

# Default field (password)
SECRET:
  from: op://Development/MySecret

# Custom fields
CUSTOM:
  from: op://Development/MyItem/custom_field
```

### Special Fields

1Password supports various field types:

- `password` - Default password field
- `username` - Username field
- `url` - Website URL
- `notes` - Secure notes
- `otp` - One-time password (TOTP)
- Custom fields by name

## Advanced Usage

### Multiple Vaults

Access items across different vaults:

```yaml
envs:
  production:
    # Personal vault
    PERSONAL_TOKEN:
      from: op://Personal/GitHub/token
    
    # Shared vault  
    TEAM_SECRET:
      from: op://Team Shared/Production DB/password
    
    # Client vault
    CLIENT_API:
      from: op://Client Projects/ClientA/api_key
```

### Using Sections

For items with multiple sections:

```yaml
envs:
  development:
    # Access field in specific section
    DB_HOST:
      from: op://Development/Database/Section.host
    
    DB_PORT:
      from: op://Development/Database/Section.port
```

### Secret Sharing

Share secrets securely with team members:

1. Create shared vault in 1Password
2. Add team members with appropriate permissions
3. Reference in dsops.yaml

```yaml
secretStores:
  team:
    type: onepassword
    vault: "Team Secrets"  # Optional: default vault

envs:
  staging:
    SHARED_SECRET:
      from: op://Team Secrets/Shared API/key
```

## Best Practices

### 1. Use UUIDs for Stability

Item names can change, but UUIDs remain constant:

```yaml
# Better: Use UUIDs
API_KEY:
  from: op://vault_uuid/item_uuid/field

# Okay: Use names (less stable)
API_KEY:
  from: op://Development/API Key/key
```

### 2. Organize with Vaults

Structure your vaults by environment or purpose:

- `Development` - Local dev secrets
- `Staging` - Staging environment
- `Production` - Production secrets
- `Team Shared` - Shared team resources

### 3. Use Service Accounts for CI/CD

Never use personal accounts in automation:

```yaml
# .github/workflows/deploy.yml
env:
  OP_SERVICE_ACCOUNT_TOKEN: ${{ secrets.OP_SA_TOKEN }}
```

### 4. Enable MFA

Always enable multi-factor authentication for additional security.

## Troubleshooting

### Session Expired

```bash
# Check session status
op whoami

# Re-authenticate
op signin
```

### Item Not Found

```bash
# List available vaults
op vault list

# List items in vault
op item list --vault Development

# Get item details
op item get "Database" --vault Development
```

### Permission Denied

Ensure you have access to the vault and item:

1. Check vault permissions in 1Password
2. Verify team membership
3. Confirm item isn't in trash

### Performance Tips

1. **Use `--cache` flag**: Enable local caching for better performance
2. **Batch operations**: Retrieve multiple secrets in one operation
3. **Consider session length**: Adjust timeout for your workflow

## Security Considerations

1. **Never commit tokens**: Keep service account tokens secure
2. **Use least privilege**: Grant minimal necessary permissions
3. **Rotate tokens regularly**: Implement token rotation policy
4. **Audit access**: Review 1Password activity logs
5. **Secure workstations**: Use biometric unlock where available

## Error Messages

Common error messages and solutions:

| Error | Solution |
|-------|----------|
| `[ERROR] 2021/09/01 09:00:00 You are not currently signed in` | Run `op signin` |
| `[ERROR] Invalid session token` | Session expired, re-authenticate |
| `[ERROR] "Database" isn't an item in the "Development" vault` | Check item name and vault |
| `[ERROR] You aren't authorized to view this item` | Request access from vault owner |

## Related Documentation

- [1Password CLI Documentation](https://developer.1password.com/docs/cli/)
- [1Password Secret References](https://developer.1password.com/docs/cli/secret-references/)
- [Service Accounts Guide](https://developer.1password.com/docs/service-accounts/)