package resolve

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/policy"
	"github.com/systmms/dsops/pkg/provider"
)

// fakeValidatingProvider is a provider that can track validation calls
type fakeValidatingProvider struct {
	name          string
	validateErr   error
	validateDelay time.Duration
	validateCalls int
}

func (f *fakeValidatingProvider) Name() string {
	return f.name
}

func (f *fakeValidatingProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	return provider.SecretValue{Value: "test-value"}, nil
}

func (f *fakeValidatingProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	return provider.Metadata{}, nil
}

func (f *fakeValidatingProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

func (f *fakeValidatingProvider) Validate(ctx context.Context) error {
	f.validateCalls++
	if f.validateDelay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(f.validateDelay):
		}
	}
	return f.validateErr
}

func TestGetTimeoutSuggestion(t *testing.T) {
	tests := []struct {
		name          string
		providerName  string
		timeoutMs     int
		expectedHint  string
	}{
		{
			name:         "bitwarden_short_timeout",
			providerName: "bitwarden",
			timeoutMs:    5000,
			expectedHint: "increasing timeout_ms to 15000",
		},
		{
			name:         "bitwarden_long_timeout",
			providerName: "bitwarden",
			timeoutMs:    20000,
			expectedHint: "bw unlock",
		},
		{
			name:         "1password_short_timeout",
			providerName: "1password",
			timeoutMs:    5000,
			expectedHint: "increasing timeout_ms to 15000",
		},
		{
			name:         "1password_long_timeout",
			providerName: "1password",
			timeoutMs:    20000,
			expectedHint: "op signin",
		},
		{
			name:         "onepassword_alias",
			providerName: "onepassword",
			timeoutMs:    5000,
			expectedHint: "increasing timeout_ms to 15000",
		},
		{
			name:         "aws_short_timeout",
			providerName: "aws",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "aws_long_timeout",
			providerName: "aws",
			timeoutMs:    15000,
			expectedHint: "region is correct",
		},
		{
			name:         "aws_secretsmanager",
			providerName: "aws-secretsmanager",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "gcp_short_timeout",
			providerName: "gcp",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "gcp_long_timeout",
			providerName: "gcp",
			timeoutMs:    15000,
			expectedHint: "authentication",
		},
		{
			name:         "google_cloud_secret_manager",
			providerName: "google-cloud-secret-manager",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "azure_short_timeout",
			providerName: "azure",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "azure_long_timeout",
			providerName: "azure",
			timeoutMs:    15000,
			expectedHint: "authentication",
		},
		{
			name:         "azure_key_vault",
			providerName: "azure-key-vault",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "vault_short_timeout",
			providerName: "vault",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "vault_long_timeout",
			providerName: "vault",
			timeoutMs:    15000,
			expectedHint: "VAULT_ADDR",
		},
		{
			name:         "hashicorp_vault",
			providerName: "hashicorp-vault",
			timeoutMs:    3000,
			expectedHint: "increasing timeout_ms to 10000",
		},
		{
			name:         "unknown_short_timeout",
			providerName: "unknown-provider",
			timeoutMs:    5000,
			expectedHint: "increasing timeout_ms",
		},
		{
			name:         "unknown_long_timeout",
			providerName: "unknown-provider",
			timeoutMs:    20000,
			expectedHint: "network connectivity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestion := getTimeoutSuggestion(tt.providerName, tt.timeoutMs)
			assert.Contains(t, suggestion, tt.expectedHint)
		})
	}
}

func TestIsTimeoutError(t *testing.T) {
	t.Run("wraps_deadline_exceeded", func(t *testing.T) {
		err := isTimeoutError(context.DeadlineExceeded, "aws", 5000)
		require.Error(t, err)

		var userErr dserrors.UserError
		require.ErrorAs(t, err, &userErr)
		assert.Equal(t, "Provider operation timed out", userErr.Message)
		assert.Contains(t, userErr.Details, "5000ms")
	})

	t.Run("passes_through_other_errors", func(t *testing.T) {
		originalErr := fmt.Errorf("some other error")
		err := isTimeoutError(originalErr, "aws", 5000)
		assert.Equal(t, originalErr, err)
	})
}

