# ADR-001R: Terminology & Externalizing Services to `dsops-data`

**Status**: Proposed
**Date**: 2025-08-19
**Supersedes**: ADR-001 (Terminology for Secret Storage vs Rotation Targets)

## Decision

We **keep the terminology** from ADR‑001 — **Secret Stores** (storage), **Services** (rotation targets), **Provider** (adapter/plugin) — **and** we split the growing catalog of Services (especially Service *instances*) into a dedicated, versioned **data repository** named **`dsops-data`**. Core code lives in `dsops` (the engine and adapters), while large, organization-specific *data* (service instances, rotation policies, principals, references) lives in `dsops-data`.

### One‑line summary

> **Code in `dsops`, data in `dsops-data`, contracts between them are explicit and versioned.**

---

## Context

The project will manage **thousands of Service instances** (e.g., GitHub orgs, Postgres databases, Stripe accounts). Keeping these under the `dsops` code repo would bloat history, slow CI, and tangle concerns. We need:

* Clear **source of truth** for Services and rotation metadata
* Fast **lookups**/search across thousands of entries
* **Version pinning** and reproducibility for runs
* **Change control** (review/approvals) independent from code changes

---

## Scope

This ADR clarifies **where data lives**, **how the engine discovers it**, **how it’s versioned**, and **how UX/CLI change**. It does **not** change the core terminology or rotation lifecycle.

---

## Repository Split

### `dsops` (code repo)

* Interfaces and core: `pkg/secretstore`, `pkg/service`, `internal/rotation`, capabilities registry
* Providers (adapters): `internal/secretstores/*`, `internal/services/*`
* CLI and API server (if present)
* Conformance test suite & validators

### `dsops-data` (data repo)

A Git repo (or package) containing **declarative descriptors**:

* **`service-types/`**: small set of type definitions (e.g., `github`, `postgres`, `stripe`)
* **`service-instances/`**: large, growing set of concrete instances (thousands)
* **`rotation-policies/`**: named policies reusable across instances
* **`principals/`**: identities the credentials belong to
* **`catalog/`**: generated indices for fast search (do not hand‑edit)

#### Proposed directory layout

```
service-types/
  github/
    kind.yaml              # defines CredentialKind, capability matrix, defaults
  postgres/
    kind.yaml
  stripe/
    kind.yaml

service-instances/
  github/
    acme-org.yaml
    beta-org.yaml
  postgres/
    prod/app-db.yaml
    staging/app-db.yaml
  stripe/
    main-account.yaml

rotation-policies/
  gh-ci-pats.yaml
  pg-two-key.yaml

principals/
  ci-bot.yaml
  app-role.yaml

catalog/
  index.yaml               # machine-generated: keys → file pointers, tags, caps
  inverted.idx             # optional search index
```

> **Note**: Secrets never live in `dsops-data`; only **references** (e.g., `store://…`).

---

## Data Model (normative)

### Common terms (unchanged)

* **SecretStore** — where secrets live (Bitwarden, 1Password, AWS SM, GCP SM, Azure KV, Vault)
* **Service** — external system where credentials are issued/rotated (GitHub, Postgres, Stripe)
* **ServiceInstance** — an addressable instance of a Service type
* **CredentialKind** — PAT, API key, db password, cert, etc.
* **Principal** — the identity the credential is for
* **RotationPolicy** — cadence + strategy
* **SecretRef / ServiceRef** — portable references

### Minimal schemas

#### `service-types/<name>/kind.yaml`

```yaml
apiVersion: dsops.io/v1
kind: ServiceType
metadata:
  name: github
spec:
  credentialKinds:
    - name: pat
      capabilities: [create, verify, revoke, rotate]
    - name: app
      capabilities: [create, verify, revoke]
  defaults:
    rateLimit: medium
```

#### `service-instances/<type>/<id>.yaml`

```yaml
apiVersion: dsops.io/v1
kind: ServiceInstance
metadata:
  type: github
  id: acme-org
  tags: [team:platform, env:prod]
spec:
  endpoint: github.com
  auth: ref:store://bitwarden/Platform/gh#fine_grained_pat
  credentialKinds:
    - name: pat
      policy: gh-ci-pats
      principals: [ci-bot]
```

#### `rotation-policies/<name>.yaml`

```yaml
apiVersion: dsops.io/v1
kind: RotationPolicy
metadata:
  name: gh-ci-pats
spec:
  strategy: two-key
  schedule: cron("0 3 * * *")
  verification: api-scope-check
  cutover:
    requireCheck: true
```

#### `principals/<name>.yaml`

```yaml
apiVersion: dsops.io/v1
kind: Principal
metadata:
  name: ci-bot
spec:
  type: service-account
  description: CI automation bot
```

---

## Addressing & References

* **ServiceRef**: `svc://<type>/<id>?kind=<credentialKind>`
  e.g., `svc://github/acme-org?kind=pat`
