---
title: "Infisical"
description: "Use Infisical open-source secret management with dsops"
lead: "Integrate with Infisical, an open-source secret management platform with end-to-end encryption."
date: 2025-01-04T12:00:00-07:00
lastmod: 2025-01-04T12:00:00-07:00
draft: false
weight: 17
---

## Overview

[Infisical](https://infisical.com/) is an open-source secret management platform designed for developers and DevOps teams. It provides end-to-end encryption, audit logs, and integrates well with modern development workflows.

## Features

- **End-to-End Encryption**: Zero-knowledge architecture ensures secrets are encrypted client-side
- **Open Source**: Self-host or use the managed cloud service
- **Project Organization**: Organize secrets by projects and environments
- **Secret Versioning**: Full history of secret changes
- **Audit Logging**: Track who accessed or modified secrets
- **Integrations**: Native integrations with CI/CD, Kubernetes, and more

## Prerequisites

1. **Infisical Account**: Cloud (infisical.com) or self-hosted instance
2. **Project**: At least one project with secrets
3. **Authentication**: Machine Identity, Service Token, or API Key

## Configuration

Add Infisical to your `dsops.yaml`:

```yaml
version: 1

secretStores:
  infisical:
    type: infisical
    project_id: "64f7e4d2-1234-5678-abcd-1234567890ab"
    environment: "dev"  # dev, staging, prod
    auth:
      method: machine_identity
      client_id: "${INFISICAL_CLIENT_ID}"
      client_secret: "${INFISICAL_CLIENT_SECRET}"

envs:
  development:
    DATABASE_URL:
      from:
        store: infisical/DATABASE_URL

    API_KEY:
      from:
        store: infisical/services/API_KEY
```

### Configuration Options

| Option | Required | Description |
|--------|----------|-------------|
| `project_id` | Yes | Infisical project identifier |
| `environment` | Yes | Environment slug (dev, staging, prod) |
| `host` | No | Instance URL (default: https://app.infisical.com) |
| `auth.method` | Yes | Authentication method |
| `auth.client_id` | For machine_identity | Machine Identity client ID |
| `auth.client_secret` | For machine_identity | Machine Identity client secret |
| `auth.token` | For service_token | Service Token value |

## Authentication Methods

### Machine Identity (Recommended)

Machine Identities provide fine-grained access control:

```yaml
secretStores:
  infisical:
    type: infisical
    project_id: "your-project-id"
    environment: "production"
    auth:
      method: machine_identity
      client_id: "${INFISICAL_CLIENT_ID}"
      client_secret: "${INFISICAL_CLIENT_SECRET}"
```

**Setup:**
1. Go to **Project Settings** > **Machine Identities**
2. Create a new Machine Identity
3. Grant access to your project with appropriate role
4. Copy the Client ID and Client Secret

### Service Token

Service Tokens are simpler but less flexible:

```yaml
secretStores:
  infisical:
    type: infisical
    project_id: "your-project-id"
    environment: "staging"
    auth:
      method: service_token
      token: "${INFISICAL_SERVICE_TOKEN}"
```

**Setup:**
1. Go to **Project Settings** > **Service Tokens**
2. Create a token for the desired environment
3. Set the token in your environment

### API Key (Development Only)

For local development:

```yaml
secretStores:
  infisical:
    type: infisical
    project_id: "your-project-id"
    environment: "dev"
    auth:
      method: api_key
      api_key: "${INFISICAL_API_KEY}"
```

**Note**: API Keys have broad access. Use Machine Identities for production.

## Key Format

```
infisical/SECRET_NAME
infisical/folder/SECRET_NAME
infisical/folder/SECRET_NAME@v2
```

- **SECRET_NAME**: The secret's key name
- **folder**: Optional folder path within the project
- **@vN**: Optional version specifier

## Usage Examples

### Basic Usage

```yaml
version: 1

secretStores:
  infisical:
    type: infisical
    project_id: "64f7e4d2-abc123"
    environment: "dev"
    auth:
      method: machine_identity
      client_id: "${INFISICAL_CLIENT_ID}"
      client_secret: "${INFISICAL_CLIENT_SECRET}"

envs:
  development:
    # Simple secret retrieval
    DATABASE_URL:
      from:
        store: infisical/DATABASE_URL

    # Secret in a folder
    STRIPE_KEY:
      from:
        store: infisical/payments/STRIPE_SECRET_KEY

    # Specific version
    ENCRYPTION_KEY:
      from:
        store: infisical/crypto/ENCRYPTION_KEY@v2
```

### Multiple Environments

```yaml
secretStores:
  infisical-dev:
    type: infisical
    project_id: "my-project"
    environment: "dev"
    auth:
      method: machine_identity
      client_id: "${INFISICAL_DEV_CLIENT_ID}"
      client_secret: "${INFISICAL_DEV_CLIENT_SECRET}"

  infisical-prod:
    type: infisical
    project_id: "my-project"
    environment: "prod"
    auth:
      method: machine_identity
      client_id: "${INFISICAL_PROD_CLIENT_ID}"
      client_secret: "${INFISICAL_PROD_CLIENT_SECRET}"

envs:
  development:
    DATABASE_URL:
      from:
        store: infisical-dev/DATABASE_URL

  production:
    DATABASE_URL:
      from:
        store: infisical-prod/DATABASE_URL
```

### Self-Hosted Instance

```yaml
secretStores:
  infisical:
    type: infisical
    host: "https://secrets.mycompany.com"
    project_id: "internal-project"
    environment: "prod"
    auth:
      method: machine_identity
      client_id: "${INFISICAL_CLIENT_ID}"
      client_secret: "${INFISICAL_CLIENT_SECRET}"
```

## Setting Up Infisical

### Cloud Setup

1. Sign up at [infisical.com](https://app.infisical.com)
2. Create a new project
3. Add secrets via the dashboard or CLI
4. Create a Machine Identity for dsops access

### Self-Hosted Setup

{{< tabs >}}
{{< tab "Docker" >}}
```bash
# Quick start with Docker Compose
git clone https://github.com/Infisical/infisical.git
cd infisical
docker-compose up -d
```
{{< /tab >}}
{{< tab "Kubernetes" >}}
```bash
# Using Helm
helm repo add infisical https://helm.infisical.com
helm install infisical infisical/infisical
```
{{< /tab >}}
{{< /tabs >}}

### CLI Setup

```bash
# Install Infisical CLI
npm install -g @infisical/cli

# Login
infisical login

# Initialize in your project
infisical init

# Add secrets
infisical secrets set DATABASE_URL="postgres://..."

# List secrets
infisical secrets list
```

## Secret Organization

### Folder Structure

Organize secrets using folders:

```
Project Root
├── DATABASE_URL
├── API_KEY
├── payments/
│   ├── STRIPE_KEY
│   └── PAYPAL_CLIENT_ID
├── auth/
│   ├── JWT_SECRET
│   └── SESSION_KEY
└── integrations/
    ├── SLACK_TOKEN
    └── GITHUB_TOKEN
```

### Environment Slugs

Use consistent environment naming:

| Slug | Purpose |
|------|---------|
| `dev` | Local development |
| `staging` | Pre-production testing |
| `prod` | Production |
| `test` | CI/CD testing |

## Security Best Practices

### 1. Use Machine Identities

Machine Identities provide:
- Granular access control
- Audit logging
- Easy rotation

### 2. Least Privilege

Grant minimum required permissions:
- Read-only access for most use cases
- Limit to specific folders when possible

### 3. Rotate Credentials

Regularly rotate:
- Machine Identity secrets
- Service Tokens
- Any compromised secrets

### 4. Environment Separation

Use different credentials per environment:

```yaml
secretStores:
  dev:
    type: infisical
    environment: "dev"
    auth:
      client_id: "${DEV_CLIENT_ID}"
      client_secret: "${DEV_CLIENT_SECRET}"

  prod:
    type: infisical
    environment: "prod"
    auth:
      client_id: "${PROD_CLIENT_ID}"
      client_secret: "${PROD_CLIENT_SECRET}"
```

## Troubleshooting

### Common Errors

| Error | Cause | Solution |
|-------|-------|----------|
| `unauthorized` | Invalid credentials | Check client_id/client_secret |
| `project not found` | Invalid project_id | Verify project ID in dashboard |
| `environment not found` | Invalid environment | Check environment slug |
| `secret not found` | Secret doesn't exist | Add secret in Infisical |
| `connection refused` | Host unreachable | Check host URL and network |

### Debug Mode

Enable verbose logging:

```bash
dsops doctor --verbose
```

### Verify Configuration

```bash
# Test authentication
curl -X POST https://app.infisical.com/api/v1/auth/universal-auth/login \
  -H "Content-Type: application/json" \
  -d '{"clientId": "your-client-id", "clientSecret": "your-client-secret"}'
```

## Integration with CI/CD

### GitHub Actions

```yaml
jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Run with secrets
        env:
          INFISICAL_CLIENT_ID: ${{ secrets.INFISICAL_CLIENT_ID }}
          INFISICAL_CLIENT_SECRET: ${{ secrets.INFISICAL_CLIENT_SECRET }}
        run: |
          dsops exec --env production -- ./deploy.sh
```

### GitLab CI

```yaml
deploy:
  script:
    - dsops exec --env production -- ./deploy.sh
  variables:
    INFISICAL_CLIENT_ID: $INFISICAL_CLIENT_ID
    INFISICAL_CLIENT_SECRET: $INFISICAL_CLIENT_SECRET
```

## Related Documentation

- [Infisical Documentation](https://infisical.com/docs/documentation)
- [Machine Identities](https://infisical.com/docs/documentation/platform/identities)
- [Self-Hosting Guide](https://infisical.com/docs/self-hosting/overview)
- [API Reference](https://infisical.com/docs/api-reference)
