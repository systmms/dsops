# Phase 1 — Data Model: Bitwarden Multi-Account Support

This feature introduces no persistent data structures of its own. It adds
configuration fields and one in-memory entity inside the Bitwarden provider.

## Entities

### AccountProfile (in-memory, per provider instance)

A single Bitwarden account that one `type: bitwarden` provider instance is
pinned to. Lives in `BitwardenProvider` struct fields; never serialized.

| Field         | Type          | Optional | Source                | Description |
|---------------|---------------|----------|-----------------------|-------------|
| `appDataDir`  | `string`      | yes      | provider config       | Absolute path passed to `bw` as `BITWARDENCLI_APPDATA_DIR`. Empty → use `bw`'s platform default. |
| `server`      | `string`      | yes      | provider config       | Bitwarden server URL (e.g. `https://vault.bitwarden.com`, `https://vw.example.com`). Empty → don't reconcile. |
| `email`       | `string`      | yes      | provider config       | Expected user email. Empty → don't verify. |
| `observedEmail` | `string`    | yes      | `bw status` JSON      | The `userEmail` field from `bw status`; populated lazily by first auth check. |
| `observedServer` | `string`   | yes      | `bw status` JSON      | The `serverUrl` field from `bw status`; populated lazily. |
| `observedStatus` | `string`   | yes      | `bw status` JSON      | One of `unauthenticated`, `locked`, `unlocked`. |

**Lifecycle**:
1. Provider constructed → `appDataDir`/`server`/`email` populated from config.
2. First Resolve/Describe → `ensureAccount` runs once (gated by
   `sync.Once`): `bw status` → reconcile server (if mismatch) → verify email
   (if configured) → record `observedEmail` / `observedServer` / `observedStatus`.
3. Subsequent calls reuse the cached observation; no further `bw status`
   calls unless the provider is asked to re-validate.

### Validation Rules

| Rule | Trigger | Outcome |
|------|---------|---------|
| `appDataDir` must be an absolute path or expandable to one | config load | reject config with clear error naming provider instance |
| `appDataDir` parent directory must exist (the leaf may be created by `bw`) | first use | error if not present, citing the configured path |
| Two provider instances sharing `appDataDir` are reported per FR-007 | doctor pass / first use | warning at `doctor` time (always); configuration error at first use only if both instances are also `headless: true` |
| `server` (if set) must be a parseable URL with `http` or `https` scheme | config load | reject with clear error |
| `email` (if set) must be a valid email address as parsed by `net/mail.ParseAddress` | config load | reject with clear error |
| `bw status`'s `userEmail` must match configured `email` case-insensitively | first auth check | `AuthError` naming provider, expected, and observed values |

### State Transitions (observedStatus)

```
                ┌──────────────────┐
                │ unauthenticated  │
                └────────┬─────────┘
                         │  bw login ...
                         ▼
                ┌──────────────────┐
                │ locked           │
                └────────┬─────────┘
                         │  bw unlock --raw
                         ▼
                ┌──────────────────┐
                │ unlocked         │  ◄── steady state during dsops run
                └──────────────────┘
```

These transitions already exist in the pre-feature code; this feature does
not change them. They are reproduced for context only.

### Relationships

- One **provider instance** ↔ at most one **AccountProfile**.
- Many provider instances can coexist in one dsops.yaml; each holds an
  independent AccountProfile.
- No cross-instance relationship: there is no shared registry or pool of
  accounts at the dsops level.

## Configuration Schema Delta

See `contracts/config-schema.md` for the YAML-level contract.

## Non-Entities (explicit non-data)

- No new database tables, files, or persisted records owned by dsops.
- No mutation of `bw`'s `data.json` beyond what `bw` itself does in response
  to `bw config server` and the normal login/unlock/sync commands.
- No cache of secret values; the existing `dsops` cache (if used) is
  unaffected by these fields.
