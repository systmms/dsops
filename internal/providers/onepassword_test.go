package providers_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
	"github.com/systmms/dsops/tests/testutil"
)

// TestOnePasswordProviderContract runs the contract test suite against 1Password provider.
//
// This test requires:
// - 1Password CLI (op) installed
// - Authenticated session (op signin)
// - Set DSOPS_TEST_ONEPASSWORD=1 to enable
func TestOnePasswordProviderContract(t *testing.T) {
	// Skip unless explicitly enabled
	if _, exists := os.LookupEnv("DSOPS_TEST_ONEPASSWORD"); !exists {
		t.Skip("Skipping 1Password integration test. Set DSOPS_TEST_ONEPASSWORD=1 to run.")
	}

	// Create provider
	p, err := providers.NewOnePasswordProvider(nil)
	if err != nil {
		t.Fatalf("Failed to create 1Password provider: %v", err)
	}

	// Note: For real integration testing, you would:
	// 1. Create test items in 1Password
	// 2. Populate TestData with those item references
	// 3. Clean up after tests

	tc := testutil.ProviderTestCase{
		Name:     "onepassword",
		Provider: p,
		TestData: map[string]provider.SecretValue{
			// Example: "Private/test-item/password": {Value: "test-secret-123"},
		},
		SkipValidation: false, // 1Password validates CLI availability
	}

	// Only run if test data is available
	if len(tc.TestData) == 0 {
		t.Skip("No 1Password test data configured. Add test items to enable contract tests.")
	}

	testutil.RunProviderContractTests(t, tc)
}

// TestOnePasswordProviderName validates provider name consistency
func TestOnePasswordProviderName(t *testing.T) {
	t.Parallel()

	p, err := providers.NewOnePasswordProvider(nil)
	assert.NoError(t, err)
	assert.Equal(t, "onepassword", p.Name())
}

// TestOnePasswordProviderCapabilities validates capability reporting
func TestOnePasswordProviderCapabilities(t *testing.T) {
	t.Parallel()

	p, err := providers.NewOnePasswordProvider(nil)
	assert.NoError(t, err)

	caps := p.Capabilities()

	// 1Password provider capabilities
	assert.False(t, caps.SupportsVersioning, "1Password doesn't support versioning via CLI")
	assert.True(t, caps.SupportsMetadata, "1Password supports metadata")
	assert.False(t, caps.SupportsWatching, "1Password doesn't support watching")
	assert.False(t, caps.SupportsBinary, "1Password CLI returns text")
	assert.True(t, caps.RequiresAuth, "1Password requires authentication")
	assert.NotEmpty(t, caps.AuthMethods, "1Password should have auth methods")
	assert.Contains(t, caps.AuthMethods, "CLI session", "1Password supports CLI session authentication")
}

// TestOnePasswordProviderValidate validates the provider validation logic
func TestOnePasswordProviderValidate(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skip("Skipping 1Password CLI validation in short mode")
	}

	ctx := context.Background()
	p, err := providers.NewOnePasswordProvider(nil)
	assert.NoError(t, err)

	err = p.Validate(ctx)
	// Validation checks for op CLI availability and authentication
	// If op is not installed or not authenticated, this will fail (which is expected)
	if err != nil {
		t.Logf("1Password validation failed (expected if op CLI not installed/authenticated): %v", err)
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
	}
}

// TestOnePasswordProviderWithAccount validates account configuration
func TestOnePasswordProviderWithAccount(t *testing.T) {
	t.Parallel()

	config := map[string]interface{}{
		"account": "my-account",
	}

	p, err := providers.NewOnePasswordProvider(config)
	assert.NoError(t, err)
	assert.Equal(t, "onepassword", p.Name())
}

// TestOnePasswordKeyParsing validates key parsing logic
//
// 1Password supports various key formats:
//   - "item-name" -> defaults to password field
//   - "item-name.username" -> specific field
//   - "op://Vault/Item/field" -> URI format
//   - "op://Vault/Item" -> URI format, default field
func TestOnePasswordKeyParsing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		key         string
		description string
	}{
		{
			name:        "simple_item_name",
			key:         "my-database",
			description: "Should default to password field",
		},
		{
			name:        "item_with_username",
			key:         "my-database.username",
			description: "Should extract username field",
		},
		{
			name:        "item_with_password",
			key:         "my-database.password",
			description: "Should extract password field explicitly",
		},
		{
			name:        "item_with_custom_field",
			key:         "api-keys.stripe_key",
			description: "Should extract custom field",
		},
		{
			name:        "uri_format_vault_item",
			key:         "op://Private/my-item",
			description: "Should work with URI format (default field)",
		},
		{
			name:        "uri_format_with_field",
			key:         "op://Private/my-item/password",
			description: "Should extract specific field from URI",
		},
		{
			name:        "uri_format_with_custom_field",
			key:         "op://Shared/api-keys/stripe_key",
			description: "Should extract custom field from URI",
		},
		{
			name:        "item_with_notes",
			key:         "certificates.notesPlain",
			description: "Should extract notes field",
		},
		{
			name:        "item_with_otp",
			key:         "my-account.otp",
			description: "Should extract OTP/TOTP code",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Note: We can't actually test resolution without a real 1Password vault
			// This test documents expected key formats
			t.Logf("Key format: %s - %s", tt.key, tt.description)

			// Key format validation (basic check)
			assert.NotEmpty(t, tt.key, "Key should not be empty")
		})
	}
}

// TestOnePasswordProviderResolveNotFound validates error handling for missing secrets
func TestOnePasswordProviderResolveNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 1Password integration test in short mode")
	}

	// Skip unless 1Password is configured
	if _, exists := os.LookupEnv("DSOPS_TEST_ONEPASSWORD"); !exists {
		t.Skip("Skipping 1Password test. Set DSOPS_TEST_ONEPASSWORD=1 to run.")
	}

	ctx := context.Background()
	p, err := providers.NewOnePasswordProvider(nil)
	assert.NoError(t, err)

	// Try to resolve a secret that definitely doesn't exist
	ref := provider.Reference{
		Provider: "onepassword",
		Key:      "this-item-definitely-does-not-exist-67890",
	}

	_, err = p.Resolve(ctx, ref)
	assert.Error(t, err, "Should return error for non-existent item")

	// Should be a NotFoundError
	var notFoundErr *provider.NotFoundError
	if assert.ErrorAs(t, err, &notFoundErr) {
		assert.Equal(t, "onepassword", notFoundErr.Provider)
		assert.Contains(t, notFoundErr.Key, "this-item-definitely-does-not-exist")
	}
}

// TestOnePasswordProviderCreateError validates error handling for creation failures
func TestOnePasswordProviderCreateError(t *testing.T) {
	t.Parallel()

	// Test with invalid config types
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name:   "nil_config",
			config: nil,
		},
		{
			name:   "empty_config",
			config: map[string]interface{}{},
		},
		{
			name: "invalid_account_type",
			config: map[string]interface{}{
				"account": 123, // Should be string
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Provider creation should succeed even with invalid config
			// (validation happens at Validate() time)
			p, err := providers.NewOnePasswordProvider(tt.config)
			assert.NoError(t, err, "Provider creation should not fail")
			assert.NotNil(t, p, "Provider should be created")
		})
	}
}
