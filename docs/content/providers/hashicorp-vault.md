---
title: "HashiCorp Vault"
description: "Secure secret management with dynamic secrets and encryption"
lead: "HashiCorp Vault provides secure access to tokens, passwords, certificates, and encryption keys. It supports dynamic secrets, detailed audit logs, and fine-grained access control policies."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 510
---

## Overview

HashiCorp Vault is an enterprise-grade secret management platform offering:

- **Dynamic Secrets**: Generate secrets on-demand with automatic expiration
- **Encryption as a Service**: Encrypt data without storing encryption keys
- **Identity-Based Access**: Rich authentication and authorization policies  
- **Secret Versioning**: Track and manage secret changes over time
- **Audit Logging**: Complete audit trail of all operations
- **Multi-Cloud Support**: Works across all major cloud providers

## Prerequisites

1. **Vault Server**: Running Vault server (cloud or self-hosted)
2. **Network Access**: Connectivity to Vault server
3. **Authentication**: Valid Vault token or auth method credentials
4. **Policies**: Appropriate Vault policies for secret access

### Vault CLI (Optional)

```bash
# Install Vault CLI
# macOS
brew install vault

# Linux
wget https://releases.hashicorp.com/vault/1.15.0/vault_1.15.0_linux_amd64.zip
unzip vault_1.15.0_linux_amd64.zip
sudo mv vault /usr/local/bin/

# Windows
winget install HashiCorp.Vault
```

## Configuration

### Basic Configuration

```yaml
version: 1

secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth_method: token
    token: ${VAULT_TOKEN}

envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: vault
        path: secret/data/database
        key: password
```

### Advanced Configuration

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    
    # Authentication configuration
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
      mount_path: auth/approle  # Optional: custom mount path
    
    # Optional: Namespace (Vault Enterprise)
    namespace: myteam
    
    # Optional: TLS configuration
    tls:
      ca_cert: /path/to/ca.pem
      client_cert: /path/to/client.pem
      client_key: /path/to/client-key.pem
      insecure_skip_verify: false
    
    # Optional: Timeout settings
    timeout: 30s
    
    # Optional: Retry configuration
    retry:
      max_attempts: 3
      backoff: exponential
```

## Authentication Methods

### 1. Token Authentication

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: token
      token: ${VAULT_TOKEN}
```

### 2. AppRole Authentication

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
      
      # Optional: Custom mount path
      mount_path: auth/my-approle
```

### 3. Kubernetes Authentication

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: kubernetes
      role: my-vault-role
      
      # Optional: Custom service account token path
      token_path: /var/run/secrets/kubernetes.io/serviceaccount/token
      
      # Optional: Custom mount path  
      mount_path: auth/kubernetes
```

### 4. AWS IAM Authentication

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: aws
      role: my-vault-role
      
      # Optional: AWS credentials
      access_key: ${AWS_ACCESS_KEY_ID}
      secret_key: ${AWS_SECRET_ACCESS_KEY}
      session_token: ${AWS_SESSION_TOKEN}
      
      # Optional: Custom mount path
      mount_path: auth/aws
```

### 5. Azure Authentication

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: azure
      role: my-vault-role
      
      # Optional: Explicit resource/subscription
      resource: https://management.azure.com/
      subscription_id: ${AZURE_SUBSCRIPTION_ID}
```

### 6. GCP Authentication

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: gcp
      role: my-vault-role
      
      # Optional: Service account credentials
      credentials: /path/to/service-account.json
```

### 7. LDAP Authentication

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: ldap
      username: ${LDAP_USERNAME}
      password: ${LDAP_PASSWORD}
```

## Secret Engines

### Key-Value v2 (Versioned)

```yaml
# Default KV v2 path
DATABASE_PASSWORD:
  from:
    store: vault
    path: secret/data/database
    key: password

# Specific version
DATABASE_PASSWORD:
  from:
    store: vault
    path: secret/data/database
    key: password
    version: 2

# Latest version explicitly
DATABASE_PASSWORD:
  from:
    store: vault
    path: secret/data/database
    key: password
    version: latest
```

### Key-Value v1 (Unversioned)

```yaml
# KV v1 path (no /data/ segment)
API_KEY:
  from:
    store: vault
    path: kv/api-keys
    key: stripe_key
```

### Dynamic Secrets - Database

```yaml
# PostgreSQL dynamic credentials
DB_CREDENTIALS:
  from:
    store: vault
    path: database/creds/readonly
    # Returns JSON with username/password
    ttl: 3600  # 1 hour lease

# Extract specific fields
DB_USERNAME:
  from:
    store: vault
    path: database/creds/readwrite
  transform:
    - type: json_extract
      path: .username

DB_PASSWORD:
  from:
    store: vault
    path: database/creds/readwrite
  transform:
    - type: json_extract
      path: .password
```

