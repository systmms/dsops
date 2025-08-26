# dsops — Developer Secret OPerationS

> A fast, cross‑platform CLI that pulls secrets from your vault(s) and renders `.env*` files or launches commands with ephemeral environment variables. Think **sops**-adjacent, but focused on developer workflows and dotenv outputs.

**Current Status**: v0.1 MVP (100% complete) | **Secret Rotation**: 91% complete | [View detailed status](STATUS.md)

---

## 0. Table of contents

- [dsops — Developer Secret OPerationS](#dsops--developer-secret-operations)
  - [0. Table of contents](#0-table-of-contents)
  - [1. Vision \& goals](#1-vision--goals)
  - [2. Non-goals](#2-non-goals)
  - [3. High-level UX](#3-high-level-ux)
  - [4. Configuration (`dsops.yaml`)](#4-configuration-dsopsyaml)
  - [5. Secret Stores \& Services](#5-secret-stores--services)
  - [6. Architecture](#6-architecture)
  - [7. Security model](#7-security-model)
    - [Guardrails \& leak response](#guardrails--leak-response)
  - [8. Secret Rotation Vision](#8-secret-rotation-vision)
  - [9. CLI spec (v0.x)](#9-cli-spec-v0x)
  - [10. File formats \& templating](#10-file-formats--templating)
  - [11. Examples](#11-examples)
    - [11.1 Minimal project](#111-minimal-project)
    - [11.2 Multi-cloud + transforms](#112-multi-cloud--transforms)
    - [11.3 Rendering a dotenv file](#113-rendering-a-dotenv-file)
    - [11.4 Arbitrary template](#114-arbitrary-template)
    - [11.5 Pre-commit hook \& CI guard](#115-pre-commit-hook--ci-guard)
    - [11.6 Leak report \& rotation](#116-leak-report--rotation)
  - [12. Testing strategy](#12-testing-strategy)
  - [13. Packaging \& releases](#13-packaging--releases)
  - [14. Implementation Status](#14-implementation-status)
  - [15. Roadmap](#15-roadmap)
  - [16. Contributing](#16-contributing)
  - [17. Governance \& license](#17-governance--license)
  - [Appendix A. RFC/ADR template](#appendix-a-rfcadr-template)
  - [Appendix B. Provider key notation](#appendix-b-provider-key-notation)

---

## 1. Vision & goals

**Problem:** Teams keep app secrets across multiple sources (1Password, Bitwarden, cloud secret stores). Developers need **repeatable** ways to hydrate local env vars for `docker compose`, Next.js, Python, Go, etc.—without leaving decrypted files laying around.

**Vision:** A single declarative config (`dsops.yaml`) that maps your app’s required variables to upstream secret providers, then either:

* **render** `.env*` files (optional) or
* **exec** your dev command with an **ephemeral** in‑memory environment.

**Primary goals**

* Smooth developer ergonomics; zero surprise defaults.
* First‑class support for macOS, Linux, Windows.
* Provider‑agnostic: fetch from password managers **and** cloud secret stores.
* Templated outputs: dotenv, JSON, YAML, or arbitrary files.
* Safe by default: redaction, minimal logging, short‑lived artifacts.

**Secondary goals**

* Pluggable provider interface (built‑ins + external shims).
* Validation of required vars; friendly diagnostics.
* Deterministic runs & lockfiles for stability.

## 2. Non-goals

* Replacing Git‑at‑rest encryption like **sops** for production IaC.
* Acting as a secret store itself. (dsops caches minimally; does not become a vault.)
* Full IAM orchestration; we rely on provider authentication.

## 3. High-level UX

```bash
# One-time project bootstrap
$ dsops init
> Created dsops.yaml with example providers and envs

# Dry-run: see which vars will resolve (no secret values shown)
$ dsops plan --env development

# Launch a command with ephemeral env (no files on disk)
$ dsops exec --env development -- docker compose up

# Render a dotenv file (explicit, opt-in)
$ dsops render --env production --out .env.production

# Get a single value for scripting
$ dsops get --key DATABASE_URL

# Doctor credentials and provider connectivity
$ dsops doctor
```

Principles:

* **Ephemeral first**, **files optional**.
* **Explicit is safer**: you must opt in to `--out` to write `.env*`.
* **No secrets in logs**; show origins & metadata only (source, version).

## 4. Configuration (`dsops.yaml`)

```yaml
version: 1

# Secret stores: where secrets are stored and retrieved
secretStores:
  bitwarden:
    type: bitwarden
    profile: default       # optional, if you use multiple bw profiles
  onepassword:
    type: onepassword
    account: myteam.1password.com
  aws_sm:
    type: aws.secretsmanager
    region: us-east-1
  aws_ssm:
    type: aws.ssm
    region: us-east-1
  gcp_sm:
    type: gcp.secretmanager
    project_id: my-gcp-proj
  azure_kv:
    type: azure.keyvault
    vault_name: kv-dev
  vault:
    type: hashicorp.vault
    address: http://127.0.0.1:8200

# Services: what uses secrets and supports rotation (data-driven via dsops-data)
services:
  postgres_dev:
    type: postgresql
    host: localhost
    port: 5432
    database: myapp_dev
    admin_store: store://aws_sm/postgres/admin
    
  stripe_dev:
    type: stripe
    environment: test

# Optional: local transforms helpers
transforms:
  # named transform chains applied to values; see examples below
  decode_b64_json: [base64_decode, json_extract:.data.password]

# Environment definitions with dual reference support.
# Variables can reference secret stores for retrieval and services for rotation.

envs:
  development:
    DATABASE_URL:
      from:
        store: store://onepassword/dev/app/db_url  # Where to retrieve
        service: svc://postgres_dev?kind=connection_string  # What uses it (optional)
    REDIS_PASSWORD:
      from:
        store: store://aws_sm/myapp/redis
      transform: json_extract:.password
    STRIPE_SECRET_KEY:
      from:
        store: store://aws_sm/stripe/dev/secret_key
        service: svc://stripe_dev?kind=secret_key  # Enables rotation
    FEATURE_FLAG:
      literal: "on"  # fallback literals supported

  production:
    DATABASE_URL:
      from:
        store: store://aws_sm/myapp/prod/db
        service: svc://postgres_prod?kind=connection_string
      transform: json_extract:.url
    JWT_PUBLIC_KEY:
      from:
        store: store://gcp_sm/jwt-pub?version=latest
      transform: multiline_to_single

# Optional: output templates, so you can render more than dotenv files
# using Go text/template + helper funcs.
templates:
  - name: docker-env
    format: dotenv              # dotenv|json|yaml|template
    env: development
    out: .env.docker
  - name: config.yml
    format: template
    env: development
    out: config.generated.yml
    template_path: templates/config.yml.tmpl
```

**Policies / Notifications / Incident** (optional additions)

```yaml
policies:
  gitignore:
    enforce: true
    auto_fix: true
    patterns:
      - ".env"
      - ".env.*"
      - ".envrc"
      - "**/.env"
      - "secrets.*"
  outputs:
    default_ttl: 10m
    forbid_commits: true

notifications:
  slack:
    webhook: env:SLACK_WEBHOOK_URL
    channel: "#secrets"
  webhooks:
    - url: https://security.example.com/hooks/dsops
      secret: env:DSOPS_WEBHOOK_SECRET

incident:
  default_visibility: private
  redact_values: true
  include_callsite: true
```

**Resolution rules**

1. `literal` wins if present.
2. Otherwise `from` must resolve via provider.
3. `transform` chain runs left→right.
4. Missing required variables cause non‑zero exit (unless `optional: true`).

**Lockfile (future):** `dsops.lock` to pin provider key versions (e.g., GCP version 7) for reproducibility.

## 5. Secret Stores & Services

dsops distinguishes between two types of integrations (see [TERMINOLOGY.md](docs/TERMINOLOGY.md) for detailed definitions):

### Secret Stores (Where secrets are stored)

**Password managers** (local dev friendly):

* **Bitwarden** (`bitwarden`): via `bw` CLI session or API key. Supports item fields, attachments.
* **1Password** (`onepassword`): via `op` CLI. Supports `op://vault/item/field` shorthand.
* **LastPass** (`lastpass`): via `lpass` CLI if available; or credential export (see security notes).
* **KeePassXC** (`keepassxc`) (optional): via DB + keychain / KDBX; read‑only.
* **pass** (`pass`): zx2c4 password-store (GPG + git); read‑only fetch.

**Cloud secret stores** ("major clouds"):

* **AWS Secrets Manager** (`aws.secretsmanager`)
* **AWS Systems Manager Parameter Store** (`aws.ssm`)
* **Google Cloud Secret Manager** (`gcp.secretmanager`)
* **Azure Key Vault** (`azure.keyvault`)
* **HashiCorp Vault** (`hashicorp.vault`) — widely used on‑prem/cloud.

### Services (What uses secrets and supports rotation)

**Database services**:
* **PostgreSQL** — User passwords, connection strings, SSL certificates
* **MySQL** — User accounts, replication credentials
* **MongoDB** — Database users, replica set keys

**API services**:
* **Stripe** — Secret keys, publishable keys, webhook secrets, Connect OAuth
* **GitHub** — Personal Access Tokens, Deploy Keys, App credentials
* **AWS IAM** — Access keys, service account credentials
* **Google Cloud IAM** — Service account keys, impersonation tokens
* **Azure AD** — Client secrets, certificates, federated identity

**Certificate authorities**:
* **Let's Encrypt/ACME** — TLS certificates via DNS/HTTP challenges
* **Venafi** — Enterprise certificate management
* **Self-signed** — Internal CA certificate generation

**Service definitions are data-driven** via the [dsops-data](https://github.com/systmms/dsops-data) community repository (84+ validated service definitions).

**Auth model**

* We **do not** manage logins. We detect & use ambient credentials (CLIs/SDKs):

  * AWS: env vars, shared config/credentials, SSO, EC2/ECS roles.
  * GCP: ADC (Application Default Credentials).
  * Azure: Az CLI login, Managed Identity (when applicable).
  * 1Password/Bitwarden/LastPass: their CLIs must be logged in; dsops can `dsops login <provider>` to assist.

## 6. Architecture

Core packages (Go):

* `cmd/dsops` — cobra/viper based CLI.
* `internal/config` — schema, parsing, validation with secret store vs service distinction.
* `internal/providers` — secret store providers (storage systems).
* `internal/services` — service integrations (rotation targets using dsops-data definitions).
* `internal/resolve` — dependency graph, transforms, redaction.
* `internal/template` — dotenv/JSON/YAML/`text/template` rendering.
* `internal/execenv` — process spawning with ephemeral env.
* `internal/rotation` — rotation engine with strategy support.
* `internal/dsopsdata` — dsops-data repository loader and validation.
* `internal/cache` — minimal in‑memory cache; optional encrypted on‑disk cache.
* `internal/logging` — structured logs with redaction.
* `internal/guard` — repo hygiene checks (gitignore, tracked files, history scan).
* `internal/git` — thin git helpers.
* `internal/incident` — incident records & audit log.
* `internal/notifier` — Slack / webhooks / GitHub Issues integrations.

**Secret Store Provider interface** (simplified):

```go
type Provider interface {
    Name() string
    Resolve(ctx context.Context, key Ref) (SecretValue, error)
    // Optional metadata APIs
    Describe(ctx context.Context, key Ref) (Meta, error)
    Capabilities() Caps
}
```

**Protocol Adapter interface for rotation**:

```go
type ProtocolAdapter interface {
    Name() string
    Protocol() string  // "sql", "http-api", "certificate", etc.
    Execute(ctx context.Context, service Service, operation Operation) (*Result, error)
}

type Service interface {
    Name() string
    Type() string  // Maps to dsops-data ServiceType  
    Protocol() string  // Which adapter to use
    Config() ServiceConfig
}

type ServiceRegistry interface {
    LoadFromDataDir(dataDir string) error
    GetServiceType(typeName string) (*ServiceType, error)
    CreateService(name string, config ServiceConfig) (Service, error)
    RouteToAdapter(service Service) (ProtocolAdapter, error)
}
```

**External providers (plugins)**

* Avoid Go `plugin` (portability issues). Support **exec‑plugin** protocol: execute a binary that speaks JSON over stdin/stdout:

  * Discovery: `$PATH/dsops-provider-*`
  * Handshake: version + capabilities
  * Methods: `resolve`, `describe`

**Transforms**

* Built‑ins: `trim`, `base64_encode`, `base64_decode`, `json_extract:.foo.bar`, `yaml_extract:.spec`, `multiline_to_single`, `join`, `replace:from:to`.
* Composable chains; fail fast on invalid transforms.

**Redaction**

* All logs contain **source metadata only**; values are redacted. `--debug` never prints raw secrets.

## 7. Security model

Threats & mitigations:

* **Disk residue**: Default to **no writes**; `render` requires `--out`. Outputs can be marked `--ttl` to auto‑delete after duration.
* **Process leaks**: `exec` sets env **in child only**; parent never sees values.
* **Crash logs**: panic handler scrubs memory dumps and disables crash uploads.
* **Clipboard**: `dsops get --copy` times out & clears after TTL.
* **Config secrets**: Support referencing secrets in `dsops.yaml` via providers; never store creds inline.
* **Local cache** (optional): OS keychain protected (macOS Keychain, Windows DPAPI, Secret Service on Linux). Disabled by default.

Operational guidance:

* Encourage `exec` for day‑to‑day. If writing files, add `.env*` to `.gitignore` and CI artifacts ignore.
* Provide `dsops shred` to securely delete known outputs.

### Guardrails & leak response

* **Repo hygiene**: `policies.gitignore` ensures `.env*` and generated files are ignored. `forbid_commits` causes guard to fail if generated outputs are staged/committed.
* **Guard commands**:

  * `dsops guard gitignore` checks `.gitignore` and `.git/info/exclude` for required patterns; optionally `--auto-fix`.
  * `dsops guard repo` scans tracked files and staging area for `.env*` or configured outputs; CI-friendly via `--ci`.
  * `dsops install-hook pre-commit` installs a minimal pre-commit hook to run `dsops guard repo`.
* **Leak incidents**:

  * `dsops leak report` records an incident under `.dsops/incidents/` (redacted), emits notifications (Slack/webhook/GitHub Issues if configured).
  * **Rotation**: `dsops rotate` attempts to create a new secret version via providers implementing the optional `Rotator` interface; otherwise instructs manual rotation.
* **Audit**: append-only local audit log `.dsops/audit.log` (redacted), with git ref, command, host, and timestamp.

## 8. Secret Rotation Vision

dsops extends beyond secret retrieval to automated secret rotation, addressing a critical gap in secret lifecycle management.

### Why Secret Rotation?

* **Compliance**: PCI-DSS, SOC2, HIPAA require regular rotation
* **Security**: Limit exposure window if secrets are compromised  
* **Automation**: Replace manual, error-prone rotation processes
* **Visibility**: Track rotation history and compliance status

### Data-Driven Architecture

dsops uses a community-maintained repository ([dsops-data](https://github.com/systmms/dsops-data)) containing service definitions for 100+ services:

```yaml
# dsops-data/providers/postgresql/service-type.yaml
name: postgresql
protocols: [sql]
capabilities:
  credential_kinds:
    - name: user_password
      rotation_capable: true
      verification_capable: true
```

### Rotation Strategies

1. **Immediate**: Replace secret instantly (brief downtime)
2. **Two-Key**: Maintain two valid secrets during transition
3. **Overlap**: Grace period where both old and new work
4. **Gradual**: Percentage-based rollout for large deployments

### Rotation Commands

```bash
# Rotate a database password
dsops secrets rotate --env prod --key DB_PASSWORD

# Check rotation status
dsops rotation status

# View rotation history
dsops rotation history --service postgres-prod --days 90

# Dry run to preview changes
dsops secrets rotate --env prod --dry-run
```

### Service Integration

Services are configured in the `services:` section and reference community definitions:

```yaml
services:
  postgres-prod:
    type: postgresql  # References dsops-data definition
    host: db.example.com
    port: 5432

envs:
  production:
    DB_PASSWORD:
      from: { store: aws-secrets, key: "/prod/db/password" }
      service: postgres-prod  # Links to rotation target
```

## 9. CLI spec (v0.x)

```
dsops init [--example <stack>]           # create dsops.yaml

dsops plan --env <name> [--json]         # show which keys will resolve

dsops render --env <name> --out <path>   # write dotenv (format auto by ext)
  [--format dotenv|json|yaml|template]
  [--template-path <file>] [--ttl 10m]

dsops exec --env <name> -- <cmd> [args...]  # run cmd with ephemeral env
  [--print]                 # print resolved vars (names only or masked)
  [--allow-override]        # allow existing env to override dsops values

dsops get --key <VAR> [--env <name>] [--copy] [--ttl 45s] [--raw]

dsops login <provider>                      # assist provider auth if possible

dsops doctor                                # check provider connectivity

dsops providers [--verbose]                 # list available providers

dsops shred [paths...]                      # secure-delete files

# NEW: guardrails & incident commands

dsops guard gitignore [--auto-fix] [--patterns <glob,...>]

dsops guard repo [--ci] [--fail-on-staged] [--include-history] [--since <ref>]

dsops install-hook pre-commit

dsops leak report --env <name> --keys KEY1,KEY2 [--note "..."] [--notify slack,github,webhook]

dsops rotate --env <name> --keys KEY1,KEY2 [--new-value file:-|literal:...|random:32]

Global flags: --config dsops.yaml --no-color --debug --non-interactive

Exit codes:
  0  success
 10  missing required variables
 11  provider auth failure
 12  key not found
 13  guardrail violation
  2  other errors
```

Exit codes:

* `0` success
* `10` missing required variables
* `11` provider auth failure
* `12` key not found
* `2` other errors

## 10. File formats & templating

**Dotenv** (default):

* Output respects `KEY=VALUE` with `\n` preserved using quoted values when needed.
* Escapes: auto where necessary.

**JSON/YAML**: values rendered as a flat object `{KEY: VALUE}`.

**Generic templates**:

* Go `text/template` with helper funcs: `env`, `has`, `json`, `b64enc`, `b64dec`, `indent`, `nindent`, `sha256`, etc.
* Example snippet:

  ```yaml
  database:
    url: {{ env "DATABASE_URL" | quote }}
  jwt:
    publicKey: |-
      {{ env "JWT_PUBLIC_KEY" | nindent 6 }}
  ```

## 11. Examples

### 11.1 Minimal project

```yaml
# dsops.yaml
version: 0
providers:
  onepassword: { type: onepassword }

envs:
  development:
    DATABASE_URL:
      from: { provider: onepassword, key: "op://Dev/App/DB_URL" }
    SECRET_KEY: { literal: "dev-only-123" }
```

```bash
# Run your app without writing files
$ dsops exec --env development -- go run ./cmd/api
```

### 11.2 Multi-cloud + transforms

```yaml
providers:
  aws_sm: { type: aws.secretsmanager, region: us-east-1 }
  gcp_sm: { type: gcp.secretmanager, project_id: myproj }

envs:
  production:
    REDIS_URL:
      from: { provider: aws_sm, key: "myapp/prod/redis" }
      transform: json_extract:.url
    JWT_PUBLIC_KEY:
      from: { provider: gcp_sm, key: "jwt-pub", version: latest }
      transform: multiline_to_single
```

### 11.3 Rendering a dotenv file

```bash
$ dsops render --env production --out .env.production
```

### 11.4 Arbitrary template

```yaml
templates:
  - name: k8s-secret
    format: template
    env: production
    out: k8s/secret.generated.yaml
    template_path: templates/secret.yaml.tmpl
```

### 11.5 Pre-commit hook & CI guard

```bash
# Install a pre-commit hook that runs repo guard
$ dsops install-hook pre-commit

# Example GitHub Actions step
- name: Check repo hygiene
  run: dsops guard repo --ci
```

### 11.6 Leak report & rotation

```bash
# Report a suspected leak for DATABASE_URL in production
$ dsops leak report --env production --keys DATABASE_URL --note "pasted in Slack"

# Attempt rotation (provider must implement Rotator)
$ dsops rotate --env production --keys DATABASE_URL --new-value random:64
```

## 12. Testing strategy

* **Unit tests**: pure logic (transforms, parsing, rendering) with table‑driven tests.
* **Provider fakes**: go interfaces + in‑memory fakes; contract tests shared across providers.
* **Integration tests**:

  * HashiCorp Vault: dev server in Docker during CI.
  * AWS: LocalStack for SSM/Secrets Manager.
  * GCP/Azure: mock SDKs (no official emulators for SM/KV); record/replay fixtures.
  * 1Password/Bitwarden/LastPass: simulate CLI JSON outputs.
* **Security checks**: ensure logs are redacted; forbid `fmt.Printf` of values.
* **Race detector**: `-race` on CI; coverage gate.
* **Guard tests**: temp git repos to verify `.gitignore` policy, staged file detection, and history scans.
* **Incident & notifier tests**: golden JSON for `.dsops/incidents/*`; fake Slack/webhook servers; table-driven rotation contract tests.

## 13. Packaging & releases

* Go 1.23+, static builds via `goreleaser` for darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64.
* Homebrew tap, Scoop manifest, apt/yum via `.deb/.rpm` (later), `asdf` plugin.
* Signed checksums (cosign) + SBOM (CycloneDX or SPDX).

## 14. Implementation Status

### Core Features (v0.1 MVP) - ✅ 100% Complete

| Component | Status | Notes |
|-----------|--------|---------|
| CLI Structure | ✅ Complete | Cobra-based with all 9 commands |
| Config Schema | ✅ Complete | Full v0/v1 parsing & validation |
| Provider Interface | ✅ Complete | 14+ providers implemented |
| Secret Resolution | ✅ Complete | Dependency graph & transforms |
| Security Features | ✅ Complete | Redaction, ephemeral exec, isolation |
| Output Formats | ✅ Complete | dotenv, JSON, YAML, Go templates |

### Provider Support - ✅ 100% Complete

**Password Managers**:
- ✅ Bitwarden - Full CLI integration
- ✅ 1Password - URI & dot notation support
- ✅ pass - Unix password store with GPG

**Cloud Providers**:
- ✅ AWS - Secrets Manager, SSM, STS, IAM Identity Center, Unified
- ✅ Google Cloud - Secret Manager, Unified Provider
- ✅ Azure - Key Vault, Managed Identity, Unified Provider
- ✅ HashiCorp Vault - Multiple auth methods
- ✅ Doppler - Developer secrets platform

### Secret Rotation (v0.3) - 🟡 91% Complete

| Feature | Status | Description |
|---------|--------|-------------|
| Rotation Engine | ✅ Complete | Full lifecycle management |
| Data-driven Architecture | ✅ Complete | dsops-data integration with 84+ service definitions |
| Protocol Adapters | ✅ Complete | SQL, NoSQL, HTTP API, Certificate |
| Rotation Strategies | ✅ Complete | Two-key, immediate, overlap |
| CLI Commands | ✅ Complete | rotate, status, history |
| Advanced Features | 🟡 38% | Notifications, rollback pending |

For detailed implementation tracking, see:
- [Full Implementation Status](VISION_IMPLEMENTATION.md) - Feature-by-feature progress
- [Rotation Implementation](VISION_ROTATE_IMPLEMENTATION.md) - Rotation-specific tracking

## 15. Roadmap

### ✅ Completed

**v0.1 (MVP)** - 100% Complete
* Core config parsing, dotenv render, `exec`
* All major providers: 1Password, Bitwarden, pass, AWS (5 variants), GCP, Azure (3 variants), Vault, Doppler
* 8 transforms: trim, json_extract, base64 encode/decode, multiline_to_single, replace, yaml_extract, join
* All core commands: init, plan, doctor, exec, render, get, login, providers, shred

**v0.2 Features** - 100% Complete
* Guardrails: `guard gitignore`/`repo`, `install-hook`
* Leak management: `leak report` with Slack/GitHub integration
* All AWS variants: SSM, STS, IAM Identity Center, Unified Provider
* Policy enforcement system

### 🚧 In Progress

**v0.3 (Secret Rotation)** - 91% Complete
* ✅ Rotation engine with lifecycle management
* ✅ Data-driven architecture via dsops-data
* ✅ Protocol adapters: SQL, NoSQL, HTTP API, Certificate
* ✅ Rotation strategies: two-key, immediate, overlap
* ✅ Commands: `secrets rotate`, `rotation status/history`
* 🔄 Advanced features: notifications, rollback (38%)

### 📅 Upcoming

**v0.4 (Q3 2025)**
* Complete rotation notifications (Slack, email, PagerDuty)
* Automatic rollback on verification failure
* Gradual rollout strategies
* Rotation policies and compliance reporting

**v0.5 (Q4 2025)**
* Plugin system for custom providers
* TUI for interactive secret management
* Watch mode for automatic re-hydration
* Enhanced Windows support

**v1.0 (Q1 2026)**
* Production stability milestone
* Comprehensive test coverage (>80%)
* Performance optimizations
* Enterprise features (SAML, audit export)

### 💡 Future Ideas
* Kubernetes operator for secret injection
* Terraform provider for infrastructure integration
* IDE plugins (VS Code, IntelliJ)
* Secret scanning in CI/CD pipelines

## 16. Contributing

**Local dev setup**

```bash
git clone https://github.com/<you>/dsops
cd dsops
make setup   # installs golangci-lint, pre-commit hooks
make test
make build
```

**Project conventions**

* `golangci-lint` clean; `go fmt` and `go vet`.
* Table‑driven tests; avoid package‑level state.
* Never print raw secrets; use `logging.Secret(value)` wrapper.

**Issue labels**: `provider:*`, `kind:feature`, `kind:bug`, `good-first-issue`, `security`, `help-wanted`.

**Code of Conduct**: Contributor Covenant.

## 17. Governance & license

* License: **Apache-2.0** (business‑friendly, permissive).
* Maintainers: TBD; require 2 reviews for provider changes.
* Security disclosures: security\@yourdomain (private email), 90‑day disclosure window.

---

## Appendix A. RFC/ADR template

```
# RFC-N: <Title>

- Status: Draft | Accepted | Rejected | Superseded by RFC-M
- Authors: <name>
- Date: <YYYY-MM-DD>

## Context
<What problem are we solving?>

## Proposal
<Design, APIs, UX, alternatives>

## Security / Privacy
<Threats, mitigations>

## Rollout
<MVP scope, migration, doc impacts>

## Open Questions
<Unknowns to validate>
```

---

## Appendix B. Provider key notation

* **Bitwarden**: `bw://<item-id>#<field>` or `bw://<collection>/<name>#<field>`
* **1Password**: `op://<vault>/<item>/<field>`
* **LastPass**: `lp://<group>/<name>#<field>` (subject to CLI availability)
* **AWS SM**: secret name (optionally JSON pointer), e.g. `aws-sm://myapp/redis#.password`
* **AWS SSM**: parameter path, e.g. `ssm:///myapp/dev/DB_URL`
* **GCP SM**: `gcp-sm://projects/<id>/secrets/<name>[:version]`
* **Azure KV**: `azure-kv://<vault-name>/<secret-name>[:version]`
* **Vault (KV v2)**: `vault://<mount>/<path>#<field>`

(Internally we normalize to `{provider, key, version?, path?, field?}`.)
