// Package provider defines the core interfaces and types for secret store providers in dsops.
//
// This package serves as the foundational abstraction layer for accessing secrets from various
// storage systems like AWS Secrets Manager, HashiCorp Vault, 1Password, Azure Key Vault, and others.
// It provides a unified interface that enables dsops to work with multiple secret storage systems
// through consistent APIs.
//
// # Architecture Overview
//
// The provider package is part of dsops's layered architecture:
//
//     ┌─────────────────────────────────────────────────────────────┐
//     │                    CLI Commands                             │
//     │              (cmd/dsops/commands/)                          │
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │                Resolution Engine                            │
//     │              (internal/resolve/)                            │
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │                Provider Interface                           │
//     │                 (pkg/provider/)                ◄────────────┤
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │              Provider Implementations                       │
//     │              (internal/providers/)                          │
//     │                                                             │
//     │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
//     │  │     AWS     │  │    Vault    │  │ 1Password   │  ...     │
//     │  │  Providers  │  │  Provider   │  │  Provider   │          │
//     │  └─────────────┘  └─────────────┘  └─────────────┘          │
//     └─────────────────────────────────────────────────────────────┘
//
// # Key Design Principles
//
// ## Separation of Concerns
//
// dsops separates secret stores (where secrets are stored) from services (what uses secrets):
//   - **Provider Interface**: Handles secret retrieval from storage systems
//   - **Rotation Interface**: Manages secret rotation within services
//   - **Service Definitions**: Define how services consume and rotate secrets
//
// ## Provider vs Service Distinction
//
// This package focuses exclusively on **secret store providers** - systems that store and
// retrieve secret values. It does NOT handle:
//   - Service integrations (handled by pkg/rotation and pkg/service)
//   - Secret rotation within services (handled by rotation interfaces)
//   - Business logic for specific services (handled by dsops-data definitions)
//
// ## Uniform Interface
//
// All provider implementations must implement the Provider interface, ensuring:
//   - Consistent error handling across all secret stores
//   - Standardized capability negotiation
//   - Unified configuration and validation patterns
//   - Common metadata and versioning support
//
// # Provider Interface
//
// The core Provider interface defines five essential methods:
//
//   - Name(): Unique identifier for the provider
//   - Resolve(): Retrieve secret values from storage
//   - Describe(): Get secret metadata without retrieving values
//   - Capabilities(): Expose provider features and limitations
//   - Validate(): Verify configuration and connectivity
//
// # Provider Capabilities
//
// Providers expose their capabilities through the Capabilities struct, allowing dsops to:
//   - Adapt behavior based on provider features
//   - Enable/disable UI features appropriately
//   - Validate configurations against provider limitations
//   - Route operations to capable providers
//
// Common capabilities include:
//   - Version management (multiple versions of secrets)
//   - Metadata support (tags, descriptions, custom attributes)
//   - Binary data support (certificates, keys, images)
//   - Real-time change notifications
//   - Available authentication methods
//
// # Error Handling
//
// The package defines standardized error types:
//   - NotFoundError: Secret doesn't exist in the provider
//   - AuthError: Authentication failed
//   - General Go errors: For other failure cases
//
// This standardization enables consistent error handling across the application
// regardless of which provider is being used.
//
// # Threading and Concurrency
//
// All Provider implementations must be thread-safe. The dsops architecture assumes
// that multiple goroutines may call provider methods concurrently, particularly
// during batch operations or parallel secret resolution.
//
// Providers should use appropriate synchronization mechanisms if they maintain
// internal state or connections.
//
// # Extension Points
//
// The package provides several extension points for advanced functionality:
//
// ## Rotator Interface
//
// Providers can optionally implement the Rotator interface to support secret
// rotation at the storage level (creating new versions, deprecating old ones).
// This is distinct from service-level rotation handled by other packages.
//
// ## Custom Authentication
//
// Providers can implement custom authentication methods by leveraging the
// AuthMethods capability field and handling authentication in their Validate
// and operational methods.
//
// ## Provider-Specific Features
//
// While the interface is standardized, providers can expose additional features
// through:
//   - Custom metadata in SecretValue.Metadata
//   - Provider-specific options in Reference
//   - Extended error information in custom error types
//
// # Implementation Guidelines
//
// When implementing a new provider:
//
//  1. **Implement the Provider interface completely**
//     - All methods must be implemented, even if some are no-ops
//     - Follow the documented contracts and error handling patterns
//     - Ensure thread-safe implementation
//
//  2. **Handle Reference formats appropriately**
//     - Parse provider-specific addressing schemes
//     - Support optional fields like Version and Field extraction
//     - Validate reference format and return appropriate errors
//
//  3. **Provide accurate capabilities**
//     - Expose actual provider limitations and features
//     - Update capabilities as provider features evolve
//     - Consider future extensibility in capability design
//
//  4. **Follow security best practices**
//     - Never log secret values (use logging.Secret wrapper)
//     - Validate all inputs to prevent injection attacks
//     - Use secure transport (TLS) for network operations
//     - Handle credentials securely in memory
//
//  5. **Support context cancellation**
//     - Respect context deadlines and cancellation
//     - Clean up resources when context is cancelled
//     - Provide reasonable default timeouts
//
// # Registration and Discovery
//
// Providers are registered with the provider registry (internal/providers/registry.go)
// and discovered through factory functions. The registration system enables:
//   - Dynamic provider loading
//   - Configuration-driven provider selection
//   - Plugin architecture for custom providers
//
// # Integration with Configuration System
//
// The provider package integrates tightly with dsops's configuration system:
//   - Provider names in configurations must match Name() return values
//   - Provider-specific configuration is passed to factory functions
//   - Configuration validation leverages provider capabilities
//
// # Future Evolution
//
// The provider interface is designed for evolution:
//   - New capabilities can be added without breaking existing providers
//   - Optional interfaces can extend functionality
//   - Versioning support enables backward compatibility
//   - Provider registration system supports dynamic loading
//
// This package represents the stable core of dsops's secret management architecture,
// providing the foundation for secure, scalable, and extensible secret operations.
package provider