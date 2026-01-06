package fakes

import (
	"github.com/systmms/dsops/internal/providers/contracts"
)

// FakeKeychainClient is a test double for contracts.KeychainClient
type FakeKeychainClient struct {
	// Secrets is a map of service -> account -> value
	Secrets map[string]map[string][]byte

	// Available controls whether the keychain reports as available
	Available bool

	// Headless controls whether the environment is reported as headless
	Headless bool

	// ValidateErr is returned by Validate() if set
	ValidateErr error

	// QueryErr is returned by Query() if set (overrides Secrets lookup)
	QueryErr error
}

// NewFakeKeychainClient creates a new fake keychain client with defaults
func NewFakeKeychainClient() *FakeKeychainClient {
	return &FakeKeychainClient{
		Secrets:   make(map[string]map[string][]byte),
		Available: true,
		Headless:  false,
	}
}

// SetSecret adds a secret to the fake keychain
func (f *FakeKeychainClient) SetSecret(service, account string, value []byte) {
	if f.Secrets == nil {
		f.Secrets = make(map[string]map[string][]byte)
	}
	if f.Secrets[service] == nil {
		f.Secrets[service] = make(map[string][]byte)
	}
	f.Secrets[service][account] = value
}

// Query retrieves a secret from the fake keychain
func (f *FakeKeychainClient) Query(service, account string) ([]byte, error) {
	if f.QueryErr != nil {
		return nil, f.QueryErr
	}

	if accounts, ok := f.Secrets[service]; ok {
		if value, ok := accounts[account]; ok {
			return value, nil
		}
	}
	return nil, ErrFakeKeychainItemNotFound
}

// Validate checks if the keychain is accessible
func (f *FakeKeychainClient) Validate() error {
	return f.ValidateErr
}

// IsAvailable returns whether keychain is available
func (f *FakeKeychainClient) IsAvailable() bool {
	return f.Available
}

// IsHeadless returns whether running in headless environment
func (f *FakeKeychainClient) IsHeadless() bool {
	return f.Headless
}

// ErrFakeKeychainItemNotFound is returned when a keychain item doesn't exist
var ErrFakeKeychainItemNotFound = &fakeKeychainError{code: "itemNotFound"}

// ErrFakeKeychainAccessDenied is returned when keychain access is denied
var ErrFakeKeychainAccessDenied = &fakeKeychainError{code: "accessDenied"}

type fakeKeychainError struct {
	code string
}

func (e *fakeKeychainError) Error() string {
	switch e.code {
	case "itemNotFound":
		return "keychain item not found"
	case "accessDenied":
		return "keychain access denied"
	default:
		return "keychain error: " + e.code
	}
}

// Ensure FakeKeychainClient implements contracts.KeychainClient
var _ contracts.KeychainClient = (*FakeKeychainClient)(nil)
