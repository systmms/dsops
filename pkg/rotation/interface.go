// Package rotation provides interfaces and types for secret value rotation in dsops.
//
// This package implements the core rotation engine that orchestrates the process of
// rotating actual secret values (passwords, API keys, certificates) as opposed to
// encryption keys used for file encryption.
//
// # Rotation Architecture
//
// dsops uses a data-driven approach to rotation built on three key components:
//
//  1. **Secret Store Providers** (pkg/provider) - Where secrets are stored (Vault, AWS, etc.)
//  2. **Service Integrations** (this package) - What uses secrets (PostgreSQL, Stripe, etc.)
//  3. **Protocol Adapters** (pkg/protocol) - How to communicate with services
//
// Service integrations are defined using community-maintained data from the dsops-data
// repository rather than hardcoded implementations. This enables support for hundreds
// of services without requiring code changes.
//
// # Rotation Strategies
//
// Different rotation strategies handle various security and availability requirements:
//
//   - **Immediate**: Replace secret instantly (brief downtime acceptable)
//   - **Two-Key**: Maintain two valid secrets for zero-downtime rotation
//   - **Overlap**: Gradual transition with configurable overlap period
//   - **Gradual**: Percentage-based rollout for large deployments
//   - **Custom**: User-defined scripts for special cases
//
// # Usage Example
//
//	// Create rotation engine
//	engine := NewRotationEngine()
//	
//	// Register strategies
//	engine.RegisterStrategy(&PostgresRotator{})
//	engine.RegisterStrategy(&StripeRotator{})
//	
//	// Perform rotation
//	request := RotationRequest{
//	    Secret: SecretInfo{
//	        Key:         "DATABASE_PASSWORD",
//	        Provider:    "aws.secretsmanager", 
//	        SecretType:  SecretTypePassword,
//	    },
//	    Strategy:  "postgres",
//	    TwoSecret: true,
//	}
//	
//	result, err := engine.Rotate(ctx, request)
//	if err != nil {
//	    return fmt.Errorf("rotation failed: %w", err)
//	}
//	
//	fmt.Printf("Rotation completed: %s\n", result.Status)
//
// # Implementing Custom Rotators
//
// To implement a custom rotation strategy:
//
//  1. Implement SecretValueRotator interface
//  2. Optionally implement TwoSecretRotator for zero-downtime support
//  3. Implement SchemaAwareRotator to use dsops-data definitions
//  4. Register your rotator with the rotation engine
//
// Example:
//
//	type MyServiceRotator struct {
//	    client MyServiceClient
//	}
//	
//	func (r *MyServiceRotator) Name() string {
//	    return "my-service"
//	}
//	
//	func (r *MyServiceRotator) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
//	    return secret.SecretType == SecretTypeAPIKey
//	}
//	
//	func (r *MyServiceRotator) Rotate(ctx context.Context, req RotationRequest) (*RotationResult, error) {
//	    // Implementation specific to your service
//	    newAPIKey, err := r.client.CreateNewAPIKey(ctx)
//	    if err != nil {
//	        return nil, err
//	    }
//	    
//	    // Store new key in secret provider
//	    // Update service to use new key  
//	    // Verify new key works
//	    // Deprecate old key
//	    
//	    return &RotationResult{
//	        Status:    StatusCompleted,
//	        RotatedAt: &now,
//	    }, nil
//	}
//
// # Security Considerations
//
// Rotation operations must:
//   - Never log secret values (use logging.Secret wrapper)
//   - Verify new secrets work before removing old ones
//   - Support rollback to previous values on failure
//   - Generate audit trails for all operations
//   - Handle concurrent rotation attempts gracefully
//   - Respect service-specific constraints and policies
//
// # Data-Driven Integration
//
// Rotators can leverage dsops-data repository for service definitions:
//
//	type DataDrivenRotator struct {
//	    repository *dsopsdata.Repository
//	}
//	
//	func (r *DataDrivenRotator) SetRepository(repo *dsopsdata.Repository) {
//	    r.repository = repo
//	}
//	
//	func (r *DataDrivenRotator) Rotate(ctx context.Context, req RotationRequest) (*RotationResult, error) {
//	    // Use repository to get service-specific configuration
//	    serviceDef, err := r.repository.GetServiceType(req.Secret.Provider)
//	    if err != nil {
//	        return nil, err
//	    }
//	    
//	    // Use service definition to determine rotation approach
//	    // ...
//	}
package rotation

import (
	"context"
	"time"

	"github.com/systmms/dsops/internal/dsopsdata"
	"github.com/systmms/dsops/pkg/provider"
)

