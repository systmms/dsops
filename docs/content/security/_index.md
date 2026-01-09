---
title: "Security"
description: "Security documentation for dsops"
weight: 60
---

# Security

dsops is a secrets management tool designed with security as a core principle. This section documents our security model, threat mitigations, and how you can verify the authenticity of releases.

## Quick Links

- **[Security Architecture](architecture/)** - How dsops protects your secrets at every stage
- **[Threat Model](threat-model/)** - What dsops protects against (and doesn't)
- **[Verify Releases](verify-releases/)** - How to verify release authenticity with cosign

## Security Philosophy

dsops follows these core security principles:

1. **Ephemeral-First**: Secrets should exist in memory only for as long as needed
2. **Defense in Depth**: Multiple layers of protection reduce risk
3. **Fail Secure**: When something goes wrong, err on the side of caution
4. **Transparency**: Open source code and signed releases enable verification

## Report a Vulnerability

If you discover a security vulnerability in dsops, please report it responsibly:

- **Preferred**: [GitHub Security Advisories](https://github.com/systmms/dsops/security/advisories)
- **Email**: security@systmms.com

See our [SECURITY.md](https://github.com/systmms/dsops/blob/main/SECURITY.md) for the complete vulnerability disclosure policy, including response timelines and what to expect.

## Security Features at a Glance

| Feature | Description |
|---------|-------------|
| **Ephemeral Execution** | `dsops exec` injects secrets without writing to disk |
| **Log Redaction** | Automatic masking of secret values in all logs |
| **Memory Protection** | Secrets protected from memory dumps via mlock |
| **Signed Releases** | All artifacts signed with Sigstore cosign |
| **SBOM** | Software Bill of Materials for dependency transparency |
| **Process Isolation** | Child processes receive secrets, parent zeros them |

## Getting Started with Security

1. **Verify your download**: Follow our [release verification guide](verify-releases/)
2. **Use ephemeral execution**: Run `dsops exec` instead of writing env files
3. **Configure mlock**: See [architecture docs](architecture/#platform-configuration) for platform-specific setup
4. **Report issues responsibly**: Use our [security policy](https://github.com/systmms/dsops/blob/main/SECURITY.md)
