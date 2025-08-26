package provider_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/systmms/dsops/pkg/provider"
)

// ProviderContractTest defines a standard test suite that all providers must pass
type ProviderContractTest struct {
	// CreateProvider creates a new instance of the provider to test
	CreateProvider func(t *testing.T) provider.Provider
	
	// SetupTestSecret creates a test secret in the provider
	// Returns the key to use for retrieval and a cleanup function
	SetupTestSecret func(t *testing.T, p provider.Provider) (key string, cleanup func())
	
	// Skip certain tests if the provider doesn't support them
	SkipValidation bool
	SkipMetadata   bool
}

// RunProviderContractTests runs the standard provider contract test suite
func RunProviderContractTests(t *testing.T, contract ProviderContractTest) {
	t.Run("Contract", func(t *testing.T) {
		t.Run("Name", func(t *testing.T) {
			testProviderName(t, contract)
		})
		
		t.Run("Capabilities", func(t *testing.T) {
			testProviderCapabilities(t, contract)
		})
		
		if !contract.SkipValidation {
			t.Run("Validate", func(t *testing.T) {
				testProviderValidate(t, contract)
			})
		}
		
		t.Run("Resolve", func(t *testing.T) {
			testProviderResolve(t, contract)
		})
		
		t.Run("ResolveNotFound", func(t *testing.T) {
			testProviderResolveNotFound(t, contract)
		})
		
		if !contract.SkipMetadata {
			t.Run("Describe", func(t *testing.T) {
				testProviderDescribe(t, contract)
			})
		}
		
		t.Run("ContextCancellation", func(t *testing.T) {
			testProviderContextCancellation(t, contract)
		})
	})
}

func testProviderName(t *testing.T, contract ProviderContractTest) {
	p := contract.CreateProvider(t)
	
	name := p.Name()
	if name == "" {
		t.Error("Provider.Name() returned empty string")
	}
	
	// Verify name is consistent
	name2 := p.Name()
	if name != name2 {
		t.Errorf("Provider.Name() not consistent: %q != %q", name, name2)
	}
}

func testProviderCapabilities(t *testing.T, contract ProviderContractTest) {
	p := contract.CreateProvider(t)
	
	caps := p.Capabilities()
	
	// Capabilities should be consistent
	caps2 := p.Capabilities()
	if caps.SupportsVersioning != caps2.SupportsVersioning ||
		caps.SupportsMetadata != caps2.SupportsMetadata ||
		caps.RequiresAuth != caps2.RequiresAuth {
		t.Error("Provider.Capabilities() not consistent between calls")
	}
	
	// If auth is required, auth methods should be specified
	if caps.RequiresAuth && len(caps.AuthMethods) == 0 {
		t.Error("Provider requires auth but specifies no auth methods")
	}
}