// SecretValueRotator defines the interface for rotating actual secret values.
//
// This interface is implemented by rotation strategies that handle the process of
// updating secret values within services (databases, APIs, etc.) and coordinating
// the update with secret storage systems.
//
// Implementations should handle the complete rotation lifecycle:
//   1. Generate or obtain new secret values
//   2. Update the target service with new credentials
//   3. Verify the new credentials work correctly
//   4. Update the secret storage system
//   5. Clean up old credentials
//   6. Handle rollback if anything fails
//
// Thread safety: Implementations must be thread-safe as multiple rotations
// may occur concurrently for different secrets.
type SecretValueRotator interface {
	// Name returns the unique strategy identifier.
	//
	// This should be a stable, lowercase identifier used to identify the rotator
	// in configuration files and CLI commands. Examples: "postgres", "stripe",
	// "github", "generic-api", "certificate".
	//
	// The name is used for:
	//   - Strategy registration and lookup
	//   - Configuration file references
	//   - Logging and audit trails
	//   - CLI command arguments
	Name() string

	// SupportsSecret determines if this rotator can handle the given secret.
	//
	// This method allows rotators to indicate which types of secrets they can
	// rotate based on the secret's type, provider, metadata, or other attributes.
	// The rotation engine uses this to select the appropriate rotator for each
	// secret.
	//
	// Common criteria for support:
	//   - Secret type (password, api_key, certificate, etc.)
	//   - Provider compatibility
	//   - Service-specific metadata
	//   - Required configuration presence
	//
	// Example:
	//
	//	func (r *PostgresRotator) SupportsSecret(ctx context.Context, secret SecretInfo) bool {
	//	    // Only handle password secrets
	//	    if secret.SecretType != SecretTypePassword {
	//	        return false
	//	    }
	//	    
	//	    // Check for required metadata
	//	    if secret.Metadata["database_type"] != "postgresql" {
	//	        return false
	//	    }
	//	    
	//	    return true
	//	}
	SupportsSecret(ctx context.Context, secret SecretInfo) bool

	// Rotate performs the complete secret rotation lifecycle.
	//
	// This is the core method that orchestrates the entire rotation process:
	//   1. Generate or retrieve new secret value
	//   2. Update the target service with new credentials
	//   3. Verify the new credentials work correctly
	//   4. Update secret storage with new value
	//   5. Clean up old credentials
	//   6. Generate audit trail
	//
	// The method should handle errors gracefully and provide detailed results
	// including timing information, verification results, and audit entries.
	//
	// For dry-run requests (request.DryRun = true), the method should:
	//   - Validate the rotation request
	//   - Check preconditions
	//   - Return what would be done without executing
	//
	// Example:
	//
	//	func (r *PostgresRotator) Rotate(ctx context.Context, req RotationRequest) (*RotationResult, error) {
	//	    if req.DryRun {
	//	        return r.planRotation(ctx, req)
	//	    }
	//	    
	//	    startTime := time.Now()
	//	    result := &RotationResult{
	//	        Secret: req.Secret,
	//	        Status: StatusRotating,
	//	    }
	//	    
	//	    // Generate new password
	//	    newPassword, err := r.generatePassword(req)
	//	    if err != nil {
	//	        result.Status = StatusFailed
	//	        result.Error = err.Error()
	//	        return result, err
	//	    }
	//	    
	//	    // Update database user
	//	    if err := r.updateDatabasePassword(ctx, newPassword); err != nil {
	//	        result.Status = StatusFailed
	//	        result.Error = err.Error()
	//	        return result, err
	//	    }
	//	    
	//	    // Verify connection works
	//	    if err := r.verifyConnection(ctx, newPassword); err != nil {
	//	        // Attempt rollback
	//	        r.rollbackPassword(ctx, req.Secret)
	//	        result.Status = StatusFailed
	//	        result.Error = err.Error()
	//	        return result, err
	//	    }
	//	    
	//	    // Update secret store
	//	    if err := r.updateSecretStore(ctx, req, newPassword); err != nil {
	//	        result.Status = StatusFailed
	//	        result.Error = err.Error()
	//	        return result, err
	//	    }
	//	    
	//	    now := time.Now()
	//	    result.Status = StatusCompleted
	//	    result.RotatedAt = &now
	//	    result.AuditTrail = r.generateAuditTrail(startTime, now)
	//	    
	//	    return result, nil
	//	}
	Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error)

	// Verify checks that new secret credentials are working correctly.
	//
	// This method performs validation tests to ensure that newly rotated
	// credentials are functional before completing the rotation process.
	// It's typically called during the Rotate method, but can also be
	// invoked independently for verification.
	//
	// The verification request includes test specifications that define
	// what checks to perform. Common verification tests:
	//   - Connection tests (can we connect with new credentials?)
	//   - Query tests (can we perform required operations?)
	//   - API tests (do API calls succeed with new tokens?)
	//   - Permission tests (do we have required access levels?)
	//
	// Example:
	//
	//	func (r *PostgresRotator) Verify(ctx context.Context, req VerificationRequest) error {
	//	    for _, test := range req.Tests {
	//	        switch test.Type {
	//	        case TestTypeConnection:
	//	            if err := r.testConnection(ctx, req.NewSecretRef); err != nil {
	//	                return fmt.Errorf("connection test failed: %w", err)
	//	            }
	//	            
	//	        case TestTypeQuery:
	//	            if err := r.testQuery(ctx, req.NewSecretRef, test.Config); err != nil {
	//	                return fmt.Errorf("query test failed: %w", err)
	//	            }
	//	        }
	//	    }
	//	    return nil
	//	}
	Verify(ctx context.Context, request VerificationRequest) error

	// Rollback reverts to the previous secret value if possible.
	//
	// This method attempts to restore service functionality by reverting to
	// a previous secret value when rotation fails or new credentials don't
	// work as expected. Not all services support rollback - some may require
	// manual intervention.
	//
	// Rollback scenarios:
	//   - New credentials fail verification
	//   - Service rejects new credentials after deployment
	//   - Manual rollback requested by operator
	//   - Rotation process encounters fatal error
	//
	// The method should:
	//   - Restore the service to use old credentials
	//   - Update secret storage if necessary
	//   - Generate audit trail for rollback operation
	//   - Return error if rollback is not possible
	//
	// Example:
	//
	//	func (r *PostgresRotator) Rollback(ctx context.Context, req RollbackRequest) error {
	//	    // Get old password from secret reference
	//	    oldPassword, err := r.getSecretValue(ctx, req.OldSecretRef)
	//	    if err != nil {
	//	        return fmt.Errorf("cannot retrieve old password for rollback: %w", err)
	//	    }
	//	    
	//	    // Restore database user password
	//	    if err := r.updateDatabasePassword(ctx, oldPassword); err != nil {
	//	        return fmt.Errorf("rollback failed: %w", err)
	//	    }
	//	    
	//	    // Verify rollback worked
	//	    if err := r.verifyConnection(ctx, oldPassword); err != nil {
	//	        return fmt.Errorf("rollback verification failed: %w", err)
	//	    }
	//	    
	//	    r.logRollback(req.Secret, req.Reason)
	//	    return nil
	//	}
	Rollback(ctx context.Context, request RollbackRequest) error

	// GetStatus returns the current rotation status and metadata for a secret.
	//
	// This method provides information about when a secret was last rotated,
	// when it should next be rotated, and whether rotation is currently possible.
	// Used by status commands and rotation scheduling.
	//
	// The returned status should include:
	//   - Current rotation state
	//   - Last rotation timestamp
	//   - Next scheduled rotation time
	//   - Whether rotation is currently allowed
	//   - Any blocking conditions or reasons
	//
	// Example:
	//
	//	func (r *PostgresRotator) GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error) {
	//	    // Check rotation history
	//	    lastRotation, err := r.getLastRotationTime(ctx, secret)
	//	    if err != nil {
	//	        return nil, err
	//	    }
	//	    
	//	    // Calculate next rotation time
	//	    nextRotation := lastRotation.Add(r.getRotationInterval(secret))
	//	    
	//	    // Check if rotation is currently possible
	//	    canRotate, reason := r.canRotateNow(ctx, secret)
	//	    
	//	    return &RotationStatusInfo{
	//	        Status:          StatusCompleted,
	//	        LastRotated:     &lastRotation,
	//	        NextRotation:    &nextRotation,
	//	        CanRotate:       canRotate,
	//	        Reason:          reason,
	//	    }, nil
	//	}
	GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error)
}

