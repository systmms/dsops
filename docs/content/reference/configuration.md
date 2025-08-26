---
title: "Configuration Reference"
description: "Complete dsops.yaml configuration file reference"
lead: "Comprehensive reference for all dsops configuration options, formats, and best practices. Master the configuration file format for any secret management scenario."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 20
---

## Overview

The `dsops.yaml` configuration file defines secret stores, services, and environments for your project. It uses a declarative format that separates secret storage from secret usage, enabling flexible and secure secret management.

## Configuration Format

dsops supports version 1.0+ configuration format with clear separation of concerns:

```yaml
version: 1

# Where secrets are stored
secretStores:
  store-name:
    type: provider-type
    # Provider-specific configuration

# What uses secrets (for rotation)  
services:
  service-name:
    type: service-type
    # Service-specific configuration

# Named environments with variable definitions
envs:
  environment-name:
    VARIABLE_NAME:
      from:
        store: store-name
        # Store-specific reference format
      # Optional: Service linkage for rotation
      service: service-name
```

## Configuration Schema

### Root Level Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `version` | integer | Yes | Configuration format version (use `1`) |
| `secretStores` | object | Yes | Secret store provider configurations |
| `services` | object | No | Service definitions for rotation |
| `envs` | object | Yes | Environment variable definitions |

### Version

```yaml
version: 1
```

**Required**: Always use `version: 1` for the current format. This enables the modern `secretStores`/`services` separation and URI-based references.

## Secret Stores Configuration

The `secretStores` section defines where secrets are stored and how to access them.

### Basic Structure

```yaml
secretStores:
  store-name:
    type: provider-type
    # Provider-specific options
```

### Store Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `type` | string | Yes | Provider type identifier |
| `optional` | boolean | No | Allow store to be unavailable (default: false) |

### Provider-Specific Configuration

Each provider type has unique configuration options. See [Provider Documentation](/providers/) for details.

#### AWS Secrets Manager

```yaml
secretStores:
  aws:
    type: aws.secretsmanager
    region: us-east-1
    # Optional: Custom endpoint
    endpoint: https://secretsmanager.us-east-1.amazonaws.com
    # Optional: Role assumption
    role_arn: arn:aws:iam::123456789012:role/SecretsRole
```

#### 1Password

```yaml
secretStores:
  onepassword:
    type: onepassword
    # Optional: Account subdomain
    account: my-team.1password.com
    # Optional: Vault UUID or name
    vault: Private
```

#### HashiCorp Vault

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    # Authentication
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
    # Optional: Namespace (Enterprise)
    namespace: team-secrets
```

#### Azure Key Vault

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    # Optional: Authentication override
    auth:
      method: managed_identity
      client_id: ${AZURE_CLIENT_ID}
```

#### Google Secret Manager

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    # Optional: Custom credentials
    credentials:
      type: service_account
      path: /path/to/service-account-key.json
```

#### Bitwarden

```yaml
secretStores:
  bitwarden:
    type: bitwarden
    server: https://vault.bitwarden.com
    # Optional: Organization ID
    organization_id: ${BW_ORG_ID}
```

#### Literal Values

```yaml
secretStores:
  literal:
    type: literal
    # No additional configuration needed
```

### Store Configuration Examples

#### Multiple Stores

```yaml
secretStores:
  # Production secrets in AWS
  aws-prod:
    type: aws.secretsmanager
    region: us-east-1
    
  # Development secrets in 1Password
  onepassword-dev:
    type: onepassword
    account: dev-team.1password.com
    
  # Shared certificates in Vault
  vault-certs:
    type: hashicorp.vault
    url: https://vault.internal.com
    auth:
      method: kubernetes
      role: cert-reader
```

#### Environment-Specific Stores

```yaml
secretStores:
  # Development
  dev-secrets:
    type: literal  # Simple values for dev
    
  # Staging  
  staging-secrets:
    type: azure.keyvault
    vault_name: staging-keyvault
    
  # Production
  prod-secrets:
    type: aws.secretsmanager
    region: us-east-1
    role_arn: arn:aws:iam::123456789012:role/ProdSecretsRole
```

## Services Configuration

The `services` section defines systems that use secrets and support rotation. Services use community-maintained definitions from [dsops-data](https://github.com/systmms/dsops-data).

### Basic Structure

```yaml
services:
  service-name:
    type: service-type
    # Service-specific configuration
