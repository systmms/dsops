# Data Model: psst Provider

**Feature Branch**: `025-psst-provider`
**Date**: 2026-01-11

## Entities

### PsstProvider

The provider implementation that wraps psst CLI interaction.

```go
// PsstProvider implements provider.Provider for psst secret stores.
type PsstProvider struct {
    config   PsstConfig
    logger   *logging.Logger
    executor pkgexec.CommandExecutor
}
```

**Relationships**:
- Implements `provider.Provider` interface
- Uses `pkgexec.CommandExecutor` for CLI invocation (enables testing)
- Uses `logging.Logger` for debug output with secret redaction

**State**: Stateless - each Resolve/Describe call invokes psst CLI

### PsstConfig

Configuration options for the psst provider.

```go
// PsstConfig represents configuration for the psst provider.
type PsstConfig struct {
    // Env specifies the psst environment to use.
    // If empty, psst's default environment is used.
    // Maps to psst's --env flag.
    Env string `yaml:"env,omitempty"`
}
```

**Validation Rules**:
- `Env`: Optional string, no validation (psst will error if invalid)

**YAML Configuration Example**:
```yaml
secretStores:
  local:
    type: psst
    # Uses default environment

  staging:
    type: psst
    env: staging  # Uses psst's staging environment
```

### SecretReference (existing)

References a secret within the psst provider.

```go
// Reference usage for psst:
ref := provider.Reference{
    Provider: "psst",        // Provider name from config
    Key:      "API_KEY",     // Secret name in psst vault
    // Version: not supported by psst
    // Path: not used by psst
    // Field: not used by psst
}
```

**Mapping**:
- `Key` → psst secret name (passed to `psst get <key>`)

## Interface Implementation

### provider.Provider Methods

| Method | Implementation |
|--------|----------------|
| `Name()` | Returns `"psst"` |
| `Resolve(ctx, ref)` | Executes `psst get <ref.Key>`, returns plain text value |
| `Describe(ctx, ref)` | Executes `psst get <ref.Key>`, returns metadata (exists, size) |
| `Capabilities()` | Returns static capabilities struct |
| `Validate(ctx)` | Checks CLI availability, tests vault access with `psst list` |

### Capabilities

```go
provider.Capabilities{
    SupportsVersioning: false,  // psst has no versioning
    SupportsMetadata:   false,  // No secret metadata available
    SupportsWatching:   false,  // No change notifications
    SupportsBinary:     false,  // Text-only secrets
    RequiresAuth:       true,   // Requires OS keychain or PSST_PASSWORD
    AuthMethods:        []string{"keychain", "password"},
}
```

## Error Mapping

| psst Error Pattern | dsops Error Type |
|-------------------|------------------|
| Secret not found in vault | `provider.NotFoundError` |
| Keychain access failure | `provider.AuthError` |
| Vault locked | `provider.AuthError` |
| CLI not found | `dserrors.UserError` (validation) |

## CLI Command Patterns

### Secret Retrieval

```bash
# Default environment
psst get "SECRET_NAME"

# Specific environment
psst --env staging get "SECRET_NAME"
```

**Shell Escaping**: All secret names wrapped with `%q` format specifier to prevent injection.

### Validation

```bash
# Check CLI exists
which psst

# Test vault access
psst list
```

## Resolution Flow

```text
┌─────────────────────────────────────────────────────────────┐
│                     dsops Resolve()                         │
├─────────────────────────────────────────────────────────────┤
│ 1. Build command: psst [--env X] get "SECRET_NAME"          │
│ 2. Execute via pkgexec.CommandExecutor                      │
│ 3. Parse stdout → secret value                              │
│ 4. Parse stderr → error detection                           │
│ 5. Return SecretValue or mapped error                       │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                      psst CLI                                │
├─────────────────────────────────────────────────────────────┤
│ 1. Check local .psst/ vault                                  │
│ 2. Fall back to global ~/.psst/ vault                        │
│ 3. Fall back to environment variable                         │
│ 4. Return value or error                                     │
└─────────────────────────────────────────────────────────────┘
```

## Doctor Output (FR-011)

The provider should report resolution source in doctor output:

```text
Secret Store: local (psst)
  ✓ CLI found: /usr/local/bin/psst
  ✓ Vault accessible: ~/.psst/
  ✓ Environment: default

  Secrets:
    API_KEY .......... ✓ (resolved from vault)
    DEBUG_TOKEN ...... ✓ (resolved from env var fallback)
```
