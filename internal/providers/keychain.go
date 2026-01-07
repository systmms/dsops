package providers

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"time"

	"github.com/systmms/dsops/internal/providers/contracts"
	"github.com/systmms/dsops/pkg/provider"
)

// KeychainProvider implements the provider interface for OS keychains
// (macOS Keychain and Linux Secret Service)
type KeychainProvider struct {
	name          string
	servicePrefix string
	accessGroup   string
	client        contracts.KeychainClient
}

// NewKeychainProvider creates a new keychain provider
func NewKeychainProvider(name string, config map[string]interface{}) *KeychainProvider {
	kc := &KeychainProvider{
		name:   name,
		client: newPlatformKeychainClient(),
	}

	if config != nil {
		if prefix, ok := config["service_prefix"].(string); ok {
			kc.servicePrefix = prefix
		}
		if accessGroup, ok := config["access_group"].(string); ok {
			kc.accessGroup = accessGroup
		}
	}

	return kc
}

// NewKeychainProviderWithClient creates a keychain provider with a custom client.
// This is primarily for testing, allowing the keychain client to be mocked.
func NewKeychainProviderWithClient(name string, config map[string]interface{}, client contracts.KeychainClient) *KeychainProvider {
	kc := &KeychainProvider{
		name:   name,
		client: client,
	}

	if config != nil {
		if prefix, ok := config["service_prefix"].(string); ok {
			kc.servicePrefix = prefix
		}
		if accessGroup, ok := config["access_group"].(string); ok {
			kc.accessGroup = accessGroup
		}
	}

	return kc
}

// Name returns the provider name
func (kc *KeychainProvider) Name() string {
	return kc.name
}

// Platform returns the current platform (darwin, linux, or unsupported)
func (kc *KeychainProvider) Platform() string {
	return runtime.GOOS
}

// Resolve retrieves a secret from the OS keychain
func (kc *KeychainProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	// Parse the key format: service/account
	kcRef, err := ParseKeychainReference(ref.Key)
	if err != nil {
		return provider.SecretValue{}, fmt.Errorf("invalid keychain reference '%s': %w", ref.Key, err)
	}

	// Apply service prefix if configured
	service := kc.applyServicePrefix(kcRef.Service)

	// Query the keychain
	value, err := kc.client.Query(service, kcRef.Account)
	if err != nil {
		if isKeychainNotFoundError(err) {
			return provider.SecretValue{}, provider.NotFoundError{
				Provider: kc.name,
				Key:      ref.Key,
			}
		}
		if isKeychainAccessDeniedError(err) {
			return provider.SecretValue{}, &KeychainError{
				Op:      "query",
				Service: service,
				Account: kcRef.Account,
				Err:     ErrKeychainAccessDenied,
			}
		}
		return provider.SecretValue{}, &KeychainError{
			Op:      "query",
			Service: service,
			Account: kcRef.Account,
			Err:     err,
		}
	}

	return provider.SecretValue{
		Value:     string(value),
		Version:   "", // Keychain doesn't support versioning
		UpdatedAt: time.Time{},
		Metadata: map[string]string{
			"provider": kc.name,
			"service":  service,
			"account":  kcRef.Account,
		},
	}, nil
}

// Describe returns metadata about a keychain item without retrieving its value
func (kc *KeychainProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	kcRef, err := ParseKeychainReference(ref.Key)
	if err != nil {
		return provider.Metadata{}, fmt.Errorf("invalid keychain reference '%s': %w", ref.Key, err)
	}

	service := kc.applyServicePrefix(kcRef.Service)

	// Try to query the item to check existence
	_, err = kc.client.Query(service, kcRef.Account)
	if err != nil {
		if isKeychainNotFoundError(err) {
			return provider.Metadata{Exists: false}, nil
		}
		return provider.Metadata{}, fmt.Errorf("failed to describe keychain item: %w", err)
	}

	return provider.Metadata{
		Exists:  true,
		Version: "", // Keychain doesn't support versioning
		Type:    "password",
		Tags: map[string]string{
			"service": service,
			"account": kcRef.Account,
		},
	}, nil
}

// Capabilities returns the provider's supported features
func (kc *KeychainProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     true,
		RequiresAuth:       false, // Uses OS-level authentication
		AuthMethods:        []string{"os"},
	}
}

// Validate checks if the keychain is accessible
func (kc *KeychainProvider) Validate(ctx context.Context) error {
	if !kc.client.IsAvailable() {
		return fmt.Errorf("keychain not supported on this platform")
	}

	if kc.client.IsHeadless() {
		return fmt.Errorf("keychain requires GUI environment (headless environment detected). Consider using a different secret provider for CI/CD environments")
	}

	if err := kc.client.Validate(); err != nil {
		return fmt.Errorf("keychain validation failed: %w", err)
	}

	return nil
}

// applyServicePrefix combines the configured prefix with the service name
func (kc *KeychainProvider) applyServicePrefix(service string) string {
	if kc.servicePrefix == "" {
		return service
	}
	// If service already starts with prefix, don't add it again
	if strings.HasPrefix(service, kc.servicePrefix) {
		return service
	}
	return kc.servicePrefix + "." + service
}

// KeychainReference represents a parsed keychain secret reference
type KeychainReference struct {
	Service string
	Account string
}

// ParseKeychainReference parses a keychain reference string
// Format: service/account
func ParseKeychainReference(key string) (*KeychainReference, error) {
	parts := strings.SplitN(key, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("keychain reference must be service/account format, got: %s", key)
	}

	service := strings.TrimSpace(parts[0])
	account := strings.TrimSpace(parts[1])

	if service == "" {
		return nil, fmt.Errorf("keychain reference service cannot be empty")
	}
	if account == "" {
		return nil, fmt.Errorf("keychain reference account cannot be empty")
	}

	return &KeychainReference{
		Service: service,
		Account: account,
	}, nil
}

// isKeychainNotFoundError checks if an error indicates item not found
func isKeychainNotFoundError(err error) bool {
	if errors.Is(err, ErrKeychainItemNotFound) {
		return true
	}
	// Check for common "not found" patterns in error messages
	errStr := err.Error()
	return strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "itemNotFound")
}

// isKeychainAccessDeniedError checks if an error indicates access was denied
func isKeychainAccessDeniedError(err error) bool {
	if errors.Is(err, ErrKeychainAccessDenied) {
		return true
	}
	errStr := err.Error()
	return strings.Contains(errStr, "access denied") ||
		strings.Contains(errStr, "accessDenied")
}