func testProviderValidate(t *testing.T, contract ProviderContractTest) {
	p := contract.CreateProvider(t)
	ctx := context.Background()
	
	// Validate should complete without hanging
	done := make(chan error, 1)
	go func() {
		done <- p.Validate(ctx)
	}()
	
	select {
	case err := <-done:
		// Provider might not be configured, which is OK for tests
		if err != nil {
			// Should return a proper error, not panic
			t.Logf("Provider validation failed (expected in test environment): %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Error("Provider.Validate() timed out after 5 seconds")
	}
}

func testProviderResolve(t *testing.T, contract ProviderContractTest) {
	if contract.SetupTestSecret == nil {
		t.Skip("SetupTestSecret not provided, skipping resolve test")
		return
	}
	
	p := contract.CreateProvider(t)
	key, cleanup := contract.SetupTestSecret(t, p)
	defer cleanup()
	
	ctx := context.Background()
	ref := provider.Reference{
		Key: key,
	}
	
	// Test successful resolution
	secret, err := p.Resolve(ctx, ref)
	if err != nil {
		t.Fatalf("Provider.Resolve() failed: %v", err)
	}
	
	// Verify we got a value
	if secret.Value == "" {
		t.Error("Provider.Resolve() returned empty value")
	}
}

func testProviderResolveNotFound(t *testing.T, contract ProviderContractTest) {
	p := contract.CreateProvider(t)
	ctx := context.Background()
	
	// Use a key that definitely doesn't exist
	ref := provider.Reference{
		Key: "this-secret-definitely-does-not-exist-" + time.Now().Format("20060102150405"),
	}
	
	secret, err := p.Resolve(ctx, ref)
	if err == nil {
		t.Errorf("Provider.Resolve() should fail for non-existent key, got value: %q", secret.Value)
	}
	
	// Check if it's a NotFoundError
	var notFoundErr provider.NotFoundError
	if errors.As(err, &notFoundErr) {
		// Good, it's the expected error type
		t.Logf("Got expected NotFoundError: %v", err)
	} else {
		// It's OK if provider returns a different error, but log it
		t.Logf("Provider returned error (not NotFoundError): %v", err)
	}
}

func testProviderDescribe(t *testing.T, contract ProviderContractTest) {
	if contract.SetupTestSecret == nil {
		t.Skip("SetupTestSecret not provided, skipping describe test")
		return
	}
	
	p := contract.CreateProvider(t)
	key, cleanup := contract.SetupTestSecret(t, p)
	defer cleanup()
	
	ctx := context.Background()
	ref := provider.Reference{
		Key: key,
	}
	
	// Test successful describe
	metadata, err := p.Describe(ctx, ref)
	if err != nil {
		// Some providers might not support metadata
		caps := p.Capabilities()
		if !caps.SupportsMetadata {
			t.Skip("Provider doesn't support metadata")
		}
		t.Fatalf("Provider.Describe() failed: %v", err)
	}
	
	// Verify we got some metadata
	if !metadata.Exists {
		t.Error("Provider.Describe() returned Exists=false for existing secret")
	}
}

func testProviderContextCancellation(t *testing.T, contract ProviderContractTest) {
	p := contract.CreateProvider(t)
	
	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	
	ref := provider.Reference{
		Key: "any-key",
	}
	
	// Provider should respect context cancellation
	_, err := p.Resolve(ctx, ref)
	if err == nil {
		t.Error("Provider.Resolve() should fail with cancelled context")
	}
	
	// Check if it's a context error
	if errors.Is(err, context.Canceled) {
		t.Logf("Got expected context.Canceled error: %v", err)
	} else {
		// It's OK if provider wraps the error differently
		t.Logf("Provider returned error with cancelled context: %v", err)
	}
}

// MockProvider is a simple mock provider for testing the contract test suite itself
type MockProvider struct {
	name         string
	values       map[string]string
	failValidate bool
}

func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:   name,
		values: make(map[string]string),
	}
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	select {
	case <-ctx.Done():
		return provider.SecretValue{}, ctx.Err()
	default:
	}
	
	value, exists := m.values[ref.Key]
	if !exists {
		return provider.SecretValue{}, provider.NotFoundError{
			Provider: m.name,
			Key:      ref.Key,
		}
	}
	
	return provider.SecretValue{
		Value: value,
	}, nil
}

func (m *MockProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	select {
	case <-ctx.Done():
		return provider.Metadata{}, ctx.Err()
	default:
	}
	
	_, exists := m.values[ref.Key]
	return provider.Metadata{
		Exists: exists,
		Type:   "mock-secret",
	}, nil
}

func (m *MockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		RequiresAuth:       false,
	}
}

func (m *MockProvider) Validate(ctx context.Context) error {
	if m.failValidate {
		return errors.New("mock validation failed")
	}
	return nil
}

// TestMockProviderContract tests that our mock provider passes the contract
func TestMockProviderContract(t *testing.T) {
	contract := ProviderContractTest{
		CreateProvider: func(t *testing.T) provider.Provider {
			return NewMockProvider("test-mock")
		},
		SetupTestSecret: func(t *testing.T, p provider.Provider) (string, func()) {
			mock := p.(*MockProvider)
			key := "test-secret"
			mock.values[key] = "test-value"
			return key, func() {
				delete(mock.values, key)
			}
		},
	}
	
	RunProviderContractTests(t, contract)
}