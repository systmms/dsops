package service

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// Service defines the interface for external systems that have credentials to be rotated
// This represents the rotation target functionality split from the original Provider interface
type Service interface {
	// Name returns the service's name
	Name() string

	// Plan creates a rotation plan for the specified credential
	Plan(ctx context.Context, req RotationRequest) (RotationPlan, error)

	// Execute performs the rotation according to the plan (idempotent by fingerprint)
	Execute(ctx context.Context, plan RotationPlan) (RotationResult, error)

	// Verify checks that the rotation was successful and the new credential works
	Verify(ctx context.Context, result RotationResult) error

	// Rollback attempts to undo a rotation if something went wrong
	Rollback(ctx context.Context, result RotationResult) error

	// GetStatus returns the current rotation status for a credential
	GetStatus(ctx context.Context, ref ServiceRef) (RotationStatus, error)

	// Capabilities returns the service's rotation capabilities
	Capabilities() ServiceCapabilities

	// Validate checks if the service is properly configured and reachable
	Validate(ctx context.Context) error
}

// ServiceRef identifies a credential within a service using the new reference format
type ServiceRef struct {
	Type       string            // Service type (e.g., "github", "postgres", "stripe")
	Instance   string            // Service instance ID (e.g., "acme-org", "prod-db")
	Kind       string            // Credential kind (e.g., "pat", "password", "api-key")
	Principal  string            // Identity the credential belongs to (e.g., "ci-bot")
	Options    map[string]string // Additional options for the service
}

// RotationRequest contains all information needed to plan a rotation
type RotationRequest struct {
	ServiceRef    ServiceRef
	Strategy      string            // Rotation strategy (e.g., "two-key", "immediate")
	Policy        string            // Rotation policy name
	NewValue      []byte            // New credential value (if provided)
	Metadata      map[string]string // Additional metadata
	DryRun        bool              // Plan only, don't execute
}

// RotationPlan describes what will happen during rotation
type RotationPlan struct {
	ServiceRef    ServiceRef
	Strategy      string
	Steps         []RotationStep
	EstimatedTime time.Duration
	Fingerprint   string            // Unique identifier for this plan
	CreatedAt     time.Time
	Metadata      map[string]string
}

// RotationStep represents a single action in the rotation process
type RotationStep struct {
	Name        string
	Description string
	Action      string // "create", "verify", "promote", "deprecate", "delete"
	Target      string // What is being acted upon
	Options     map[string]string
}

// RotationResult contains the outcome of a rotation execution
type RotationResult struct {
	ServiceRef      ServiceRef
	Plan            RotationPlan
	Status          string // "success", "failed", "partial"
	OldCredential   CredentialInfo
	NewCredential   CredentialInfo
	ExecutedSteps   []ExecutedStep
	StartedAt       time.Time
	CompletedAt     time.Time
	Error           string
	Metadata        map[string]string
}

// ExecutedStep tracks the execution of a single rotation step
type ExecutedStep struct {
	Step        RotationStep
	Status      string // "success", "failed", "skipped"
	StartedAt   time.Time
	CompletedAt time.Time
	Output      string
	Error       string
}

// CredentialInfo describes a credential without exposing its value
type CredentialInfo struct {
	ID          string
	Version     string
	Status      string // "active", "deprecated", "revoked"
	CreatedAt   time.Time
	ExpiresAt   *time.Time
	LastUsed    *time.Time
	Metadata    map[string]string
}

// RotationStatus provides information about current rotation state
type RotationStatus struct {
	ServiceRef        ServiceRef
	CurrentCredential CredentialInfo
	LastRotation      *RotationResult
	NextRotation      *time.Time
	Status            string // "current", "needs_rotation", "rotation_in_progress"
	Warnings          []string
}

// ServiceCapabilities describes what rotation operations a service supports
type ServiceCapabilities struct {
	SupportedStrategies []string          // Strategies this service can use
	MaxActiveKeys       int               // Maximum concurrent credentials (0 = unlimited)
	SupportsExpiration  bool              // Can set expiration dates
	SupportsVersioning  bool              // Maintains credential versions
	SupportsRevocation  bool              // Can revoke old credentials
	SupportsVerification bool             // Can verify credential functionality
	MinRotationInterval time.Duration     // Minimum time between rotations
	Constraints         map[string]string // Format, length, character constraints
}

