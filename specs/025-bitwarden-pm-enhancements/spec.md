# SPEC-025: Bitwarden Password Manager Provider Enhancements

**Status**: Implemented
**Feature Branch**: `claude/add-bitwarden-support-4BBwb`
**Provider Type**: `bitwarden`
**Related**:
- SPEC-010: Bitwarden Provider (initial implementation)
- SPEC-024: Bitwarden Secrets Manager Provider

## Summary

A capability comparison against Bitwarden's current product surface identified three gaps in the existing Password Manager provider. This spec captures the enhancements that closed them:

- **Truth-up** — implement features the user-facing docs claimed but the code didn't (`item.custom.field-name`, `item.attachment.filename`); remove docs for features that aren't going to ship (collection/folder filtering).
- **Item type coverage** — first-class extraction for Card, Identity, and the new SSH Key item type, plus per-item-type default field selection.
- **Headless / CI-friendly auth** — opt-in `bw sync` once-per-process and opt-in API-key login + `--passwordenv` unlock so dsops can authenticate to Bitwarden in CI without external scripting.

## User Stories

### User Story 1: Custom field and attachment retrieval (P1)

Users address Bitwarden custom fields and attachments using the syntax already documented in `docs/content/providers/bitwarden.md`.

**Acceptance Criteria**:
1. **Given** a key `item.custom.<name>`, **Then** dsops returns the value of the named custom field.
2. **Given** a key `item.attachment.<filename>`, **Then** dsops invokes `bw get attachment <filename> --itemid <id> --raw` and returns the bytes base64-encoded with `content_type` metadata.
3. **Given** a custom field name with dots (`aws.region`), **Then** the dot is preserved in the lookup.
4. **Given** an attachment filename with dots (`fullchain.pem`), **Then** the dot is preserved.

### User Story 2: Card / Identity / SSH Key item types (P1)

Users address the secret-bearing fields of Bitwarden Card, Identity, and SSH Key items by name, just like Login fields today.

**Acceptance Criteria**:
1. **Given** a Card item, **Then** `item.number`, `item.code` (alias `cvv`), `item.cardholderName`, `item.brand`, `item.expMonth`, `item.expYear` resolve.
2. **Given** an Identity item, **Then** all 18 documented identity fields resolve.
3. **Given** an SSH Key item, **Then** `privateKey`, `publicKey`, `keyFingerprint` resolve.
4. **Given** an item with no trailing field in the key, **Then** dsops applies a per-type default: Login→password, Card→number, Identity→email, SshKey→privateKey, Note→notes.

### User Story 3: Headless / CI-friendly auth (P2)

Users opt in to dsops driving the bw CLI through unauthenticated → unlocked transitions in CI, without needing external scripting.

**Acceptance Criteria**:
1. **Given** `headless: true` and `unauthenticated` status with `BW_CLIENTID`+`BW_CLIENTSECRET` set, **Then** dsops runs `bw login --apikey`.
2. **Given** `headless: true` and `locked` status with `BW_PASSWORD` set, **Then** dsops runs `bw unlock --passwordenv BW_PASSWORD --raw` and reuses the resulting session token.
3. **Given** `headless: true` but the required env vars are missing, **Then** dsops returns a clear `AuthError` naming the missing var.
4. **Given** `sync: true`, **Then** dsops runs `bw sync` exactly once per provider lifetime before the first Resolve/Describe.
5. **Given** the default config (both flags false), **Then** behavior is unchanged from SPEC-010.

## Implementation

### Files modified

- `internal/providers/bitwarden.go` — parseKey, extractField, per-type extractors, attachment retrieval, headless auth, sync gate
- `internal/providers/bitwarden_internal_test.go` — parseKey + custom-prefix tests, test helper for `bwLookPath` override
- `internal/providers/bitwarden_mock_test.go` — Card/Identity/SshKey/Note tests, attachment test, sync-once test, headless login/unlock tests
- `internal/providers/bitwarden_test.go` — capability flip (`SupportsBinary=true`)
- `docs/content/providers/bitwarden.md` — Item Types section, Headless / CI subsection, removal of Collections and Folders section
- `examples/bitwarden.yaml` — commented `sync` and `headless` examples

### Behavior changes

- Capability `SupportsBinary` flipped from `false` to `true` (binary attachments).
- `parseKey` default field changed from `"password"` to empty string; `extractField` applies a per-type default. The user-visible effect: addressing a non-Login item without a field used to error with "no password field found" and now returns the type's natural default. Existing flat-key Login configs are unaffected.
- New configs: `sync: bool`, `headless: bool` — both default false.

## Capabilities Table (post-implementation)

| Capability | Supported | Notes |
|---|---|---|
| Versioning | ❌ | bw CLI doesn't expose history |
| Metadata | ✅ | Describe returns revisionDate, organization, folder |
| List Secrets | ❌ | Out of scope |
| Rotation | ❌ | Out of scope; tracked separately |
| Encryption at Rest | ✅ | Bitwarden handles |
| Binary | ✅ | Attachments base64-encoded |

## Out of scope (explicit non-goals)

1. Collection / folder filtering (docs section was removed; future work)
2. Native Go SDK
3. Watching / change notifications
4. Per-call secret value caching beyond the existing item fetch
5. Editing SPEC-010 — the historical retrospective is preserved as-is.
