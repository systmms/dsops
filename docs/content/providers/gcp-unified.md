---
title: "GCP Unified Provider"
description: "Smart routing to appropriate Google Cloud secret services"
lead: "The GCP Unified Provider automatically routes secret requests to the appropriate Google Cloud service based on the secret path pattern. It provides a single interface for Secret Manager, Cloud KMS, and other GCP secret services."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 320
---

## Overview

The GCP Unified Provider simplifies secret management across Google Cloud Platform by:

- **Automatic Routing**: Routes to Secret Manager, Cloud KMS, or other services based on path
- **Single Configuration**: One provider handles all GCP secret types
- **Consistent Interface**: Uniform URI format across services
- **Smart Defaults**: Automatically detects the appropriate service
- **Cross-Service Support**: Access secrets from multiple GCP services

## How It Works

The provider examines the secret path and automatically routes to:

| Path Pattern | Service | Example |
|--------------|---------|---------|
| `secrets/*` | Secret Manager | `secrets/my-secret` |
| `keys/*` | Cloud KMS | `keys/my-keyring/my-key` |
| `config/*` | Runtime Config | `config/my-config/my-var` |
| `sm://*` | Secret Manager (explicit) | `sm://my-secret` |
| `kms://*` | Cloud KMS (explicit) | `kms://my-keyring/my-key` |

## Prerequisites

1. **Google Cloud SDK** installed and configured
2. **Authentication** set up (gcloud auth or service account)
3. **Required APIs** enabled:
   ```bash
   gcloud services enable secretmanager.googleapis.com
   gcloud services enable cloudkms.googleapis.com
   gcloud services enable runtimeconfig.googleapis.com
   ```

## Configuration

### Basic Configuration

```yaml
version: 1

secretStores:
  gcp:
    type: gcp.unified
    project: my-project-id

envs:
  production:
    # Automatically routes to Secret Manager
    DATABASE_PASSWORD:
      from:
        store: gcp
        path: secrets/database-password
    
    # Automatically routes to Cloud KMS for encryption
    ENCRYPTION_KEY:
      from:
        store: gcp
        path: keys/app-keyring/data-key
```

### Advanced Configuration

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project-id
    
    # Optional: Service-specific overrides
    services:
      secret_manager:
        cache_ttl: 300
        locations:
          - us-central1
          - us-east1
      
      cloud_kms:
        keyring_location: us-central1
        default_algorithm: GOOGLE_SYMMETRIC_ENCRYPTION
      
      runtime_config:
        config_name: app-config
    
    # Optional: Authentication
    credentials:
      type: service_account
      path: ${GOOGLE_APPLICATION_CREDENTIALS}
    
    # Optional: Global defaults
    defaults:
      timeout: 30s
      retry_attempts: 3
```

### Multi-Project Support

```yaml
secretStores:
  gcp-prod:
    type: gcp.unified
    project: production-project
  
  gcp-shared:
    type: gcp.unified
    project: shared-services-project
  
  gcp-dev:
    type: gcp.unified
    project: development-project

envs:
  production:
    # Secrets from different projects
    APP_SECRET:
      from:
        store: gcp-prod
        path: secrets/app-secret
    
    SHARED_KEY:
      from:
        store: gcp-shared
        path: secrets/shared-api-key
```

## Usage Examples

### Secret Manager Integration

```yaml
# Implicit routing (recommended)
API_KEY:
  from:
    store: gcp
    path: secrets/api-key

# Explicit routing
API_KEY:
  from:
    store: gcp
    path: sm://api-key

# With version
API_KEY:
  from:
    store: gcp
    path: secrets/api-key
    version: latest  # or specific version number

# With transforms
DATABASE_URL:
  from:
    store: gcp
    path: secrets/db-config
  transform:
    - type: json_extract
      path: .connection_string
