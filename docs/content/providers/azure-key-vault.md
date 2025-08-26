---
title: "Azure Key Vault"
description: "Microsoft's cloud-based secret management service"
lead: "Azure Key Vault safeguards cryptographic keys, secrets, and certificates used by cloud applications and services. It provides centralized secret management with fine-grained access control and comprehensive audit logging."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 410
---

## Overview

Azure Key Vault is Microsoft's native secret management solution offering:

- **Centralized Secret Storage**: Store application secrets, keys, and certificates
- **Hardware Security Modules**: FIPS 140-2 Level 2 validated HSMs
- **Access Control**: Azure AD-based authentication and authorization
- **Audit Trail**: Complete logging of all vault operations
- **Compliance**: Meets various regulatory requirements
- **Global Availability**: Available in all Azure regions

## Prerequisites

Before using the Azure Key Vault provider:

1. **Azure CLI Installation**:
   ```bash
   # macOS
   brew install azure-cli
   
   # Linux
   curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
   
   # Windows
   winget install Microsoft.AzureCLI
   ```

2. **Authentication**:
   ```bash
   # Interactive login
   az login
   
   # Service principal login
   az login --service-principal -u CLIENT_ID -p CLIENT_SECRET --tenant TENANT_ID
   ```

3. **Key Vault Access Policy**:
   - `get` permission for secrets
   - `list` permission for browsing (optional)

## Configuration

### Basic Configuration

```yaml
version: 1

secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    # Uses default Azure credential chain

envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: azure
        name: database-password
```

### Advanced Configuration

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    
    # Optional: Override default credentials
    auth:
      method: service_principal
      client_id: ${AZURE_CLIENT_ID}
      client_secret: ${AZURE_CLIENT_SECRET}
      tenant_id: ${AZURE_TENANT_ID}
    
    # Optional: Custom vault URL (for non-public clouds)
    vault_url: https://my-keyvault.vault.azure.net/
    
    # Optional: API version
    api_version: "7.4"
    
    # Optional: Performance settings
    cache:
      enabled: true
      ttl: 300  # 5 minutes
```

## Authentication Methods

### 1. Azure CLI (Default)

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    # No auth field - uses az login credentials
```

### 2. Managed Identity

```yaml
# For Azure VMs, App Service, etc.
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    auth:
      method: managed_identity
      # Optional: User-assigned identity
      client_id: ${MANAGED_IDENTITY_CLIENT_ID}
```

### 3. Service Principal

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    auth:
      method: service_principal
      client_id: ${AZURE_CLIENT_ID}
      client_secret: ${AZURE_CLIENT_SECRET}
      tenant_id: ${AZURE_TENANT_ID}
```

### 4. Certificate Authentication

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    auth:
      method: certificate
      client_id: ${AZURE_CLIENT_ID}
      certificate_path: /path/to/cert.pfx
      certificate_password: ${CERT_PASSWORD}
      tenant_id: ${AZURE_TENANT_ID}
```

### 5. Workload Identity (AKS)

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    auth:
      method: workload_identity
      # Automatically uses federated credentials
```

## Secret Management

### Basic Secret Reference

```yaml
# Latest version
DATABASE_PASSWORD:
  from:
    store: azure
    name: database-password

# Specific version
DATABASE_PASSWORD:
  from:
    store: azure
    name: database-password
    version: a1b2c3d4e5f6

# Tags-based selection
API_KEY:
  from:
    store: azure
    name: api-key
    tags:
      environment: production
      service: api
```

### Secret Types

```yaml
# String secret
PASSWORD:
  from:
    store: azure
    name: app-password

# Certificate
TLS_CERT:
  from:
    store: azure
    name: app-certificate
    type: certificate

# Key (for encryption)
ENCRYPTION_KEY:
  from:
    store: azure
    name: data-encryption-key
    type: key
```

### JSON Secrets

```yaml
# Full JSON secret
CONFIG:
  from:
    store: azure
    name: app-config

# Extract specific field
DATABASE_HOST:
  from:
    store: azure
    name: database-config
  transform:
    - type: json_extract
      path: .host
```

## Advanced Features

### Multi-Vault Support

```yaml
secretStores:
  azure-prod:
    type: azure.keyvault
    vault_name: prod-keyvault
  
  azure-shared:
    type: azure.keyvault
    vault_name: shared-keyvault
    subscription_id: different-subscription-id

