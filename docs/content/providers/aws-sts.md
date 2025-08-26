---
title: "AWS STS (Security Token Service)"
description: "Use AWS STS for temporary security credentials and cross-account access"
lead: "AWS Security Token Service (STS) enables you to request temporary, limited-privilege credentials for AWS Identity and Access Management (IAM) users or for users that you authenticate (federated users)."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 30
---

## Overview

AWS STS is a web service that enables you to request temporary, limited-privilege credentials. It's particularly useful for:

- **Cross-Account Access**: Access resources in other AWS accounts
- **Temporary Credentials**: Short-lived credentials that auto-expire
- **MFA-Protected Access**: Require MFA for sensitive operations
- **Federated Access**: Grant access to users authenticated outside AWS
- **Least Privilege**: Scope down permissions with session policies

## Features

- **AssumeRole**: Most common operation for cross-account access
- **Session Duration**: 15 minutes to 12 hours (configurable)
- **MFA Support**: Require multi-factor authentication
- **Session Policies**: Further restrict permissions
- **External ID**: Additional security for third-party access
- **Session Tags**: Pass attributes to control access

## Use Cases

1. **Cross-Account Access**: Access resources in another AWS account
2. **CI/CD Pipelines**: Temporary credentials for deployments
3. **Service-to-Service**: One service accessing another's resources
4. **Developer Access**: Time-limited access to production resources
5. **Third-Party Integration**: Secure access for external services

## Configuration

### Basic AssumeRole

```yaml
version: 1

secretStores:
  # STS provider for cross-account access
  aws-sts-prod:
    type: aws.sts
    region: us-east-1
    role_arn: arn:aws:iam::123456789012:role/ProdAccessRole
    role_session_name: dsops-session
    # Optional: session duration (seconds)
    duration_seconds: 3600

  # Access secrets using assumed role credentials
  aws-secrets-prod:
    type: aws.secretsmanager
    region: us-east-1
    # Use credentials from STS provider
    credentials_from: aws-sts-prod

envs:
  production:
    DATABASE_URL:
      from:
        store: aws-secrets-prod
        key: prod/database/connection
```

### With External ID

For third-party access scenarios:

```yaml
secretStores:
  aws-sts-customer:
    type: aws.sts
    region: us-east-1
    role_arn: arn:aws:iam::987654321098:role/VendorAccess
    role_session_name: vendor-dsops
    external_id: unique-customer-id-12345
    duration_seconds: 1800
```

### With MFA

Require multi-factor authentication:

```yaml
secretStores:
  aws-sts-sensitive:
    type: aws.sts
    region: us-east-1
    role_arn: arn:aws:iam::123456789012:role/SensitiveAccess
    role_session_name: mfa-required-session
    # MFA device serial number
    serial_number: arn:aws:iam::123456789012:mfa/username
    # MFA token code (from environment variable)
    token_code: ${MFA_TOKEN}
```

### With Session Policy

Apply additional restrictions:

```yaml
secretStores:
  aws-sts-restricted:
    type: aws.sts
    region: us-east-1
    role_arn: arn:aws:iam::123456789012:role/PowerUserRole
    role_session_name: restricted-session
    # Inline session policy to limit permissions
    session_policy: |
      {
        "Version": "2012-10-17",
        "Statement": [{
          "Effect": "Allow",
          "Action": [
            "secretsmanager:GetSecretValue"
          ],
          "Resource": "arn:aws:secretsmanager:*:*:secret:prod/*"
        }]
      }
```

## Role Configuration

### Trust Policy (in target account)

The role being assumed must trust the source:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "arn:aws:iam::098765432109:root"
      },
      "Action": "sts:AssumeRole",
      "Condition": {
        "StringEquals": {
          "sts:ExternalId": "unique-external-id"
        },
        "IpAddress": {
          "aws:SourceIp": "203.0.113.0/24"
        }
      }
    }
  ]
}
```

### Role Permissions

The role needs appropriate permissions:

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
      "Resource": "arn:aws:secretsmanager:*:*:secret:prod/*"
    }
  ]
}
```

## Advanced Patterns

### Chain of Roles

Assume role through multiple hops:

