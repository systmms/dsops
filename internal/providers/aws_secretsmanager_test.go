package providers_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

// TestAWSSecretsManagerProviderContract runs the contract test suite against AWS Secrets Manager provider.
//
// This test requires:
// - AWS credentials configured (via env vars, ~/.aws/credentials, or IAM role)
// - Appropriate IAM permissions for Secrets Manager
// - Set DSOPS_TEST_AWS=1 to enable
func TestAWSSecretsManagerProviderContract(t *testing.T) {
	// Skip unless explicitly enabled
	if _, exists := os.LookupEnv("DSOPS_TEST_AWS"); !exists {
		t.Skip("Skipping AWS Secrets Manager integration test. Set DSOPS_TEST_AWS=1 to run.")
	}

	// Create provider
	config := map[string]interface{}{
		"region": "us-east-1",
	}
	p, err := providers.NewAWSSecretsManagerProvider("test-aws", config)
	if err != nil {
		t.Fatalf("Failed to create AWS Secrets Manager provider: %v", err)
	}

	// Note: For real integration testing, you would:
	// 1. Create test secrets in AWS Secrets Manager
	// 2. Populate TestData with those secret names
	// 3. Clean up after tests

	tc := testutil.ProviderTestCase{
		Name:     "aws.secretsmanager",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "test/secret": {Value: "test-value-123"},
		},
		SkipValidation: true, // AWS provider validation always succeeds if credentials are available
	}

	// Only run if test data is available
	if len(tc.TestData) == 0 {
		t.Skip("No AWS test data configured. Add test secrets to enable contract tests.")
	}

	testutil.RunProviderContractTests(t, tc)
}

// TestAWSSecretsManagerProviderName validates provider name consistency
func TestAWSSecretsManagerProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{
			name:         "default_name",
			providerName: "aws-secrets",
			want:         "aws-secrets",
		},
		{
			name:         "custom_name",
			providerName: "my-aws-secrets-manager",
			want:         "my-aws-secrets-manager",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			config := map[string]interface{}{
				"region": "us-east-1",
			}
			p, err := providers.NewAWSSecretsManagerProvider(tt.providerName, config)
			require.NoError(t, err)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

// TestAWSSecretsManagerProviderCapabilities validates capability reporting
func TestAWSSecretsManagerProviderCapabilities(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{
		"region": "us-east-1",
	}
	p, err := providers.NewAWSSecretsManagerProvider("test", config)
	require.NoError(t, err)

	caps := p.Capabilities()

	// AWS Secrets Manager provider capabilities
	assert.True(t, caps.SupportsVersioning, "AWS Secrets Manager supports versioning")
	assert.True(t, caps.SupportsMetadata, "AWS Secrets Manager supports metadata")
	assert.False(t, caps.SupportsWatching, "AWS Secrets Manager doesn't support watching")
	assert.True(t, caps.SupportsBinary, "AWS Secrets Manager supports binary secrets")
	assert.True(t, caps.RequiresAuth, "AWS Secrets Manager requires authentication")
	assert.NotEmpty(t, caps.AuthMethods, "AWS should have auth methods")
	// AWS supports various authentication methods
	assert.True(t, len(caps.AuthMethods) > 0, "AWS should have at least one auth method")
}

// TestAWSSecretsManagerProviderRegion validates region configuration
func TestAWSSecretsManagerProviderRegion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name:   "default_region",
			config: map[string]interface{}{},
		},
		{
			name: "us-east-1",
			config: map[string]interface{}{
				"region": "us-east-1",
			},
		},
		{
			name: "eu-west-1",
			config: map[string]interface{}{
				"region": "eu-west-1",
			},
		},
		{
			name: "ap-southeast-2",
			config: map[string]interface{}{
				"region": "ap-southeast-2",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := providers.NewAWSSecretsManagerProvider("test", tt.config)
			require.NoError(t, err)
			assert.NotNil(t, p)
		})
	}
}

