---
title: "Google Secret Manager"
description: "Centralized secret management for Google Cloud Platform"
lead: "Google Secret Manager provides a secure and convenient method for storing API keys, passwords, certificates, and other sensitive data in GCP. Integrates seamlessly with Google Cloud IAM for access control."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 310
---

## Overview

Google Secret Manager is GCP's native secret management service, offering:

- **Centralized Management**: Store all secrets in one secure location
- **Automatic Encryption**: Secrets encrypted at rest with Google-managed keys
- **Version Control**: Track secret changes with automatic versioning
- **IAM Integration**: Fine-grained access control with Cloud IAM
- **Audit Logging**: Complete audit trail via Cloud Logging
- **Global Replication**: Automatic replication across regions

## Prerequisites

Before using the Google Secret Manager provider:

1. **Google Cloud SDK**:
   ```bash
   # Install gcloud CLI
   curl https://sdk.cloud.google.com | bash
   exec -l $SHELL
   
   # Initialize and authenticate
   gcloud init
   gcloud auth application-default login
   ```

2. **Enable Secret Manager API**:
   ```bash
   gcloud services enable secretmanager.googleapis.com
   ```

3. **Required IAM Permissions**:
   - `secretmanager.secrets.get`
   - `secretmanager.versions.access`
   - `secretmanager.versions.list` (for latest version)

## Configuration

### Basic Configuration

```yaml
version: 1

secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id

envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: gcp
        name: database-password
```

### Advanced Configuration

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    
    # Optional: Override default credentials
    credentials:
      type: service_account
      path: /path/to/service-account-key.json
    
    # Optional: Default location for new secrets
    locations:
      - us-central1
      - us-east1
    
    # Optional: Customer-managed encryption key
    encryption_key: projects/PROJECT/locations/LOCATION/keyRings/RING/cryptoKeys/KEY
```

### Authentication Methods

#### 1. Application Default Credentials (Recommended)

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    # No credentials field - uses ADC
```

#### 2. Service Account Key File

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    credentials:
      type: service_account
      path: ${GOOGLE_APPLICATION_CREDENTIALS}
```

#### 3. Workload Identity (GKE)

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    credentials:
      type: workload_identity
      service_account: my-service@project.iam.gserviceaccount.com
```

#### 4. Impersonation

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    credentials:
      type: impersonated
      target_service_account: deploy@project.iam.gserviceaccount.com
      scopes:
        - https://www.googleapis.com/auth/cloud-platform
```

## URI Format

### Basic Secret Reference

```yaml
# Latest version (recommended)
DATABASE_PASSWORD:
  from:
    store: gcp
    name: database-password

# Specific version
DATABASE_PASSWORD:
  from:
    store: gcp
    name: database-password
    version: 2

# Version alias
DATABASE_PASSWORD:
  from:
    store: gcp
    name: database-password
    version: latest  # or "1", "2", etc.
```

### Cross-Project Access

```yaml
secretStores:
  gcp-prod:
    type: google.secretmanager
    project: prod-project
  
  gcp-shared:
    type: google.secretmanager
    project: shared-secrets-project

envs:
  production:
    # From prod project
    APP_SECRET:
      from:
        store: gcp-prod
        name: app-secret
    
    # From shared project
    SHARED_KEY:
      from:
        store: gcp-shared
        name: shared-api-key
```

### Secret Labels and Filters

```yaml
# Reference secrets by labels
API_KEYS:
  from:
    store: gcp
    filter: "labels.environment=production AND labels.service=api"
```

## Secret Types

### Simple Values

```yaml
# String secret
DATABASE_PASSWORD:
  from:
    store: gcp
    name: db-password

# Binary secret (automatically base64 decoded)
TLS_CERTIFICATE:
  from:
    store: gcp
    name: tls-cert
    encoding: binary
```

### JSON Secrets

```yaml
# Full JSON secret
CONFIG:
  from:
    store: gcp
    name: app-config

# Extract specific field
DATABASE_HOST:
  from:
    store: gcp
    name: database-config
  transform:
    - type: json_extract
      path: .host
```

### Multi-Line Secrets

```yaml
# Certificates, keys, etc.
PRIVATE_KEY:
  from:
    store: gcp
    name: app-private-key
```

## Advanced Usage

### Secret Rotation

Google Secret Manager integrates with dsops rotation:

```yaml
services:
  postgres-prod:
    type: postgresql
    host: db.example.com
    secret_manager:
      project: my-project
      rotation:
        enabled: true
        schedule: "0 2 * * SUN"  # Weekly

envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: gcp
        name: postgres-password
      service: postgres-prod
      rotation:
        strategy: two-key
```

### Secret Policies

Configure secret-level policies:

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    policies:
      replication:
        automatic: true
        locations:
          - us-central1
          - europe-west1
      
      rotation:
        rotation_period: 90d
        next_rotation_time: "2025-01-01T00:00:00Z"
```

### Access Control

Manage IAM bindings for secrets:

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    access_control:
      # Grant access to specific secrets
      bindings:
        - secret: database-password
          members:
            - serviceAccount:app@project.iam.gserviceaccount.com
          role: roles/secretmanager.secretAccessor
