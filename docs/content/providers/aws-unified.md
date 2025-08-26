---
title: "AWS Unified Provider"
description: "Intelligent routing to the appropriate AWS service based on your secret reference"
lead: "The AWS Unified Provider automatically detects and routes to the correct AWS service (Secrets Manager, SSM Parameter Store, STS, or SSO) based on your secret reference format. One provider, all AWS services."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 40
---

## Overview

The AWS Unified Provider simplifies AWS secret management by automatically routing to the appropriate service based on the secret reference format. Instead of configuring multiple providers, use one unified provider that understands all AWS secret types.

## Features

- **Automatic Service Detection**: Routes to the correct AWS service
- **Unified Configuration**: Single provider for all AWS services  
- **Smart Parsing**: Understands ARNs, paths, and special formats
- **Credential Sharing**: One authentication for all services
- **Backwards Compatible**: Works with existing secret references

## Service Detection Rules

The Unified Provider routes based on these patterns:

| Pattern | Service | Example |
|---------|---------|---------|
| `arn:aws:secretsmanager:*` | Secrets Manager | `arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-AbCdEf` |
| `/path/to/parameter` | SSM Parameter Store | `/myapp/prod/database/password` |
| `parameter/*` | SSM Parameter Store | `parameter/myapp/prod/config` |
| `ssm:*` | SSM Parameter Store | `ssm:/myapp/prod/api-key` |
| `sm:*` | Secrets Manager | `sm:prod/database/credentials` |
| `role:*` | STS AssumeRole | `role:arn:aws:iam::123456789012:role/CrossAccount` |
| `sso:*` | IAM Identity Center | `sso:123456789012:DeveloperAccess` |
| Plain names | Secrets Manager (default) | `prod-database-password` |

## Configuration

### Basic Setup

```yaml
version: 1

secretStores:
  aws:
    type: aws.unified
    region: us-east-1
    # Optional: AWS profile
    profile: production

envs:
  production:
    # Automatically routes to Secrets Manager
    DB_PASSWORD:
      from:
        store: aws
        key: prod/database/password
    
    # Automatically routes to SSM Parameter Store
    API_ENDPOINT:
      from:
        store: aws
        key: /myapp/prod/api/endpoint
    
    # Explicit service prefix
    STRIPE_KEY:
      from:
        store: aws
        key: sm:prod/stripe/secret-key
```

### With Authentication Options

```yaml
secretStores:
  aws:
    type: aws.unified
    region: us-east-1
    # IAM role assumption
    role_arn: arn:aws:iam::123456789012:role/SecretReader
    # Or SSO profile
    profile: prod-readonly
    # Or explicit credentials (not recommended)
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
```

### Multi-Region Support

```yaml
secretStores:
  # US East region
  aws-us:
    type: aws.unified
    region: us-east-1

  # EU West region
  aws-eu:
    type: aws.unified
    region: eu-west-1

envs:
  global:
    US_SECRET:
      from:
        store: aws-us
        key: regional/us-only/secret
    
    EU_SECRET:
      from:
        store: aws-eu
        key: regional/eu-only/secret
```

## Usage Examples

### Mixed Service Types

```yaml
envs:
  production:
    # Secrets Manager (default for plain names)
    DATABASE_PASSWORD:
      from:
        store: aws
        key: prod-database-password
    
    # SSM Parameter Store (path-based)
    FEATURE_FLAGS:
      from:
        store: aws
        key: /myapp/prod/features/flags
    
    # Secrets Manager with ARN
    RDS_MASTER_PASSWORD:
      from:
        store: aws
        key: arn:aws:secretsmanager:us-east-1:123456789012:secret:rds/prod/master
    
    # SSM with prefix
    CONFIG_VALUE:
      from:
        store: aws
        key: ssm:/myapp/prod/config/timeout
```

### With Transforms

```yaml
envs:
  production:
    # Extract JSON field from Secrets Manager
    DB_HOST:
      from:
        store: aws
        key: prod/database/connection
      transform: json_extract:.host
    
    # Base64 decode from SSM
    CERTIFICATE:
      from:
        store: aws
        key: /myapp/prod/certs/ssl
      transform: base64_decode
```

### Cross-Account Access

```yaml
secretStores:
  # Assume role automatically
  aws-cross-account:
    type: aws.unified
    region: us-east-1
    role_arn: arn:aws:iam::987654321098:role/CrossAccountReader

envs:
  production:
    # Access secrets in another account
    VENDOR_API_KEY:
      from:
        store: aws-cross-account
        key: vendor/api/key
```

## Service-Specific Features

### Secrets Manager Features

When routing to Secrets Manager:

```yaml
# Version support
PREVIOUS_SECRET:
  from:
    store: aws
    key: prod/api/key
    version: AWSPREVIOUS

# JSON extraction
DB_USERNAME:
  from:
    store: aws
    key: sm:prod/database/creds
  transform: json_extract:.username
```

### SSM Parameter Store Features

When routing to SSM:

```yaml
# Get multiple parameters by path
ALL_CONFIGS:
  from:
    store: aws
    key: /myapp/prod/config/*

# Specific version
OLD_CONFIG:
  from:
    store: aws
    key: /myapp/prod/config/main:3
```

### STS AssumeRole

Using role prefix for cross-account access:

```yaml
envs:
  production:
    # First assume role, then get secret
    CROSS_ACCOUNT_SECRET:
      from:
        store: aws
        key: role:arn:aws:iam::123456789012:role/Reader|sm:prod/secret
```

## Advanced Patterns

### Service Chaining

Chain multiple services together:

