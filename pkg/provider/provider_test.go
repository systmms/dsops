package provider

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// TestNotFoundError tests the NotFoundError error type
func TestNotFoundError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      NotFoundError
		expected string
	}{
		{
			name: "basic error message",
			err: NotFoundError{
				Provider: "aws.secretsmanager",
				Key:      "my-secret",
			},
			expected: "secret not found: my-secret in aws.secretsmanager",
		},
		{
			name: "vault provider",
			err: NotFoundError{
				Provider: "hashicorp.vault",
				Key:      "secret/data/app",
			},
			expected: "secret not found: secret/data/app in hashicorp.vault",
		},
		{
			name: "empty provider name",
			err: NotFoundError{
				Provider: "",
				Key:      "key",
			},
			expected: "secret not found: key in ",
		},
		{
			name: "empty key",
			err: NotFoundError{
				Provider: "provider",
				Key:      "",
			},
			expected: "secret not found:  in provider",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("NotFoundError.Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestNotFoundErrorTypeChecking tests that NotFoundError can be identified with errors.As
func TestNotFoundErrorTypeChecking(t *testing.T) {
	t.Parallel()

	baseErr := NotFoundError{
		Provider: "test-provider",
		Key:      "missing-secret",
	}

	// Wrap the error
	wrappedErr := errors.New("failed to resolve: " + baseErr.Error())

	// Type checking should work
	var notFoundErr NotFoundError
	if !errors.As(baseErr, &notFoundErr) {
		t.Error("errors.As should identify NotFoundError")
	}
	if notFoundErr.Provider != "test-provider" {
		t.Errorf("Provider = %q, want %q", notFoundErr.Provider, "test-provider")
	}
	if notFoundErr.Key != "missing-secret" {
		t.Errorf("Key = %q, want %q", notFoundErr.Key, "missing-secret")
	}

	// Wrapped error should not be a NotFoundError (unless we implement Unwrap)
	var notFoundErr2 NotFoundError
	if errors.As(wrappedErr, &notFoundErr2) {
		// This is actually expected to fail because wrappedErr is a plain error
		t.Log("Wrapped error was identified as NotFoundError - unexpected but not wrong")
	}
}

// TestAuthError tests the AuthError error type
func TestAuthError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      AuthError
		expected string
	}{
		{
			name: "basic auth error",
			err: AuthError{
				Provider: "aws.secretsmanager",
				Message:  "invalid credentials",
			},
			expected: "authentication failed for aws.secretsmanager: invalid credentials",
		},
		{
			name: "expired token",
			err: AuthError{
				Provider: "vault",
				Message:  "token has expired",
			},
			expected: "authentication failed for vault: token has expired",
		},
		{
			name: "permission denied",
			err: AuthError{
				Provider: "onepassword",
				Message:  "insufficient permissions to access vault",
			},
			expected: "authentication failed for onepassword: insufficient permissions to access vault",
		},
		{
			name: "empty message",
			err: AuthError{
				Provider: "provider",
				Message:  "",
			},
			expected: "authentication failed for provider: ",
		},
		{
			name: "empty provider",
			err: AuthError{
				Provider: "",
				Message:  "auth failed",
			},
			expected: "authentication failed for : auth failed",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := tt.err.Error()
			if result != tt.expected {
				t.Errorf("AuthError.Error() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestAuthErrorTypeChecking tests that AuthError can be identified with errors.As
func TestAuthErrorTypeChecking(t *testing.T) {
	t.Parallel()

	baseErr := AuthError{
		Provider: "azure.keyvault",
		Message:  "client certificate expired",
	}

	var authErr AuthError
	if !errors.As(baseErr, &authErr) {
		t.Error("errors.As should identify AuthError")
	}
	if authErr.Provider != "azure.keyvault" {
		t.Errorf("Provider = %q, want %q", authErr.Provider, "azure.keyvault")
	}
	if authErr.Message != "client certificate expired" {
		t.Errorf("Message = %q, want %q", authErr.Message, "client certificate expired")
	}
}

// TestReferenceInitialization tests Reference struct creation and fields
func TestReferenceInitialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ref  Reference
	}{
		{
			name: "aws secrets manager reference",
			ref: Reference{
				Provider: "aws.secretsmanager",
				Key:      "prod/database/password",
				Version:  "AWSCURRENT",
				Path:     "",
				Field:    "",
			},
		},
		{
			name: "vault kv reference",
			ref: Reference{
				Provider: "hashicorp.vault",
				Key:      "api_key",
				Version:  "2",
				Path:     "secret/data/app",
				Field:    "",
			},
		},
		{
			name: "1password reference",
			ref: Reference{
				Provider: "onepassword",
				Key:      "database-item-id",
				Version:  "",
				Path:     "",
				Field:    "password",
			},
		},
		{
			name: "zero value reference",
			ref:  Reference{},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Verify fields are accessible
			_ = tt.ref.Provider
			_ = tt.ref.Key
			_ = tt.ref.Version
			_ = tt.ref.Path
			_ = tt.ref.Field
		})
	}
}

// TestSecretValueInitialization tests SecretValue struct creation
func TestSecretValueInitialization(t *testing.T) {
	t.Parallel()

	now := time.Now()
	sv := SecretValue{
		Value:     "super-secret-password",
		Version:   "v1.2.3",
		UpdatedAt: now,
		Metadata: map[string]string{
			"environment": "production",
			"owner":       "platform-team",
		},
	}

	if sv.Value != "super-secret-password" {
		t.Errorf("Value = %q, want %q", sv.Value, "super-secret-password")
	}
	if sv.Version != "v1.2.3" {
		t.Errorf("Version = %q, want %q", sv.Version, "v1.2.3")
	}
	if !sv.UpdatedAt.Equal(now) {
		t.Errorf("UpdatedAt = %v, want %v", sv.UpdatedAt, now)
	}
	if sv.Metadata["environment"] != "production" {
		t.Errorf("Metadata[environment] = %q, want %q", sv.Metadata["environment"], "production")
	}
}

// TestSecretValueZeroValue tests SecretValue with zero values
func TestSecretValueZeroValue(t *testing.T) {
	t.Parallel()

	var sv SecretValue
	if sv.Value != "" {
		t.Errorf("Zero value Value = %q, want empty string", sv.Value)
	}
	if sv.Version != "" {
		t.Errorf("Zero value Version = %q, want empty string", sv.Version)
	}
	if !sv.UpdatedAt.IsZero() {
		t.Errorf("Zero value UpdatedAt = %v, want zero time", sv.UpdatedAt)
	}
	if sv.Metadata != nil {
		t.Errorf("Zero value Metadata = %v, want nil", sv.Metadata)
	}
}

// TestMetadataInitialization tests Metadata struct creation
func TestMetadataInitialization(t *testing.T) {
	t.Parallel()

	now := time.Now()
	meta := Metadata{
		Exists:      true,
		Version:     "AWSCURRENT",
		UpdatedAt:   now,
		Size:        256,
		Type:        "password",
		Permissions: []string{"read", "list"},
		Tags: map[string]string{
			"environment": "production",
			"team":        "platform",
		},
	}

	if !meta.Exists {
		t.Error("Exists should be true")
	}
	if meta.Version != "AWSCURRENT" {
		t.Errorf("Version = %q, want %q", meta.Version, "AWSCURRENT")
	}
	if meta.Size != 256 {
		t.Errorf("Size = %d, want %d", meta.Size, 256)
	}
	if meta.Type != "password" {
		t.Errorf("Type = %q, want %q", meta.Type, "password")
	}
	if len(meta.Permissions) != 2 {
		t.Errorf("Permissions length = %d, want %d", len(meta.Permissions), 2)
	}
	if meta.Tags["team"] != "platform" {
		t.Errorf("Tags[team] = %q, want %q", meta.Tags["team"], "platform")
	}
}

// TestCapabilitiesInitialization tests Capabilities struct creation
func TestCapabilitiesInitialization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		caps Capabilities
	}{
		{
			name: "full capabilities",
			caps: Capabilities{
				SupportsVersioning: true,
				SupportsMetadata:   true,
				SupportsWatching:   false,
				SupportsBinary:     true,
				RequiresAuth:       true,
				AuthMethods:        []string{"api_key", "oauth2", "iam"},
			},
		},
		{
			name: "no auth required",
			caps: Capabilities{
				SupportsVersioning: false,
				SupportsMetadata:   false,
				SupportsWatching:   false,
				SupportsBinary:     false,
				RequiresAuth:       false,
				AuthMethods:        nil,
			},
		},
		{
			name: "vault capabilities",
			caps: Capabilities{
				SupportsVersioning: true,
				SupportsMetadata:   true,
				SupportsWatching:   false,
				SupportsBinary:     true,
				RequiresAuth:       true,
				AuthMethods:        []string{"token", "approle", "ldap", "kubernetes"},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// Verify all fields are accessible
			_ = tt.caps.SupportsVersioning
			_ = tt.caps.SupportsMetadata
			_ = tt.caps.SupportsWatching
			_ = tt.caps.SupportsBinary
			_ = tt.caps.RequiresAuth
			_ = tt.caps.AuthMethods

			// If auth is required, should have methods
			if tt.caps.RequiresAuth && len(tt.caps.AuthMethods) == 0 {
				t.Error("RequiresAuth is true but AuthMethods is empty")
			}
		})
	}
}

// TestRotationMetadataInitialization tests RotationMetadata struct creation
func TestRotationMetadataInitialization(t *testing.T) {
	t.Parallel()

	lastRotated := time.Now().Add(-24 * time.Hour)
	nextRotation := time.Now().Add(30 * 24 * time.Hour)

	meta := RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: true,
		MaxValueLength:     4096,
		MinValueLength:     12,
		AllowedCharacters:  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*",
		RotationInterval:   "30d",
		LastRotated:        &lastRotated,
		NextRotation:       &nextRotation,
		Constraints: map[string]string{
			"complexity": "high",
			"policy":     "corporate-password-policy",
		},
	}

	if !meta.SupportsRotation {
		t.Error("SupportsRotation should be true")
	}
	if !meta.SupportsVersioning {
		t.Error("SupportsVersioning should be true")
	}
	if meta.MaxValueLength != 4096 {
		t.Errorf("MaxValueLength = %d, want %d", meta.MaxValueLength, 4096)
	}
	if meta.MinValueLength != 12 {
		t.Errorf("MinValueLength = %d, want %d", meta.MinValueLength, 12)
	}
	if meta.RotationInterval != "30d" {
		t.Errorf("RotationInterval = %q, want %q", meta.RotationInterval, "30d")
	}
	if meta.LastRotated == nil {
		t.Error("LastRotated should not be nil")
	}
	if meta.NextRotation == nil {
		t.Error("NextRotation should not be nil")
	}
	if meta.Constraints["complexity"] != "high" {
		t.Errorf("Constraints[complexity] = %q, want %q", meta.Constraints["complexity"], "high")
	}
}