```

### Audit Configuration

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    audit:
      # Log all access
      log_access: true
      
      # Alert on unauthorized access
      alerts:
        - type: unauthorized_access
          notification_channel: projects/PROJECT/notificationChannels/CHANNEL
```

## Security Best Practices

### 1. Use Least Privilege IAM

```bash
# Create custom role with minimal permissions
gcloud iam roles create secret_reader \
  --project=my-project \
  --permissions=secretmanager.versions.access

# Grant to service account
gcloud projects add-iam-policy-binding my-project \
  --member=serviceAccount:app@my-project.iam.gserviceaccount.com \
  --role=projects/my-project/roles/secret_reader
```

### 2. Enable Audit Logging

```yaml
# Cloud Logging query for secret access
resource.type="secretmanager.googleapis.com/Secret"
protoPayload.methodName="google.cloud.secretmanager.v1.SecretManagerService.AccessSecretVersion"
```

### 3. Use VPC Service Controls

```bash
# Add Secret Manager to VPC perimeter
gcloud access-context-manager perimeters update my-perimeter \
  --add-restricted-services=secretmanager.googleapis.com
```

### 4. Implement Secret Scanning

```yaml
# Pre-commit hook example
- repo: https://github.com/Yelp/detect-secrets
  hooks:
    - id: detect-secrets
      args: ['--exclude-secrets', 'projects/.*/secrets/.*']
```

## Performance Optimization

### Caching Strategy

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    cache:
      # Cache for 5 minutes
      ttl: 300
      
      # Max cache size
      max_entries: 1000
```

### Batch Operations

```yaml
# Fetch multiple secrets efficiently
envs:
  production:
    _refs:
      - &db_config
        store: gcp
        name: database-config
    
    DB_HOST:
      from: *db_config
      transform:
        - type: json_extract
          path: .host
    
    DB_PORT:
      from: *db_config
      transform:
        - type: json_extract
          path: .port
```

### Connection Pooling

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    connection:
      # Reuse connections
      pool_size: 10
      
      # Connection timeout
      timeout: 30s
```

## Cost Optimization

### 1. Minimize Version Creation

```yaml
# Use single JSON secret for multiple values
APP_CONFIG:
  from:
    store: gcp
    name: app-config  # Contains multiple settings

# Extract individual values
API_KEY:
  from:
    store: gcp
    name: app-config
  transform:
    - type: json_extract
      path: .api_key
```

### 2. Regional Secrets

```yaml
# Use regional secrets when possible
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    locations:
      - us-central1  # Single region = lower cost
```

### 3. Access Patterns

```yaml
# Cache frequently accessed secrets
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    cache:
      # Cache static secrets longer
      rules:
        - pattern: "static-*"
          ttl: 3600
        - pattern: "dynamic-*"
          ttl: 60
```

## Troubleshooting

### Permission Denied

```bash
# Check current authentication
gcloud auth list

# Verify project
gcloud config get-value project

# Test access
gcloud secrets versions access latest --secret=my-secret

# Check IAM permissions
gcloud projects get-iam-policy PROJECT_ID \
  --flatten="bindings[].members" \
  --filter="bindings.role:secretmanager"
```

### Secret Not Found

```bash
# List all secrets
gcloud secrets list --project=PROJECT_ID

# Check secret exists
gcloud secrets describe SECRET_NAME --project=PROJECT_ID

# Verify secret has versions
gcloud secrets versions list SECRET_NAME --project=PROJECT_ID
```

### Authentication Issues

```bash
# Reset application default credentials
gcloud auth application-default revoke
gcloud auth application-default login

# For service accounts
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json

# Verify credentials
gcloud auth application-default print-access-token
```

### Performance Issues

Enable debug logging:

```yaml
secretStores:
  gcp:
    type: google.secretmanager
    project: my-project-id
    debug: true
    metrics:
      enabled: true
      export_interval: 60s
```

## Migration Guide

### From Environment Variables

```bash
#!/bin/bash
# Migrate .env to Secret Manager

while IFS='=' read -r key value; do
  echo "$value" | gcloud secrets create "$key" \
    --data-file=- \
    --labels=migrated=true,source=env
done < .env
```

### From Other Secret Stores

```yaml
# Gradual migration approach
secretStores:
  old-vault:
    type: hashicorp.vault
    url: https://vault.example.com
  
  gcp:
    type: google.secretmanager
    project: my-project-id

envs:
  production:
    # Migrate one secret at a time
    OLD_SECRET:
      from:
        store: old-vault
        path: secret/data/app
        key: password
    
    NEW_SECRET:
      from:
        store: gcp
        name: app-password
```

## Best Practices Summary

1. **Always use versioning**: Reference `latest` unless you need a specific version
2. **Implement proper IAM**: Use least-privilege access with custom roles
3. **Enable audit logging**: Monitor all secret access
4. **Use labels**: Organize secrets with meaningful labels
5. **Implement rotation**: Use dsops rotation features for automatic updates
6. **Cache appropriately**: Balance performance with freshness
7. **Monitor costs**: Use regional secrets and batch operations

## Related Documentation

- [Google Secret Manager Docs](https://cloud.google.com/secret-manager/docs)
- [GCP Unified Provider](/providers/gcp-unified/)
- [Provider Comparison](/providers/comparison/)
- [Security Best Practices](/security/)
- [Rotation Guide](/rotation/)