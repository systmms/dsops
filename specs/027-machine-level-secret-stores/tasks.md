# Tasks: Machine-level Secret Store Declarations

**Input**: Design documents from `specs/027-machine-level-secret-stores/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅, quickstart.md ✅
**Tests**: Required (constitution Principle VII). Tests for each story are written FIRST and MUST fail before implementation.
**Working branch**: `claude/machine-level-secret-refs-g533q7`

## Format: `[ID] [P?] [Story] Description`

## Phase 1: Setup

- [x] T001 Baseline: `go build ./...`, `go test -short -cover ./internal/config/` (83.0%)

## Phase 2: Foundational (`internal/config`)

- [x] T002 [P] RED: `TestResolveUserConfigPath`, `TestExpandHome`, `TestDefaultProjectConfigPath` in `user_config_test.go`
- [x] T003 GREEN: `ResolveUserConfigPath`, `DefaultUserConfigLookup`, `ExpandHome`, `DefaultProjectConfigPath` in `user_config.go`
- [x] T004 [P] RED: `Load()` tests — not configured, default missing, explicit missing, fills gaps/project wins, legacy providers, empty, disallowed sections, malformed, not-a-mapping, version, directory, unreadable, duplicate name, project missing, warnings + nil logger, reset between calls
- [x] T005 GREEN: `Config` fields, `loadProject`/`loadUserConfig`/`parseUserConfig`/`mergeUserConfig`, nil-safe logging
- [x] T006 [P] RED: `TestUserConfigPermissionWarnings` (modes, parent dir, symlink target, windows, stat failure)
- [x] T007 GREEN: `userConfigPermissionWarnings`
- [x] T008 `MissingProviderSuggestion`; `GetProvider` uses it (tests `TestMissingProviderSuggestion`, `TestGetProvider_NotFound_MentionsUserConfig`)
- [x] T009 `examples/user-config.yaml`, `examples/portable-project.yaml`, `example_user_config_test.go`

**Checkpoint**: `internal/config` 88.6% coverage; zero behaviour change without a user file.

## Phase 3: User Story 1 — Maintainer declares stores once (P1)

- [x] T010 [P] [US1] `cmd/dsops/main_test.go`: flag defined, `DSOPS_CONFIG` honoured, `none` sentinel, flag/env/XDG origins
- [x] T011 [US1] `main.go`: `newRootCommand`, `--user-config`, `DSOPS_CONFIG` default, discovery in `PersistentPreRun`
- [x] T012 [P] [US1] `TestResolverMissingProviderSuggestionMentionsUserConfig`
- [x] T013 [US1] `resolver.go`: both missing-provider sites use `MissingProviderSuggestion`

## Phase 4: User Story 2 — Contributor onboarding / provenance (P2)

- [x] T014 [P] [US2] doctor tests: sources block, none-found line, shadowed warning, disallowed section fails
- [x] T015 [US2] `doctor.go`: `renderConfigSources`, `Source` field, `SOURCE` column
- [x] T016 [P] [US2] providers tests: sources + sorted `SOURCE` column; broken user file still lists built-ins
- [x] T017 [US2] `providers.go`: sources preamble, `SOURCE` column, sorted names, warn on load error
- [x] T018 [US2] `plan.go`: next-step hint names the user file

## Phase 5: User Story 3 — CI pin/disable (P3)

- [x] T019 [US3] covered by `none` sentinel + explicit-missing error tests (T004, T010)

## Phase 6: Documentation

- [x] T020 [P] `docs/content/getting-started/configuration.md` "Machine-level secret stores" + nix snippet
- [x] T021 [P] `docs/content/reference/cli.md`, `docs/content/reference/configuration.md` flag/env tables
- [x] T022 [P] `docs/content/reference/status.md` SPEC-027 entry
- [x] T023 [P] `specs/002-configuration-parsing/spec.md` cross-reference; `CLAUDE.md` Active Technologies line

## Phase 7: Verification

- [x] T024 `go vet ./...`, `go test -short -race ./...`, `golangci-lint run`, `GOOS=windows/darwin go build ./cmd/dsops`
- [x] T025 Manual smoke per quickstart.md
