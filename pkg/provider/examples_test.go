package provider_test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/systmms/dsops/pkg/provider"
)

// Example demonstrates basic usage of a provider
func ExampleProvider_basic() {
	// Create a mock provider for demonstration
	mockProvider := &MockProvider{
		name: "example-provider",
		secrets: map[string]provider.SecretValue{
			"database/password": {
				Value:     "secret-password-123",
				Version:   "v1",
				UpdatedAt: time.Now(),
				Metadata: map[string]string{
					"environment": "production",
					"owner":       "platform-team",
				},
			},
		},
	}

	// Validate the provider is properly configured
	ctx := context.Background()
	if err := mockProvider.Validate(ctx); err != nil {
		log.Fatalf("Provider validation failed: %v", err)
	}

	// Create a reference to a secret
	ref := provider.Reference{
		Provider: "example-provider",
		Key:      "database/password",
		Version:  "v1",
	}

	// Resolve the secret value
	secret, err := mockProvider.Resolve(ctx, ref)
	if err != nil {
		log.Fatalf("Failed to resolve secret: %v", err)
	}

	fmt.Printf("Secret version: %s\n", secret.Version)
	fmt.Printf("Updated at: %s\n", secret.UpdatedAt.Format("2006-01-02"))
	fmt.Printf("Environment: %s\n", secret.Metadata["environment"])

	// Output:
	// Secret version: v1
	// Updated at: 2025-08-26
	// Environment: production
}

// Example demonstrates error handling with providers
func ExampleProvider_errorHandling() {
	mockProvider := &MockProvider{
		name:    "example-provider",
		secrets: make(map[string]provider.SecretValue), // Empty for demonstration
	}

	ctx := context.Background()
	ref := provider.Reference{
		Provider: "example-provider",
		Key:      "nonexistent/secret",
	}

	// Attempt to resolve a non-existent secret
	_, err := mockProvider.Resolve(ctx, ref)
	if err != nil {
		// Check for specific error types
		var notFoundErr provider.NotFoundError
		if errors.As(err, &notFoundErr) {
			fmt.Printf("Secret not found: %s in provider %s\n", 
				notFoundErr.Key, notFoundErr.Provider)
		} else {
			fmt.Printf("Other error: %v\n", err)
		}
	}

	// Output:
	// Secret not found: nonexistent/secret in provider example-provider
}

// Example demonstrates using provider capabilities
func ExampleProvider_capabilities() {
	mockProvider := &MockProvider{
		name: "example-provider",
		capabilities: provider.Capabilities{
			SupportsVersioning: true,
			SupportsMetadata:   true,
			SupportsWatching:   false,
			SupportsBinary:     true,
			RequiresAuth:       true,
			AuthMethods:        []string{"api_key", "oauth2"},
		},
	}

	caps := mockProvider.Capabilities()

	fmt.Printf("Provider name: %s\n", mockProvider.Name())
	fmt.Printf("Supports versioning: %t\n", caps.SupportsVersioning)
	fmt.Printf("Supports metadata: %t\n", caps.SupportsMetadata)
	fmt.Printf("Requires auth: %t\n", caps.RequiresAuth)
	fmt.Printf("Auth methods: %v\n", caps.AuthMethods)

	// Use capabilities to adapt behavior
	if caps.SupportsVersioning {
		fmt.Println("Version-specific operations available")
	}

	if !caps.SupportsWatching {
		fmt.Println("Real-time updates not supported")
	}

	// Output:
	// Provider name: example-provider
	// Supports versioning: true
	// Supports metadata: true
	// Requires auth: true
	// Auth methods: [api_key oauth2]
	// Version-specific operations available
	// Real-time updates not supported
}

// Example demonstrates the Describe method for metadata-only operations
func ExampleProvider_describe() {
	mockProvider := &MockProvider{
		name: "example-provider",
		metadata: map[string]provider.Metadata{
			"database/config": {
				Exists:    true,
				Version:   "v2.1",
				UpdatedAt: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
				Size:      1024,
				Type:      "json",
				Permissions: []string{"read", "list"},
				Tags: map[string]string{
					"environment": "production",
					"team":        "platform",
					"criticality": "high",
				},
			},
		},
	}

	ctx := context.Background()
	ref := provider.Reference{
		Provider: "example-provider",
		Key:      "database/config",
	}

	// Get metadata without retrieving the secret value
	meta, err := mockProvider.Describe(ctx, ref)
	if err != nil {
		log.Fatalf("Failed to describe secret: %v", err)
	}

	if meta.Exists {
		fmt.Printf("Secret exists: %s\n", ref.Key)
		fmt.Printf("Version: %s\n", meta.Version)
		fmt.Printf("Size: %d bytes\n", meta.Size)
		fmt.Printf("Type: %s\n", meta.Type)
		fmt.Printf("Last updated: %s\n", meta.UpdatedAt.Format("2006-01-02 15:04"))
		fmt.Printf("Permissions: %v\n", meta.Permissions)
		fmt.Printf("Environment: %s\n", meta.Tags["environment"])
	} else {
		fmt.Println("Secret does not exist")
	}

	// Output:
	// Secret exists: database/config
	// Version: v2.1
	// Size: 1024 bytes
	// Type: json
	// Last updated: 2024-01-15 10:30
	// Permissions: [read list]
	// Environment: production
}

