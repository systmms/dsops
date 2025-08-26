// Package secretstore provides interfaces and types for secret storage systems in dsops.
//
// This package defines the modern secret store abstraction that replaces the older
// provider interface. Secret stores are systems that store and retrieve secret values,
// as distinct from services that consume secrets for operational purposes.
//
// # Secret Store vs Service Distinction
//
// dsops separates two key concepts:
//   - **Secret Stores**: Where secrets are stored (AWS Secrets Manager, Vault, 1Password, etc.)
//   - **Services**: What uses secrets (PostgreSQL, Stripe API, GitHub, etc.)
//
// This package focuses exclusively on secret stores - the storage layer.
//
// # Modern Reference Format
//
// This package uses the new URI-based reference format:
//
//	store://store-name/path#field?version=v&option=value
//
// Examples:
//   - store://aws-prod/database/password
//   - store://vault/secret/app#api_key?version=2
//   - store://onepassword/Production/Database#password
//
// # Key Features
//
//   - **URI-based references**: Consistent addressing across all stores
//   - **Capability negotiation**: Stores expose their supported features
//   - **Version support**: Built-in support for secret versioning
//   - **Metadata access**: Describe secrets without retrieving values
//   - **Standardized errors**: Consistent error types across stores
//   - **Rotation integration**: Native support for secret rotation
//
// # Implementing a Secret Store
//
// To implement a custom secret store:
//
//  1. Implement the SecretStore interface
//  2. Handle URI parsing for your store's format
//  3. Provide appropriate capabilities
//  4. Register with the secret store registry
//
// Example:
//
//	type MySecretStore struct {
//	    client MyStoreClient
//	    name   string
//	}
//	
//	func (s *MySecretStore) Name() string {
//	    return s.name
//	}
//	
//	func (s *MySecretStore) Resolve(ctx context.Context, ref SecretRef) (SecretValue, error) {
//	    // Validate reference format
//	    if !ref.IsValid() {
//	        return SecretValue{}, ValidationError{
//	            Store:   s.name,
//	            Message: "invalid secret reference",
//	        }
//	    }
//	    
//	    // Retrieve secret from your storage system
//	    value, err := s.client.GetSecret(ref.Path)
//	    if err != nil {
//	        if isNotFound(err) {
//	            return SecretValue{}, NotFoundError{
//	                Store: s.name,
//	                Path:  ref.Path,
//	            }
//	        }
//	        return SecretValue{}, err
//	    }
//	    
//	    // Extract specific field if requested
//	    if ref.Field != "" {
//	        fieldValue, err := extractField(value, ref.Field)
//	        if err != nil {
//	            return SecretValue{}, err
//	        }
//	        value = fieldValue
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
// Use the standardized error types:
//   - NotFoundError: Secret doesn't exist
//   - AuthError: Authentication failed
//   - ValidationError: Invalid request or configuration
//
// # Security Considerations
//
// Secret store implementations must:
//   - Never log secret values (use logging.Secret wrapper)
//   - Validate all inputs to prevent injection attacks
//   - Use secure transport (TLS) for network operations
//   - Support context cancellation for timeouts
//   - Handle concurrent access safely
package secretstore

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// SecretStore defines the interface for systems that store and retrieve secrets.
//
// This interface replaces the storage functionality from the original Provider interface,
// providing a cleaner separation between secret stores (storage) and services (consumers).
//
// All secret store implementations must be thread-safe as multiple goroutines may
// call these methods concurrently.
//
// Example usage:
//
//	store := &MySecretStore{}
//	if err := store.Validate(ctx); err != nil {
//	    return fmt.Errorf("store validation failed: %w", err)
//	}
//	
//	ref := SecretRef{
//	    Store: "my-store",
//	    Path:  "app/database/password",
//	}
//	
//	secret, err := store.Resolve(ctx, ref)
//	if err != nil {
//	    return fmt.Errorf("failed to resolve secret: %w", err)
//	}
type SecretStore interface {
	// Name returns the secret store's unique identifier.
	//
	// This should be a stable identifier that matches the store name used in
	// configuration and URI references. Examples: "aws-prod", "vault-dev", "onepassword".
	//
	// The name is used for:
	//   - Error messages and logging
	//   - Store registration and lookup
	//   - Configuration validation
	Name() string

	// Resolve retrieves a secret value from the store.
	//
	// This is the primary method for accessing secret values. It takes a SecretRef
	// that specifies the exact secret to retrieve, including optional field
	// extraction and version selection.
	//
	// The method should:
	//   - Support context cancellation and timeouts
	//   - Return NotFoundError for missing secrets
	//   - Return AuthError for authentication failures
	//   - Extract specific fields if ref.Field is specified
	//   - Handle version selection if ref.Version is specified
	//   - Never log the secret value
	//
	// Example:
	//
	//	ref := SecretRef{
	//	    Store:   "aws-secrets",
	//	    Path:    "prod/database/credentials",
	//	    Field:   "password",
	//	    Version: "AWSCURRENT",
	//	}
	//	
	//	secret, err := store.Resolve(ctx, ref)
	//	if err != nil {
	//	    var notFound NotFoundError
	//	    if errors.As(err, &notFound) {
	//	        // Handle missing secret
	//	    }
	//	    return err
	//	}
	Resolve(ctx context.Context, ref SecretRef) (SecretValue, error)

	// Describe returns metadata about a secret without retrieving its value.
	//
	// This method provides information about a secret's existence, properties,
	// and attributes without exposing the actual secret value. It's useful for:
	//   - Validation and planning operations
	//   - Checking secret existence before retrieval
	//   - Gathering metadata for audit and reporting
	//
	// Unlike Resolve, this method should:
	//   - Be faster since no secret value is retrieved
	//   - Return metadata with Exists=false for missing secrets
	//   - Not return NotFoundError (use Exists field instead)
	//   - Include available version and size information
	//
	// Example:
	//
	//	meta, err := store.Describe(ctx, ref)
	//	if err != nil {
	//	    return err
	//	}
	//	if !meta.Exists {
	//	    fmt.Println("Secret does not exist")
	//	} else {
	//	    fmt.Printf("Secret size: %d bytes, version: %s\n", meta.Size, meta.Version)
	//	}
	Describe(ctx context.Context, ref SecretRef) (SecretMetadata, error)

	// Capabilities returns the secret store's supported features and limitations.
	//
	// This method exposes what functionality the secret store supports, allowing
	// dsops to adapt its behavior accordingly. The capabilities are used for:
	//   - Feature availability checks
	//   - Configuration validation
	//   - UI/CLI feature enablement
	//   - Rotation planning
	//
	// Example:
	//
	//	caps := store.Capabilities()
	//	if !caps.SupportsVersioning {
	//	    fmt.Println("Warning: Store doesn't support versioning")
	//	}
	//	if caps.Rotation != nil && caps.Rotation.SupportsRotation {
	//	    fmt.Println("Store supports secret rotation")
	//	}
	Capabilities() SecretStoreCapabilities

	// Validate checks if the secret store is properly configured and authenticated.
	//
	// This method verifies that the store can successfully connect to its
	// backend system and has appropriate permissions. It should be called
	// before performing any secret operations.
	//
	// The method should:
	//   - Test connectivity to the backend system
	//   - Verify authentication credentials
	//   - Check minimum required permissions
	//   - Support context cancellation
	//   - Return AuthError for authentication failures
	//   - Return descriptive errors for configuration issues
	//
	// Example:
	//
	//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	//	defer cancel()
	//	
	//	if err := store.Validate(ctx); err != nil {
	//	    var authErr AuthError
	//	    if errors.As(err, &authErr) {
	//	        fmt.Printf("Authentication failed: %v\n", authErr)
	//	    } else {
	//	        fmt.Printf("Validation failed: %v\n", err)
	//	    }
	//	    return err
	//	}
	Validate(ctx context.Context) error
}

