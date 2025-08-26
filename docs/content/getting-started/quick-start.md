---
title: "Quick Start"
description: "Get dsops running in 5 minutes"
lead: "This quick start guide will have you pulling secrets and executing commands in just a few minutes."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 20
---

## 1. Install dsops

The quickest way to install dsops is with Homebrew:

```bash
brew install systmms/tap/dsops
```

Or download the latest binary from [GitHub Releases](https://github.com/systmms/dsops/releases).

## 2. Initialize Configuration

Create a sample configuration file:

```bash
dsops init
```

This creates a `dsops.yaml` file with examples for popular providers.

## 3. Configure Your Provider

Edit `dsops.yaml` to set up your secret provider. Here's an example with Bitwarden:

```yaml
version: 1

secretStores:
  bitwarden:
    type: bitwarden
    # Optional: specify account email
    email: you@example.com

envs:
  development:
    DATABASE_URL:
      from:
        store: bitwarden
        key: "Database/PostgreSQL"
    
    # You can mix providers
    AWS_SECRET:
      from:
        store: aws-secrets  # Define this in secretStores
        key: dev/api/key
```

### Popular Provider Examples

{{< tabs >}}
{{< tab "Password Managers" >}}
```yaml
# Bitwarden
secretStores:
  bitwarden:
    type: bitwarden

# 1Password  
secretStores:
  op:
    type: onepassword
    
envs:
  development:
    SECRET:
      from:
        store: op
        key: op://Development/API/key
```
{{< /tab >}}
{{< tab "AWS" >}}
```yaml
# AWS Unified (auto-detects service)
secretStores:
  aws:
    type: aws.unified
    region: us-east-1
    
envs:
  production:
    # Secrets Manager
    API_KEY:
      from:
        store: aws
        key: prod/api/key
    
    # SSM Parameter Store
    CONFIG:
      from:
        store: aws
        key: /myapp/prod/config
```
{{< /tab >}}
{{< tab "Cloud Providers" >}}
```yaml
# Google Cloud
secretStores:
  gcp:
    type: gcp.secretmanager
    project: my-project

# Azure
secretStores:
  azure:
    type: azure.keyvault
    vault_name: mykeyvault
```
{{< /tab >}}
{{< /tabs >}}

## 4. Authenticate

Login to your provider:

{{< tabs >}}
{{< tab "Bitwarden" >}}
```bash
bw login
# Or with API key
export BW_CLIENTID="your-client-id"
export BW_CLIENTSECRET="your-client-secret"
```
{{< /tab >}}
{{< tab "1Password" >}}
```bash
op signin
# Or with service account
export OP_SERVICE_ACCOUNT_TOKEN="ops_..."
```
{{< /tab >}}
{{< tab "AWS" >}}
```bash
# Uses standard AWS auth methods
aws configure
# Or with SSO
aws sso login --profile prod
# Or with IAM role (automatic on EC2)
```
{{< /tab >}}
{{< tab "Google Cloud" >}}
```bash
# Application Default Credentials
gcloud auth application-default login
# Or service account
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/key.json"
```
{{< /tab >}}
{{< tab "Azure" >}}
```bash
# Azure CLI
az login
# Or Managed Identity (automatic on Azure resources)
```
{{< /tab >}}
{{< /tabs >}}

### Verify Authentication

```bash
# Check all providers are working
dsops doctor
```

## 5. Test Secret Resolution

Check that dsops can access your secrets:

```bash
dsops plan --env development
```

## 6. Execute with Secrets

Run a command with secrets injected:

```bash
# Secrets exist only during command execution
dsops exec --env development -- env | grep DATABASE_URL

# Use with any command
dsops exec --env development -- npm start
dsops exec --env development -- docker-compose up
dsops exec --env development -- python manage.py runserver
```

### Alternative: Render Files

If you need a file (less secure):

```bash
# Create .env file
dsops render --env development --out .env

# With automatic deletion after 10 minutes
dsops render --env development --out .env --ttl 10m

# Different formats
dsops render --env development --out config.json --format json
dsops render --env development --out config.yaml --format yaml
```

## Next Steps

- **[Configure Providers](/providers/)** - Set up your secret stores
- **[Advanced Configuration](/getting-started/configuration/)** - Transforms, policies, multiple environments
- **[Secret Rotation](/rotation/)** - Automate credential rotation
- **[Security Best Practices](/reference/security/)** - Secure your workflow

## Real-World Examples

### Multi-Environment Setup

```yaml
version: 1

secretStores:
  # Shared password manager
  passwords:
    type: bitwarden
  
  # Environment-specific cloud providers
  aws-dev:
    type: aws.unified
    region: us-east-1
    profile: development
    
  aws-prod:
    type: aws.unified
    region: us-east-1
    profile: production

envs:
  development:
    DATABASE_URL:
      from:
        store: passwords
        key: "Dev/Database/URL"
    AWS_SECRETS:
      from:
        store: aws-dev
        key: /myapp/dev/config
        
  production:
    DATABASE_URL:
      from:
        store: aws-prod
        key: sm:prod/database/connection
    API_KEY:
      from:
        store: aws-prod
        key: sm:prod/api/key
```

## Common Issues

### Provider Not Found

```bash
# Check provider status
dsops doctor

# List all available providers
dsops providers
```

### Authentication Failed

```bash
# Get provider-specific help
dsops login bitwarden

# Check current authentication
dsops get --env development --key DATABASE_URL
```

### Secret Not Found

```bash
# List what will be resolved
dsops plan --env development

# Debug mode for detailed errors
dsops plan --env development --debug
```

### Performance Issues

```yaml
# Use unified providers to reduce configuration
secretStores:
  aws:
    type: aws.unified  # One provider for all AWS services
    region: us-east-1
```

## Security Tips

1. **Never commit secrets**: Use `dsops guard` to check
2. **Prefer exec over render**: Keep secrets in memory
3. **Use TTL for files**: Auto-delete rendered files
4. **Enable MFA**: On all secret providers
5. **Audit access**: Check provider logs regularly