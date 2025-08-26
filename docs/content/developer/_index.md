---
title: "Developer Documentation"
description: "API documentation, interfaces, and developer guides for extending dsops"
lead: "Comprehensive developer documentation for building custom providers, implementing rotation strategies, and integrating dsops into your applications."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 60
---

## Overview

Welcome to the dsops developer documentation. This section provides comprehensive guides for developers who want to:

- Implement custom secret store providers
- Create new rotation strategies  
- Integrate dsops into applications
- Contribute to the dsops project
- Understand the internal architecture

## API Documentation

dsops is written in Go and provides several well-documented interfaces and packages for extension:

### Core Packages

- **[pkg/provider](https://pkg.go.dev/github.com/systmms/dsops/pkg/provider)** - Core provider interface for secret stores
- **[pkg/secretstore](https://pkg.go.dev/github.com/systmms/dsops/pkg/secretstore)** - Modern secret store abstraction with URI references
- **[pkg/rotation](https://pkg.go.dev/github.com/systmms/dsops/pkg/rotation)** - Secret rotation interfaces and engine
- **[pkg/service](https://pkg.go.dev/github.com/systmms/dsops/pkg/service)** - Service definitions for rotation targets
- **[pkg/protocol](https://pkg.go.dev/github.com/systmms/dsops/pkg/protocol)** - Protocol adapters for service communication

### Internal Packages

- **[internal/providers](https://pkg.go.dev/github.com/systmms/dsops/internal/providers)** - Built-in provider implementations
- **[internal/resolve](https://pkg.go.dev/github.com/systmms/dsops/internal/resolve)** - Secret resolution engine
- **[internal/config](https://pkg.go.dev/github.com/systmms/dsops/internal/config)** - Configuration parsing and validation
- **[internal/logging](https://pkg.go.dev/github.com/systmms/dsops/internal/logging)** - Security-aware logging system

## Quick Links

{{< cards >}}
  {{< card title="Provider Interface Guide" href="/developer/provider-interface/" >}}
    Learn how to implement custom secret store providers
  {{< /card >}}
  {{< card title="Rotation Development" href="/developer/rotation-development/" >}}
    Build rotation strategies for your services
  {{< /card >}}
  {{< card title="Architecture Overview" href="/developer/architecture/" >}}
    Understand the dsops architecture and design principles
  {{< /card >}}
  {{< card title="Testing Guide" href="/developer/testing/" >}}
    Write tests for providers and rotators
  {{< /card >}}
  {{< card title="Security Guidelines" href="/developer/security/" >}}
    Security requirements and best practices
  {{< /card >}}
  {{< card title="Contributing" href="/developer/contributing/" >}}
    How to contribute to the dsops project
  {{< /card >}}
{{< /cards >}}

## Key Concepts

### Separation of Concerns

dsops separates two fundamental concepts:

1. **Secret Stores**: Where secrets are stored (AWS Secrets Manager, Vault, 1Password, etc.)
2. **Services**: What uses secrets (PostgreSQL, Stripe API, GitHub, etc.)

This separation enables flexible mixing of storage and consumption patterns.

### Modern URI References

dsops uses a URI-based reference system for secrets:

```
store://store-name/path/to/secret#field?version=v&option=value
```

This provides a consistent, extensible way to reference secrets across all providers.

### Capability-Driven Design

Providers expose their capabilities, allowing dsops to adapt behavior:

```go
type SecretStoreCapabilities struct {
    SupportsVersioning bool
    SupportsMetadata   bool
    SupportsWatching   bool
    SupportsBinary     bool
    RequiresAuth       bool
    AuthMethods        []string
    Rotation           *RotationCapabilities
}
```

### Data-Driven Rotation

Rotation uses community-maintained service definitions from [dsops-data](https://github.com/systmms/dsops-data), enabling support for hundreds of services without hardcoded implementations.

## Getting Started

### For Provider Developers

1. Read the [Provider Interface Guide](/developer/provider-interface/)
2. Study existing providers in `internal/providers/`
3. Implement the `provider.Provider` interface
4. Write tests using the contract test suite
5. Register your provider in the registry

### For Rotation Developers

1. Read the [Rotation Development Guide](/developer/rotation-development/)
2. Understand the rotation strategies
3. Implement `SecretValueRotator` interface
4. Add verification and rollback logic
5. Test with real services

### For Contributors

1. Read our [Contributing Guide](/developer/contributing/)
2. Check the [Architecture Overview](/developer/architecture/)
3. Follow the [Security Guidelines](/developer/security/)
4. Write tests for your changes
5. Submit a pull request

## Example Code

### Implementing a Provider

```go
type MyProvider struct {
    client MyClient
    name   string
}

func (p *MyProvider) Name() string {
    return p.name
}

func (p *MyProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    // Fetch secret from your storage
    value, err := p.client.GetSecret(ref.Key)
    if err != nil {
        return provider.SecretValue{}, err
    }
    
    return provider.SecretValue{
        Value:     value,
        Version:   "v1",
        UpdatedAt: time.Now(),
    }, nil
}
```

### Implementing a Rotator

```go
type MyRotator struct {
    service MyService
}

func (r *MyRotator) Name() string {
    return "my-service"
}

func (r *MyRotator) Rotate(ctx context.Context, req rotation.RotationRequest) (*rotation.RotationResult, error) {
    // Generate new secret
    newSecret, err := r.generateSecret()
    if err != nil {
        return nil, err
    }
    
    // Update service
    if err := r.service.UpdateSecret(newSecret); err != nil {
        return nil, err
    }
    
    // Verify it works
    if err := r.service.TestConnection(newSecret); err != nil {
        // Rollback
        return nil, err
    }
    
    return &rotation.RotationResult{
        Status: rotation.StatusCompleted,
        RotatedAt: time.Now(),
    }, nil
}
```

## Resources

- **[GoDoc API Reference](https://pkg.go.dev/github.com/systmms/dsops)** - Complete API documentation
- **[GitHub Repository](https://github.com/systmms/dsops)** - Source code and issues
- **[dsops-data Repository](https://github.com/systmms/dsops-data)** - Community service definitions
- **[Example Implementations](https://github.com/systmms/dsops/tree/main/examples)** - Sample code and patterns

## Need Help?

- Check the [FAQ](/developer/faq/) for common questions
- Join our [Discord community](https://discord.gg/dsops)
- File an issue on [GitHub](https://github.com/systmms/dsops/issues)
- Read the [troubleshooting guide](/developer/troubleshooting/)