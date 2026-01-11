# Quickstart: Security Trust Infrastructure

## Overview

This feature adds trust-building infrastructure to dsops:
1. **SECURITY.md** - Vulnerability disclosure policy
2. **Security documentation** - Threat model and architecture docs
3. **Release signing** - Cosign keyless signatures
4. **SBOM generation** - Software Bill of Materials
5. **Memory protection** - Secure handling of secrets in memory

## Prerequisites

- Go 1.25+
- GoReleaser v2
- cosign (for verification testing)
- syft (for SBOM validation)

## Quick Implementation Guide

### Phase 1: Documentation (No Code)

1. **Create SECURITY.md** at repository root
2. **Create docs/content/security/** with threat model and architecture docs
3. **Test**: Hugo build succeeds, docs render correctly

### Phase 2: Release Signing

1. **Add to .goreleaser.yml**:
   ```yaml
   sboms:
     - artifacts: archive
       documents:
         - "{{ .ProjectName }}_{{ .Version }}_sbom.spdx.json"
   ```

2. **Add to .github/workflows/release.yml**:
   ```yaml
   - name: Install cosign
     uses: sigstore/cosign-installer@v3

   - name: Sign checksums
     run: cosign sign-blob --yes dist/*_checksums.txt
   ```

3. **Test**: Create test release, verify with `cosign verify-blob`

### Phase 3: Memory Protection

1. **Create internal/secure/enclave.go**:
   ```go
   package secure

   import "github.com/awnumar/memguard"

   type SecureBuffer struct {
       enclave *memguard.Enclave
   }
   ```

2. **Integrate in resolver** to wrap secret values

3. **Test**: Unit tests + core dump verification

## Verification Commands

```bash
# Verify release signature
cosign verify-blob \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  dsops_*_checksums.txt

# Verify Docker image
cosign verify \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/systmms/dsops:latest

# Validate SBOM
syft packages dsops_*_sbom.spdx.json
```

## Key Files

| File | Purpose |
|------|---------|
| SECURITY.md | Vulnerability disclosure policy |
| docs/content/security/_index.md | Security overview |
| docs/content/security/threat-model.md | Threat model |
| .goreleaser.yml | SBOM + signing config |
| .github/workflows/release.yml | Signing workflow |
| internal/secure/enclave.go | Memory protection |