// TwoSecretRotator extends SecretValueRotator for zero-downtime rotation.
//
// This interface enables the two-key rotation strategy where both old and new
// credentials are valid simultaneously during the rotation process. This
// eliminates downtime by ensuring services can continue using old credentials
// while new ones are being deployed and verified.
//
// The two-secret rotation process follows these phases:
//   1. Create secondary (new) secret alongside existing primary
//   2. Deploy and verify secondary secret works in all systems
//   3. Promote secondary to primary (make it the active credential)
//   4. Deprecate old primary after grace period
//
// This strategy is ideal for:
//   - High-availability services that cannot tolerate downtime
//   - Complex distributed systems with multiple deployment phases
//   - Services where credential propagation takes time
//   - Systems requiring validation periods before full cutover
//
// Example implementation:
//
//	type DatabaseTwoKeyRotator struct {
//	    client DatabaseClient
//	}
//	
//	func (r *DatabaseTwoKeyRotator) CreateSecondarySecret(ctx context.Context, req SecondarySecretRequest) (*SecretReference, error) {
//	    // Generate new password
//	    newPassword, err := r.generatePassword()
//	    if err != nil {
//	        return nil, err
//	    }
//	    
//	    // Create new database user with same permissions
//	    newUsername := req.Secret.Key + "_new"
//	    if err := r.client.CreateUser(ctx, newUsername, newPassword); err != nil {
//	        return nil, err
//	    }
//	    
//	    return &SecretReference{
//	        Provider:   req.Secret.Provider,
//	        Key:        newUsername,
//	        Identifier: newUsername,
//	    }, nil
//	}
type TwoSecretRotator interface {
	SecretValueRotator

	// CreateSecondarySecret creates the inactive/secondary secret.
	//
	// This method creates a new version of the secret without affecting the
	// current active version. The secondary secret should have the same
	// permissions and access as the primary but with a new value.
	//
	// For databases, this might create a new user with identical permissions.
	// For API services, this might generate a new API key while keeping the old one active.
	// For certificates, this might issue a new certificate with the same subject.
	//
	// The returned SecretReference points to the newly created secondary secret
	// and can be used in subsequent promote/deprecate operations.
	//
	// Example:
	//
	//	req := SecondarySecretRequest{
	//	    Secret: SecretInfo{
	//	        Key:         "api_key",
	//	        Provider:    "stripe",
	//	        SecretType:  SecretTypeAPIKey,
	//	    },
	//	    NewValue: &NewSecretValue{
	//	        Type: ValueTypeGenerated,
	//	    },
	//	}
	//	
	//	secondaryRef, err := rotator.CreateSecondarySecret(ctx, req)
	//	if err != nil {
	//	    return fmt.Errorf("failed to create secondary secret: %w", err)
	//	}
	CreateSecondarySecret(ctx context.Context, request SecondarySecretRequest) (*SecretReference, error)

	// PromoteSecondarySecret makes the secondary secret the active/primary secret.
	//
	// This method transitions the system to use the secondary secret as the primary.
	// After promotion, new connections and operations should use the secondary
	// secret, but existing connections using the old primary may continue to work
	// during the grace period.
	//
	// The implementation should:
	//   - Update service configuration to use secondary as primary
	//   - Maintain backward compatibility during grace period
	//   - Verify the promotion was successful
	//   - Roll back if promotion fails
	//
	// If VerifyFirst is true, the method should test the secondary secret
	// before promoting it to avoid promoting non-functional credentials.
	//
	// Example:
	//
	//	req := PromoteRequest{
	//	    Secret:       secretInfo,
	//	    SecondaryRef: secondaryRef,
	//	    GracePeriod:  5 * time.Minute,
	//	    VerifyFirst:  true,
	//	}
	//	
	//	if err := rotator.PromoteSecondarySecret(ctx, req); err != nil {
	//	    return fmt.Errorf("failed to promote secondary secret: %w", err)
	//	}
	PromoteSecondarySecret(ctx context.Context, request PromoteRequest) error

	// DeprecatePrimarySecret marks the old primary secret as deprecated.
	//
	// This method is called after successful promotion and grace period to
	// clean up the old primary secret. Depending on the service and HardDelete
	// setting, this may:
	//   - Delete the old secret entirely
	//   - Mark it as inactive but retain for audit/rollback
	//   - Reduce its permissions or scope
	//   - Move it to a "deprecated" state
	//
	// The grace period allows time for:
	//   - Existing connections to naturally close
	//   - All services to pick up the new primary
	//   - Verification that no systems are still using old primary
	//
	// If HardDelete is false, the old secret should be retained in a disabled
	// state for potential rollback scenarios.
	//
	// Example:
	//
	//	req := DeprecateRequest{
	//	    Secret:      secretInfo,
	//	    OldRef:      oldPrimaryRef,
	//	    GracePeriod: 10 * time.Minute,
	//	    HardDelete:  false, // Keep for rollback
	//	}
	//	
	//	if err := rotator.DeprecatePrimarySecret(ctx, req); err != nil {
	//	    log.Warnf("Failed to deprecate old primary: %v", err)
	//	    // This may not be fatal - new primary is already active
	//	}
	DeprecatePrimarySecret(ctx context.Context, request DeprecateRequest) error
}