func TestValidateProvider(t *testing.T) {
	t.Run("provider_not_registered", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"test": {Type: "literal"},
				},
			},
		}
		resolver := New(cfg)

		err := resolver.ValidateProvider(context.Background(), "nonexistent")
		require.Error(t, err)

		var configErr dserrors.ConfigError
		require.ErrorAs(t, err, &configErr)
		assert.Equal(t, "provider not registered", configErr.Message)
	})

	t.Run("provider_config_not_found", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version:   1,
				Providers: map[string]config.ProviderConfig{},
			},
		}
		resolver := New(cfg)

		// Register provider but don't add config
		prov := &fakeValidatingProvider{name: "test"}
		resolver.RegisterProvider("test", prov)

		err := resolver.ValidateProvider(context.Background(), "test")
		require.Error(t, err)
	})

	t.Run("successful_validation", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"test": {Type: "literal"},
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{name: "test"}
		resolver.RegisterProvider("test", prov)

		err := resolver.ValidateProvider(context.Background(), "test")
		assert.NoError(t, err)
		assert.Equal(t, 1, prov.validateCalls)
	})

	t.Run("validation_failure", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"test": {Type: "literal"},
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{
			name:        "test",
			validateErr: fmt.Errorf("validation failed"),
		}
		resolver.RegisterProvider("test", prov)

		err := resolver.ValidateProvider(context.Background(), "test")
		require.Error(t, err)
		// Error is wrapped by ProviderError with "provider error during"
		assert.Contains(t, err.Error(), "provider error during")
	})

	t.Run("validation_timeout", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"aws-test": {
						Type:      "aws-secretsmanager",
						TimeoutMs: 10, // Very short timeout
					},
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{
			name:          "aws-test",
			validateDelay: 100 * time.Millisecond, // Longer than timeout
		}
		resolver.RegisterProvider("aws-test", prov)

		err := resolver.ValidateProvider(context.Background(), "aws-test")
		require.Error(t, err)

		// Should be wrapped as a UserError with timeout suggestion
		var userErr dserrors.UserError
		if assert.ErrorAs(t, err, &userErr) {
			assert.Equal(t, "Provider operation timed out", userErr.Message)
		}
	})
}

func TestResolveWithEnvironment(t *testing.T) {
	t.Run("environment_not_found", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Envs:    map[string]config.Environment{},
			},
		}
		resolver := New(cfg)

		_, err := resolver.Resolve(context.Background(), "nonexistent")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to get environment")
	})

	t.Run("successful_resolution", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"literal": {
						Type:   "literal",
						Config: map[string]interface{}{"value": "test"},
					},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "literal",
								Key:      "value",
							},
						},
					},
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{name: "literal"}
		resolver.RegisterProvider("literal", prov)

		result, err := resolver.Resolve(context.Background(), "test")
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "test-value", result["VAR1"].Value)
	})
}

func TestEnforcePoliciesIntegration(t *testing.T) {
	t.Run("no_policies_configured", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"literal": {Type: "literal"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "literal",
								Key:      "value",
							},
						},
					},
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{name: "literal"}
		resolver.RegisterProvider("literal", prov)

		// Should work without policies
		_, err := resolver.Resolve(context.Background(), "test")
		require.NoError(t, err)
	})

	t.Run("with_policy_enforcement", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"literal": {Type: "literal"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "literal",
								Key:      "value",
							},
						},
					},
				},
				Policies: &policy.PolicyConfig{
					// Empty policy config - should pass
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{name: "literal"}
		resolver.RegisterProvider("literal", prov)

		result, err := resolver.Resolve(context.Background(), "test")
		require.NoError(t, err)
		require.Len(t, result, 1)
	})
}

