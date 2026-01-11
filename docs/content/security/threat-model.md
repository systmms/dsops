---
title: "Threat Model"
description: "Understanding what dsops protects against and its security boundaries"
weight: 10
---

# Threat Model

This document describes the threats that dsops is designed to mitigate and the boundaries of its security model. Understanding these boundaries helps you make informed decisions about your security posture.

## Threat Categories

### Threats We Mitigate

dsops provides defense-in-depth against these common attack vectors:

#### 1. Disk Residue

**Threat**: Secrets written to `.env` files or configuration can persist on disk, be backed up, or end up in version control.

**Mitigation**:
- `dsops exec` injects secrets directly into process environments without writing to disk
- Ephemeral-first design discourages file-based workflows
- Warning messages when using `--out` flag

**Protection Level**: High - Primary design goal

#### 2. Log Exposure

**Threat**: Secrets accidentally printed in logs, CI output, or error messages.

**Mitigation**:
- All logging uses `logging.Secret()` wrapper for automatic redaction
- Debug output never shows secret values
- Error messages designed to be helpful without revealing secrets

**Protection Level**: High - Built into all logging paths

#### 3. Process Snooping

**Threat**: Parent process memory examined to extract secrets after passing them to child.

**Mitigation**:
- Secrets zeroed from parent memory after injection to child process
- Child process isolation through OS process boundaries
- Parent environment never contains secrets (only child's does)

**Protection Level**: Medium - Best effort zeroing, limited by Go runtime

#### 4. Memory Dumps

**Threat**: Core dumps or memory forensics reveal secret values.

**Mitigation**:
- Memory protection via memguard (mlock prevents swapping)
- Encryption at rest in memory when secrets not actively used
- Explicit memory zeroing on destruction

**Protection Level**: Medium - Depends on platform mlock configuration

#### 5. Supply Chain Attacks

**Threat**: Compromised binaries or Docker images distributed to users.

**Mitigation**:
- All releases signed with Sigstore cosign (keyless)
- Signatures recorded in Rekor transparency log
- SBOM (Software Bill of Materials) for dependency visibility
- GitHub Actions OIDC identity verification

**Protection Level**: High - Cryptographically verifiable

#### 6. Dependency Vulnerabilities

**Threat**: Vulnerable dependencies introduce security issues.

**Mitigation**:
- SBOM enables dependency auditing
- `govulncheck` integrated in CI pipeline
- `gosec` static analysis for security issues
- Regular dependency updates

**Protection Level**: Medium - Depends on timely updates

---

### Threats We Do NOT Mitigate

dsops has explicit boundaries. The following threats require additional measures:

#### 1. Compromised Secret Store

**Threat**: Attacker compromises 1Password, AWS Secrets Manager, Vault, etc.

**Why Not Protected**:
- Provider security is outside dsops's control
- dsops faithfully retrieves whatever the provider returns

**Your Responsibility**:
- Secure your secret store access (IAM, MFA, audit logs)
- Rotate secrets regularly
- Use provider's security features (encryption, access controls)

#### 2. Insider with Root Access

**Threat**: Attacker with root/administrator access to the machine running dsops.

**Why Not Protected**:
- Root can read any process's memory
- Root can attach debuggers, modify binaries
- No software can defend against root

**Your Responsibility**:
- Limit root access
- Use security hardening
- Monitor privileged access

#### 3. Hardware Attacks

**Threat**: Cold boot attacks, DMA attacks, hardware keyloggers.

**Why Not Protected**:
- Physical access bypasses software security
- Hardware-level attacks require hardware solutions

**Your Responsibility**:
- Physical security
- Full disk encryption
- Hardware security modules (HSM) for high-value secrets

#### 4. Side-Channel Attacks (Spectre/Meltdown)

**Threat**: CPU vulnerabilities that leak data across security boundaries.

**Why Not Protected**:
- OS and hardware responsibility
- Requires microcode updates and kernel patches

**Your Responsibility**:
- Keep systems patched
- Apply security updates promptly

#### 5. Malicious Child Process

**Threat**: The application you run with `dsops exec` is itself malicious.

**Why Not Protected**:
- dsops intentionally passes secrets to the child process
- Cannot distinguish legitimate use from malicious exfiltration

**Your Responsibility**:
- Trust your applications
- Audit third-party dependencies
- Use least-privilege secret access

#### 6. Network Interception

**Threat**: Man-in-the-middle attacks on connections to secret stores.

**Why Not Protected**:
- Provider SDK responsibility
- dsops uses providers' official SDKs with their TLS implementation

**Your Responsibility**:
- Ensure network security
- Use private networks where possible
- Verify provider TLS configuration

---

## Trust Boundaries

```
┌──────────────────────────────────────────────────────────────────────────┐
│                         TRUSTED BOUNDARY                                  │
│                                                                           │
│  ┌─────────────────┐                          ┌────────────────────┐     │
│  │  Secret Stores  │◀──── TLS ────────────────│    dsops CLI       │     │
│  │  (1Password,    │                          │                    │     │
│  │   AWS, Vault)   │                          │  • Fetches secrets │     │
│  └─────────────────┘                          │  • Transforms      │     │
│         │                                     │  • Injects to      │     │
│         │                                     │    child process   │     │
│         │                                     └─────────┬──────────┘     │
│         │                                               │                 │
│         │ Provider's responsibility                     │ dsops's        │
│         │ for authentication and                        │ responsibility │
│         │ authorization                                 │ for memory     │
│         │                                               │ and process    │
│         │                                               │ isolation      │
│         ▼                                               ▼                 │
│  ┌─────────────────┐                          ┌────────────────────┐     │
│  │  Provider IAM   │                          │   Child Process    │     │
│  │  (access control│                          │   (your app)       │     │
│  │   audit logs)   │                          │                    │     │
│  └─────────────────┘                          └────────────────────┘     │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
                                    │
                                    │
                                    ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                       UNTRUSTED / OUT OF SCOPE                            │
│                                                                           │
│  • Physical access to machines                                            │
│  • Root/administrator access                                              │
│  • Compromised operating system                                           │
│  • Hardware vulnerabilities                                               │
│  • Network infrastructure                                                 │
│                                                                           │
└──────────────────────────────────────────────────────────────────────────┘
```

## Risk Assessment Matrix

| Threat | Likelihood | Impact | Mitigation Status |
|--------|------------|--------|-------------------|
| Secrets in logs | High | High | **Mitigated** |
| Secrets on disk | High | High | **Mitigated** |
| Memory dumps | Medium | High | **Partially Mitigated** |
| Supply chain | Medium | Critical | **Mitigated** |
| Compromised provider | Low | Critical | Out of scope |
| Root access | Low | Critical | Out of scope |
| Hardware attacks | Very Low | Critical | Out of scope |

## Recommendations by Environment

### Development

- Use `dsops exec` for local development
- Don't commit `.env` files
- Use separate development secrets from production

### CI/CD

- Use OIDC-based secret injection where possible
- Verify dsops binary signatures before use
- Audit pipeline access controls

### Production

- Configure mlock limits for memory protection
- Use ephemeral execution exclusively
- Monitor secret access via provider audit logs
- Rotate secrets on a schedule

## See Also

- [Security Architecture](../architecture/) - Technical implementation details
- [Verify Releases](../verify-releases/) - How to verify release authenticity