```

### Cloud KMS Integration

```yaml
# Decrypt data with KMS
ENCRYPTED_CONFIG:
  from:
    store: gcp
    path: keys/app-keyring/config-key
    operation: decrypt
    ciphertext: ${ENCRYPTED_DATA}

# Use KMS for signing
SIGNATURE:
  from:
    store: gcp
    path: keys/signing-keyring/app-key
    operation: sign
    data: ${DATA_TO_SIGN}
```

### Runtime Config Integration

```yaml
# Access runtime configuration
FEATURE_FLAGS:
  from:
    store: gcp
    path: config/app-config/feature-flags

# With variable extraction
SPECIFIC_FLAG:
  from:
    store: gcp
    path: config/app-config/features/new-ui
```

### Mixed Services

```yaml
envs:
  production:
    # Different services in one configuration
    DATABASE_PASSWORD:
      from:
        store: gcp
        path: secrets/db-password
    
    ENCRYPTION_KEY:
      from:
        store: gcp
        path: keys/data-keyring/master-key
    
    FEATURE_CONFIG:
      from:
        store: gcp
        path: config/features/production
```

## Path Patterns

### Smart Detection

The provider uses these patterns to route requests:

```yaml
# Pattern: secrets/* → Secret Manager
DATABASE_PASSWORD:
  from:
    store: gcp
    path: secrets/prod/database/password

# Pattern: keys/* → Cloud KMS
MASTER_KEY:
  from:
    store: gcp
    path: keys/prod-keyring/master-key

# Pattern: config/* → Runtime Config
APP_CONFIG:
  from:
    store: gcp
    path: config/prod-app/settings

# Explicit service prefix
SECRET_EXPLICIT:
  from:
    store: gcp
    path: sm://my-secret

KMS_EXPLICIT:
  from:
    store: gcp
    path: kms://keyring/key
```

### Custom Routing Rules

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    routing:
      # Custom patterns
      rules:
        - pattern: "vault/*"
          service: secret_manager
          strip_prefix: true
        
        - pattern: "crypto/*"
          service: cloud_kms
          keyring: app-crypto
        
        - pattern: "env/*"
          service: runtime_config
          config: app-environment
```

## Advanced Features

### Cross-Region Access

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    regions:
      primary: us-central1
      fallback:
        - us-east1
        - europe-west1
      
      # Service-specific regions
      services:
        secret_manager:
          locations:
            - us-central1
            - europe-west1
        
        cloud_kms:
          locations:
            - global
            - us-central1