// SchemaAwareRotator defines strategies that can use dsops-data schema information.
//
// This interface enables rotators to leverage community-maintained service
// definitions from the dsops-data repository. Instead of hardcoding service-specific
// rotation logic, rotators can use standardized schemas that define:
//   - Service connection patterns
//   - Credential types and formats  
//   - Rotation procedures and constraints
//   - Verification tests and validation rules
//
// Benefits of schema-aware rotation:
//   - Support hundreds of services without custom code
//   - Community-maintained and continuously updated
//   - Consistent rotation patterns across services
//   - Extensible through data-driven configuration
//
// Example implementation:
//
//	type GenericSchemaRotator struct {
//	    repository *dsopsdata.Repository
//	}
//	
//	func (r *GenericSchemaRotator) SetRepository(repo *dsopsdata.Repository) {
//	    r.repository = repo
//	}
//	
//	func (r *GenericSchemaRotator) Rotate(ctx context.Context, req RotationRequest) (*RotationResult, error) {
//	    // Get service definition from dsops-data
//	    serviceDef, err := r.repository.GetServiceType(req.Secret.Provider)
//	    if err != nil {
//	        return nil, fmt.Errorf("unknown service type: %w", err)
//	    }
//	    
//	    // Use schema to determine rotation approach
//	    switch serviceDef.RotationType {
//	    case "api_key_rotation":
//	        return r.rotateAPIKey(ctx, req, serviceDef)
//	    case "password_rotation":
//	        return r.rotatePassword(ctx, req, serviceDef)
//	    default:
//	        return nil, fmt.Errorf("unsupported rotation type: %s", serviceDef.RotationType)
//	    }
//	}
//
type SchemaAwareRotator interface {
	// SetRepository sets the dsops-data repository for schema-aware rotation.
	//
	// This method provides access to the dsops-data repository containing
	// community-maintained service definitions. Rotators implementing this
	// interface can use these definitions to:
	//   - Determine supported credential types for services
	//   - Access rotation procedures and constraints
	//   - Use standardized verification tests
	//   - Follow service-specific best practices
	//
	// The repository is typically set once during rotator initialization
	// and remains available for all subsequent rotation operations.
	//
	// Example:
	//
	//	rotator := &GenericRotator{}
	//	repository, err := dsopsdata.LoadRepository("./dsops-data")
	//	if err != nil {
	//	    return err
	//	}
	//	
	//	rotator.SetRepository(repository)
	//	
	//	// Rotator can now use service definitions
	//	result, err := rotator.Rotate(ctx, rotationRequest)
	SetRepository(repository *dsopsdata.Repository)
}

