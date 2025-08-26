package providers_test

import (
	"context"
	"errors"
	"testing"

	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/pkg/provider"
)

func TestLiteralProviderContract(t *testing.T) {
	contract := provider.ContractTest{
		CreateProvider: func(t *testing.T) provider.Provider {
			values := map[string]string{
				"test-key":     "test-value",
				"another-key":  "another-value",
			}
			return providers.NewLiteralProvider("test-literal", values)
		},
		SetupTestSecret: func(t *testing.T, p provider.Provider) (string, func()) {
			// Literal provider already has values set up in CreateProvider
			return "test-key", func() {
				// No cleanup needed for literal provider
			}
		},
		// Literal provider doesn't need validation
		SkipValidation: false,
		// Literal provider supports basic metadata
		SkipMetadata: false,
	}

	provider.RunContractTests(t, contract)
}

// Unit tests for literal provider specific behavior
func TestLiteralProviderValues(t *testing.T) {
	values := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	
	p := providers.NewLiteralProvider("test", values)
	
	// Test that all values can be retrieved
	for key, expectedValue := range values {
		t.Run("Resolve_"+key, func(t *testing.T) {
			ref := provider.Reference{Key: key}
			secret, err := p.Resolve(context.TODO(), ref)
			
			if err != nil {
				t.Fatalf("Unexpected error resolving %s: %v", key, err)
			}
			
			if secret.Value != expectedValue {
				t.Errorf("Expected value %q, got %q", expectedValue, secret.Value)
			}
		})
	}
	
	// Test non-existent key
	t.Run("Resolve_NonExistent", func(t *testing.T) {
		ref := provider.Reference{Key: "non-existent"}
		_, err := p.Resolve(context.TODO(), ref)
		
		if err == nil {
			t.Error("Expected error for non-existent key, got nil")
		}
		
		// Should be a NotFoundError (could be value or pointer)
		var notFoundErr provider.NotFoundError
		var notFoundPtrErr *provider.NotFoundError
		if errors.As(err, &notFoundErr) {
			if notFoundErr.Key != "non-existent" {
				t.Errorf("Expected key 'non-existent' in error, got %q", notFoundErr.Key)
			}
		} else if errors.As(err, &notFoundPtrErr) {
			if notFoundPtrErr.Key != "non-existent" {
				t.Errorf("Expected key 'non-existent' in error, got %q", notFoundPtrErr.Key)
			}
		} else {
			t.Errorf("Expected NotFoundError, got %T: %v", err, err)
		}
	})
}