```

### Batch Operations

```yaml
# Efficiently fetch multiple secrets
envs:
  production:
    # Batch fetch from same path
    _batch:
      db_secrets:
        store: gcp
        path: secrets/database/*
    
    # Individual extraction
    DB_HOST:
      from: 
        batch: db_secrets
        key: host
    
    DB_PORT:
      from:
        batch: db_secrets
        key: port
    
    DB_PASSWORD:
      from:
        batch: db_secrets
        key: password
```

### Service-Specific Features

```yaml
# Secret Manager features
API_SECRET:
  from:
    store: gcp
    path: secrets/api-key
    secret_manager:
      version: latest
      labels:
        environment: production

# KMS features
ENCRYPTED_DATA:
  from:
    store: gcp
    path: keys/data-keyring/encryption-key
    kms:
      algorithm: GOOGLE_SYMMETRIC_ENCRYPTION
      purpose: ENCRYPT_DECRYPT
      additional_authenticated_data: ${AAD}

# Runtime Config features
CONFIG_VALUE:
  from:
    store: gcp
    path: config/app/setting
    runtime_config:
      is_text: true
      watch: true
```

### Caching Strategy

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    cache:
      # Global cache settings
      enabled: true
      default_ttl: 300
      
      # Service-specific cache
      services:
        secret_manager:
          ttl: 600  # Cache secrets longer
        
        cloud_kms:
          ttl: 3600  # KMS keys are stable
        
        runtime_config:
          ttl: 60  # Config may change often
      
      # Pattern-based cache
      rules:
        - pattern: "*/static/*"
          ttl: 86400  # 24 hours for static
        
        - pattern: "*/dynamic/*"
          ttl: 0  # No cache for dynamic
```

## Security Features

### Access Control

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    access_control:
      # Enforce least privilege
      verify_permissions: true
      
      # Service account impersonation
      impersonate: deploy@my-project.iam.gserviceaccount.com
      
      # Resource-level IAM
      bindings:
        - resource: secrets/production/*
          role: roles/secretmanager.secretAccessor
          members:
            - serviceAccount:app@my-project.iam
```

### Audit and Compliance

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    audit:
      # Log all access
      log_access: true
      log_level: INFO
      
      # Compliance tags
      tags:
        compliance: pci-dss
        data_classification: sensitive
      
      # Export to Cloud Logging
      export:
        enabled: true
        include_metadata: true
```

### Encryption Options

```yaml
# Customer-managed encryption keys (CMEK)
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    encryption:
      # Default CMEK for Secret Manager
      secret_manager:
        kms_key: projects/PROJECT/locations/LOCATION/keyRings/RING/cryptoKeys/KEY
      
      # Envelope encryption
      use_envelope_encryption: true
```

## Performance Optimization

### Connection Management

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    connection:
      # Connection pooling
      pool_size: 20
      max_idle: 10
      
      # Timeouts
      connect_timeout: 10s
      read_timeout: 30s
      
      # Keep-alive
      keep_alive: true
      keep_alive_interval: 30s
```

### Parallel Fetching

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    performance:
      # Parallel operations
      max_concurrent: 10
      
      # Prefetching
      prefetch:
        enabled: true
        patterns:
          - secrets/common/*
          - config/app/*
```

## Troubleshooting

### Debug Mode

```yaml
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    debug:
      enabled: true
      log_requests: true
      log_responses: false  # Don't log sensitive data
      trace_routing: true
```

### Common Issues

#### Routing Errors

```bash
# Test routing logic
dsops secrets test --store gcp --path secrets/my-secret --dry-run

# Check service availability
gcloud services list --enabled | grep -E "(secretmanager|cloudkms|runtimeconfig)"
```

#### Permission Issues

```bash
# Check effective permissions
gcloud projects get-iam-policy PROJECT \
  --flatten="bindings[].members" \
  --filter="bindings.members:$(gcloud config get-value account)"

# Test specific service access
gcloud secrets list
gcloud kms keyrings list --location=global
```

#### Performance Problems

```yaml
# Enable metrics
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
    metrics:
      enabled: true
      export:
        type: cloud_monitoring
        interval: 60s
```

## Migration Guide

### From Individual Providers

```yaml
# Before: Multiple providers
secretStores:
  gcp-secrets:
    type: google.secretmanager
    project: my-project
  
  gcp-kms:
    type: google.cloudkms
    project: my-project
    location: us-central1

# After: Unified provider
secretStores:
  gcp:
    type: gcp.unified
    project: my-project
```

### Path Migration

```yaml
# Old paths → New paths
envs:
  production:
    # Before
    SECRET_OLD:
      from:
        store: gcp-secrets
        secret_id: my-secret
    
    # After
    SECRET_NEW:
      from:
        store: gcp
        path: secrets/my-secret
```

## Best Practices

1. **Use Smart Routing**: Let the provider detect services from paths
2. **Consistent Naming**: Follow pattern conventions for clarity
3. **Cache Appropriately**: Different TTLs for different services
4. **Monitor Performance**: Enable metrics and logging
5. **Test Routing**: Verify paths route to expected services
6. **Batch When Possible**: Reduce API calls with batch operations
7. **Handle Failures**: Implement retry and fallback strategies

## Related Documentation

- [Google Secret Manager Provider](/providers/google-secret-manager/)
- [Provider Comparison](/providers/comparison/)
- [Security Best Practices](/security/)
- [Performance Tuning](/guides/performance/)