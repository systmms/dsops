// Package contracts defines interfaces for provider client abstractions.
// These interfaces enable dependency injection for testing.
package contracts

// KeychainClient abstracts OS keychain operations for testing
type KeychainClient interface {
	// Query retrieves a secret from the keychain
	Query(service, account string) ([]byte, error)

	// Validate checks if the keychain is accessible
	Validate() error

	// IsAvailable returns true if keychain is available on this platform
	IsAvailable() bool

	// IsHeadless returns true if running in headless environment
	IsHeadless() bool
}

// KeychainReference represents a parsed keychain secret reference
type KeychainReference struct {
	Service string
	Account string
}
