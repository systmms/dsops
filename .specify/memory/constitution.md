# dsops Constitution

<!--
Sync Impact Report:
Version: 1.0.0 (Initial)
Created: 2025-11-11
Rationale: Establishing foundational principles for dsops spec-driven development
Templates Status: ✅ All templates aligned with constitution
-->

## Core Principles

### I. Ephemeral-First Architecture (NON-NEGOTIABLE)

**Secrets exist in memory only by default; file writes require explicit opt-in.**

- Primary workflow: `dsops exec` injects secrets into child process environment only
- Parent process never sees secret values (process isolation)
- File rendering requires explicit `--out` flag
- Generated files can be marked with `--ttl` for auto-deletion
- No secrets cached to disk without user consent
- Prefer ephemeral over persistent in all design decisions

**Why**: Minimize attack surface by eliminating disk residue. Secrets that never touch disk cannot be leaked via filesystem.

### II. Security by Default (NON-NEGOTIABLE)

**All logging automatically redacts sensitive values; security is not optional.**

- Use `logging.Secret()` wrapper for all values that could contain secrets
- `--debug` flag never prints raw secrets (only metadata like source, version)
- Panic handler scrubs memory dumps and disables crash uploads
- Local cache (if enabled) uses OS keychain protection (macOS Keychain, Windows DPAPI, Linux Secret Service)
- Configuration can reference secrets via providers; never store credentials inline in `dsops.yaml`
- All test fixtures use `optional: true` to gracefully handle missing auth

**Why**: Security failures are catastrophic. Build security into the foundation so it cannot be accidentally disabled.

### III. Provider-Agnostic Interfaces (NON-NEGOTIABLE)

**Secret stores and services are abstracted behind well-defined interfaces.**

Secret Store Provider Interface:
```go
type Provider interface {
    Name() string
    Resolve(ctx context.Context, ref Reference) (SecretValue, error)
    Describe(ctx context.Context, ref Reference) (Metadata, error)
    Capabilities() Capabilities
    Validate(ctx context.Context) error
}
```

Service Integration Interface:
```go
type Service interface {
    Name() string
    Type() string      // Maps to dsops-data ServiceType
    Protocol() string  // Which adapter to use
    Config() ServiceConfig
}
```

- New providers register in `internal/providers/registry.go`
- Each provider has a factory function that creates instances from configuration
- Protocol adapters route to implementations via `ServiceRegistry.RouteToAdapter()`
- Support exec-plugin protocol for external providers (`$PATH/dsops-provider-*`)

**Why**: Abstraction enables supporting 14+ providers today and hundreds tomorrow without changing core logic.

### IV. Data-Driven Service Architecture (NON-NEGOTIABLE)

**Service integrations use community-maintained definitions from dsops-data repository, not hardcoded implementations.**

- Service types defined in `dsops-data/providers/SERVICE_NAME/service-type.yaml`
- Capabilities, credential kinds, and protocols declared in data
- dsops code provides generic rotation engine + protocol adapters
- Service instances reference types: `type: postgresql` looks up definition
- Enables supporting 100+ services without 100 custom implementations

**Why**: Separating data from code allows community contributions of service definitions without code changes. Data-driven scales better than hardcoded.

### V. Developer Experience First

**Optimize for developer happiness with zero-surprise defaults and friendly diagnostics.**

- Commands named for intent: `plan`, `exec`, `render`, `doctor`, `rotate`
- Error messages explain problems and suggest solutions
- `dsops doctor` validates credentials and connectivity before failures
- Configuration uses YAML with clear naming (`secretStores`, `services`, `envs`)
- Examples provided for every major use case in `examples/` directory
- Documentation includes user guides, CLI references, and architecture docs

**Why**: A tool developers avoid using provides zero security benefit. Usability drives adoption.

### VI. Cross-Platform Support

**Full feature parity across macOS, Linux, and Windows.**

- Pure Go implementation for maximum portability
- Provider CLIs wrapped with cross-platform abstractions
- File paths use `filepath.Join()` and respect OS conventions
- Tests run on all platforms in CI
- Documentation notes platform-specific behavior where unavoidable

**Why**: Teams work on diverse platforms. Platform lock-in fragments the ecosystem.

### VII. Test-Driven Development (NON-NEGOTIABLE)

**All implementation follows Test-Driven Development; tests written before code.**

Testing categories:
- **Unit tests**: Individual components in isolation
- **Provider contract tests**: Validate all providers implement interfaces correctly
- **Integration tests**: Test with real provider CLIs (requires authentication)
- **Security tests**: Ensure secret redaction works correctly
- **Race detection**: Run tests with `-race` flag to catch concurrency bugs

