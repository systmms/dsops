package secretstore_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/systmms/dsops/pkg/secretstore"
)

// Example demonstrates basic secret store usage with URI references
func ExampleSecretStore_basic() {
	// Create a mock secret store for demonstration
	store := &MockSecretStore{
		name: "example-store",
		secrets: map[string]secretstore.SecretValue{
			"database/credentials": {
				Value:     `{"username":"dbuser","password":"secret123"}`,
				Version:   "v1.0",
				UpdatedAt: time.Now(),
				Metadata: map[string]string{
					"environment": "production",
					"created_by":  "platform-team",
				},
			},
		},
	}

	ctx := context.Background()

	// Validate store is properly configured
	if err := store.Validate(ctx); err != nil {
		log.Fatalf("Store validation failed: %v", err)
	}

	// Create a secret reference using the new format
	ref := secretstore.SecretRef{
		Store: "example-store",
		Path:  "database/credentials",
		Field: "password", // Extract specific field from JSON
	}

	// Resolve the secret
	secret, err := store.Resolve(ctx, ref)
	if err != nil {
		log.Fatalf("Failed to resolve secret: %v", err)
	}

	fmt.Printf("Store name: %s\n", store.Name())
	fmt.Printf("Secret version: %s\n", secret.Version)
	fmt.Printf("Environment: %s\n", secret.Metadata["environment"])
	fmt.Printf("Field extracted: %s\n", secret.Value) // Would be "secret123" after JSON extraction

	// Output:
	// Store name: example-store
	// Secret version: v1.0
	// Environment: production
	// Field extracted: {"username":"dbuser","password":"secret123"}
}

// Example demonstrates URI parsing and reference creation
func ExampleParseSecretRef() {
	// Parse various URI formats
	examples := []string{
		"store://aws-prod/database/password",
		"store://vault/secret/app#api_key?version=2",
		"store://onepassword/Production/Database#password?vault=Private",
		"store://azure/app-secrets?version=latest&region=eastus",
	}

	for _, uri := range examples {
		ref, err := secretstore.ParseSecretRef(uri)
		if err != nil {
			fmt.Printf("Failed to parse %s: %v\n", uri, err)
			continue
		}

		fmt.Printf("URI: %s\n", uri)
		fmt.Printf("  Store: %s\n", ref.Store)
		fmt.Printf("  Path: %s\n", ref.Path)
		if ref.Field != "" {
			fmt.Printf("  Field: %s\n", ref.Field)
		}
		if ref.Version != "" {
			fmt.Printf("  Version: %s\n", ref.Version)
		}
		if len(ref.Options) > 0 {
			fmt.Printf("  Options: %v\n", ref.Options)
		}
		fmt.Printf("  Valid: %t\n", ref.IsValid())
		fmt.Println()
	}

	// Output:
	// URI: store://aws-prod/database/password
	//   Store: aws-prod
	//   Path: database/password
	//   Valid: true
	//
	// URI: store://vault/secret/app#api_key?version=2
	//   Store: vault
	//   Path: secret/app
	//   Field: api_key
	//   Version: 2
	//   Valid: true
	//
	// URI: store://onepassword/Production/Database#password?vault=Private
	//   Store: onepassword
	//   Path: Production/Database
	//   Field: password
	//   Options: map[vault:Private]
	//   Valid: true
	//
	// URI: store://azure/app-secrets?version=latest&region=eastus
	//   Store: azure
	//   Path: app-secrets
	//   Version: latest
	//   Options: map[region:eastus]
	//   Valid: true
}

// Example demonstrates error handling with structured error types
func ExampleSecretStore_errorHandling() {
	store := &MockSecretStore{
		name:    "example-store",
		secrets: make(map[string]secretstore.SecretValue),
	}

	ctx := context.Background()
	ref := secretstore.SecretRef{
		Store: "example-store",
		Path:  "nonexistent/secret",
	}

	_, err := store.Resolve(ctx, ref)
	if err != nil {
		// Handle different error types appropriately
		var notFoundErr secretstore.NotFoundError
		var authErr secretstore.AuthError
		var validationErr secretstore.ValidationError

		switch {
		case errors.As(err, &notFoundErr):
			fmt.Printf("Secret not found: %s in store %s\n", 
				notFoundErr.Path, notFoundErr.Store)
			// Could implement fallback logic here
			
		case errors.As(err, &authErr):
			fmt.Printf("Authentication failed for store %s: %s\n", 
				authErr.Store, authErr.Message)
			// Could trigger re-authentication here
			
		case errors.As(err, &validationErr):
			fmt.Printf("Validation error: %s\n", validationErr.Message)
			// Could provide user guidance here
			
		default:
			fmt.Printf("Unexpected error: %v\n", err)
		}
	}

	// Output:
	// Secret not found: nonexistent/secret in store example-store
}

