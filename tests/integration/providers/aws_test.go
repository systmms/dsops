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

func TestAWSSecretsManagerIntegration(t *testing.T) {
	// Skip if short mode (no Docker)
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start LocalStack for AWS services emulation
	env := testutil.StartDockerEnv(t, []string{"localstack"})
	defer env.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get LocalStack client for seeding test data
	localstackClient := env.LocalStackClient()

	t.Run("basic_secret_retrieval", func(t *testing.T) {
		// Seed test secret in Secrets Manager
		testSecret := map[string]interface{}{
			"username": "admin",
			"password": "test-password-123",
			"host":     "localhost",
		}

		err := localstackClient.CreateSecret("test/database/credentials", testSecret)
		require.NoError(t, err, "Failed to seed test secret in Secrets Manager")

		// Create AWS Secrets Manager provider
		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err, "Failed to create AWS Secrets Manager provider")

		// Resolve secret
		ref := provider.Reference{
			Key: "test/database/credentials",
		}

		secret, err := awsProvider.Resolve(ctx, ref)
		require.NoError(t, err, "Failed to resolve secret from Secrets Manager")

		// Verify secret values
		assert.Contains(t, secret.Value, "admin")
		assert.Contains(t, secret.Value, "test-password-123")
		assert.Contains(t, secret.Value, "localhost")
	})

	t.Run("json_secret_retrieval", func(t *testing.T) {
		// Seed JSON-formatted secret
		testSecret := map[string]interface{}{
			"api_key":    "key-abc123",
			"api_secret": "secret-xyz789",
			"region":     "us-east-1",
		}

		err := localstackClient.CreateSecret("test/api/credentials", testSecret)
		require.NoError(t, err)

		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "test/api/credentials",
		}

		secret, err := awsProvider.Resolve(ctx, ref)
		require.NoError(t, err)

		assert.Contains(t, secret.Value, "key-abc123")
		assert.Contains(t, secret.Value, "secret-xyz789")
		assert.Contains(t, secret.Value, "us-east-1")
	})

	t.Run("secret_not_found", func(t *testing.T) {
		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "nonexistent/secret/path",
		}

		_, err = awsProvider.Resolve(ctx, ref)
		assert.Error(t, err, "Expected error for nonexistent secret")
		assert.Contains(t, err.Error(), "not found", "Error should indicate secret not found")
	})

	t.Run("provider_validate", func(t *testing.T) {
		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err)

		// Validate should succeed with LocalStack
		err = awsProvider.Validate(ctx)
		assert.NoError(t, err, "Validate should succeed with valid AWS configuration")
	})

	t.Run("provider_capabilities", func(t *testing.T) {
		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err)

		caps := awsProvider.Capabilities()

		// AWS Secrets Manager supports versioning
		assert.True(t, caps.SupportsVersioning, "AWS Secrets Manager should support versioning")

		// AWS Secrets Manager supports metadata
		assert.True(t, caps.SupportsMetadata, "AWS Secrets Manager should support metadata")
	})

	t.Run("hierarchical_secret_paths", func(t *testing.T) {
		// Test deeply nested secret paths
		secrets := map[string]map[string]interface{}{
			"prod/database/mysql":      {"password": "mysql-pass-123"},
			"prod/database/postgresql": {"password": "pg-pass-456"},
			"dev/database/mysql":       {"password": "mysql-dev-789"},
		}

		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err)

		for path, data := range secrets {
			err := localstackClient.CreateSecret(path, data)
			require.NoError(t, err, "Failed to create secret: %s", path)

			ref := provider.Reference{Key: path}
			secret, err := awsProvider.Resolve(ctx, ref)
			require.NoError(t, err, "Failed to resolve secret: %s", path)

			assert.Contains(t, secret.Value, "password",
				"Secret should contain password for path: %s", path)
		}
	})

	t.Run("concurrent_secret_access", func(t *testing.T) {
		// Seed a test secret
		testSecret := map[string]interface{}{
			"value": "concurrent-test-secret",
		}

		err := localstackClient.CreateSecret("test/concurrent", testSecret)
		require.NoError(t, err)

		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err)

		// Run 50 concurrent resolutions
		numGoroutines := 50
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				ref := provider.Reference{
					Key: "test/concurrent",
				}

				secret, err := awsProvider.Resolve(ctx, ref)
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
	})
}

func TestAWSSSMParameterStoreIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"localstack"})
	defer env.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	localstackClient := env.LocalStackClient()

	t.Run("basic_parameter_retrieval", func(t *testing.T) {
		// Seed test parameter in SSM Parameter Store
		err := localstackClient.PutParameter("/test/database/host", "localhost")
		require.NoError(t, err, "Failed to seed test parameter in SSM")

		// Create AWS SSM provider
		awsProvider, err := providers.NewAWSSSMProvider("aws-ssm-test", env.LocalStackConfig())
		require.NoError(t, err, "Failed to create AWS SSM provider")

		// Resolve parameter
		ref := provider.Reference{
			Key: "/test/database/host",
		}

		secret, err := awsProvider.Resolve(ctx, ref)
		require.NoError(t, err, "Failed to resolve parameter from SSM")

		// Verify parameter value (SSM returns the raw value, not JSON)
		assert.Equal(t, "localhost", secret.Value, "Parameter value should match")
	})

	t.Run("hierarchical_parameters", func(t *testing.T) {
		// Test hierarchical parameter paths
		parameters := map[string]string{
			"/app/prod/database/host":     "prod-db.example.com",
			"/app/prod/database/port":     "5432",
			"/app/dev/database/host":      "dev-db.example.com",
			"/app/dev/database/port":      "5433",
			"/app/shared/api_endpoint":    "https://api.example.com",
		}

		awsProvider, err := providers.NewAWSSSMProvider("aws-ssm-test", env.LocalStackConfig())
		require.NoError(t, err)

		for path, value := range parameters {
			err := localstackClient.PutParameter(path, value)
			require.NoError(t, err, "Failed to put parameter: %s", path)

			ref := provider.Reference{Key: path}
			secret, err := awsProvider.Resolve(ctx, ref)
			require.NoError(t, err, "Failed to resolve parameter: %s", path)

			assert.Contains(t, secret.Value, value,
				"Parameter value mismatch for path: %s", path)
		}
	})

	t.Run("parameter_not_found", func(t *testing.T) {
		awsProvider, err := providers.NewAWSSSMProvider("aws-ssm-test", env.LocalStackConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "/nonexistent/parameter",
		}

		_, err = awsProvider.Resolve(ctx, ref)
		assert.Error(t, err, "Expected error for nonexistent parameter")
		assert.Contains(t, err.Error(), "not found", "Error should indicate parameter not found")
	})

	t.Run("provider_validate", func(t *testing.T) {
		awsProvider, err := providers.NewAWSSSMProvider("aws-ssm-test", env.LocalStackConfig())
		require.NoError(t, err)

		// Validate should succeed with LocalStack
		err = awsProvider.Validate(ctx)
		assert.NoError(t, err, "Validate should succeed with valid AWS configuration")
	})

	t.Run("concurrent_parameter_access", func(t *testing.T) {
		// Seed a test parameter
		err := localstackClient.PutParameter("/test/concurrent/param", "concurrent-value")
		require.NoError(t, err)

		awsProvider, err := providers.NewAWSSSMProvider("aws-ssm-test", env.LocalStackConfig())
		require.NoError(t, err)

		// Run 50 concurrent resolutions
		numGoroutines := 50
		results := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func() {
				ref := provider.Reference{
					Key: "/test/concurrent/param",
				}

				secret, err := awsProvider.Resolve(ctx, ref)
				if err != nil {
					results <- err
					return
				}

				if !assert.Contains(t, secret.Value, "concurrent-value") {
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
	})
}

func TestAWSSecretsManagerDescribe(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"localstack"})
	defer env.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	localstackClient := env.LocalStackClient()

	t.Run("describe_returns_metadata", func(t *testing.T) {
		// Seed test secret
		testSecret := map[string]interface{}{
			"password": "test-secret-123",
		}

		err := localstackClient.CreateSecret("test/metadata/secret", testSecret)
		require.NoError(t, err)

		awsProvider, err := providers.NewAWSSecretsManagerProvider("aws-test", env.LocalStackConfig())
		require.NoError(t, err)

		ref := provider.Reference{
			Key: "test/metadata/secret",
		}

		// Describe should return metadata without secret values
		metadata, err := awsProvider.Describe(ctx, ref)
		require.NoError(t, err)

		// Metadata should NOT contain secret values
		// (Describe returns metadata, not the actual secret)
		assert.NotNil(t, metadata, "Metadata should not be nil")
	})
}
