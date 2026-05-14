# Contract: dsops.yaml — Bitwarden Provider (multi-account)

Documents the YAML configuration surface that users author. This contract is
backwards-compatible: omitting all three new fields is equivalent to the
pre-feature provider.

## Schema (informal)

```yaml
providers:
  <name>:                       # string, dsops-internal identifier
    type: bitwarden             # unchanged

    # Existing (SPEC-010, SPEC-025)
    profile: <string>           # optional; passed as bw --session <value>
    sync: <bool>                # optional; default false
    headless: <bool>            # optional; default false

    # New (SPEC-026)
    appDataDir: <path>          # optional; absolute or ~-expandable filesystem path
    server: <url>               # optional; e.g. https://vault.bitwarden.com or https://vw.example.com
    email: <email>              # optional; expected user email for verification
```

## Field semantics

### `appDataDir` (new)

- **Type**: filesystem path (string).
- **Required**: no.
- **Default**: unset → use `bw`'s platform default (`~/.config/Bitwarden CLI`
  on Linux, `~/Library/Application Support/Bitwarden CLI` on macOS,
  `%AppData%\Bitwarden CLI` on Windows).
- **Effect**: dsops sets `BITWARDENCLI_APPDATA_DIR=<resolved abs path>` on
  every `bw` subprocess spawned by this provider instance, and only this
  instance.
- **Validation**: must resolve to an absolute path; parent directory must
  exist at first-use time.
- **Collision**: two instances sharing the same `appDataDir` is a warning at
  `doctor` time; an error at first use if both also set `headless: true`.

### `server` (new)

- **Type**: URL (string).
- **Required**: no.
- **Default**: unset → don't reconcile (whatever the state dir was last
  configured for is used).
- **Effect**: at first use per process, dsops runs `bw config server <url>`
  if `bw status`'s `serverUrl` differs from the configured value.
- **Validation**: must parse as a URL with `http` or `https` scheme.

### `email` (new)

- **Type**: email (string).
- **Required**: no.
- **Default**: unset → don't verify.
- **Effect**: at first use per process, dsops compares `bw status`'s
  `userEmail` (case-insensitive) against the configured value and returns an
  auth error on mismatch.
- **Validation**: must contain a single `@`; both sides non-empty.

## Example: two accounts side-by-side

```yaml
version: 1

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
    headless: true                # uses BW_CLIENTID/BW_CLIENTSECRET/BW_PASSWORD

envs:
  dev:
    PERSONAL_API_KEY:
      from: store://bw-personal/<item-id>.password
    WORK_DB_URL:
      from: store://bw-work/<item-id>.fields.database_url
```

## Backwards compatibility

A pre-feature config like:

```yaml
providers:
  bitwarden:
    type: bitwarden
    sync: true
    headless: true
```

continues to behave identically post-feature: `appDataDir`, `server`, and
`email` are all unset, so no env injection, no server reconciliation, and no
email verification take place. This is the basis for FR-008 and SC-003.

## Error envelope

Validation errors surface through the same plumbing the Bitwarden provider
already uses (`AuthError`, generic provider error). New error messages
always include:

- The provider instance name (`<name>` above).
- The offending field name.
- The offending value (with email redacted in logs per FR-006; shown
  plainly in user-facing error messages so the user can diagnose).
