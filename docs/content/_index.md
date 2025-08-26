---
title: "dsops - Developer Secret Operations"
description: "Secure secrets management across multiple providers with rotation capabilities"
lead: "Pull secrets from any vault, render .env files, or execute commands with ephemeral environment variables. Built for modern cloud development."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
seo:
  title: "dsops - Developer Secret Operations"
  description: "Secure secrets management tool supporting Bitwarden, 1Password, AWS Secrets Manager, and more. Features automated rotation, ephemeral secrets, and zero-trust architecture."
  canonical: ""
  noindex: false
---

{{< hero >}}
  {{< hero-content >}}
    <h1 class="hero-title">Developer Secret Operations</h1>
    <p class="hero-lead">Secure secrets management across multiple providers with rotation capabilities.</p>
    <div class="hero-actions">
      {{< button href="/getting-started/quick-start/" text="Get Started" type="primary" size="lg" >}}
      {{< button href="https://github.com/systmms/dsops" text="View on GitHub" type="secondary" size="lg" >}}
    </div>
  {{< /hero-content >}}
{{< /hero >}}

## Key Features

{{< cards >}}
  {{< card title="Multi-Provider Support" icon="server" >}}
    Works with Bitwarden, 1Password, AWS Secrets Manager, HashiCorp Vault, and more. One tool for all your secrets.
  {{< /card >}}
  {{< card title="Ephemeral by Design" icon="shield" >}}
    Secrets never touch disk by default. Execute commands with injected environment variables that vanish when done.
  {{< /card >}}
  {{< card title="Rotation Ready" icon="refresh" >}}
    Automated secret rotation with support for multiple strategies. Keep your credentials fresh and secure.
  {{< /card >}}
{{< /cards >}}

## Quick Example

```bash
# Initialize configuration
dsops init

# Execute command with secrets
dsops exec --env production -- npm start

# Render .env file (explicit opt-in)
dsops render --env production --out .env
```

## Why dsops?

- **Security First**: Automatic secret redaction in logs, memory-only by default
- **Provider Agnostic**: Switch between secret stores without changing your workflow
- **DevOps Ready**: Built for CI/CD pipelines and local development
- **Extensible**: Plugin architecture for custom providers and strategies

{{< button href="/getting-started/installation/" text="Install dsops" type="primary" >}}
