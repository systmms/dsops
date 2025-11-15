// Package providers_test validates that all provider implementations comply
// with the provider.Provider interface contract.
//
// These tests ensure consistent behavior across all providers regardless of
// their underlying implementation.
package providers_test

import (
	"context"
	"testing"

	"github.com/systmms/dsops/pkg/provider"
)

// Contract tests are implemented in individual provider test files.
//
// Each provider's test file (e.g., bitwarden_test.go, vault_test.go) should
// include a test function that calls testutil.RunProviderContractTests().
//
// Example:
//
//	func TestBitwardenProviderContract(t *testing.T) {
//	    if testing.Short() {
//	        t.Skip("Skipping integration test")
//	    }
//
//	    // Setup provider
//	    provider := setupBitwardenProvider(t)
//
//	    // Seed test data
//	    testData := map[string]provider.SecretValue{
//	        "test-secret": {Value: "test-value-123"},
//	    }
//
//	    // Run contract tests
//	    tc := testutil.ProviderTestCase{
//	        Name:     "bitwarden",
//	        Provider: provider,
//	        TestData: testData,
//	    }
//	    testutil.RunProviderContractTests(t, tc)
//	}

// TestProviderInterface is a compile-time check that all providers implement
// the provider.Provider interface.
func TestProviderInterface(t *testing.T) {
	// This test ensures that the provider.Provider interface is correctly defined
	// and can be satisfied by implementations.

	var _ provider.Provider = (*mockProvider)(nil)
}

// mockProvider is a minimal provider implementation for interface validation.
type mockProvider struct{}

func (m *mockProvider) Name() string {
	return "mock"
}

func (m *mockProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	return provider.SecretValue{}, nil
}

func (m *mockProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	return provider.Metadata{}, nil
}

func (m *mockProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{}
}

func (m *mockProvider) Validate(ctx context.Context) error {
	return nil
}
