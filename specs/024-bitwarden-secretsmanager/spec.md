# SPEC-024: Bitwarden Secrets Manager Provider

**Status**: Draft
**Feature Branch**: `claude/add-bitwarden-support-4BBwb`
**Provider Type**: `bitwarden.secretsmanager`
**Related**:
- SPEC-010: Bitwarden (Password Manager) provider
- SPEC-002: Configuration Parsing
- SPEC-003: Secret Resolution Engine
- SPEC-005: Provider Registry

## Summary

Bitwarden ships two distinct products: **Password Manager** (already supported via the `bitwarden` provider) and **Secrets Manager** — a separate product positioned for developers and CI/CD workloads. Secrets Manager has its own CLI (`bws`), its own auth model (machine-account access tokens), and a flat data model (Organization → Projects → Secrets) that more closely matches dsops's primary use case than the human-oriented Password Manager.

This spec adds a new provider type `bitwarden.secretsmanager` that wraps the `bws` CLI and resolves Bitwarden Secrets Manager secrets via dsops's standard `Provider` interface.

## User Stories

### User Story 1: Authenticate with a Machine Account access token (P1)

CI pipelines authenticate to Bitwarden Secrets Manager with a long-lived access token tied to a machine account, supplied via the `BWS_ACCESS_TOKEN` environment variable.

**Acceptance Criteria**:
1. **Given** `BWS_ACCESS_TOKEN` is set, **Then** Validate succeeds.
2. **Given** the env var is unset, **Then** an `AuthError` names the missing var.
3. **Given** the var name has been overridden via `access_token_env`, **Then** dsops reads from that name instead.
4. **Given** the `bws` CLI is not in PATH, **Then** Validate returns a clear remediation error.

### User Story 2: Resolve secrets by UUID (P1)

Users retrieve Secrets Manager secrets by their UUID identifier.

**Acceptance Criteria**:
1. **Given** key `<uuid>`, **Then** dsops invokes `bws secret get <uuid>` and returns the secret's `value` field.
2. **Given** a non-existent UUID, **Then** a `NotFoundError` surfaces.
3. **Given** `Reference.Field == "note"`, **Then** dsops returns the secret's `note` field instead of `value`.

### User Story 3: Resolve secrets by `<projectName>/<secretKey>` path (P2)

Users address secrets by the human-readable project + key path. The provider lists projects and the project's secrets to disambiguate.

**Acceptance Criteria**:
1. **Given** key `myproject/db_password`, **Then** dsops resolves the project name → ID via `bws project list`, lists secrets via `bws secret list --project-id <id>`, filters by `key`, and returns the value.
2. **Given** an ambiguous key (multiple matches), **Then** an error names the candidate IDs.
3. **Given** project list / secret list are called multiple times, **Then** results are cached for the provider lifetime to avoid quadratic CLI invocations.

### User Story 4: Self-hosted Bitwarden Secrets Manager (P3)

Users running self-hosted Bitwarden override the server URL.

**Acceptance Criteria**:
1. **Given** `server_url: https://...`, **Then** all `bws` invocations include `--server-url <value>`.

## Implementation

### Architecture

**Key files**:
- `internal/providers/bitwarden_secretsmanager.go` — provider impl
- `internal/providers/bitwarden_secretsmanager_test.go` — public + contract tests, gated `DSOPS_TEST_BITWARDEN_SM=1`
- `internal/providers/bitwarden_secretsmanager_mock_test.go` — mock-executor unit tests
- `internal/providers/registry.go` — registration as `bitwarden.secretsmanager`
- `examples/bitwarden-secretsmanager.yaml` — example config
- `docs/content/providers/bitwarden-secretsmanager.md` — user docs

### Config schema

```yaml
secretStores:
  bw-sm:
    type: bitwarden.secretsmanager
    # access_token_env: BWS_ACCESS_TOKEN  # default
    # server_url: https://vault.bitwarden.com
```

Tokens are NEVER read from yaml. The `access_token_env` setting names an env var, not a value. The bws state directory is configured outside dsops via `bws config state-dir <path>` or `BWS_CONFIG_FILE`.