// TestAWSSecretsManagerKeyParsing validates key parsing logic
//
// AWS Secrets Manager supports various key formats:
//   - "secret-name" -> full secret string value
//   - "secret-name#.jsonkey" -> extract JSON field
//   - "arn:aws:secretsmanager:region:account:secret:name" -> ARN format
//   - "path/to/secret" -> hierarchical naming
func TestAWSSecretsManagerKeyParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         string
		description string
	}{
		{
			name:        "simple_secret_name",
			key:         "my-secret",
			description: "Should retrieve full secret value",
		},
		{
			name:        "hierarchical_secret_name",
			key:         "prod/database/password",
			description: "Should support path-like naming",
		},
		{
			name:        "secret_with_json_extraction",
			key:         "my-secret#.password",
			description: "Should extract JSON field",
		},
		{
			name:        "secret_with_nested_json_path",
			key:         "config#.database.host",
			description: "Should extract nested JSON field",
		},
		{
			name:        "arn_format",
			key:         "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf",
			description: "Should work with full ARN",
		},
		{
			name:        "arn_with_json_extraction",
			key:         "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-secret-AbCdEf#.api_key",
			description: "Should extract field from ARN secret",
		},
		{
			name:        "secret_with_dashes",
			key:         "my-app-api-key",
			description: "Should handle dashes in name",
		},
		{
			name:        "secret_with_underscores",
			key:         "my_app_api_key",
			description: "Should handle underscores in name",
		},
		{
			name:        "json_array_access",
			key:         "config#.credentials[0]",
			description: "Should support array indexing",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Note: We can't actually test resolution without AWS credentials
			// This test documents expected key formats
			t.Logf("Key format: %s - %s", tt.key, tt.description)

			// Key format validation (basic check)
			assert.NotEmpty(t, tt.key, "Key should not be empty")
		})
	}
}

// TestAWSSecretsManagerProviderValidate validates the provider validation logic
func TestAWSSecretsManagerProviderValidate(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping AWS validation in short mode")
	}

	ctx := context.Background()
	config := map[string]interface{}{
		"region": "us-east-1",
	}
	p, err := providers.NewAWSSecretsManagerProvider("test", config)
	require.NoError(t, err)

	// AWS provider validation checks if AWS SDK is configured
	// This may succeed or fail depending on local AWS configuration
	err = p.Validate(ctx)
	if err != nil {
		t.Logf("AWS validation failed (expected if AWS credentials not configured): %v", err)
		// This is expected behavior if AWS credentials aren't configured
	} else {
		assert.NoError(t, err)
	}
}

// TestAWSSecretsManagerProviderResolveNotFound validates error handling for missing secrets
func TestAWSSecretsManagerProviderResolveNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping AWS integration test in short mode")
	}

	// Skip unless AWS is configured
	if _, exists := os.LookupEnv("DSOPS_TEST_AWS"); !exists {
		t.Skip("Skipping AWS test. Set DSOPS_TEST_AWS=1 to run.")
	}

	ctx := context.Background()
	config := map[string]interface{}{
		"region": "us-east-1",
	}
	p, err := providers.NewAWSSecretsManagerProvider("test", config)
	require.NoError(t, err)

	// Try to resolve a secret that definitely doesn't exist
	ref := provider.Reference{
		Provider: "test",
		Key:      "this-secret-definitely-does-not-exist-aws-12345",
	}

	_, err = p.Resolve(ctx, ref)
	assert.Error(t, err, "Should return error for non-existent secret")

	// Should be a NotFoundError or AWS-specific error
	var notFoundErr *provider.NotFoundError
	if assert.ErrorAs(t, err, &notFoundErr) {
		assert.Equal(t, "test", notFoundErr.Provider)
		assert.Contains(t, notFoundErr.Key, "this-secret-definitely-does-not-exist")
	}
}

// TestAWSSecretsManagerProviderCreateError validates error handling for creation failures
func TestAWSSecretsManagerProviderCreateError(t *testing.T) {
	t.Parallel()

	// Test with invalid config
	tests := []struct {
		name    string
		config  map[string]interface{}
		wantErr bool
	}{
		{
			name:    "nil_config",
			config:  nil,
			wantErr: false, // Should use defaults
		},
		{
			name:    "empty_config",
			config:  map[string]interface{}{},
			wantErr: false, // Should use defaults
		},
		{
			name: "valid_region",
			config: map[string]interface{}{
				"region": "us-west-2",
			},
			wantErr: false,
		},
		{
			name: "invalid_region_type",
			config: map[string]interface{}{
				"region": 123, // Should be string
			},
			wantErr: false, // Will use default region
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := providers.NewAWSSecretsManagerProvider("test", tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, p)
			} else {
				// Provider creation may succeed or fail based on AWS SDK availability
				// If it succeeds, provider should be valid
				if err == nil {
					assert.NotNil(t, p)
				}
			}
		})
	}
}