func TestResolveEnvironmentWithErrors(t *testing.T) {
	t.Run("failed_variable_skipped_in_simple_map", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"failing": {Type: "failing"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "failing",
								Key:      "value",
							},
							Optional: true, // Optional so no error
						},
					},
				},
			},
		}
		resolver := New(cfg)

		// Create a provider that always fails
		failingProvider := &failingProviderImpl{name: "failing"}
		resolver.RegisterProvider("failing", failingProvider)

		result, err := resolver.ResolveEnvironment(context.Background(), cfg.Definition.Envs["test"])
		require.NoError(t, err)
		// Failed optional variable is not in the simple map
		assert.Len(t, result, 0)
	})

	t.Run("multiple_failures_aggregated", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"failing": {Type: "failing"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "failing",
								Key:      "value1",
							},
						},
						"VAR2": {
							From: &config.Reference{
								Provider: "failing",
								Key:      "value2",
							},
						},
					},
				},
			},
		}
		resolver := New(cfg)

		failingProvider := &failingProviderImpl{name: "failing"}
		resolver.RegisterProvider("failing", failingProvider)

		_, err := resolver.ResolveEnvironment(context.Background(), cfg.Definition.Envs["test"])
		require.Error(t, err)

		// Should aggregate multiple errors
		var userErr dserrors.UserError
		if assert.ErrorAs(t, err, &userErr) {
			assert.Contains(t, userErr.Message, "Failed to resolve 2 variables")
		}
	})

	t.Run("single_failure_not_aggregated", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"failing": {Type: "failing"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "failing",
								Key:      "value1",
							},
						},
					},
				},
			},
		}
		resolver := New(cfg)

		failingProvider := &failingProviderImpl{name: "failing"}
		resolver.RegisterProvider("failing", failingProvider)

		_, err := resolver.ResolveEnvironment(context.Background(), cfg.Definition.Envs["test"])
		require.Error(t, err)

		// Single error should be returned directly
		var userErr dserrors.UserError
		if assert.ErrorAs(t, err, &userErr) {
			assert.Contains(t, userErr.Message, "Failed to resolve variable 'VAR1'")
		}
	})
}

// failingProviderImpl is a provider that always fails to resolve
type failingProviderImpl struct {
	name string
}

func (f *failingProviderImpl) Name() string {
	return f.name
}

func (f *failingProviderImpl) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	return provider.SecretValue{}, fmt.Errorf("provider error: cannot resolve")
}

func (f *failingProviderImpl) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	return provider.Metadata{}, nil
}

func (f *failingProviderImpl) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

func (f *failingProviderImpl) Validate(ctx context.Context) error {
	return nil
}

func TestPlanWithErrors(t *testing.T) {
	t.Run("provider_not_found_in_plan", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"existing": {Type: "literal"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "nonexistent",
								Key:      "value",
							},
						},
					},
				},
			},
		}
		resolver := New(cfg)

		result, err := resolver.Plan(context.Background(), "test")
		require.NoError(t, err)
		require.Len(t, result.Variables, 1)
		assert.NotNil(t, result.Variables[0].Error)
		assert.Contains(t, result.Variables[0].Error.Error(), "provider 'nonexistent' not registered")
	})

	t.Run("plan_with_transforms", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"literal": {Type: "literal"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "literal",
								Key:      "value",
							},
							Transform: "base64_decode",
						},
					},
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{name: "literal"}
		resolver.RegisterProvider("literal", prov)

		result, err := resolver.Plan(context.Background(), "test")
		require.NoError(t, err)
		require.Len(t, result.Variables, 1)
		assert.Equal(t, "base64_decode", result.Variables[0].Transform)
	})

	t.Run("plan_with_optional_flag", func(t *testing.T) {
		cfg := &config.Config{
			Logger: logging.New(false, true),
			Definition: &config.Definition{
				Version: 1,
				Providers: map[string]config.ProviderConfig{
					"literal": {Type: "literal"},
				},
				Envs: map[string]config.Environment{
					"test": {
						"VAR1": {
							From: &config.Reference{
								Provider: "literal",
								Key:      "value",
							},
							Optional: true,
						},
					},
				},
			},
		}
		resolver := New(cfg)

		prov := &fakeValidatingProvider{name: "literal"}
		resolver.RegisterProvider("literal", prov)

		result, err := resolver.Plan(context.Background(), "test")
		require.NoError(t, err)
		require.Len(t, result.Variables, 1)
		assert.True(t, result.Variables[0].Optional)
	})
}

func TestWithProviderTimeout(t *testing.T) {
	t.Run("creates_timeout_context", func(t *testing.T) {
		ctx := context.Background()
		timeoutMs := 1000

		timeoutCtx, cancel := withProviderTimeout(ctx, timeoutMs)
		defer cancel()

		deadline, ok := timeoutCtx.Deadline()
		require.True(t, ok)

		// Deadline should be approximately 1 second from now
		expectedDeadline := time.Now().Add(1 * time.Second)
		assert.WithinDuration(t, expectedDeadline, deadline, 100*time.Millisecond)
	})

	t.Run("context_times_out", func(t *testing.T) {
		ctx := context.Background()
		timeoutMs := 10 // Very short timeout

		timeoutCtx, cancel := withProviderTimeout(ctx, timeoutMs)
		defer cancel()

		// Wait for timeout
		<-timeoutCtx.Done()
		assert.Equal(t, context.DeadlineExceeded, timeoutCtx.Err())
	})
}
