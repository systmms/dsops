---
title: "Azure Unified Provider"
description: "Intelligent routing for all Azure secret services"
lead: "The Azure Unified Provider automatically routes secret requests to the appropriate Azure service based on the secret path. It provides a single interface for Key Vault, App Configuration, Storage, and other Azure services."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 430
---

## Overview

The Azure Unified Provider simplifies Azure secret management by:

- **Automatic Service Detection**: Routes to the right Azure service based on path
- **Single Configuration**: One provider for all Azure secret services
- **Intelligent Authentication**: Uses the best available auth method
- **Cross-Service Support**: Access Key Vault, Storage, SQL, and more
- **Seamless Migration**: Easy transition between Azure services

## Service Routing

The provider automatically detects and routes to:

| Path Pattern | Azure Service | Example |
|--------------|--------------|---------|
| `vault/*` or `kv/*` | Key Vault | `vault/prod-keyvault/database-password` |
| `storage/*` | Storage Account | `storage/myaccount/key1` |
| `sql/*` | SQL Database | `sql/myserver/mydb/connection` |
| `cosmos/*` | Cosmos DB | `cosmos/myaccount/primary-key` |
| `config/*` | App Configuration | `config/myconfig/feature-flag` |
| `redis/*` | Redis Cache | `redis/mycache/primary-key` |

## Prerequisites

1. **Azure CLI** installed and configured
2. **Authentication** configured (Azure CLI, Managed Identity, or Service Principal)
3. **Appropriate permissions** for each service you'll access
4. **Network access** to Azure services

## Configuration

### Basic Configuration

```yaml
version: 1

secretStores:
  azure:
    type: azure.unified
    # Automatically detects authentication method

envs:
  production:
    # Automatically routes to Key Vault
    DATABASE_PASSWORD:
      from:
        store: azure
        path: vault/prod-keyvault/database-password
    
    # Automatically routes to Storage
    STORAGE_KEY:
      from:
        store: azure
        path: storage/prod-storage/key1
```

### Advanced Configuration

```yaml
secretStores:
  azure:
    type: azure.unified
    
    # Optional: Default subscription
    subscription: ${AZURE_SUBSCRIPTION_ID}
    
    # Optional: Authentication preferences
    auth:
      preferred_method: managed_identity
      fallback_chain:
        - managed_identity
        - azure_cli
        - service_principal
      
      # Service principal (if used)
      service_principal:
        client_id: ${AZURE_CLIENT_ID}
        client_secret: ${AZURE_CLIENT_SECRET}
        tenant_id: ${AZURE_TENANT_ID}
    
    # Optional: Service-specific settings
    services:
      keyvault:
        api_version: "7.4"
        cache_ttl: 300
      
      storage:
        use_secondary: true
        sas_duration: 24h
      
      sql:
        connection_timeout: 30
        encrypt: true
    
    # Optional: Performance settings
    cache:
      enabled: true
      default_ttl: 600
```

## Usage Examples

### Key Vault Secrets

```yaml
# Basic secret access
API_KEY:
  from:
    store: azure
    path: vault/mykeyvault/api-key

# With version
API_KEY:
  from:
    store: azure
    path: vault/mykeyvault/api-key
    version: abc123def456

# Certificate access
TLS_CERT:
  from:
    store: azure
    path: vault/mykeyvault/app-cert
    type: certificate

# Key operations
SIGNATURE:
  from:
    store: azure
    path: vault/mykeyvault/signing-key
    operation: sign
    algorithm: RS256
    data: ${DATA_TO_SIGN}
```

### Storage Account

```yaml
# Storage account key
STORAGE_KEY:
  from:
    store: azure
    path: storage/myaccount/key1

# Generate SAS token
BLOB_SAS:
  from:
    store: azure
    path: storage/myaccount/sas
    permissions: read,list
    expiry: 24h
    container: mycontainer

# Connection string
STORAGE_CONNECTION:
  from:
    store: azure
    path: storage/myaccount/connection-string
```

### SQL Database

```yaml
# SQL password
SQL_PASSWORD:
  from:
    store: azure
    path: sql/myserver/mydb/password

# Full connection string
SQL_CONNECTION:
  from:
    store: azure
    path: sql/myserver/mydb/connection-string
    auth_type: managed_identity  # or sql_auth

# Admin password
SQL_ADMIN_PASSWORD:
  from:
    store: azure
    path: sql/myserver/admin-password
```

### Cosmos DB

```yaml
# Primary key
COSMOS_KEY:
  from:
    store: azure
    path: cosmos/myaccount/primary-key

# Connection string
COSMOS_CONNECTION:
  from:
    store: azure
    path: cosmos/myaccount/connection-string

# Read-only key
COSMOS_READONLY_KEY:
  from:
    store: azure
    path: cosmos/myaccount/readonly-key
```

### App Configuration

