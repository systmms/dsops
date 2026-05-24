# Research: psst Provider Integration

**Date**: 2026-01-11
**Feature Branch**: `025-psst-provider`

## Summary

Research conducted during `/speckit.clarify` session to resolve technical unknowns for psst provider integration.

## Research Findings

### 1. psst Authentication Model

**Decision**: Delegate authentication entirely to psst CLI

**Rationale**:
- psst uses OS keychain by default (macOS Keychain, Linux libsecret, Windows Credential Manager)
- Authentication is transparent when user is logged in - no password needed
- Optional `psst lock` command encrypts vault with password for portability
- `PSST_PASSWORD` environment variable supports headless/CI contexts
- dsops cannot and should not manage psst's auth state

**Alternatives Considered**:
1. Pre-validate auth by running `psst list` during init → Rejected: adds complexity, could get out of sync
2. Check for `PSST_PASSWORD` in CI and warn if missing → Rejected: psst handles this already

**Sources**:
- https://github.com/Michaelliv/psst
- https://michaellivs.com/blog/psst-v013/

### 2. Secret Retrieval Method

**Decision**: Use `psst get <name>` plain output for retrieval

**Rationale**:
- `psst get <name>` returns raw secret value directly (no parsing needed)
- Simpler and less brittle than JSON parsing
- `--json` flag available for `psst list` if needed for metadata/doctor output

**Alternatives Considered**:
1. Use `psst get <name> --json` and parse → Rejected: adds parsing complexity, JSON schema undocumented
2. Use `psst export --json` to dump all secrets → Rejected: retrieves more than needed, memory overhead

### 3. Vault Path Configuration

**Decision**: Use psst's default vault precedence, only expose `env` option

**Rationale**:
- psst has intuitive precedence: local `.psst/` → global `~/.psst/`
- Developers expect project-local vaults to take precedence
- Adding custom vault path config adds complexity without benefit
- `--env` flag cleanly handles named environments within vaults

**Alternatives Considered**:
1. Expose `vault` config for custom paths → Rejected: psst's defaults work well, reduces config surface
2. Expose both `vault` and `scope` options → Rejected: over-engineering

### 4. Environment Variable Fallback

**Decision**: Document behavior and surface resolution source in `dsops doctor`

**Rationale**:
- psst automatically falls back to env vars if secret not in vault
- This is a feature, not a bug - enables flexible local development
- Users should understand the resolution chain for debugging
- No CLI flag exists to disable fallback - behavior is baked into psst

**Alternatives Considered**:
1. Bypass fallback with explicit flag → Not possible: psst has no such flag
2. Just document in provider docs → Partial solution, but doctor visibility helps debugging

### 5. Secret Names and Argument Safety

**Decision**: Pass secret names as discrete argv arguments via `pkg/exec.CommandExecutor`; do NOT shell-escape or apply `%q`

**Rationale**:
- The executor runs `exec.CommandContext(name, args...)` — arguments go straight to the process, there is no shell to interpret metacharacters, so command injection is not possible by construction
- This is correct for names with spaces, quotes, or special characters: each name is a single literal argument
- Manual quoting (e.g. Go's `%q`) would be actively wrong here — `os/exec` passes the argument verbatim, so psst would receive literal quote characters as part of the name and the lookup would fail. (`%q` produces Go-syntax string literals; it is not a shell-escaping tool.)

**Alternatives Considered**:
1. Wrap names with `%q` for "shell escaping" → **Rejected**: there is no shell; `%q` corrupts the argument with literal quotes (flagged in PR #54 review by both Gemini and Codex)
2. Reject special characters at config validation → Rejected: too restrictive, psst allows them

## psst CLI Reference

### Commands Used by Provider

| Command | Purpose | Output |
|---------|---------|--------|
| `psst get <name>` | Retrieve secret value | Plain text (secret value) |
| `psst list` | List all secrets | Text/JSON (with `--json`) |
| `psst --env <name> get <secret>` | Get from specific environment | Plain text |

### Environment Variables

| Variable | Purpose |
|----------|---------|
| `PSST_PASSWORD` | Vault password for headless/CI contexts |
| `PSST_ENV` | Default environment name |

### Vault Structure

```text
~/.psst/                    # Global vault (default)
~/.psst/envs/<name>/        # Named environments
.psst/                      # Project-local vault (takes precedence)
.psst/envs/<name>/          # Local named environments
```

## Implementation Implications

1. **Provider Config**: Only `env` field needed (optional)
2. **Validation**: Check `psst` CLI exists, run `psst list` to verify vault access
3. **Error Mapping**: Parse stderr for "not found" patterns → `NotFoundError`
4. **Capabilities**: `RequiresAuth=true`, `SupportsVersioning=false`, `SupportsBinary=false`
5. **Doctor Output**: Show resolution source (vault vs env fallback) per FR-011, via an optional `DoctorInfoProvider` interface that `doctor.go` type-asserts (core `Provider` interface unchanged)
