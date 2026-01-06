//go:build !darwin && !linux

package providers

import (
	"github.com/systmms/dsops/internal/providers/contracts"
)

// unsupportedKeychainClient is a stub for unsupported platforms
type unsupportedKeychainClient struct{}

// newPlatformKeychainClient creates a stub client for unsupported platforms
func newPlatformKeychainClient() contracts.KeychainClient {
	return &unsupportedKeychainClient{}
}

// Query returns an error on unsupported platforms
func (c *unsupportedKeychainClient) Query(service, account string) ([]byte, error) {
	return nil, ErrKeychainUnsupportedPlatform
}

// Validate returns an error on unsupported platforms
func (c *unsupportedKeychainClient) Validate() error {
	return ErrKeychainUnsupportedPlatform
}

// IsAvailable returns false on unsupported platforms
func (c *unsupportedKeychainClient) IsAvailable() bool {
	return false
}

// IsHeadless returns false (irrelevant on unsupported platforms)
func (c *unsupportedKeychainClient) IsHeadless() bool {
	return false
}

// Ensure unsupportedKeychainClient implements contracts.KeychainClient
var _ contracts.KeychainClient = (*unsupportedKeychainClient)(nil)
