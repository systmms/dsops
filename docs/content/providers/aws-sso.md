---
title: "AWS IAM Identity Center (SSO)"
description: "Use AWS IAM Identity Center for centralized access to multiple AWS accounts"
lead: "AWS IAM Identity Center (formerly AWS SSO) provides single sign-on access to multiple AWS accounts and business applications. dsops integrates with Identity Center to retrieve temporary credentials for your AWS accounts."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 35
---

## Overview

AWS IAM Identity Center enables centralized access management across AWS accounts and applications. It provides:

- **Single Sign-On**: One login for all AWS accounts
- **Temporary Credentials**: Auto-rotating security credentials
- **Permission Sets**: Reusable permission templates
- **Multi-Account Access**: Switch between accounts seamlessly
- **SAML Integration**: Connect with external identity providers

## Features

- **Browser-Based Authentication**: Secure login through web browser
- **CLI Integration**: Works with AWS CLI v2 profiles
- **Automatic Refresh**: Credentials refresh automatically
- **MFA Support**: Enforce multi-factor authentication
- **Audit Trail**: All access logged in CloudTrail
- **No Long-Term Credentials**: Everything is temporary

## Prerequisites

1. **AWS IAM Identity Center**: Configured in your organization
2. **AWS CLI v2**: Version 2.0+ required for SSO support
3. **Account Assignment**: Access granted to appropriate accounts
4. **SSO Portal URL**: Your organization's SSO start URL

### Initial Setup

```bash
# Configure AWS CLI with SSO
aws configure sso

# Interactive prompts:
# SSO start URL: https://my-org.awsapps.com/start
# SSO region: us-east-1
# Account: 123456789012 (Production)
# Role: DeveloperAccess
# CLI default region: us-east-1
# CLI default output format: json
# CLI profile name: prod-dev
```

## Configuration

### Basic SSO Configuration

```yaml
version: 1

secretStores:
  # IAM Identity Center authentication
  aws-sso-prod:
    type: aws.sso
    sso_start_url: https://my-org.awsapps.com/start
    sso_region: us-east-1
    account_id: "123456789012"
    role_name: DeveloperAccess
    region: us-east-1

  # Use SSO credentials for Secrets Manager
  aws-secrets-prod:
    type: aws.secretsmanager
    region: us-east-1
    credentials_from: aws-sso-prod

envs:
  production:
    DATABASE_URL:
      from:
        store: aws-secrets-prod
        key: prod/database/connection
```

### Multiple Account Access

```yaml
secretStores:
  # Development account
  aws-sso-dev:
    type: aws.sso
    sso_start_url: https://my-org.awsapps.com/start
    sso_region: us-east-1
    account_id: "111111111111"
    role_name: DeveloperAccess
    region: us-east-1

  # Production account
  aws-sso-prod:
    type: aws.sso
    sso_start_url: https://my-org.awsapps.com/start
    sso_region: us-east-1
    account_id: "222222222222"
    role_name: ReadOnlyAccess
    region: us-east-1

  # Staging account
  aws-sso-staging:
    type: aws.sso
    sso_start_url: https://my-org.awsapps.com/start
    sso_region: us-east-1
    account_id: "333333333333"
    role_name: PowerUserAccess
    region: us-east-1
```

### Using CLI Profile

Reference existing AWS CLI SSO profiles:

```yaml
secretStores:
  aws-sso-existing:
    type: aws.sso
    profile: prod-dev  # References ~/.aws/config profile
    region: us-east-1
```

## Authentication Flow

### First-Time Authentication

```bash
# dsops will prompt to authenticate
$ dsops doctor --config config.yaml

Attempting to automatically open the SSO authorization page in your browser.
If the browser does not open, you can manually open the following URL:

https://device.sso.us-east-1.amazonaws.com?user_code=ABCD-EFGH

Successfully logged in to SSO!
```

### Token Management

SSO tokens are cached locally:

- **Location**: `~/.aws/sso/cache/`
- **Duration**: 8 hours (configurable by admin)
- **Auto-Refresh**: dsops handles refresh automatically
- **Secure Storage**: Tokens encrypted at rest

## Permission Sets

### Understanding Permission Sets

Permission sets define what users can do in each account:

```yaml
# Common permission sets
- AdministratorAccess    # Full admin rights
- PowerUserAccess       # Everything except IAM
- DeveloperAccess       # Development resources
- ReadOnlyAccess        # View-only permissions
- SecurityAudit         # Audit and compliance
```

### Custom Permission Sets

Your organization may have custom permission sets:

```yaml
secretStores:
  aws-sso-custom:
    type: aws.sso
    sso_start_url: https://my-org.awsapps.com/start
    sso_region: us-east-1
    account_id: "123456789012"
    role_name: CustomDeveloperRole  # Organization-specific
    region: us-east-1
```

## Best Practices

### 1. Use Descriptive Store Names

```yaml
secretStores:
  # Good: Clear purpose and environment
  aws-sso-prod-readonly:
    type: aws.sso
    account_id: "123456789012"
    role_name: ReadOnlyAccess

  # Avoid: Generic names
  aws-1:
    type: aws.sso
    account_id: "123456789012"
```

### 2. Organize by Environment

```yaml
# Development stores
aws-sso-dev-admin:
  account_id: "111111111111"
  role_name: AdministratorAccess

aws-sso-dev-readonly:
  account_id: "111111111111"
  role_name: ReadOnlyAccess

# Production stores  
aws-sso-prod-readonly:
  account_id: "222222222222"
  role_name: ReadOnlyAccess

aws-sso-prod-deploy:
  account_id: "222222222222"
  role_name: DeploymentRole
```

### 3. Leverage Credential Chaining

```yaml
secretStores:
  # SSO for authentication
  auth:
    type: aws.sso
    account_id: "123456789012"
    role_name: DeveloperAccess

  # Services using SSO credentials
  secrets:
    type: aws.secretsmanager
    credentials_from: auth

  parameters:
    type: aws.ssm
    credentials_from: auth

  s3:
    type: aws.s3  # If implemented
    credentials_from: auth
```

### 4. Handle Multiple Organizations

If working with multiple organizations:

```yaml
secretStores:
  # Internal organization
  internal-sso:
    type: aws.sso
    sso_start_url: https://internal.awsapps.com/start
    sso_region: us-east-1
    account_id: "111111111111"

  # Client organization
  client-sso:
    type: aws.sso
    sso_start_url: https://client.awsapps.com/start
    sso_region: eu-west-1
    account_id: "222222222222"
```

## Troubleshooting

### SSO Session Expired

```bash
# Check current session
aws sso logout --profile prod-dev
aws sso login --profile prod-dev

# Or let dsops handle it
dsops doctor --config config.yaml
```

### Browser Issues

If browser doesn't open automatically:

```bash
# Set browser explicitly
export AWS_SSO_BROWSER="google-chrome"

# Or use CLI-only mode
export AWS_SSO_DISABLE_BROWSER=true
# Then manually open the provided URL
```

### Cache Problems

Clear SSO cache if experiencing issues:

```bash
# Clear SSO cache
rm -rf ~/.aws/sso/cache/

# Clear CLI cache
rm -rf ~/.aws/cli/cache/
```

### Permission Errors

```bash
# Verify your access
aws sts get-caller-identity --profile prod-dev

# List your permission sets
aws sso-admin list-permission-sets-provisioned-to-account \
  --instance-arn $SSO_INSTANCE_ARN \
  --account-id 123456789012
```

## Security Considerations

### 1. Token Security

- SSO tokens are short-lived (8 hours max)
- Stored encrypted in local cache
- Automatically invalidated on logout
- Can't be used outside authorized IP ranges (if configured)

### 2. MFA Enforcement

Configure Identity Center to require MFA:

```yaml
# MFA is handled by Identity Center, not in dsops config
# Users will be prompted during browser authentication
```