### Dynamic Secrets - AWS

```yaml
# AWS temporary credentials
AWS_CREDS:
  from:
    store: vault
    path: aws/creds/s3-readonly
    ttl: 1800  # 30 minutes

# Individual fields
AWS_ACCESS_KEY_ID:
  from:
    store: vault
    path: aws/creds/ec2-admin
  transform:
    - type: json_extract
      path: .access_key

AWS_SECRET_ACCESS_KEY:
  from:
    store: vault
    path: aws/creds/ec2-admin
  transform:
    - type: json_extract
      path: .secret_key
```

### PKI (Certificates)

```yaml
# Generate certificate
TLS_CERT:
  from:
    store: vault
    path: pki/issue/web-server
    parameters:
      common_name: api.example.com
      alt_names: api.internal.example.com,localhost
      ttl: 720h  # 30 days

# Extract certificate components
CERT_PEM:
  from:
    store: vault
    path: pki/issue/web-server
    parameters:
      common_name: api.example.com
  transform:
    - type: json_extract
      path: .certificate

PRIVATE_KEY:
  from:
    store: vault
    path: pki/issue/web-server
    parameters:
      common_name: api.example.com
  transform:
    - type: json_extract
      path: .private_key
```

### Transit (Encryption as a Service)

```yaml
# Encrypt data
ENCRYPTED_DATA:
  from:
    store: vault
    path: transit/encrypt/my-key
    parameters:
      plaintext: ${DATA_TO_ENCRYPT}
  transform:
    - type: json_extract
      path: .ciphertext

# Decrypt data
DECRYPTED_DATA:
  from:
    store: vault
    path: transit/decrypt/my-key
    parameters:
      ciphertext: ${ENCRYPTED_DATA}
  transform:
    - type: json_extract
      path: .plaintext
    - type: base64_decode
```

## Advanced Features

### Token Management

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
    
    # Token management
    token:
      # Renew token before expiry
      auto_renew: true
      renew_buffer: 300  # 5 minutes before expiry
      
      # Wrap token responses
      wrap_ttl: 300
```

### Response Wrapping

```yaml
# Get wrapped secret
WRAPPED_SECRET:
  from:
    store: vault
    path: secret/data/api-key
    wrap_ttl: 300  # 5 minutes
    # Returns wrapping token

# Unwrap later
API_KEY:
  from:
    store: vault
    path: sys/wrapping/unwrap
    token: ${WRAPPED_SECRET.wrap_info.token}
  transform:
    - type: json_extract
      path: .data.data.api_key
```

### Batch Requests

```yaml
# Efficient batch read
envs:
  production:
    _batch:
      app_secrets:
        store: vault
        requests:
          - path: secret/data/database
          - path: secret/data/api-keys  
          - path: database/creds/readonly
    
    # Extract from batch
    DB_PASSWORD:
      from:
        batch: app_secrets
        index: 0
      transform:
        - type: json_extract
          path: .data.data.password
    
    API_KEY:
      from:
        batch: app_secrets
        index: 1
      transform:
        - type: json_extract
          path: .data.data.stripe
```

### Multi-Mount Support

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

envs:
  production:
    # Different secret engines
    STATIC_SECRET:
      from:
        store: vault
        path: secret/data/app-config  # KV v2
        key: database_url
    
    DYNAMIC_CREDS:
      from:
        store: vault
        path: database/creds/app-role  # Database engine
    
    CERTIFICATE:
      from:
        store: vault
        path: pki/issue/server  # PKI engine
        parameters:
          common_name: app.example.com
```

## Security Best Practices

### 1. Use Appropriate Auth Methods

```yaml
# Production: AppRole with restricted policies
secretStores:
  vault-prod:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

# Development: Token for simplicity
secretStores:
  vault-dev:
    type: hashicorp.vault
    url: https://vault-dev.example.com
    auth:
      method: token
      token: ${VAULT_DEV_TOKEN}
```

### 2. Implement Least Privilege Policies

```hcl
# Vault policy example
path "secret/data/myapp/*" {
  capabilities = ["read"]
}

path "database/creds/readonly" {
  capabilities = ["read"]
}

path "transit/encrypt/myapp-key" {
  capabilities = ["update"]
}

path "transit/decrypt/myapp-key" {
  capabilities = ["update"]
}
```

### 3. Use Short-Lived Secrets

```yaml
# Prefer dynamic secrets with short TTLs
DB_CREDS:
  from:
    store: vault
    path: database/creds/app-role
    ttl: 1800  # 30 minutes

# Renew before expiry
TOKEN:
  from:
    store: vault
    path: auth/token/renew-self
    auto_renew: true
```

### 4. Enable Audit Logging

