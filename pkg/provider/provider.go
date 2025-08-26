// Package provider defines the core interfaces and types for secret store providers in dsops.
//
// This package provides the foundational abstraction for accessing secrets from various
// storage systems like AWS Secrets Manager, HashiCorp Vault, 1Password, Azure Key Vault,
// and others. All provider implementations must implement the Provider interface to
// ensure consistent behavior across different secret storage systems.
//
// # Provider Architecture
//
// dsops separates secret stores (where secrets are stored) from services (what uses secrets).
// This package focuses on secret stores - the systems that store and retrieve secret values.
//
// The Provider interface provides a uniform API for:
//   - Resolving secret values from storage
//   - Describing secret metadata without retrieving values
//   - Validating provider configuration and connectivity
//   - Exposing provider capabilities
//
// # Implementing a Custom Provider
//
// To implement a custom provider:
//
//  1. Implement the Provider interface
//  2. Optionally implement Rotator for rotation support
//  3. Register your provider in the provider registry
//  4. Add configuration support
//
// Example:
//
//	type MyProvider struct {
//	    config MyProviderConfig
//	}
//
//	func (p *MyProvider) Name() string {
//	    return "my-provider"
//	}
//
//	func (p *MyProvider) Resolve(ctx context.Context, ref Reference) (SecretValue, error) {
//	    // Fetch secret from your storage system
//	    value, err := p.fetchSecret(ref.Key)
//	    if err != nil {
//	        return SecretValue{}, err
//	    }
//	    
//	    return SecretValue{
//	        Value:     value,
//	        Version:   "1",
//	        UpdatedAt: time.Now(),
//	    }, nil
//	}
//
//	// ... implement other methods
//
// # Error Handling
//
// Providers should use the standard error types defined in this package:
//   - NotFoundError for missing secrets
//   - AuthError for authentication failures
//   - Standard Go errors for other cases
//
// # Security Considerations
//
// Providers must:
//   - Never log secret values (use logging.Secret() wrapper)
//   - Validate authentication before operations
//   - Handle network timeouts gracefully
//   - Support context cancellation
//   - Use secure transport (TLS) when applicable
//
// # Threading and Concurrency
//
// Provider implementations must be thread-safe. Multiple goroutines may call
// provider methods concurrently. Use appropriate synchronization mechanisms
// if your provider maintains internal state.
package provider

import (
	"context"
	"time"
)

