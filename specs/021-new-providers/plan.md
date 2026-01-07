# Implementation Plan: New Secret Store Providers

**Branch**: `021-new-providers` | **Date**: 2026-01-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/021-new-providers/spec.md`

## Summary

Add three new secret store providers to dsops: OS Keychain (`keychain`), Infisical (`infisical`), and Akeyless (`akeyless`). Each provider implements the standard `provider.Provider` interface and integrates with the existing registry pattern. Providers support the shared requirements including 30-second network timeouts, per-process credential caching, and doctor command integration.

## Technical Context

**Language/Version**: Go 1.25+ (matches existing project)
**Primary Dependencies**:
- `github.com/keybase/go-keychain` (already in go.sum) - OS Keychain
- Infisical REST API (no official Go SDK, use standard HTTP client)
- `github.com/akeylesslabs/akeyless-go` - Akeyless SDK
**Storage**: N/A (providers retrieve from external stores)
**Testing**: `go test` with table-driven tests, provider contract tests, mock interfaces
**Target Platform**: macOS, Linux (keychain); All platforms (Infisical, Akeyless)
**Project Type**: Single project (Go CLI tool)
**Performance Goals**: <2 second secret retrieval (excluding auth prompts)
**Constraints**: 30-second network timeout, per-process credential caching only

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Status | Evidence |
|-----------|--------|----------|
| I. Ephemeral-First | ✅ PASS | Providers only read secrets; no disk writes |
| II. Security by Default | ✅ PASS | Per-process caching only (FR-017); no disk credential storage |
| III. Provider-Agnostic Interfaces | ✅ PASS | All providers implement standard `provider.Provider` interface |
| IV. Data-Driven Service Architecture | ✅ N/A | These are secret stores, not services |
| V. Developer Experience First | ✅ PASS | Clear error messages with remediation (FR-013); doctor integration (FR-014) |
| VI. Cross-Platform Support | ⚠️ PARTIAL | Keychain is macOS/Linux only; Windows returns clear error |
| VII. Test-Driven Development | ✅ PASS | Contract tests required for all providers |
| VIII. Explicit Over Implicit | ✅ PASS | Configuration required; no auto-discovery |
| IX. Deterministic and Reproducible | ✅ PASS | Same config → same provider behavior |

**Gate Result**: ✅ PASS (Cross-platform partial is acceptable - documented in Out of Scope)

## Project Structure

### Documentation (this feature)

```text
specs/021-new-providers/
├── plan.md              # This file
├── spec.md              # Feature specification
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (Go interfaces)
│   ├── keychain.go
│   ├── infisical.go
│   └── akeyless.go
└── tasks.md             # Phase 2 output (/speckit.tasks)
```

### Source Code (repository root)

```text
internal/providers/
├── keychain.go              # OS Keychain provider implementation
├── keychain_test.go         # Unit tests
├── keychain_darwin.go       # macOS-specific implementation
├── keychain_linux.go        # Linux Secret Service implementation
├── keychain_unsupported.go  # Windows/other stub with clear error
├── infisical.go             # Infisical provider implementation
├── infisical_test.go        # Unit tests
├── infisical_client.go      # HTTP client wrapper (mockable)
├── akeyless.go              # Akeyless provider implementation
├── akeyless_test.go         # Unit tests
├── registry.go              # Updated with new factory registrations

tests/
├── contract/
│   └── provider_contract_test.go  # Contract tests include new providers
├── integration/
│   ├── keychain_integration_test.go   # Requires macOS/Linux
│   ├── infisical_integration_test.go  # Requires Infisical instance
│   └── akeyless_integration_test.go   # Requires Akeyless account
└── fakes/
    ├── fake_keychain.go      # Fake for unit tests
    ├── fake_infisical.go     # Fake HTTP responses
    └── fake_akeyless.go      # Fake SDK client

examples/
├── keychain.yaml             # Example configuration
├── infisical.yaml            # Example configuration
└── akeyless.yaml             # Example configuration

docs/content/providers/
├── keychain.md               # User documentation
├── infisical.md              # User documentation
└── akeyless.md               # User documentation
```

**Structure Decision**: Single project structure following existing patterns in `internal/providers/`. Platform-specific code uses Go build tags (`_darwin.go`, `_linux.go`, `_unsupported.go`).

## Complexity Tracking

No constitution violations requiring justification.

## Implementation Phases

### Phase 0: Research (Complete)

See [research.md](./research.md) for:
- Go-keychain library usage patterns
- Infisical REST API authentication flows
- Akeyless SDK integration patterns
- Platform detection best practices

### Phase 1: Design (Complete)

See [data-model.md](./data-model.md) for:
- Provider configuration structures
- Error types and handling
- State management (per-process caching)

See [contracts/](./contracts/) for:
- Provider factory function signatures
- Mock interface definitions
- Configuration validation contracts

### Phase 2: Tasks

Generated by `/speckit.tasks` command.

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Keychain prompts block CI | Medium | Medium | Detect headless environment, return clear error |
| Infisical API changes | Low | Medium | Version lock API calls, monitor changelog |
| Akeyless SDK breaking changes | Low | Low | Pin SDK version, test on upgrade |
| Platform detection edge cases | Medium | Low | Comprehensive platform testing in CI |

## Dependencies

**External Libraries (to add to go.mod):**
- `github.com/akeylesslabs/akeyless-go` - Akeyless SDK

**Existing Libraries (already in go.mod):**
- `github.com/keybase/go-keychain` - OS Keychain access

**No new dependencies needed for Infisical** - uses standard `net/http` client.