// TestRotationMetadataJSONMarshal tests JSON serialization of RotationMetadata
func TestRotationMetadataJSONMarshal(t *testing.T) {
	t.Parallel()

	meta := RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: false,
		MaxValueLength:     256,
		MinValueLength:     8,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	var decoded RotationMetadata
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() failed: %v", err)
	}

	if decoded.SupportsRotation != meta.SupportsRotation {
		t.Errorf("SupportsRotation = %v, want %v", decoded.SupportsRotation, meta.SupportsRotation)
	}
	if decoded.SupportsVersioning != meta.SupportsVersioning {
		t.Errorf("SupportsVersioning = %v, want %v", decoded.SupportsVersioning, meta.SupportsVersioning)
	}
	if decoded.MaxValueLength != meta.MaxValueLength {
		t.Errorf("MaxValueLength = %d, want %d", decoded.MaxValueLength, meta.MaxValueLength)
	}
	if decoded.MinValueLength != meta.MinValueLength {
		t.Errorf("MinValueLength = %d, want %d", decoded.MinValueLength, meta.MinValueLength)
	}
}

// MockRotator implements the Rotator interface for testing
type MockRotator struct {
	name         string
	values       map[string][]byte
	versions     map[string][]string // key -> list of version IDs
	metadata     map[string]RotationMetadata
	failOnCreate bool
	failOnDeprecate bool
}

