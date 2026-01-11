# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**dsops** is a cross-platform CLI tool that pulls secrets from various providers (password managers like 1Password/Bitwarden, cloud secret stores like AWS Secrets Manager) and either renders environment files or executes commands with ephemeral environment variables. It's designed with security-first principles: secrets are never written to disk by default, all logging automatically redacts sensitive values, and the primary workflow is ephemeral execution.

## Project Documentation Structure

**Constitution** (`.specify/memory/constitution.md`): Defines governing principles, development philosophy, and non-negotiable architectural decisions. This is the source of truth for "why" dsops is built the way it is.

**Specifications** (`specs/`): Feature-specific implementation plans with user stories, acceptance criteria, and technical details. This is the source of truth for "how" features are built.

**Status Dashboard** (`docs/content/reference/status.md`): High-level implementation status with links to active specs and current priorities. **This is the single source of truth for "what's being worked on now".**

**Always update tracking documents when doing work**:
- **Active specs**: Update spec frontmatter (`Status: Draft` → `In Progress` → `Implemented`)
- **Status dashboard**: Update docs/content/reference/status.md when major milestones completed
- **Constitution**: Update only when adding/changing governing principles

**To find current priorities**: See [Status Dashboard](docs/content/reference/status.md) for active specifications and implementation focus areas.

**Note**: All VISION*.md documents have been retired. Implementation tracking is now exclusively via spec-kit specifications.

Current status: v0.1 MVP is 100% complete with Bitwarden, 1Password, and AWS Secrets Manager providers.

## Finding Current Work and Priorities

**Not sure what's being worked on?** → Check [Status Dashboard](docs/content/reference/status.md)