// SecretRef identifies a secret within a secret store using the modern URI-based format.
//
// This structure represents a parsed store:// URI and provides a standardized way
// to reference secrets across different storage systems. It replaces the older
// Provider+Key format with a more flexible and extensible approach.
//
// Different stores use different addressing patterns:
//   - AWS Secrets Manager: Path is the secret name, Field for JSON extraction
//   - HashiCorp Vault: Path is the full secret path, Field is the key name
//   - 1Password: Path is Vault/Item format, Field is the field name
//   - Azure Key Vault: Path is the secret name, Version for specific versions
//
// Examples:
//
//	// AWS Secrets Manager with JSON field extraction
//	SecretRef{
//	    Store:   "aws-prod",
//	    Path:    "database/credentials",
//	    Field:   "password",
//	    Version: "AWSCURRENT",
//	}
//	
//	// HashiCorp Vault KV store
//	SecretRef{
//	    Store:   "vault",
//	    Path:    "secret/data/app",
//	    Field:   "api_key",
//	    Version: "2",
//	}
//	
//	// 1Password item
//	SecretRef{
//	    Store:   "onepassword",
//	    Path:    "Production/Database",
//	    Field:   "password",
//	    Options: map[string]string{"vault": "Private"},
//	}
type SecretRef struct {
	// Store is the name of the secret store that contains this secret.
	// Must match a configured store name in dsops.yaml.
	Store string

	// Path identifies the secret within the store's namespace.
	// Format varies by store type:
	//   - AWS: Secret name or ARN
	//   - Vault: Full path like "secret/data/app"
	//   - 1Password: "Vault/Item" format
	//   - Azure: Secret name
	Path string

	// Field specifies a particular field within a structured secret.
	// Used when secrets contain multiple values (JSON objects, key-value pairs).
	// Optional - if empty, the entire secret value is returned.
	Field string

	// Version specifies a particular version of the secret.
	// Format is store-specific:
	//   - AWS: "AWSCURRENT", "AWSPENDING", or version UUID
	//   - Vault: Integer version number as string
	//   - Others: Store-defined versioning scheme
	// Optional - if empty, returns the current/latest version.
	Version string

	// Options contains additional store-specific parameters.
	// Common options include:
	//   - "vault": Vault name for 1Password
	//   - "namespace": Namespace for Vault Enterprise
	//   - "region": AWS region override
	Options map[string]string
}

