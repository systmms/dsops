# Research: Machine-level Secret Store Declarations

**Date**: 2026-09-06
**Researcher**: dsops maintainers (with Claude Code)
**Type**: Technical Research
**Status**: Complete

## Executive Summary

dsops config was strictly project-local, so an open-source project could not
reference a secret store without hardcoding one maintainer's machine layout.
A user-level config file that declares *only* secret stores, merged underneath
the project file with project-wins precedence, gives portable project configs
while keeping a clean trust boundary. For a maintainer whose machine is managed
by nix, the file is a natural home-manager output: it contains references, not
secrets, so living in the world-readable, read-only nix store is a feature, not
a risk.

## Research Questions

- How should "this machine uses the company Vault" be expressed without
  putting it in every repo?
- What are the pros and cons for an open-source project whose maintainer uses
  nix but whose contributors may not?
- Where should the file live on Linux, macOS and Windows?
- How much of the schema should the user file be allowed to carry?
- Who wins when both files declare the same store name?
- What trust/permission checks are appropriate?

## Methodology

- Code review of `internal/config`, `cmd/dsops/main.go`, the resolver and the
  `doctor`/`providers` commands.
- Review of prior art: git config scoping (system/global/local), XDG Base
  Directory spec, home-manager `xdg.configFile`, direnv layering.
- Comparison of three mechanisms: user config file, `${VAR}` expansion,
  `extends:` base file.

## Key Findings

### Finding 1: Three candidate mechanisms; the user config file wins
**Description**: (a) A user-level file merged underneath the project; (b)
`${VAR}` expansion of project values from env vars nix sets; (c) `extends:`
pointing at a base file.
**Evidence**: (b) still hardcodes store *types* and every contributor must
export the same variables. (c) suits company-internal monorepos but the base
path is per-machine and must itself be discovered. (a) keeps the project file
free of any machine detail and maps 1:1 onto `xdg.configFile`.
**Decision**: (a).

### Finding 2: Pros for an open-source project + nix-managed machine
- **Portability**: `dsops.yaml` names stores and keys, never a home path,
  Vault URL, AWS profile, email or `appDataDir`. Anyone can clone it.
- **Declarative and reproducible**: bindings live in the dotfiles flake,
  versioned with the rest of the machine. `home-manager switch` rolls back.
  A rebuilt laptop restores them.
- **Nix store is fine**: the file holds only references, never secret values
  (constitution I). World-readable and read-only are acceptable. `builtins.toJSON`
  produces valid YAML for the existing loader.
- **Works without nix**: contributors hand-write `~/.config/dsops/config.yaml`
  from `examples/user-config.yaml` or point `DSOPS_USER_CONFIG` anywhere.
- **CI unaffected**: no file on the runner by default; `DSOPS_USER_CONFIG=none`
  pins it off; an explicit path pins it on.
- **Trust boundary**: the repo cannot reach into or override machine config;
  the machine file cannot inject `envs`, templates, policies or rotation
  config, so it can never cause a secret to be rendered somewhere the project
  did not ask for.

### Finding 3: Cons and mitigations
- **Indirection / "works on my machine"**: `store://work-vault/...` is
  meaningless until the reader knows the binding. → provenance in
  `doctor`/`providers`; missing-store errors name both files.
- **Read-only nix store**: dsops must never write the user file (no
  `dsops config add-store`, by design); every edit needs a rebuild.
- **Redirection point**: whoever can write the file can point `work-vault` at
  their own backend. → warn when file or parent dir (symlink target) is
  group/world-writable. No ownership check, because nix-store files are
  root-owned 0444 and must pass.
- **Type drift between contributors**: `work-vault` might be a `vault` on one
  machine and `literal` on another. → deferred `requires:` (Finding 7).
- **No deep merge**: a project entry fully replaces a same-named user entry.
- **Exposure**: a world-readable file reveals store addresses and emails to
  other local users (never secrets).
- **Windows**: no XDG convention; falls back to `%APPDATA%`.

### Finding 4: Path discovery
**Decision**: flag > `DSOPS_USER_CONFIG` > `$XDG_CONFIG_HOME/dsops/config.yaml`
> `~/.config/dsops/config.yaml` on every non-Windows OS including macOS >
`%APPDATA%\dsops\config.yaml`.
**Evidence**: home-manager's `xdg.configFile` writes to `~/.config` on both
Linux and macOS, so `os.UserConfigDir()` (`~/Library/Application Support`)
would never be hit by nix users. Matches the existing XDG style in
`internal/rotation/storage/file_storage.go`. A relative `XDG_CONFIG_HOME` is
ignored per the spec.

### Finding 5: Disable sentinel
**Decision**: the single value `none` for either the flag or the env var.
**Evidence**: one code path, identical semantics for flag and env, trivially
scriptable; an empty env string stays "unset" like `DSOPS_ROTATION_DIR`.

### Finding 6: Reject, don't warn, on disallowed sections
**Decision**: any key outside `version`/`secretStores`/`providers` is a
`ConfigError` listing every offending key. Keys are scanned from the YAML
node tree so typos such as `secretstores:` are caught too.
**Evidence**: a warning scrolls past and lets users believe `envs:` in the
machine file works; rejecting keeps the trust boundary crisp.

### Finding 7: `requires:` / type expectations — deferred
**Evidence**: it changes the *project* schema (older binaries would then
instantiate a `type: vault` store with no address and fail confusingly); it
is the one case needing user-overrides-project; `doctor`'s type + source
column already gives most of the value. Reserved as a separate top-level key.

### Finding 8: Hermeticity for tests
**Decision**: discovery runs in `main.go` `PersistentPreRun`; `Load()` only
consults `Config.UserConfig` when set. The zero value disables it.
**Evidence**: every existing test constructs `&config.Config{Path: ...}`; if
`Load()` self-discovered, a developer's real `~/.config/dsops/config.yaml`
would leak into the suite. `t.Setenv` is also incompatible with
`t.Parallel()`, so the lookup is injectable.

## Implications for dsops

- New `internal/config/user_config.go`; `Config` gains `UserConfig`,
  `StoreSources`, `ShadowedUserStores`, `LoadWarnings`, `LoadedUserConfigPath`.
- `dsops doctor` and `dsops providers` print a "Configuration sources" block
  and a `SOURCE` column.
- `DSOPS_CONFIG` (documented for a long time but never read) is now honoured
  as the default for `--config`.

## Sources

- XDG Base Directory Specification — https://specifications.freedesktop.org/basedir-spec/latest/
- home-manager `xdg.configFile` option — https://nix-community.github.io/home-manager/options.xhtml#opt-xdg.configFile
- git-config scopes (system/global/local) — https://git-scm.com/docs/git-config#SCOPES
- SPEC-002 future enhancement "Config Profiles"; SPEC-026 `appDataDir`
