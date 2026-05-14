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
    # profile: default  # Optional: pass to bw via --session
    # sync: false       # Optional: if true, run `bw sync` once before first resolve
    # headless: false   # Optional: if true, attempt API-key login + passwordenv unlock

envs:
  development:
    DATABASE_PASSWORD:
      from: { provider: bitwarden, key: "dev-database.password" }
```

### Headless / CI/CD

When `headless: true` is set, dsops will recover from unauthenticated and locked vault states by invoking the bw CLI itself rather than asking the user to run commands manually. Three environment variables control this:

| Env var | Purpose |
|---|---|
| `BW_CLIENTID` | Client ID for `bw login --apikey`. Required for headless login from an unauthenticated state. |
| `BW_CLIENTSECRET` | Client secret for `bw login --apikey`. Required for headless login from an unauthenticated state. |
| `BW_PASSWORD` | Master password used by `bw unlock --passwordenv BW_PASSWORD --raw`. Required for headless unlock from a locked state. The session token bw returns is captured in-memory and reused for subsequent calls. |

These env vars are read by the bw CLI itself (via `--passwordenv`) or by dsops to gate the headless flow. dsops does not log them.

```yaml
providers:
  bitwarden:
    type: bitwarden
    headless: true
    sync: true
```

```bash
export BW_CLIENTID="..."
export BW_CLIENTSECRET="..."
export BW_PASSWORD="..."
dsops exec --env production -- ./deploy.sh
```

When `headless: false` (the default), dsops will surface clear errors telling the user to run `bw login` / `bw unlock` themselves; no CLI state is mutated.

### Key Formats

Bitwarden keys follow these patterns:

- **Simple**: `item-name` - Returns the password field
- **Field specific**: `item-name.field` - Returns a built-in field (`password`, `username`, `totp`, `notes`, `name`) or, as a back-compat fallback, a custom field whose name matches `field`
- **Custom field (explicit)**: `item-name.custom.field-name` - Returns the value of the custom field whose `Name` matches `field-name`. Names containing dots are preserved (e.g. `item.custom.aws.region` looks up custom field named `aws.region`).
- **Attachment**: `item-name.attachment.filename` - Returns the attachment bytes, base64-encoded. The `content_type` metadata is set to `application/octet-stream`. Filenames containing dots are preserved (e.g. `item.attachment.cert.pem`).
- **URI**: `item-name.uri`, `item-name.uri0`, `item-name.uri1`, ... - Returns an indexed URI from a Login item

> Item names cannot contain `.` characters. If your item name contains a dot, address it by item ID instead (`bw list items | jq -r '.[] | "\(.id)\t\(.name)"'`).

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
    
    # Custom field (explicit form recommended)
    DB_HOST:
      from: { provider: bitwarden, key: "Production Database.custom.hostname" }
    
    # Notes field
    DB_CONNECTION:
      from: { provider: bitwarden, key: "Production Database.notes" }

    # Attachment (base64-encoded)
    TLS_CERT:
      from: { provider: bitwarden, key: "tls-bundle.attachment.fullchain.pem" }
```

### Item Types

Bitwarden has five item types. The dsops provider supports all of them. When you address an item without a trailing field, dsops applies a per-type default.

| Type | Default field | Available fields |
|---|---|---|
| Login (1) | `password` | `password`, `username`, `totp`, `notes`, `name`, `uri`, `uri0..uriN`, custom fields |
| Note (2) | `notes` | `notes`, `name`, custom fields |
| Card (3) | `number` | `number`, `code` (alias `cvv`), `cardholderName`, `brand`, `expMonth`, `expYear`, `notes`, `name`, custom fields |
| Identity (4) | `email` | `title`, `firstName`, `middleName`, `lastName`, `address1`, `address2`, `address3`, `city`, `state`, `postalCode`, `country`, `company`, `email`, `phone`, `ssn`, `username`, `passportNumber`, `licenseNumber`, `notes`, `name`, custom fields |
| SSH Key (5) | `privateKey` | `privateKey`, `publicKey`, `keyFingerprint`, `notes`, `name`, custom fields |

Examples:

```yaml
envs:
  production:
    # SSH key (default field is privateKey)
    DEPLOY_KEY:
      from: { provider: bitwarden, key: "deploy-key" }

    # Card CVV
    CARD_CVV:
      from: { provider: bitwarden, key: "company-card.cvv" }

    # Identity email
    SUPPORT_EMAIL:
      from: { provider: bitwarden, key: "support-contact.email" }
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
## Multiple accounts (SPEC-026)

The Bitwarden desktop app supports keeping multiple accounts logged in at
once. To mirror that setup in dsops, declare one `type: bitwarden` provider
per account and pin each to its own `appDataDir`. dsops sets
`BITWARDENCLI_APPDATA_DIR` on every `bw` subprocess so each provider
operates against an isolated on-disk state — sessions, server URL, and
cached vault data don't bleed across.

Three optional per-instance fields enable this:

| Field        | Purpose                                                                            |
|--------------|------------------------------------------------------------------------------------|
| `appDataDir` | Absolute (or `~`-prefixed) path; injected as `BITWARDENCLI_APPDATA_DIR` per call.  |
| `server`     | Bitwarden server URL; reconciled via `bw config server` only when set + mismatched.|
| `email`      | Expected user email; verified case-insensitively against `bw status`'s userEmail.  |

Omitting all three is equivalent to the pre-SPEC-026 single-account
behavior (FR-008).

### Example

```yaml
version: 0

providers:
  bw-personal:
    type: bitwarden
    appDataDir: ~/.config/dsops/bw-personal
    email: alice@example.com

  bw-work:
    type: bitwarden
    appDataDir: ~/.config/dsops/bw-work
    server: https://vw.corp.example.com
    email: alice@corp.example.com

envs:
  dev:
    PERSONAL_API_KEY:
      from: { provider: bw-personal, key: "<personal-item-id>.password" }
    WORK_DB_URL:
      from: { provider: bw-work,     key: "<work-item-id>.custom.database_url" }
```

See [`examples/bitwarden-multi-account.yaml`](https://github.com/systmms/dsops/blob/main/examples/bitwarden-multi-account.yaml)
for the runnable worked example.

### Diagnostics with `dsops doctor`

`dsops doctor` adds a per-Bitwarden-instance block showing the resolved
`appDataDir`, configured `server`, and observed account email/status. If
two providers point at the same `appDataDir`, a `⚠ shared with:` warning
flags the collision under each block.

### Headless CI (one account per process)

`bw` reads `BW_CLIENTID`, `BW_CLIENTSECRET`, and `BW_PASSWORD` from its
own process environment, which is process-wide. SPEC-026 isolates
*state directories*, not *credential env vars*: two `headless: true`
providers in the same process cannot hold distinct identities. The
realistic CI shape is one dsops invocation per identity, each with its
own credential env vars and its own `appDataDir`.

Two headless providers sharing an `appDataDir` is a configuration error
(their `bw login` / `bw unlock` calls would race the same on-disk
state). dsops fails fast and names both providers plus the shared dir.

### Explicitly out of scope

- Reading the Bitwarden desktop app's local data file or any GUI-internal
  storage.
- Native messaging / IPC integration with a running Bitwarden desktop
  process.
- `bw serve` REST-API fast path.
- Per-variable account override (selecting an account at the `store://`
  reference level rather than the provider level).