```

### Service Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `type` | string | Yes | Service type from dsops-data |
| `instance_ref` | string | No | Reference to dsops-data instance |
| `overrides` | object | No | Override instance defaults |

### Common Service Types

#### Database Services

```yaml
services:
  postgres-prod:
    type: postgresql
    host: db.production.example.com
    port: 5432
    database: app_production
    ssl_mode: require
    
    # Admin credentials for rotation
    admin_credentials:
      username: postgres
      password:
        from:
          store: aws-prod
          name: postgres-admin-password

  mysql-staging:
    type: mysql
    host: staging-db.example.com
    port: 3306
    database: app_staging
```

#### API Services

```yaml
services:
  stripe-prod:
    type: stripe
    environment: production
    # Rotation uses Stripe's API key management
    
  github-org:
    type: github
    organization: my-company
    # Uses GitHub's token rotation API
    
  datadog-monitoring:
    type: datadog
    site: datadoghq.com
    # API key rotation via Datadog API
```

#### Using dsops-data References

```yaml
services:
  postgres-rds:
    type: postgresql
    # Use pre-defined configuration
    instance_ref: dsops-data/providers/postgresql/instances/aws-rds.yaml
    
    # Override specific values
    overrides:
      host: my-specific-host.rds.amazonaws.com
      port: 5433
      
  kubernetes-cluster:
    type: kubernetes
    instance_ref: dsops-data/providers/kubernetes/instances/eks.yaml
    overrides:
      cluster_name: production-cluster
      region: us-west-2
```

## Environment Variables Configuration

The `envs` section defines named environments with their variable definitions.

### Basic Structure

```yaml
envs:
  environment-name:
    VARIABLE_NAME:
      from:
        store: store-name
        # Store-specific reference
      # Optional: Additional properties
```

### Variable Properties

| Property | Type | Required | Description |
|----------|------|----------|-------------|
| `from` | object | Yes | Secret reference definition |
| `service` | string | No | Service name for rotation |
| `optional` | boolean | No | Allow missing secret (default: false) |
| `default` | any | No | Default value if secret unavailable |
| `transform` | array | No | Value transformation pipeline |

### Variable Reference Formats

#### Basic Reference

```yaml
DATABASE_PASSWORD:
  from:
    store: aws-prod
    name: database-password
```

#### Store-Specific Formats

Different stores use different reference formats:

**AWS Secrets Manager:**
```yaml
API_KEY:
  from:
    store: aws
    name: api-keys
    key: stripe_key  # Extract specific key from JSON
```

**1Password:**
```yaml
DATABASE_URL:
  from:
    store: onepassword
    vault: Production
    item: Database Credentials
    field: connection_string
```

**HashiCorp Vault:**
```yaml
ENCRYPTION_KEY:
  from:
    store: vault
    path: secret/data/app
    key: encryption_key
    version: 2  # Specific version
```

**Azure Key Vault:**
```yaml
CERTIFICATE:
  from:
    store: azure
    name: app-certificate
    type: certificate
```

### Environment Examples

#### Multi-Environment Setup

```yaml
envs:
  development:
    DATABASE_URL:
      from:
        store: literal
        value: "postgres://dev:dev@localhost:5432/myapp_dev"
    
    API_KEY:
      from:
        store: onepassword-dev
        vault: Development
        item: API Keys
        field: stripe_test_key
    
    DEBUG_MODE:
      from:
        store: literal
        value: "true"

  staging:
    DATABASE_URL:
      from:
        store: azure-staging
        name: database-connection-string
    
    API_KEY:
      from:
        store: azure-staging
        name: stripe-api-key
    
    DEBUG_MODE:
      from:
        store: literal
        value: "false"

  production:
    DATABASE_URL:
      from:
        store: aws-prod
        name: database/connection_string
      service: postgres-prod  # Enable rotation
    
    API_KEY:
      from:
        store: aws-prod
        name: stripe/api_key
      service: stripe-prod
    
    DEBUG_MODE:
      from:
        store: literal
        value: "false"
