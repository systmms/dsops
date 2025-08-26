---
title: "Azure Managed Identity"
description: "Passwordless authentication for Azure resources"
lead: "Azure Managed Identity provides automatic identity management for applications running in Azure. It eliminates the need for credentials in code by providing Azure-managed identities for Azure services."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 420
---

## Overview

Azure Managed Identity offers secure, credential-free authentication:

- **No Credentials**: Eliminates passwords and keys in code
- **Automatic Rotation**: Azure handles credential lifecycle
- **Two Types**: System-assigned and user-assigned identities
- **Wide Support**: Works with most Azure services
- **Zero Management**: No manual rotation or expiry handling
- **RBAC Integration**: Standard Azure role-based access control

## Identity Types

### System-Assigned Identity

- Created automatically with the Azure resource
- Tied to resource lifecycle (deleted with resource)
- One identity per resource
- Perfect for single-resource scenarios

### User-Assigned Identity

- Created as standalone Azure resource
- Can be assigned to multiple resources
- Independent lifecycle management
- Better for shared access scenarios

## Prerequisites

1. **Azure Resource**: Must be running on Azure (VM, App Service, AKS, etc.)
2. **Identity Enabled**: Managed identity must be enabled on the resource
3. **RBAC Permissions**: Identity must have appropriate role assignments
4. **Network Access**: Resource must reach Azure AD endpoints

## Configuration

### Basic Configuration

```yaml
version: 1

secretStores:
  azure:
    type: azure.managed_identity
    # Automatically uses the managed identity

envs:
  production:
    # Access Key Vault secret
    DATABASE_PASSWORD:
      from:
        store: azure
        vault: prod-keyvault
        secret: database-password
```

### System-Assigned Identity

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    # Uses system-assigned identity by default
    
    # Optional: Explicit configuration
    identity:
      type: system_assigned

envs:
  production:
    SECRET:
      from:
        store: azure
        vault: my-keyvault
        secret: app-secret
```

### User-Assigned Identity

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    identity:
      type: user_assigned
      client_id: 12345678-1234-1234-1234-123456789012
      # Or use resource ID
      resource_id: /subscriptions/SUB_ID/resourcegroups/RG/providers/Microsoft.ManagedIdentity/userAssignedIdentities/my-identity

envs:
  production:
    SECRET:
      from:
        store: azure
        vault: my-keyvault
        secret: app-secret
```

### Multiple Identities

```yaml
secretStores:
  # Default managed identity
  azure-default:
    type: azure.managed_identity
  
  # Specific user-assigned identity
  azure-data:
    type: azure.managed_identity
    identity:
      type: user_assigned
      client_id: ${DATA_IDENTITY_CLIENT_ID}
  
  # Another user-assigned identity
  azure-app:
    type: azure.managed_identity
    identity:
      type: user_assigned
      client_id: ${APP_IDENTITY_CLIENT_ID}

envs:
  production:
    # Use different identities for different secrets
    DATABASE_PASSWORD:
      from:
        store: azure-data
        vault: data-keyvault
        secret: db-password
    
    APP_SECRET:
      from:
        store: azure-app
        vault: app-keyvault
        secret: api-key
```

## Supported Services

### Azure Key Vault

```yaml
# Access Key Vault secrets
API_KEY:
  from:
    store: azure
    vault: my-keyvault
    secret: api-key

# Access certificates
TLS_CERT:
  from:
    store: azure
    vault: my-keyvault
    secret: app-certificate
    type: certificate

# Access keys
ENCRYPTION_KEY:
  from:
    store: azure
    vault: my-keyvault
    secret: data-key
    type: key
```

### Azure Storage

```yaml
# Access storage account keys
STORAGE_KEY:
  from:
    store: azure
    resource_type: storage
    account: mystorageaccount
    key: key1

# Generate SAS token
STORAGE_SAS:
  from:
    store: azure
    resource_type: storage
    account: mystorageaccount
    operation: generate_sas
    permissions: rl  # read, list
    expiry: 24h
```

### Azure SQL Database

```yaml
# Get connection string with managed identity auth
SQL_CONNECTION:
  from:
    store: azure
    resource_type: sql
    server: myserver.database.windows.net
    database: mydb
    format: connection_string
```

### Azure Cosmos DB

```yaml
# Access Cosmos DB keys
COSMOS_KEY:
  from:
    store: azure
    resource_type: cosmosdb
    account: mycosmosaccount
    key: primary

# Connection string
COSMOS_CONNECTION:
  from:
    store: azure
    resource_type: cosmosdb
    account: mycosmosaccount
    format: connection_string
```