```yaml
secretStores:
  # First hop: assume jump role
  aws-sts-jump:
    type: aws.sts
    region: us-east-1
    role_arn: arn:aws:iam::111111111111:role/JumpRole
    role_session_name: jump-session

  # Second hop: from jump to target
  aws-sts-target:
    type: aws.sts
    region: us-east-1
    role_arn: arn:aws:iam::222222222222:role/TargetRole
    role_session_name: target-session
    credentials_from: aws-sts-jump
```

### Dynamic Role Selection

Use environment variables for flexibility:

```yaml
secretStores:
  aws-sts-dynamic:
    type: aws.sts
    region: ${AWS_REGION}
    role_arn: ${TARGET_ROLE_ARN}
    role_session_name: dsops-${USER}-${TIMESTAMP}
    external_id: ${EXTERNAL_ID}
```

### Multi-Region Access

```yaml
secretStores:
  # US East role
  aws-sts-us-east:
    type: aws.sts
    region: us-east-1
    role_arn: arn:aws:iam::123456789012:role/RegionalRole

  # EU West role
  aws-sts-eu-west:
    type: aws.sts
    region: eu-west-1
    role_arn: arn:aws:iam::123456789012:role/RegionalRole

  # Secrets in different regions
  secrets-us:
    type: aws.secretsmanager
    region: us-east-1
    credentials_from: aws-sts-us-east

  secrets-eu:
    type: aws.secretsmanager
    region: eu-west-1
    credentials_from: aws-sts-eu-west
```

## Session Management

### Session Duration

Different services have different maximum durations:

| Service | Default | Maximum |
|---------|---------|----------|
| AssumeRole | 1 hour | 12 hours |
| AssumeRoleWithSAML | 1 hour | 12 hours |
| AssumeRoleWithWebIdentity | 1 hour | 12 hours |
| GetSessionToken | 12 hours | 36 hours |

### Credential Caching

dsops handles credential caching automatically:

```yaml
secretStores:
  aws-sts-cached:
    type: aws.sts
    role_arn: arn:aws:iam::123456789012:role/AppRole
    role_session_name: cached-session
    duration_seconds: 3600  # 1 hour
    # Credentials cached until ~15 min before expiry
```

## Security Best Practices

### 1. Use External ID

Always use external ID for third-party access:

```yaml
secretStores:
  aws-sts-secure:
    type: aws.sts
    role_arn: arn:aws:iam::123456789012:role/ThirdPartyAccess
    external_id: ${EXTERNAL_ID}  # Keep this secret!
```

### 2. Principle of Least Privilege

Use session policies to limit permissions:

```yaml
session_policy: |
  {
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Deny",
      "Action": ["iam:*", "sts:*"],
      "Resource": "*"
    }]
  }
```

### 3. Require MFA for Sensitive Operations

```yaml
secretStores:
  aws-sts-mfa:
    type: aws.sts
    role_arn: arn:aws:iam::123456789012:role/AdminAccess
    serial_number: arn:aws:iam::123456789012:mfa/admin
    token_code: ${MFA_CODE}
```

### 4. Use Condition Keys

Restrict based on conditions:

```json
{
  "Condition": {
    "IpAddress": {
      "aws:SourceIp": ["10.0.0.0/8", "172.16.0.0/12"]
    },
    "StringEquals": {
      "aws:userid": "AIDAI23HXD3MBVD4EFPWX"
    }
  }
}
```

## Troubleshooting

### Access Denied Errors

```bash
# Test assume role
aws sts assume-role \
  --role-arn arn:aws:iam::123456789012:role/TestRole \
  --role-session-name test-session

# Check who you are
aws sts get-caller-identity
```

### Trust Policy Issues

Common trust policy mistakes:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {
      // Wrong: specific user
      "AWS": "arn:aws:iam::098765432109:user/alice"
      // Right: allow anyone in account (controlled by IAM)
      "AWS": "arn:aws:iam::098765432109:root"
    },
    "Action": "sts:AssumeRole"
  }]
}
```

### Session Duration Errors

```yaml
# Error: duration exceeds maximum
aws-sts-long:
  type: aws.sts
  role_arn: arn:aws:iam::123456789012:role/AppRole
  duration_seconds: 86400  # 24 hours - TOO LONG!

# Correct: within limits
aws-sts-valid:
  type: aws.sts
  role_arn: arn:aws:iam::123456789012:role/AppRole
  duration_seconds: 43200  # 12 hours - OK