```

#### Complex Variable Definitions

```yaml
envs:
  production:
    # JSON extraction
    DATABASE_HOST:
      from:
        store: aws-prod
        name: database-config
      transform:
        - type: json_extract
          path: .host
    
    DATABASE_PORT:
      from:
        store: aws-prod
        name: database-config
      transform:
        - type: json_extract
          path: .port
    
    # Base64 decoding
    PRIVATE_KEY:
      from:
        store: vault
        path: pki/cert/app-key
      transform:
        - type: base64_decode
    
    # Template rendering
    FULL_DATABASE_URL:
      from:
        store: aws-prod
        name: database-template
      transform:
        - type: template
          template: "postgres://{{.username}}:{{.password}}@{{.host}}:{{.port}}/{{.database}}"
          values:
            username:
              from:
                store: aws-prod
                name: database-user
            password:
              from:
                store: aws-prod
                name: database-password
            host:
              from:
                store: aws-prod
                name: database-host
            port:
              from:
                store: literal
                value: "5432"
            database:
              from:
                store: literal
                value: "production"
    
    # Optional with default
    OPTIONAL_FEATURE:
      from:
        store: aws-prod
        name: feature-flag
      optional: true
      default: "disabled"
    
    # Service linkage for rotation
    STRIPE_API_KEY:
      from:
        store: aws-prod
        name: stripe/secret_key
      service: stripe-prod
      rotation:
        strategy: two-key
        ttl: 90d
```

## Advanced Configuration

### Transforms

Transform secret values during resolution:

```yaml
API_CONFIG:
  from:
    store: vault
    path: secret/api-config
  transform:
    # Extract JSON field
    - type: json_extract
      path: .api_key
    
    # Decode base64
    - type: base64_decode
    
    # Apply template
    - type: template
      template: "Bearer {{.}}"
```

**Available Transforms:**
- `json_extract` - Extract field from JSON
- `base64_decode` - Decode base64 content  
- `base64_encode` - Encode to base64
- `template` - Apply Go template
- `regex_replace` - Regex find/replace
- `trim` - Trim whitespace
- `upper` - Convert to uppercase
- `lower` - Convert to lowercase

### Rotation Configuration

Configure secret rotation for services:

```yaml
envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: aws-prod
        name: postgres/password
      service: postgres-prod
      rotation:
        strategy: two-key  # or immediate, overlap, gradual
        ttl: 30d          # Rotate every 30 days
        schedule: "0 2 * * SUN"  # Weekly at 2 AM
        
        # Notifications
        notifications:
          success:
            - type: slack
              webhook: ${SLACK_WEBHOOK}
          failure:
            - type: email
              recipients: [ops@example.com]
        
        # Verification
        verification:
          enabled: true
          timeout: 30s
          query: "SELECT 1"
```

### Conditional Configuration

Use environment variables or conditionals:

```yaml
secretStores:
  dynamic-store:
    type: ${SECRET_STORE_TYPE:-aws.secretsmanager}
    region: ${AWS_REGION:-us-east-1}

envs:
  production:
    DATABASE_URL:
      from:
        store: dynamic-store
        name: ${DATABASE_SECRET_NAME:-database/connection}
```

### Multi-Store Fallbacks

Define fallback chains:

```yaml
envs:
  production:
    API_KEY:
      from:
        store: primary-vault
        name: api-key
      fallback:
        - from:
            store: backup-vault
            name: api-key
        - from:
            store: literal
            value: "fallback-key"
```

### YAML Anchors and References

Use YAML features for reusability:

```yaml
# Define common configurations
common_aws: &aws_config
  type: aws.secretsmanager
  region: us-east-1

database_ref: &db_ref
  store: aws-prod
  name: database/connection_string

secretStores:
  aws-prod:
    <<: *aws_config
    role_arn: arn:aws:iam::123456789012:role/ProdRole
    
  aws-staging:
    <<: *aws_config
    role_arn: arn:aws:iam::123456789012:role/StagingRole

envs:
  production:
    DATABASE_URL:
      from: *db_ref
      
  staging:
    DATABASE_URL:
      from:
        <<: *db_ref
        name: staging/database/connection_string
```

## Legacy Format Support

dsops maintains backward compatibility with the legacy `providers:` format:

### Legacy Format (v0)

```yaml
# Legacy format - still supported
providers:
  aws:
    type: aws.secretsmanager
    region: us-east-1

envs:
  production:
    DATABASE_PASSWORD:
      from:
        provider: aws
        name: database-password
```

### Migration to v1

```yaml
# Modern format (recommended)
version: 1

secretStores:  # Was 'providers'
  aws:
    type: aws.secretsmanager
    region: us-east-1

envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: aws  # Was 'provider'
        name: database-password
```

**Migration Benefits:**
- Clear separation of secret stores vs services
- Support for rotation configuration
- URI-based references (`store://`, `svc://`)
- Better integration with dsops-data

## Validation

### Configuration Validation

dsops validates configuration on load:

```bash
# Check configuration syntax
dsops doctor

# Validate specific environment
dsops doctor --env production

# Show resolution plan
dsops plan --env production
```