```yaml
# Configuration value
FEATURE_FLAG:
  from:
    store: azure
    path: config/myappconfig/feature-new-ui

# With label
SETTING:
  from:
    store: azure
    path: config/myappconfig/database-timeout
    label: production

# Key Vault reference resolution
SECRET_FROM_CONFIG:
  from:
    store: azure
    path: config/myappconfig/api-key
    resolve_references: true
```

### Redis Cache

```yaml
# Primary access key
REDIS_KEY:
  from:
    store: azure
    path: redis/mycache/primary-key

# Connection string
REDIS_CONNECTION:
  from:
    store: azure
    path: redis/mycache/connection-string
    ssl: true
```

## Advanced Patterns

### Cross-Subscription Access

```yaml
# Access resources in different subscriptions
secretStores:
  azure:
    type: azure.unified
    default_subscription: ${PRIMARY_SUBSCRIPTION}

envs:
  production:
    # Default subscription
    PRIMARY_SECRET:
      from:
        store: azure
        path: vault/primary-kv/secret
    
    # Different subscription
    SHARED_SECRET:
      from:
        store: azure
        path: vault/shared-kv/secret
        subscription: ${SHARED_SUBSCRIPTION}
```

### Service Composition

```yaml
envs:
  production:
    # Compose connection string from multiple sources
    DATABASE_URL:
      from:
        store: azure
        path: sql/myserver/mydb/connection-template
      transform:
        - type: template
          template: "Server={{.Host}};Database={{.Database}};User ID={{.User}};Password={{.Password}}"
          values:
            Host:
              from:
                store: azure
                path: config/app/db-host
            Database:
              from:
                store: azure
                path: config/app/db-name
            User:
              from:
                store: azure
                path: config/app/db-user
            Password:
              from:
                store: azure
                path: vault/prod-kv/db-password
```

### Batch Operations

```yaml
# Fetch multiple secrets efficiently
envs:
  production:
    # Define batch from Key Vault
    _batch:
      app_secrets:
        store: azure
        path: vault/prod-kv/*
        filter: "tags.app=myapp"
    
    # Extract individual secrets
    API_KEY:
      from:
        batch: app_secrets
        key: api-key
    
    CLIENT_SECRET:
      from:
        batch: app_secrets
        key: client-secret
```

### Multi-Region Failover

```yaml
secretStores:
  azure:
    type: azure.unified
    regions:
      primary: eastus
      secondary: westus
      
      # Service-specific regions
      services:
        keyvault:
          vaults:
            - name: prod-kv-eastus
              region: eastus
              priority: 1
            - name: prod-kv-westus
              region: westus
              priority: 2
```

## Authentication

### Automatic Detection

The provider automatically tries authentication methods in order:

1. **Managed Identity** (if running in Azure)
2. **Azure CLI** (if logged in)
3. **Environment Variables** (if set)
4. **Service Principal** (if configured)

### Explicit Configuration

```yaml
# Force specific authentication
secretStores:
  azure-managed:
    type: azure.unified
    auth:
      method: managed_identity
      client_id: ${USER_ASSIGNED_IDENTITY_CLIENT_ID}
  
  azure-sp:
    type: azure.unified
    auth:
      method: service_principal
      client_id: ${AZURE_CLIENT_ID}
      client_secret: ${AZURE_CLIENT_SECRET}
      tenant_id: ${AZURE_TENANT_ID}
  
  azure-cli:
    type: azure.unified
    auth:
      method: azure_cli
      subscription: ${AZURE_SUBSCRIPTION_ID}
```

### Per-Service Authentication

```yaml
secretStores:
  azure:
    type: azure.unified
    auth:
      # Default auth
      method: managed_identity
      
      # Service-specific auth
      services:
        storage:
          method: connection_string
          connection_string: ${STORAGE_CONNECTION_STRING}
        
        sql:
          method: sql_auth
          username: ${SQL_USERNAME}
          password: ${SQL_PASSWORD}
```

## Performance Optimization

### Intelligent Caching

```yaml
secretStores:
  azure:
    type: azure.unified
    cache:
      enabled: true
      
      # Service-specific TTLs
      services:
        keyvault:
          secret_ttl: 3600      # 1 hour
          certificate_ttl: 86400 # 24 hours
        
        storage:
          key_ttl: 7200         # 2 hours
          sas_ttl: 0            # Never cache SAS
        
        config:
          ttl: 300              # 5 minutes
      
      # Pattern-based caching
      rules:
        - pattern: "*/prod/*"
          ttl: 3600
        - pattern: "*/dev/*"
          ttl: 300
```

### Connection Pooling

```yaml
secretStores:
  azure:
    type: azure.unified
    connections:
      pool_size: 20
      idle_timeout: 300
      
      # Per-service pools
      services:
        keyvault:
          pool_size: 10
        storage:
          pool_size: 5
        sql:
          pool_size: 15
```

### Parallel Processing