// SecretInfo contains comprehensive information about the secret to be rotated.
//
// This structure provides all the context needed for rotation strategies to:
//   - Identify the specific secret and its location
//   - Understand the type of secret and its constraints
//   - Access metadata for rotation planning
//   - Apply appropriate rotation policies
//
// Example:
//
//	secretInfo := SecretInfo{
//	    Key:         "database_password",
//	    Provider:    "aws.secretsmanager",
//	    ProviderRef: provider.Reference{
//	        Provider: "aws.secretsmanager",
//	        Key:      "prod/db/password",
//	        Version:  "AWSCURRENT",
//	    },
//	    SecretType: SecretTypePassword,
//	    Metadata: map[string]string{
//	        "database":    "postgresql",
//	        "environment": "production",
//	        "owner":       "platform-team",
//	    },
//	    Constraints: &RotationConstraints{
//	        MinRotationInterval: 7 * 24 * time.Hour, // Weekly minimum
//	        MinValueLength:      16,
//	        RequiredTests: []VerificationTest{
//	            {Name: "connection", Type: TestTypeConnection, Required: true},
//	        },
//	    },
//	}
type SecretInfo struct {
	// Key is the logical name/identifier for this secret in the configuration.
	// Used for logging, error messages, and referencing in rotation policies.
	Key string `json:"key"`

	// Provider identifies which secret store or service manages this secret.
	// Examples: "aws.secretsmanager", "onepassword", "postgresql", "stripe"
	Provider string `json:"provider"`

	// ProviderRef contains the specific reference information needed to
	// locate and retrieve this secret from the provider.
	ProviderRef provider.Reference `json:"provider_ref"`

	// SecretType categorizes what kind of secret this is, which determines
	// the appropriate rotation strategy and validation procedures.
	SecretType SecretType `json:"secret_type"`

	// Metadata contains additional context about the secret, its usage,
	// and rotation requirements. Common keys include:
	//   - "environment": deployment environment (prod, staging, dev)
	//   - "service": which service uses this secret
	//   - "owner": team or individual responsible
	//   - "criticality": importance level (high, medium, low)
	Metadata map[string]string `json:"metadata"`

	// Constraints define rotation policies, limits, and requirements
	// specific to this secret. Nil if no special constraints apply.
	Constraints *RotationConstraints `json:"constraints,omitempty"`
}

// RotationRequest contains all information needed to perform a rotation.
//
// This structure encapsulates a complete rotation operation including the secret
// to rotate, the strategy to use, and various options controlling the rotation
// process. It serves as the input to rotation operations.
//
// Example:
//
//	request := RotationRequest{
//	    Secret: SecretInfo{
//	        Key:         "api_key",
//	        Provider:    "stripe",
//	        SecretType:  SecretTypeAPIKey,
//	    },
//	    Strategy:  "stripe",
//	    DryRun:    false,
//	    Force:     false,
//	    TwoSecret: true, // Use two-key rotation for zero downtime
//	    Config: map[string]interface{}{
//	        "environment": "production",
//	        "scopes":      []string{"read", "write"},
//	    },
//	    Notification: []string{"ops-team", "on-call"},
//	}
type RotationRequest struct {
	// Secret contains detailed information about what secret to rotate.
	Secret SecretInfo `json:"secret"`

	// Strategy specifies which rotation strategy/rotator to use.
	// Must match a registered rotator's Name() method.
	Strategy string `json:"strategy"`

	// NewValue specifies how to generate the new secret value.
	// If nil, the rotator will use its default value generation.
	NewValue *NewSecretValue `json:"new_value,omitempty"`

	// DryRun indicates this is a planning/validation run that should not
	// make any actual changes. Used for testing and verification.
	DryRun bool `json:"dry_run"`

	// Force bypasses safety checks and constraints (use with caution).
	// This can skip minimum rotation intervals, verification failures, etc.
	Force bool `json:"force"`

	// TwoSecret enables two-key rotation strategy if the rotator supports it.
	// This creates overlap periods for zero-downtime rotation.
	TwoSecret bool `json:"two_secret"`

	// Config provides additional configuration specific to the rotation
	// strategy. Contents depend on the specific rotator being used.
	Config map[string]interface{} `json:"config,omitempty"`

	// Notification specifies who should be notified about rotation events.
	// Format depends on notification system configuration.
	Notification []string `json:"notification,omitempty"`
}