// Example demonstrates using a provider with the Rotator interface
func ExampleRotator() {
	// Create a provider that also implements Rotator
	rotatingProvider := &MockRotatingProvider{
		MockProvider: MockProvider{
			name: "rotating-provider",
			secrets: map[string]provider.SecretValue{
				"api/key": {
					Value:     "current-api-key-123",
					Version:   "v1",
					UpdatedAt: time.Now(),
				},
			},
		},
	}

	ctx := context.Background()
	ref := provider.Reference{
		Provider: "rotating-provider",
		Key:      "api/key",
	}

	// Check if provider supports rotation
	if rotator, ok := interface{}(rotatingProvider).(provider.Rotator); ok {
		fmt.Println("Provider supports rotation")

		// Get rotation metadata
		rotationMeta, err := rotator.GetRotationMetadata(ctx, ref)
		if err != nil {
			log.Fatalf("Failed to get rotation metadata: %v", err)
		}

		fmt.Printf("Supports rotation: %t\n", rotationMeta.SupportsRotation)
		fmt.Printf("Max value length: %d\n", rotationMeta.MaxValueLength)

		if rotationMeta.SupportsRotation {
			// Create a new version
			newValue := []byte("new-api-key-456")
			metadata := map[string]string{
				"rotated_by": "example-rotation",
				"reason":     "scheduled-rotation",
			}

			newVersion, err := rotator.CreateNewVersion(ctx, ref, newValue, metadata)
			if err != nil {
				log.Fatalf("Failed to create new version: %v", err)
			}

			fmt.Printf("Created new version: %s\n", newVersion)

			// In a real scenario, you would verify the new version works
			// before deprecating the old one
		}
	} else {
		fmt.Println("Provider does not support rotation")
	}

	// Output:
	// Provider supports rotation
	// Supports rotation: true
	// Max value length: 4096
	// Created new version: v2
}

// MockProvider implements the Provider interface for examples and testing
type MockProvider struct {
	name         string
	secrets      map[string]provider.SecretValue
	metadata     map[string]provider.Metadata
	capabilities provider.Capabilities
	validateErr  error
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	secret, exists := m.secrets[ref.Key]
	if !exists {
		return provider.SecretValue{}, provider.NotFoundError{
			Provider: m.name,
			Key:      ref.Key,
		}
	}

	// Handle version filtering if specified
	if ref.Version != "" && ref.Version != secret.Version {
		return provider.SecretValue{}, provider.NotFoundError{
			Provider: m.name,
			Key:      ref.Key + " (version " + ref.Version + ")",
		}
	}

	return secret, nil
}

func (m *MockProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	if m.metadata == nil {
		// Default behavior - check if secret exists
		_, exists := m.secrets[ref.Key]
		return provider.Metadata{Exists: exists}, nil
	}

	meta, exists := m.metadata[ref.Key]
	if !exists {
		return provider.Metadata{Exists: false}, nil
	}

	return meta, nil
}

func (m *MockProvider) Capabilities() provider.Capabilities {
	return m.capabilities
}

func (m *MockProvider) Validate(ctx context.Context) error {
	return m.validateErr
}

// MockRotatingProvider implements both Provider and Rotator interfaces
type MockRotatingProvider struct {
	MockProvider
	versionCounter int
}

func (m *MockRotatingProvider) CreateNewVersion(ctx context.Context, ref provider.Reference, newValue []byte, meta map[string]string) (string, error) {
	m.versionCounter++
	newVersion := fmt.Sprintf("v%d", m.versionCounter+1)

	// Store the new version
	secret := provider.SecretValue{
		Value:     string(newValue),
		Version:   newVersion,
		UpdatedAt: time.Now(),
		Metadata:  meta,
	}

	m.secrets[ref.Key] = secret
	return newVersion, nil
}

func (m *MockRotatingProvider) DeprecateVersion(ctx context.Context, ref provider.Reference, version string) error {
	// In a real implementation, this would mark the version as deprecated
	// For the mock, we'll just remove it
	if secret, exists := m.secrets[ref.Key]; exists && secret.Version == version {
		delete(m.secrets, ref.Key)
	}
	return nil
}

func (m *MockRotatingProvider) GetRotationMetadata(ctx context.Context, ref provider.Reference) (provider.RotationMetadata, error) {
	return provider.RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: true,
		MaxValueLength:     4096,
		MinValueLength:     8,
		AllowedCharacters:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
		RotationInterval:   "30d",
		LastRotated:        nil,
		NextRotation:       nil,
		Constraints: map[string]string{
			"complexity": "medium",
			"format":     "alphanumeric",
		},
	}, nil
}