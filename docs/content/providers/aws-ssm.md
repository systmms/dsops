---
title: "AWS Systems Manager Parameter Store"
description: "Use AWS SSM Parameter Store for hierarchical configuration and secret storage"
lead: "AWS Systems Manager Parameter Store provides secure, hierarchical storage for configuration data and secrets. It's a cost-effective alternative to Secrets Manager for simpler use cases."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 25
---

## Overview

AWS Systems Manager (SSM) Parameter Store is a secure storage service for configuration data and secrets. It offers a hierarchical structure, versioning, and integration with other AWS services, making it ideal for application configuration and simple secrets that don't require automatic rotation.

## Features

- **Hierarchical Storage**: Organize parameters using paths like `/app/prod/db/password`
- **Parameter Types**: String, StringList, and SecureString (encrypted)
- **Versioning**: Track parameter changes over time
- **Cost-Effective**: Free tier includes 10,000 parameters
- **IAM Integration**: Fine-grained access control
- **Parameter Policies**: Expiration and notification policies

## SSM vs Secrets Manager

| Feature | SSM Parameter Store | Secrets Manager |
|---------|-------------------|-----------------|
| **Cost** | Free tier (10k params) | $0.40/secret/month |
| **Rotation** | Manual only | Automatic available |
| **API Throughput** | Lower | Higher |
| **Max Size** | 4KB (standard), 8KB (advanced) | 64KB |
| **Use Case** | Config, simple secrets | Complex secrets, rotation |

## Prerequisites

1. **AWS Account**: With appropriate IAM permissions
2. **IAM Permissions**: Read access to parameters
3. **KMS Key Access**: For SecureString parameters

### Required IAM Permissions

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ssm:GetParameter",
        "ssm:GetParameters",
        "ssm:GetParametersByPath"
      ],
      "Resource": "arn:aws:ssm:*:*:parameter/*"
    },
    {
      "Effect": "Allow",
      "Action": "kms:Decrypt",
      "Resource": "arn:aws:kms:*:*:key/*",
      "Condition": {
        "StringEquals": {
          "kms:ViaService": "ssm.region.amazonaws.com"
        }
      }
    }
  ]
}
```

## Configuration

Add SSM Parameter Store to your `dsops.yaml`:

```yaml
version: 1

secretStores:
  aws-params:
    type: aws.ssm
    region: us-east-1
    # Optional: AWS profile
    profile: production
    # Optional: assume role
    role_arn: arn:aws:iam::123456789012:role/ParameterReader