## Platform-Specific Setup

### Azure App Service

```bash
# Enable system-assigned identity
az webapp identity assign \
  --resource-group myRG \
  --name myapp

# Grant Key Vault access
az keyvault set-policy \
  --name mykeyvault \
  --object-id <IDENTITY_OBJECT_ID> \
  --secret-permissions get list
```

### Azure Virtual Machines

```bash
# Enable system-assigned identity
az vm identity assign \
  --resource-group myRG \
  --name myvm

# Assign user-assigned identity
az vm identity assign \
  --resource-group myRG \
  --name myvm \
  --identities /subscriptions/SUB_ID/resourcegroups/myRG/providers/Microsoft.ManagedIdentity/userAssignedIdentities/myidentity
```

### Azure Kubernetes Service (AKS)

```bash
# Enable pod identity
az aks update \
  --resource-group myRG \
  --name myaks \
  --enable-pod-identity

# Create pod identity
az aks pod-identity add \
  --resource-group myRG \
  --cluster-name myaks \
  --namespace default \
  --name my-pod-identity \
  --identity-resource-id <IDENTITY_RESOURCE_ID>
```

```yaml
# Pod configuration
apiVersion: v1
kind: Pod
metadata:
  name: my-app
  labels:
    aadpodidbinding: my-pod-identity
spec:
  containers:
  - name: app
    image: myapp
    env:
    - name: DATABASE_PASSWORD
      value: "$(dsops exec --env production printenv DATABASE_PASSWORD)"
```

### Azure Container Instances

```bash
# Create container with managed identity
az container create \
  --resource-group myRG \
  --name mycontainer \
  --image myimage \
  --assign-identity [system] \
  --environment-variables \
    DSOPS_CONFIG=/app/dsops.yaml
```

## Advanced Features

### Cross-Subscription Access

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    # Access resources in different subscription
    default_subscription: ${PRIMARY_SUBSCRIPTION_ID}
    
    # Per-resource subscription override
    resources:
      - type: keyvault
        name: shared-keyvault
        subscription: ${SHARED_SUBSCRIPTION_ID}

envs:
  production:
    SHARED_SECRET:
      from:
        store: azure
        vault: shared-keyvault
        secret: shared-api-key
        subscription: ${SHARED_SUBSCRIPTION_ID}
```

### Token Management

```yaml
# Get Azure AD token directly
AAD_TOKEN:
  from:
    store: azure
    operation: get_token
    resource: https://vault.azure.net

# Custom resource token
CUSTOM_API_TOKEN:
  from:
    store: azure
    operation: get_token
    resource: https://myapi.example.com
    
# Graph API token
GRAPH_TOKEN:
  from:
    store: azure
    operation: get_token
    resource: https://graph.microsoft.com
```

### Fallback Strategies

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    fallback:
      # Try managed identity first, fall back to CLI
      - type: managed_identity
      - type: azure_cli
      - type: service_principal
        client_id: ${AZURE_CLIENT_ID}
        client_secret: ${AZURE_CLIENT_SECRET}
        tenant_id: ${AZURE_TENANT_ID}
```

## Security Best Practices

### 1. Use System-Assigned When Possible

```yaml
# Preferred for single-resource scenarios
secretStores:
  azure:
    type: azure.managed_identity
    # System-assigned is default and most secure
```

### 2. Least Privilege Access

```bash
# Grant minimal required permissions
az role assignment create \
  --assignee <IDENTITY_OBJECT_ID> \
  --role "Key Vault Secrets User" \
  --scope /subscriptions/SUB_ID/resourceGroups/RG/providers/Microsoft.KeyVault/vaults/mykeyvault/secrets/mysecret
```

### 3. Resource Locks

```bash
# Prevent identity deletion
az lock create \
  --name identity-lock \
  --resource-group myRG \
  --resource-type Microsoft.ManagedIdentity/userAssignedIdentities \
  --resource-name myidentity \
  --lock-type CanNotDelete
```

### 4. Audit Access

```bash
# Enable diagnostic logs
az monitor diagnostic-settings create \
  --name identity-logs \
  --resource <IDENTITY_RESOURCE_ID> \
  --logs '[{"category": "AuditLogs", "enabled": true}]' \
  --workspace <LOG_ANALYTICS_WORKSPACE_ID>
```

## Performance Optimization

### Token Caching

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    cache:
      # Cache tokens until near expiry
      token_cache:
        enabled: true
        buffer: 300  # Refresh 5 minutes before expiry
      
      # Cache secret values
      secret_cache:
        enabled: true
        ttl: 600  # 10 minutes