// Provider defines the interface that all secret store providers must implement.
//
// The Provider interface abstracts different secret storage systems (AWS Secrets Manager,
// HashiCorp Vault, 1Password, etc.) behind a common API. This enables dsops to work
// with multiple secret stores through a unified interface.
//
// Implementations must be thread-safe as multiple goroutines may call these methods
// concurrently.
//
// Example usage:
//
//	provider := &MyProvider{config: cfg}
//	if err := provider.Validate(ctx); err != nil {
//	    return fmt.Errorf("provider validation failed: %w", err)
//	}
//	
//	ref := Reference{Provider: "my-provider", Key: "api-key"}
//	secret, err := provider.Resolve(ctx, ref)
//	if err != nil {
//	    return fmt.Errorf("failed to resolve secret: %w", err)
//	}
type Provider interface {
	// Name returns the provider's unique identifier.
	//
	// This should be a stable, lowercase identifier that matches the provider type
	// used in configuration files. Examples: "aws.secretsmanager", "hashicorp.vault", 
	// "onepassword", "literal".
	//
	// The name is used for logging, error messages, and provider registration.
	Name() string

	// Resolve retrieves a secret value from the provider.
	//
	// This is the core method that fetches actual secret values from the storage system.
	// The Reference parameter specifies which secret to retrieve, including any
	// provider-specific addressing information.
	//
	// Implementations should:
	//   - Support context cancellation
	//   - Return NotFoundError for missing secrets
	//   - Return AuthError for authentication failures
	//   - Include metadata like version and update time when available
	//   - Never log the secret value
	//
	// Example:
	//
	//	ref := Reference{
	//	    Provider: "aws.secretsmanager",
	//	    Key:      "prod/database/password",
	//	    Version:  "AWSCURRENT",
	//	}
	//	secret, err := provider.Resolve(ctx, ref)
	//	if err != nil {
	//	    return err
	//	}
	//	fmt.Println("Retrieved secret version:", secret.Version)
	Resolve(ctx context.Context, ref Reference) (SecretValue, error)

	// Describe returns metadata about a secret without retrieving its value.
	//
	// This method provides information about a secret's existence, size, version,
	// and other attributes without exposing the actual secret value. It's useful
	// for validation, planning, and auditing operations.
	//
	// Returns Metadata with Exists=false if the secret doesn't exist.
	// Should not return NotFoundError - use Metadata.Exists field instead.
	//
	// Implementations should:
	//   - Be faster than Resolve since no secret value is retrieved
	//   - Support context cancellation
	//   - Include available metadata like version, size, tags
	//   - Return empty Metadata with Exists=false for missing secrets
	//
	// Example:
	//
	//	meta, err := provider.Describe(ctx, ref)
	//	if err != nil {
	//	    return err
	//	}
	//	if !meta.Exists {
	//	    fmt.Println("Secret does not exist")
	//	} else {
	//	    fmt.Printf("Secret size: %d bytes, version: %s\n", meta.Size, meta.Version)
	//	}
	Describe(ctx context.Context, ref Reference) (Metadata, error)

	// Capabilities returns the provider's supported features and limitations.
	//
	// This method exposes what functionality the provider supports, such as:
	//   - Version management (multiple versions of secrets)
	//   - Metadata support (tags, descriptions, etc.)
	//   - Binary data support (certificates, keys)
	//   - Real-time change notifications
	//   - Available authentication methods
	//
	// This information is used by dsops to:
	//   - Validate configuration compatibility
	//   - Enable/disable features based on provider support
	//   - Provide appropriate user feedback
	//   - Route operations to capable providers
	//
	// Example:
	//
	//	caps := provider.Capabilities()
	//	if !caps.SupportsVersioning {
	//	    fmt.Println("Warning: Provider doesn't support versioning")
	//	}
	//	if caps.RequiresAuth {
	//	    fmt.Printf("Authentication methods: %v\n", caps.AuthMethods)
	//	}
	Capabilities() Capabilities

	// Validate checks if the provider is properly configured and authenticated.
	//
	// This method verifies that the provider can successfully connect to its
	// backend system and has appropriate permissions. It should be called
	// before performing any secret operations.
	//
	// Implementations should:
	//   - Test connectivity to the backend system
	//   - Verify authentication credentials
	//   - Check minimum required permissions
	//   - Support context cancellation and timeouts
	//   - Return specific AuthError for auth failures
	//   - Return descriptive errors for other issues
	//
	// Common validation checks:
	//   - Network connectivity to the secret store
	//   - API credentials are valid and not expired
	//   - Required permissions are granted
	//   - Provider-specific configuration is correct
	//
	// Example:
	//
	//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	//	defer cancel()
	//	
	//	if err := provider.Validate(ctx); err != nil {
	//	    var authErr AuthError
	//	    if errors.As(err, &authErr) {
	//	        fmt.Printf("Authentication failed: %v\n", authErr)
	//	    } else {
	//	        fmt.Printf("Validation failed: %v\n", err)
	//	    }
	//	}
	Validate(ctx context.Context) error
}

// Reference identifies a secret within a provider.
//
// Different providers use different addressing schemes for secrets:
//   - AWS Secrets Manager: Key is the secret name/ARN
//   - HashiCorp Vault: Path is the full path, Key is the field name
//   - 1Password: Key is the item ID, Field is the field name
//   - Azure Key Vault: Key is the secret name
//
// Examples:
//
//	// AWS Secrets Manager
//	ref := Reference{
//	    Provider: "aws.secretsmanager",
//	    Key:      "prod/database/password",
//	    Version:  "AWSCURRENT", // Optional: specific version
//	}
//
//	// HashiCorp Vault KV v2
//	ref := Reference{
//	    Provider: "hashicorp.vault",
//	    Path:     "secret/data/app",
//	    Key:      "api_key",
//	    Version:  "2", // Optional: version number
//	}
//
//	// 1Password
//	ref := Reference{
//	    Provider: "onepassword",
//	    Key:      "database-item-id",
//	    Field:    "password",
//	}
type Reference struct {
	// Provider is the name of the provider that owns this secret.
	// Must match the provider's Name() method return value.
	Provider string

	// Key identifies the secret within the provider's namespace.
	// For most providers, this is the primary identifier (secret name, item ID, etc.).
	Key string

	// Version specifies a particular version of the secret.
	// Optional - if empty, the provider should return the current/latest version.
	// Version semantics are provider-specific:
	//   - AWS: "AWSCURRENT", "AWSPENDING", or UUID
	//   - Vault: Integer version number as string
	//   - Others: Provider-defined versioning scheme
	Version string

	// Path provides hierarchical addressing for providers that support it.
	// Used by providers like HashiCorp Vault where secrets are organized in paths.
	// For other providers, this may be combined with Key or ignored.
	Path string

	// Field specifies a particular field within a structured secret.
	// Used when secrets contain multiple fields (JSON objects, key-value pairs).
	// Examples:
	//   - 1Password: Field name within an item
	//   - Vault: Key within a KV secret
	//   - AWS: JSON path within a JSON secret
	Field string
}

