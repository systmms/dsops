---
title: "Security Architecture"
description: "How dsops protects your secrets at every stage"
weight: 20
---

# Security Architecture

dsops is built with security as a core principle. This document explains the security mechanisms that protect your secrets throughout their lifecycle.

## Design Principles

### Ephemeral-First Execution

The primary way to use dsops is through `dsops exec`, which:

1. Fetches secrets from configured providers
2. Injects them into a child process's environment
3. Zeros the secrets from dsops's memory after injection
4. Child process runs with secrets available only in its environment
5. When the child exits, its environment is destroyed by the OS

**Secrets never touch disk** unless you explicitly request it with `dsops env --out`.

```bash
# Recommended: Ephemeral execution
dsops exec production -- ./my-app

# Not recommended: Writing to files
dsops env production --out .env  # Only when absolutely necessary
```

### Log Redaction

All dsops logging automatically redacts sensitive values using `logging.Secret()`. This prevents accidental secret exposure in logs, CI output, or debugging sessions.

```go
// Internal implementation - secrets are wrapped before logging
logger.Debug("Fetched secret: %s", logging.Secret(secretValue))
// Output: "Fetched secret: [REDACTED]"
```

Even if you enable verbose debug logging, secret values are never printed.

### Process Isolation

When you run `dsops exec`, secrets are passed to the child process via environment variables. The parent dsops process:

1. Never stores secrets longer than necessary
2. Zeros secret memory after the child process starts
3. Does not export secrets to its own environment (only the child's)

This means a compromised parent process (after exec) has no access to the secrets it just passed.

## Memory Protection

### How It Works

dsops uses [memguard](https://github.com/awnumar/memguard) for secure memory handling of sensitive data:

| Feature | Protection |
|---------|------------|
| **Memory Locking (mlock)** | Prevents secrets from being swapped to disk |
| **Encryption at Rest** | Secrets encrypted in memory when not actively used |
| **Secure Wiping** | Memory overwritten with zeros on destruction |
| **Guard Pages** | Detect buffer overflow attacks |

### Platform Configuration

Memory protection relies on the operating system's ability to lock memory pages (prevent swapping).

#### Linux

On Linux, mlock is limited by `RLIMIT_MEMLOCK`. Check your current limit:

```bash
ulimit -l
# Output: 64 (default, in KB)
```

To increase the limit for your user:

```bash
# Add to /etc/security/limits.conf
your_username soft memlock 65536
your_username hard memlock 65536
```

Or for the current session:

```bash
ulimit -l 65536
```

For systemd services, add to your unit file:

```ini
[Service]
LimitMEMLOCK=infinity
```

#### macOS

macOS allows mlock by default with no special configuration required.

#### Windows

Windows uses `VirtualLock` which works out of the box with no configuration.

### Graceful Degradation

If mlock fails (e.g., due to resource limits), dsops:

1. Logs a warning message
2. Continues operation using standard memory
3. Secret protection is reduced but functionality preserved

```
WARN: Unable to lock memory (RLIMIT_MEMLOCK too low). Secrets may be swapped to disk.
```

**Recommendation**: Configure mlock limits on production systems for maximum security.

## Secret Lifecycle

```
                                    ┌─────────────────┐
                                    │  Secret Store   │
                                    │ (1Password, AWS,│
                                    │  Vault, etc.)   │
                                    └────────┬────────┘
                                             │
                                        1. Fetch
                                             │
                                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                        dsops process                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │
│  │   Provider   │───▶│   Resolver   │───▶│   Executor   │          │
│  │   (fetch)    │    │  (transform) │    │   (inject)   │          │
│  └──────────────┘    └──────────────┘    └──────┬───────┘          │
│                                                  │                   │
│                                           2. Inject to              │
│                                              child env              │
│                                                  │                   │
│                                           3. Zero memory            │
│                                              (after start)          │
└──────────────────────────────────────────│──────────────────────────┘
                                           │
                                           ▼
                                    ┌─────────────────┐
                                    │  Child Process  │
                                    │ (your app runs  │
                                    │  with secrets)  │
                                    └─────────────────┘
                                           │
                                    4. Process exits,
                                       OS cleans up
                                       environment
```

## What dsops Protects Against

| Threat | Protection | Component |
|--------|------------|-----------|
| Secrets written to disk | Ephemeral execution | `dsops exec` |
| Secrets in logs | Automatic redaction | `logging.Secret()` |
| Process snooping | Child process isolation | `execenv` |
| Memory dumps | mlock + encryption | `internal/secure` |
| Swap file exposure | Memory locking | memguard |
| Supply chain attacks | Signed releases | cosign |
| Dependency vulnerabilities | SBOM transparency | SPDX |

## What dsops Does NOT Protect Against

dsops is not a complete security solution. The following threats require additional measures:

| Threat | Why Not Protected | Your Responsibility |
|--------|-------------------|---------------------|
| Compromised secret store | Out of scope | Provider security, access controls |
| Root access to running process | Cannot defend against root | Access controls, hardening |
| Hardware attacks (cold boot, DMA) | Physical security | Data center security |
| Spectre/Meltdown | OS/hardware vulnerability | System updates, patching |
| Malicious child process | Child has secrets intentionally | Trust your applications |
| Network interception | Provider responsibility | TLS, secure networks |

## Best Practices

1. **Always use `dsops exec`** instead of writing env files
2. **Configure mlock** on production systems
3. **Verify releases** using cosign before deployment
4. **Rotate secrets regularly** using your provider's rotation features
5. **Audit access** to your secret stores
6. **Keep dsops updated** for security patches

## See Also

- [Threat Model](../threat-model/) - Detailed threat analysis
- [Verify Releases](../verify-releases/) - How to verify release authenticity
- [SECURITY.md](https://github.com/systmms/dsops/blob/main/SECURITY.md) - Vulnerability disclosure policy