// SecretValue represents a retrieved secret with its metadata.
//
// This structure contains the actual secret value along with version information
// and timestamps. The Value field contains the raw secret data as a string.
// For binary data, this should be base64 encoded by the store implementation.
//
// Example:
//
//	secret := SecretValue{
//	    Value:     "super-secret-password",
//	    Version:   "v1.2.3",
//	    UpdatedAt: time.Now(),
//	    Metadata: map[string]string{
//	        "created_by":   "rotation-service",
//	        "environment": "production",
//	        "owner":       "platform-team",
//	    },
//	}
type SecretValue struct {
	// Value is the actual secret data as a string.
	// For binary data, this should be base64 encoded.
	// Implementations must never log this field.
	Value string

	// Version identifies the specific version of this secret.
	// Format is store-specific. May be empty if versioning not supported.
	Version string

	// UpdatedAt indicates when this secret was last modified.
	// May be zero time if the store doesn't support timestamps.
	UpdatedAt time.Time

	// Metadata contains store-specific information about the secret.
	// Common keys include:
	//   - "created_by": Who created the secret
	//   - "environment": Environment tag (prod, staging, dev)
	//   - "owner": Team or individual responsible
	//   - "rotation_id": Rotation tracking identifier
	//   - "content_type": MIME type for binary data
	Metadata map[string]string
}

