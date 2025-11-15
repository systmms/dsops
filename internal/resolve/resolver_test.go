package resolve

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
)

// createTestConfig creates a minimal test configuration
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Logger: logging.New(false, true), // debug=false, noColor=true
		Definition: &config.Definition{
			Version: 1,
			SecretStores: map[string]config.SecretStoreConfig{
				"test-provider": {
					Type: "literal",
				},
			},
		},
	}
}

// TestResolverSimpleResolution tests basic secret resolution without dependencies
func TestResolverSimpleResolution(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		envVars       config.Environment
		providerData  map[string]string
		expectedVars  map[string]string
		expectedError bool
	}{
		{
			name: "single literal variable",
			envVars: config.Environment{
				"DB_HOST": {Literal: "localhost"},
			},
			expectedVars: map[string]string{
				"DB_HOST": "localhost",
			},
		},
		{
			name: "single provider variable",
			envVars: config.Environment{
				"DB_PASSWORD": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "database/password",
					},
				},
			},
			providerData: map[string]string{
				"database/password": "secret-password-123",
			},
			expectedVars: map[string]string{
				"DB_PASSWORD": "secret-password-123",
			},
		},
		{
			name: "mixed literal and provider variables",
			envVars: config.Environment{
				"DB_HOST": {Literal: "localhost"},
				"DB_PASSWORD": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "database/password",
					},
				},
			},
			providerData: map[string]string{
				"database/password": "secret-password-123",
			},
			expectedVars: map[string]string{
				"DB_HOST":     "localhost",
				"DB_PASSWORD": "secret-password-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test configuration
			cfg := createTestConfig(t)

			// Create resolver
			resolver := New(cfg)

			// Setup fake provider
			fakeProvider := fakes.NewFakeProvider("test-provider")
			for key, value := range tt.providerData {
				fakeProvider.WithSecret(key, provider.SecretValue{
					Value: value,
				})
			}
			resolver.RegisterProvider("test-provider", fakeProvider)

			// Resolve variables
			result, err := resolver.ResolveEnvironment(context.Background(), tt.envVars)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedVars, result)
		})
	}
}

// TestResolverDependencyChains tests resolution of variables with dependencies
func TestResolverDependencyChains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		envVars       config.Environment
		providerData  map[string]string
		expectedVars  map[string]string
		expectedError bool
	}{
		{
			name: "transform dependency chain",
			envVars: config.Environment{
				"DB_PASSWORD_ENCODED": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "database/password",
					},
				},
				"DB_PASSWORD": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "database/password",
					},
					Transform: "base64_decode",
				},
			},
			providerData: map[string]string{
				"database/password": "c2VjcmV0LXBhc3N3b3JkLTEyMw==", // base64 encoded "secret-password-123"
			},
			expectedVars: map[string]string{
				"DB_PASSWORD_ENCODED": "c2VjcmV0LXBhc3N3b3JkLTEyMw==",
				"DB_PASSWORD":         "secret-password-123",
			},
		},
		{
			name: "multiple independent variables",
			envVars: config.Environment{
				"API_KEY_1": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "api/key1",
					},
				},
				"API_KEY_2": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "api/key2",
					},
				},
				"API_KEY_3": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "api/key3",
					},
				},
			},
			providerData: map[string]string{
				"api/key1": "key-1",
				"api/key2": "key-2",
				"api/key3": "key-3",
			},
			expectedVars: map[string]string{
				"API_KEY_1": "key-1",
				"API_KEY_2": "key-2",
				"API_KEY_3": "key-3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test configuration
			cfg := createTestConfig(t)

			// Create resolver
			resolver := New(cfg)

			// Setup fake provider
			fakeProvider := fakes.NewFakeProvider("test-provider")
			for key, value := range tt.providerData {
				fakeProvider.WithSecret(key, provider.SecretValue{
					Value: value,
				})
			}
			resolver.RegisterProvider("test-provider", fakeProvider)

			// Resolve variables
			result, err := resolver.ResolveEnvironment(context.Background(), tt.envVars)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedVars, result)
		})
	}
}

// TestResolverParallelResolution tests concurrent resolution of multiple variables
func TestResolverParallelResolution(t *testing.T) {
	t.Parallel()

	// Create a large number of variables to test parallel execution
	envVars := make(config.Environment)
	providerData := make(map[string]string)
	expectedVars := make(map[string]string)

	for i := 0; i < 50; i++ {
		varName := fmt.Sprintf("VAR_%d", i)
		key := fmt.Sprintf("secret/var%d", i)
		value := fmt.Sprintf("value-%d", i)

		envVars[varName] = config.Variable{
			From: &config.Reference{
				Provider: "test-provider",
				Key:      key,
			},
		}
		providerData[key] = value
		expectedVars[varName] = value
	}

	// Create test configuration
	cfg := createTestConfig(t)

	// Create resolver
	resolver := New(cfg)

	// Setup fake provider
	fakeProvider := fakes.NewFakeProvider("test-provider")
	for key, value := range providerData {
		fakeProvider.WithSecret(key, provider.SecretValue{
			Value: value,
		})
	}
	resolver.RegisterProvider("test-provider", fakeProvider)

	// Resolve variables (should execute in parallel)
	result, err := resolver.ResolveEnvironment(context.Background(), envVars)

	require.NoError(t, err)
	assert.Equal(t, expectedVars, result)
	assert.Equal(t, 50, len(result), "Should resolve all 50 variables")
}

