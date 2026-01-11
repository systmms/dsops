---
title: "Verifying Releases"
description: "How to verify dsops release authenticity using cosign signatures and checksums"
weight: 30
---

# Verifying Release Authenticity

All dsops releases are cryptographically signed using [Sigstore cosign](https://www.sigstore.dev/). This allows you to verify that binaries and Docker images haven't been tampered with.

## Prerequisites

Install cosign:

```bash
# macOS (Homebrew)
brew install cosign

# Linux
# Download from https://github.com/sigstore/cosign/releases

# Verify cosign installation
cosign version
```

## Verifying Binary Releases

### Method 1: Cosign Signature Verification (Recommended)

Each release includes a signed checksums file. Verify it using:

```bash
# Download the release files
VERSION="0.1.0"  # Replace with your version
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_linux_amd64.tar.gz"
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_checksums.txt"
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_checksums.txt.sig"
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_checksums.txt.pem"

# Verify the checksums file signature
cosign verify-blob \
  --certificate "dsops_${VERSION}_checksums.txt.pem" \
  --signature "dsops_${VERSION}_checksums.txt.sig" \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  "dsops_${VERSION}_checksums.txt"

# If verification succeeds, verify the binary checksum
sha256sum -c "dsops_${VERSION}_checksums.txt" --ignore-missing
```

Expected output for successful verification:

```
Verified OK
dsops_0.1.0_linux_amd64.tar.gz: OK
```

### Method 2: SHA256 Checksum Verification (Fallback)

If you cannot install cosign, you can manually verify checksums:

```bash
# Download files
VERSION="0.1.0"
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_linux_amd64.tar.gz"
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_checksums.txt"

# Verify checksum
sha256sum -c "dsops_${VERSION}_checksums.txt" --ignore-missing
```

> **Note**: This method verifies integrity but not authenticity. The checksums file could have been modified by an attacker. Cosign verification is strongly recommended.

## Verifying Docker Images

Docker images pushed to `ghcr.io/systmms/dsops` are signed with cosign.

### Verify Image Signature

```bash
# Verify the latest tag
cosign verify \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/systmms/dsops:latest

# Verify a specific version
cosign verify \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/systmms/dsops:0.1.0
```

Expected output:

```
Verification for ghcr.io/systmms/dsops:latest --
The following checks were performed on each of these signatures:
  - The cosign claims were validated
  - Existence of the claims in the transparency log was verified offline
  - The code-signing certificate was verified using trusted certificate authority certificates
```

### Pull and Verify in One Command

For Kubernetes or Docker Compose deployments:

```bash
# Verify and pull
cosign verify \
  --certificate-identity-regexp='https://github.com/systmms/dsops/.*' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/systmms/dsops:latest \
&& docker pull ghcr.io/systmms/dsops:latest
```

## Verifying the SBOM

Each release includes a Software Bill of Materials (SBOM) in SPDX format:

```bash
VERSION="0.1.0"
curl -LO "https://github.com/systmms/dsops/releases/download/v${VERSION}/dsops_${VERSION}_sbom.spdx.json"

# View SBOM contents (requires jq)
jq '.packages[].name' "dsops_${VERSION}_sbom.spdx.json"

# Or use syft to analyze
syft packages "dsops_${VERSION}_sbom.spdx.json"
```

## Understanding Verification Identities

The verification commands use these identity parameters:

| Parameter | Value | Meaning |
|-----------|-------|---------|
| `certificate-identity-regexp` | `https://github.com/systmms/dsops/.*` | Signer must be a GitHub Actions workflow in the dsops repository |
| `certificate-oidc-issuer` | `https://token.actions.githubusercontent.com` | Token must come from GitHub Actions OIDC |

These ensure that:
1. Only the official dsops GitHub repository can produce valid signatures
2. Signatures are created during GitHub Actions workflow execution (not manually)
3. The signing identity is recorded in the Rekor transparency log

## Troubleshooting

### "no matching signatures found"

This typically means:
- The release predates signature implementation
- You're using an unofficial mirror
- The image/binary was modified

### "certificate identity mismatch"

Ensure you're using the correct identity regexp. The pattern must match the workflow that signed the release.

### Offline Verification

For air-gapped environments, you can verify against a local copy of the Rekor log:

```bash
# Download signature bundle for offline verification
cosign download signature ghcr.io/systmms/dsops:latest > signature.json
```

## Security Considerations

- **Always verify before deploying to production**: Even if you trust the source, verification catches supply chain attacks
- **Pin to specific versions**: Use exact version tags instead of `latest` for reproducibility
- **Automate verification**: Include cosign verification in your CI/CD pipeline
- **Report suspicious artifacts**: If verification fails unexpectedly, report it via our [security policy](https://github.com/systmms/dsops/blob/main/SECURITY.md)