// SecretMetadata describes a secret without exposing its value.
//
// Used by the Describe method to provide information about a secret's
// existence, properties, and attributes without retrieving the actual
// secret value. Useful for validation, auditing, and planning operations.
//
// Example:
//
//	meta := SecretMetadata{
//	    Exists:      true,
//	    Version:     "AWSCURRENT",
//	    UpdatedAt:   time.Now(),
//	    Size:        256, // bytes
//	    Type:        "password",
//	    Permissions: []string{"read", "list"},
//	    Tags: map[string]string{
//	        "environment": "production",
//	        "team":        "platform",
//	        "criticality": "high",
//	    },
//	}
type SecretMetadata struct {
	// Exists indicates whether the secret exists in the store.
	// If false, other fields may be empty or meaningless.
	Exists bool

	// Version identifies the current version of the secret.
	// Empty if versioning not supported or secret doesn't exist.
	Version string

	// UpdatedAt indicates when the secret was last modified.
	// Zero time if not supported or secret doesn't exist.
	UpdatedAt time.Time

	// Size is the approximate size of the secret value in bytes.
	// May be 0 if not supported, not available, or secret doesn't exist.
	Size int

	// Type describes the kind of secret (password, certificate, api_key, etc.).
	// Store-specific classification. May be empty.
	Type string

	// Permissions lists the operations the current credentials can perform
	// on this secret. Common values: "read", "write", "delete", "list".
	// Empty slice if not supported.
	Permissions []string

	// Tags contains store-specific metadata and labels.
	// Common keys include environment, owner, purpose, etc.
	// Empty map if not supported.
	Tags map[string]string
}

// SecretStoreCapabilities describes what features and operations a secret store supports.
//
// This structure allows dsops to adapt its behavior based on store capabilities,
// enable/disable features appropriately, and provide accurate user feedback
// about what operations are possible.
//
// Example:
//
//	caps := SecretStoreCapabilities{
//	    SupportsVersioning: true,
//	    SupportsMetadata:   true,
//	    SupportsWatching:   false,
//	    SupportsBinary:     true,
//	    RequiresAuth:       true,
//	    AuthMethods:        []string{"iam", "api_key"},
//	    Rotation: &RotationCapabilities{
//	        SupportsRotation:   true,
//	        SupportsVersioning: true,
//	        MaxVersions:        10,
//	        MinRotationTime:    time.Hour,
//	    },
//	}
type SecretStoreCapabilities struct {
	// SupportsVersioning indicates if the store maintains multiple versions
	// of secrets and can retrieve specific versions.
	SupportsVersioning bool

	// SupportsMetadata indicates if the store supports additional metadata
	// like tags, descriptions, and custom attributes beyond the secret value.
	SupportsMetadata bool

	// SupportsWatching indicates if the store can notify about secret changes
	// in real-time or support long-polling for updates.
	SupportsWatching bool

	// SupportsBinary indicates if the store can store and retrieve binary data
	// (certificates, keys, images) or only text-based secrets.
	SupportsBinary bool

	// RequiresAuth indicates if the store requires authentication to access secrets.
	// If false, the store may work without credentials (e.g., literal store).
	RequiresAuth bool

	// AuthMethods lists the authentication methods supported by this store.
	// Common values include:
	//   - "api_key": API key/token authentication
	//   - "basic": Username/password authentication
	//   - "oauth2": OAuth2 flow
	//   - "iam": Cloud IAM roles
	//   - "certificate": Client certificate authentication
	//   - "cli": CLI-based authentication (like aws configure)
	AuthMethods []string

	// Rotation contains rotation-related capabilities for stores that
	// support creating and managing multiple versions for rotation.
	// Nil if the store doesn't support rotation operations.
	Rotation *RotationCapabilities
}