// RotationResult contains the complete outcome of a rotation attempt.
//
// This structure provides comprehensive information about what happened during
// rotation, including success/failure status, timing information, verification
// results, and detailed audit trails. It serves as the output from rotation
// operations and can be used for monitoring, alerting, and compliance.
//
// Example:
//
//	result := &RotationResult{
//	    Secret: secretInfo,
//	    Status: StatusCompleted,
//	    NewSecretRef: &SecretReference{
//	        Provider:    "aws.secretsmanager",
//	        Key:         "prod/db/password", 
//	        Version:     "v2",
//	        Identifier:  "db_user_prod",
//	    },
//	    RotatedAt: &rotationTime,
//	    ExpiresAt: &expirationTime,
//	    VerificationResults: []VerificationResult{
//	        {
//	            Test:     connectionTest,
//	            Status:   TestStatusPassed,
//	            Duration: 1500 * time.Millisecond,
//	            Message:  "Database connection successful",
//	        },
//	    },
//	    AuditTrail: []AuditEntry{
//	        {Timestamp: time.Now(), Action: "rotation_started", Status: "success"},
//	        {Timestamp: time.Now(), Action: "secret_generated", Status: "success"},
//	        {Timestamp: time.Now(), Action: "service_updated", Status: "success"},
//	        {Timestamp: time.Now(), Action: "verification_completed", Status: "success"},
//	    },
//	}
type RotationResult struct {
	// Secret identifies which secret was rotated.
	Secret SecretInfo `json:"secret"`

	// Status indicates the final state of the rotation operation.
	Status RotationStatus `json:"status"`

	// NewSecretRef points to the newly created/rotated secret version.
	// Present for successful rotations.
	NewSecretRef *SecretReference `json:"new_secret_ref,omitempty"`

	// OldSecretRef points to the previous secret version that was replaced.
	// Used for rollback operations if needed.
	OldSecretRef *SecretReference `json:"old_secret_ref,omitempty"`

	// RotatedAt indicates when the rotation was successfully completed.
	// Nil for failed or incomplete rotations.
	RotatedAt *time.Time `json:"rotated_at,omitempty"`

	// ExpiresAt indicates when the rotated secret expires or should be rotated again.
	// Used for scheduling future rotations.
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// VerificationResults contains outcomes of all verification tests performed
	// during the rotation to ensure the new secret works correctly.
	VerificationResults []VerificationResult `json:"verification_results,omitempty"`

	// Error contains error message if the rotation failed.
	// Empty string indicates successful rotation.
	Error string `json:"error,omitempty"`

	// Warnings contains non-fatal issues encountered during rotation.
	// These don't prevent success but should be reviewed.
	Warnings []string `json:"warnings,omitempty"`

	// AuditTrail provides detailed record of all actions taken during rotation.
	// Used for compliance, debugging, and operational monitoring.
	AuditTrail []AuditEntry `json:"audit_trail,omitempty"`
}

// VerificationRequest contains information for verifying a rotated secret
type VerificationRequest struct {
	Secret       SecretInfo        `json:"secret"`
	NewSecretRef SecretReference   `json:"new_secret_ref"`
	Tests        []VerificationTest `json:"tests"`
	Timeout      time.Duration     `json:"timeout"`
}

// RollbackRequest contains information for rolling back a rotation
type RollbackRequest struct {
	Secret       SecretInfo      `json:"secret"`
	OldSecretRef SecretReference `json:"old_secret_ref"`
	Reason       string          `json:"reason"`
}

// SecondarySecretRequest for two-secret strategy
type SecondarySecretRequest struct {
	Secret       SecretInfo            `json:"secret"`
	NewValue     *NewSecretValue       `json:"new_value,omitempty"`
	Config       map[string]interface{} `json:"config,omitempty"`
}

// PromoteRequest for promoting secondary to primary
type PromoteRequest struct {
	Secret           SecretInfo      `json:"secret"`
	SecondaryRef     SecretReference `json:"secondary_ref"`
	GracePeriod      time.Duration   `json:"grace_period"`
	VerifyFirst      bool            `json:"verify_first"`
}

// DeprecateRequest for deprecating old primary
type DeprecateRequest struct {
	Secret       SecretInfo      `json:"secret"`
	OldRef       SecretReference `json:"old_ref"`
	GracePeriod  time.Duration   `json:"grace_period"`
	HardDelete   bool            `json:"hard_delete"`
}