envs:
  development:
    # Simple parameter
    API_ENDPOINT:
      from:
        store: aws-params
        key: /myapp/dev/api/endpoint
    
    # SecureString parameter
    DATABASE_PASSWORD:
      from:
        store: aws-params
        key: /myapp/dev/db/password
    
    # Get by path (multiple parameters)
    APP_CONFIG:
      from:
        store: aws-params
        key: /myapp/dev/config/*
```

## Parameter Types

### String Parameters

Plain text configuration values:

```bash
aws ssm put-parameter \
  --name /myapp/dev/api/endpoint \
  --value "https://api.dev.example.com" \
  --type String
```

```yaml
API_ENDPOINT:
  from:
    store: aws-params
    key: /myapp/dev/api/endpoint
```

### SecureString Parameters

Encrypted sensitive data:

```bash
# Using default KMS key
aws ssm put-parameter \
  --name /myapp/prod/db/password \
  --value "secret123" \
  --type SecureString

# Using custom KMS key
aws ssm put-parameter \
  --name /myapp/prod/api/key \
  --value "sk_live_abc123" \
  --type SecureString \
  --key-id alias/myapp-prod
```

```yaml
DB_PASSWORD:
  from:
    store: aws-params
    key: /myapp/prod/db/password
```

### StringList Parameters

Comma-separated values:

```bash
aws ssm put-parameter \
  --name /myapp/dev/allowed-origins \
  --value "https://dev.example.com,https://staging.example.com" \
  --type StringList
```

```yaml
ALLOWED_ORIGINS:
  from:
    store: aws-params
    key: /myapp/dev/allowed-origins
```

## Hierarchical Organization

### Naming Conventions

Use a consistent hierarchy:

```
/environment/application/service/parameter
│            │           │       │
│            │           │       └── Specific parameter name
│            │           └────────── Service or component
│            └────────────────────── Application name
└─────────────────────────────────── Environment

Examples:
/dev/myapp/database/host
/prod/myapp/redis/connection-string
/staging/api-gateway/rate-limit
/shared/certificates/root-ca
```

### Path-based Retrieval

Get all parameters under a path:

```yaml
envs:
  production:
    # This requires special handling in your app
    ALL_CONFIG:
      from:
        store: aws-params
        key: /myapp/prod/*
```

## Versioning

SSM automatically versions parameters:

```yaml
envs:
  production:
    # Latest version (default)
    CURRENT_CONFIG:
      from:
        store: aws-params
        key: /myapp/prod/config
    
    # Specific version
    PREVIOUS_CONFIG:
      from:
        store: aws-params
        key: /myapp/prod/config:2
    
    # Label-based (if using labels)
    STABLE_CONFIG:
      from:
        store: aws-params
        key: /myapp/prod/config:stable
```

## Parameter Policies

### Expiration Policy

Set parameters to expire:

```json
{
  "Type": "Expiration",
  "Version": "1.0",
  "Attributes": {
    "Timestamp": "2025-12-31T23:59:59Z"
  }
}
```

### Change Notification

Get notified on parameter changes:

```json
{
  "Type": "ExpirationNotification",
  "Version": "1.0",
  "Attributes": {
    "Before": "30",
    "Unit": "Days"
  }
}
```

## Advanced Features

### Parameter Store with KMS

```yaml
secretStores:
  aws-params-encrypted:
    type: aws.ssm
    region: us-east-1
    # Specify KMS key for SecureString parameters
    kms_key_id: alias/parameter-store
```

### Cross-Account Access

```yaml
secretStores:
  shared-params:
    type: aws.ssm
    region: us-east-1
    role_arn: arn:aws:iam::987654321098:role/CrossAccountParameterReader
    external_id: unique-external-id
```

### Public Parameters

Access AWS public parameters:

```yaml
envs:
  production:
    # Latest Amazon Linux 2 AMI
    AMI_ID:
      from:
        store: aws-params
        key: /aws/service/ami-amazon-linux-latest/amzn2-ami-hvm-x86_64-gp2
```

## Best Practices

### 1. Use Hierarchical Paths

```yaml
# Good: Clear hierarchy
DATABASE_HOST:
  from: /prod/myapp/rds/endpoint

# Avoid: Flat naming
DATABASE_HOST:
  from: prod-myapp-database-host
```

### 2. Separate Config from Secrets

```yaml
# Configuration (String type)
APP_VERSION:
  from: /myapp/prod/config/version

# Secrets (SecureString type)  
API_KEY:
  from: /myapp/prod/secrets/api-key
```

### 3. Use Standard Parameters for Static Config

Standard parameters are free and suitable for non-sensitive configuration:

```bash
# Configuration values
aws ssm put-parameter \
  --name /myapp/prod/feature-flags \
  --value '{"new-ui": true, "beta-api": false}' \
  --type String
```

### 4. Tag Your Parameters

```bash
aws ssm add-tags-to-resource \
  --resource-type Parameter \
  --resource-id /myapp/prod/database/password \
  --tags Key=Environment,Value=Production Key=Application,Value=MyApp
```

## Cost Optimization

### Parameter Tiers

| Feature | Standard | Advanced |
|---------|----------|----------|
| **Parameter Limit** | 10,000 | 100,000 |
| **Max Size** | 4 KB | 8 KB |
| **Parameter Policies** | No | Yes |
| **Cost** | Free | $0.05 per parameter/month |

### Cost-Saving Tips

1. **Use Standard Parameters**: For configuration under 4KB
2. **Avoid Over-Versioning**: Set version limits
3. **Clean Up**: Delete unused parameters
4. **Batch Operations**: Use GetParameters for multiple values

```yaml
# Efficient: Batch retrieval
envs:
  production:
    # Consider using a single JSON parameter
    APP_CONFIG:
      from:
        store: aws-params
        key: /myapp/prod/config
      transform: json_extract
```

## Troubleshooting

### Parameter Not Found

```bash
# List parameters
aws ssm describe-parameters \
  --filters "Key=Path,Values=/myapp/dev"

# Check specific parameter
aws ssm get-parameter \
  --name /myapp/dev/api/key
```

### Access Denied

```bash
# Check IAM permissions
aws ssm get-parameter \
  --name /test/parameter \
  --with-decryption

# For SecureString, ensure KMS access
aws kms describe-key \
  --key-id alias/aws/ssm
```

### Decryption Failures

```bash
# Verify KMS key access
aws kms decrypt \
  --ciphertext-blob fileb://encrypted.txt \
  --key-id alias/parameter-store
```

### Rate Limiting

SSM has lower rate limits than Secrets Manager:

1. **Use caching**: Cache parameter values locally
2. **Batch requests**: Use GetParameters for multiple values
3. **Implement backoff**: Exponential backoff for retries

## Security Considerations

### 1. Least Privilege Access

```json
{
  "Effect": "Allow",
  "Action": "ssm:GetParameter",
  "Resource": "arn:aws:ssm:region:account:parameter/myapp/prod/*",
  "Condition": {
    "StringEquals": {
      "ssm:resourceTag/Environment": "Production"
    }
  }
}
```

### 2. Encryption Best Practices

- Always use SecureString for sensitive data
- Use customer-managed KMS keys for additional control
- Rotate KMS keys regularly
- Audit key usage with CloudTrail

### 3. Parameter Access Logging

Enable CloudTrail logging:

```bash
# Check who accessed parameters
aws cloudtrail lookup-events \
  --lookup-attributes AttributeKey=EventName,AttributeValue=GetParameter \
  --query 'Events[?Resources[?ResourceName==`/myapp/prod/database/password`]]'
```

### 4. VPC Endpoints

Use VPC endpoints for private access:

```bash
aws ec2 create-vpc-endpoint \
  --vpc-id vpc-12345678 \
  --service-name com.amazonaws.region.ssm
```

## Integration Examples

### With EC2 User Data

```bash
#!/bin/bash
# Retrieve parameter in EC2 user data
DB_PASSWORD=$(aws ssm get-parameter \
  --name /myapp/prod/db/password \
  --with-decryption \
  --query 'Parameter.Value' \
  --output text)
```

### With Lambda

```python
import boto3
import os

ssm = boto3.client('ssm')

def lambda_handler(event, context):
    # Get parameter
    response = ssm.get_parameter(
        Name=os.environ['PARAMETER_NAME'],
        WithDecryption=True
    )
    secret_value = response['Parameter']['Value']
```

### With ECS/Fargate

Task definition using SSM parameters:

```json
{
  "family": "myapp",
  "containerDefinitions": [{
    "name": "app",
    "secrets": [{
      "name": "DB_PASSWORD",
      "valueFrom": "arn:aws:ssm:region:account:parameter/myapp/prod/db/password"
    }]
  }]
}
```

## Migration Guide

### From Environment Variables

```bash
# Export env vars to SSM
echo "$DATABASE_URL" | aws ssm put-parameter \
  --name /myapp/dev/database/url \
  --type SecureString \
  --value file:///dev/stdin
```

### From Secrets Manager

```bash
# Migrate from Secrets Manager to SSM
SECRET=$(aws secretsmanager get-secret-value \
  --secret-id prod/database \
  --query SecretString --output text)

aws ssm put-parameter \
  --name /myapp/prod/database/config \
  --type SecureString \
  --value "$SECRET"
```

### Bulk Import

```bash
# Import from JSON file
cat parameters.json | jq -r '.[] | 
  "aws ssm put-parameter --name \(.name) --value \(.value) --type \(.type)"' | 
  bash
```

## Performance Tips

### 1. Implement Caching

Cache parameters to reduce API calls:

```yaml
# Consider caching these values in your application
envs:
  production:
    # Static configuration
    FEATURE_FLAGS:
      from: /myapp/prod/features
      # Cache for 1 hour in your app
    
    # Sensitive data
    API_KEY:
      from: /myapp/prod/api-key
      # Cache for 5 minutes
```

### 2. Use GetParameters for Batch

```bash
# Efficient: Get multiple parameters
aws ssm get-parameters \
  --names /myapp/prod/db/host /myapp/prod/db/port /myapp/prod/db/name

# Inefficient: Multiple GetParameter calls
aws ssm get-parameter --name /myapp/prod/db/host
aws ssm get-parameter --name /myapp/prod/db/port
aws ssm get-parameter --name /myapp/prod/db/name
```

## Related Documentation

- [AWS Systems Manager Parameter Store](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-parameter-store.html)
- [Parameter Store Best Practices](https://docs.aws.amazon.com/systems-manager/latest/userguide/parameter-store-best-practices.html)
- [KMS Integration Guide](https://docs.aws.amazon.com/kms/latest/developerguide/services-parameter-store.html)
- [Parameter Store vs Secrets Manager](https://docs.aws.amazon.com/systems-manager/latest/userguide/integration-ps-secretsmanager.html)