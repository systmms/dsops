---
title: "Configuration"
description: "Understanding dsops.yaml configuration"
lead: "The dsops.yaml file is the heart of your secret management setup. Learn how to configure providers, environments, and variables."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 30
---

## Configuration File

dsops uses a YAML configuration file (default: `dsops.yaml`) to define:

- Secret providers (where secrets are stored)
- Environments (dev, staging, production, etc.)
- Variable mappings (which secrets to pull)

## Basic Structure

```yaml
version: 0

providers:
  bitwarden:
    type: bitwarden
  
  aws:
    type: aws.secretsmanager
    region: us-east-1

envs:
  development:
    DATABASE_URL:
      from: { provider: bitwarden, key: "dev-db-url" }
    
    API_KEY:
      from: { provider: aws, key: "/dev/api/key" }
```

## Providers Section

Define your secret storage providers:

```yaml
providers:
  # Password manager
  bitwarden:
    type: bitwarden
    timeout_ms: 5000  # Optional timeout
  
  # Cloud provider
  aws-prod:
    type: aws.secretsmanager
    region: us-east-1
    role_arn: "arn:aws:iam::123456789:role/dsops"  # Optional
  
  # Multiple instances of same type
  vault-dev:
    type: vault
    address: https://vault-dev.company.com
  
  vault-prod:
    type: vault
    address: https://vault-prod.company.com
```

## Environments Section

Define different environments with their variables:

```yaml
envs:
  # Development environment
  development:
    DATABASE_URL:
      from: { provider: bitwarden, key: "dev-db" }
    
    DEBUG:
      literal: "true"  # Literal values
  
  # Production with more security
  production:
    DATABASE_URL:
      from: { provider: aws-prod, key: "/prod/db/url" }
      optional: false  # Fail if not found
    
    API_KEY:
      from: { provider: vault-prod, key: "secret/api-key" }
      transform: base64_decode  # Apply transformation
```

## Variable Options

Each variable supports several options:

### Source Options

```yaml
envs:
  example:
    # From a provider
    SECRET_FROM_PROVIDER:
      from:
        provider: bitwarden
        key: "item-name"
        version: "2"  # Optional version
    
    # Literal value
    STATIC_VALUE:
      literal: "always-this-value"
    
    # With transformation
    DECODED_SECRET:
      from: { provider: aws, key: "base64-secret" }
      transform: base64_decode
```

### Control Options

```yaml
envs:
  example:
    # Optional variable (won't fail if missing)
    OPTIONAL_VAR:
      from: { provider: bitwarden, key: "may-not-exist" }
      optional: true
    
    # With metadata for tools
    DATABASE_URL:
      from: { provider: aws, key: "/prod/db" }
      metadata:
        rotate: "monthly"
        owner: "platform-team"
```

## Transforms

Apply transformations to secret values:

```yaml
transforms:
  # Chain multiple transforms
  api_config:
    - json_extract: ".credentials"
    - base64_decode

envs:
  production:
    API_TOKEN:
      from: { provider: vault, key: "api/config" }
      transform: api_config  # Use named transform
```

## Advanced Features

### Provider Inheritance

```yaml
providers:
  # Base configuration
  aws-base:
    type: aws.secretsmanager
    timeout_ms: 10000
  
  # Inherit and override
  aws-us-east:
    inherit: aws-base
    region: us-east-1
  
  aws-eu-west:
    inherit: aws-base
    region: eu-west-1
```

### Environment Inheritance

```yaml
envs:
  # Base environment
  _base:
    LOG_LEVEL:
      literal: "info"
    APP_NAME:
      literal: "myapp"
  
  # Inherit from base
  development:
    inherit: _base
    LOG_LEVEL:
      literal: "debug"  # Override
    DATABASE_URL:
      from: { provider: bitwarden, key: "dev-db" }
```

### Multiple Config Files

```bash
# Use different config file
dsops --config production.yaml exec -- ./app

# Or via environment
export DSOPS_CONFIG=production.yaml
dsops exec -- ./app
```

## Best Practices

1. **Use descriptive names** for providers and keys
2. **Separate environments** clearly
3. **Mark sensitive variables** as non-optional
4. **Document with metadata** for team clarity
5. **Version your config** in git (it contains no secrets!)

## Example: Multi-Cloud Setup

```yaml
version: 0

providers:
  # AWS for production secrets
  aws-prod:
    type: aws.secretsmanager
    region: us-east-1
    role_arn: "${AWS_ROLE_ARN}"  # From environment
  
  # GCP for analytics secrets
  gcp:
    type: gcp.secretmanager
    project: my-project-123
  
  # Bitwarden for development
  bitwarden:
    type: bitwarden

envs:
  production:
    # Mix providers as needed
    DATABASE_URL:
      from: { provider: aws-prod, key: "/prod/database/url" }
    
    ANALYTICS_KEY:
      from: { provider: gcp, key: "analytics-api-key" }
    
    FEATURE_FLAGS:
      from: { provider: aws-prod, key: "/prod/features" }
      transform: json_extract: ".flags"
```

## Next Steps

- [Explore available providers](/providers/)
- [Learn about secret rotation](/rotation/)
- [See CLI command reference](/reference/cli/)