// Example demonstrates capability-driven feature detection
func ExampleSecretStore_capabilities() {
	store := &MockSecretStore{
		name: "advanced-store",
		capabilities: secretstore.SecretStoreCapabilities{
			SupportsVersioning: true,
			SupportsMetadata:   true,
			SupportsWatching:   false,
			SupportsBinary:     true,
			RequiresAuth:       true,
			AuthMethods:        []string{"iam", "api_key"},
			Rotation: &secretstore.RotationCapabilities{
				SupportsRotation:   true,
				SupportsVersioning: true,
				MaxVersions:        10,
				MinRotationTime:    time.Hour,
				Constraints: map[string]string{
					"max_length":  "4096",
					"complexity":  "high",
				},
			},
		},
	}

	caps := store.Capabilities()

	fmt.Printf("Store: %s\n", store.Name())
	fmt.Printf("Supports versioning: %t\n", caps.SupportsVersioning)
	fmt.Printf("Supports metadata: %t\n", caps.SupportsMetadata)
	fmt.Printf("Supports binary: %t\n", caps.SupportsBinary)
	fmt.Printf("Requires auth: %t\n", caps.RequiresAuth)
	fmt.Printf("Auth methods: %v\n", caps.AuthMethods)

	// Check rotation capabilities
	if caps.Rotation != nil {
		fmt.Printf("Supports rotation: %t\n", caps.Rotation.SupportsRotation)
		fmt.Printf("Max versions: %d\n", caps.Rotation.MaxVersions)
		fmt.Printf("Min rotation time: %s\n", caps.Rotation.MinRotationTime)
		
		if constraint, ok := caps.Rotation.Constraints["max_length"]; ok {
			fmt.Printf("Max secret length: %s\n", constraint)
		}
	}

	// Use capabilities to adapt application behavior
	if caps.SupportsVersioning {
		fmt.Println("Version-specific operations available")
	}

	if !caps.SupportsWatching {
		fmt.Println("Real-time notifications not supported - using polling")
	}

	// Output:
	// Store: advanced-store
	// Supports versioning: true
	// Supports metadata: true
	// Supports binary: true
	// Requires auth: true
	// Auth methods: [iam api_key]
	// Supports rotation: true
	// Max versions: 10
	// Min rotation time: 1h0m0s
	// Max secret length: 4096
	// Version-specific operations available
	// Real-time notifications not supported - using polling
}

// Example demonstrates metadata-only operations for efficiency
func ExampleSecretStore_describe() {
	store := &MockSecretStore{
		name: "metadata-store",
		metadata: map[string]secretstore.SecretMetadata{
			"app/certificate": {
				Exists:    true,
				Version:   "v3.2",
				UpdatedAt: time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC),
				Size:      2048,
				Type:      "certificate",
				Permissions: []string{"read", "rotate"},
				Tags: map[string]string{
					"environment": "production",
					"team":        "security",
					"expires":     "2024-12-31",
				},
			},
		},
	}

	ctx := context.Background()
	ref := secretstore.SecretRef{
		Store: "metadata-store", 
		Path:  "app/certificate",
	}

	// Get metadata without retrieving the secret value
	meta, err := store.Describe(ctx, ref)
	if err != nil {
		log.Fatalf("Failed to describe secret: %v", err)
	}

	if meta.Exists {
		fmt.Printf("Secret: %s\n", ref.Path)
		fmt.Printf("Version: %s\n", meta.Version)
		fmt.Printf("Size: %d bytes\n", meta.Size)
		fmt.Printf("Type: %s\n", meta.Type)
		fmt.Printf("Updated: %s\n", meta.UpdatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("Permissions: %v\n", meta.Permissions)
		fmt.Printf("Team: %s\n", meta.Tags["team"])
		fmt.Printf("Expires: %s\n", meta.Tags["expires"])
		
		// Use metadata to make decisions without retrieving the secret
		if meta.Type == "certificate" {
			fmt.Println("Certificate detected - checking expiration...")
		}
		
		if contains(meta.Permissions, "rotate") {
			fmt.Println("Secret supports rotation")
		}
	} else {
		fmt.Printf("Secret %s does not exist\n", ref.Path)
	}

	// Output:
	// Secret: app/certificate
	// Version: v3.2
	// Size: 2048 bytes
	// Type: certificate
	// Updated: 2024-03-15 14:30
	// Permissions: [read rotate]
	// Team: security
	// Expires: 2024-12-31
	// Certificate detected - checking expiration...
	// Secret supports rotation
}

