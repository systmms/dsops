---
title: "Bitwarden"
description: "Configure dsops with Bitwarden password manager"
lead: "Bitwarden is an open source password manager that works across all platforms. dsops integrates with the Bitwarden CLI for secure secret access."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 10
---

## Prerequisites

### Install Bitwarden CLI

{{< tabs >}}
{{< tab "npm" >}}
```bash
npm install -g @bitwarden/cli
```
{{< /tab >}}
{{< tab "Homebrew" >}}
```bash
brew install bitwarden-cli
```
{{< /tab >}}
{{< tab "Download" >}}
Download from [Bitwarden CLI releases](https://github.com/bitwarden/cli/releases)
{{< /tab >}}
{{< /tabs >}}

### Authenticate

```bash
# Login (one time)
bw login your-email@example.com

# Unlock vault (each session)
bw unlock
# This will give you a session key - export it:
export BW_SESSION="your-session-key"

# Verify authentication
bw status
```

## Configuration

### Basic Setup

```yaml
version: 0

providers:
  bitwarden:
    type: bitwarden
    # profile: default  # Optional: if you use multiple profiles

envs:
  development:
    DATABASE_PASSWORD:
      from: { provider: bitwarden, key: "dev-database.password" }
```

### Key Formats

Bitwarden keys follow these patterns:

- **Simple**: `item-name` - Returns the password field
- **Field specific**: `item-name.field` - Returns a specific field
- **Custom field**: `item-name.custom.field-name` - Returns custom field value
- **Attachment**: `item-name.attachment.filename` - Returns attachment content

### Examples

```yaml
envs:
  production:
    # Password field (default)
    DB_PASS:
      from: { provider: bitwarden, key: "Production Database" }
    
    # Username field
    DB_USER:
      from: { provider: bitwarden, key: "Production Database.username" }
    
    # Custom field
    DB_HOST:
      from: { provider: bitwarden, key: "Production Database.custom.hostname" }
    
    # Notes field
    DB_CONNECTION:
      from: { provider: bitwarden, key: "Production Database.notes" }
```

## Collections and Folders

If you use Bitwarden collections or folders:

```yaml
providers:
  bitwarden:
    type: bitwarden
    collection: "Development Team"  # Optional: filter by collection
    folder: "Databases"            # Optional: filter by folder
```

## Security Best Practices

1. **Session Management**
   - Never commit `BW_SESSION` to version control
   - Use `bw lock` when done
   - Sessions expire after 30 minutes of inactivity

2. **API Key Authentication** (Recommended for CI/CD)
   ```bash
   export BW_CLIENTID="your-client-id"
   export BW_CLIENTSECRET="your-client-secret"
   ```

3. **Vault Timeout**
   ```bash
   bw config server https://your-server.com  # For self-hosted
   bw login --apikey
   ```

## Troubleshooting

### Session Required
```
Error: Vault is locked
```
**Solution**: Run `bw unlock` and export the session key

### Item Not Found
```
Error: Item "foo" not found
```
**Solution**: Verify item name with `bw list items | grep foo`

### Multiple Items Found
```
Error: Multiple items found for "database"
```
**Solution**: Use more specific names or item IDs

## CI/CD Integration

For GitHub Actions:

```yaml
- name: Setup Bitwarden
  env:
    BW_CLIENTID: ${{ secrets.BW_CLIENTID }}
    BW_CLIENTSECRET: ${{ secrets.BW_CLIENTSECRET }}
  run: |
    npm install -g @bitwarden/cli
    bw login --apikey
    export BW_SESSION=$(bw unlock --raw)
    
- name: Deploy with secrets
  run: |
    dsops exec --env production -- ./deploy.sh
```