Red-Green-Refactor cycle:
1. Write failing test that describes desired behavior
2. Implement minimal code to make test pass
3. Refactor with confidence that tests prevent regressions

**Why**: TDD ensures code correctness, prevents regressions, and documents expected behavior through tests.

### VIII. Explicit Over Implicit

**Require explicit user action for operations with security or side-effect implications.**

- Writing files requires `--out` flag (not automatic)
- Rotation requires explicit confirmation unless `--auto-approve` flag provided
- `dsops exec` shows warning if about to inject secrets into untrusted command
- `dsops render` warns when overwriting existing files
- Policies can enforce additional guardrails (gitignore, forbid commits)

**Why**: Explicit actions reduce accidents. Users should consciously opt into operations with security implications.

### IX. Deterministic and Reproducible

**Same configuration produces same results; lockfiles prevent surprise changes.**

- `dsops.lock` pins provider versions and secret metadata
- Resolution order is deterministic and documented
- Transform pipeline is stateless and pure (same input → same output)
- CI/CD can verify lockfile matches configuration

**Why**: Reproducibility enables debugging and prevents environment drift.

## Additional Constraints

### Terminology Precision

Use precise terminology consistently:
- **Secret Store**: Systems that store and retrieve secret values (AWS Secrets Manager, Vault, 1Password)
- **Service**: Systems that use secrets and support rotation (PostgreSQL, Stripe, GitHub)
- **Provider**: Historical term—clarify as "secret store provider" or "service provider"
- **Secret Value**: The actual credential data
- **Secret Reference**: Pointer using `store://` or `svc://` URI format

Never say "provider" alone; always specify "secret store provider" or "service provider."

### Configuration Versioning

- `version: 1` required in `dsops.yaml` (enables future breaking changes)
- Legacy `providers:` format supported for backward compatibility but deprecated
- New features use `secretStores:` and `services:` sections

### Documentation Requirements

Every feature requires:
- User-facing documentation in `docs/` Hugo site
- Working examples in `examples/` directory
- Architecture documentation in `docs/content/architecture/` if non-trivial
- CLI help text in command implementation
- Spec in `specs/` directory (going forward)

### Security Audit Trail

- Append-only audit log at `.dsops/audit.log` (redacted)
- Leak incidents recorded under `.dsops/incidents/` (redacted)
- Notifications sent to configured channels (Slack/webhook/GitHub Issues)
- Rotation history tracked with timestamps and principals

## Development Workflow

### Spec-Driven Development

1. **Draft spec** → Create specification in `specs/` using `/speckit.specify`
2. **Research** (if needed) → Document findings in `docs/content/contributing/research/` using research template
3. **ADR** (for architecture) → Create Architecture Decision Record in `docs/content/contributing/adr/` using ADR template
4. **Plan** → Generate implementation plan using `/speckit.plan`
5. **Tasks** → Break down into tasks using `/speckit.tasks`
6. **Implement** → TDD cycle: test → implement → refactor
7. **Document** → Update Hugo docs, examples, CLI help
8. **Update tracking** → Mark spec as implemented, update VISION.md if needed

### Code Review Standards

All PRs must:
- Include tests (unit + integration where applicable)
- Pass `make check` (lint + vet + tests)
- Include documentation updates
- Reference spec or ADR if applicable
- Demonstrate security test coverage for new secret-handling code

### Release Process

1. Update `CHANGELOG.md` from completed specs
2. Run full test suite across platforms
3. Build binaries for macOS, Linux, Windows (amd64 + arm64)
4. Tag release with semantic version
5. Generate GitHub release notes
6. Update documentation site

## Governance

### Constitutional Authority

This constitution supersedes all other practices and documentation. In case of conflict between this document and other docs, this document prevails.

### Amendment Process

1. Propose amendment via spec in `specs/future/`
2. Document rationale and impact analysis
3. Discuss with maintainers and community
4. Update constitution with version bump:
   - **MAJOR**: Backward-incompatible principle removal or redefinition
   - **MINOR**: New principle or section added
   - **PATCH**: Clarifications, wording improvements, typo fixes
5. Cascade changes to dependent templates and documentation
6. Commit with message: `docs: amend constitution to vX.Y.Z (description)`

### Compliance Verification

- All PRs reviewed for constitutional compliance
- Complexity must be justified (maintain simplicity)
- Security violations are blocking issues
- Use `CLAUDE.md` for AI assistant guidance on constitutional adherence

**Version**: 1.0.0 | **Ratified**: 2025-11-11 | **Last Amended**: 2025-11-11
