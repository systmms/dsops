//go:build darwin

package providers

import (
	"errors"
	"os"

	"github.com/zalando/go-keyring"

	"github.com/systmms/dsops/internal/providers/contracts"
)

// darwinKeychainClient implements KeychainClient for macOS
type darwinKeychainClient struct{}

// newPlatformKeychainClient creates the platform-specific keychain client
func newPlatformKeychainClient() contracts.KeychainClient {
	return &darwinKeychainClient{}
}

// Query retrieves a secret from the macOS keychain
func (c *darwinKeychainClient) Query(service, account string) ([]byte, error) {
	secret, err := keyring.Get(service, account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrKeychainItemNotFound
		}
		// Check for common access denied patterns
		if isAccessDenied(err) {
			return nil, ErrKeychainAccessDenied
		}
		return nil, err
	}
	return []byte(secret), nil
}

// Validate checks if the keychain is accessible
func (c *darwinKeychainClient) Validate() error {
	// On macOS, keychain is always available if we're running on the platform
	return nil
}

// IsAvailable returns true since we're on macOS
func (c *darwinKeychainClient) IsAvailable() bool {
	return true
}

// IsHeadless returns true if running in headless environment
func (c *darwinKeychainClient) IsHeadless() bool {
	// Check for SSH session
	if os.Getenv("SSH_TTY") != "" {
		return true
	}
	// Check for CI environments
	if os.Getenv("CI") != "" {
		return true
	}
	return false
}

// isAccessDenied checks if an error indicates access was denied
func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return contains(errStr, "access denied") ||
		contains(errStr, "user denied") ||
		contains(errStr, "canceled")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Ensure darwinKeychainClient implements contracts.KeychainClient
var _ contracts.KeychainClient = (*darwinKeychainClient)(nil)