// Example demonstrates SecretRef round-trip conversion
func ExampleSecretRef_String() {
	// Create a complex SecretRef
	ref := secretstore.SecretRef{
		Store:   "production-vault",
		Path:    "services/api/credentials",
		Field:   "api_key",
		Version: "latest",
		Options: map[string]string{
			"namespace": "production",
			"region":    "us-east-1",
		},
	}

	// Convert to URI string
	uri := ref.String()
	fmt.Printf("Original ref: %+v\n", ref)
	fmt.Printf("URI: %s\n", uri)

	// Parse back from URI
	parsed, err := secretstore.ParseSecretRef(uri)
	if err != nil {
		log.Fatalf("Failed to parse URI: %v", err)
	}

	fmt.Printf("Parsed ref: %+v\n", parsed)
	fmt.Printf("Round-trip successful: %t\n", 
		ref.Store == parsed.Store &&
		ref.Path == parsed.Path &&
		ref.Field == parsed.Field &&
		ref.Version == parsed.Version)

	// Output:
	// Original ref: {Store:production-vault Path:services/api/credentials Field:api_key Version:latest Options:map[namespace:production region:us-east-1]}
	// URI: store://production-vault/services/api/credentials#api_key?version=latest&namespace=production&region=us-east-1
	// Parsed ref: {Store:production-vault Path:services/api/credentials Field:api_key Version:latest Options:map[namespace:production region:us-east-1]}
	// Round-trip successful: true
}

// MockSecretStore implements SecretStore interface for examples and testing
type MockSecretStore struct {
	name         string
	secrets      map[string]secretstore.SecretValue
	metadata     map[string]secretstore.SecretMetadata
	capabilities secretstore.SecretStoreCapabilities
	validateErr  error
}

func (m *MockSecretStore) Name() string {
	return m.name
}

func (m *MockSecretStore) Resolve(ctx context.Context, ref secretstore.SecretRef) (secretstore.SecretValue, error) {
	if !ref.IsValid() {
		return secretstore.SecretValue{}, secretstore.ValidationError{
			Store:   m.name,
			Message: "invalid secret reference",
		}
	}

	secret, exists := m.secrets[ref.Path]
	if !exists {
		return secretstore.SecretValue{}, secretstore.NotFoundError{
			Store: m.name,
			Path:  ref.Path,
		}
	}

	// Handle version filtering if specified
	if ref.Version != "" && ref.Version != secret.Version {
		return secretstore.SecretValue{}, secretstore.NotFoundError{
			Store: m.name,
			Path:  ref.Path + " (version " + ref.Version + ")",
		}
	}

	// Handle field extraction (simplified - real implementation would parse JSON)
	if ref.Field != "" {
		// In a real implementation, this would extract the field from JSON/YAML
		// For the mock, we'll just return the full value
		fmt.Printf("Field extraction requested: %s\n", ref.Field)
	}

	return secret, nil
}

func (m *MockSecretStore) Describe(ctx context.Context, ref secretstore.SecretRef) (secretstore.SecretMetadata, error) {
	if !ref.IsValid() {
		return secretstore.SecretMetadata{}, secretstore.ValidationError{
			Store:   m.name,
			Message: "invalid secret reference",
		}
	}

	if m.metadata != nil {
		if meta, exists := m.metadata[ref.Path]; exists {
			return meta, nil
		}
	}

	// Default behavior - check if secret exists
	_, exists := m.secrets[ref.Path]
	return secretstore.SecretMetadata{Exists: exists}, nil
}

func (m *MockSecretStore) Capabilities() secretstore.SecretStoreCapabilities {
	return m.capabilities
}

func (m *MockSecretStore) Validate(ctx context.Context) error {
	return m.validateErr
}

// Helper function used in examples
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}