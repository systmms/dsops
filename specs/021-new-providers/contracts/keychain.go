// Package contracts defines interfaces and types for the new providers.
// This file contains contracts for the OS Keychain provider.
package contracts

import (
	"github.com/systmms/dsops/pkg/provider"
)

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

// KeychainProvider implements provider.Provider for OS keychains
type KeychainProvider interface {
	provider.Provider

	// Platform returns the current platform (darwin, linux, unsupported)
	Platform() string
}

// KeychainConfig holds configuration for the keychain provider
type KeychainConfig struct {
	// ServicePrefix is prepended to service names
	ServicePrefix string

	// AccessGroup is macOS-specific keychain access group
	AccessGroup string
}

// NewKeychainProviderFunc is the factory function signature
type NewKeychainProviderFunc func(name string, config map[string]interface{}) (provider.Provider, error)

// KeychainReference represents a parsed keychain secret reference
type KeychainReference struct {
	Service string
	Account string
}

// ParseKeychainReference parses a keychain reference string
// Format: service/account
func ParseKeychainReference(key string) (*KeychainReference, error) {
	// Implementation in keychain.go
	return nil, nil
}

// MockKeychainClient is a test double for KeychainClient
type MockKeychainClient struct {
	Secrets     map[string]map[string][]byte // service -> account -> value
	Available   bool
	Headless    bool
	ValidateErr error
}

func (m *MockKeychainClient) Query(service, account string) ([]byte, error) {
	if accounts, ok := m.Secrets[service]; ok {
		if value, ok := accounts[account]; ok {
			return value, nil
		}
	}
	return nil, ErrKeychainItemNotFound
}

func (m *MockKeychainClient) Validate() error {
	return m.ValidateErr
}

func (m *MockKeychainClient) IsAvailable() bool {
	return m.Available
}

func (m *MockKeychainClient) IsHeadless() bool {
	return m.Headless
}

// ErrKeychainItemNotFound is returned when a keychain item doesn't exist
var ErrKeychainItemNotFound = &keychainError{code: "itemNotFound"}

// ErrKeychainAccessDenied is returned when keychain access is denied
var ErrKeychainAccessDenied = &keychainError{code: "accessDenied"}

// ErrKeychainUnsupportedPlatform is returned on unsupported platforms
var ErrKeychainUnsupportedPlatform = &keychainError{code: "unsupportedPlatform"}

// ErrKeychainHeadless is returned in headless environments
var ErrKeychainHeadless = &keychainError{code: "headlessEnvironment"}

type keychainError struct {
	code string
}

func (e *keychainError) Error() string {
	switch e.code {
	case "itemNotFound":
		return "keychain item not found"
	case "accessDenied":
		return "keychain access denied"
	case "unsupportedPlatform":
		return "keychain not supported on this platform"
	case "headlessEnvironment":
		return "keychain requires GUI environment for authentication"
	default:
		return "keychain error: " + e.code
	}
}

// Ensure MockKeychainClient implements KeychainClient
var _ KeychainClient = (*MockKeychainClient)(nil)
