package providers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

// TestLiteralProviderContract runs the complete contract test suite against
// the literal provider.
func TestLiteralProviderContract(t *testing.T) {
	// Create provider with test data
	testData := map[string]string{
		"test-secret-1": "test-value-1",
		"test-secret-2": "test-value-2",
		"database-url":  "postgres://localhost:5432/testdb",
		"api-key":       "api-key-12345",
	}

	literalProvider := providers.NewLiteralProvider("test-literal", testData)

	// Build test case
	tc := testutil.ProviderTestCase{
		Name:     "literal",
		Provider: literalProvider,
		TestData: map[string]provider.SecretValue{
			"test-secret-1": {Value: "test-value-1"},
			"test-secret-2": {Value: "test-value-2"},
			"database-url":  {Value: "postgres://localhost:5432/testdb"},
			"api-key":       {Value: "api-key-12345"},
		},
		SkipValidation: false, // Literal provider has Validate()
	}

	// Run all contract tests
	testutil.RunProviderContractTests(t, tc)
}

// TestLiteralProviderName validates name consistency
func TestLiteralProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{
			name:         "simple_name",
			providerName: "literal",
			want:         "literal",
		},
		{
			name:         "custom_name",
			providerName: "my-literal-provider",
			want:         "my-literal-provider",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := providers.NewLiteralProvider(tt.providerName, nil)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

// TestLiteralProviderResolve validates secret resolution
func TestLiteralProviderResolve(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name      string
		values    map[string]string
		key       string
		wantValue string
		wantErr   bool
	}{
		{
			name:      "simple_value",
			values:    map[string]string{"test": "value"},
			key:       "test",
			wantValue: "value",
			wantErr:   false,
		},
		{
			name:      "complex_value",
			values:    map[string]string{"db": "postgres://user:pass@localhost/db"},
			key:       "db",
			wantValue: "postgres://user:pass@localhost/db",
			wantErr:   false,
		},
		{
			name:      "missing_key",
			values:    map[string]string{"exists": "value"},
			key:       "missing",
			wantValue: "",
			wantErr:   true,
		},
		{
			name:      "empty_value",
			values:    map[string]string{"empty": ""},
			key:       "empty",
			wantValue: "",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := providers.NewLiteralProvider("test", tt.values)
			ref := provider.Reference{
				Provider: "test",
				Key:      tt.key,
			}

			secret, err := p.Resolve(ctx, ref)

			if tt.wantErr {
				assert.Error(t, err)
				var notFoundErr *provider.NotFoundError
				assert.ErrorAs(t, err, &notFoundErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantValue, secret.Value)
			assert.Equal(t, "1", secret.Version)
			assert.NotZero(t, secret.UpdatedAt)
			assert.NotNil(t, secret.Metadata)
		})
	}
}

// TestLiteralProviderDescribe validates metadata retrieval
func TestLiteralProviderDescribe(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name       string
		values     map[string]string
		key        string
		wantExists bool
		wantSize   int
	}{
		{
			name:       "existing_secret",
			values:     map[string]string{"test": "value"},
			key:        "test",
			wantExists: true,
			wantSize:   5, // len("value")
		},
		{
			name:       "missing_secret",
			values:     map[string]string{"exists": "value"},
			key:        "missing",
			wantExists: false,
			wantSize:   0,
		},
		{
			name:       "empty_value",
			values:     map[string]string{"empty": ""},
			key:        "empty",
			wantExists: true,
			wantSize:   0,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := providers.NewLiteralProvider("test", tt.values)
			ref := provider.Reference{
				Provider: "test",
				Key:      tt.key,
			}

			meta, err := p.Describe(ctx, ref)

			require.NoError(t, err)
			assert.Equal(t, tt.wantExists, meta.Exists)
			if tt.wantExists {
				assert.Equal(t, "1", meta.Version)
				assert.Equal(t, tt.wantSize, meta.Size)
				assert.Equal(t, "string", meta.Type)
				assert.NotNil(t, meta.Tags)
			}
		})
	}
}

// TestLiteralProviderCapabilities validates capability reporting
func TestLiteralProviderCapabilities(t *testing.T) {
	t.Parallel()

	p := providers.NewLiteralProvider("test", nil)
	caps := p.Capabilities()

	// Literal provider does not support versioning
	assert.False(t, caps.SupportsVersioning)

	// Literal provider supports metadata
	assert.True(t, caps.SupportsMetadata)

	// Literal provider does not support watching
	assert.False(t, caps.SupportsWatching)

	// Literal provider does not support binary
	assert.False(t, caps.SupportsBinary)

	// Literal provider does not require auth
	assert.False(t, caps.RequiresAuth)

	// No auth methods
	assert.Empty(t, caps.AuthMethods)
}

// TestLiteralProviderValidate validates validation logic
func TestLiteralProviderValidate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	tests := []struct {
		name    string
		values  map[string]string
		wantErr bool
	}{
		{
			name:    "empty_provider",
			values:  nil,
			wantErr: false,
		},
		{
			name:    "provider_with_values",
			values:  map[string]string{"test": "value"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := providers.NewLiteralProvider("test", tt.values)
			err := p.Validate(ctx)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestLiteralProviderSetValue validates dynamic value updates
func TestLiteralProviderSetValue(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	p := providers.NewLiteralProvider("test", nil)

	// Initially empty
	ref := provider.Reference{Provider: "test", Key: "new-key"}
	_, err := p.Resolve(ctx, ref)
	assert.Error(t, err) // Should not exist

	// Add value
	p.SetValue("new-key", "new-value")

	// Now should exist
	secret, err := p.Resolve(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "new-value", secret.Value)

	// Update value
	p.SetValue("new-key", "updated-value")

	// Should return updated value
	secret, err = p.Resolve(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "updated-value", secret.Value)
}

// TestLiteralProviderConcurrency validates thread safety
func TestLiteralProviderConcurrency(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	ctx := context.Background()
	p := providers.NewLiteralProvider("test", map[string]string{
		"key": "value",
	})

	const goroutines = 100
	errors := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			ref := provider.Reference{Provider: "test", Key: "key"}
			secret, err := p.Resolve(ctx, ref)
			if err != nil {
				errors <- err
				return
			}
			if secret.Value != "value" {
				errors <- assert.AnError
			}
		}()
	}

	// Wait a bit for goroutines
	time.Sleep(100 * time.Millisecond)
	close(errors)

	// Check for errors
	for err := range errors {
		t.Error(err)
	}
}

// TestLiteralProviderContextCancellation validates context handling
func TestLiteralProviderContextCancellation(t *testing.T) {
	t.Parallel()

	p := providers.NewLiteralProvider("test", map[string]string{
		"test": "value",
	})

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ref := provider.Reference{Provider: "test", Key: "test"}

	// Literal provider is synchronous, so it may complete before checking context
	// This test just ensures it doesn't panic with cancelled context
	_, err := p.Resolve(ctx, ref)

	// Either succeeds (completed before context check) or returns context error
	if err != nil {
		t.Logf("Context cancellation resulted in error (OK): %v", err)
	}
}

// TestLiteralProviderNilValues validates behavior with nil values map
func TestLiteralProviderNilValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	// Create provider with nil values
	p := providers.NewLiteralProvider("test", nil)

	// Should handle nil gracefully
	ref := provider.Reference{Provider: "test", Key: "any-key"}
	_, err := p.Resolve(ctx, ref)
	assert.Error(t, err) // Should return NotFoundError

	// Describe should also work
	meta, err := p.Describe(ctx, ref)
	require.NoError(t, err)
	assert.False(t, meta.Exists)
}
