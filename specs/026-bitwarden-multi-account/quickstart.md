# Quickstart: Two Bitwarden Accounts in One dsops.yaml

A 5-minute walkthrough that exercises every story in the spec (P1, P2, P3).

## Prerequisites

- `dsops` built from this branch on `PATH`.
- `bw` CLI installed and on `PATH`.
- Two Bitwarden accounts you control:
  - **Personal**: any account on `https://vault.bitwarden.com`.
  - **Work**: any second account; can be cloud or self-hosted Vaultwarden.

## Step 1 — Initialize two isolated state directories

```bash
mkdir -p ~/.config/dsops/bw-personal ~/.config/dsops/bw-work

BITWARDENCLI_APPDATA_DIR=~/.config/dsops/bw-personal bw login
BITWARDENCLI_APPDATA_DIR=~/.config/dsops/bw-personal bw unlock --raw   # remember the printed token

BITWARDENCLI_APPDATA_DIR=~/.config/dsops/bw-work     bw config server https://vw.corp.example.com  # if self-hosted
BITWARDENCLI_APPDATA_DIR=~/.config/dsops/bw-work     bw login
BITWARDENCLI_APPDATA_DIR=~/.config/dsops/bw-work     bw unlock --raw
```

You now have two completely separate `bw` state directories. Sanity check:

```bash
BITWARDENCLI_APPDATA_DIR=~/.config/dsops/bw-personal bw status | jq '.serverUrl, .userEmail'
BITWARDENCLI_APPDATA_DIR=~/.config/dsops/bw-work     bw status | jq '.serverUrl, .userEmail'
```

## Step 2 — Write a dsops.yaml that uses both

```yaml
# ~/dsops-demo.yaml
version: 0   # current dsops loader only accepts version: 0

providers:
  bw-personal:
    type: bitwarden
    appDataDir: ~/.config/dsops/bw-personal
    email: alice@example.com           # the personal account's email

  bw-work:
    type: bitwarden
    appDataDir: ~/.config/dsops/bw-work
    server: https://vw.corp.example.com
    email: alice@corp.example.com

envs:
  dev:
    PERSONAL_API_KEY:
      from: store://bw-personal/<personal-item-id>.password
    WORK_DB_URL:
      from: store://bw-work/<work-item-id>.custom.database_url
```

> The Bitwarden provider parser recognises `item.<field>` for built-in
> Login/Card/Identity fields, `item.custom.<name>` for custom fields, and
> `item.attachment.<filename>` for binary attachments.

Replace the `<...-item-id>` placeholders with real item IDs from each
account (`bw list items | jq '.[].id'` against the corresponding state dir).

## Step 3 — Verify with `doctor` (US1, US2)

```bash
dsops doctor --config ~/dsops-demo.yaml
```

Expected (abbreviated):

```
provider bw-personal (bitwarden):
  appDataDir: /home/alice/.config/dsops/bw-personal
  server:     https://vault.bitwarden.com
  email:      alice@example.com   (status: unlocked)

provider bw-work (bitwarden):
  appDataDir: /home/alice/.config/dsops/bw-work
  server:     https://vw.corp.example.com
  email:      alice@corp.example.com   (status: unlocked)
```

If you intentionally point both providers at the same `appDataDir`, you'll
see a `⚠ shared with: <other-name>` line under each — exercising FR-007.

## Step 4 — Resolve secrets from both accounts in one call (US1)

```bash
dsops exec --config ~/dsops-demo.yaml --env dev -- env | grep -E '^(PERSONAL_API_KEY|WORK_DB_URL)='
```

Both variables resolve. Neither provider's CLI session is invalidated by
the other (this is the SC-001 acceptance signal).

## Step 5 — Headless / CI shape (US3)

`bw` reads its API-key + master-password env values (`BW_CLIENTID`,
`BW_CLIENTSECRET`, `BW_PASSWORD`) from its own process environment, which
is process-wide. SPEC-026 isolates *state directories*, not *credential
env vars*. The realistic CI shape is therefore **one dsops invocation per
account identity**, each in its own CI step with its own credentials.

Personal account config (`~/dsops-personal.yaml`):

```yaml
version: 0

providers:
  bw-personal:
    type: bitwarden
    appDataDir: /tmp/ci/bw-personal
    email: alice@example.com
    headless: true

envs:
  dev:
    PERSONAL_API_KEY:
      from: store://bw-personal/<personal-item-id>.password
```

Work account config (`~/dsops-work.yaml`):

```yaml
version: 0

providers:
  bw-work:
    type: bitwarden
    appDataDir: /tmp/ci/bw-work
    server: https://vw.corp.example.com
    email: alice@corp.example.com
    headless: true

envs:
  dev:
    WORK_DB_URL:
      from: store://bw-work/<work-item-id>.custom.database_url
```

Drive each in its own step:

```bash
# CI step A — personal account
BW_CLIENTID=$PERSONAL_CLIENT_ID \
BW_CLIENTSECRET=$PERSONAL_CLIENT_SECRET \
BW_PASSWORD=$PERSONAL_PASSWORD \
  dsops exec --config ~/dsops-personal.yaml --env dev -- \
    sh -c 'echo "$PERSONAL_API_KEY" | …'

# CI step B — work account
BW_CLIENTID=$WORK_CLIENT_ID \
BW_CLIENTSECRET=$WORK_CLIENT_SECRET \
BW_PASSWORD=$WORK_PASSWORD \
  dsops exec --config ~/dsops-work.yaml --env dev -- \
    sh -c 'echo "$WORK_DB_URL" | …'
```

Holding two distinct headless identities in a single dsops process is
explicitly out of scope for SPEC-026 (see the spec's "Note on credentials
in a single CI process"). A future per-instance credential-env-var
override would unblock that, but it's not implemented here.

## Step 6 — Negative tests

| Test | Expected outcome |
|------|-------------------|
| Configure `email: bob@example.com` against a state dir logged in as `alice@example.com`. | Fail fast with an `AuthError` naming `bw-personal` and both emails. |
| Configure `server: https://wrong.example.com` against a state dir already pointed at `vault.bitwarden.com`. | `bw config server` runs once at first use; if the URL is unreachable when `bw` next sync's, error names the provider and URL. |
| Configure two providers with the same `appDataDir` and both `headless: true`. | dsops refuses to run, error names both providers. |
| Omit all three new fields. | Identical behavior to a pre-feature dsops; same logs, same exit codes. |

## Cleanup

```bash
rm -rf ~/.config/dsops/bw-personal ~/.config/dsops/bw-work
```

The Bitwarden desktop GUI and any other `bw` CLI sessions you have under the
platform-default state dir are untouched.
