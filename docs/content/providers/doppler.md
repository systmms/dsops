---
title: "Doppler"
description: "Modern secret management for developers and DevOps teams"
lead: "Doppler is a developer-first secret management platform that syncs secrets across teams, environments, and applications. It provides a simple interface with powerful automation and security features."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 520
---

## Overview

Doppler provides modern secret management with:

- **Environment-First Design**: Organize secrets by project and environment
- **Real-Time Sync**: Instant propagation across all connected services
- **Rich Integrations**: Native support for popular deployment platforms
- **Developer Experience**: Intuitive UI with powerful CLI tools
- **Audit & Compliance**: Complete activity logs and access controls
- **GitOps Friendly**: Version control integration with branch-based workflows

## Prerequisites

1. **Doppler Account**: Sign up at [doppler.com](https://doppler.com)
2. **Service Token**: Create a service token for API access
3. **Network Access**: Connectivity to Doppler's API endpoints

### Doppler CLI (Optional)

```bash
# macOS
brew install dopplerhq/cli/doppler

# Linux
curl -Ls https://cli.doppler.com/install.sh | sh

# Windows
winget install Doppler.DopplerCLI

# Verify installation
doppler --version
```

## Configuration

### Basic Configuration

```yaml
version: 1

secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    project: my-project
    config: prd  # Environment/config name

envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: doppler
        name: DATABASE_PASSWORD

    API_KEY:
      from:
        store: doppler
        name: API_KEY
```

### Advanced Configuration

```yaml
secretStores:
  doppler:
    type: doppler
    
    # Authentication
    token: ${DOPPLER_TOKEN}
    
    # Project configuration
    project: my-application
    config: production
    
    # Optional: Custom API endpoint
    api_url: https://api.doppler.com
    
    # Optional: Request timeout
    timeout: 30s
    
    # Optional: Retry configuration
    retry:
      max_attempts: 3
      backoff: exponential
      max_backoff: 30s
    
    # Optional: Caching
    cache:
      enabled: true
      ttl: 300  # 5 minutes
```

### Multi-Environment Setup

```yaml
secretStores:
  doppler-dev:
    type: doppler
    token: ${DOPPLER_DEV_TOKEN}
    project: my-app
    config: dev
  
  doppler-staging:
    type: doppler
    token: ${DOPPLER_STAGING_TOKEN}
    project: my-app
    config: stg
  
  doppler-prod:
    type: doppler
    token: ${DOPPLER_PROD_TOKEN}
    project: my-app
    config: prd

envs:
  development:
    DATABASE_URL:
      from:
        store: doppler-dev
        name: DATABASE_URL
  
  staging:
    DATABASE_URL:
      from:
        store: doppler-staging
        name: DATABASE_URL
  
  production:
    DATABASE_URL:
      from:
        store: doppler-prod
        name: DATABASE_URL
```

## Authentication Methods

### 1. Service Token (Recommended)

```yaml
# Service token provides read-only access to specific config
secretStores:
  doppler:
    type: doppler
    token: dp.st.production.1234567890abcdef  # Service token
    # Project and config are embedded in token
```

### 2. CLI Token

```yaml
# CLI token provides broader access (use with caution)
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_CLI_TOKEN}
    project: my-project
    config: production
```

### 3. Personal Token

```yaml
# Personal token for development/testing
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_PERSONAL_TOKEN}
    project: my-project
    config: dev
```

## Secret Management

### Basic Secret Access

```yaml
envs:
  production:
    # Simple secret
    DATABASE_PASSWORD:
      from:
        store: doppler
        name: DATABASE_PASSWORD
    
    # Complex secret
    REDIS_URL:
      from:
        store: doppler
        name: REDIS_URL
    
    # JSON secret
    AWS_CONFIG:
      from:
        store: doppler
        name: AWS_CONFIG
```

### Secret Transformation

```yaml
# Extract from JSON secret
DATABASE_HOST:
  from:
    store: doppler
    name: DATABASE_CONFIG
  transform:
    - type: json_extract
      path: .host

DATABASE_PORT:
  from:
    store: doppler
    name: DATABASE_CONFIG
  transform:
    - type: json_extract
      path: .port

# Base64 decode
PRIVATE_KEY:
  from:
    store: doppler
    name: PRIVATE_KEY_B64
  transform:
    - type: base64_decode

# Template replacement
CONNECTION_STRING:
  from:
    store: doppler
    name: DB_CONNECTION_TEMPLATE
  transform:
    - type: template
      template: "postgres://{{.username}}:{{.password}}@{{.host}}:{{.port}}/{{.database}}"
      values:
        username:
          from:
            store: doppler
            name: DB_USERNAME
        password:
          from:
            store: doppler
            name: DB_PASSWORD
        host:
          from:
            store: doppler
            name: DB_HOST
        port:
          from:
            store: doppler
            name: DB_PORT
        database:
          from:
            store: doppler
            name: DB_NAME
```

### Dynamic References

```yaml
# Reference secrets from different configs
envs:
  production:
    # Production database
    DATABASE_URL:
      from:
        store: doppler
        name: DATABASE_URL
        config: prd
    
    # Shared API key from common config
    MONITORING_API_KEY:
      from:
        store: doppler
        name: MONITORING_API_KEY
        config: shared
        project: infrastructure
```

## Advanced Features

### Branch-Based Workflows

```yaml
# Use Doppler branches for feature development
secretStores:
  doppler-feature:
    type: doppler
    token: ${DOPPLER_TOKEN}
    project: my-app
    config: dev
    branch: feature/new-api  # Feature branch

envs:
  feature:
    API_ENDPOINT:
      from:
        store: doppler-feature
        name: API_ENDPOINT
        # Gets value from feature branch first, falls back to main
```

### Config Cloning

```yaml
# Clone configuration for testing
clone_config:
  from_project: production-app
  from_config: prd
  to_project: staging-app
  to_config: test
  
  # Override specific secrets
  overrides:
    DATABASE_URL: "postgres://test:test@localhost/testdb"
    DEBUG_MODE: "true"
```

### Batch Secret Operations

```yaml
# Efficiently fetch all secrets from a config
envs:
  production:
    _batch:
      app_secrets:
        store: doppler
        # Fetches all secrets at once
    
    # Use secrets from batch
    DATABASE_URL:
      from:
        batch: app_secrets
        name: DATABASE_URL
    
    API_KEY:
      from:
        batch: app_secrets
        name: API_KEY
    
    REDIS_URL:
      from:
        batch: app_secrets
        name: REDIS_URL
```

### Secret References

```yaml
# Use Doppler's secret referencing within the platform
API_BASE_URL:
  from:
    store: doppler
    name: API_BASE_URL
    # In Doppler: API_BASE_URL = "${API_PROTOCOL}://${API_HOST}:${API_PORT}"
    # Doppler resolves references automatically
```

## Integration Patterns

### Docker Integration

```yaml
# Use with Docker containers
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}

# docker-compose.yml equivalent
services:
  app:
    environment:
      # dsops will inject all environment variables
      DATABASE_URL:
        from:
          store: doppler
          name: DATABASE_URL
      API_KEY:
        from:
          store: doppler
          name: API_KEY
```

### Kubernetes Integration

```yaml
# Kubernetes deployment pattern
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  template:
    spec:
      initContainers:
      - name: doppler-secrets
        image: dsops:latest
        command: ['dsops', 'render', '--env', 'production', '--out', '/tmp/secrets/.env']
        volumeMounts:
        - name: secrets
          mountPath: /tmp/secrets
        env:
        - name: DOPPLER_TOKEN
          valueFrom:
            secretKeyRef:
              name: doppler-token
              key: token
      
      containers:
      - name: app
        image: my-app:latest
        envFrom:
        - configMapRef:
            name: app-secrets
        volumeMounts:
        - name: secrets
          mountPath: /app/secrets
```

### CI/CD Integration

```yaml
# GitHub Actions example
name: Deploy
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup dsops
        run: |
          curl -L https://github.com/systmms/dsops/releases/latest/download/dsops-linux-amd64 -o dsops
          chmod +x dsops
      
      - name: Deploy with secrets
        env:
          DOPPLER_TOKEN: ${{ secrets.DOPPLER_TOKEN }}
        run: |
          ./dsops exec --env production -- ./deploy.sh
```

## Security Best Practices

### 1. Use Service Tokens

```yaml
# Preferred: Service tokens are scoped to specific configs
secretStores:
  doppler:
    type: doppler
    token: dp.st.production.abcd1234  # Service token (read-only)
```

### 2. Rotate Tokens Regularly

```bash
# Create new service token
doppler configs tokens create production --name dsops-token-v2

# Update configuration
# Delete old token after verification
doppler configs tokens delete production dsops-token-v1
```

### 3. Use Environment-Specific Tokens

```yaml
# Don't use production tokens in development
secretStores:
  doppler-dev:
    type: doppler
    token: dp.st.dev.xyz789  # Development service token
  
  doppler-prod:
    type: doppler
    token: dp.st.production.abc123  # Production service token
```

### 4. Monitor Access Logs

```yaml
# Enable audit logging
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    audit:
      enabled: true
      webhook: https://my-audit-system.com/doppler-events
```

### 5. Use Least Privilege

```bash
# Create service tokens with minimal required access
doppler configs tokens create production \
  --name minimal-access \
  --config production \
  --project my-app
```

## Performance Optimization

### Caching Configuration

```yaml
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    cache:
      enabled: true
      ttl: 300  # 5 minutes
      
      # Cache based on config change frequency
      rules:
        - config: "production"
          ttl: 1800  # 30 minutes - stable
        
        - config: "dev"
          ttl: 60   # 1 minute - changes often
```

### Batch Fetching

```yaml
# Fetch all secrets at once (more efficient)
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    fetch_strategy: batch  # Default: individual

envs:
  production:
    # All secrets fetched in single API call
    DATABASE_URL:
      from:
        store: doppler
        name: DATABASE_URL
    
    API_KEY:
      from:
        store: doppler
        name: API_KEY
```

### Connection Pooling

```yaml
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    connection:
      pool_size: 5
      keep_alive: true
      timeout: 30s
```

## Rotation Support

### Automatic Rotation

```yaml
# Doppler integrates with external rotation systems
services:
  postgres:
    type: postgresql
    host: db.example.com
    doppler_integration:
      project: my-app
      config: production
      auto_sync: true

envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: doppler
        name: DATABASE_PASSWORD
      service: postgres
      rotation:
        # Doppler webhook triggers rotation
        webhook: https://api.doppler.com/v3/configs/config/secrets/webhook
        strategy: immediate
```

### Manual Rotation

```bash
# Update secret in Doppler (triggers dsops refresh)
doppler secrets set DATABASE_PASSWORD=new-password --config production

# Or use dsops rotation
dsops secrets rotate --env production --key DATABASE_PASSWORD
```

## Troubleshooting

### Authentication Issues

```bash
# Test Doppler connection
doppler auth status

# Test service token
curl -H "Authorization: Bearer $DOPPLER_TOKEN" \
  https://api.doppler.com/v3/me

# Verify project access
doppler secrets --project my-project --config production
```

### Secret Not Found

```bash
# List available secrets
doppler secrets --project my-project --config production

# Check config exists
doppler configs --project my-project

# Verify token scope
doppler configs tokens --config production
```

### Debug Configuration

```yaml
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    debug:
      enabled: true
      log_requests: true
      log_responses: false  # Don't log secrets
      trace_api_calls: true
```

### Rate Limiting

```yaml
# Handle API rate limits
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    rate_limit:
      requests_per_second: 10
      burst: 50
      backoff: exponential
```

## Migration Guide

### From Environment Variables

```bash
# Import .env file to Doppler
doppler secrets upload .env --project my-app --config dev

# Or individual secrets
while IFS='=' read -r key value; do
  doppler secrets set "$key=$value" --project my-app --config dev
done < .env
```

### From Other Secret Managers

```yaml
# Gradual migration strategy
secretStores:
  # Existing secret manager
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    token: ${VAULT_TOKEN}
  
  # New Doppler setup
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    project: my-app
    config: production

envs:
  production:
    # Migrate one secret at a time
    OLD_SECRET:
      from:
        store: vault
        path: secret/data/api-keys
        key: stripe
    
    NEW_SECRET:
      from:
        store: doppler
        name: STRIPE_API_KEY
```

### Bulk Migration Script

```bash
#!/bin/bash
# Migrate Vault KV to Doppler

# Source Vault secrets
vault kv get -format=json secret/myapp | jq -r '.data.data | to_entries[] | "\(.key)=\(.value)"' > secrets.env

# Import to Doppler
doppler secrets upload secrets.env --project myapp --config production

# Cleanup
rm secrets.env
```

## Monitoring and Alerting

### Activity Monitoring

```yaml
# Monitor Doppler activity via webhooks
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    webhooks:
      - url: https://monitoring.example.com/doppler
        events:
          - secret.created
          - secret.updated
          - secret.deleted
          - config.created
```

### Health Checks

```yaml
# Regular health checks
secretStores:
  doppler:
    type: doppler
    token: ${DOPPLER_TOKEN}
    health_check:
      enabled: true
      interval: 300s  # 5 minutes
      endpoint: https://api.doppler.com/v3/me
      timeout: 10s
```

## Best Practices Summary

1. **Use Service Tokens**: Scope tokens to specific configs
2. **Organize by Environment**: Clear project/config structure
3. **Enable Caching**: Reduce API calls with appropriate TTLs
4. **Monitor Usage**: Track secret access and changes
5. **Rotate Tokens**: Regular token rotation schedule
6. **Use References**: Leverage Doppler's built-in referencing
7. **Document Access**: Clear naming and tagging conventions

## Related Documentation

- [Doppler Documentation](https://docs.doppler.com/)
- [Doppler CLI Reference](https://docs.doppler.com/docs/cli)
- [Provider Comparison](/providers/comparison/)
- [Security Best Practices](/security/)
- [Integration Guides](/guides/integrations/)