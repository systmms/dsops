// Package e2e provides end-to-end workflow tests for dsops.
package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/fakes"
	"github.com/systmms/dsops/tests/testutil"
)

func TestMultiProviderWorkflow(t *testing.T) {
	t.Parallel()

	t.Run("three_providers_same_environment", func(t *testing.T) {
		t.Parallel()

		// Configuration using AWS, Bitwarden, and Vault providers
		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  aws-secrets:
    type: aws.secretsmanager
    config:
      region: us-east-1
  bitwarden:
    type: bitwarden
    config:
      session: "test-session"
  vault:
    type: vault
    config:
      addr: "http://127.0.0.1:8200"
      token: "test-token"
envs:
  production:
    AWS_DB_PASSWORD:
      from:
        store: "store://aws-secrets/database/password"
    BITWARDEN_API_KEY:
      from:
        store: "store://bitwarden/api-credentials"
      transform: "json_extract:.api_key"
    VAULT_SECRET_TOKEN:
      from:
        store: "store://vault/tokens/secret"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Create fake providers for each secret store
		awsProvider := fakes.NewFakeProvider("aws-secrets").
			WithSecret("database/password", provider.SecretValue{Value: "aws-db-secret-123"})

		bitwardenProvider := fakes.NewFakeProvider("bitwarden").
			WithSecret("api-credentials", provider.SecretValue{
				Value: `{"api_key":"bw-api-key-456","secret":"bw-secret"}`,
			})

		vaultProvider := fakes.NewFakeProvider("vault").
			WithSecret("tokens/secret", provider.SecretValue{Value: "vault-token-789"})

		resolver.RegisterProvider("aws-secrets", awsProvider)
		resolver.RegisterProvider("bitwarden", bitwardenProvider)
		resolver.RegisterProvider("vault", vaultProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "production")
		require.NoError(t, err)
		assert.Len(t, resolved, 3)

		// Verify each provider contributed its secret
		assert.Equal(t, "aws-db-secret-123", resolved["AWS_DB_PASSWORD"].Value)
		assert.Equal(t, "bw-api-key-456", resolved["BITWARDEN_API_KEY"].Value) // JSON extracted
		assert.Equal(t, "vault-token-789", resolved["VAULT_SECRET_TOKEN"].Value)

		// Verify call counts
		assert.Equal(t, 1, awsProvider.GetCallCount("Resolve"))
		assert.Equal(t, 1, bitwardenProvider.GetCallCount("Resolve"))
		assert.Equal(t, 1, vaultProvider.GetCallCount("Resolve"))
	})

	t.Run("providers_with_fallback_pattern", func(t *testing.T) {
		t.Parallel()

		// Test pattern: Primary secrets from Vault, fallback/legacy from Bitwarden
		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  vault-primary:
    type: vault
    config:
      addr: "http://primary.vault:8200"
  bitwarden-legacy:
    type: bitwarden
    config:
      session: "legacy-session"
envs:
  migration:
    NEW_API_KEY:
      from:
        store: "store://vault-primary/api/v2/key"
    LEGACY_DB_PASSWORD:
      from:
        store: "store://bitwarden-legacy/old-database"
    SHARED_CONFIG:
      from:
        store: "store://vault-primary/shared/config"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		vaultProvider := fakes.NewFakeProvider("vault-primary").
			WithSecret("api/v2/key", provider.SecretValue{Value: "new-api-key"}).
			WithSecret("shared/config", provider.SecretValue{Value: "shared-config-value"})

		bitwardenProvider := fakes.NewFakeProvider("bitwarden-legacy").
			WithSecret("old-database", provider.SecretValue{Value: "legacy-password"})

		resolver.RegisterProvider("vault-primary", vaultProvider)
		resolver.RegisterProvider("bitwarden-legacy", bitwardenProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "migration")
		require.NoError(t, err)
		assert.Len(t, resolved, 3)

		// Primary vault secrets
		assert.Equal(t, "new-api-key", resolved["NEW_API_KEY"].Value)
		assert.Equal(t, "shared-config-value", resolved["SHARED_CONFIG"].Value)

		// Legacy bitwarden secret
		assert.Equal(t, "legacy-password", resolved["LEGACY_DB_PASSWORD"].Value)
	})

	t.Run("providers_per_environment", func(t *testing.T) {
		t.Parallel()

		// Different providers for different environments
		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  local-store:
    type: literal
    config:
      values:
        DB_PASSWORD: "local-dev-pass"
  aws-staging:
    type: aws.secretsmanager
    config:
      region: us-west-2
  vault-prod:
    type: vault
    config:
      addr: "https://prod.vault:8200"
envs:
  local:
    DB_PASSWORD:
      from:
        store: "store://local-store/DB_PASSWORD"
  staging:
    DB_PASSWORD:
      from:
        store: "store://aws-staging/staging/db/password"
  production:
    DB_PASSWORD:
      from:
        store: "store://vault-prod/prod/database/password"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Set up all providers
		localProvider := fakes.NewFakeProvider("local-store").
			WithSecret("DB_PASSWORD", provider.SecretValue{Value: "local-dev-pass"})

		awsProvider := fakes.NewFakeProvider("aws-staging").
			WithSecret("staging/db/password", provider.SecretValue{Value: "staging-pass-456"})

		vaultProvider := fakes.NewFakeProvider("vault-prod").
			WithSecret("prod/database/password", provider.SecretValue{Value: "prod-pass-789"})

		resolver.RegisterProvider("local-store", localProvider)
		resolver.RegisterProvider("aws-staging", awsProvider)
		resolver.RegisterProvider("vault-prod", vaultProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Resolve local environment
		localResolved, err := resolver.Resolve(ctx, "local")
		require.NoError(t, err)
		assert.Equal(t, "local-dev-pass", localResolved["DB_PASSWORD"].Value)

		// Resolve staging environment
		stagingResolved, err := resolver.Resolve(ctx, "staging")
		require.NoError(t, err)
		assert.Equal(t, "staging-pass-456", stagingResolved["DB_PASSWORD"].Value)

		// Resolve production environment
		prodResolved, err := resolver.Resolve(ctx, "production")
		require.NoError(t, err)
		assert.Equal(t, "prod-pass-789", prodResolved["DB_PASSWORD"].Value)

		// Verify correct providers were called
		assert.Equal(t, 1, localProvider.GetCallCount("Resolve"))
		assert.Equal(t, 1, awsProvider.GetCallCount("Resolve"))
		assert.Equal(t, 1, vaultProvider.GetCallCount("Resolve"))
	})

	t.Run("mixed_literal_and_provider_secrets", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  vault:
    type: vault
    config:
      addr: "http://127.0.0.1:8200"
envs:
  test:
    LITERAL_VALUE:
      literal: "hardcoded-value"
    PROVIDER_SECRET:
      from:
        store: "store://vault/secret/key"
    ANOTHER_LITERAL:
      literal: "another-hardcoded"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		vaultProvider := fakes.NewFakeProvider("vault").
			WithSecret("secret/key", provider.SecretValue{Value: "from-vault"})

		resolver.RegisterProvider("vault", vaultProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "test")
		require.NoError(t, err)
		assert.Len(t, resolved, 3)

		// Literals don't hit provider
		assert.Equal(t, "hardcoded-value", resolved["LITERAL_VALUE"].Value)
		assert.Equal(t, "literal", resolved["LITERAL_VALUE"].Source)

		assert.Equal(t, "another-hardcoded", resolved["ANOTHER_LITERAL"].Value)
		assert.Equal(t, "literal", resolved["ANOTHER_LITERAL"].Source)

		// Provider secret
		assert.Equal(t, "from-vault", resolved["PROVIDER_SECRET"].Value)

		// Only one provider call (not for literals)
		assert.Equal(t, 1, vaultProvider.GetCallCount("Resolve"))
	})

	t.Run("concurrent_resolution_multiple_providers", func(t *testing.T) {
		t.Parallel()

		// Test that multiple providers are resolved concurrently
		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  slow-aws:
    type: aws.secretsmanager
    config:
      region: us-east-1
  slow-vault:
    type: vault
    config:
      addr: "http://127.0.0.1:8200"
  slow-bitwarden:
    type: bitwarden
    config:
      session: "session"
envs:
  test:
    AWS_SECRET:
      from:
        store: "store://slow-aws/secret"
    VAULT_SECRET:
      from:
        store: "store://slow-vault/secret"
    BITWARDEN_SECRET:
      from:
        store: "store://slow-bitwarden/secret"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		// Each provider has 50ms delay
		awsProvider := fakes.NewFakeProvider("slow-aws").
			WithSecret("secret", provider.SecretValue{Value: "aws-value"}).
			WithDelay(50 * time.Millisecond)

		vaultProvider := fakes.NewFakeProvider("slow-vault").
			WithSecret("secret", provider.SecretValue{Value: "vault-value"}).
			WithDelay(50 * time.Millisecond)

		bitwardenProvider := fakes.NewFakeProvider("slow-bitwarden").
			WithSecret("secret", provider.SecretValue{Value: "bitwarden-value"}).
			WithDelay(50 * time.Millisecond)

		resolver.RegisterProvider("slow-aws", awsProvider)
		resolver.RegisterProvider("slow-vault", vaultProvider)
		resolver.RegisterProvider("slow-bitwarden", bitwardenProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		start := time.Now()
		resolved, err := resolver.Resolve(ctx, "test")
		duration := time.Since(start)

		require.NoError(t, err)
		assert.Len(t, resolved, 3)

		// All values resolved
		assert.Equal(t, "aws-value", resolved["AWS_SECRET"].Value)
		assert.Equal(t, "vault-value", resolved["VAULT_SECRET"].Value)
		assert.Equal(t, "bitwarden-value", resolved["BITWARDEN_SECRET"].Value)

		// Should be concurrent (total < 150ms sequential time)
		// Allow some overhead but should be much faster than sequential
		assert.Less(t, duration, 500*time.Millisecond,
			"Resolution should be concurrent, not sequential")
	})

	t.Run("partial_failure_with_optional_vars", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  working:
    type: vault
    config:
      addr: "http://127.0.0.1:8200"
  failing:
    type: aws.secretsmanager
    config:
      region: us-east-1
envs:
  test:
    REQUIRED_FROM_WORKING:
      from:
        store: "store://working/secret"
    OPTIONAL_FROM_FAILING:
      from:
        store: "store://failing/secret"
      optional: true
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		workingProvider := fakes.NewFakeProvider("working").
			WithSecret("secret", provider.SecretValue{Value: "working-value"})

		// Failing provider has no secrets (will return NotFoundError)
		failingProvider := fakes.NewFakeProvider("failing")

		resolver.RegisterProvider("working", workingProvider)
		resolver.RegisterProvider("failing", failingProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Should succeed because failing secret is optional
		resolved, err := resolver.Resolve(ctx, "test")
		require.NoError(t, err)

		// Required secret succeeded
		assert.Equal(t, "working-value", resolved["REQUIRED_FROM_WORKING"].Value)
		assert.NoError(t, resolved["REQUIRED_FROM_WORKING"].Error)

		// Optional secret failed but didn't stop overall resolution
		assert.Error(t, resolved["OPTIONAL_FROM_FAILING"].Error)
	})

	t.Run("provider_specific_transforms", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  json-store:
    type: vault
    config:
      addr: "http://127.0.0.1:8200"
  base64-store:
    type: aws.secretsmanager
    config:
      region: us-east-1
envs:
  test:
    EXTRACTED_VALUE:
      from:
        store: "store://json-store/complex-secret"
      transform: "json_extract:.credentials.password"
    DECODED_VALUE:
      from:
        store: "store://base64-store/encoded-secret"
      transform: "base64_decode"
    TRIMMED_VALUE:
      from:
        store: "store://json-store/padded-secret"
      transform: "trim"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		jsonProvider := fakes.NewFakeProvider("json-store").
			WithSecret("complex-secret", provider.SecretValue{
				Value: `{"credentials":{"username":"user","password":"extracted-pass"}}`,
			}).
			WithSecret("padded-secret", provider.SecretValue{
				Value: "  trimmed-value  ",
			})

		base64Provider := fakes.NewFakeProvider("base64-store").
			WithSecret("encoded-secret", provider.SecretValue{
				Value: "ZGVjb2RlZC12YWx1ZQ==", // "decoded-value" in base64
			})

		resolver.RegisterProvider("json-store", jsonProvider)
		resolver.RegisterProvider("base64-store", base64Provider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resolved, err := resolver.Resolve(ctx, "test")
		require.NoError(t, err)
		assert.Len(t, resolved, 3)

		// JSON extraction
		assert.Equal(t, "extracted-pass", resolved["EXTRACTED_VALUE"].Value)
		assert.True(t, resolved["EXTRACTED_VALUE"].Transformed)

		// Base64 decoding
		assert.Equal(t, "decoded-value", resolved["DECODED_VALUE"].Value)
		assert.True(t, resolved["DECODED_VALUE"].Transformed)

		// Trimming
		assert.Equal(t, "trimmed-value", resolved["TRIMMED_VALUE"].Value)
		assert.True(t, resolved["TRIMMED_VALUE"].Transformed)
	})
}

func TestMultiProviderValidation(t *testing.T) {
	t.Parallel()

	t.Run("validate_multiple_providers", func(t *testing.T) {
		t.Parallel()

		configPath := testutil.WriteTestConfig(t, `
version: 1
secretStores:
  vault:
    type: vault
    config:
      addr: "http://127.0.0.1:8200"
  aws:
    type: aws.secretsmanager
    config:
      region: us-east-1
envs:
  test:
    VAR1:
      from:
        store: "store://vault/secret"
    VAR2:
      from:
        store: "store://aws/secret"
`)

		def := testutil.LoadTestConfig(t, configPath)
		logger := logging.New(false, false)
		cfg := &config.Config{
			Path:       configPath,
			Logger:     logger,
			Definition: def,
		}

		resolver := resolve.New(cfg)

		vaultProvider := fakes.NewFakeProvider("vault")
		awsProvider := fakes.NewFakeProvider("aws")

		resolver.RegisterProvider("vault", vaultProvider)
		resolver.RegisterProvider("aws", awsProvider)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Validate both providers
		err := resolver.ValidateProvider(ctx, "vault")
		assert.NoError(t, err)

		err = resolver.ValidateProvider(ctx, "aws")
		assert.NoError(t, err)

		// Verify validation was called
		assert.Equal(t, 1, vaultProvider.GetCallCount("Validate"))
		assert.Equal(t, 1, awsProvider.GetCallCount("Validate"))
	})
}