### 3. IP Restrictions

Identity Center supports IP-based access control:

- Configure in AWS Organizations
- Applies to all SSO authentication
- Transparent to dsops users

### 4. Audit Logging

All SSO activities are logged:

```bash
# View SSO authentication events
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=EventName,AttributeValue=Authenticate

# View role assumptions
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=EventName,AttributeValue=AssumeRoleWithSAML
```

## CI/CD Integration

### GitHub Actions

```yaml
# Not recommended: SSO requires interactive auth
# Use IAM roles or OIDC instead for CI/CD
```

### Alternative for Automation

For non-interactive environments, use AWS IAM Identity Center with:

1. **SCIM**: Automate user provisioning
2. **API Access**: Programmatic permission set management
3. **Service Accounts**: Create dedicated automation users

## Common Patterns

### Development Workflow

```yaml
secretStores:
  # Development with full access
  dev:
    type: aws.sso
    sso_start_url: ${COMPANY_SSO_URL}
    account_id: ${DEV_ACCOUNT_ID}
    role_name: DeveloperAccess

  # Production with read-only
  prod-read:
    type: aws.sso
    sso_start_url: ${COMPANY_SSO_URL}
    account_id: ${PROD_ACCOUNT_ID}
    role_name: ReadOnlyAccess

envs:
  local:
    # Use dev secrets for local development
    DATABASE_URL:
      from:
        store: dev-secrets
        key: app/database/url

  verify-prod:
    # Read production values (not write)
    PROD_API_ENDPOINT:
      from:
        store: prod-parameters
        key: /app/api/endpoint
```

### Multi-Region Access

```yaml
secretStores:
  # US East region
  sso-us-east:
    type: aws.sso
    account_id: "123456789012"
    role_name: DeveloperAccess
    region: us-east-1

  # EU West region (same account)
  sso-eu-west:
    type: aws.sso
    account_id: "123456789012"
    role_name: DeveloperAccess
    region: eu-west-1
```

## Migration from IAM Users

### Before (IAM Users)

```yaml
secretStores:
  aws-iam:
    type: aws.secretsmanager
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
```

### After (SSO)

```yaml
secretStores:
  aws-sso:
    type: aws.sso
    sso_start_url: https://my-org.awsapps.com/start
    account_id: "123456789012"
    role_name: DeveloperAccess

  aws-secrets:
    type: aws.secretsmanager
    credentials_from: aws-sso
```

## Performance Tips

### 1. Cache Credentials

SSO credentials are cached automatically:
- 8-hour default duration
- Refresh happens transparently
- No manual intervention needed

### 2. Parallel Authentication

When using multiple accounts:
```bash
# Authenticate to all profiles at once
aws sso login --profile dev &
aws sso login --profile staging &
aws sso login --profile prod &
wait
```

### 3. Profile Organization

```ini
# ~/.aws/config
[profile dev]
sso_start_url = https://my-org.awsapps.com/start
sso_region = us-east-1
sso_account_id = 111111111111
sso_role_name = DeveloperAccess

[profile prod]
sso_start_url = https://my-org.awsapps.com/start
sso_region = us-east-1
sso_account_id = 222222222222
sso_role_name = ReadOnlyAccess
```

## Known Limitations

1. **Interactive Only**: Requires browser for initial auth
2. **No Service Accounts**: Not suitable for fully automated workflows
3. **Token Duration**: Maximum 12 hours (admin-configured)
4. **Regional**: SSO is regional, may need multiple configurations

## Related Documentation

- [AWS IAM Identity Center Guide](https://docs.aws.amazon.com/singlesignon/latest/userguide/)
- [AWS CLI v2 SSO](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-sso.html)
- [Permission Sets](https://docs.aws.amazon.com/singlesignon/latest/userguide/permissionsets.html)
- [SCIM Provisioning](https://docs.aws.amazon.com/singlesignon/latest/userguide/provision-automatically.html)