// SecretReference points to a specific version/instance of a secret
type SecretReference struct {
	Provider    string            `json:"provider"`
	Key         string            `json:"key"`
	Version     string            `json:"version,omitempty"`
	Identifier  string            `json:"identifier,omitempty"` // DB user, API key ID, etc.
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// NewSecretValue specifies how to generate the new secret value
type NewSecretValue struct {
	Type   ValueType             `json:"type"`
	Config map[string]interface{} `json:"config,omitempty"`
	Value  string                `json:"value,omitempty"` // For literal values
}

// ValueType specifies how to generate new secret values
type ValueType string

const (
	ValueTypeRandom      ValueType = "random"      // Generate random value
	ValueTypeLiteral     ValueType = "literal"     // Use provided value
	ValueTypeFile        ValueType = "file"        // Read from file
	ValueTypeGenerated   ValueType = "generated"   // Provider generates (API key, cert)
	ValueTypeRotated     ValueType = "rotated"     // Provider rotates (OAuth refresh)
)

// SecretType categorizes the type of secret being rotated
type SecretType string

const (
	SecretTypePassword    SecretType = "password"    // Database passwords
	SecretTypeAPIKey      SecretType = "api_key"     // API keys, tokens
	SecretTypeCertificate SecretType = "certificate" // TLS certificates
	SecretTypeOAuth       SecretType = "oauth"       // OAuth tokens, client secrets
	SecretTypeEncryption  SecretType = "encryption"  // Application encryption keys
	SecretTypeGeneric     SecretType = "generic"     // Other secrets
)

// RotationStatus represents the current state of secret rotation
type RotationStatus string

const (
	StatusPending     RotationStatus = "pending"      // Ready to rotate
	StatusRotating    RotationStatus = "rotating"     // In progress
	StatusVerifying   RotationStatus = "verifying"    // Verifying new value
	StatusCompleted   RotationStatus = "completed"    // Successfully rotated
	StatusFailed      RotationStatus = "failed"       // Rotation failed
	StatusRolledBack  RotationStatus = "rolled_back"  // Rolled back to old value
	StatusDeprecated  RotationStatus = "deprecated"   // Old version deprecated
)

// RotationConstraints define limits and requirements for rotation
type RotationConstraints struct {
	MinRotationInterval time.Duration     `json:"min_rotation_interval,omitempty"`
	MaxValueLength      int               `json:"max_value_length,omitempty"`
	MinValueLength      int               `json:"min_value_length,omitempty"`
	AllowedCharsets     []string          `json:"allowed_charsets,omitempty"`
	RequiredTests       []VerificationTest `json:"required_tests,omitempty"`
	GracePeriod         time.Duration     `json:"grace_period,omitempty"`
	NotificationRequired bool             `json:"notification_required,omitempty"`
}

// VerificationTest defines how to verify a rotated secret works
type VerificationTest struct {
	Name        string        `json:"name"`
	Type        TestType      `json:"type"`
	Config      map[string]interface{} `json:"config,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`
	Required    bool          `json:"required"`
	Description string        `json:"description,omitempty"`
}

// TestType specifies the type of verification test
type TestType string

const (
	TestTypeConnection  TestType = "connection"   // Test database/service connection
	TestTypeQuery       TestType = "query"        // Execute test query/request
	TestTypeAPI         TestType = "api"          // Test API endpoint
	TestTypePing        TestType = "ping"         // Simple connectivity test
	TestTypeCustom      TestType = "custom"       // Custom test script
)

// VerificationResult contains the outcome of a verification test
type VerificationResult struct {
	Test        VerificationTest `json:"test"`
	Status      TestStatus       `json:"status"`
	Duration    time.Duration    `json:"duration"`
	Message     string           `json:"message,omitempty"`
	Error       string           `json:"error,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

// TestStatus represents the result of a verification test
type TestStatus string

const (
	TestStatusPassed  TestStatus = "passed"
	TestStatusFailed  TestStatus = "failed"
	TestStatusSkipped TestStatus = "skipped"
	TestStatusTimeout TestStatus = "timeout"
)

// AuditEntry records an action taken during rotation
type AuditEntry struct {
	Timestamp   time.Time         `json:"timestamp"`
	Action      string            `json:"action"`
	Component   string            `json:"component"`
	Status      string            `json:"status"`
	Message     string            `json:"message,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Error       string            `json:"error,omitempty"`
}

// RotationStatusInfo contains detailed information about rotation status
type RotationStatusInfo struct {
	Status           RotationStatus `json:"status"`
	LastRotated      *time.Time     `json:"last_rotated,omitempty"`
	NextRotation     *time.Time     `json:"next_rotation,omitempty"`
	RotationVersion  string         `json:"rotation_version,omitempty"`
	CanRotate        bool           `json:"can_rotate"`
	Reason           string         `json:"reason,omitempty"`
}

// RotationEngine orchestrates rotation across multiple strategies.
//
// The RotationEngine serves as the central coordination point for all secret rotation
// operations in dsops. It manages a registry of rotation strategies, routes rotation
// requests to appropriate strategies, and provides higher-level rotation operations
// like batch processing and scheduling.
//
// Key responsibilities:
//   - Strategy registration and discovery
//   - Request routing based on secret type and strategy name
//   - Batch operations for rotating multiple secrets
//   - Rotation history tracking and retrieval
//   - Scheduling future rotations
//   - Error aggregation and reporting
//
// Example usage:
//
//	// Create and configure engine
//	engine := NewRotationEngine()
//	
//	// Register rotation strategies
//	engine.RegisterStrategy(&PostgreSQLRotator{})
//	engine.RegisterStrategy(&StripeRotator{})
//	engine.RegisterStrategy(&GenericAPIKeyRotator{})
//	
//	// Perform single rotation
//	request := RotationRequest{
//	    Secret: SecretInfo{Key: "db_password", Provider: "postgresql"},
//	    Strategy: "postgresql",
//	}
//	result, err := engine.Rotate(ctx, request)
//	
//	// Batch rotate multiple secrets
//	requests := []RotationRequest{dbRequest, apiRequest, certRequest}
//	results, err := engine.BatchRotate(ctx, requests)
//	
//	// Schedule future rotation
//	nextWeek := time.Now().Add(7 * 24 * time.Hour)
//	err = engine.ScheduleRotation(ctx, request, nextWeek)
type RotationEngine interface {
	// RegisterStrategy adds a rotation strategy to the engine's registry.
	//
	// This method registers a new rotation strategy that can handle specific
	// types of secrets or services. The strategy's Name() method must return
	// a unique identifier that will be used to route rotation requests.
	//
	// Multiple strategies can be registered, and the engine will route requests
	// based on the strategy name specified in RotationRequest.
	//
	// Returns error if:
	//   - A strategy with the same name is already registered
	//   - The strategy is nil or invalid
	//
	// Example:
	//
	//	postgresRotator := &PostgreSQLRotator{}
	//	if err := engine.RegisterStrategy(postgresRotator); err != nil {
	//	    return fmt.Errorf("failed to register postgres rotator: %w", err)
	//	}
	RegisterStrategy(strategy SecretValueRotator) error

	// GetStrategy returns a registered rotation strategy by name.
	//
	// This method looks up a previously registered strategy by its name.
	// Used internally for request routing and can be used externally for
	// direct strategy access or introspection.
	//
	// Returns error if no strategy with the given name is registered.
	//
	// Example:
	//
	//	strategy, err := engine.GetStrategy("postgresql")
	//	if err != nil {
	//	    return fmt.Errorf("postgresql strategy not available: %w", err)
	//	}
	//	if strategy.SupportsSecret(ctx, secretInfo) {
	//	    // Use strategy for custom rotation logic
	//	}
	GetStrategy(name string) (SecretValueRotator, error)

	// ListStrategies returns names of all registered rotation strategies.
	//
	// This method provides discovery of available strategies for UI display,
	// configuration validation, and operational introspection.
	//
	// Returns empty slice if no strategies are registered.
	//
	// Example:
	//
	//	strategies := engine.ListStrategies()
	//	fmt.Printf("Available rotation strategies: %v\n", strategies)
	//	for _, name := range strategies {
	//	    strategy, _ := engine.GetStrategy(name)
	//	    caps := strategy.Capabilities() // If strategy implements capability interface
	//	}
	ListStrategies() []string

	// Rotate performs rotation using the appropriate strategy.
	//
	// This is the primary method for executing rotation operations. It:
	//   - Routes the request to the specified strategy
	//   - Validates the request and strategy compatibility
	//   - Executes the rotation with proper error handling
	//   - Records results in rotation history
	//   - Handles notifications if configured
	//
	// Returns RotationResult with detailed outcome information, including
	// success/failure status, timing, verification results, and audit trail.
	//
	// Example:
	//
	//	request := RotationRequest{
	//	    Secret: SecretInfo{
	//	        Key:        "database_password",
	//	        Provider:   "postgresql",
	//	        SecretType: SecretTypePassword,
	//	    },
	//	    Strategy:   "postgresql",
	//	    TwoSecret:  true, // Zero-downtime rotation
	//	}
	//	
	//	result, err := engine.Rotate(ctx, request)
	//	if err != nil {
	//	    return fmt.Errorf("rotation failed: %w", err)
	//	}
	//	if result.Status != StatusCompleted {
	//	    return fmt.Errorf("rotation incomplete: %s", result.Error)
	//	}
	Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error)

	// BatchRotate rotates multiple secrets efficiently.
	//
	// This method processes multiple rotation requests, potentially in parallel,
	// with proper error handling and coordination. It's more efficient than
	// calling Rotate() multiple times for bulk operations.
	//
	// The method handles:
	//   - Parallel execution where safe
	//   - Dependency ordering (rotate A before B)
	//   - Error isolation (failure of one doesn't stop others)
	//   - Progress reporting for long-running operations
	//
	// Returns slice of RotationResult corresponding to input requests.
	// Check individual results for per-secret success/failure status.
	//
	// Example:
	//
	//	requests := []RotationRequest{
	//	    {Secret: dbSecret, Strategy: "postgresql"},
	//	    {Secret: apiSecret, Strategy: "stripe"},
	//	    {Secret: certSecret, Strategy: "certificate"},
	//	}
	//	
	//	results, err := engine.BatchRotate(ctx, requests)
	//	if err != nil {
	//	    return fmt.Errorf("batch rotation failed: %w", err)
	//	}
	//	
	//	for i, result := range results {
	//	    if result.Status != StatusCompleted {
	//	        log.Errorf("Request %d failed: %s", i, result.Error)
	//	    }
	//	}
	BatchRotate(ctx context.Context, requests []RotationRequest) ([]RotationResult, error)

	// GetRotationHistory returns rotation history for a specific secret.
	//
	// This method retrieves historical rotation records for audit, monitoring,
	// and troubleshooting purposes. History includes successful rotations,
	// failures, and any rollback operations.
	//
	// The limit parameter controls how many historical records to return.
	// Use 0 for no limit, positive number for most recent N records.
	//
	// Returns records in reverse chronological order (newest first).
	//
	// Example:
	//
	//	// Get last 10 rotation attempts for this secret
	//	history, err := engine.GetRotationHistory(ctx, secretInfo, 10)
	//	if err != nil {
	//	    return fmt.Errorf("failed to get history: %w", err)
	//	}
	//	
	//	for _, record := range history {
	//	    fmt.Printf("%s: %s (status: %s)\n", 
	//	        record.RotatedAt.Format(time.RFC3339),
	//	        record.Secret.Key,
	//	        record.Status)
	//	}
	GetRotationHistory(ctx context.Context, secret SecretInfo, limit int) ([]RotationResult, error)

	// ScheduleRotation schedules a rotation for future execution.
	//
	// This method adds a rotation request to the scheduling system for execution
	// at a specified time. Useful for planned maintenance windows, regular
	// rotation cycles, and coordinated multi-secret rotations.
	//
	// The scheduling system should handle:
	//   - Persistent storage of scheduled operations
	//   - Reliable execution at the specified time
	//   - Retry logic for transient failures
	//   - Notification of schedule completion
	//
	// Example:
	//
	//	// Schedule rotation for next maintenance window
	//	maintenanceWindow := time.Date(2024, 1, 15, 2, 0, 0, 0, time.UTC)
	//	request := RotationRequest{
	//	    Secret:   criticalSecret,
	//	    Strategy: "postgresql",
	//	    TwoSecret: true, // Zero downtime
	//	}
	//	
	//	if err := engine.ScheduleRotation(ctx, request, maintenanceWindow); err != nil {
	//	    return fmt.Errorf("failed to schedule rotation: %w", err)
	//	}
	ScheduleRotation(ctx context.Context, request RotationRequest, when time.Time) error
}