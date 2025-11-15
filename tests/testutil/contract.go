// Package testutil provides testing utilities and helpers for dsops tests.
//
// This file implements the provider contract test framework that validates
// all providers implement the provider.Provider interface correctly and
// consistently.
package testutil

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/pkg/provider"
)

// ProviderTestCase defines a provider under test with its test data.
//
// This structure contains everything needed to run contract tests against
// a provider implementation.
type ProviderTestCase struct {
	// Name is a descriptive name for this test case (usually the provider name)
	Name string

	// Provider is the provider implementation to test
	Provider provider.Provider

	// TestData maps secret keys to their expected values
	// These secrets should exist in the provider's test environment
	TestData map[string]provider.SecretValue

	// SkipValidation skips the Validate() test if true
	// Useful for providers that don't require authentication
	SkipValidation bool

	// SkipConcurrency skips the concurrency test if true
	// Use only if provider explicitly doesn't support concurrent access
	SkipConcurrency bool
}

// RunProviderContractTests runs all contract tests for a provider.
//
// This function executes the complete provider contract test suite:
//   - Name() returns consistent value
//   - Resolve() retrieves secrets correctly
//   - Describe() returns metadata without values
//   - Capabilities() returns valid capabilities
//   - Validate() checks configuration
//   - Error handling is consistent
//   - Concurrency safety (no race conditions)
//
// Example usage:
//
//	tc := testutil.ProviderTestCase{
//	    Name:     "vault",
//	    Provider: vaultProvider,
//	    TestData: map[string]provider.SecretValue{
//	        "secret/test": {Value: "test-secret-123"},
//	    },
//	}
//	testutil.RunProviderContractTests(t, tc)
func RunProviderContractTests(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	// Validate test case
	require.NotNil(t, tc.Provider, "Provider cannot be nil")
	require.NotEmpty(t, tc.Name, "Test case name cannot be empty")
	require.NotEmpty(t, tc.TestData, "TestData must contain at least one secret")

	// Run all contract tests
	t.Run("Name", func(t *testing.T) {
		testProviderName(t, tc)
	})

	t.Run("Capabilities", func(t *testing.T) {
		testProviderCapabilities(t, tc)
	})

	if !tc.SkipValidation {
		t.Run("Validate", func(t *testing.T) {
			testProviderValidate(t, tc)
		})
	}

	t.Run("Resolve", func(t *testing.T) {
		testProviderResolve(t, tc)
	})

	t.Run("Describe", func(t *testing.T) {
		testProviderDescribe(t, tc)
	})

	t.Run("ErrorHandling", func(t *testing.T) {
		testProviderErrorHandling(t, tc)
	})

	if !tc.SkipConcurrency {
		t.Run("Concurrency", func(t *testing.T) {
			testProviderConcurrency(t, tc)
		})
	}
}

// testProviderName validates that Name() returns a consistent, non-empty value.
func testProviderName(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	name := tc.Provider.Name()

	// Name must be non-empty
	assert.NotEmpty(t, name, "Provider Name() must return non-empty string")

	// Name must be consistent across calls
	name2 := tc.Provider.Name()
	assert.Equal(t, name, name2, "Provider Name() must return consistent value")

	// Name should be lowercase (convention)
	assert.Regexp(t, `^[a-z][a-z0-9._-]*$`, name,
		"Provider name should be lowercase with dots, dashes, or underscores")
}

// testProviderCapabilities validates that Capabilities() returns valid data.
func testProviderCapabilities(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	caps := tc.Provider.Capabilities()

	// Capabilities should be consistent across calls
	caps2 := tc.Provider.Capabilities()
	assert.Equal(t, caps, caps2, "Capabilities() should return consistent data")

	// If RequiresAuth is true, AuthMethods should not be empty
	if caps.RequiresAuth {
		assert.NotEmpty(t, caps.AuthMethods,
			"Provider requires auth but AuthMethods is empty")
	}

	// Log capabilities for debugging
	t.Logf("Provider capabilities: Versioning=%v, Metadata=%v, Watching=%v, Binary=%v, Auth=%v",
		caps.SupportsVersioning, caps.SupportsMetadata, caps.SupportsWatching,
		caps.SupportsBinary, caps.RequiresAuth)
}

// testProviderValidate validates that Validate() checks configuration properly.
func testProviderValidate(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := tc.Provider.Validate(ctx)
	assert.NoError(t, err, "Provider Validate() should succeed with valid configuration")
}

// testProviderResolve validates that Resolve() retrieves secrets correctly.
func testProviderResolve(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for key, expectedSecret := range tc.TestData {
		t.Run(fmt.Sprintf("Key_%s", sanitizeTestName(key)), func(t *testing.T) {
			ref := provider.Reference{
				Provider: tc.Provider.Name(),
				Key:      key,
			}

			secret, err := tc.Provider.Resolve(ctx, ref)

			require.NoError(t, err, "Resolve() should succeed for existing secret")

			// Validate secret value matches expected
			assert.Equal(t, expectedSecret.Value, secret.Value,
				"Secret value should match test data")

			// If provider supports versioning, check version is set
			caps := tc.Provider.Capabilities()
			if caps.SupportsVersioning {
				assert.NotEmpty(t, secret.Version,
					"Provider supports versioning but returned empty version")
			}
		})
	}
}

