---
title: "Akeyless"
description: "Use Akeyless enterprise secret management with dsops"
lead: "Integrate with Akeyless, an enterprise zero-knowledge secret management platform with FIPS 140-2 certification."
date: 2025-01-04T12:00:00-07:00
lastmod: 2025-01-04T12:00:00-07:00
draft: false
weight: 18
---

## Overview

[Akeyless](https://www.akeyless.io/) is an enterprise-grade secret management platform built on zero-knowledge architecture using patented Distributed Fragment Cryptography (DFC). It's designed for regulated industries requiring FIPS 140-2 compliance.

## Features

- **Zero-Knowledge**: Patented DFC ensures Akeyless never has access to your secrets
- **FIPS 140-2 Certified**: Enterprise-grade compliance
- **Multi-Cloud Authentication**: AWS IAM, Azure AD, GCP, OIDC, SAML, and more
- **Dynamic Secrets**: Generate credentials on-demand for databases, clouds, and services
- **Certificate Management**: PKI and SSH certificate automation
- **Unified Access**: Single platform for static, dynamic, and rotated secrets

## Prerequisites

1. **Akeyless Account**: Cloud or self-hosted gateway
2. **Access ID**: Authentication identity
3. **Authentication Method**: API Key, AWS IAM, Azure AD, or GCP

## Configuration

Add Akeyless to your `dsops.yaml`:

```yaml
version: 1

secretStores:
  akeyless:
    type: akeyless
    access_id: "p-1234567890abcdef"
    auth:
      method: api_key
      access_key: "${AKEYLESS_ACCESS_KEY}"

envs:
  development:
    DATABASE_PASSWORD:
      from:
        store: akeyless/Dev/Database/password

    API_KEY:
      from:
        store: akeyless/Dev/Services/api-key
```

### Configuration Options

| Option | Required | Description |
|--------|----------|-------------|
| `access_id` | Yes | Akeyless Access ID (p-xxx format) |
| `gateway_url` | No | Gateway URL (default: https://api.akeyless.io) |
| `auth.method` | Yes | Authentication method |
| `auth.access_key` | For api_key | API access key |
| `auth.azure_ad_object_id` | For azure_ad | Azure AD object ID |
| `auth.gcp_audience` | For gcp | GCP audience (default: akeyless.io) |

## Authentication Methods

### API Key

Simplest method for getting started:

```yaml
secretStores:
  akeyless:
    type: akeyless
    access_id: "p-1234567890abcdef"
    auth:
      method: api_key
      access_key: "${AKEYLESS_ACCESS_KEY}"
```

**Setup:**
1. Go to **Access Roles** in Akeyless Console
2. Create an API Key access role
3. Note the Access ID and Access Key

### AWS IAM (Recommended for AWS)

Uses AWS instance credentials or IAM roles:

```yaml
secretStores:
  akeyless:
    type: akeyless
    access_id: "p-aws-role-id"
    auth:
      method: aws_iam
```

**Setup:**
1. Create an AWS IAM Auth Method in Akeyless
2. Configure allowed AWS account IDs, roles, or instance IDs
3. Create an Access Role with the auth method

### Azure AD

Uses Azure Managed Identity or Service Principal:

```yaml
secretStores:
  akeyless:
    type: akeyless
    access_id: "p-azure-role-id"
    auth:
      method: azure_ad
      azure_ad_object_id: "12345678-1234-1234-1234-123456789012"
```

**Setup:**
1. Create an Azure AD Auth Method in Akeyless
2. Configure allowed Azure tenant and object IDs
3. Create an Access Role with the auth method

### GCP

Uses GCP Service Account credentials:

```yaml
secretStores:
  akeyless:
    type: akeyless
    access_id: "p-gcp-role-id"
    auth:
      method: gcp
      gcp_audience: "akeyless.io"
```

**Setup:**
1. Create a GCP Auth Method in Akeyless
2. Configure allowed GCP service accounts
3. Create an Access Role with the auth method

## Key Format

```
akeyless/path/to/secret
akeyless/path/to/secret@v2
```

- **path**: Hierarchical path to the secret (must start with /)
- **@vN**: Optional version specifier

## Usage Examples

### Basic Static Secrets

```yaml
version: 1

secretStores:
  akeyless:
    type: akeyless
    access_id: "p-1234567890"
    auth:
      method: api_key
      access_key: "${AKEYLESS_ACCESS_KEY}"

envs:
  development:
    # Simple secret path
    DATABASE_PASSWORD:
      from:
        store: akeyless/Dev/Database/password

    # Nested path
    STRIPE_KEY:
      from:
        store: akeyless/Dev/Payments/Stripe/secret-key

    # Specific version
    ENCRYPTION_KEY:
      from:
        store: akeyless/Dev/Crypto/encryption-key@v2
```

### Multi-Cloud Setup

```yaml
secretStores:
  # AWS environment using AWS IAM
  akeyless-aws:
    type: akeyless
    access_id: "p-aws-access-id"
    auth:
      method: aws_iam

  # Azure environment using Azure AD
  akeyless-azure:
    type: akeyless
    access_id: "p-azure-access-id"
    auth:
      method: azure_ad
      azure_ad_object_id: "${AZURE_OBJECT_ID}"

  # GCP environment
  akeyless-gcp:
    type: akeyless
    access_id: "p-gcp-access-id"
    auth:
      method: gcp

envs:
  aws-production:
    DATABASE_URL:
      from:
        store: akeyless-aws/Prod/Database/connection-string

  azure-production:
    DATABASE_URL:
      from:
        store: akeyless-azure/Prod/Database/connection-string

  gcp-production:
    DATABASE_URL:
      from:
        store: akeyless-gcp/Prod/Database/connection-string
```

### Custom Gateway

For self-hosted or private gateways:

```yaml
secretStores:
  akeyless:
    type: akeyless
    access_id: "p-private-id"
    gateway_url: "https://akeyless.mycompany.com"
    auth:
      method: api_key
      access_key: "${AKEYLESS_PRIVATE_KEY}"
```

## Secret Organization

### Recommended Path Structure

```
/
├── Dev/
│   ├── Database/
│   │   ├── password
│   │   └── connection-string
│   ├── Services/
│   │   ├── api-key
│   │   └── webhook-secret
│   └── Certs/
│       ├── tls-cert
│       └── tls-key
├── Staging/
│   └── ...
└── Prod/
    └── ...
```

### Naming Conventions

- Use PascalCase for folder names: `/Prod/Database`
- Use kebab-case for secret names: `api-key`, `connection-string`
- Include environment in path: `/Dev/`, `/Staging/`, `/Prod/`

## Dynamic Secrets

Akeyless supports generating credentials on-demand:

### AWS Dynamic Secrets

```yaml
envs:
  production:
    # Dynamic AWS credentials
    AWS_ACCESS_KEY_ID:
      from:
        store: akeyless/Dynamic/AWS/prod-account

    # The dynamic secret returns JSON with multiple fields
    # Use transforms to extract specific values
```

### Database Dynamic Secrets

```yaml
envs:
  production:
    DB_USERNAME:
      from:
        store: akeyless/Dynamic/PostgreSQL/prod
      transform: json_extract:.username

    DB_PASSWORD:
      from:
        store: akeyless/Dynamic/PostgreSQL/prod
      transform: json_extract:.password
```

## Security Best Practices

### 1. Use Cloud-Native Auth

Prefer AWS IAM, Azure AD, or GCP auth over API keys:
- No credentials to store
- Uses cloud provider's identity
- Automatic credential rotation

### 2. Least Privilege

Create specific access roles:
- Limit to required paths
- Use read-only permissions when possible
- Separate roles per environment

### 3. Path-Based Access Control

```
/Prod/*    -> Production access role only
/Dev/*     -> Development access role
/Shared/*  -> Cross-environment resources
```

### 4. Enable Audit Logging

Configure audit logs to:
- Track secret access
- Monitor authentication attempts
- Detect anomalies

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `authentication failed` | Invalid credentials | Check access_id and access_key |
| `access denied` | No permission | Verify access role permissions |
| `secret not found` | Path doesn't exist | Check secret path (must start with /) |
| `gateway unreachable` | Network issue | Verify gateway_url and connectivity |

### Debug Commands

```bash
# Check configuration
dsops doctor --verbose

# Test with Akeyless CLI
akeyless auth --access-id p-xxx --access-key xxx
akeyless get-secret-value --name /Dev/Database/password
```

### Verify Authentication

```bash
# Test API key auth
curl -X POST https://api.akeyless.io/auth \
  -H "Content-Type: application/json" \
  -d '{"access-id": "p-xxx", "access-key": "xxx"}'
```

## Gateway Configuration

### Cloud Gateway

Default gateway for Akeyless cloud:

```yaml
gateway_url: "https://api.akeyless.io"  # Default, can be omitted
```

### Customer Gateway

For private networks or compliance:

```yaml
gateway_url: "https://gateway.internal.company.com:8080"
```

### Gateway Features

- **Private Access**: Access secrets without internet exposure
- **High Availability**: Multiple gateway instances
- **Caching**: Improved performance for frequently accessed secrets
- **Compliance**: Keep traffic within your network

## Integration with CI/CD

### GitHub Actions with AWS IAM

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      id-token: write
    steps:
      - uses: aws-actions/configure-aws-credentials@v2
        with:
          role-to-assume: arn:aws:iam::123456789:role/deploy
          aws-region: us-east-1

      - name: Deploy with secrets
        run: dsops exec --env production -- ./deploy.sh
```

### Kubernetes with AWS IAM

```yaml
apiVersion: v1
kind: Pod
spec:
  serviceAccountName: akeyless-access
  containers:
    - name: app
      env:
        - name: AKEYLESS_ACCESS_ID
          value: "p-aws-role-id"
```

## Related Documentation

- [Akeyless Documentation](https://docs.akeyless.io/)
- [Authentication Methods](https://docs.akeyless.io/docs/access-and-authentication-methods)
- [Dynamic Secrets](https://docs.akeyless.io/docs/dynamic-secrets)
- [Gateway Setup](https://docs.akeyless.io/docs/gateway)