```

### External ID Mismatch

```bash
# Debug external ID issues
aws sts assume-role \
  --role-arn arn:aws:iam::123456789012:role/TestRole \
  --role-session-name debug \
  --external-id "test-external-id"
```

## Integration Examples

### With CI/CD

GitHub Actions example:

```yaml
# .github/workflows/deploy.yml
env:
  ROLE_ARN: arn:aws:iam::123456789012:role/GitHubActions

steps:
  - name: Configure AWS Credentials
    uses: aws-actions/configure-aws-credentials@v1
    with:
      role-to-assume: ${{ env.ROLE_ARN }}
      
  - name: Deploy with dsops
    run: |
      dsops exec --env production -- deploy.sh
```

### With Lambda

```python
import boto3

def lambda_handler(event, context):
    # Assume role from Lambda
    sts = boto3.client('sts')
    assumed_role = sts.assume_role(
        RoleArn='arn:aws:iam::123456789012:role/CrossAccountRole',
        RoleSessionName='lambda-session'
    )
    
    # Use assumed credentials
    credentials = assumed_role['Credentials']
    secrets_client = boto3.client(
        'secretsmanager',
        aws_access_key_id=credentials['AccessKeyId'],
        aws_secret_access_key=credentials['SecretAccessKey'],
        aws_session_token=credentials['SessionToken']
    )
```

### With ECS/Fargate

Task role configuration:

```json
{
  "family": "my-app",
  "taskRoleArn": "arn:aws:iam::123456789012:role/EcsTaskRole",
  "executionRoleArn": "arn:aws:iam::123456789012:role/EcsExecutionRole",
  "containerDefinitions": [{
    "name": "app",
    "environment": [{
      "name": "ASSUME_ROLE_ARN",
      "value": "arn:aws:iam::987654321098:role/CrossAccountAccess"
    }]
  }]
}
```

## Common Patterns

### Development to Production

```yaml
# Development account assumes production role
secretStores:
  prod-access:
    type: aws.sts
    role_arn: arn:aws:iam::${PROD_ACCOUNT_ID}:role/DeveloperAccess
    role_session_name: ${USER}-dev-to-prod
    duration_seconds: 3600  # 1 hour limit for dev access
```

### Service Account Pattern

```yaml
# Service assuming role in customer account
secretStores:
  customer-access:
    type: aws.sts
    role_arn: ${CUSTOMER_ROLE_ARN}
    role_session_name: service-${SERVICE_NAME}-${TIMESTAMP}
    external_id: ${CUSTOMER_EXTERNAL_ID}
```

### Break Glass Access

```yaml
# Emergency access with MFA
secretStores:
  break-glass:
    type: aws.sts
    role_arn: arn:aws:iam::123456789012:role/EmergencyAccess
    role_session_name: emergency-${USER}-${TIMESTAMP}
    serial_number: ${MFA_DEVICE_ARN}
    token_code: ${MFA_TOKEN}
    duration_seconds: 900  # 15 minutes only
```

## Monitoring and Audit

### CloudTrail Events

Monitor STS API calls:

```json
{
  "eventName": "AssumeRole",
  "userIdentity": {
    "type": "IAMUser",
    "principalId": "AIDAI23HXD3MBVD4EFPWX",
    "arn": "arn:aws:iam::098765432109:user/alice"
  },
  "requestParameters": {
    "roleArn": "arn:aws:iam::123456789012:role/ProdAccess",
    "roleSessionName": "alice-session"
  }
}
```

### Metrics to Monitor

1. **AssumeRole Failures**: Access denied or trust policy issues
2. **Session Duration**: Track how long sessions are used
3. **Unique Principals**: Who's assuming roles
4. **Cross-Account Access**: Which accounts are accessed

## Cost Considerations

STS API calls are free, but consider:

1. **CloudTrail Costs**: If logging all STS events
2. **Indirect Costs**: Actions performed with assumed credentials
3. **Rate Limits**: Respect API throttling limits

## Related Documentation

- [AWS STS Documentation](https://docs.aws.amazon.com/STS/latest/APIReference/)
- [AssumeRole Best Practices](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use.html)
- [Session Policies](https://docs.aws.amazon.com/IAM/latest/UserGuide/access_policies.html#policies_session)
- [External ID Guide](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_create_for-user_externalid.html)