envs:
  production:
    # From production vault
    APP_SECRET:
      from:
        store: azure-prod
        name: app-secret
    
    # From shared vault
    SHARED_API_KEY:
      from:
        store: azure-shared
        name: shared-api-key
```

### Key Vault References

```yaml
# Use Key Vault references in App Service
APP_SERVICE_SECRET:
  from:
    store: azure
    name: app-secret
    reference_format: true
  # Outputs: @Microsoft.KeyVault(VaultName=my-keyvault;SecretName=app-secret)
```

### Certificate Operations

```yaml
# Get full certificate with private key
FULL_CERT:
  from:
    store: azure
    name: app-cert
    type: certificate
    include_private_key: true

# Get only public certificate
PUBLIC_CERT:
  from:
    store: azure
    name: app-cert
    type: certificate
    format: pem

# Get certificate thumbprint
CERT_THUMBPRINT:
  from:
    store: azure
    name: app-cert
    type: certificate
  transform:
    - type: json_extract
      path: .x5t
```

### Encryption Operations

```yaml
# Use Key Vault key for encryption
ENCRYPTED_DATA:
  from:
    store: azure
    name: encryption-key
    type: key
    operation: encrypt
    plaintext: ${DATA_TO_ENCRYPT}

# Decrypt data
DECRYPTED_DATA:
  from:
    store: azure
    name: encryption-key
    type: key
    operation: decrypt
    ciphertext: ${ENCRYPTED_DATA}

# Sign data
SIGNATURE:
  from:
    store: azure
    name: signing-key
    type: key
    operation: sign
    algorithm: RS256
    data: ${DATA_TO_SIGN}
```

## Security Best Practices

### 1. Access Policies

```bash
# Grant minimal permissions
az keyvault set-policy \
  --name my-keyvault \
  --spn $AZURE_CLIENT_ID \
  --secret-permissions get list

# Use Azure RBAC (recommended)
az role assignment create \
  --role "Key Vault Secrets User" \
  --assignee $AZURE_CLIENT_ID \
  --scope /subscriptions/SUB_ID/resourceGroups/RG/providers/Microsoft.KeyVault/vaults/my-keyvault
```

### 2. Network Security

```yaml
# Configure network restrictions
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    network:
      # Private endpoint
      use_private_endpoint: true
      
      # IP restrictions
      allowed_ips:
        - 10.0.0.0/8
        - 172.16.0.0/12
```

### 3. Audit Logging

```bash
# Enable diagnostic logging
az monitor diagnostic-settings create \
  --name keyvault-logs \
  --resource /subscriptions/SUB_ID/resourceGroups/RG/providers/Microsoft.KeyVault/vaults/my-keyvault \
  --logs '[{"category": "AuditEvent", "enabled": true}]' \
  --workspace LOG_ANALYTICS_WORKSPACE_ID
```

### 4. Soft Delete Protection

```bash
# Enable soft delete and purge protection
az keyvault create \
  --name my-keyvault \
  --resource-group my-rg \
  --enable-soft-delete true \
  --enable-purge-protection true \
  --retention-days 90
```

## Performance Optimization

### Caching Configuration

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    cache:
      enabled: true
      ttl: 600  # 10 minutes
      max_size: 1000
      
      # Cache by pattern
      rules:
        - pattern: "static-*"
          ttl: 3600  # 1 hour
        - pattern: "dynamic-*"
          ttl: 60    # 1 minute
```

### Batch Operations

```yaml
# Fetch multiple secrets efficiently
envs:
  production:
    # Define batch
    _batch:
      db_config:
        store: azure
        prefix: database-
    
    # Extract values
    DB_HOST:
      from:
        batch: db_config
        name: database-host
    
    DB_PORT:
      from:
        batch: db_config
        name: database-port
    
    DB_PASSWORD:
      from:
        batch: db_config
        name: database-password
```

### Connection Pooling

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    connection:
      pool_size: 10
      timeout: 30s
      retry:
        max_attempts: 3
        backoff: exponential
```

## Rotation Support

### Automatic Rotation

```yaml
services:
  sql-server:
    type: azure.sql
    server: myserver.database.windows.net
    database: mydb

envs:
  production:
    SQL_PASSWORD:
      from:
        store: azure
        name: sql-password
      service: sql-server
      rotation:
        strategy: two-key
        schedule: "0 2 * * SUN"  # Weekly
