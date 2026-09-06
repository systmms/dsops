# Implementation Plan: Machine-level Secret Store Declarations

**Branch**: `claude/machine-level-secret-refs-g533q7`
**Spec dir**: `027-machine-level-secret-stores`
**Date**: 2026-09-06
**Spec**: [./spec.md](./spec.md)

## Summary

Add a user-level config file that declares secret stores only, discovered via
`--user-config` / `DSOPS_USER_CONFIG` / XDG / `~/.config`, merged underneath
the project `dsops.yaml` with project-wins precedence, with provenance shown
by `doctor` and `providers`. Also honour the long-documented `DSOPS_CONFIG`.

## Technical Context

**Language/Version**: Go 1.25 (existing project minimum)
**Primary Dependencies**: standard library + `gopkg.in/yaml.v3` (already used); no new modules.
**Storage**: Reads one extra YAML file; never writes it.
**Testing**: Table-driven unit tests with injected env/home lookups (no `t.Setenv` in parallel tests); command tests capture stdout; example-config tests load the committed examples.
**Target Platform**: Linux, macOS, Windows.
**Project Type**: Single-binary Go CLI; no new packages.
**Performance Goals**: One extra `stat`/read per invocation; negligible.
**Constraints**: Zero behaviour change without a user file; `internal/config` ≥85% coverage; no reading of the real home dir from `config.Load()`.
**Scale/Scope**: One new file in `internal/config`, small edits in `main.go`, resolver, `doctor`, `providers`, `plan`; docs, examples, spec.

## Constitution Check

- **I (secrets never on disk)**: the user file holds references only. PASS.
- **II (redacted logging)**: no secret values are logged; paths and store names only. PASS.
- **VI (cross-platform)**: XDG on unix, `%APPDATA%` on Windows; permission check skipped on Windows. PASS.
- **VII (TDD, ≥85% critical coverage)**: tests written first; `internal/config` 83.0% → 88.6%. PASS.
- **VIII (explicit over implicit)**: explicit paths error when missing; `none` sentinel; provenance displayed. PASS.
- **IX (reproducibility)**: same files → same merge; sorted iteration for deterministic shadow lists and output. PASS.

**Initial gate: PASS.** No Complexity Tracking entries needed.

## Project Structure

### Documentation (this feature)

```
specs/027-machine-level-secret-stores/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── tasks.md
├── contracts/config-schema.md
└── checklists/requirements.md
```

### Source Code (repository root)

```
internal/config/user_config.go          # discovery, restricted schema, merge, provenance, permission check
internal/config/user_config_test.go
internal/config/example_user_config_test.go
internal/config/config.go               # Config fields, Load() split, GetProvider suggestion
internal/resolve/resolver.go            # shared missing-provider suggestion
cmd/dsops/main.go (+ main_test.go)      # newRootCommand, --user-config, DSOPS_CONFIG
cmd/dsops/commands/doctor.go            # Configuration sources block, SOURCE column
cmd/dsops/commands/providers.go         # sources preamble, SOURCE column, sorted, warn on load error
cmd/dsops/commands/plan.go              # next-step hint names the user file
examples/user-config.yaml
examples/portable-project.yaml
docs/content/getting-started/configuration.md
docs/content/reference/{cli,configuration,status}.md
```

**Structure Decision**: Single-project Go CLI; no new packages. Discovery lives in the CLI layer so `config.Load()` stays hermetic.

## Phase Outputs

- Phase 0 research → [research.md](./research.md)
- Phase 1 design → [data-model.md](./data-model.md), [contracts/config-schema.md](./contracts/config-schema.md), [quickstart.md](./quickstart.md)
- Phase 2 tasks → [tasks.md](./tasks.md)

## Complexity Tracking

None.
