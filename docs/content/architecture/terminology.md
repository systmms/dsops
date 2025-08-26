---
title: "Terminology"
description: "Key terms and concepts in dsops"
lead: "This document defines key terms and concepts used throughout the dsops project to ensure consistent understanding and communication."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 10
---

## Core Concepts

### Secret Storage vs Service Providers

dsops distinguishes between two fundamental types of providers:

#### Secret Stores (Storage Systems)
Systems that **store and retrieve** secret values. These are the "where" of secret management.

**Examples:**
- AWS Secrets Manager
- Google Cloud Secret Manager
- Azure Key Vault
- HashiCorp Vault
- 1Password
- Bitwarden
- Pass (Unix password store)
- Literal values (for testing/development)

**Characteristics:**
- Primary purpose: Store and retrieve secret values
- Support versioning, metadata, access control
- Focus on secure storage, encryption at rest
- Used via `store://` URI format

#### Services (Rotation Targets)
Systems that **use secrets** and can have their credentials rotated. These are the "what" of secret management.

**Examples:**
- PostgreSQL database
- MySQL database
- Stripe API
- GitHub API
- AWS IAM
- Certificate authorities (Let's Encrypt, Venafi)
- OAuth providers

**Characteristics:**
- Primary purpose: Provide business functionality
- Support credential rotation, user management
- Focus on application integration
- Used via `svc://` URI format

### Key Terminology

#### Secret Value
The actual credential data (password, API key, certificate) stored and managed by dsops.

**Types:**
- **Password**: Authentication credential for databases, services
- **API Key**: Authentication token for API services
- **Certificate**: X.509 certificates for TLS/mTLS authentication
- **Token**: Temporary authentication credentials (OAuth, JWT)
- **Connection String**: Complete connection information including credentials

#### Secret Reference
A pointer to where a secret value is stored or how it should be rotated.

**Formats:**
- **Legacy format**: `provider: aws-prod, key: database/password`
- **Store URI format**: `store://aws-prod/database/password?version=latest`
- **Service URI format**: `svc://postgres/prod-db?kind=db_password`

#### Provider
A configured instance of a secret store or service integration.

**Configuration:**
```yaml
secretStores:
  aws-prod:
    type: aws.secretsmanager
    region: us-east-1
    
services:
  postgres-prod:
    type: postgresql
    host: db.example.com
    port: 5432
```

#### Environment
A named collection of variables that represent a deployment context (dev, staging, production).

#### Variable
A single environment variable definition that specifies how to obtain its value.

**Components:**
- **Name**: Environment variable name
- **Source**: Where to get the value (from secret store or literal)
- **Transform**: Optional data transformation pipeline
- **Metadata**: Additional context for rotation, permissions

## Rotation Terminology

### Rotation Strategy
The approach used to update a secret value without service disruption.

**Strategies:**
- **Immediate**: Replace secret immediately (high risk)
- **Two-key**: Create new secret, test, switch, remove old (safe)
- **Overlap**: Multiple secrets active simultaneously (zero downtime)
- **Webhook**: External service handles rotation
- **Script**: Custom script performs rotation

### Rotation Lifecycle
The complete process of updating a secret value.

**Phases:**
1. **Planning**: Determine what needs rotation
2. **Generation**: Create new secret value
3. **Verification**: Test new secret works
4. **Cutover**: Switch to new secret
5. **Cleanup**: Remove old secret
6. **Audit**: Log rotation completion

### Service Types vs Service Instances

#### Service Type
A generic definition of a service's capabilities, defined in dsops-data repository.

**Example**: `postgresql` service type defines:
- Credential kinds: `db_password`, `connection_string`, `ssl_certificate`
- Capabilities: `create`, `verify`, `rotate`, `revoke`
- Constraints: TTL, format requirements
- Default rotation strategy

#### Service Instance  
A specific deployment of a service type with concrete configuration.

**Example**: `prod-primary` PostgreSQL instance:
- Host: `prod-db.example.com`
- Port: `5432`
- Database: `app_production`
- Admin credentials: Reference to admin secret

## Data-Driven Architecture

### dsops-data Repository
Community-maintained repository containing service definitions, policies, and operational patterns.

**Structure:**
```
providers/
├── postgresql/
│   ├── service-type.yaml      # Generic PostgreSQL capabilities
│   ├── instances/             # Specific deployments
│   ├── policies/              # Rotation schedules and rules
│   └── principals/            # Identity and access definitions
```

### Schema Components

#### ServiceType
Defines what a service can do regarding credential management.

#### ServiceInstance
Specific deployment configuration for a service.

#### RotationPolicy
Rules for when and how rotation should occur.

#### Principal
Identity that can perform operations (users, services, CI/CD).

## Configuration Formats

### Legacy Format (dsops v0.1)
```yaml
providers:
  aws-prod:
    type: aws.secretsmanager
    
envs:
  production:
    DATABASE_URL:
      from:
        provider: aws-prod
        key: database/connection_string
```

### New URI Format (dsops v1.0+)
```yaml
secretStores:
  aws-prod:
    type: aws.secretsmanager
    
services:
  postgres-prod:
    type: postgresql
    
envs:
  production:
    DATABASE_URL:
      from:
        store: store://aws-prod/database/connection_string
        service: svc://postgres-prod?kind=connection_string
```

## Operation Types

### Retrieval Operations
Getting secret values for application use.

**Commands:**
- `dsops plan` - Show what values would be retrieved
- `dsops render` - Output secrets in various formats  
- `dsops exec` - Run command with secrets in environment

### Rotation Operations
Updating secret values in services and stores.

**Commands:**
- `dsops secrets rotate` - Rotate specific secrets
- `dsops secrets status` - Check rotation status
- `dsops secrets history` - View rotation history

### Management Operations
Configure and validate dsops setup.

**Commands:**
- `dsops init` - Initialize configuration
- `dsops doctor` - Validate configuration and connectivity
- `dsops providers` - List available providers

## Common Confusion Points

### "Provider" Ambiguity
The term "provider" historically meant both storage and services. Now we distinguish:
- **Secret Store Provider**: Where secrets are stored (AWS Secrets Manager)
- **Service Provider**: What uses secrets (PostgreSQL database)

### "Rotation" vs "Key Rotation"  
- **Secret Value Rotation**: Updating passwords, API keys (dsops focus)
- **Encryption Key Rotation**: Updating file encryption keys (SOPS focus)

### "Reference" vs "URI"
- **Reference**: Generic pointer to a secret
- **URI**: Specific format like `store://` or `svc://`

## Relationship to Other Tools

### dsops vs SOPS
- **SOPS**: Encrypts/decrypts files, rotates encryption keys
- **dsops**: Manages runtime secrets, rotates secret values
- **Complementary**: Use both in GitOps workflows

### dsops vs HashiCorp Vault
- **Vault**: Secret storage system with dynamic secrets
- **dsops**: Multi-provider secret management with rotation
- **Integration**: dsops can use Vault as a secret store

### dsops vs Cloud-Native Secret Management
- **Cloud tools**: Provider-specific (AWS Secrets Manager, GCP Secret Manager)
- **dsops**: Provider-agnostic with unified interface
- **Enhancement**: dsops adds rotation capabilities across providers

## Evolution of Terms

### Historical Changes
- `providers` → `secretStores` + `services` (v1.0)
- `provider: aws, key: secret` → `store://aws/secret` (URI format)
- Hardcoded strategies → Data-driven via dsops-data (v1.0+)

### Deprecated Terms
- **Secret Store** (legacy): Now "Provider" for storage systems
- **Transform Chain**: Now "Transform Pipeline" 
- **Rotation Target**: Now "Service" or "Service Instance"

---

## Quick Reference

| Term | Definition | Example |
|------|------------|---------|
| Secret Store | System that stores secrets | AWS Secrets Manager |
| Service | System that uses secrets | PostgreSQL database |
| Secret Value | The actual credential | `p@ssw0rd123` |
| Secret Reference | Pointer to a secret | `store://aws/db/pass` |
| Provider | Configured store/service | `aws-prod` configuration |
| Environment | Variable collection | `production` env |
| Variable | Single env var definition | `DATABASE_URL` |
| Rotation | Updating secret values | Password change process |
| Strategy | Rotation approach | `two-key`, `immediate` |

For implementation details, see:
- [VISION.md](../VISION.md) - Overall product vision
- [VISION_ROTATE.md](../VISION_ROTATE.md) - Rotation-specific vision  
- [ADR-001](adr/ADR-001-terminology-and-dsops-data.md) - Terminology decisions
- [PROVIDERS.md](PROVIDERS.md) - Provider-specific documentation