```

### Manual Rotation

```bash
# Rotate a secret
dsops secrets rotate --env production --key SQL_PASSWORD

# View rotation history
dsops rotation history sql-server
```

## Troubleshooting

### Authentication Failures

```bash
# Check current Azure login
az account show

# List available subscriptions
az account list

# Switch subscription
az account set --subscription SUBSCRIPTION_ID

# Test Key Vault access
az keyvault secret list --vault-name my-keyvault
```

### Permission Errors

```bash
# Check current permissions
az keyvault show --name my-keyvault --query "properties.accessPolicies[?objectId=='OBJECT_ID']"

# Check RBAC assignments
az role assignment list --assignee $AZURE_CLIENT_ID --scope /subscriptions/SUB_ID/resourceGroups/RG/providers/Microsoft.KeyVault/vaults/my-keyvault
```

### Network Issues

```bash
# Check network rules
az keyvault network-rule list --name my-keyvault

# Test connectivity
nslookup my-keyvault.vault.azure.net
curl -I https://my-keyvault.vault.azure.net/
```

### Debug Mode

```yaml
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    debug:
      enabled: true
      log_requests: true
      log_auth: true
```

## Cost Optimization

### 1. Use Caching

```yaml
# Reduce API calls with caching
secretStores:
  azure:
    type: azure.keyvault
    vault_name: my-keyvault
    cache:
      enabled: true
      ttl: 1800  # 30 minutes for stable secrets
```

### 2. Batch Secrets

```yaml
# Store related values together
APP_CONFIG:
  from:
    store: azure
    name: app-config  # JSON with multiple values

# Extract as needed
API_KEY:
  from:
    store: azure
    name: app-config
  transform:
    - type: json_extract
      path: .api_key
```

### 3. Regional Deployment

```bash
# Deploy Key Vault in same region as applications
az keyvault create \
  --name my-keyvault \
  --resource-group my-rg \
  --location eastus  # Same as app region
```

## Migration Guide

### From Azure App Configuration

```yaml
# Gradual migration
secretStores:
  azure-config:
    type: azure.appconfiguration
    endpoint: https://myconfig.azconfig.io
  
  azure-vault:
    type: azure.keyvault
    vault_name: my-keyvault

envs:
  production:
    # Non-sensitive from App Configuration
    FEATURE_FLAGS:
      from:
        store: azure-config
        key: features
    
    # Sensitive from Key Vault
    API_KEY:
      from:
        store: azure-vault
        name: api-key
```

### From Environment Variables

```bash
#!/bin/bash
# Migrate .env to Key Vault

while IFS='=' read -r key value; do
  az keyvault secret set \
    --vault-name my-keyvault \
    --name "$key" \
    --value "$value" \
    --tags source=env migration=v1
done < .env
```

## Integration Examples

### Azure DevOps Pipeline

```yaml
# azure-pipelines.yml
variables:
  - group: keyvault-secrets  # Linked to Key Vault

steps:
  - script: |
      dsops exec --env production -- npm run deploy
    env:
      AZURE_CLIENT_ID: $(ClientId)
      AZURE_CLIENT_SECRET: $(ClientSecret)
      AZURE_TENANT_ID: $(TenantId)
```

### GitHub Actions

```yaml
# .github/workflows/deploy.yml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: azure/login@v1
        with:
          creds: ${{ secrets.AZURE_CREDENTIALS }}
      
      - run: |
          dsops exec --env production -- npm run deploy
```

## Best Practices Summary

1. **Use Managed Identity**: When running in Azure, prefer managed identity
2. **Enable RBAC**: Use Azure RBAC instead of access policies
3. **Network Isolation**: Use private endpoints and firewall rules
4. **Audit Everything**: Enable diagnostic logging
5. **Cache Wisely**: Balance performance with security
6. **Rotate Regularly**: Implement automated rotation
7. **Tag Resources**: Use tags for organization and compliance

## Related Documentation

- [Azure Key Vault Docs](https://docs.microsoft.com/en-us/azure/key-vault/)
- [Azure Unified Provider](/providers/azure-unified/)
- [Azure Managed Identity Provider](/providers/azure-managed-identity/)
- [Security Best Practices](/security/)
- [Rotation Guide](/rotation/)