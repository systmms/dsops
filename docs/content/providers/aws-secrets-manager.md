---
title: "AWS Secrets Manager"
description: "Use AWS Secrets Manager for secure, scalable secret storage with automatic rotation"
lead: "AWS Secrets Manager helps you protect access to your applications, services, and IT resources without the upfront investment and on-going maintenance costs of operating your own infrastructure."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 20
---

## Overview

AWS Secrets Manager is a secrets management service that helps you protect access to your applications, services, and IT resources. It enables you to easily rotate, manage, and retrieve database credentials, API keys, and other secrets throughout their lifecycle.

## Features

- **Automatic Rotation**: Built-in rotation for RDS, Redshift, and DocumentDB
- **Fine-grained Access**: IAM policies for precise access control
- **Encryption**: Automatic encryption using AWS KMS
- **Versioning**: Maintain multiple versions with staging labels
- **Cross-Region Replication**: Replicate secrets across AWS regions
- **Audit Trail**: Full integration with AWS CloudTrail

## Prerequisites

1. **AWS Account**: Active AWS account with appropriate permissions
2. **IAM Permissions**: Access to create and read secrets
3. **AWS CLI** (optional): For testing and troubleshooting

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "secretsmanager:GetSecretValue",
        "secretsmanager:DescribeSecret"
      ],
      "Resource": "arn:aws:secretsmanager:*:*:secret:*"
    },
    {
      "Effect": "Allow",
      "Action": "secretsmanager:ListSecrets",
      "Resource": "*"
    }
  ]
}
```

## Configuration

Add AWS Secrets Manager to your `dsops.yaml`:

```yaml
version: 1

secretStores:
  aws-sm:
    type: aws.secretsmanager
    region: us-east-1
    # Optional: specific AWS profile
    profile: production
    # Optional: assume role
    role_arn: arn:aws:iam::123456789012:role/SecretsReader

envs:
  development:
    DATABASE_URL:
      from:
        store: aws-sm
        key: dev/database/connection
    
    # Extract specific field from JSON secret
    DB_PASSWORD:
      from:
        store: aws-sm
        key: dev/database/credentials
      transform: json_extract:.password
    
    # Get specific version
    API_KEY:
      from:
        store: aws-sm
        key: prod/api/key
        version: AWSPREVIOUS
```

## Authentication Methods

### 1. IAM Instance Profile (Recommended for EC2)

```yaml
secretStores:
  aws-sm:
    type: aws.secretsmanager
    region: us-east-1
    # No credentials needed - uses instance profile
```

### 2. AWS Profile

```yaml
secretStores:
  aws-sm:
    type: aws.secretsmanager
    region: us-east-1
    profile: production
```

### 3. Assume Role

```yaml
secretStores:
  aws-sm:
    type: aws.secretsmanager
    region: us-east-1
    role_arn: arn:aws:iam::123456789012:role/SecretsReader
    # Optional: external ID for additional security
    external_id: unique-external-id
```

### 4. Explicit Credentials (Not Recommended)

```yaml
secretStores:
  aws-sm:
    type: aws.secretsmanager
    region: us-east-1
    access_key_id: ${AWS_ACCESS_KEY_ID}
    secret_access_key: ${AWS_SECRET_ACCESS_KEY}
```

## Secret Formats

### Plain Text Secrets

```bash
# Create plain text secret
aws secretsmanager create-secret \
  --name dev/api/key \
  --secret-string "sk_test_1234567890"
```

```yaml
API_KEY:
  from:
    store: aws-sm
    key: dev/api/key
```

### JSON Secrets

```bash
# Create JSON secret
aws secretsmanager create-secret \
  --name dev/database/credentials \
  --secret-string '{"username":"admin","password":"secret123","host":"db.example.com"}'
```

```yaml
# Extract specific field
DB_PASSWORD:
  from:
    store: aws-sm
    key: dev/database/credentials
  transform: json_extract:.password

# Get entire JSON
DB_CONFIG:
  from:
    store: aws-sm
    key: dev/database/credentials
```

### Binary Secrets

```yaml
# Base64 encoded binary data
CERTIFICATE:
  from:
    store: aws-sm
    key: prod/certificates/ssl
  transform: base64_decode
```

## Secret Naming Conventions

Follow AWS best practices for organizing secrets:

```
environment/service/resource
│           │       │
│           │       └── Specific resource (database, api-key, certificate)
│           └────────── Service or application name
└────────────────────── Environment (dev, staging, prod)

Examples:
- dev/myapp/database
- prod/api-gateway/key
- staging/rds/admin-password
- prod/lambda/env-vars
```

## Versioning and Staging Labels

AWS Secrets Manager uses staging labels to manage versions:

```yaml
envs:
  production:
    # Current version (default)
    CURRENT_KEY:
      from:
        store: aws-sm
        key: prod/api/key
    
    # Previous version
    FALLBACK_KEY:
      from:
        store: aws-sm
        key: prod/api/key
        version: AWSPREVIOUS
    
    # Specific version by ID
    ARCHIVED_KEY:
      from:
        store: aws-sm
        key: prod/api/key
        version: "a1b2c3d4-5678-90ab-cdef-EXAMPLE11111"
```

## Rotation Configuration

### Automatic Rotation

For supported services (RDS, Redshift, DocumentDB):

```bash
aws secretsmanager rotate-secret \
  --secret-id prod/rds/admin \
  --rotation-lambda-arn arn:aws:lambda:region:123456789012:function:SecretsManagerRotation