* **SecretRef**: `store://<store>/<path>#<field>`
* **DataRef** (new): pin to a specific data snapshot

  * Git URL + ref: `git+ssh://git@github.com/acme/dsops-data@refs/tags/v2025.08.19`
  * Or local path: `/opt/dsops-data`

The engine attaches `dataRef` to every RotationRun for provenance and reproducibility.

---

## Engine → Data integration

### Discovery order

1. `--data-ref` flag
2. `$DSOPS_DATA_REF` env
3. Default: local path `./dsops-data` or `~/.config/dsops/dsops-data`

### Caching & performance

* Shallow clone at first use; **pin** to the requested ref
* Build `catalog/index.yaml` on CI in `dsops-data` and ship it
* On first read, the CLI loads the index into an **embedded SQLite** cache per ref
* Queries (by `type`, `id`, `tags`, `capabilities`) hit the cache, not the filesystem tree

### Offline

* If `--data-ref` points to a previously cached ref, runs are fully offline

---

## CLI/UX changes

```bash
# Select a data snapshot explicitly
export DSOPS_DATA_REF="git+ssh://git@github.com/acme/dsops-data@prod"

# Discover and describe a service instance
dsops describe service svc://github/acme-org?kind=pat

# List all instances matching tags (uses catalog index)
dsops list services --type github --tag env=prod --capability rotate

# Rotate using pinned data ref
dsops rotate \
  --service svc://github/acme-org?kind=pat \
  --principal ci-bot \
  --policy gh-ci-pats \
  --data-ref git+ssh://git@github.com/acme/dsops-data@refs/tags/2025.08.19
```

Flags added/recognized: `--data-ref`, `--data-path`.

---

## Governance, Security, and Provenance

* **No secrets** in `dsops-data`; only references
* Require **code owners** and **review** on `service-instances/` and `rotation-policies/`
* **Signing**: sign tags/releases of `dsops-data` (e.g., GPG or Sigstore)
* **Validation**: CI in `dsops-data` runs schema + semantic validators from `dsops` (e.g., `dsops validate --strict`)
* **RBAC**: optional folder ownership (teams own their subtrees)
* **Audit**: RotationRuns record `dataRef`, `ServiceRef`, `SecretRef`, fingerprints, and outcomes

---

## Migration Plan

1. **Introduce data ref** support in `dsops` (non‑breaking)
2. **Create `dsops-data`** with the structure above; add CI that builds `catalog/`
3. **Codemod**: move existing service instance YAMLs from `dsops` into `dsops-data`
4. **Dual-read**: engine looks in both locations; prefer `--data-ref` if set
5. **Docs & examples**: update to use `dsops-data`; keep legacy examples with warnings
6. **Finalize**: after one minor release, stop reading service instances from `dsops`

---

## Consequences

**Pros**

* Scales to thousands of Service instances without bloating the code repo
* Clear change control and ownership boundaries
* Reproducible runs via `dataRef` pinning
* Faster search via prebuilt `catalog`

**Cons**

* Additional repo to manage and secure
* Requires initial migration and contributor education

---

## Open Questions (tracked)

* Do we publish `dsops-data` snapshots as OCI artifacts or tarballs for air‑gapped use? (Optional future)
* How big can the `catalog/` get before we move to a purpose‑built indexer? (Monitor)
* Do we need per‑env overlays (`overlays/prod`, `overlays/staging`) or keep env as tags? (Start with tags)

---

## Appendix A — Validators (outline)

* **Schema**: JSON Schema for all four kinds; enforced in `dsops-data` CI
* **Semantic**: cross‑file checks (policy existence, principal existence, provider capabilities)
* **Capability**: ensure `(ServiceType, CredentialKind)` supports requested operations

---

## Appendix B — Minimal Engine Interfaces (unchanged)

```go
// pkg/secretstore
type SecretStore interface {
  Resolve(ctx context.Context, ref Reference) (SecretValue, error)
}

// pkg/service
type Service interface {
  Plan(ctx context.Context, req RotationRequest) (RotationPlan, error)
  Execute(ctx context.Context, plan RotationPlan) (RotationResult, error) // idempotent by fingerprint
  Verify(ctx context.Context, res RotationResult) error
  Rollback(ctx context.Context, res RotationResult) error
}
```

`fingerprint := hash(serviceInstance, credentialKind, principal, policyID)`

---

## Appendix C — Example `catalog/index.yaml`

```yaml
apiVersion: dsops.io/v1
kind: CatalogIndex
services:
  - ref: svc://github/acme-org
    kinds: [pat, app]
    tags: [team:platform, env:prod]
    path: service-instances/github/acme-org.yaml
  - ref: svc://postgres/prod/app-db
    kinds: [db_password]
    tags: [team:app, env:prod]
    path: service-instances/postgres/prod/app-db.yaml
```
