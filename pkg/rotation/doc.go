// Package rotation provides interfaces and types for secret value rotation in dsops.
//
// This package implements the core rotation engine that orchestrates the process of
// rotating actual secret values (passwords, API keys, certificates) within services,
// as opposed to encryption keys used for file encryption or storage-level versioning.
//
// # Architecture Overview
//
// The rotation system follows a multi-layered architecture that separates concerns
// and enables flexible, extensible secret rotation across diverse service types:
//
//     ┌─────────────────────────────────────────────────────────────┐
//     │                  CLI Commands                               │
//     │            (cmd/dsops/commands/)                            │
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │                Rotation Engine                              │
//     │               (pkg/rotation/)                   ◄───────────┤
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │              Rotation Strategies                            │
//     │                                                             │
//     │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
//     │  │  PostgreSQL │  │   Stripe    │  │   Generic   │  ...     │
//     │  │  Rotator    │  │  Rotator    │  │  Rotator    │          │
//     │  └─────────────┘  └─────────────┘  └─────────────┘          │
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │              Protocol Adapters                              │
//     │               (pkg/protocol/)                               │
//     │                                                             │
//     │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
//     │  │   SQL       │  │  HTTP API   │  │ Certificate │  ...     │
//     │  │  Adapter    │  │  Adapter    │  │  Adapter    │          │
//     │  └─────────────┘  └─────────────┘  └─────────────┘          │
//     └─────────────────────────┬───────────────────────────────────┘
//                               │
//     ┌─────────────────────────▼───────────────────────────────────┐
//     │                dsops-data Repository                        │
//     │          Community Service Definitions                      │
//     └─────────────────────────────────────────────────────────────┘
//
// # Core Concepts
//
// ## Secret Value Rotation vs Storage Rotation
//
// This package handles **secret value rotation** - updating the actual credential
// values used by services. This is distinct from:
//   - Storage-level versioning (handled by secret store providers)
//   - Encryption key rotation for file encryption
//   - Database encryption key rotation
//
// Examples of secret value rotation:
//   - Changing a PostgreSQL user's password
//   - Generating new API keys in Stripe
//   - Issuing new TLS certificates
//   - Refreshing OAuth tokens
//
// ## Data-Driven Architecture
//
// The rotation system uses a data-driven approach built on three key components:
//
//  1. **Secret Store Providers** (pkg/provider) - Where secrets are stored
//  2. **Service Integrations** (this package) - What uses secrets and supports rotation
//  3. **Protocol Adapters** (pkg/protocol) - How to communicate with services
//
// Service integrations are defined using community-maintained data from the
// dsops-data repository rather than hardcoded implementations. This enables
// support for hundreds of services without requiring code changes.
//
// ## Rotation Strategies
//
// The package supports multiple rotation strategies to handle different
// security and availability requirements:
//
//   - **Immediate**: Replace secret instantly (brief downtime acceptable)
//   - **Two-Key**: Maintain two valid secrets for zero-downtime rotation
//   - **Overlap**: Gradual transition with configurable overlap period  
//   - **Gradual**: Percentage-based rollout for large deployments
//   - **Custom**: User-defined scripts for special cases
//
// Each strategy is implemented as a separate rotator that can be registered
// with the rotation engine and selected based on service requirements.
//
// # Rotation Engine
//
// The RotationEngine serves as the central coordination point for all rotation
// operations. It provides:
//
//   - Strategy registration and discovery
//   - Request routing based on secret type and strategy
//   - Batch operations for rotating multiple secrets
//   - Rotation history tracking and retrieval
//   - Scheduling future rotations
//   - Error aggregation and reporting
//
// # Rotation Lifecycle
//
// A complete rotation operation follows this lifecycle:
//
//  1. **Planning**: Validate request, check constraints, select strategy
//  2. **Generation**: Create new secret value according to requirements
//  3. **Service Update**: Update target service with new credentials
//  4. **Verification**: Test that new credentials work correctly
//  5. **Storage Update**: Store new secret in secret management system
//  6. **Cleanup**: Remove or deprecate old credentials
//  7. **Audit**: Record all actions taken for compliance
//
// Each step can fail and trigger rollback procedures to restore service functionality.
//
// # Core Interfaces
//
// ## SecretValueRotator
//
// The primary interface that all rotation strategies must implement. Defines methods for:
//   - Capability checking (SupportsSecret)
//   - Primary rotation operation (Rotate)
//   - Verification of new credentials (Verify)
//   - Rollback to previous values (Rollback)
//   - Status monitoring (GetStatus)
//
// ## TwoSecretRotator
//
// Extended interface for zero-downtime rotation strategies. Enables:
//   - Creating secondary secrets alongside primary
//   - Promoting secondary to primary status
//   - Deprecating old primary after verification
//
// ## SchemaAwareRotator
//
// Interface for rotators that leverage dsops-data community definitions.
// Enables data-driven rotation without hardcoded service logic.
//
// # Data Structures
//
// The package defines comprehensive data structures for rotation operations:
//
// ## Core Request/Response Types
//
//   - **RotationRequest**: Complete specification of what to rotate and how
//   - **RotationResult**: Detailed outcome including timing and audit trail
//   - **SecretInfo**: Comprehensive secret metadata and constraints
//   - **VerificationRequest**: Specification of how to test new credentials
//
// ## Supporting Types
//
//   - **SecretReference**: Points to specific versions of secrets
//   - **NewSecretValue**: Specifies how to generate new values
//   - **RotationConstraints**: Defines limits and requirements
//   - **AuditEntry**: Records actions taken during rotation
//
// # Rotation Strategies Implementation
//
// ## Immediate Strategy
//
// Replaces credentials instantly:
//  1. Generate new secret value
//  2. Update service configuration
//  3. Verify new credentials work
//  4. Update secret storage
//  5. Clean up old credentials
//
// Suitable for: Development environments, non-critical services, maintenance windows
//
// ## Two-Key Strategy
//
// Maintains two valid credentials during transition:
//  1. Create secondary secret alongside primary
//  2. Deploy secondary to all systems
//  3. Verify secondary works everywhere
//  4. Promote secondary to primary
//  5. Deprecate old primary after grace period
//
// Suitable for: High-availability services, distributed systems, critical databases
//
// ## Overlap Strategy
//
// Gradual transition with configurable overlap:
//  1. Create new credentials
//  2. Begin directing percentage of traffic to new credentials
//  3. Gradually increase percentage over time
//  4. Monitor for issues and rollback if needed
//  5. Complete transition after validation period
//
// Suitable for: Large-scale services, gradual rollouts, risk-averse environments
//
// # Integration with dsops-data
//
// The rotation system leverages the dsops-data community repository for
// service definitions, enabling:
//
//   - **Service Type Definitions**: Standard schemas for common services
//   - **Instance Configurations**: Pre-configured deployment patterns
//   - **Rotation Policies**: Best practices and constraints
//   - **Protocol Specifications**: Communication patterns and APIs
//
// This data-driven approach means new services can be supported by contributing
// definitions to dsops-data without modifying dsops code.
//
// # Security Architecture
//
// The rotation system implements multiple security layers:
//
// ## Secret Handling
//
//   - Secrets never logged (use logging.Secret wrapper)
//   - In-memory only during rotation operations
//   - Secure cleanup of memory after operations
//   - Context-based cancellation for timeout handling
//
// ## Verification and Rollback
//
//   - Comprehensive verification before completing rotation
//   - Automatic rollback on verification failures
//   - Manual rollback capabilities for emergency scenarios
//   - Audit trails for all operations
//
// ## Access Controls
//
//   - Service-level permission validation
//   - Rotation constraints and policies
//   - Rate limiting and abuse prevention
//   - Integration with enterprise IAM systems
//
// # Error Handling and Observability
//
// The package provides comprehensive error handling:
//
//   - Structured error types for different failure modes
//   - Detailed error messages with actionable guidance
//   - Error aggregation for batch operations
//   - Integration with monitoring and alerting systems
//
// Observability features include:
//   - Detailed audit trails for compliance
//   - Timing metrics for performance monitoring
//   - Success/failure rate tracking
//   - Integration with OpenTelemetry
//
// # Extension Points
//
// The architecture provides multiple extension points:
//
// ## Custom Rotators
//
// Implement SecretValueRotator for service-specific rotation logic:
//
//     type CustomRotator struct {
//         client CustomServiceClient
//     }
//     
//     func (r *CustomRotator) Rotate(ctx context.Context, req RotationRequest) (*RotationResult, error) {
//         // Custom rotation logic
//     }
//
// ## Protocol Adapters
//
// Create adapters for new communication protocols in pkg/protocol.
//
// ## Verification Tests
//
// Define custom verification procedures for ensuring new credentials work.
//
// ## Notification Integrations
//
// Add support for new notification channels (Slack, email, webhooks, etc.).
//
// # Future Evolution
//
// The rotation architecture is designed for evolution:
//
//   - **Plugin Architecture**: Support for external rotation strategies
//   - **Advanced Scheduling**: Cron-like scheduling with dependencies
//   - **Multi-Region Coordination**: Coordinate rotations across regions
//   - **ML-Driven Optimization**: Optimize rotation timing and strategies
//   - **Compliance Automation**: Automated compliance reporting and validation
//
// This package represents the core of dsops's secret rotation capabilities,
// providing a secure, scalable, and extensible foundation for automated
// secret lifecycle management across diverse service ecosystems.
package rotation