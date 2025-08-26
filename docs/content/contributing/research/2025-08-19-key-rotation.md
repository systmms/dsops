# Key Rotation Approaches Research

**Date**: 2025-08-19  
**Researcher**: Claude (AI Assistant)  
**Type**: Technical Research  
**Status**: Complete

## Executive Summary

SOPS implements both data key (DEK) and master key (KEK) rotation, not just encryption key rotation as initially assumed. The `rotate` command generates new data keys and re-encrypts all values, while `updatekeys` only modifies key access. This has significant implications for dsops rotation architecture, requiring support for both types of rotation patterns.

## Research Questions

- How does SOPS actually handle key rotation?
- What's the difference between data key and encryption key rotation?
- Do we need to support both secret value rotation and encryption key rotation?
- What rotation patterns do other tools implement?

## Methodology

- Web search for SOPS documentation and implementation details
- Analysis of envelope encryption patterns
- Review of cloud provider KMS rotation approaches
- Evaluation of rotation strategies across platforms

## Key Findings

### Finding 1: SOPS Rotates Data Keys, Not Just Master Keys
**Description**: SOPS `rotate` command creates new data encryption keys (DEK) and re-encrypts all secret values.
**Evidence**: 
- "The rotate command generates a new data encryption key and reencrypt all values with the new key"
- Each file uses single 256-bit data key encrypted by multiple master keys
- Rotation involves complete re-encryption of all values
**Source**: 
- https://github.com/getsops/sops
- SOPS documentation on rotation internals

### Finding 2: Two Distinct SOPS Commands
**Description**: SOPS provides separate commands for different rotation scenarios.
**Evidence**:
- `updatekeys`: Only changes which master keys can decrypt (KEK modification)
- `rotate`: Generates new data key and re-encrypts everything (DEK rotation)
- "Use updatekeys if you want to add a key without rotating the data key"
**Source**: SOPS GitHub issues and documentation

### Finding 3: Envelope Encryption Architecture
**Description**: SOPS implements standard envelope encryption pattern.
**Evidence**:
- Master keys (KEK) encrypt data key (DEK)  
- Data key encrypts actual secret values
- Each value gets unique initialization vector
- AES256_GCM used for encryption
**Source**: SOPS implementation documentation

### Finding 4: Cloud Provider KMS Rotation
**Description**: Cloud providers handle KEK rotation differently than DEK rotation.
**Evidence**:
- AWS KMS: Automatic yearly rotation of master keys
- GCP Cloud KMS: Configurable rotation periods (1 day to 100 years)
- Azure Key Vault: Minimum 28-day rotation intervals
- All maintain previous key versions for decryption
**Source**: Cloud provider documentation

### Finding 5: Security Best Practices
**Description**: Industry recommends different rotation frequencies based on risk.
**Evidence**:
- "When removing keys, it is recommended to rotate the data key using -r"
- NIST recommends rotation based on encryption operations (2^32 limit)
- Microsoft recommends 2-year maximum key lifetime
**Source**: Security best practices documentation

## Analysis

The research reveals that rotation is more complex than initially understood:

1. **Two-Layer Architecture**: Both DEK and KEK rotation serve different purposes
2. **Security Trade-offs**: DEK rotation provides stronger security but higher operational cost
3. **Tool Complexity**: SOPS handles significant complexity behind simple commands
4. **Enterprise Needs**: Organizations need both types of rotation for different scenarios

The distinction between encryption key rotation and secret value rotation is crucial:
- **Encryption Key Rotation**: Changing the keys that protect secrets (SOPS focus)
- **Secret Value Rotation**: Changing the actual secret content (database passwords, API keys)

## Implications for dsops

### Design Implications
- Must support both DEK and KEK rotation patterns
- Need clear distinction between encryption rotation and value rotation
- Should implement envelope encryption for file-based secrets
- Require rollback and verification capabilities

### Feature Implications
- `dsops keys rotate` for encryption key rotation (like SOPS)
- `dsops secrets rotate` for secret value rotation (original vision)
- Provider-specific rotation strategies for each type
- Integration with cloud KMS rotation APIs

### Market Implications
- Complexity creates barrier to entry for competitors
- Significant engineering investment required
- Enterprise market values comprehensive rotation capabilities
- Developer market needs simple interfaces over complex functionality

## Recommendations

### Immediate Actions
- [x] Update VISION_ROTATE.md to clarify rotation types
- [x] Document distinction between DEK and secret value rotation
- [ ] Design dual rotation architecture for dsops
- [ ] Create proof-of-concept for envelope encryption

### Future Considerations
- [ ] Implement SOPS-compatible file encryption rotation
- [ ] Build cloud KMS rotation integrations
- [ ] Create rotation strategy decision tree for users
- [ ] Develop comprehensive testing framework

## Sources

### Primary Sources
- SOPS GitHub repository and documentation
- Cloud provider KMS documentation (AWS, GCP, Azure)
- NIST Cybersecurity Framework guidelines

### Secondary Sources
- Security best practices documentation
- Academic papers on envelope encryption
- Industry blog posts on key rotation

### Tools/Platforms Evaluated
- Mozilla SOPS
- AWS KMS
- Google Cloud KMS
- Azure Key Vault
- HashiCorp Vault Transit Engine

## Appendix

### SOPS Rotation Process Detail
```
1. sops rotate command execution
2. Generate new 256-bit AES data key
3. Decrypt all values with old data key
4. Re-encrypt all values with new data key
5. Encrypt new data key with all master keys
6. Update file metadata with new encrypted data key
7. Preserve old master key access (unless removed)
```

### Envelope Encryption Pattern
```
Master Key (AWS KMS) → Encrypts → Data Key → Encrypts → Secret Values
     ↓                              ↓              ↓
  KEK Rotation                   DEK Rotation   Value Rotation
(Cloud provider)               (SOPS rotate)  (dsops rotate)
```

### Rotation Frequency Recommendations
| Risk Level | KEK Rotation | DEK Rotation | Value Rotation |
|------------|--------------|--------------|----------------|
| High | 90 days | 30 days | Weekly |
| Medium | 1 year | 90 days | Monthly |
| Low | 2 years | 1 year | Quarterly |

## Follow-up Questions

- How should dsops handle partial rotation failures?
- What rollback strategies work best for each rotation type?
- How can we simplify the mental model for users?
- What testing frameworks exist for rotation validation?

---

**Document History**:
- 2025-08-19: Initial research on SOPS rotation
- 2025-08-19: Added envelope encryption analysis
- 2025-08-19: Final version with architecture implications