// RotationCapabilities describes rotation features supported by the store.
//
// This structure defines what rotation operations a secret store can perform,
// including version management, timing constraints, and format requirements.
// Used by the rotation engine to plan and validate rotation operations.
//
// Example:
//
//	rotationCaps := &RotationCapabilities{
//	    SupportsRotation:   true,
//	    SupportsVersioning: true,
//	    MaxVersions:        5,           // Keep 5 versions
//	    MinRotationTime:    time.Hour,   // Minimum 1 hour between rotations
//	    Constraints: map[string]string{
//	        "max_length":    "4096",
//	        "min_length":    "8",
//	        "charset":       "alphanumeric",
//	        "complexity":    "high",
//	    },
//	}
type RotationCapabilities struct {
	// SupportsRotation indicates if the store can create new versions
	// of existing secrets for rotation purposes.
	SupportsRotation bool

	// SupportsVersioning indicates if the store maintains multiple versions
	// simultaneously, allowing for zero-downtime rotation strategies.
	SupportsVersioning bool

	// MaxVersions specifies the maximum number of versions the store will keep.
	// 0 means unlimited versions are supported.
	MaxVersions int

	// MinRotationTime specifies the minimum time that must pass between
	// rotation attempts for the same secret.
	MinRotationTime time.Duration

	// Constraints contains store-specific rotation constraints and requirements.
	// Common keys include:
	//   - "max_length": Maximum secret value length
	//   - "min_length": Minimum secret value length
	//   - "charset": Allowed character set
	//   - "complexity": Password complexity requirements
	//   - "format": Required format (e.g., "json", "pem")
	Constraints map[string]string
}

// Error types for secret store operations

// NotFoundError indicates that a requested secret does not exist in the store.
//
// Secret stores should return this error when a SecretRef points to a
// non-existent secret. This is distinct from authentication failures or
// permission errors.
//
// Example:
//
//	if !secretExists(ref.Path) {
//	    return SecretValue{}, NotFoundError{
//	        Store: s.Name(),
//	        Path:  ref.Path,
//	    }
//	}
type NotFoundError struct {
	// Store is the name of the secret store where the secret was not found.
	Store string

	// Path is the secret path that could not be found.
	Path string
}

// Error implements the error interface.
func (e NotFoundError) Error() string {
	return "secret not found: " + e.Path + " in store " + e.Store
}

// AuthError indicates that authentication to the secret store failed.
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
//	        Store:   s.Name(),
//	        Message: "API key is invalid or expired",
//	    }
//	}
type AuthError struct {
	// Store is the name of the secret store that failed authentication.
	Store string

	// Message provides details about the authentication failure.
	Message string
}

// Error implements the error interface.
func (e AuthError) Error() string {
	return "authentication failed for store " + e.Store + ": " + e.Message
}

// ValidationError indicates that a request or configuration is invalid.
//
// This error should be returned when:
//   - SecretRef format is invalid
//   - Required parameters are missing
//   - Configuration values are out of range
//   - URI parsing fails
//
// Example:
//
//	if !ref.IsValid() {
//	    return SecretValue{}, ValidationError{
//	        Store:   s.Name(),
//	        Message: "invalid secret reference format",
//	    }
//	}
type ValidationError struct {
	// Store is the name of the secret store where validation failed.
	// May be empty for general validation errors.
	Store string

	// Message provides details about what validation failed.
	Message string
}

// Error implements the error interface.
func (e ValidationError) Error() string {
	if e.Store == "" {
		return "validation failed: " + e.Message
	}
	return "validation failed for store " + e.Store + ": " + e.Message
}

// Helper functions for working with SecretRef