// NewMockRotator creates a new mock rotator for testing
func NewMockRotator(name string) *MockRotator {
	return &MockRotator{
		name:     name,
		values:   make(map[string][]byte),
		versions: make(map[string][]string),
		metadata: make(map[string]RotationMetadata),
	}
}

// Name returns the rotator's name
func (m *MockRotator) Name() string {
	return m.name
}

// Resolve retrieves a secret value
func (m *MockRotator) Resolve(ctx context.Context, ref Reference) (SecretValue, error) {
	select {
	case <-ctx.Done():
		return SecretValue{}, ctx.Err()
	default:
	}

	value, exists := m.values[ref.Key]
	if !exists {
		return SecretValue{}, NotFoundError{
			Provider: m.name,
			Key:      ref.Key,
		}
	}

	version := ""
	if versions := m.versions[ref.Key]; len(versions) > 0 {
		version = versions[len(versions)-1]
	}

	return SecretValue{
		Value:   string(value),
		Version: version,
	}, nil
}

// Describe returns metadata about a secret
func (m *MockRotator) Describe(ctx context.Context, ref Reference) (Metadata, error) {
	select {
	case <-ctx.Done():
		return Metadata{}, ctx.Err()
	default:
	}

	_, exists := m.values[ref.Key]
	version := ""
	if versions := m.versions[ref.Key]; len(versions) > 0 {
		version = versions[len(versions)-1]
	}
	return Metadata{
		Exists:  exists,
		Version: version,
		Type:    "rotatable-secret",
	}, nil
}

