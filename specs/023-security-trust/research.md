# Research: Security Trust Infrastructure

**Date**: 2026-01-09
**Branch**: 023-security-trust

## Research Tasks

### 1. Cosign Keyless Signing with GitHub Actions

**Decision**: Use cosign keyless signing via Sigstore OIDC

**Rationale**:
- No key management required (keys are ephemeral, generated per-signing)
- GitHub Actions provides OIDC identity automatically
- Signatures recorded in Rekor transparency log for auditability
- Industry standard for open source projects (Kubernetes, etc.)

**Implementation**:
```yaml
# .github/workflows/release.yml additions
permissions:
  id-token: write  # Required for OIDC

steps:
  - name: Install cosign
    uses: sigstore/cosign-installer@v3

  - name: Sign checksums
    run: cosign sign-blob --yes dist/*_checksums.txt > dist/checksums.sig
    env:
      COSIGN_EXPERIMENTAL: "1"  # Enable keyless

  - name: Sign Docker image
    run: cosign sign --yes ghcr.io/systmms/dsops:${{ github.ref_name }}
```

**Verification command** (for users):
```bash
# Verify blob (checksums file)
cosign verify-blob \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  dist/dsops_*_checksums.txt

# Verify Docker image
cosign verify \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/systmms/dsops:latest
```

**Alternatives Considered**:
- GPG signing: Requires key management, less transparent
- Custom PKI: Complex to manage, not industry standard
- No signing: Unacceptable for security-focused tool

---

### 2. SBOM Generation with GoReleaser

**Decision**: Use GoReleaser's built-in syft integration for SPDX format

**Rationale**:
- GoReleaser v2 has native SBOM support via syft
- SPDX is widely adopted (Linux Foundation standard)
- Integrates seamlessly with existing release workflow
- No additional tooling required

**Implementation**:
```yaml
# .goreleaser.yml additions
sboms:
  - artifacts: archive
    documents:
      - "{{ .ProjectName }}_{{ .Version }}_sbom.spdx.json"
```

**Output**: Each release will include `dsops_X.Y.Z_sbom.spdx.json` containing all Go dependencies.

**Alternatives Considered**:
- CycloneDX format: Less widely adopted than SPDX
- Manual syft invocation: More complex, GoReleaser integration is cleaner
- No SBOM: Unacceptable for supply chain transparency

---

### 3. Memory Protection with memguard

**Decision**: Use memguard for secure memory handling with graceful degradation fallback

**Rationale**:
- Pure Go implementation (no CGO required)
- Provides encryption at rest in memory (XSalsa20Poly1305)
- Prevents swapping via mlock
- Guard pages detect buffer overflows
- High reputation in security community
- Used by similar security tools

**Key Features** (from [memguard](https://github.com/awnumar/memguard)):
- Encrypts and authenticates sensitive data in memory
- Bypasses Go GC by using direct system calls
- Memory locked to prevent swapping to disk
- Guard pages and canary values for overflow detection
- Constant-time operations to prevent timing attacks
- Core dump protection

**Implementation Approach**:

```go
// internal/secure/enclave.go
package secure

import "github.com/awnumar/memguard"

// SecureBuffer wraps memguard for secret storage
type SecureBuffer struct {
    enclave *memguard.Enclave
}

// NewSecureBuffer creates a protected buffer from secret bytes
func NewSecureBuffer(data []byte) (*SecureBuffer, error) {
    enclave := memguard.NewEnclave(data)
    return &SecureBuffer{enclave: enclave}, nil
}

// Open returns the plaintext for use (caller must call Close)
func (s *SecureBuffer) Open() (*memguard.LockedBuffer, error) {
    return s.enclave.Open()
}

// Destroy securely wipes the memory
func (s *SecureBuffer) Destroy() {
    s.enclave.Destroy()
}
```

**Integration Points**:
1. `internal/resolve/resolver.go` - Wrap resolved secret values
2. `internal/execenv/exec.go` - Zero after injection to child process

**Platform Behavior**:
| Platform | mlock | Guard Pages | Notes |
|----------|-------|-------------|-------|
| Linux | ✅ Yes (RLIMIT_MEMLOCK) | ✅ Yes | May need `ulimit -l` increase |
| macOS | ✅ Yes | ✅ Yes | Works out of box |
| Windows | ✅ Yes (VirtualLock) | ✅ Yes | Works out of box |

**Fallback Strategy**:
If memguard fails to allocate locked memory:
1. Log warning with `logging.Warn()`
2. Continue with standard Go memory (best-effort)
3. Document in `dsops doctor` output

**Alternatives Considered**:
- Manual mlock: Lower level, more error-prone, less features
- No memory protection: Unacceptable given constitution requirements
- Custom implementation: Would duplicate memguard functionality

---

### 4. SECURITY.md Best Practices

**Decision**: Follow GitHub's security policy template with dsops-specific details

**Structure**:
```markdown
# Security Policy

## Supported Versions
| Version | Supported |
|---------|-----------|
| 0.x     | ✅ Yes    |

## Reporting a Vulnerability

### Methods
1. GitHub Security Advisories (preferred)
2. Email: security@systmms.com

### Response Timeline
- 48 hours: Initial acknowledgment
- 7 days: Severity assessment and timeline
- 90 days: Coordinated disclosure deadline

### Out of Scope
- DoS without security impact
- Social engineering
- Physical attacks
```

**Sources**: GitHub security policy template, OWASP guidelines

---

### 5. Threat Model Documentation

**Decision**: Document specific threats dsops mitigates and explicitly does NOT mitigate

**Threats Mitigated**:
| Threat | Protection | Component |
|--------|------------|-----------|
| Disk residue | Ephemeral execution | `dsops exec` |
| Log exposure | Automatic redaction | `logging.Secret()` |
| Process snooping | Child process isolation | `execenv` |
| Memory dumps | mlock + encryption | `internal/secure` |
| Supply chain tampering | Cosign signatures | Release workflow |
| Dependency vulnerabilities | SBOM + govulncheck | CI/CD |

**Threats NOT Mitigated** (document honestly):
| Threat | Why Not | User Responsibility |
|--------|---------|---------------------|
| Compromised provider | Out of scope | Provider security |
| Insider with root | Cannot defend | Access controls |
| Hardware keyloggers | Physical attack | Physical security |
| Spectre/Meltdown | OS/hardware | System updates |

---

## Summary

All research tasks completed. No NEEDS CLARIFICATION markers remain.

| Technology | Decision | Confidence |
|------------|----------|------------|
| Cosign signing | Keyless via Sigstore OIDC | High |
| SBOM format | SPDX via GoReleaser/syft | High |
| Memory protection | memguard with fallback | High |
| Documentation | GitHub + OWASP patterns | High |
