# Research: Secret Rotation Strategies Across Providers

**Date**: 2025-08-19  
**Researcher**: Claude (AI Assistant)  
**Type**: Technical Research  
**Status**: Complete

## Executive Summary

Research reveals that not all secret providers support multiple active keys, necessitating multiple rotation strategies. Major providers like AWS IAM (2 keys) and Stripe support zero-downtime rotation, while GitHub/GitLab PATs require immediate replacement with potential downtime. This led to implementing three core strategies: two-key (zero downtime), immediate replacement, and overlap period.

## Research Questions

- Do all password/API key providers allow for the two-key rotation method?
- What rotation strategies do different providers support?
- How do we handle providers that don't support multiple active keys?
- What are the best practices for each provider type?

## Methodology

- Web research using Brave Search API
- Documentation review of major providers (AWS, Azure, GitHub, GitLab, Stripe, Datadog, Okta)
- Analysis of existing rotation implementations (Doppler, GitGuardian, CyberArk)
- Evaluation of provider capabilities and limitations

## Key Findings

### Finding 1: Provider Support Varies Significantly

**Description**: Providers have vastly different capabilities for supporting multiple active keys, ranging from strict limits to unlimited.

**Evidence**: 
- AWS IAM: "You can have a maximum of two access keys per user"
- Datadog: "Your org must have at least one API key and at most 50 API keys"
- GitHub PATs: Multiple tokens allowed but each is independent (not true key pairs)
- Azure Service Principals: Supports multiple client secrets with overlapping validity

**Source**: 
- [AWS IAM Documentation](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_access-keys.html)
- [Datadog API Keys Documentation](https://docs.datadoghq.com/account_management/api-app-keys/)
- [GitHub PAT Documentation](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)

### Finding 2: Zero-Downtime Rotation Requires Provider Support

**Description**: True zero-downtime rotation (two-key method) only works when providers support multiple simultaneously active credentials.

**Evidence**:
- Stripe documentation shows support for multiple API keys for rotation
- AWS explicitly designed 2-key limit for rotation purposes
- GitHub/GitLab require immediate replacement causing potential downtime

**Source**:
- [Stripe API Keys Documentation](https://docs.stripe.com/keys)
- [AWS Security Blog on Key Rotation](https://aws.amazon.com/blogs/security/how-to-rotate-access-keys-for-iam-users/)

### Finding 3: Industry Uses Multiple Rotation Patterns

**Description**: Different rotation strategies have emerged to handle various provider limitations and use cases.

**Evidence**:
- Doppler's two-secret strategy for zero downtime
- Azure's dual credential rotation tutorial
- Certificate rotation with overlap periods (7-30 days typical)
- OAuth refresh token patterns for token rotation

**Source**:
- [Azure Rotation Tutorial for Dual Credentials](https://learn.microsoft.com/en-us/azure/key-vault/secrets/tutorial-rotation-dual)
- [Shopify's GitHub Token Rotation](https://shopify.engineering/automatically-rotate-github-tokens)

## Analysis

The research reveals a fundamental challenge: the ideal two-key rotation strategy for zero downtime is not universally supported. This creates three distinct categories of providers:

1. **Two-Key Compatible** (AWS IAM, Stripe, Datadog): These providers explicitly support multiple active credentials, enabling true zero-downtime rotation.

2. **Version/Time-Based** (Certificates, some API keys): These support expiration dates or versioning, allowing overlap periods for gradual migration.

3. **Single-Key Limited** (GitHub PATs, GitLab PATs): These require immediate replacement, accepting brief downtime as a tradeoff for simplicity.

The industry has adapted by developing multiple strategies rather than forcing a one-size-fits-all approach. This aligns with the principle of "mechanism, not policy" - providing flexible tools that work with provider constraints.

## Implications for dsops

### Design Implications
- Must implement multiple rotation strategies, not just two-key
- Need provider capability detection to choose appropriate strategy
- Architecture must support composable strategies (wrapping base rotators)

### Feature Implications  
- Priority order: Two-key → Overlap → Immediate replacement
- Need clear warnings about downtime risks for immediate replacement
- Should implement strategy selection logic based on provider capabilities

### Market Implications
- Competitive advantage by supporting more providers than tools that only do two-key
- Clear documentation needed to explain strategy differences
- Position as "works with your existing providers" not "requires provider changes"

## Recommendations

### Immediate Actions
- [x] Implement three core strategies: two-key, immediate, overlap
- [x] Create ProviderCapabilities detection system
- [x] Document strategy selection guidelines
- [x] Add warnings for strategies with downtime risk

### Future Considerations
- [ ] Implement OAuth refresh strategy for modern APIs
- [ ] Add emergency rotation for compromised secrets
- [ ] Create provider-specific optimization guides
- [ ] Build strategy recommendation engine

## Sources

### Primary Sources
- AWS IAM User Guide - Access Keys section
- Stripe API Documentation - API Keys
- GitHub Docs - Personal Access Tokens
- Azure Key Vault - Rotation Tutorials
- Datadog Account Management - API Keys

### Secondary Sources  
- Shopify Engineering Blog - "Automatically Rotating GitHub Tokens"
- Reddit r/devops - "How do you rotate 3rd parties API keys?"
- GitGuardian Blog - "How to Become Great at API Key Rotation"
- howtorotate.com - Provider-specific rotation guides

### Tools/Platforms Evaluated
- AWS (IAM, Secrets Manager)
- Azure (Service Principals, Key Vault)
- GitHub (Personal Access Tokens)
- GitLab (Personal Access Tokens)
- Stripe (API Keys)
- Datadog (API Keys)
- Okta (Static tokens vs OAuth)
- PostgreSQL (Database users)

## Appendix

### Provider Capability Matrix

| Provider | Max Active Keys | Supports Expiration | Supports Versioning | Best Strategy |
|----------|----------------|-------------------|-------------------|--------------|
| AWS IAM | 2 | No | No | Two-Key |
| AWS Secrets Manager | Unlimited | Yes | Yes | Versioned/Overlap |
| Azure Service Principal | Unlimited | Yes | No | Two-Key/Overlap |
| GitHub PAT | Unlimited* | Yes | No | Immediate/Overlap |
| Stripe | Unlimited | No | No | Two-Key |
| Datadog | 50 | No | No | Two-Key |
| PostgreSQL User | 1 | No | No | Immediate |
| Certificates | N/A | Yes | No | Overlap |

*Multiple independent tokens, not true active/inactive pairs

### Strategy Decision Tree

```
Provider supports 2+ active keys?
├─ Yes → Use Two-Key Strategy (zero downtime)
└─ No → Secret has expiration/validity period?
    ├─ Yes → Use Overlap Strategy (gradual migration)
    └─ No → Use Immediate Strategy (accept brief downtime)
```

## Follow-up Questions

- How do we handle hybrid scenarios (e.g., database with read-only replicas)?
- Should we support custom rotation strategies via plugins?
- How do we measure and minimize downtime for immediate replacement?
- Can we auto-detect provider capabilities or need manual configuration?
- How do we handle rotation failures mid-process?

---

**Document History**:
- 2025-08-19: Initial research and documentation
- 2025-08-19: Added provider capability matrix and decision tree