### Common Validation Errors

**Invalid Version:**
```yaml
# Error: Missing or invalid version
version: 0  # Should be 1
```

**Missing Required Fields:**
```yaml
secretStores:
  aws:
    # Error: Missing required 'type' field
    region: us-east-1
```

**Invalid Store Reference:**
```yaml
envs:
  production:
    API_KEY:
      from:
        store: nonexistent-store  # Error: Store not defined
        name: api-key
```

**Type Mismatch:**
```yaml
envs:
  production:
    PORT:
      from:
        store: literal
        value: 8080  # Should be string: "8080"
```

## Environment Variables

Override configuration with environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `DSOPS_CONFIG` | Configuration file path | `export DSOPS_CONFIG=prod.yaml` |
| `DSOPS_ENV` | Default environment | `export DSOPS_ENV=production` |
| `${VAR}` | Variable substitution | `region: ${AWS_REGION}` |

### Variable Substitution

```yaml
secretStores:
  aws:
    type: aws.secretsmanager
    region: ${AWS_REGION:-us-east-1}
    role_arn: ${AWS_ROLE_ARN}

envs:
  production:
    DATABASE_NAME:
      from:
        store: aws
        name: ${DB_SECRET_NAME:-database/name}
```

## Best Practices

### Organization

```yaml
# Group by environment and purpose
secretStores:
  # Production stores
  aws-prod:
    type: aws.secretsmanager
    region: us-east-1
  
  vault-prod:
    type: hashicorp.vault
    url: https://vault.prod.example.com
  
  # Development stores
  literal-dev:
    type: literal
  
  onepassword-dev:
    type: onepassword
    account: dev-team.1password.com

# Clear service definitions
services:
  # Databases
  postgres-prod:
    type: postgresql
    host: prod-db.example.com
  
  # APIs
  stripe-prod:
    type: stripe
    environment: production

# Environment separation
envs:
  development:
    # Use literal/local for development
  
  staging:
    # Mix of cloud and literal
    
  production:
    # All from secure cloud stores
```

### Security

```yaml
# Use appropriate stores for sensitivity
envs:
  production:
    # High-security: Use vault/cloud stores
    DATABASE_PASSWORD:
      from:
        store: vault-prod
        path: database/prod/password
    
    # Medium-security: Encrypted cloud storage
    API_KEYS:
      from:
        store: aws-prod
        name: api-keys
    
    # Low-security: Can use literals
    LOG_LEVEL:
      from:
        store: literal
        value: "info"

# Enable rotation for critical secrets
    MASTER_KEY:
      from:
        store: vault-prod
        path: master-key
      service: encryption-service
      rotation:
        strategy: immediate
        ttl: 7d  # Weekly rotation
```

### Documentation

```yaml
# Comment configuration sections
secretStores:
  # Primary production secret store
  aws-prod:
    type: aws.secretsmanager
    region: us-east-1
    # Uses IAM role for authentication

services:
  # Main application database
  postgres-prod:
    type: postgresql
    host: prod-db.internal.example.com
    # Rotation handled by RDS

envs:
  production:
    # Database connection (rotated weekly)
    DATABASE_URL:
      from:
        store: aws-prod
        name: rds/production/connection
      service: postgres-prod
      
    # Stripe API key (rotated monthly)  
    STRIPE_KEY:
      from:
        store: aws-prod
        name: stripe/secret_key
      service: stripe-prod
```

## Troubleshooting

### Common Issues

**Configuration Not Found:**
```bash
# Specify config file location
dsops --config /path/to/dsops.yaml doctor
```

**Provider Authentication Failed:**
```bash
# Check provider setup
dsops doctor --verbose

# Test specific environment
dsops plan --env production
```

**Variable Resolution Failed:**
```bash
# Debug specific variable
dsops get --env prod DATABASE_URL

# Check transforms
dsops plan --env prod --format json | jq '.variables.DATABASE_URL'
```

**Rotation Configuration Invalid:**
```bash
# Validate rotation setup
dsops rotation status

# Test rotation dry-run
dsops secrets rotate --env prod --dry-run
```

## Related Documentation

- [CLI Reference](/reference/cli/) - Command-line interface
- [Provider Documentation](/providers/) - Provider-specific configuration
- [Rotation Guide](/rotation/) - Secret rotation configuration
- [Security Best Practices](/security/) - Security guidelines
- [Getting Started](/getting-started/) - Initial setup guide