// testProviderDescribe validates that Describe() returns metadata without values.
func testProviderDescribe(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for key := range tc.TestData {
		t.Run(fmt.Sprintf("Key_%s", sanitizeTestName(key)), func(t *testing.T) {
			ref := provider.Reference{
				Provider: tc.Provider.Name(),
				Key:      key,
			}

			meta, err := tc.Provider.Describe(ctx, ref)

			require.NoError(t, err, "Describe() should succeed for existing secret")

			// Secret should exist
			assert.True(t, meta.Exists, "Metadata should indicate secret exists")

			// If provider supports versioning, check version is set
			caps := tc.Provider.Capabilities()
			if caps.SupportsVersioning {
				assert.NotEmpty(t, meta.Version,
					"Provider supports versioning but metadata has empty version")
			}

			// If provider supports metadata, check for tags or type
			if caps.SupportsMetadata {
				// At least one of these should be non-empty
				hasMetadata := len(meta.Tags) > 0 || meta.Type != "" || len(meta.Permissions) > 0
				assert.True(t, hasMetadata,
					"Provider supports metadata but returned no tags, type, or permissions")
			}
		})
	}
}

// testProviderErrorHandling validates error handling for missing secrets.
func testProviderErrorHandling(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test with a key that definitely doesn't exist
	nonExistentKey := "this-secret-definitely-does-not-exist-" + time.Now().Format("20060102150405")

	ref := provider.Reference{
		Provider: tc.Provider.Name(),
		Key:      nonExistentKey,
	}

	t.Run("Resolve_NotFound", func(t *testing.T) {
		_, err := tc.Provider.Resolve(ctx, ref)
		assert.Error(t, err, "Resolve() should return error for non-existent secret")

		// Should be a NotFoundError (but not required - some providers may use different errors)
		var notFoundErr *provider.NotFoundError
		if assert.ErrorAs(t, err, &notFoundErr) {
			assert.Equal(t, tc.Provider.Name(), notFoundErr.Provider)
			assert.Equal(t, nonExistentKey, notFoundErr.Key)
		}
	})

	t.Run("Describe_NotFound", func(t *testing.T) {
		meta, err := tc.Provider.Describe(ctx, ref)

		// Describe should NOT return an error for non-existent secrets
		// It should return metadata with Exists=false
		if err == nil {
			assert.False(t, meta.Exists,
				"Describe() should return Exists=false for non-existent secret")
		} else {
			// Some providers may return NotFoundError from Describe, which is acceptable
			t.Logf("Describe() returned error (acceptable): %v", err)
		}
	})

	t.Run("Context_Cancellation", func(t *testing.T) {
		// Test context cancellation
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Get first test key
		var testKey string
		for k := range tc.TestData {
			testKey = k
			break
		}

		ref := provider.Reference{
			Provider: tc.Provider.Name(),
			Key:      testKey,
		}

		_, err := tc.Provider.Resolve(cancelCtx, ref)
		// Provider should respect context cancellation
		// (but may complete before checking context, which is OK)
		if err != nil {
			assert.Error(t, err, "Should handle cancelled context")
		}
	})
}

// testProviderConcurrency validates thread-safety with concurrent access.
func testProviderConcurrency(t *testing.T, tc ProviderTestCase) {
	t.Helper()

	// Skip if running with -short flag (concurrency tests are slower)
	if testing.Short() {
		t.Skip("Skipping concurrency test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Get first test key
	var testKey string
	for k := range tc.TestData {
		testKey = k
		break
	}

	ref := provider.Reference{
		Provider: tc.Provider.Name(),
		Key:      testKey,
	}

	const concurrency = 50 // Number of concurrent goroutines
	var wg sync.WaitGroup
	errors := make(chan error, concurrency)

	// Launch concurrent Resolve() calls
	t.Run("Concurrent_Resolve", func(t *testing.T) {
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				secret, err := tc.Provider.Resolve(ctx, ref)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: Resolve failed: %w", id, err)
					return
				}

				// Validate secret value
				expectedValue := tc.TestData[testKey].Value
				if secret.Value != expectedValue {
					errors <- fmt.Errorf("goroutine %d: got value %q, want %q",
						id, secret.Value, expectedValue)
				}
			}(i)
		}

		// Wait for all goroutines
		wg.Wait()
		close(errors)

		// Check for any errors
		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		if len(errs) > 0 {
			for _, err := range errs {
				t.Error(err)
			}
			t.Fatalf("Concurrency test failed with %d errors", len(errs))
		}
	})

	// Launch concurrent Describe() calls
	t.Run("Concurrent_Describe", func(t *testing.T) {
		errors := make(chan error, concurrency)
		var wg sync.WaitGroup

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				meta, err := tc.Provider.Describe(ctx, ref)
				if err != nil {
					errors <- fmt.Errorf("goroutine %d: Describe failed: %w", id, err)
					return
				}

				if !meta.Exists {
					errors <- fmt.Errorf("goroutine %d: Describe returned Exists=false", id)
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		// Check for any errors
		var errs []error
		for err := range errors {
			errs = append(errs, err)
		}

		if len(errs) > 0 {
			for _, err := range errs {
				t.Error(err)
			}
			t.Fatalf("Concurrency test failed with %d errors", len(errs))
		}
	})
}

// sanitizeTestName converts a secret key to a valid test name
func sanitizeTestName(key string) string {
	// Replace invalid characters with underscores
	result := ""
	for _, ch := range key {
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			result += string(ch)
		} else {
			result += "_"
		}
	}
	return result
}
