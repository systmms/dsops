---
title: "Getting Started"
description: "Get up and running with dsops in minutes"
lead: "Learn how to install dsops, configure your first provider, and manage secrets securely."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 10
---

Welcome to dsops! This guide will help you get started with secure secrets management across multiple providers.

## What is dsops?

dsops is a security-first CLI tool that:
- **Pulls secrets** from 14+ providers (password managers, cloud stores)
- **Injects them** into your environment without writing to disk
- **Supports rotation** to keep credentials fresh
- **Works everywhere** - macOS, Linux, Windows (WSL2)

## What You'll Learn

- How to install dsops on your system
- Configure multiple secret providers
- Execute commands with ephemeral secrets
- Set up automated rotation
- Security best practices

## Prerequisites

- Command line access (Terminal, PowerShell, etc.)
- At least one secret provider account:
  - **Password Managers**: Bitwarden, 1Password, pass
  - **Cloud Providers**: AWS, Google Cloud, Azure
  - **Enterprise**: HashiCorp Vault, Doppler
- 5 minutes to get started!

## Why dsops?

### The Problem
Teams store secrets across multiple systems - some in 1Password, others in AWS Secrets Manager, more in Azure Key Vault. Developers need these secrets for local development, but:
- 🚫 Can't put them in `.env` files (security risk)
- 😕 Switching between provider CLIs is painful
- 🔄 Manual rotation is error-prone
- 📝 No audit trail across providers

### The Solution
dsops provides:
- **One tool, all providers** - Single CLI for 14+ secret stores
- **Memory-only secrets** - Never written to disk by default
- **Automated rotation** - Keep secrets fresh with one command
- **Provider agnostic** - Switch providers without changing workflow

## Choose Your Path

{{< cards >}}
  {{< card title="Installation" href="/getting-started/installation/" >}}
    Install dsops using Homebrew, Docker, or from source.
  {{< /card >}}
  {{< card title="Quick Start" href="/getting-started/quick-start/" >}}
    5-minute tutorial to get your first secret working.
  {{< /card >}}
  {{< card title="Configuration" href="/getting-started/configuration/" >}}
    Deep dive into dsops.yaml configuration options.
  {{< /card >}}
{{< /cards >}}

## Supported Providers

### Password Managers
- ✅ **Bitwarden** - Open source, team-friendly
- ✅ **1Password** - Popular enterprise choice  
- ✅ **pass** - Unix philosophy, GPG-based

### Cloud Providers
- ✅ **AWS** - Secrets Manager, SSM, STS, IAM Identity Center
- ✅ **Google Cloud** - Secret Manager with versioning
- ✅ **Azure** - Key Vault, Managed Identity

### Enterprise
- ✅ **HashiCorp Vault** - Industry standard
- ✅ **Doppler** - Developer-first platform

## Common Use Cases

1. **Local Development** - Pull production-like secrets without `.env` files
2. **CI/CD Pipelines** - Unified secret access across providers
3. **Secret Rotation** - Automate password changes across services
4. **Multi-Cloud** - One tool for AWS, GCP, and Azure secrets
5. **Team Collaboration** - Share secrets securely via password managers