package providers

import (
	"fmt"

	"github.com/systmms/dsops/pkg/provider"
)

// KeychainError wraps OS keychain errors with context
type KeychainError struct {
	Op      string // Operation: "query", "validate", "access"
	Service string
	Account string
	Err     error
}

func (e *KeychainError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("keychain %s error for %s/%s: %v", e.Op, e.Service, e.Account, e.Err)
	}
	return fmt.Sprintf("keychain %s error for %s/%s", e.Op, e.Service, e.Account)
}

func (e *KeychainError) Unwrap() error {
	return e.Err
}

// Keychain sentinel errors
var (
	ErrKeychainItemNotFound       = fmt.Errorf("keychain item not found")
	ErrKeychainAccessDenied       = fmt.Errorf("keychain access denied")
	ErrKeychainUnsupportedPlatform = fmt.Errorf("keychain not supported on this platform")
	ErrKeychainHeadless           = fmt.Errorf("keychain requires GUI environment for authentication")
	ErrKeychainLocked             = fmt.Errorf("keychain is locked")
)

// InfisicalError wraps Infisical API errors with context
type InfisicalError struct {
	Op         string // Operation: "auth", "fetch", "list"
	StatusCode int
	Message    string
	Err        error
}

func (e *InfisicalError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("infisical %s error (status %d): %s", e.Op, e.StatusCode, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("infisical %s error: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("infisical %s error: %s", e.Op, e.Message)
}

func (e *InfisicalError) Unwrap() error {
	return e.Err
}

// IsInfisicalNotFound returns true if the error is a not found error
func IsInfisicalNotFound(err error) bool {
	if ie, ok := err.(*InfisicalError); ok {
		return ie.StatusCode == 404
	}
	return false
}

// Infisical sentinel errors
var (
	ErrInfisicalSecretNotFound = fmt.Errorf("infisical secret not found")
	ErrInfisicalUnauthorized   = fmt.Errorf("infisical unauthorized")
	ErrInfisicalForbidden      = fmt.Errorf("infisical forbidden")
	ErrInfisicalRateLimited    = fmt.Errorf("infisical rate limited")
)

// AkeylessError wraps Akeyless SDK errors with context
type AkeylessError struct {
	Op      string // Operation: "auth", "fetch", "list", "describe"
	Path    string
	Message string
	Err     error
}

func (e *AkeylessError) Error() string {
	if e.Path != "" {
		return fmt.Sprintf("akeyless %s error for %s: %s", e.Op, e.Path, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("akeyless %s error: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("akeyless %s error: %s", e.Op, e.Message)
}

func (e *AkeylessError) Unwrap() error {
	return e.Err
}

// IsAkeylessNotFound returns true if the error is a not found error
func IsAkeylessNotFound(err error) bool {
	if ae, ok := err.(*AkeylessError); ok {
		return ae.Message == "secret not found" || ae.Message == "item not found"
	}
	return false
}

// Akeyless sentinel errors
var (
	ErrAkeylessSecretNotFound = fmt.Errorf("akeyless secret not found")
	ErrAkeylessUnauthorized   = fmt.Errorf("akeyless unauthorized")
	ErrAkeylessPermission     = fmt.Errorf("akeyless permission denied")
	ErrAkeylessRateLimited    = fmt.Errorf("akeyless rate limited")
)

// ToNotFoundError converts provider-specific errors to the standard NotFoundError
func ToNotFoundError(providerName, key string, err error) provider.NotFoundError {
	return provider.NotFoundError{
		Provider: providerName,
		Key:      key,
	}
}

// ToAuthError converts provider-specific errors to the standard AuthError
func ToAuthError(providerName string, err error) provider.AuthError {
	return provider.AuthError{
		Provider: providerName,
		Message:  err.Error(),
	}
}
