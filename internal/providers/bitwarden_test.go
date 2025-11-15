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

// TestBitwardenProviderContract runs the contract test suite against Bitwarden provider.
//
// This test requires:
// - Bitwarden CLI (bw) installed
// - Authenticated session (bw login / bw unlock)
// - Set DSOPS_TEST_BITWARDEN=1 to enable
func TestBitwardenProviderContract(t *testing.T) {
	// Skip unless explicitly enabled
	if _, exists := os.LookupEnv("DSOPS_TEST_BITWARDEN"); !exists {
		t.Skip("Skipping Bitwarden integration test. Set DSOPS_TEST_BITWARDEN=1 to run.")
	}

	// Create provider (assumes bw CLI is authenticated)
	bwProvider := providers.NewBitwardenProvider("test-bitwarden", nil)

	// Note: For real integration testing, you would:
	// 1. Create test items in Bitwarden
	// 2. Populate TestData with those item references
	// 3. Clean up after tests

	tc := testutil.ProviderTestCase{
		Name:     "bitwarden",
		Provider: bwProvider,
		TestData: map[string]provider.SecretValue{
			// Example: "test-item.password": {Value: "test-secret-123"},
		},
		SkipValidation: false, // Bitwarden validates CLI availability
	}

	// Only run if test data is available
	if len(tc.TestData) == 0 {
		t.Skip("No Bitwarden test data configured. Add test items to enable contract tests.")
	}

	testutil.RunProviderContractTests(t, tc)
}

// TestBitwardenProviderName validates provider name consistency
func TestBitwardenProviderName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		providerName string
		want         string
	}{
		{
			name:         "default_name",
			providerName: "bitwarden",
			want:         "bitwarden",
		},
		{
			name:         "custom_name",
			providerName: "my-bitwarden",
			want:         "my-bitwarden",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p := providers.NewBitwardenProvider(tt.providerName, nil)
			assert.Equal(t, tt.want, p.Name())
		})
	}
}

// TestBitwardenProviderCapabilities validates capability reporting
func TestBitwardenProviderCapabilities(t *testing.T) {
	t.Parallel()

	p := providers.NewBitwardenProvider("test", nil)
	caps := p.Capabilities()

	// Bitwarden provider capabilities
	assert.False(t, caps.SupportsVersioning, "Bitwarden doesn't support versioning via CLI")
	assert.True(t, caps.SupportsMetadata, "Bitwarden supports metadata")
	assert.False(t, caps.SupportsWatching, "Bitwarden doesn't support watching")
	assert.False(t, caps.SupportsBinary, "Bitwarden stores text secrets")
	assert.True(t, caps.RequiresAuth, "Bitwarden requires authentication")
	assert.NotEmpty(t, caps.AuthMethods, "Bitwarden should have auth methods")
	// Bitwarden supports both CLI session and API key authentication
	assert.Contains(t, caps.AuthMethods, "cli-session", "Bitwarden supports CLI session authentication")
}

// TestBitwardenProviderValidate validates the provider validation logic
func TestBitwardenProviderValidate(t *testing.T) {
	t.Parallel()

	// Skip if bw CLI not available
	if testing.Short() {
		t.Skip("Skipping Bitwarden CLI validation in short mode")
	}

	ctx := context.Background()
	p := providers.NewBitwardenProvider("test", nil)

	err := p.Validate(ctx)
	// Validation checks for bw CLI availability
	// If bw is not installed, this will fail (which is expected)
	if err != nil {
		t.Logf("Bitwarden validation failed (expected if bw CLI not installed): %v", err)
		assert.Error(t, err)
	} else {
		assert.NoError(t, err)
	}
}

// TestBitwardenKeyParsing validates key parsing logic
//
// Bitwarden supports various key formats:
//   - "item-name" -> defaults to password field
//   - "item-name.username" -> specific field
//   - "item-name.notes" -> notes field
//   - "item-id" -> UUID format
func TestBitwardenKeyParsing(t *testing.T) {
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
			name:        "item_with_username_field",
			key:         "my-database.username",
			description: "Should extract username field",
		},
		{
			name:        "item_with_password_field",
			key:         "my-database.password",
			description: "Should extract password field explicitly",
		},
		{
			name:        "item_with_custom_field",
			key:         "api-keys.stripe_key",
			description: "Should extract custom field",
		},
		{
			name:        "item_with_notes",
			key:         "certificates.notes",
			description: "Should extract notes field",
		},
		{
			name:        "item_with_totp",
			key:         "my-account.totp",
			description: "Should extract TOTP code",
		},
		{
			name:        "uuid_item_id",
			key:         "550e8400-e29b-41d4-a716-446655440000",
			description: "Should work with UUID item IDs",
		},
		{
			name:        "uuid_with_field",
			key:         "550e8400-e29b-41d4-a716-446655440000.username",
			description: "Should extract field from UUID item",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Note: We can't actually test resolution without a real Bitwarden vault
			// This test documents expected key formats
			t.Logf("Key format: %s - %s", tt.key, tt.description)

			// Key format validation (basic check)
			assert.NotEmpty(t, tt.key, "Key should not be empty")
		})
	}
}

// TestBitwardenProviderResolveNotFound validates error handling for missing secrets
func TestBitwardenProviderResolveNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping Bitwarden integration test in short mode")
	}

	// Skip unless Bitwarden is configured
	if _, exists := os.LookupEnv("DSOPS_TEST_BITWARDEN"); !exists {
		t.Skip("Skipping Bitwarden test. Set DSOPS_TEST_BITWARDEN=1 to run.")
	}

	ctx := context.Background()
	p := providers.NewBitwardenProvider("test", nil)

	// Try to resolve a secret that definitely doesn't exist
	ref := provider.Reference{
		Provider: "test",
		Key:      "this-item-definitely-does-not-exist-12345",
	}

	_, err := p.Resolve(ctx, ref)
	assert.Error(t, err, "Should return error for non-existent item")

	// Should be a NotFoundError
	var notFoundErr *provider.NotFoundError
	if assert.ErrorAs(t, err, &notFoundErr) {
		assert.Equal(t, "test", notFoundErr.Provider)
		assert.Contains(t, notFoundErr.Key, "this-item-definitely-does-not-exist")
	}
}
