---
title: "Bitwarden Secrets Manager"
description: "Configure dsops with Bitwarden Secrets Manager (bws) for CI/CD secret retrieval"
lead: "Bitwarden Secrets Manager is a separate product from the Bitwarden Password Manager, designed for developers and CI/CD workloads. dsops integrates with the bws CLI."
date: 2026-05-08T12:00:00-07:00
lastmod: 2026-05-08T12:00:00-07:00
draft: false
weight: 11
---

> Note: This is the **Secrets Manager** product, not Password Manager. Use the [`bitwarden`](./bitwarden) provider for vault items (logins, notes, cards, identities, SSH keys, attachments) and `bitwarden.secretsmanager` for Secrets Manager projects/secrets/machine accounts.

## Prerequisites

### Install the bws CLI

```bash
# Homebrew
brew install bitwarden-sdk-secrets

# Or download a release
# https://github.com/bitwarden/sdk-sm/releases
```

### Issue a Machine Account access token

In the Bitwarden Secrets Manager web UI, create or pick a Machine Account, grant it access to the project(s) you need, and generate an access token. Then:

```bash
export BWS_ACCESS_TOKEN="0.your-token-id.your-token-secret"
```

## Configuration

```yaml
version: 1

secretStores:
  bw-sm:
    type: bitwarden.secretsmanager
    # access_token_env: BWS_ACCESS_TOKEN     # default
    # server_url: https://vault.bitwarden.com  # for self-hosted
    # state_file: /var/lib/bws/state

envs:
  production:
    DATABASE_URL:
      from:
        store: store://bw-sm/11111111-1111-4111-8111-111111111111
```

Tokens are **never** read from yaml. The `access_token_env` setting names an env var; it does not accept the token value.

## Reference key formats

| Form | Example | Behavior |
|---|---|---|
| UUID | `7c1f9a4b-2b9d-4a35-9d07-4f3f7a3b9e4b` | Direct `bws secret get <uuid>`. |
| Path | `production/db_password` | Looks up project by name, lists project secrets, filters by key. |

The path form requires that project names be unique within your organization. If two projects share a name, dsops surfaces an `ambiguous project name` error rather than guessing.

### Retrieving the note instead of the value

Each Secrets Manager secret has both a `value` and a `note` field. To retrieve the note:

```yaml
DB_ROTATION_INFO:
  from:
    store: store://bw-sm/<uuid>
    field: note
```

## Capabilities

| Capability | Supported |
|---|---|
| Versioning | ❌ (bws does not expose history) |
| Metadata via `Describe` | ✅ |
| Watching / change notifications | ❌ |
| Binary values | ❌ (UTF-8 strings only) |
| Self-hosted | ✅ via `server_url` |

## Security notes

- The access token is currently passed to `bws` via the `--access-token` flag. This is **visible in `ps`** on shared hosts. Future work will pass it via the child-process environment instead. Until then, prefer running dsops on dedicated CI runners.
- dsops does not write the token to disk. The bws CLI itself may persist a state file under `~/.config/bws/` (override with `state_file:`).

## Troubleshooting

### `missing access token: set BWS_ACCESS_TOKEN`

The named env var is unset. Either export it, or override the env var name with `access_token_env:`.

### `auth probe (bws project list) failed`

The token is invalid or expired, or the bws CLI cannot reach the Bitwarden server. Run `bws project list` directly to confirm.

### `ambiguous project name`

Two projects in your organization share the same name. Address the secret by UUID instead, or rename one of the projects.

## Performance

The provider caches the project list and per-project secret lists for the lifetime of the dsops process. A single dsops invocation that resolves N secrets from one project will issue at most one `bws project list` and one `bws secret list --project-id ...` call, regardless of N. Resolution by UUID never lists projects.