```

### Parallel Fetching

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    performance:
      # Fetch secrets in parallel
      max_concurrent: 10
      
      # Batch Key Vault requests
      batch_size: 25
```

### Regional Optimization

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    # Use nearest region
    preferred_regions:
      - eastus
      - westus
      - centralus
```

## Troubleshooting

### Identity Not Found

```bash
# Check if identity is enabled
az vm identity show \
  --resource-group myRG \
  --name myvm

# For App Service
az webapp identity show \
  --resource-group myRG \
  --name myapp

# List all identities in subscription
az identity list --query "[].{name:name, clientId:clientId}"
```

### Permission Denied

```bash
# Check role assignments
az role assignment list \
  --assignee <IDENTITY_OBJECT_ID> \
  --all

# Verify Key Vault access policy
az keyvault show \
  --name mykeyvault \
  --query "properties.accessPolicies[?objectId=='<IDENTITY_OBJECT_ID>']"

# Test access
curl -H "Metadata: true" \
  "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://vault.azure.net"
```

### Token Issues

```bash
# Test IMDS endpoint (from Azure VM)
curl -H "Metadata: true" \
  "http://169.254.169.254/metadata/instance?api-version=2021-02-01"

# Get token manually
curl -H "Metadata: true" \
  "http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com/"
```

### Debug Mode

```yaml
secretStores:
  azure:
    type: azure.managed_identity
    debug:
      enabled: true
      log_tokens: false  # Never log tokens
      log_requests: true
      trace_identity: true
```

## Common Patterns

### Multi-Environment Setup

```yaml
secretStores:
  azure:
    type: azure.managed_identity

envs:
  development:
    DATABASE_PASSWORD:
      from:
        store: azure
        vault: dev-keyvault
        secret: database-password
  
  production:
    DATABASE_PASSWORD:
      from:
        store: azure
        vault: prod-keyvault
        secret: database-password
```

### Hybrid Authentication

```yaml
# Use managed identity in Azure, service principal locally
secretStores:
  azure:
    type: azure.managed_identity
    fallback:
      - type: managed_identity
      - type: environment
        client_id_env: AZURE_CLIENT_ID
        client_secret_env: AZURE_CLIENT_SECRET
        tenant_id_env: AZURE_TENANT_ID
```

### Cross-Service Access

```yaml
envs:
  production:
    # Key Vault secret
    API_KEY:
      from:
        store: azure
        vault: mykeyvault
        secret: api-key
    
    # Storage account key
    STORAGE_KEY:
      from:
        store: azure
        resource_type: storage
        account: mystorageaccount
        key: key1
    
    # Cosmos DB connection
    COSMOS_CONNECTION:
      from:
        store: azure
        resource_type: cosmosdb
        account: mycosmosaccount
        format: connection_string
```

## Migration Guide

### From Service Principal

```yaml
# Before: Service principal with credentials
secretStores:
  azure:
    type: azure.keyvault
    vault_name: mykeyvault
    auth:
      client_id: ${AZURE_CLIENT_ID}
      client_secret: ${AZURE_CLIENT_SECRET}
      tenant_id: ${AZURE_TENANT_ID}

# After: Managed identity (no credentials)
secretStores:
  azure:
    type: azure.managed_identity
    # No auth section needed!
```

### From Connection Strings

```yaml
# Before: Hardcoded connection strings
DATABASE_CONNECTION: "Server=tcp:myserver.database.windows.net;Authentication=Active Directory Password;User ID=user;Password=pass"

# After: Managed identity connection
DATABASE_CONNECTION:
  from:
    store: azure
    resource_type: sql
    server: myserver.database.windows.net
    database: mydb
    format: connection_string
    # Automatically uses: Authentication=Active Directory Managed Identity
```

## Best Practices Summary

1. **Prefer System-Assigned**: Simpler lifecycle management
2. **Use RBAC**: Azure RBAC over Key Vault access policies
3. **Minimal Permissions**: Grant only required access
4. **Enable Logging**: Audit all identity usage
5. **Cache Tokens**: Reduce metadata service calls
6. **Handle Fallbacks**: Plan for local development
7. **Document Access**: Track which identity accesses what

## Related Documentation

- [Azure Managed Identity Docs](https://docs.microsoft.com/en-us/azure/active-directory/managed-identities-azure-resources/)
- [Azure Key Vault Provider](/providers/azure-key-vault/)
- [Azure Unified Provider](/providers/azure-unified/)
- [Security Best Practices](/security/)
- [Platform Setup Guide](/guides/platforms/azure/)