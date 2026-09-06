# Phase 1 — Data Model: Machine-level Secret Store Declarations

## Entities

### UserConfigSpec (input to `Config.Load()`)

| Field  | Type             | Notes |
|--------|------------------|-------|
| Path   | string           | Absolute path of the user file; `""` disables loading |
| Origin | UserConfigOrigin | `""` none, `flag`, `env`, `default` |

`Explicit()` is true for `flag`/`env`: a missing file is then an error.

### StoreSource (provenance, output of `Load()`)

| Field | Type       | Notes |
|-------|------------|-------|
| Scope | StoreScope | `project` or `user` |
| Path  | string     | Absolute path of the declaring file |

Recorded in `Config.StoreSources[name]` for every project secret store,
service and legacy provider, and for every user store that filled a gap.

### userConfigFile (restricted schema)

| Key          | Type                          | Required |
|--------------|-------------------------------|----------|
| version      | int (must be 0)               | no |
| secretStores | map[string]SecretStoreConfig  | no |
| providers    | map[string]ProviderConfig     | no |

Any other top-level key is rejected.

### Config additions

| Field                | Type                    | Set by |
|----------------------|-------------------------|--------|
| UserConfig           | UserConfigSpec          | CLI (`PersistentPreRun`) or tests |
| LoadedUserConfigPath | string                  | `Load()`; `""` when nothing contributed |
| StoreSources         | map[string]StoreSource  | `Load()` |
| ShadowedUserStores   | []string (sorted)       | `Load()` |
| LoadWarnings         | []string                | `Load()` |

## Merge algorithm

```
def  := loadProject()                     # required; errors as before
reset provenance state
mark every project store/service/provider as {project, abs(projectPath)}
user := loadUserConfig()                  # nil when disabled or default-missing
for name in sorted(user.secretStores):
    if name in def.secretStores ∪ def.services ∪ def.providers: shadowed += name
    else: def.secretStores[name] = store; sources[name] = {user, abs(userPath)}
same for user.providers → def.providers
LoadedUserConfigPath = abs(userPath)
```

## Validation rules (user file)

| Condition | Result |
|-----------|--------|
| default origin, file missing | skip (debug log) |
| explicit origin, file missing | ConfigError field `user-config` |
| path is a directory | ConfigError |
| unreadable | UserError |
| empty file | ok |
| invalid YAML / not a mapping | ConfigError |
| disallowed top-level key(s) | ConfigError listing all keys |
| version ≠ 0 | ConfigError field `version` |
| same name in secretStores and providers | ConfigError |
| file or parent dir mode & 0o022 ≠ 0 (non-Windows, symlink-resolved) | warning appended to LoadWarnings |

## Non-Entities

- No new on-disk state owned by dsops; the user file is read-only from dsops's
  point of view.
- No per-field merge between a project store and a user store.