```yaml
secretStores:
  aws:
    type: aws.unified
    region: us-east-1

envs:
  production:
    # Get role ARN from SSM, then assume it, then get secret
    DYNAMIC_SECRET:
      from:
        store: aws
        key: chain:/config/role-arn|role:*|sm:prod/database
```

### Fallback Patterns

```yaml
envs:
  production:
    # Try Secrets Manager first, fall back to SSM
    API_KEY:
      from:
        store: aws
        key: prod/api/key
        fallback: /myapp/prod/api/key
```

### Dynamic Service Selection

```yaml
envs:
  production:
    # Service determined by environment variable
    CONFIG_VALUE:
      from:
        store: aws
        key: ${SECRET_SERVICE}:${SECRET_PATH}
```

## Migration Guide

### From Multiple Providers

Before:
```yaml
secretStores:
  aws-secrets:
    type: aws.secretsmanager
    region: us-east-1

  aws-params:
    type: aws.ssm
    region: us-east-1

envs:
  production:
    SECRET:
      from:
        store: aws-secrets
        key: prod/secret
    
    PARAM:
      from:
        store: aws-params
        key: /prod/param
```

After:
```yaml
secretStores:
  aws:
    type: aws.unified
    region: us-east-1

envs:
  production:
    SECRET:
      from:
        store: aws
        key: prod/secret  # Auto-detects Secrets Manager
    
    PARAM:
      from:
        store: aws
        key: /prod/param  # Auto-detects Parameter Store
```

### Explicit Service Control

If you need explicit service control:

```yaml
envs:
  production:
    # Force Secrets Manager
    CONFIG_AS_SECRET:
      from:
        store: aws
        key: sm:/configuration/data
    
    # Force Parameter Store
    SECRET_AS_PARAM:
      from:
        store: aws
        key: ssm:secure-value
```

## Best Practices

### 1. Use Clear Naming

```yaml
# Good: Service is obvious from the name/path
DATABASE_URL:
  from: prod/database/url  # Clearly a secret

API_ENDPOINT:
  from: /config/api/endpoint  # Clearly a parameter

# Avoid: Ambiguous names
VALUE:
  from: myvalue  # Which service?
```

### 2. Organize by Pattern

```yaml
# Secrets Manager for sensitive data
secrets:
  - prod/database/password
  - prod/api/private-key
  - prod/certificates/ssl

# Parameter Store for configuration
parameters:
  - /myapp/prod/config/timeout
  - /myapp/prod/features/flags
  - /myapp/prod/endpoints/api
```

### 3. Be Consistent

Choose a pattern and stick with it:

```yaml
# Pattern 1: Service prefixes everywhere
envs:
  production:
    SECRET: { from: "sm:prod/secret" }
    PARAM: { from: "ssm:/prod/param" }

# Pattern 2: Natural detection
envs:
  production:
    SECRET: { from: "prod/secret" }
    PARAM: { from: "/prod/param" }
```

### 4. Document Service Choice

```yaml
envs:
  production:
    # Stored in Secrets Manager (rotates monthly)
    DATABASE_PASSWORD:
      from:
        store: aws
        key: prod/database/password
    
    # Stored in Parameter Store (static config)
    RATE_LIMIT:
      from:
        store: aws
        key: /config/api/rate-limit
```

## Troubleshooting

### Service Detection Issues

```bash
# Test service detection
dsops plan --env production --debug

# Output shows which service was selected:
# DEBUG: Unified provider routing 'prod/secret' to Secrets Manager
# DEBUG: Unified provider routing '/config/param' to SSM Parameter Store
```

### Ambiguous References

If a reference could match multiple services:

```yaml
# Ambiguous - could be either service
UNCLEAR:
  from: configuration

# Clear - use prefix
CLEAR:
  from: sm:configuration  # Forces Secrets Manager
```

### Permission Errors

Ensure IAM role has permissions for all services:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "ssm:GetParameter",
        "ssm:GetParameters",
        "sts:AssumeRole"
      ],
      "Resource": "*"
    }
  ]
}
```

### Performance Considerations

The Unified Provider adds minimal overhead:
- Service detection is done once per secret
- No additional API calls for routing
- Same performance as direct providers

## Security Considerations

### 1. Least Privilege

Even though using one provider, apply least privilege:

```json
{
  "Statement": [
    {
      "Effect": "Allow",
      "Action": "secretsmanager:GetSecretValue",
      "Resource": "arn:aws:secretsmanager:*:*:secret:prod/*"
    },
    {
      "Effect": "Allow",
      "Action": "ssm:GetParameter",
      "Resource": "arn:aws:ssm:*:*:parameter/myapp/prod/*"
    }
  ]
}
```

### 2. Service Isolation

Consider separate unified providers for different security contexts:

```yaml
secretStores:
  # High-privilege secrets
  aws-sensitive:
    type: aws.unified
    role_arn: arn:aws:iam::123456789012:role/HighPrivilege

  # Low-privilege configs
  aws-config:
    type: aws.unified
    role_arn: arn:aws:iam::123456789012:role/ConfigReader
```

### 3. Audit Considerations

Unified Provider maintains full audit trail:
- CloudTrail logs show actual service accessed
- No loss of audit information
- Service-specific events preserved

## Limitations

1. **Service-Specific Features**: Some advanced features may require direct providers
2. **Complex Patterns**: Very complex routing may need custom logic
3. **New Services**: Must be updated to support new AWS services

## Related Documentation

- [AWS Secrets Manager](./aws-secrets-manager/)
- [AWS Systems Manager Parameter Store](./aws-ssm/)
- [AWS STS](./aws-sts/)
- [AWS IAM Identity Center](./aws-sso/)
- [Unified Provider Design Doc](/contributing/adr/unified-providers/)