The status dashboard shows:
- ✅ **Active specifications** by category (testing, rotation, providers, etc.)
- ✅ **Current implementation progress** (% complete, what's next)
- ✅ **Priority features** for upcoming milestones
- ✅ **Links to detailed specs** for each active feature

**Examples**:
- Working on testing? → Status dashboard links to active testing spec
- Working on rotation? → Status dashboard links to active rotation specs
- Need to see roadmap? → Status dashboard has v0.2, v0.3+ milestones

**Do NOT reference specific spec numbers in CLAUDE.md** - they change status over time. Always reference status.md which is kept up-to-date.

## Development Environment

This project uses Nix for reproducible development environments:

```bash
# Enter development shell (has Go, tools, provider CLIs)
nix develop --impure

# Or use direnv for automatic activation
echo "use flake --impure" > .envrc && direnv allow
```

## Essential Commands

```bash
# Development workflow
make setup           # Setup dependencies
make build           # Build binary to ./bin/dsops
make test            # Run unit tests
make test-coverage   # Run tests with coverage report
make lint            # Run golangci-lint
make check           # Run lint + vet + tests

# Development helpers
make dev             # Build and run with --debug flag
make watch           # Auto-rebuild on file changes
make clean           # Clean build artifacts

# Testing specific components
go test -v ./internal/providers    # Test all providers
go test -v ./internal/resolve      # Test resolution logic
go test -race ./...                # Run with race detection

# Testing with real configs
./bin/dsops plan --config examples/test-1password.yaml --env test
./bin/dsops doctor --config examples/bitwarden.yaml

# Spec-Kit workflow (for feature development)
uv tool run --from specify-cli specify --help  # or use 'speckit' alias after sourcing .envrc
```

## Spec-Driven Development with Spec-Kit

This project uses [GitHub Spec-Kit](https://github.com/github/spec-kit) for specification-driven development. Specs live in `specs/` directory and follow a structured workflow.

### Using Spec-Kit

**Command format:** `uv tool run --from specify-cli specify [command]`
**Alias:** After sourcing `.envrc`, you can use `speckit [command]`

### Key Spec-Kit Commands

- **`speckit init`** - Initialize spec-kit in a new project (already done)
- **`/speckit.constitution`** - Review project constitution and principles
- **`/speckit.specify`** - Create a new feature specification
- **`/speckit.plan`** - Generate technical implementation plan from spec
- **`/speckit.tasks`** - Generate actionable task list from plan
- **`/speckit.implement`** - Execute all tasks systematically
- **`/speckit.clarify`** - Resolve ambiguities in specs
- **`/speckit.analyze`** - Check cross-artifact consistency
- **`/speckit.checklist`** - Quality validation checklist

### Spec Directory Structure

```
specs/
├── 001-cli-framework/           # Core CLI framework
├── 002-configuration-parsing/   # Config parsing
├── 003-secret-resolution-engine/ # Resolution engine
├── 004-transform-pipeline/      # Transform pipeline
├── 005-testing-strategy/        # Testing infrastructure
├── 006-plan-command/            # Plan command (dry-run)
├── 007-exec-command/            # Exec command (ephemeral execution)
├── 008-doctor-command/          # Doctor command (diagnostics)
├── 009-phase-5-completion/      # Rotation features
├── 010-bitwarden/               # Provider specs
├── 011-onepassword/
├── 012-literal/
├── 013-pass/
├── 014-doppler/
├── 015-vault/
├── 016-aws-secretsmanager/
├── 017-aws-ssm/
├── 018-azure-keyvault/
└── 019-gcp-secretmanager/
```

**Numbering**: Sequential (001, 002, 003...) - next spec uses highest existing number + 1

Each spec directory follows the standard spec-kit structure:
- `spec.md` - Feature specification (required)
- `plan.md` - Technical implementation plan (created by `/speckit.plan`)
- `tasks.md` - Task breakdown (created by `/speckit.tasks`)
- `research.md` - Research findings and decisions
- `data-model.md` - Data models and schemas
- `contracts/` - API contracts and interfaces

### When to Create Specs

- **New features**: Always create a spec before implementation
- **New providers**: Use spec-kit to document provider design
- **Architectural changes**: Create spec + ADR for major decisions
- **Bug fixes**: Small bugs don't need specs, but complex bug fixes may benefit from one

### Spec Lifecycle

1. **Draft** → Research and define requirements
2. **In Review** → Team reviews and provides feedback
3. **Accepted** → Ready for implementation planning
4. **In Progress** → Implementation underway
5. **Implemented** → Feature complete and tested
6. **Retrospective** → Created after-the-fact for existing features

### Integration with Existing Documentation

Specs complement but don't replace:
- **Constitution**: Governing principles and project philosophy (specs reference principles)
- **ADRs**: Architectural decisions (specs link to relevant ADRs)
- **Research docs**: Investigation findings (specs cite research)
- **Hugo docs**: User/developer guides (specs inform documentation)

## Terminology and Architecture

### Important Terminology

**Always use the correct terminology** defined in [docs/TERMINOLOGY.md](docs/TERMINOLOGY.md):

- **Secret Store**: Systems that **store and retrieve** secret values (AWS Secrets Manager, Vault, 1Password, etc.)
- **Service**: Systems that **use secrets** and can have their credentials rotated (PostgreSQL, Stripe API, GitHub, etc.)
- **Provider**: Historical term - now clarify as either "secret store provider" or "service provider"
- **Secret Value**: The actual credential data (password, API key, certificate)
- **Secret Reference**: A pointer to where a secret is stored (`store://`) or how it's rotated (`svc://`)

### Terminology Usage Guidelines for Claude

When discussing dsops architecture or implementation:

1. **Be specific**: Say "secret store" or "service", not just "provider"
2. **Use correct format**: `secretStores:` and `services:` sections, not `providers:`
3. **Reference dsops-data**: Services use community definitions, not hardcoded implementations
4. **URI format**: Use `store://` for retrieval, `svc://` for rotation
5. **Distinguish operations**: "retrieval" vs "rotation" have different purposes and components

**Correct usage examples:**
- ✅ "Add support for Azure Key Vault secret store"
- ✅ "Implement PostgreSQL service integration using dsops-data definitions"
- ✅ "The variable references store://aws/database/password for retrieval"
- ❌ "Add a new Azure provider" (ambiguous - store or service?)
- ❌ "Configure the postgres provider" (should be "service")

### Core Architecture

- **`pkg/provider/`**: Secret store provider interface - handles retrieval from storage systems
- **`internal/providers/`**: Secret store implementations (Bitwarden, 1Password, AWS Secrets Manager, etc.)
- **`internal/services/`**: Service integrations for rotation (PostgreSQL, Stripe, GitHub, etc.) using dsops-data
- **`internal/dsopsdata/`**: Loader for community service definitions from dsops-data repository
- **`internal/rotation/`**: Rotation engine with strategy support (two-key, immediate, overlap, etc.)
- **`internal/resolve/`**: Secret resolution engine that handles dependency graphs, transforms, and error aggregation
- **`internal/config/`**: Configuration parsing with `secretStores` vs `services` distinction
- **`internal/template/`**: Template rendering for various output formats (dotenv, JSON, YAML, Go templates)
- **`internal/logging/`**: Security-aware logging with automatic secret redaction using `logging.Secret()` wrapper
- **`internal/execenv/`**: Ephemeral process execution with environment injection
- **`cmd/dsops/commands/`**: CLI command implementations using Cobra framework

### Secret Store vs Service System

dsops has two distinct integration systems:

**Secret Store Providers** handle retrieval from storage systems. All secret stores implement the `provider.Provider` interface:

```go
type Provider interface {
    Name() string
    Resolve(ctx context.Context, ref Reference) (SecretValue, error)
    Describe(ctx context.Context, ref Reference) (Metadata, error)
    Capabilities() Capabilities
    Validate(ctx context.Context) error
}
```

New secret store providers are registered in `internal/providers/registry.go`. Each provider has a factory function that creates instances from configuration.

**Service Integrations** handle rotation targets using data-driven definitions from dsops-data repository. Services are configured in the `services:` section of dsops.yaml and use community-maintained service definitions rather than hardcoded implementations.

### Resolution Pipeline

1. **Configuration Loading**: `internal/config/config.go` parses `dsops.yaml`
2. **Provider Registration**: `internal/providers/registry.go` creates provider instances
3. **Resolution Planning**: `internal/resolve/resolver.go` builds dependency graph
4. **Secret Fetching**: Providers retrieve values via their `Resolve()` method
5. **Transform Pipeline**: `internal/resolve/transforms.go` applies transforms like `json_extract`, `base64_decode`
6. **Output Generation**: `internal/template/render.go` formats results

### Security Architecture

- **Ephemeral First**: Primary workflow is `dsops exec` which injects secrets into process environment only
- **Redacted Logging**: All logging uses `logging.Secret()` to automatically mask sensitive values
- **Memory Only**: Secrets exist only in memory during execution
- **Process Isolation**: Parent process never sees secret values, only child processes
- **Explicit File Opt-in**: Writing files requires explicit `--out` flag

## Configuration Structure

**Use the v1.0+ format** - `dsops.yaml` has this structure:
- **`secretStores`**: Where secrets are stored (AWS Secrets Manager, Vault, 1Password, etc.)
- **`services`**: What uses secrets and supports rotation (PostgreSQL, Stripe, GitHub, etc.)
- **`envs`**: Named environments containing variable definitions
- Each variable can reference both stores (`store://`) and services (`svc://`)

**Example:**
```yaml
version: 1

secretStores:
  aws-prod:
    type: aws.secretsmanager
    region: us-east-1

services:
  postgres-prod:
    type: postgresql  # Uses dsops-data definitions
    host: db.example.com

envs:
  production:
    DATABASE_URL:
      from:
        store: store://aws-prod/database/connection_string  # Where to get it
        service: svc://postgres-prod?kind=connection_string  # What uses it (for rotation)
```

**Legacy format** (`providers:`) is supported for backward compatibility but should be migrated to the new format.

## Development Guidelines

### Adding New Secret Store Providers

When adding new secret store providers:

1. Create provider implementation in `internal/providers/`
2. Implement the `provider.Provider` interface
3. Add factory function and register in `internal/providers/registry.go`
4. Add provider type to `cmd/dsops/commands/providers.go` descriptions
5. Create example configuration in `examples/`
6. Add documentation to `docs/PROVIDERS.md`
7. Create retrospective spec in `specs/providers/` (e.g., SPEC-090 for new provider)
8. Update `docs/content/reference/status.md` high-level metrics if needed

### Adding Service Support

When adding new service integrations:

1. **Prefer dsops-data definitions** - contribute service types to the community repository
2. Create service definitions in `dsops-data/providers/SERVICE_NAME/` with:
   - `service-type.yaml` - capabilities and credential kinds
   - `instances/` - example deployments
   - `policies/` - rotation schedules and rules
   - `principals/` - access control definitions
3. Service implementations use generic rotation engine + data-driven configuration
4. Update `SPEC-050` when completing rotation Phase 5 service integrations

## Provider Evaluation Framework

When evaluating whether to implement a new provider, use this decision framework:

### **Priority Scoring (1-10 scale)**

**Core Purpose (40% weight):**
- Does it primarily store secrets? (10 points)
- Does it provide authentication/credentials? (8 points)
- Does it manage configuration with secret references? (6 points)
- Is it a service-specific credential store? (3 points)
- Is it primarily for non-secret configuration? (1 point)

**Cloud/Platform Integration (25% weight):**
- Is it the platform's recommended secret management solution? (10 points)
- Does it follow platform authentication patterns? (8 points)
- Is it widely adopted in enterprise environments? (6 points)
- Is it a niche/specialized service? (3 points)
- Is it deprecated or being phased out? (0 points)

**Technical Merit (20% weight):**
- Supports versioning, metadata, encryption at rest? (10 points)
- Has robust SDK/API with good error handling? (8 points)
- Integrates well with platform IAM/RBAC? (6 points)
- Limited API or poor documentation? (3 points)
- Requires complex workarounds or hacks? (1 point)

**User Value (15% weight):**
- Solves common, widespread use cases? (10 points)
- Fills gaps in existing provider coverage? (8 points)
- Enables new deployment patterns? (6 points)
- Duplicates existing functionality? (3 points)
- Very narrow/specialized use case? (1 point)

### **Implementation Thresholds:**
- **8.0+ points**: High priority - implement immediately
- **6.0-7.9 points**: Medium priority - implement after core features
- **4.0-5.9 points**: Low priority - consider based on user requests
- **Below 4.0**: Skip unless compelling user demand

### **Examples:**
- **AWS Secrets Manager**: Core=10, Integration=10, Technical=10, Value=10 → **10.0** (Essential)
- **Azure Key Vault**: Core=10, Integration=10, Technical=9, Value=9 → **9.6** (Essential)
- **AWS Service-specific (RDS, etc.)**: Core=3, Integration=6, Technical=6, Value=4 → **4.6** (Low priority)
- **Deprecated service**: Core=varies, Integration=0, Technical=1, Value=1 → **Skip**

This framework ensures we prioritize providers that deliver maximum value while maintaining focus on core secret management functionality.

## Testing Strategy

dsops follows Test-Driven Development (TDD) as mandated by Constitution Principle VII. All new code must have tests written **before** implementation.

### Coverage Requirements

- **Overall target**: ≥80% coverage (enforced by CI)
- **Critical packages**: ≥85% (providers, resolve, config, rotation)
- **Standard packages**: ≥70% (commands, template, execenv)

### Test Categories

1. **Unit Tests** - Pure logic testing with no external dependencies
   - Use `t.Parallel()` for independent tests
   - Table-driven pattern for multiple cases
   - Test both success and error paths

2. **Provider Contract Tests** - Validate all providers implement `provider.Provider` interface consistently
   - Generic contract tests applied to all providers
   - Validates Resolve(), Describe(), Validate(), Capabilities()
   - Includes concurrency testing

3. **Integration Tests** - Test with real services using Docker
   - Skip with `testing.Short()` for fast local development
   - Use `testutil.StartDockerEnv()` for Docker lifecycle
   - Services: Vault, PostgreSQL, LocalStack, MongoDB

4. **Security Tests** - Validate secret redaction and prevent leaks
   - All secret-handling code must have redaction tests
   - Use `testutil.NewTestLogger()` to capture logs
   - Run with race detector (`-race` flag)

### TDD Workflow (Red-Green-Refactor)

**Always follow this cycle**:

1. **RED**: Write failing test that defines desired behavior
2. **GREEN**: Write minimal code to make test pass
3. **REFACTOR**: Improve code quality while keeping tests green
4. **REPEAT**: Continue for each acceptance criterion

### Running Tests

```bash
# Fast unit tests (local development)
make test
go test -short ./...

# With coverage report
make test-coverage

# Integration tests (requires Docker)
make test-integration

# Race detection (required before commit)
make test-race
go test -race ./...

# Full test suite
make test-all
```

### Test Utilities

- **`tests/testutil/`** - Test helpers and utilities
  - `TestConfigBuilder` - Programmatic config building
  - `FakeProvider` - Manual fake for unit tests
  - `DockerTestEnv` - Docker lifecycle management
  - `TestLogger` - Log capture for redaction tests

- **`tests/fakes/`** - Manual test doubles
  - `FakeProvider` - Fake `provider.Provider` implementation

- **`tests/fixtures/`** - Test data and configurations
  - Pre-built configs for common scenarios
  - Mock secret data (never real credentials)

### Documentation

**Comprehensive testing guides**:
- **[TDD Workflow Guide](docs/developer/tdd-workflow.md)** - Red-Green-Refactor cycle with examples
- **[Testing Strategy Guide](docs/developer/testing.md)** - Test categories, coverage requirements, best practices
- **[Test Patterns](docs/developer/test-patterns.md)** - Common patterns and ready-to-use examples
- **[Test Infrastructure Guide](tests/README.md)** - Docker setup, test utilities, troubleshooting

### CI/CD Enforcement

Pull requests automatically blocked if:
- ❌ Tests fail
- ❌ Coverage drops below 80%
- ❌ Race conditions detected
- ❌ New code lacks tests

### Best Practices

✅ **DO**:
- Write tests before implementation (TDD)
- Use table-driven tests for multiple cases
- Test error paths, not just happy paths
- Use `t.Parallel()` for independent tests
- Check `testing.Short()` to skip slow tests
- Use `FakeProvider` for unit tests
- Use `DockerTestEnv` for integration tests

❌ **DON'T**:
- Skip writing tests ("I'll add them later")
- Test implementation details (test behavior)
- Use Docker for pure logic tests
- Leak secrets in test fixtures
- Ignore test failures locally
- Commit failing tests

## Key Files to Know

**Documentation & Tracking**:
- **`.specify/memory/constitution.md`**: Project principles and governing philosophy
- **`docs/content/reference/status.md`**: **Single source of truth for current priorities, active specs, and implementation status**
- **`specs/`**: All feature specifications (organized by category: features/, rotation/, providers/, future/)

**Core Code**:
- **`internal/providers/registry.go`**: Provider registration system
- **`internal/resolve/resolver.go`**: Core resolution logic
- **`pkg/provider/provider.go`**: Provider interface definition
- **`internal/logging/logger.go`**: Security-aware logging system

**Documentation Resources**:
- **`docs/PROVIDERS.md`**: Provider-specific documentation
- **`docs/research/`**: Research findings - **always document research using the template**
- **`docs/DSOPS_DATA_INTEGRATION.md`**: Architecture for integrating with dsops-data repository
- **`examples/`**: Working configuration examples for testing and documentation

**Finding Current Work**: See [Status Dashboard](docs/content/reference/status.md) for active specs in testing, rotation, and other areas.

## Research Documentation

When conducting research (market analysis, technical investigations, provider evaluations):

1. **Always use the research template**: `docs/templates/research-template.md`
2. **Name files with date prefix**: `YYYY-MM-DD-topic-name.md`
3. **Document sources**: Include all URLs, tools tested, and documentation reviewed
4. **Focus on implications**: How findings affect dsops design and implementation
5. **Update related docs**: If research impacts constitution.md, specs, or other docs, update them

This ensures knowledge is preserved and decisions are traceable.

## Rotation Feature Development

When working on secret rotation features:

1. **Check current rotation specs**: See [Status Dashboard](docs/content/reference/status.md) for active rotation specifications
2. **Update active specs**: Mark features as started/in-progress/implemented in spec frontmatter
3. **Update constitution.md**: If adding new architectural principles or rotation philosophies
4. **Follow data-driven approach**: Use dsops-data for service definitions rather than hardcoding
5. **Test with dsops-data**: Use `--data-dir ./dsops-data` flag to test with community service definitions
6. **Document integration patterns**: Show how new features work with existing dsops-data schemas

**Note**: All rotation work is tracked via spec-kit specifications in `specs/rotation/`. Check status.md for current rotation priorities.

The rotation system is built on a data-driven architecture where service definitions come from the dsops-data repository, enabling support for hundreds of services without hardcoded implementations.

## Architecture Decision Records (ADRs)

When making significant architectural decisions (naming, interfaces, major design choices):

1. **Always create an ADR**: Use template `docs/templates/adr-template.md`
2. **Name with format**: `ADR-NNN-title.md` in `docs/adr/` directory
3. **Link to research**: Reference any research documents that influenced the decision
4. **Update when implemented**: Change status from Draft → Accepted → Implemented
5. **Track alternatives**: Document options considered and why they were rejected

ADRs provide decision history and rationale for future maintainers. See `docs/adr/README.md` for full guidelines.

**IMPORTANT**: When implementing features from ADR-001, always update the implementation status table in `docs/ADR-001-IMPACT-ANALYSIS.md`. Change status from ❌ **Not Started** → 🟢 **Started** → ✅ **Complete** as work progresses.

## Active Technologies
- Go 1.25+ (matches existing project) + GoReleaser (v2.x), GitHub Actions, Docker (020-release-distribution)
- N/A (stateless release infrastructure) (020-release-distribution)
- Go 1.25 (existing project), Bash (Makefile/CI), YAML (Lefthook config) + Lefthook (via npx - no global install required) (022-mod-tidy-check)
- N/A (no data persistence) (022-mod-tidy-check)
- Go 1.25 + GoReleaser v2, cosign (Sigstore), syft (SBOM), memguard (023-security-trust)
- N/A (documentation + CI/CD changes + runtime memory protection) (023-security-trust)

## Recent Changes
- 020-release-distribution: Added Go 1.25+ (matches existing project) + GoReleaser (v2.x), GitHub Actions, Docker
