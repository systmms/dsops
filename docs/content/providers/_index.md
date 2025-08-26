---
title: "Providers"
description: "Supported secret providers and their configuration"
lead: "dsops integrates with popular secret management tools and cloud providers. Choose the providers that fit your workflow."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 20
---

## Supported Providers

dsops currently supports 14+ secret storage providers across different categories:

### Password Managers
- [Bitwarden](/providers/bitwarden/) - Open source password manager with team features
- [1Password](/providers/1password/) - Popular team password management solution
- [pass](/providers/pass/) - Unix password store using GPG encryption

### AWS Secret Stores
- [AWS Secrets Manager](/providers/aws-secrets-manager/) - Native AWS secret storage with rotation
- [AWS Systems Manager (SSM)](/providers/aws-ssm/) - Parameter store with SecureString support
- [AWS STS](/providers/aws-sts/) - Temporary security credentials via role assumption
- [AWS IAM Identity Center](/providers/aws-sso/) - Centralized AWS account access (formerly SSO)
- [AWS Unified Provider](/providers/aws-unified/) - Smart routing to appropriate AWS service

### Google Cloud
- [Google Secret Manager](/providers/google-secret-manager/) - GCP native secret storage
- [GCP Unified Provider](/providers/gcp-unified/) - Intelligent GCP service selection

### Microsoft Azure  
- [Azure Key Vault](/providers/azure-key-vault/) - Azure's secret and key management
- [Azure Managed Identity](/providers/azure-identity/) - Passwordless Azure authentication
- [Azure Unified Provider](/providers/azure-unified/) - Automatic Azure service routing

### Enterprise Solutions
- [HashiCorp Vault](/providers/vault/) - Enterprise secret management platform
- [Doppler](/providers/doppler/) - Developer-first secrets platform

### Development & Testing
- [Literal Provider](/providers/literal/) - Static values for testing and development

## Provider Capabilities

Each provider supports different features:

| Provider | Type | Auth Methods | Rotation | Versioning | Free Tier |
|----------|------|--------------|----------|------------|-----------|
| **Password Managers** |
| Bitwarden | CLI/API | Password, API Key | ✅ | ✅ | ✅ |
| 1Password | CLI | Biometric, Token | ✅ | ✅ | ❌ |
| pass | Local | GPG Key | ❌ | Git | ✅ |
| **Cloud Providers** |
| AWS Secrets Manager | SDK | IAM, STS | ✅ | ✅ | 💰 |
| AWS SSM | SDK | IAM, STS | ❌ | ✅ | ✅ |
| Google Secret Manager | SDK | Service Account, ADC | ✅ | ✅ | 💰 |
| Azure Key Vault | SDK | MSI, Service Principal | ✅ | ✅ | 💰 |
| **Enterprise** |
| HashiCorp Vault | API | Token, AppRole, K8s | ✅ | ✅ | ✅ OSS |
| Doppler | API | API Token | ✅ | ✅ | 💰 |

Legend: ✅ Full support | 💰 Usage-based pricing | ❌ Not available

## Choosing a Provider

Consider these factors:

1. **Team Size**: Some providers are better suited for individuals vs enterprises
2. **Cloud Integration**: Native cloud providers integrate well with their platforms
3. **Rotation Support**: Not all providers support automated rotation
4. **Pricing**: Consider both seat-based and usage-based pricing
5. **Compliance**: Some providers offer compliance certifications

## Multi-Provider Setup

You can use multiple providers simultaneously:

```yaml
providers:
  bitwarden:
    type: bitwarden
  
  aws:
    type: aws.secretsmanager
    region: us-east-1

envs:
  production:
    # Mix providers as needed
    DATABASE_URL:
      from: { provider: aws, key: "/prod/database/url" }
    
    STRIPE_KEY:
      from: { provider: bitwarden, key: "stripe-prod-key" }
```