// Capabilities returns the rotator's capabilities
func (m *MockRotator) Capabilities() Capabilities {
	return Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		RequiresAuth:       false,
	}
}

// Validate checks if the rotator is properly configured
func (m *MockRotator) Validate(ctx context.Context) error {
	return nil
}

// CreateNewVersion creates a new version of a secret
func (m *MockRotator) CreateNewVersion(ctx context.Context, ref Reference, newValue []byte, meta map[string]string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	if m.failOnCreate {
		return "", errors.New("mock create version failed")
	}

	m.values[ref.Key] = newValue
	// Use nanosecond precision to ensure unique versions
	newVersion := time.Now().Format("20060102150405.000000000")
	if m.versions[ref.Key] == nil {
		m.versions[ref.Key] = []string{}
	}
	m.versions[ref.Key] = append(m.versions[ref.Key], newVersion)

	return newVersion, nil
}

// DeprecateVersion marks a version as deprecated
func (m *MockRotator) DeprecateVersion(ctx context.Context, ref Reference, version string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if m.failOnDeprecate {
		return errors.New("mock deprecate version failed")
	}

	// Remove the version from the list
	versions := m.versions[ref.Key]
	for i, v := range versions {
		if v == version {
			m.versions[ref.Key] = append(versions[:i], versions[i+1:]...)
			break
		}
	}

	return nil
}

// GetRotationMetadata returns rotation metadata for a secret
func (m *MockRotator) GetRotationMetadata(ctx context.Context, ref Reference) (RotationMetadata, error) {
	select {
	case <-ctx.Done():
		return RotationMetadata{}, ctx.Err()
	default:
	}

	if meta, exists := m.metadata[ref.Key]; exists {
		return meta, nil
	}

	// Return default metadata
	return RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: true,
		MaxValueLength:     4096,
		MinValueLength:     1,
	}, nil
}