```

### Manual Rotation with dsops

```bash
# Rotate a secret managed by dsops
dsops secrets rotate --env production --key DATABASE_PASSWORD

# Check rotation status
dsops rotation status --service aws-rds-prod
```

### Rotation Configuration in dsops

```yaml
services:
  rds-prod:
    type: aws-rds
    secret_arn: arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/rds/admin

envs:
  production:
    DB_PASSWORD:
      from:
        store: aws-sm
        key: prod/rds/admin
      service: rds-prod  # Links to rotation
```

## Best Practices

### 1. Use Hierarchical Names

```yaml
# Good: Clear hierarchy
DATABASE_URL:
  from: prod/myapp/rds/connection-string

# Avoid: Flat naming
DATABASE_URL:
  from: prod-myapp-database
```

### 2. Tag Your Secrets

```bash
aws secretsmanager tag-resource \
  --secret-id prod/database/admin \
  --tags Key=Environment,Value=Production Key=Application,Value=MyApp
```

### 3. Enable Cross-Region Replication

For disaster recovery:

```bash
aws secretsmanager replicate-secret-to-regions \
  --secret-id prod/critical/secret \
  --replica-regions Region=us-west-2,KmsKeyId=alias/aws/secretsmanager
```

### 4. Use KMS Keys

```yaml
secretStores:
  aws-sm-encrypted:
    type: aws.secretsmanager
    region: us-east-1
    kms_key_id: alias/my-app-secrets
```

## Cost Optimization

### Storage Costs
- $0.40 per secret per month
- $0.05 per 10,000 API calls

### Cost-Saving Tips

1. **Consolidate Secrets**: Store related values in JSON
2. **Use Parameter Store**: For non-rotating, simple values
3. **Clean Up**: Delete unused versions and secrets
4. **Cache Wisely**: Implement client-side caching

```yaml
# Consolidate related secrets
envs:
  production:
    # Instead of multiple secrets...
    DB_HOST:
      from: prod/db/host
    DB_PORT:
      from: prod/db/port
    DB_USER:
      from: prod/db/user
    
    # Use one JSON secret
    DB_CONFIG:
      from:
        store: aws-sm
        key: prod/db/config
      # Then extract in your app or use transforms
```

## Troubleshooting

### Access Denied

```bash
# Check IAM permissions
aws secretsmanager get-secret-value \
  --secret-id dev/test \
  --query SecretString

# Verify resource policy
aws secretsmanager get-resource-policy \
  --secret-id dev/test
```

### Secret Not Found

```bash
# List available secrets
aws secretsmanager list-secrets \
  --filters Key=name,Values=dev/

# Check secret details
aws secretsmanager describe-secret \
  --secret-id dev/database/config
```

### Version Issues

```bash
# List all versions
aws secretsmanager list-secret-version-ids \
  --secret-id prod/api/key

# Get specific version
aws secretsmanager get-secret-value \
  --secret-id prod/api/key \
  --version-stage AWSPREVIOUS
```

### Performance Issues

1. **Enable caching**: Reduce API calls
2. **Use regional endpoints**: Minimize latency
3. **Batch operations**: Retrieve multiple secrets efficiently

## Security Considerations

### 1. Resource Policies

Limit access to specific principals:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::123456789012:role/MyApp"
      },
      "Action": "secretsmanager:GetSecretValue",
      "Resource": "*"
    }
  ]
}
```

### 2. VPC Endpoints

Use VPC endpoints for private connectivity:

```bash
aws ec2 create-vpc-endpoint \
  --vpc-id vpc-12345678 \
  --service-name com.amazonaws.region.secretsmanager
```

### 3. Audit with CloudTrail

Monitor secret access:

```bash
# Check who accessed a secret
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=ResourceName,AttributeValue=prod/database/admin
```

### 4. Encryption

- Secrets encrypted at rest using AWS KMS
- Encrypted in transit using TLS
- Use customer-managed KMS keys for additional control

## Integration Examples

### With RDS

```yaml
envs:
  production:
    # Automatic rotation enabled
    DB_CONNECTION:
      from:
        store: aws-sm
        key: prod/rds/myapp/connection
      transform: json_extract:.connectionString
```

### With Lambda

```yaml
envs:
  production:
    # Lambda environment variables
    FUNCTION_CONFIG:
      from:
        store: aws-sm
        key: prod/lambda/myfunction/env
```

### With ECS/Fargate

```yaml
# Task definition reference
envs:
  production:
    APP_SECRETS:
      from:
        store: aws-sm
        key: prod/ecs/myapp/secrets
```

## Migration Guide

### From Environment Variables

```bash
# Export existing env vars to Secrets Manager
echo "$DATABASE_URL" | aws secretsmanager create-secret \
  --name dev/database/url \
  --secret-string file:///dev/stdin
```

### From Parameter Store

```bash
# Migrate from SSM Parameter Store
VALUE=$(aws ssm get-parameter --name /dev/database/password --query Parameter.Value --output text)
aws secretsmanager create-secret \
  --name dev/database/password \
  --secret-string "$VALUE"
```

## Related Documentation

- [AWS Secrets Manager Documentation](https://docs.aws.amazon.com/secretsmanager/)
- [Rotation Function Templates](https://github.com/aws-samples/aws-secrets-manager-rotation-lambdas)
- [Best Practices Guide](https://docs.aws.amazon.com/secretsmanager/latest/userguide/best-practices.html)
- [Pricing Details](https://aws.amazon.com/secrets-manager/pricing/)