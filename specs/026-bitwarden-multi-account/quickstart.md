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
version: 1

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
      from: store://bw-work/<work-item-id>.fields.database_url
```

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

Replace the example with:

```yaml
providers:
  bw-personal:
    type: bitwarden
    appDataDir: /tmp/ci/bw-personal
    email: alice@example.com
    headless: true

  bw-work:
    type: bitwarden
    appDataDir: /tmp/ci/bw-work
    server: https://vw.corp.example.com
    email: alice@corp.example.com
    headless: true
```

Drive it with per-account credentials. dsops reads the *same* env-var names
that `bw` itself reads, so you need one set per account at the time of
invocation:

```bash
# Account A (personal) login + unlock for the first provider.
BW_CLIENTID=$PERSONAL_CLIENT_ID \
BW_CLIENTSECRET=$PERSONAL_CLIENT_SECRET \
BW_PASSWORD=$PERSONAL_PASSWORD \
  dsops exec --config ~/dsops-demo.yaml --env dev --only bw-personal -- env | grep PERSONAL_API_KEY

# Account B (work).
BW_CLIENTID=$WORK_CLIENT_ID \
BW_CLIENTSECRET=$WORK_CLIENT_SECRET \
BW_PASSWORD=$WORK_PASSWORD \
  dsops exec --config ~/dsops-demo.yaml --env dev --only bw-work -- env | grep WORK_DB_URL
```

In CI where both must resolve in one process, follow your CI provider's
docs on per-step env scoping; the dsops side will inject whichever values
are present on each `bw` subprocess.

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