// TestMockRotatorCreateNewVersion tests creating new secret versions
func TestMockRotatorCreateNewVersion(t *testing.T) {
	t.Parallel()

	rotator := NewMockRotator("test-rotator")
	ctx := context.Background()
	ref := Reference{Key: "db-password"}

	// Create initial version
	version1, err := rotator.CreateNewVersion(ctx, ref, []byte("password1"), nil)
	if err != nil {
		t.Fatalf("CreateNewVersion() failed: %v", err)
	}
	if version1 == "" {
		t.Error("CreateNewVersion() returned empty version")
	}

	// Verify the value was stored
	secret, err := rotator.Resolve(ctx, ref)
	if err != nil {
		t.Fatalf("Resolve() failed: %v", err)
	}
	if secret.Value != "password1" {
		t.Errorf("Value = %q, want %q", secret.Value, "password1")
	}

	// Create second version
	time.Sleep(time.Millisecond) // Ensure different timestamp
	version2, err := rotator.CreateNewVersion(ctx, ref, []byte("password2"), map[string]string{"rotated_by": "test"})
	if err != nil {
		t.Fatalf("CreateNewVersion() failed: %v", err)
	}
	if version2 == "" {
		t.Error("CreateNewVersion() returned empty version")
	}
	if version2 == version1 {
		t.Error("Second version should be different from first")
	}

	// Verify new value
	secret, err = rotator.Resolve(ctx, ref)
	if err != nil {
		t.Fatalf("Resolve() failed: %v", err)
	}
	if secret.Value != "password2" {
		t.Errorf("Value = %q, want %q", secret.Value, "password2")
	}
}

// TestMockRotatorDeprecateVersion tests deprecating old versions
func TestMockRotatorDeprecateVersion(t *testing.T) {
	t.Parallel()

	rotator := NewMockRotator("test-rotator")
	ctx := context.Background()
	ref := Reference{Key: "api-key"}

	// Create two versions
	version1, _ := rotator.CreateNewVersion(ctx, ref, []byte("key1"), nil)
	time.Sleep(time.Millisecond)
	_, _ = rotator.CreateNewVersion(ctx, ref, []byte("key2"), nil)

	// Deprecate first version
	err := rotator.DeprecateVersion(ctx, ref, version1)
	if err != nil {
		t.Fatalf("DeprecateVersion() failed: %v", err)
	}

	// Verify version was removed from list
	if len(rotator.versions[ref.Key]) != 1 {
		t.Errorf("versions count = %d, want %d", len(rotator.versions[ref.Key]), 1)
	}
}

// TestMockRotatorGetRotationMetadata tests getting rotation metadata
func TestMockRotatorGetRotationMetadata(t *testing.T) {
	t.Parallel()

	rotator := NewMockRotator("test-rotator")
	ctx := context.Background()
	ref := Reference{Key: "test-secret"}

	// Set custom metadata
	rotator.metadata[ref.Key] = RotationMetadata{
		SupportsRotation:   true,
		SupportsVersioning: true,
		MaxValueLength:     1024,
		MinValueLength:     16,
		AllowedCharacters:  "abcdefghijklmnopqrstuvwxyz",
	}

	meta, err := rotator.GetRotationMetadata(ctx, ref)
	if err != nil {
		t.Fatalf("GetRotationMetadata() failed: %v", err)
	}

	if meta.MaxValueLength != 1024 {
		t.Errorf("MaxValueLength = %d, want %d", meta.MaxValueLength, 1024)
	}
	if meta.MinValueLength != 16 {
		t.Errorf("MinValueLength = %d, want %d", meta.MinValueLength, 16)
	}
	if meta.AllowedCharacters != "abcdefghijklmnopqrstuvwxyz" {
		t.Errorf("AllowedCharacters = %q, want %q", meta.AllowedCharacters, "abcdefghijklmnopqrstuvwxyz")
	}
}

// TestMockRotatorContextCancellation tests context cancellation
func TestMockRotatorContextCancellation(t *testing.T) {
	t.Parallel()

	rotator := NewMockRotator("test-rotator")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ref := Reference{Key: "test-secret"}

	// All methods should respect context cancellation
	_, err := rotator.CreateNewVersion(ctx, ref, []byte("value"), nil)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("CreateNewVersion() error = %v, want context.Canceled", err)
	}

	err = rotator.DeprecateVersion(ctx, ref, "v1")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("DeprecateVersion() error = %v, want context.Canceled", err)
	}

	_, err = rotator.GetRotationMetadata(ctx, ref)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("GetRotationMetadata() error = %v, want context.Canceled", err)
	}
}