### Reference Key syntax

| Form | Example | Behavior |
|---|---|---|
| UUID | `7c1f9a4b-2b9d-4a35-9d07-4f3f7a3b9e4b` | direct `bws secret get <uuid>` |
| Path | `production/db_password` | resolve project name → ID, list project secrets, filter by key |

### Capabilities

```go
provider.Capabilities{
    SupportsVersioning: false,  // bws exposes revisionDate but no historical retrieval
    SupportsMetadata:   true,
    SupportsWatching:   false,
    SupportsBinary:     false,  // bws values are utf-8 strings
    RequiresAuth:       true,
    AuthMethods:        []string{"access-token"},
}
```

### Resolve flow

1. Read access token from env var (default `BWS_ACCESS_TOKEN`); empty → `AuthError`.
2. Detect key form: UUID (regex `^[0-9a-f]{8}-...`) → direct fetch. Otherwise treat as `<project>/<key>`.
3. Build base args: `--access-token <token> --output json` (+ `--server-url`, `--state-file` if set).
4. UUID form: run `bws secret get <uuid>`, parse JSON to `bwsSecret{id,key,value,note,projectId,creationDate,revisionDate}`.
5. Path form: ensure project list cached (`bws project list`); ensure secret list for that project cached (`bws secret list --project-id <id>`); filter by `Key`.
6. Return `SecretValue{Value, Version: revisionDate, UpdatedAt, Metadata: {provider, secret_id, project_id, secret_key}}`. If `ref.Field == "note"`, return the note instead of the value.

### Token plumbing

When the default `BWS_ACCESS_TOKEN` env var is used (the common case), dsops does NOT pass `--access-token` in argv. The `bws` child process inherits the variable from the parent environment and reads it natively, keeping the token out of `/proc/PID/cmdline`. When a custom `access_token_env` is configured, the token is passed via `--access-token` and is visible in `ps` — this trade-off is documented and recommends sticking to the default env var on shared hosts.

## Capabilities Table

| Capability | Supported | Notes |
|---|---|---|
| Versioning | ❌ | `bws` does not expose version history |
| Metadata | ✅ | `Describe()` returns revisionDate, projectID, etc. |
| List Secrets | ❌ | Not part of v1 (would require adding a list operation to the Provider interface) |
| Rotation | ❌ | `bws secret edit` exists but rotation belongs in a separate spec |
| Encryption at Rest | ✅ | Bitwarden handles it server-side |
| Self-hosted | ✅ | Via `server_url` config |

## Out of scope (v1)

1. Native Go SDK — community-tracked, unstable. Stick with CLI wrapper.
2. Implementing the `Rotator` interface for SM secrets.
3. Token rotation / refresh.
4. Watching / change notifications (no `bws` API for this).
5. Cross-call secret value caching (the project/secret-list cache is within-call only).
6. A unifying `bitwarden` umbrella provider that auto-routes to PM vs SM based on config shape.

## Testing strategy

**Unit (mock executor)**:
- Resolve by UUID (success, NotFound, malformed JSON)
- Resolve by path (single-match, ambiguous, empty project)
- `ref.Field == "note"` returns note instead of value
- Validate: token present, token absent, `bws` not in PATH (via `bwsLookPath` override)
- Self-hosted: `--server-url` and `--state-file` are forwarded
- Project + secret list cache: list commands invoked at most once per project per provider lifetime

**Integration (real `bws`)**:
- Gated behind `DSOPS_TEST_BITWARDEN_SM=1` and `BWS_ACCESS_TOKEN`
- Requires a project with at least one secret in a Bitwarden Secrets Manager org
- Mirrors the existing `DSOPS_TEST_BITWARDEN=1` pattern

## Risks

- **`bws` JSON shape drift** — pin a tested minor version range in docs; integration tests catch shape changes.
- **`--access-token` flag visible in `ps`** — documented; replaceable when `pkgexec.CommandExecutor` gains env-passing.
- **Project name ambiguity** — projects can have duplicate names within an org; v1 surfaces an error rather than guessing.
