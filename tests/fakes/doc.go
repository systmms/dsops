// Package fakes provides test doubles for dsops provider interfaces.
//
// This package contains fake implementations of external client interfaces
// that allow unit testing of providers without real service dependencies.
// Fakes are manually implemented (not generated) to provide precise control
// over test behavior.
//
// Usage:
//
//	fake := &fakes.FakeKeychainClient{
//	    Secrets: map[string]map[string][]byte{
//	        "myapp": {"api-key": []byte("secret123")},
//	    },
//	    Available: true,
//	}
//	provider := keychain.NewWithClient(fake)
//	// Test provider methods...
package fakes
