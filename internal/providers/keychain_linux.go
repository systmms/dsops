//go:build linux

package providers

import (
	"errors"
	"os"

	"github.com/zalando/go-keyring"

	"github.com/systmms/dsops/internal/providers/contracts"
)

// linuxKeychainClient implements KeychainClient for Linux (Secret Service)
type linuxKeychainClient struct{}

// newPlatformKeychainClient creates the platform-specific keychain client
func newPlatformKeychainClient() contracts.KeychainClient {
	return &linuxKeychainClient{}
}

// Query retrieves a secret from Linux Secret Service
func (c *linuxKeychainClient) Query(service, account string) ([]byte, error) {
	secret, err := keyring.Get(service, account)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return nil, ErrKeychainItemNotFound
		}
		return nil, err
	}
	return []byte(secret), nil
}

// Validate checks if Secret Service is accessible
func (c *linuxKeychainClient) Validate() error {
	// On Linux, we need a Secret Service implementation running
	// (gnome-keyring, KWallet, etc.)
	return nil
}

// IsAvailable returns true if Secret Service is available
func (c *linuxKeychainClient) IsAvailable() bool {
	// Check if DISPLAY is set (X11) or WAYLAND_DISPLAY is set
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	return true
}

// IsHeadless returns true if running in headless environment
func (c *linuxKeychainClient) IsHeadless() bool {
	// Check for SSH session
	if os.Getenv("SSH_TTY") != "" {
		return true
	}
	// Check if no display is available
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return true
	}
	// Check for CI environments
	if os.Getenv("CI") != "" {
		return true
	}
	return false
}

// Ensure linuxKeychainClient implements contracts.KeychainClient
var _ contracts.KeychainClient = (*linuxKeychainClient)(nil)
