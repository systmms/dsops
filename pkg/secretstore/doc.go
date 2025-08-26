// Package secretstore provides interfaces and types for secret storage systems in dsops.
//
// This package defines the modern secret store abstraction that replaces the older
// provider interface with a cleaner, more focused API specifically designed for
// secret storage operations. It represents the storage layer in dsops's architecture,
// handling where secrets are stored and how they are retrieved.
//
// # Architecture Overview
//
// The secretstore package sits at the foundation of dsops's layered architecture,
// providing the storage abstraction upon which all other operations are built:
//
//     ┌─────────────────────────────────────────────────────────────┐
//     │                    Application Layer                        │
//     │               (CLI, Config, Templates)                      │
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │                  Resolution Engine                          │
//     │               (internal/resolve/)                           │
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │                Secret Store Interface                       │
//     │                 (pkg/secretstore/)             ◄────────────┤
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │              Secret Store Implementations                   │
//     │              (internal/secretstores/)                       │
//     │                                                             │
//     │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
//     │  │     AWS     │  │    Vault    │  │ 1Password   │  ...     │
//     │  │   Secrets   │  │   Store     │  │   Store     │          │
//     │  └─────────────┘  └─────────────┘  └─────────────┘          │
//     └─────────────────────────────────────────────────────────────┘
//
// # Design Philosophy
//
// ## Separation of Concerns
//
// The secretstore package embodies dsops's core architectural principle of
// separating secret storage from secret consumption:
//
//   - **Secret Stores**: Systems that store and retrieve secret values
//     - AWS Secrets Manager, HashiCorp Vault, 1Password, Azure Key Vault
//     - Focus: Storage, versioning, access control, retrieval
//     - This package's domain
//
//   - **Services**: Systems that consume secrets for operations
//     - PostgreSQL, Stripe API, GitHub, Kubernetes clusters
//     - Focus: Service configuration, rotation, lifecycle management
//     - Handled by pkg/service and pkg/rotation
//
// This separation enables:
//   - Clear architectural boundaries
//   - Independent evolution of storage and consumption layers
//   - Flexible mixing and matching of stores and services
//   - Simplified testing and maintenance
//
// ## Modern Reference System
//
// The package introduces a modern URI-based reference system that replaces
// the legacy provider+key approach:
//
//     Legacy:  provider: "aws", key: "database/password"
//     Modern:  store://aws-prod/database/password#password?version=current
//
// Benefits of the new system:
//   - **Hierarchical addressing**: Natural path-based organization
//   - **Field extraction**: Direct access to specific JSON fields  
//   - **Version selection**: Native support for versioned secrets
//   - **Extensible options**: Store-specific parameters via query strings
//   - **URL-like familiarity**: Intuitive for developers
//
// ## Capability-Driven Design
//
// Rather than assuming all secret stores have identical capabilities, the
// package uses capability negotiation to adapt behavior:
//
//   - **Feature detection**: Stores expose what they can do
//   - **Graceful degradation**: Operations adapt to store limitations
//   - **Future-proof**: New capabilities can be added without breaking changes
//   - **User transparency**: Clear feedback about what operations are possible
//
// # Core Interface: SecretStore
//
// The SecretStore interface defines five essential operations:
//
//  1. **Name()**: Unique identifier for the store instance
//  2. **Resolve()**: Retrieve secret values with metadata
//  3. **Describe()**: Get secret metadata without retrieving values
//  4. **Capabilities()**: Expose supported features and limitations
//  5. **Validate()**: Verify configuration and connectivity
//
// This minimal but complete interface enables:
//   - Consistent behavior across all secret stores
//   - Efficient metadata-only operations
//   - Capability-aware application logic
//   - Robust error handling and validation
//
// # Reference System Deep Dive
//
// ## SecretRef Structure
//
// The SecretRef type provides structured access to URI components:
//
//     type SecretRef struct {
//         Store   string            // Store instance name
//         Path    string            // Path within the store
//         Field   string            // Optional field extraction
//         Version string            // Optional version selection  
//         Options map[string]string // Store-specific parameters
//     }
//
// ## URI Format
//
// The complete URI format supports rich addressing:
//
//     store://store-name/path/to/secret#field?version=v&option=value
//
// Examples across different stores:
//
//     # AWS Secrets Manager
//     store://aws-prod/database/credentials#password?version=AWSCURRENT
//     
//     # HashiCorp Vault KV v2
//     store://vault/secret/data/app#api_key?version=2
//     
//     # 1Password
//     store://onepassword/Production/Database#password?vault=Private
//     
//     # Azure Key Vault
//     store://azure/app-secrets#connection-string?version=latest
//
// ## Parsing and Validation
//
// The package provides robust parsing with comprehensive validation:
//   - URI format validation
//   - Required component checking
//   - Query parameter parsing
//   - Error reporting with actionable messages
//
// # Metadata and Capabilities
//
// ## SecretMetadata
//
// The SecretMetadata type enables metadata-only operations:
//   - Existence checking without value retrieval
//   - Size and version information
//   - Permission and tag data
//   - Performance optimization for validation
//
// ## SecretStoreCapabilities
//
// Comprehensive capability reporting enables adaptive behavior:
//
//   - **Feature flags**: Versioning, metadata, binary support, watching
//   - **Authentication**: Required methods and supported mechanisms
//   - **Rotation support**: Version management and constraints
//   - **Performance characteristics**: Batch operation support, caching
//
// This information drives:
//   - UI feature enablement/disabling
//   - Configuration validation
//   - Operation routing and optimization
//   - User experience adaptation
//
// # Error Handling Strategy
//
// The package defines a comprehensive error taxonomy:
//
// ## NotFoundError
//
// Indicates missing secrets with precise location information:
//   - Distinguishes from authentication failures
//   - Enables retry logic and fallback strategies
//   - Provides actionable error messages
//
// ## AuthError
//
// Covers authentication and authorization failures:
//   - Invalid credentials
//   - Expired tokens
//   - Insufficient permissions
//   - Network authentication issues
//
// ## ValidationError
//
// Handles malformed requests and configuration issues:
//   - Invalid URI format
//   - Missing required parameters
//   - Constraint violations
//   - Configuration errors
//
// This structured approach enables:
//   - Appropriate error handling strategies
//   - Clear user feedback
//   - Automated retry and recovery logic
//   - Comprehensive logging and monitoring
//
// # Implementation Guidelines
//
// ## Threading and Concurrency
//
// All SecretStore implementations must be thread-safe:
//   - Multiple goroutines may call methods concurrently
//   - Internal state must be properly synchronized
//   - Context cancellation must be respected
//   - Resource cleanup must be thread-safe
//
// ## Security Requirements
//
// Secret store implementations must follow security best practices:
//   - Never log secret values (use logging.Secret wrapper)
//   - Validate all inputs to prevent injection attacks
//   - Use secure transport (TLS) for network operations
//   - Handle credentials securely in memory
//   - Support proper context cancellation
//
// ## Performance Considerations
//
// Implementations should optimize for common patterns:
//   - Describe() operations should be faster than Resolve()
//   - Connection pooling and reuse where appropriate
//   - Efficient batch operations when supported
//   - Appropriate caching with security considerations
//   - Respect context timeouts and cancellation
//
// ## Store-Specific Adaptations
//
// Each store type has unique characteristics that implementations should handle:
//
// ### AWS Secrets Manager
//   - JSON field extraction for structured secrets
//   - Version labels (AWSCURRENT, AWSPENDING)
//   - Cross-region replication
//   - IAM-based access control
//
// ### HashiCorp Vault
//   - Path-based organization with engines
//   - Version numbers for KV v2
//   - Token-based authentication
//   - Namespace support in Enterprise
//
// ### 1Password
//   - Vault/Item/Field hierarchy
//   - SCIM integration for team management
//   - CLI-based authentication flow
//   - Item template variations
//
// ### Azure Key Vault
//   - Managed identity integration
//   - Certificate vs secret vs key distinction
//   - Soft delete and purge protection
//   - Network access restrictions
//
// # Integration Patterns
//
// ## Configuration Integration
//
// Secret stores integrate with dsops configuration through:
//   - Store definitions in dsops.yaml secretStores section
//   - Factory functions registered in the registry
//   - Validation during configuration loading
//   - Runtime capability checking
//
// ## Resolution Integration
//
// The resolution engine leverages secret stores through:
//   - URI parsing and SecretRef creation
//   - Store selection based on reference
//   - Parallel resolution for batch operations
//   - Error aggregation and reporting
//
// ## Rotation Integration
//
// Rotation operations interact with secret stores for:
//   - Storing newly generated secret values
//   - Version management during rotation
//   - Cleanup of deprecated versions
//   - Audit trail storage
//
// # Testing Strategy
//
// The package supports comprehensive testing through:
//
// ## Interface Contracts
//   - Standard test suites for all implementations
//   - Capability verification tests
//   - Error condition testing
//   - Thread safety validation
//
// ## Mock Implementations
//   - In-memory stores for unit testing
//   - Configurable behavior for edge cases
//   - Performance and concurrency testing
//   - Error injection for resilience testing
//
// ## Integration Testing
//   - Real store connectivity tests
//   - Authentication validation
//   - Cross-store compatibility tests
//   - Performance benchmarking
//
// # Future Evolution
//
// The secretstore package is designed for extensibility:
//
// ## New Store Types
//   - Plugin architecture for external stores
//   - Community-contributed implementations
//   - Experimental and preview stores
//   - Legacy system integrations
//
// ## Enhanced Capabilities
//   - Real-time change notifications
//   - Advanced caching strategies
//   - Batch operation optimization
//   - Cross-store replication
//
// ## Protocol Evolution
//   - New authentication methods
//   - Enhanced metadata schemas
//   - Performance optimizations
//   - Security enhancements
//
// This package provides the stable foundation for dsops's secret management
// capabilities, enabling secure, scalable, and reliable secret storage
// operations across diverse infrastructure environments.
package secretstore