// SecretValue represents a retrieved secret with its metadata.
//
// Contains the actual secret value along with version information and
// timestamps. The Value field contains the raw secret data as a string.
//
// Example:
//
//	secret := SecretValue{
//	    Value:     "super-secret-password",
//	    Version:   "v1.2.3",
//	    UpdatedAt: time.Now(),
//	    Metadata: map[string]string{
//	        "environment": "production",
//	        "owner":       "platform-team",
//	    },
//	}
type SecretValue struct {
	// Value is the actual secret data as a string.
	// For binary data, this should be base64 encoded.
	// Providers must never log this field.
	Value string

	// Version identifies the specific version of this secret.
	// Format is provider-specific. May be empty if versioning not supported.
	Version string

	// UpdatedAt indicates when this secret was last modified.
	// May be zero time if the provider doesn't support timestamps.
	UpdatedAt time.Time

	// Metadata contains provider-specific information about the secret.
	// Common keys include:
	//   - "created_by": Who created the secret
	//   - "environment": Environment tag
	//   - "rotation_id": Rotation tracking ID  
	//   - "content_type": MIME type for binary data
	Metadata map[string]string
}

// Metadata describes a secret without exposing its value.
//
// Used by the Describe method to provide information about a secret's
// existence, properties, and attributes without retrieving the actual
// secret value. Useful for validation, auditing, and planning operations.
//
// Example:
//
//	meta := Metadata{
//	    Exists:      true,
//	    Version:     "AWSCURRENT", 
//	    UpdatedAt:   time.Now(),
//	    Size:        256, // bytes
//	    Type:        "password",
//	    Permissions: []string{"read", "list"},
//	    Tags: map[string]string{
//	        "environment": "production",
//	        "team":        "platform",
//	    },
//	}
type Metadata struct {
	// Exists indicates whether the secret exists in the provider.
	// If false, other fields may be empty or meaningless.
	Exists bool

	// Version identifies the current version of the secret.
	// Empty if versioning not supported or secret doesn't exist.
	Version string

	// UpdatedAt indicates when the secret was last modified.
	// Zero time if not supported or secret doesn't exist.
	UpdatedAt time.Time

	// Size is the approximate size of the secret value in bytes.
	// May be 0 if not supported or not available.
	Size int

	// Type describes the kind of secret (password, certificate, api_key, etc.).
	// Provider-specific classification. May be empty.
	Type string

	// Permissions lists the operations the current credentials can perform
	// on this secret. Common values: "read", "write", "delete", "list".
	// Empty slice if not supported.
	Permissions []string

	// Tags contains provider-specific metadata and labels.
	// Common keys include environment, owner, purpose, etc.
	// Empty map if not supported.
	Tags map[string]string
}

// Capabilities describes what features and operations a provider supports.
//
// This structure allows dsops to adapt its behavior based on provider
// capabilities, enable/disable features appropriately, and provide
// accurate user feedback about what operations are possible.
//
// Example:
//
//	caps := Capabilities{
//	    SupportsVersioning: true,
//	    SupportsMetadata:   true, 
//	    SupportsWatching:   false,
//	    SupportsBinary:     true,
//	    RequiresAuth:       true,
//	    AuthMethods:        []string{"api_key", "oauth2"},
//	}
type Capabilities struct {
	// SupportsVersioning indicates if the provider maintains multiple versions
	// of secrets and can retrieve specific versions.
	SupportsVersioning bool

	// SupportsMetadata indicates if the provider supports additional metadata
	// like tags, descriptions, and custom attributes beyond the secret value.
	SupportsMetadata bool

	// SupportsWatching indicates if the provider can notify about secret changes
	// in real-time or support long-polling for updates.
	SupportsWatching bool

	// SupportsBinary indicates if the provider can store and retrieve binary data
	// (certificates, keys, images) or only text-based secrets.
	SupportsBinary bool

	// RequiresAuth indicates if the provider requires authentication to access secrets.
	// If false, the provider may work without credentials (e.g., literal provider).
	RequiresAuth bool

	// AuthMethods lists the authentication methods supported by this provider.
	// Common values include:
	//   - "api_key": API key/token authentication
	//   - "basic": Username/password authentication  
	//   - "oauth2": OAuth2 flow
	//   - "iam": Cloud IAM roles
	//   - "certificate": Client certificate authentication
	//   - "cli": CLI-based authentication (like aws configure)
	AuthMethods []string
}

