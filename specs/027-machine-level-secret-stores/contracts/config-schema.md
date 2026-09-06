# Contract: Machine-level user config file

**File**: `$XDG_CONFIG_HOME/dsops/config.yaml` → `~/.config/dsops/config.yaml`
(Linux/macOS) or `%APPDATA%\dsops\config.yaml` (Windows), unless overridden by
`--user-config <path>` or `DSOPS_USER_CONFIG=<path>`. The value `none` disables
it.

## Schema (informal)

```yaml
version: 0                      # optional; must be 0 when present
secretStores:                   # optional
  <name>:
    type: <secret store type>   # same types as the project file
    timeout_ms: <int>           # optional
    <provider-specific keys>    # e.g. address, region, appDataDir, email
providers:                      # optional, legacy section; needed for
  <name>:                       # keychain, infisical, akeyless,
    type: <provider type>       # bitwarden.secretsmanager
```

Any other top-level key (`envs`, `services`, `templates`, `transforms`,
`policies`, `notifications`, `metrics`, or a typo) is a configuration error.

## Precedence

1. Project `dsops.yaml` `secretStores:` / `services:` / `providers:` — always win.
2. User file `secretStores:` / `providers:` — fill remaining names only.

A user name that collides with any project name is reported as shadowed by
`dsops doctor` and otherwise ignored.

## Discovery order

`--user-config` > `DSOPS_USER_CONFIG` > `$XDG_CONFIG_HOME/dsops/config.yaml`
(absolute values only) > `~/.config/dsops/config.yaml` (non-Windows) >
`%APPDATA%\dsops\config.yaml` (Windows). `~` and `~/` are expanded in explicit
values; `~user` is not.

## CLI surface

| Flag / env | Meaning |
|------------|---------|
| `--user-config <path>` | explicit user file; missing → error; `none` disables |
| `DSOPS_USER_CONFIG` | same as the flag; empty = unset |
| `--config <path>` | project file (default `DSOPS_CONFIG` or `dsops.yaml`) |
| `DSOPS_CONFIG` | default for `--config` |

## Error envelope

| Situation | Error type | Field |
|-----------|------------|-------|
| explicit path missing | ConfigError | `user-config` |
| path is a directory | ConfigError | `user-config` |
| unreadable | UserError | — |
| invalid YAML / not a mapping / bad structure | ConfigError | `user-config` |
| disallowed section(s) | ConfigError | `user-config` (message lists keys) |
| version ≠ 0 | ConfigError | `version` |
| duplicate name across sections | ConfigError | `user-config` |
| missing store at resolve time | ConfigError | `provider`; suggestion names both files |

## Output contract (`doctor`, `providers`)

```
Configuration sources:
  Project config: /abs/path/dsops.yaml
  User config:    /home/me/.config/dsops/config.yaml (2 store(s))
  ⚠ Store 'x' in /home/me/.config/dsops/config.yaml is shadowed by the project config

PROVIDER  TYPE     SOURCE   STATUS      MESSAGE
```

`User config:` reads `none found at <path>` when the default location has no
file, or `disabled` when turned off.

## nix / home-manager

```nix
xdg.configFile."dsops/config.yaml".text = builtins.toJSON {
  version = 0;
  secretStores = {
    work-vault = { type = "vault"; address = "https://vault.corp.example.com"; };
    bw-work = { type = "bitwarden"; appDataDir = "~/.config/dsops/bitwarden/work"; email = "you@example.com"; };
  };
};
```

JSON is valid YAML. The resulting nix-store file is 0444 and passes the
permission check.