```yaml
secretStores:
  azure:
    type: azure.unified
    performance:
      # Fetch secrets in parallel
      max_concurrent: 10
      
      # Batch size for list operations
      batch_size: 100
      
      # Prefetch common secrets
      prefetch:
        - vault/prod-kv/common/*
        - config/app/features/*
```

## Security Best Practices

### 1. Use Path-Based Access Control

```yaml
secretStores:
  azure:
    type: azure.unified
    access_control:
      # Define allowed paths
      allowed_paths:
        - vault/prod-kv/*
        - storage/prod-storage/*
        - sql/prod-server/*
      
      # Block specific paths
      blocked_paths:
        - vault/*/admin-*
        - sql/*/sa-password
```

### 2. Audit All Access

```yaml
secretStores:
  azure:
    type: azure.unified
    audit:
      enabled: true
      log_level: INFO
      
      # Log to Azure Monitor
      destinations:
        - type: azure_monitor
          workspace_id: ${LOG_ANALYTICS_WORKSPACE_ID}
        
        - type: file
          path: /var/log/dsops/azure-access.log
```

### 3. Network Isolation

```yaml
secretStores:
  azure:
    type: azure.unified
    network:
      # Use private endpoints
      use_private_endpoints: true
      
      # Service-specific networking
      services:
        keyvault:
          private_dns_zone: privatelink.vaultcore.azure.net
        
        storage:
          private_dns_zone: privatelink.blob.core.windows.net
```

### 4. Rotation Integration

```yaml
services:
  azure-sql:
    type: azure.sql
    provider: azure

envs:
  production:
    SQL_PASSWORD:
      from:
        store: azure
        path: sql/myserver/mydb/app-user-password
      service: azure-sql
      rotation:
        strategy: two-key
        schedule: "0 2 * * SUN"
```

## Troubleshooting

### Path Resolution Issues

```bash
# Test path resolution
dsops secrets test --store azure --path vault/mykv/secret --debug

# Check service detection
dsops provider info azure --show-routes
```

### Authentication Problems

```bash
# Check current auth
az account show

# Test specific service access
# Key Vault
az keyvault secret list --vault-name mykv

# Storage
az storage account keys list --account-name myaccount

# SQL
az sql server list
```

### Debug Configuration

```yaml
secretStores:
  azure:
    type: azure.unified
    debug:
      enabled: true
      log_auth: true
      log_routes: true
      trace_requests: true
      
      # Service-specific debug
      services:
        keyvault:
          log_operations: true
        storage:
          log_sas_generation: true
```

### Common Error Messages

| Error | Cause | Solution |
|-------|-------|----------|
| "No route found for path" | Invalid path format | Check path pattern documentation |
| "Authentication failed" | Wrong auth method | Verify credentials and permissions |
| "Service not available" | Service disabled | Enable required Azure services |
| "Access denied" | Missing permissions | Grant appropriate RBAC roles |

## Migration Guide

### From Individual Providers

```yaml
# Before: Multiple providers
secretStores:
  kv:
    type: azure.keyvault
    vault_name: mykv
  
  storage:
    type: azure.storage
    account_name: myaccount
  
  sql:
    type: azure.sql
    server: myserver

# After: Unified provider
secretStores:
  azure:
    type: azure.unified
    # All services accessible through path routing
```

### Path Migration Examples

```yaml
# Key Vault
# Before:
DATABASE_PASSWORD:
  from:
    store: kv
    name: database-password

# After:
DATABASE_PASSWORD:
  from:
    store: azure
    path: vault/mykv/database-password

# Storage
# Before:
STORAGE_KEY:
  from:
    store: storage
    key: key1

# After:
STORAGE_KEY:
  from:
    store: azure
    path: storage/myaccount/key1
```

### Gradual Migration

```yaml
# Keep both during transition
secretStores:
  # Old individual providers
  azure-kv:
    type: azure.keyvault
    vault_name: mykv
  
  # New unified provider
  azure:
    type: azure.unified

envs:
  production:
    # Migrate one secret at a time
    OLD_SECRET:
      from:
        store: azure-kv
        name: api-key
    
    NEW_SECRET:
      from:
        store: azure
        path: vault/mykv/api-key
```

## Best Practices Summary

1. **Use Path Conventions**: Follow standard path patterns for clarity
2. **Leverage Auto-Detection**: Let the provider handle routing
3. **Cache Strategically**: Different TTLs for different services
4. **Monitor Performance**: Enable metrics and logging
5. **Plan for Failover**: Configure multi-region support
6. **Audit Everything**: Track all secret access
7. **Test Migrations**: Verify paths before switching

## Related Documentation

- [Azure Key Vault Provider](/providers/azure-key-vault/)
- [Azure Managed Identity Provider](/providers/azure-managed-identity/)
- [Provider Comparison](/providers/comparison/)
- [Security Best Practices](/security/)
- [Performance Guide](/guides/performance/)