// NotFoundError indicates that a requested secret does not exist in the provider.
//
// Providers should return this error when a secret reference points to a
// non-existent secret. This is distinct from authentication failures or
// permission errors.
//
// Example:
//
//	if !secretExists(ref.Key) {
//	    return SecretValue{}, NotFoundError{
//	        Provider: p.Name(),
//	        Key:      ref.Key,
//	    }
//	}
type NotFoundError struct {
	// Provider is the name of the provider where the secret was not found.
	Provider string
	
	// Key is the secret identifier that could not be found.
	Key string
}

// Error implements the error interface.
func (e NotFoundError) Error() string {
	return "secret not found: " + e.Key + " in " + e.Provider
}

// AuthError indicates that authentication to the provider failed.
//
// This error should be returned when:
//   - Credentials are invalid or expired
//   - Authentication method is not supported
//   - Network authentication fails
//   - Permission is denied for the requested operation
//
// Example:
//
//	if !isAuthenticated() {
//	    return SecretValue{}, AuthError{
//	        Provider: p.Name(),
//	        Message:  "API key is invalid or expired",
//	    }
//	}
type AuthError struct {
	// Provider is the name of the provider that failed authentication.
	Provider string
	
	// Message provides details about the authentication failure.
	Message string
}

// Error implements the error interface.
func (e AuthError) Error() string {
	return "authentication failed for " + e.Provider + ": " + e.Message
}

// Rotator defines the interface for providers that support secret rotation within the storage system.
//
// This interface extends the basic Provider functionality to enable providers to
// participate in secret rotation workflows. Not all providers support rotation -
// some may only support secret retrieval.
//
// Providers implementing this interface can:
//   - Create new versions of existing secrets
//   - Deprecate old versions during rotation
//   - Provide metadata about rotation capabilities and constraints
//
// This is distinct from the rotation.SecretValueRotator interface, which handles
// service-side rotation (e.g., updating database passwords, rotating API keys).
// The Rotator interface here handles storage-side operations.
//
// Example implementation:
//
//	func (p *MyProvider) CreateNewVersion(ctx context.Context, ref Reference, 
//	    newValue []byte, meta map[string]string) (string, error) {
//	    
//	    version, err := p.client.CreateSecretVersion(ref.Key, newValue, meta)
//	    if err != nil {
//	        return "", fmt.Errorf("failed to create new version: %w", err)
//	    }
//	    return version, nil
//	}
type Rotator interface {
	// CreateNewVersion creates a new version of an existing secret in the storage system.
	//
	// This method adds a new version to an existing secret without removing or
	// modifying existing versions. The new version typically becomes the "current"
	// or "active" version for future retrievals.
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts
	//   - ref: Reference to the secret to update
	//   - newValue: The new secret value as bytes
	//   - meta: Optional metadata to associate with the new version
	//
	// Returns:
	//   - string: Version identifier for the newly created version
	//   - error: Any error that occurred during creation
	//
	// The returned version string should be usable in subsequent Reference.Version
	// fields to retrieve this specific version.
	//
	// Example:
	//
	//	newPassword := []byte("new-secure-password")
	//	meta := map[string]string{
	//	    "rotated_by": "dsops",
	//	    "timestamp":  time.Now().Format(time.RFC3339),
	//	}
	//	version, err := rotator.CreateNewVersion(ctx, ref, newPassword, meta)
	//	if err != nil {
	//	    return fmt.Errorf("rotation failed: %w", err)
	//	}
	//	log.Printf("Created new version: %s", version)
	CreateNewVersion(ctx context.Context, ref Reference, newValue []byte, meta map[string]string) (string, error)
	
	// DeprecateVersion marks an old version as deprecated, disabled, or deleted.
	//
	// This method is called during rotation cleanup to remove or disable old
	// versions after a new version has been successfully deployed and verified.
	// The exact behavior depends on the provider:
	//   - Some providers may delete the version entirely
	//   - Others may mark it as deprecated but keep it for rollback
	//   - Some may disable it but retain it for audit purposes
	//
	// Parameters:
	//   - ctx: Context for cancellation and timeouts  
	//   - ref: Reference to the secret
	//   - version: Specific version to deprecate
	//
	// The version parameter should match a version string previously returned
	// by CreateNewVersion or retrieved via Resolve/Describe.
	//
	// Example:
	//
	//	// After successful rotation and verification
	//	if err := rotator.DeprecateVersion(ctx, ref, oldVersion); err != nil {
	//	    log.Warnf("Failed to deprecate old version %s: %v", oldVersion, err)
	//	    // This might not be fatal - new version is already active
	//	}
	DeprecateVersion(ctx context.Context, ref Reference, version string) error
	
	// GetRotationMetadata returns information about rotation capabilities and constraints.
	//
	// This method provides details about what rotation operations are supported
	// for a specific secret, including constraints on value length, allowed
	// characters, rotation frequency, and other provider-specific limitations.
	//
	// Used by the rotation engine to:
	//   - Validate rotation requests before attempting them
	//   - Generate appropriate new values within constraints
	//   - Schedule rotations according to provider limitations
	//   - Provide user feedback about rotation capabilities
	//
	// Example:
	//
	//	meta, err := rotator.GetRotationMetadata(ctx, ref)
	//	if err != nil {
	//	    return err
	//	}
	//	if !meta.SupportsRotation {
	//	    return fmt.Errorf("secret %s does not support rotation", ref.Key)
	//	}
	//	if meta.MinValueLength > 0 && len(newValue) < meta.MinValueLength {
	//	    return fmt.Errorf("new value too short, minimum %d characters", 
	//	        meta.MinValueLength)
	//	}
	GetRotationMetadata(ctx context.Context, ref Reference) (RotationMetadata, error)
}