// TestMockRotatorFailures tests error scenarios
func TestMockRotatorFailures(t *testing.T) {
	t.Parallel()

	rotator := NewMockRotator("test-rotator")
	rotator.failOnCreate = true
	ctx := context.Background()
	ref := Reference{Key: "test-secret"}

	// Test create failure
	_, err := rotator.CreateNewVersion(ctx, ref, []byte("value"), nil)
	if err == nil {
		t.Error("CreateNewVersion() should fail when failOnCreate is true")
	}

	// Test deprecate failure
	rotator.failOnCreate = false
	rotator.failOnDeprecate = true
	_, _ = rotator.CreateNewVersion(ctx, ref, []byte("value"), nil)

	err = rotator.DeprecateVersion(ctx, ref, "any-version")
	if err == nil {
		t.Error("DeprecateVersion() should fail when failOnDeprecate is true")
	}
}

// Ensure MockRotator implements Rotator interface
var _ Rotator = (*MockRotator)(nil)

// Ensure MockRotator also implements Provider interface
var _ Provider = (*MockRotator)(nil)

// TestRunContractTests tests the RunContractTests helper function from testing.go
func TestRunContractTests(t *testing.T) {
	t.Parallel()

	// Create a simple test provider
	testProvider := &SimpleTestProvider{
		name:   "test-provider",
		values: make(map[string]string),
	}

	contract := ContractTest{
		CreateProvider: func(t *testing.T) Provider {
			return testProvider
		},
		SetupTestSecret: func(t *testing.T, p Provider) (string, func()) {
			provider := p.(*SimpleTestProvider)
			key := "test-secret"
			provider.values[key] = "test-value"
			return key, func() {
				delete(provider.values, key)
			}
		},
	}

	// Run the contract tests
	RunContractTests(t, contract)
}

// TestRunContractTestsWithSkips tests the RunContractTests helper with skip options
func TestRunContractTestsWithSkips(t *testing.T) {
	t.Parallel()

	testProvider := &SimpleTestProvider{
		name:   "skip-test-provider",
		values: make(map[string]string),
	}

	contract := ContractTest{
		CreateProvider: func(t *testing.T) Provider {
			return testProvider
		},
		SetupTestSecret: func(t *testing.T, p Provider) (string, func()) {
			provider := p.(*SimpleTestProvider)
			key := "test-secret"
			provider.values[key] = "test-value"
			return key, func() {
				delete(provider.values, key)
			}
		},
		SkipValidation: true,
		SkipMetadata:   true,
	}

	// Run the contract tests with skips
	RunContractTests(t, contract)
}

// TestRunContractTestsWithoutSetupSecret tests behavior when SetupTestSecret is nil
func TestRunContractTestsWithoutSetupSecret(t *testing.T) {
	t.Parallel()

	testProvider := &SimpleTestProvider{
		name:   "no-setup-provider",
		values: make(map[string]string),
	}

	contract := ContractTest{
		CreateProvider: func(t *testing.T) Provider {
			return testProvider
		},
		SetupTestSecret: nil, // No setup function
	}

	// Run the contract tests - should skip resolve and describe tests
	RunContractTests(t, contract)
}

// SimpleTestProvider is a minimal provider implementation for testing
type SimpleTestProvider struct {
	name   string
	values map[string]string
}

func (p *SimpleTestProvider) Name() string {
	return p.name
}

func (p *SimpleTestProvider) Resolve(ctx context.Context, ref Reference) (SecretValue, error) {
	select {
	case <-ctx.Done():
		return SecretValue{}, ctx.Err()
	default:
	}

	value, exists := p.values[ref.Key]
	if !exists {
		return SecretValue{}, NotFoundError{
			Provider: p.name,
			Key:      ref.Key,
		}
	}

	return SecretValue{
		Value:   value,
		Version: "1",
	}, nil
}

func (p *SimpleTestProvider) Describe(ctx context.Context, ref Reference) (Metadata, error) {
	select {
	case <-ctx.Done():
		return Metadata{}, ctx.Err()
	default:
	}

	_, exists := p.values[ref.Key]
	return Metadata{
		Exists:  exists,
		Version: "1",
		Type:    "test-secret",
	}, nil
}

func (p *SimpleTestProvider) Capabilities() Capabilities {
	return Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       false,
		AuthMethods:        nil,
	}
}

func (p *SimpleTestProvider) Validate(ctx context.Context) error {
	return nil
}

var _ Provider = (*SimpleTestProvider)(nil)