// TestResolverErrorAggregation tests error handling and aggregation
func TestResolverErrorAggregation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		envVars       config.Environment
		providerData  map[string]string
		providerErrors map[string]error
		expectedError bool
		errorContains string
	}{
		{
			name: "single provider error",
			envVars: config.Environment{
				"DB_PASSWORD": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "database/password",
					},
				},
			},
			providerErrors: map[string]error{
				"database/password": fmt.Errorf("secret not found"),
			},
			expectedError: true,
			errorContains: "Failed to resolve variable 'DB_PASSWORD'",
		},
		{
			name: "multiple provider errors",
			envVars: config.Environment{
				"DB_PASSWORD": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "database/password",
					},
				},
				"API_KEY": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "api/key",
					},
				},
			},
			providerErrors: map[string]error{
				"database/password": fmt.Errorf("secret not found"),
				"api/key":           fmt.Errorf("access denied"),
			},
			expectedError: true,
			errorContains: "Failed to resolve 2 variables",
		},
		{
			name: "optional variable error is ignored",
			envVars: config.Environment{
				"DB_PASSWORD": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "database/password",
					},
					Optional: true,
				},
				"API_KEY": {
					From: &config.Reference{
						Provider: "test-provider",
						Key:      "api/key",
					},
				},
			},
			providerData: map[string]string{
				"api/key": "valid-key",
			},
			providerErrors: map[string]error{
				"database/password": fmt.Errorf("secret not found"),
			},
			expectedError: false, // Optional variable error is ignored
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create test configuration
			cfg := createTestConfig(t)

			// Create resolver
			resolver := New(cfg)

			// Setup fake provider
			fakeProvider := fakes.NewFakeProvider("test-provider")
			for key, value := range tt.providerData {
				fakeProvider.WithSecret(key, provider.SecretValue{
					Value: value,
				})
			}
			for key, err := range tt.providerErrors {
				fakeProvider.WithError(key, err)
			}
			resolver.RegisterProvider("test-provider", fakeProvider)

			// Resolve variables
			_, err := resolver.ResolveEnvironment(context.Background(), tt.envVars)

			if tt.expectedError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestResolverProviderRegistration tests provider registration and lookup
func TestResolverProviderRegistration(t *testing.T) {
	t.Parallel()

	cfg := createTestConfig(t)

	resolver := New(cfg)

	// Test registering a provider
	fakeProvider := fakes.NewFakeProvider("test-provider")
	resolver.RegisterProvider("test-provider", fakeProvider)

	// Test getting a registered provider
	retrieved, exists := resolver.GetProvider("test-provider")
	assert.True(t, exists)
	assert.Equal(t, fakeProvider, retrieved)

	// Test getting a non-existent provider
	_, exists = resolver.GetProvider("non-existent")
	assert.False(t, exists)

	// Test getting all registered providers
	allProviders := resolver.GetRegisteredProviders()
	assert.Equal(t, 1, len(allProviders))
	assert.Contains(t, allProviders, "test-provider")
}

// TestResolverMissingProviderError tests error when provider is not registered
func TestResolverMissingProviderError(t *testing.T) {
	t.Parallel()

	cfg := createTestConfig(t)

	resolver := New(cfg)

	envVars := config.Environment{
		"DB_PASSWORD": {
			From: &config.Reference{
				Provider: "missing-provider",
				Key:      "database/password",
			},
		},
	}

	_, err := resolver.ResolveEnvironment(context.Background(), envVars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "provider not found")
}

// TestResolverServiceReferenceError tests that service references are rejected
func TestResolverServiceReferenceError(t *testing.T) {
	t.Parallel()

	cfg := createTestConfig(t)

	resolver := New(cfg)

	envVars := config.Environment{
		"DB_PASSWORD": {
			From: &config.Reference{
				Service: "postgres-prod",
			},
		},
	}

	_, err := resolver.ResolveEnvironment(context.Background(), envVars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "service references (svc://) are for credential rotation")
}

// TestResolverVariableWithNoSource tests error when variable has no source
func TestResolverVariableWithNoSource(t *testing.T) {
	t.Parallel()

	cfg := createTestConfig(t)

	resolver := New(cfg)

	envVars := config.Environment{
		"DB_PASSWORD": {
			// No Literal or From specified
		},
	}

	// ResolveVariablesConcurrently should return an error because the variable is required (not optional)
	_, err := resolver.ResolveVariablesConcurrently(context.Background(), envVars)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variable has no source defined")
}

// TestResolverPlan tests the Plan method (dry-run resolution)
func TestResolverPlan(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		envName       string
		environments  map[string]config.Environment
		expectedVars  int
		expectedError bool
	}{
		{
			name:    "plan single environment",
			envName: "test",
			environments: map[string]config.Environment{
				"test": {
					"DB_HOST": {Literal: "localhost"},
					"DB_PASSWORD": {
						From: &config.Reference{
							Provider: "test-provider",
							Key:      "database/password",
						},
					},
				},
			},
			expectedVars: 2,
		},
		{
			name:    "plan non-existent environment",
			envName: "non-existent",
			environments: map[string]config.Environment{
				"test": {
					"DB_HOST": {Literal: "localhost"},
				},
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := &config.Config{
				Logger: logging.New(false, true),
				Definition: &config.Definition{
					Version: 1,
					SecretStores: map[string]config.SecretStoreConfig{
						"test-provider": {
							Type: "literal",
						},
					},
					Envs: tt.environments,
				},
			}

			resolver := New(cfg)
			fakeProvider := fakes.NewFakeProvider("test-provider")
			resolver.RegisterProvider("test-provider", fakeProvider)

			result, err := resolver.Plan(context.Background(), tt.envName)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedVars, len(result.Variables))
		})
	}
}
