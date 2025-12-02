// Package fakes provides manual fake implementations for testing.
//
// Fakes are test doubles that have working implementations but take shortcuts
// compared to production code. They are more realistic than mocks but simpler
// than real implementations, making them ideal for testing.
package fakes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/systmms/dsops/pkg/provider"
)

// FakeProvider is a manual fake implementation of provider.Provider interface.
//
// It provides a predictable, configurable fake provider for unit testing without
// requiring real provider services or Docker containers. The fake stores secrets
// in memory and can be configured to return specific values or errors.
//
// Example usage:
//
//	fake := fakes.NewFakeProvider("test").
//	    WithSecret("db/password", provider.SecretValue{Value: "secret123"}).
//	    WithError("api/key", errors.New("connection failed"))
//
//	// Use in tests
//	secret, err := fake.Resolve(ctx, provider.Reference{Key: "db/password"})
type FakeProvider struct {
	name         string
	capabilities provider.Capabilities

	// Test data storage
	secrets  map[string]provider.SecretValue  // key -> secret value
	metadata map[string]provider.Metadata     // key -> metadata

	// Behavior control
	failOn       map[string]error // key -> error to return
	resolveDelay time.Duration    // simulate network latency
	callCount    map[string]int   // method call tracking

	// Thread safety
	mu sync.RWMutex
}

// NewFakeProvider creates a new FakeProvider with the given name.
//
// The provider starts with empty secrets and default capabilities.
// Use builder methods to configure secrets, metadata, and behavior.
func NewFakeProvider(name string) *FakeProvider {
	return &FakeProvider{
		name:      name,
		secrets:   make(map[string]provider.SecretValue),
		metadata:  make(map[string]provider.Metadata),
		failOn:    make(map[string]error),
		callCount: make(map[string]int),
		capabilities: provider.Capabilities{
			SupportsVersioning: true,
			SupportsMetadata:   true,
			SupportsWatching:   false,
			SupportsBinary:     false,
			RequiresAuth:       false,
			AuthMethods:        []string{},
		},
	}
}

// WithSecret adds a secret to the fake provider.
//
// Fluent API for configuring test data. The secret will be returned
// when Resolve is called with a matching key.
func (f *FakeProvider) WithSecret(key string, value provider.SecretValue) *FakeProvider {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Set default metadata if not provided
	if value.Metadata == nil {
		value.Metadata = make(map[string]string)
	}
	if value.Version == "" {
		value.Version = "v1"
	}
	if value.UpdatedAt.IsZero() {
		value.UpdatedAt = time.Now()
	}

	f.secrets[key] = value

	// Auto-create metadata entry
	f.metadata[key] = provider.Metadata{
		Exists:    true,
		Version:   value.Version,
		UpdatedAt: value.UpdatedAt,
		Size:      len(value.Value),
		Tags:      value.Metadata,
	}

	return f
}

// WithMetadata adds metadata for a secret.
//
// Fluent API for configuring secret metadata. This is used by the
// Describe method to return secret information without the value.
func (f *FakeProvider) WithMetadata(key string, meta provider.Metadata) *FakeProvider {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.metadata[key] = meta
	return f
}

// WithError configures the fake to return an error for a specific key.
//
// Fluent API for simulating error conditions. When Resolve is called
// with this key, the configured error will be returned instead of a secret.
func (f *FakeProvider) WithError(key string, err error) *FakeProvider {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.failOn[key] = err
	return f
}

// WithDelay adds artificial latency to Resolve calls.
//
// Fluent API for simulating network latency in tests. Useful for
// testing timeout handling and concurrent access patterns.
func (f *FakeProvider) WithDelay(d time.Duration) *FakeProvider {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.resolveDelay = d
	return f
}

// WithCapability sets a specific capability flag.
//
// Fluent API for configuring provider capabilities. Use this to test
// behavior when certain features are supported or not supported.
func (f *FakeProvider) WithCapability(cap string, supported bool) *FakeProvider {
	f.mu.Lock()
	defer f.mu.Unlock()

	switch cap {
	case "versioning":
		f.capabilities.SupportsVersioning = supported
	case "metadata":
		f.capabilities.SupportsMetadata = supported
	case "watching":
		f.capabilities.SupportsWatching = supported
	case "binary":
		f.capabilities.SupportsBinary = supported
	case "auth":
		f.capabilities.RequiresAuth = supported
	}

	return f
}

// Name returns the provider's unique identifier.
func (f *FakeProvider) Name() string {
	return f.name
}

// Resolve retrieves a secret value from the fake provider.
//
// Returns the configured secret value for the key, or an error if
// one was configured with WithError(). Increments the call count
// for tracking in tests.
func (f *FakeProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	f.trackCall("Resolve")

	// Simulate network delay if configured
	if f.resolveDelay > 0 {
		select {
		case <-time.After(f.resolveDelay):
		case <-ctx.Done():
			return provider.SecretValue{}, ctx.Err()
		}
	}

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check for configured errors first
	if err, ok := f.failOn[ref.Key]; ok {
		return provider.SecretValue{}, err
	}

	// Check if secret exists
	secret, ok := f.secrets[ref.Key]
	if !ok {
		return provider.SecretValue{}, provider.NotFoundError{
			Provider: f.name,
			Key:      ref.Key,
		}
	}

	return secret, nil
}

// Describe returns metadata about a secret without retrieving its value.
//
// Returns the configured metadata for the key, or empty metadata with
// Exists=false if the secret doesn't exist.
func (f *FakeProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	f.trackCall("Describe")

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check for configured errors
	if err, ok := f.failOn[ref.Key]; ok {
		return provider.Metadata{}, err
	}

	// Check if metadata exists
	meta, ok := f.metadata[ref.Key]
	if !ok {
		// Return empty metadata with Exists=false
		return provider.Metadata{Exists: false}, nil
	}

	return meta, nil
}

// Capabilities returns the provider's supported features.
//
// Returns the configured capabilities. Use WithCapability to customize.
func (f *FakeProvider) Capabilities() provider.Capabilities {
	f.trackCall("Capabilities")

	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.capabilities
}

// Validate checks if the provider is properly configured.
//
// The fake provider always validates successfully unless explicitly
// configured to fail with WithError("_validate", err).
func (f *FakeProvider) Validate(ctx context.Context) error {
	f.trackCall("Validate")

	f.mu.RLock()
	defer f.mu.RUnlock()

	// Check if validation should fail
	if err, ok := f.failOn["_validate"]; ok {
		return err
	}

	return nil
}

// GetCallCount returns the number of times a method was called.
//
// Useful for verifying that certain operations occurred in tests.
// Method names: "Resolve", "Describe", "Capabilities", "Validate".
func (f *FakeProvider) GetCallCount(method string) int {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return f.callCount[method]
}

// ResetCallCount resets all method call counters to zero.
//
// Useful when sharing a fake provider across multiple test cases
// and needing fresh call counts for each case.
func (f *FakeProvider) ResetCallCount() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.callCount = make(map[string]int)
}

// trackCall increments the call counter for a method.
func (f *FakeProvider) trackCall(method string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.callCount[method]++
}

// String returns a string representation of the fake provider.
func (f *FakeProvider) String() string {
	f.mu.RLock()
	defer f.mu.RUnlock()

	return fmt.Sprintf("FakeProvider{name=%s, secrets=%d}", f.name, len(f.secrets))
}
