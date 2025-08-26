package providers

import (
	"context"
	"time"

	"github.com/systmms/dsops/pkg/provider"
)

// LiteralProvider provides literal values for testing and simple use cases
// It doesn't actually fetch from external systems, but allows testing the resolution pipeline
type LiteralProvider struct {
	name   string
	values map[string]string
}

// NewLiteralProvider creates a new literal provider with predefined values
func NewLiteralProvider(name string, values map[string]string) *LiteralProvider {
	if values == nil {
		values = make(map[string]string)
	}
	
	return &LiteralProvider{
		name:   name,
		values: values,
	}
}

// Name returns the provider's name
func (l *LiteralProvider) Name() string {
	return l.name
}

// Resolve retrieves a literal value
func (l *LiteralProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	value, exists := l.values[ref.Key]
	if !exists {
		return provider.SecretValue{}, &provider.NotFoundError{
			Provider: l.name,
			Key:      ref.Key,
		}
	}

	return provider.SecretValue{
		Value:     value,
		Version:   "1",
		UpdatedAt: time.Now(),
		Metadata: map[string]string{
			"provider": l.name,
			"type":     "literal",
		},
	}, nil
}

// Describe returns metadata about a literal value
func (l *LiteralProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	value, exists := l.values[ref.Key]
	if !exists {
		return provider.Metadata{
			Exists: false,
		}, nil
	}

	return provider.Metadata{
		Exists:    true,
		Version:   "1",
		UpdatedAt: time.Now(),
		Size:      len(value),
		Type:      "string",
		Tags: map[string]string{
			"provider": l.name,
			"type":     "literal",
		},
	}, nil
}

// Capabilities returns the provider's capabilities
func (l *LiteralProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       false,
		AuthMethods:        []string{},
	}
}

// Validate checks if the provider is properly configured
func (l *LiteralProvider) Validate(ctx context.Context) error {
	return nil // Literal provider is always valid
}

// SetValue sets a literal value (useful for testing)
func (l *LiteralProvider) SetValue(key, value string) {
	l.values[key] = value
}

// MockProvider provides mock values that simulate external provider behavior
type MockProvider struct {
	name     string
	values   map[string]string
	failures map[string]error
	delay    time.Duration
}

// NewMockProvider creates a new mock provider for testing
func NewMockProvider(name string) *MockProvider {
	return &MockProvider{
		name:     name,
		values:   make(map[string]string),
		failures: make(map[string]error),
		delay:    0,
	}
}

// Name returns the provider's name
func (m *MockProvider) Name() string {
	return m.name
}

// Resolve retrieves a mock value, potentially with simulated failures or delays
func (m *MockProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Simulate network delay
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return provider.SecretValue{}, ctx.Err()
		}
	}

	// Check for simulated failures
	if err, exists := m.failures[ref.Key]; exists {
		return provider.SecretValue{}, err
	}

	value, exists := m.values[ref.Key]
	if !exists {
		return provider.SecretValue{}, &provider.NotFoundError{
			Provider: m.name,
			Key:      ref.Key,
		}
	}

	return provider.SecretValue{
		Value:     value,
		Version:   "mock-v1",
		UpdatedAt: time.Now(),
		Metadata: map[string]string{
			"provider":  m.name,
			"type":      "mock",
			"simulated": "true",
		},
	}, nil
}

// Describe returns metadata about a mock value
func (m *MockProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	if err, exists := m.failures[ref.Key]; exists {
		return provider.Metadata{}, err
	}

	value, exists := m.values[ref.Key]
	return provider.Metadata{
		Exists:    exists,
		Version:   "mock-v1",
		UpdatedAt: time.Now(),
		Size:      len(value),
		Type:      "string",
		Tags: map[string]string{
			"provider":  m.name,
			"type":      "mock",
			"simulated": "true",
		},
	}, nil
}

// Capabilities returns the provider's capabilities
func (m *MockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: true,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       false,
		AuthMethods:        []string{},
	}
}

// Validate checks if the provider is properly configured
func (m *MockProvider) Validate(ctx context.Context) error {
	return nil // Mock provider is always valid
}

// SetValue sets a mock value
func (m *MockProvider) SetValue(key, value string) {
	m.values[key] = value
}

// SetFailure simulates a failure for a specific key
func (m *MockProvider) SetFailure(key string, err error) {
	m.failures[key] = err
}

// SetDelay sets a simulated network delay
func (m *MockProvider) SetDelay(delay time.Duration) {
	m.delay = delay
}

// JSONProvider creates mock JSON values for testing transforms
type JSONProvider struct {
	*MockProvider
}

// NewJSONProvider creates a provider with JSON test data
func NewJSONProvider(name string) *JSONProvider {
	mock := NewMockProvider(name)
	
	// Add some test JSON data
	mock.SetValue("user-config", `{"name": "john", "email": "john@example.com", "settings": {"theme": "dark"}}`)
	mock.SetValue("db-config", `{"url": "postgresql://user:pass@localhost:5432/db", "max_connections": 100}`)
	mock.SetValue("api-keys", `{"primary": "api-key-123", "secondary": "api-key-456"}`)
	
	return &JSONProvider{MockProvider: mock}
}