// RotationMetadata describes rotation capabilities and constraints for a specific secret.
//
// This structure provides detailed information about what rotation operations
// are supported and what constraints apply to new secret values. Different
// providers and even different secrets within the same provider may have
// varying capabilities.
//
// Example:
//
//	meta := RotationMetadata{
//	    SupportsRotation:   true,
//	    SupportsVersioning: true,
//	    MaxValueLength:     4096,
//	    MinValueLength:     12,
//	    AllowedCharacters:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*",
//	    RotationInterval:   "30d",
//	    LastRotated:        &lastRotationTime,
//	    NextRotation:       &nextRotationTime,
//	    Constraints: map[string]string{
//	        "complexity": "high",
//	        "policy":     "corporate-password-policy",
//	    },
//	}
type RotationMetadata struct {
	// SupportsRotation indicates if this specific secret can be rotated.
	// Even if the provider implements Rotator, individual secrets may not
	// support rotation due to their type, configuration, or permissions.
	SupportsRotation bool `json:"supports_rotation"`

	// SupportsVersioning indicates if the secret supports multiple concurrent versions.
	// If false, rotation may require immediate replacement without overlap periods.
	SupportsVersioning bool `json:"supports_versioning"`

	// MaxValueLength specifies the maximum allowed length for secret values in bytes.
	// Zero means no limit or limit is unknown.
	MaxValueLength int `json:"max_value_length,omitempty"`

	// MinValueLength specifies the minimum required length for secret values in bytes.
	// Zero means no minimum or minimum is unknown.
	MinValueLength int `json:"min_value_length,omitempty"`

	// AllowedCharacters specifies the character set allowed in secret values.
	// Empty means all characters are allowed or constraint is unknown.
	// Format is a string containing all allowed characters.
	AllowedCharacters string `json:"allowed_characters,omitempty"`

	// RotationInterval specifies the recommended or required rotation frequency.
	// Format follows Go duration syntax: "30d", "90d", "24h", etc.
	// Empty means no specific interval is required.
	RotationInterval string `json:"rotation_interval,omitempty"`

	// LastRotated indicates when this secret was last rotated.
	// Nil if never rotated or information is unavailable.
	LastRotated *time.Time `json:"last_rotated,omitempty"`

	// NextRotation indicates when this secret should next be rotated.
	// Nil if no rotation is scheduled or information is unavailable.
	NextRotation *time.Time `json:"next_rotation,omitempty"`

	// Constraints contains additional provider-specific rotation constraints.
	// Common keys might include:
	//   - "complexity": Password complexity requirements
	//   - "policy": Associated policy name or ID
	//   - "approval": Whether rotation requires approval
	//   - "notification": Required notification methods
	Constraints map[string]string `json:"constraints,omitempty"`
}