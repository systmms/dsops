# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**dsops** is a cross-platform CLI tool that pulls secrets from various providers (password managers like 1Password/Bitwarden, cloud secret stores like AWS Secrets Manager) and either renders environment files or executes commands with ephemeral environment variables. It's designed with security-first principles: secrets are never written to disk by default, all logging automatically redacts sensitive values, and the primary workflow is ephemeral execution.

## Vision and Implementation Tracking

**VISION.md** defines the complete product vision, architecture, security model, and roadmap for dsops. It serves as the source of truth for what the tool should become.

**VISION_IMPLEMENTATION.md** is a living document that tracks implementation progress against VISION.md. It contains detailed tables showing completion status of all features, providers, commands, and components.

**VISION_ROTATE.md** defines the complete secret rotation vision, including data-driven architecture using dsops-data.

**VISION_ROTATE_IMPLEMENTATION.md** tracks implementation progress for secret rotation features, including the new data-driven service architecture.

**Always update vision documents when doing rotation work**:
- **VISION_ROTATE.md**: Update when adding new rotation features, capabilities, or architectural changes
- **VISION_ROTATE_IMPLEMENTATION.md**: Update when completing rotation features - change status from ❌ **Not Started** → 🟢 **Started** → ✅ **Complete**
- **VISION_IMPLEMENTATION.md**: Update when completing core features (non-rotation)

These documents are essential for tracking progress and planning future work.

Current status: v0.1 MVP is 100% complete with Bitwarden, 1Password, and AWS Secrets Manager providers.

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
├── 001-cli-framework/
│   └── spec.md                  # Retrospective spec
├── 002-configuration-parsing/
│   └── spec.md
├── 003-secret-resolution-engine/
│   └── spec.md
├── 004-transform-pipeline/
│   └── spec.md
├── 080-bitwarden/
│   └── spec.md                  # Provider retrospective specs
├── 081-onepassword/
│   └── spec.md
├── 082-literal/
│   └── spec.md
... (additional numbered spec directories)
└── future/                      # Unstructured future ideas
```

**Numbering convention:**
- **001-049**: Core feature specifications
- **050-079**: Rotation feature specifications
- **080-099**: Secret store provider specifications
- **100+**: Future/planned features

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
7. Update `VISION_IMPLEMENTATION.md` to mark provider as complete

### Adding Service Support

When adding new service integrations:

1. **Prefer dsops-data definitions** - contribute service types to the community repository
2. Create service definitions in `dsops-data/providers/SERVICE_NAME/` with:
   - `service-type.yaml` - capabilities and credential kinds
   - `instances/` - example deployments
   - `policies/` - rotation schedules and rules
   - `principals/` - access control definitions
3. Service implementations use generic rotation engine + data-driven configuration
4. Update `VISION_ROTATE_IMPLEMENTATION.md` when completing service integrations

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

- **Unit Tests**: Test individual components in isolation
- **Provider Contract Tests**: Validate all providers implement the interface correctly
- **Integration Tests**: Test with real provider CLIs (requires authentication)
- **Security Tests**: Ensure secret redaction works correctly
- Use `optional: true` in test configurations to gracefully handle missing provider authentication

## Key Files to Know

- **`VISION.md`**: Complete product vision and architecture
- **`VISION_IMPLEMENTATION.md`**: Implementation progress tracking (update when completing features!)
- **`VISION_ROTATE.md`**: Secret rotation vision including data-driven architecture (**update when adding rotation features**)
- **`VISION_ROTATE_IMPLEMENTATION.md`**: Rotation feature implementation progress (**update when completing rotation work**)
- **`internal/providers/registry.go`**: Provider registration system
- **`internal/resolve/resolver.go`**: Core resolution logic
- **`pkg/provider/provider.go`**: Provider interface definition
- **`internal/logging/logger.go`**: Security-aware logging system
- **`docs/PROVIDERS.md`**: Provider-specific documentation
- **`docs/research/`**: Research findings - **always document research using the template**
- **`docs/DSOPS_DATA_INTEGRATION.md`**: Architecture for integrating with dsops-data repository
- **`examples/`**: Working configuration examples for testing and documentation

## Research Documentation

When conducting research (market analysis, technical investigations, provider evaluations):

1. **Always use the research template**: `docs/templates/research-template.md`
2. **Name files with date prefix**: `YYYY-MM-DD-topic-name.md`
3. **Document sources**: Include all URLs, tools tested, and documentation reviewed
4. **Focus on implications**: How findings affect dsops design and implementation
5. **Update related docs**: If research impacts VISION.md or other docs, update them

This ensures knowledge is preserved and decisions are traceable.

## Rotation Feature Development

When working on secret rotation features:

1. **Update VISION_ROTATE.md**: Document new rotation capabilities, architectural changes, or feature additions
2. **Update VISION_ROTATE_IMPLEMENTATION.md**: Mark features as started/completed, update progress percentages
3. **Follow data-driven approach**: Use dsops-data for service definitions rather than hardcoding
4. **Test with dsops-data**: Use `--data-dir ./dsops-data` flag to test with community service definitions
5. **Document integration patterns**: Show how new features work with existing dsops-data schemas

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