// ParseSecretRef parses a store:// URI into a SecretRef.
//
// This function converts URI-based secret references into structured SecretRef
// objects. It supports the full dsops store:// URI format with optional field
// extraction, version selection, and custom options.
//
// URI Format:
//
//	store://store-name/path#field?version=v&option=value
//
// Components:
//   - scheme: Must be "store://"
//   - store-name: Name of the secret store (required)
//   - path: Path to the secret within the store (required)
//   - field: Optional field name for extraction (after #)
//   - version: Optional version specifier (in query params)
//   - options: Additional store-specific parameters (in query params)
//
// Examples:
//
//	// Basic secret reference
//	ref, err := ParseSecretRef("store://aws-prod/database/password")
//	
//	// With field extraction
//	ref, err := ParseSecretRef("store://vault/secret/app#api_key")
//	
//	// With version and options
//	ref, err := ParseSecretRef("store://aws-prod/cert?version=AWSCURRENT&region=us-west-2")
//	
//	// Complex example
//	ref, err := ParseSecretRef("store://onepassword/Production/Database#password?vault=Private")
//
// Returns ValidationError for malformed URIs or missing required components.
func ParseSecretRef(uri string) (SecretRef, error) {
	if uri == "" {
		return SecretRef{}, ValidationError{Message: "empty URI"}
	}

	// Check for store:// scheme
	if !strings.HasPrefix(uri, "store://") {
		return SecretRef{}, ValidationError{Message: "URI must start with store://"}
	}

	// Remove scheme
	uri = strings.TrimPrefix(uri, "store://")

	// Split on ? to separate path from query parameters first
	var queryParams string
	if idx := strings.Index(uri, "?"); idx != -1 {
		queryParams = uri[idx+1:]
		uri = uri[:idx]
	}

	// Then split on # to separate path from field
	var field string
	if idx := strings.Index(uri, "#"); idx != -1 {
		field = uri[idx+1:]
		uri = uri[:idx]
	}

	// Split remaining URI to get store and path
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		return SecretRef{}, ValidationError{Message: "store name is required"}
	}

	store := parts[0]
	path := ""
	if len(parts) > 1 {
		path = parts[1]
	}

	if path == "" {
		return SecretRef{}, ValidationError{Message: "path is required"}
	}

	// Parse query parameters
	options := make(map[string]string)
	var version string

	if queryParams != "" {
		params, err := url.ParseQuery(queryParams)
		if err != nil {
			return SecretRef{}, ValidationError{Message: "invalid query parameters: " + err.Error()}
		}

		for key, values := range params {
			if len(values) > 0 {
				if key == "version" {
					version = values[0]
				} else {
					options[key] = values[0]
				}
			}
		}
	}

	return SecretRef{
		Store:   store,
		Path:    path,
		Field:   field,
		Version: version,
		Options: options,
	}, nil
}

// String converts a SecretRef back to URI format.
//
// This method reconstructs a store:// URI from a SecretRef structure,
// including all optional components like field extraction, version
// selection, and custom options.
//
// The output format matches what ParseSecretRef expects:
//
//	store://store-name/path#field?version=v&option=value
//
// Examples:
//
//	ref := SecretRef{
//	    Store:   "aws-prod",
//	    Path:    "database/password",
//	    Field:   "password",
//	    Version: "AWSCURRENT",
//	    Options: map[string]string{"region": "us-west-2"},
//	}
//	
//	uri := ref.String()
//	// Result: "store://aws-prod/database/password#password?version=AWSCURRENT&region=us-west-2"
//
// Returns empty string if the SecretRef is invalid (missing Store or Path).
func (ref SecretRef) String() string {
	if ref.Store == "" || ref.Path == "" {
		return ""
	}

	uri := "store://" + ref.Store + "/" + ref.Path

	// Add field if present
	if ref.Field != "" {
		uri += "#" + ref.Field
	}

	// Add query parameters
	params := url.Values{}
	if ref.Version != "" {
		params.Set("version", ref.Version)
	}
	for key, value := range ref.Options {
		params.Set(key, value)
	}

	if len(params) > 0 {
		uri += "?" + params.Encode()
	}

	return uri
}

// IsValid checks if a SecretRef has all required fields.
//
// A valid SecretRef must have both Store and Path fields populated.
// Field, Version, and Options are optional and don't affect validity.
//
// This method is used for validation before performing secret operations
// to ensure the reference can be processed correctly.
//
// Example:
//
//	ref := SecretRef{
//	    Store: "aws-prod",
//	    Path:  "database/password",
//	    // Field, Version, Options are optional
//	}
//	
//	if !ref.IsValid() {
//	    return ValidationError{
//	        Message: "invalid secret reference",
//	    }
//	}
//
// Returns true if both Store and Path are non-empty, false otherwise.
func (ref SecretRef) IsValid() bool {
	return ref.Store != "" && ref.Path != ""
}