```bash
# Enable file audit device
vault audit enable file file_path=/vault/logs/audit.log

# Enable syslog audit device  
vault audit enable syslog tag="vault" facility="local0"
```

## Performance Optimization

### Caching Configuration

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
    
    cache:
      enabled: true
      
      # Cache static secrets longer
      kv_ttl: 3600  # 1 hour
      
      # Don't cache dynamic secrets
      dynamic_ttl: 0  # No cache
      
      # Cache policies
      rules:
        - path: "secret/data/static/*"
          ttl: 7200  # 2 hours
        
        - path: "database/creds/*"
          ttl: 0  # Never cache dynamic
```

### Connection Pooling

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
    
    connection:
      pool_size: 10
      idle_timeout: 300
      keep_alive: true
```

## High Availability

### Multiple Vault Servers

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    urls:
      - https://vault-1.example.com
      - https://vault-2.example.com
      - https://vault-3.example.com
    
    # Load balancing
    load_balancer:
      method: round_robin
      health_check: true
      timeout: 5s
    
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
```

### Disaster Recovery

```yaml
secretStores:
  vault-primary:
    type: hashicorp.vault
    url: https://vault-primary.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
  
  vault-dr:
    type: hashicorp.vault
    url: https://vault-dr.example.com
    auth:
      method: approle
      role_id: ${VAULT_DR_ROLE_ID}
      secret_id: ${VAULT_DR_SECRET_ID}

# Fallback configuration
envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: vault-primary
        path: secret/data/database
        key: password
      fallback:
        from:
          store: vault-dr
          path: secret/data/database
          key: password
```

## Rotation Integration

### Database Rotation

```yaml
services:
  postgres-vault:
    type: postgresql
    host: db.example.com
    vault_integration:
      enabled: true
      mount: database
      role: app-role

envs:
  production:
    DATABASE_CREDENTIALS:
      from:
        store: vault
        path: database/creds/app-role
      service: postgres-vault
      rotation:
        strategy: dynamic  # Vault handles rotation
        lease_duration: 3600
```

### Static Secret Rotation

```yaml
services:
  api-service:
    type: custom
    api_endpoint: https://api.example.com

envs:
  production:
    API_KEY:
      from:
        store: vault
        path: secret/data/api-keys
        key: production
      service: api-service
      rotation:
        strategy: immediate
        schedule: "0 2 * * SUN"
```

## Troubleshooting

### Authentication Issues

```bash
# Test Vault connection
vault status

# Test authentication
vault auth -method=approle role_id=${VAULT_ROLE_ID} secret_id=${VAULT_SECRET_ID}

# Check token capabilities
vault token capabilities secret/data/myapp/config
```

### Permission Errors

```bash
# Check current token policies
vault token lookup

# Test path access
vault kv get secret/myapp/config

# Check policy details
vault policy read my-policy
```

### Debug Configuration

```yaml
secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}
    
    debug:
      enabled: true
      log_requests: true
      log_responses: false  # Don't log secrets
      trace_auth: true
```

### Common Error Messages

| Error | Cause | Solution |
|-------|-------|----------|
| "permission denied" | Insufficient policy | Update Vault policy |
| "invalid role_id" | Wrong AppRole credentials | Check role_id/secret_id |
| "connection refused" | Network/firewall issue | Check connectivity |
| "token expired" | Token needs renewal | Enable auto_renew |

## Migration Guide

### From Environment Variables

```bash
# Migrate .env to Vault
while IFS='=' read -r key value; do
  vault kv put secret/myapp/$key value="$value"
done < .env
```

### From Other Secret Stores

```yaml
# Gradual migration
secretStores:
  # Old provider
  aws:
    type: aws.secretsmanager
    region: us-east-1
  
  # New Vault provider
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth:
      method: approle
      role_id: ${VAULT_ROLE_ID}
      secret_id: ${VAULT_SECRET_ID}

envs:
  production:
    # Migrate one secret at a time
    OLD_SECRET:
      from:
        store: aws
        name: api-key
    
    NEW_SECRET:
      from:
        store: vault
        path: secret/data/api-keys
        key: production
```

## Best Practices Summary

1. **Use AppRole**: Preferred auth method for applications
2. **Implement Least Privilege**: Minimal required policies
3. **Prefer Dynamic Secrets**: When supported by target service
4. **Enable Auto-Renewal**: For long-running applications
5. **Monitor Audit Logs**: Track all secret access
6. **Use Namespaces**: Organize secrets in Vault Enterprise
7. **Plan for HA**: Multiple Vault servers for production

## Related Documentation

- [HashiCorp Vault Docs](https://www.vaultproject.io/docs)
- [Vault Auth Methods](https://www.vaultproject.io/docs/auth)
- [Vault Secret Engines](https://www.vaultproject.io/docs/secrets)
- [Security Best Practices](/security/)
- [Rotation Guide](/rotation/)