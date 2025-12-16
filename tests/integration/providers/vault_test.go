package providers_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers/vault"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

func TestVaultProviderIntegration(t *testing.T) {
	// Skip if short mode (no Docker)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start Docker Compose services
	env := testutil.StartDockerEnv(t, []string{"vault"})
	defer env.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get Vault client for seeding test data
	vaultClient := env.VaultClient()

	t.Run("basic_secret_retrieval", func(t *testing.T) {
		// Seed test data in Vault KV v2
		testSecret := map[string]interface{}{
			"password": "test-secret-123",
			"username": "testuser",
		}

		err := vaultClient.Write("secret/data/test/basic", testSecret)
		require.NoError(t, err, "Failed to seed test secret in Vault")

		// Create Vault provider
		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err, "Failed to create Vault provider")

		// Resolve secret
		ref := provider.Reference{
			Key: "secret/data/test/basic",
		}

		secret, err := vaultProvider.Resolve(ctx, ref)
		require.NoError(t, err, "Failed to resolve secret from Vault")

		// Verify secret value is a JSON string
		assert.NotEmpty(t, secret.Value, "Secret value should not be empty")
		assert.Contains(t, secret.Value, "test-secret-123", "Secret should contain password")
		assert.Contains(t, secret.Value, "testuser", "Secret should contain username")
	})

	t.Run("nested_secret_path", func(t *testing.T) {
		// Test deeply nested secret paths
		testSecret := map[string]interface{}{
			"api_key":     "key-abc123",
			"api_secret":  "secret-xyz789",
			"environment": "test",
		}

		err := vaultClient.Write("secret/data/app/production/database", testSecret)
		require.NoError(t, err)

		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "secret/data/app/production/database",
		}

		secret, err := vaultProvider.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.NotEmpty(t, secret.Value)
		assert.Contains(t, secret.Value, "key-abc123")
		assert.Contains(t, secret.Value, "secret-xyz789")
		assert.Contains(t, secret.Value, "test")
	})

	t.Run("secret_not_found", func(t *testing.T) {
		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "secret/data/nonexistent/path",
		}

		_, err = vaultProvider.Resolve(ctx, ref)
		assert.Error(t, err, "Expected error for nonexistent secret")
		assert.Contains(t, err.Error(), "not found", "Error should indicate not found")
	})

	t.Run("provider_validate", func(t *testing.T) {
		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err)

		// Validate should succeed with correct credentials
		err = vaultProvider.Validate(ctx)
		assert.NoError(t, err, "Validate should succeed with correct Vault token")
	})

	t.Run("provider_capabilities", func(t *testing.T) {
		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err)

		caps := vaultProvider.Capabilities()

		// Vault should support versioning
		assert.True(t, caps.SupportsVersioning, "Vault should support versioning")

		// Vault should support metadata
		assert.True(t, caps.SupportsMetadata, "Vault should support metadata")
	})

	t.Run("multiple_secrets_parallel", func(t *testing.T) {
		// Seed multiple secrets
		secrets := map[string]map[string]interface{}{
			"secret/data/test/secret1": {"value": "secret1-value"},
			"secret/data/test/secret2": {"value": "secret2-value"},
			"secret/data/test/secret3": {"value": "secret3-value"},
		}

		for path, data := range secrets {
			err := vaultClient.Write(path, data)
			require.NoError(t, err)
		}

		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err)

		// Resolve secrets in parallel
		type result struct {
			path string
			sec  provider.SecretValue
			err  error
		}

		results := make(chan result, len(secrets))

		for path := range secrets {
			path := path // Capture loop variable
			go func() {
				ref := provider.Reference{Key: path}
				sec, err := vaultProvider.Resolve(ctx, ref)
				results <- result{path: path, sec: sec, err: err}
			}()
		}

		// Collect results
		for i := 0; i < len(secrets); i++ {
			res := <-results
			assert.NoError(t, res.err, "Parallel resolution failed for %s", res.path)
			assert.NotEmpty(t, res.sec.Value, "Secret value should not be empty")
		}
	})

	t.Run("special_characters_in_secret", func(t *testing.T) {
		// Test secrets with special characters
		testSecret := map[string]interface{}{
			"password": "p@$$w0rd!#&*()[]{}|\\<>?",
			"json":     `{"nested":"value with spaces"}`,
			"unicode":  "Hello 世界 🌍",
		}

		err := vaultClient.Write("secret/data/test/special", testSecret)
		require.NoError(t, err)

		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "secret/data/test/special",
		}

		secret, err := vaultProvider.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.NotEmpty(t, secret.Value)
		assert.Contains(t, secret.Value, testSecret["password"])
		assert.Contains(t, secret.Value, testSecret["json"])
		assert.Contains(t, secret.Value, testSecret["unicode"])
	})
}

func TestVaultProviderInvalidAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"vault"})
	defer env.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Run("invalid_token", func(t *testing.T) {
		invalidConfig := map[string]interface{}{
			"address": "http://127.0.0.1:8200",
			"token":   "invalid-token-12345",
		}

		vaultProvider, err := vault.NewVaultProvider("vault-test", invalidConfig)
		require.NoError(t, err, "Provider creation should succeed")

		// Validate only checks config presence, not token validity
		// So we test that Resolve fails with invalid token instead
		ref := provider.Reference{Key: "secret/data/test"}
		_, err = vaultProvider.Resolve(ctx, ref)
		require.Error(t, err, "Resolve should fail with invalid token")
		assert.Contains(t, err.Error(), "permission denied", "Error should indicate permission denied")
	})

	t.Run("invalid_address", func(t *testing.T) {
		invalidConfig := map[string]interface{}{
			"address": "http://127.0.0.1:9999", // Wrong port
			"token":   "test-root-token",
		}

		vaultProvider, err := vault.NewVaultProvider("vault-test", invalidConfig)
		require.NoError(t, err, "Provider creation should succeed")

		// Resolution should fail with connection error
		ref := provider.Reference{Key: "secret/data/test"}
		_, err = vaultProvider.Resolve(ctx, ref)
		assert.Error(t, err, "Resolve should fail with invalid address")
	})
}

func TestVaultProviderDescribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"vault"})
	defer env.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vaultClient := env.VaultClient()

	t.Run("describe_returns_metadata", func(t *testing.T) {
		// Seed test secret
		testSecret := map[string]interface{}{
			"password": "test-secret-123",
		}

		err := vaultClient.Write("secret/data/test/metadata", testSecret)
		require.NoError(t, err)

		vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "secret/data/test/metadata",
		}

		// Describe should return metadata without secret values
		metadata, err := vaultProvider.Describe(ctx, ref)
		require.NoError(t, err)

		// Metadata should NOT contain secret values
		// (Describe returns metadata, not the actual secret)
		assert.NotNil(t, metadata, "Metadata should not be nil")
	})
}

func TestVaultProviderConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"vault"})
	defer env.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	vaultClient := env.VaultClient()

	// Seed a test secret
	testSecret := map[string]interface{}{
		"password": "concurrent-test-secret",
	}

	err := vaultClient.Write("secret/data/test/concurrent", testSecret)
	require.NoError(t, err)

	vaultProvider, err := vault.NewVaultProvider("vault-test", env.VaultConfig())
	require.NoError(t, err)

	// Run 100 concurrent resolutions
	numGoroutines := 100
	results := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			ref := provider.Reference{
				Key: "secret/data/test/concurrent",
			}

			secret, err := vaultProvider.Resolve(ctx, ref)
			if err != nil {
				results <- err
				return
			}

			if !assert.Contains(t, secret.Value, "concurrent-test-secret") {
				results <- assert.AnError
				return
			}

			results <- nil
		}()
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		err := <-results
		assert.NoError(t, err, "Concurrent resolution should succeed")
	}
}