// Error types for service operations
type ServiceNotFoundError struct {
	ServiceRef ServiceRef
}

func (e ServiceNotFoundError) Error() string {
	return "service not found: " + e.ServiceRef.String()
}

type CredentialNotFoundError struct {
	ServiceRef ServiceRef
}

func (e CredentialNotFoundError) Error() string {
	return "credential not found: " + e.ServiceRef.String()
}

type RotationNotSupportedError struct {
	ServiceRef ServiceRef
	Strategy   string
	Reason     string
}

func (e RotationNotSupportedError) Error() string {
	return "rotation not supported for " + e.ServiceRef.String() + " with strategy " + e.Strategy + ": " + e.Reason
}

type VerificationError struct {
	ServiceRef ServiceRef
	Message    string
}

func (e VerificationError) Error() string {
	return "verification failed for " + e.ServiceRef.String() + ": " + e.Message
}

// Helper functions for working with ServiceRef

// ParseServiceRef parses a svc:// URI into a ServiceRef
// Format: svc://type/instance?kind=credential&principal=identity&option=value
func ParseServiceRef(uri string) (ServiceRef, error) {
	if uri == "" {
		return ServiceRef{}, ServiceNotFoundError{ServiceRef: ServiceRef{}}
	}

	// Check for svc:// scheme
	if !strings.HasPrefix(uri, "svc://") {
		return ServiceRef{}, ServiceNotFoundError{ServiceRef: ServiceRef{}}
	}

	// Remove scheme
	uri = strings.TrimPrefix(uri, "svc://")

	// Split on ? to separate path from query parameters
	var queryParams string
	if idx := strings.Index(uri, "?"); idx != -1 {
		queryParams = uri[idx+1:]
		uri = uri[:idx]
	}

	// Split remaining URI to get type and instance
	parts := strings.SplitN(uri, "/", 2)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return ServiceRef{}, ServiceNotFoundError{ServiceRef: ServiceRef{}}
	}

	serviceType := parts[0]
	instance := parts[1]

	// Parse query parameters
	options := make(map[string]string)
	var kind, principal string

	if queryParams != "" {
		params, err := url.ParseQuery(queryParams)
		if err != nil {
			return ServiceRef{}, ServiceNotFoundError{ServiceRef: ServiceRef{}}
		}

		for key, values := range params {
			if len(values) > 0 {
				switch key {
				case "kind":
					kind = values[0]
				case "principal":
					principal = values[0]
				default:
					options[key] = values[0]
				}
			}
		}
	}

	if kind == "" {
		return ServiceRef{}, ServiceNotFoundError{ServiceRef: ServiceRef{}}
	}

	return ServiceRef{
		Type:      serviceType,
		Instance:  instance,
		Kind:      kind,
		Principal: principal,
		Options:   options,
	}, nil
}

// String converts a ServiceRef to URI format
func (ref ServiceRef) String() string {
	if ref.Type == "" || ref.Instance == "" || ref.Kind == "" {
		return ""
	}

	uri := "svc://" + ref.Type + "/" + ref.Instance

	// Build query parameters
	params := url.Values{}
	params.Set("kind", ref.Kind)
	
	if ref.Principal != "" {
		params.Set("principal", ref.Principal)
	}
	
	for key, value := range ref.Options {
		params.Set(key, value)
	}

	uri += "?" + params.Encode()
	return uri
}

// IsValid checks if a ServiceRef has required fields
func (ref ServiceRef) IsValid() bool {
	return ref.Type != "" && ref.Instance != "" && ref.Kind != ""
}

// GenerateFingerprint creates a unique identifier for a rotation request
func GenerateFingerprint(req RotationRequest) string {
	// TODO: Implement fingerprint generation based on service, principal, policy
	// This should be deterministic for the same rotation parameters
	return "fp_" + req.ServiceRef.String() + "_" + req.Strategy
}