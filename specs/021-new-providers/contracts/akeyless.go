// Package contracts defines interfaces and types for the new providers.
// This file contains contracts for the Akeyless provider.
package contracts

import (
	"context"
	"time"

	"github.com/systmms/dsops/pkg/provider"
)

// AkeylessClient abstracts Akeyless SDK operations for testing
type AkeylessClient interface {
	// Authenticate obtains an access token
	Authenticate(ctx context.Context) (token string, expiresIn time.Duration, err error)

	// GetSecret retrieves a secret by path
	GetSecret(ctx context.Context, token, path string, version *int) (*AkeylessSecret, error)

	// DescribeItem gets metadata about a secret without retrieving value
	DescribeItem(ctx context.Context, token, path string) (*AkeylessMetadata, error)

	// ListItems lists secrets at a path (for doctor validation)
	ListItems(ctx context.Context, token, path string) ([]string, error)
}

// AkeylessSecret represents a secret from Akeyless
type AkeylessSecret struct {
	Path      string
	Value     string
	Version   int
	CreatedAt time.Time
	UpdatedAt time.Time
	Tags      map[string]string
}

// AkeylessMetadata represents secret metadata
type AkeylessMetadata struct {
	Path               string
	ItemType           string
	Version            int
	CreationDate       time.Time
	LastModified       time.Time
	Tags               map[string]string
	RotationInterval   string
	LastRotationDate   *time.Time
}

// AkeylessConfig holds configuration for the Akeyless provider
type AkeylessConfig struct {
	AccessID   string
	GatewayURL string
	Auth       AkeylessAuth
	Timeout    time.Duration
}

// AkeylessAuth defines authentication configuration
type AkeylessAuth struct {
	Method          string // api_key, aws_iam, azure_ad, gcp, oidc, saml
	AccessKey       string
	AzureADObjectID string
	GCPAudience     string
}

// NewAkeylessProviderFunc is the factory function signature
type NewAkeylessProviderFunc func(name string, config map[string]interface{}) (provider.Provider, error)

// AkeylessReference represents a parsed Akeyless secret reference
type AkeylessReference struct {
	Path    string // e.g., "/prod/database/password"
	Version *int   // nil for latest
}

// ParseAkeylessReference parses an Akeyless reference string
// Format: /path/to/secret[@vN]
func ParseAkeylessReference(key string) (*AkeylessReference, error) {
	// Implementation in akeyless.go
	return nil, nil
}

// MockAkeylessClient is a test double for AkeylessClient
type MockAkeylessClient struct {
	Token       string
	TokenTTL    time.Duration
	Secrets     map[string]*AkeylessSecret
	Metadata    map[string]*AkeylessMetadata
	AuthErr     error
	GetErr      error
	DescribeErr error
	ListErr     error
}

func (m *MockAkeylessClient) Authenticate(ctx context.Context) (string, time.Duration, error) {
	if m.AuthErr != nil {
		return "", 0, m.AuthErr
	}
	return m.Token, m.TokenTTL, nil
}

func (m *MockAkeylessClient) GetSecret(ctx context.Context, token, path string, version *int) (*AkeylessSecret, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if secret, ok := m.Secrets[path]; ok {
		return secret, nil
	}
	return nil, ErrAkeylessSecretNotFound
}

func (m *MockAkeylessClient) DescribeItem(ctx context.Context, token, path string) (*AkeylessMetadata, error) {
	if m.DescribeErr != nil {
		return nil, m.DescribeErr
	}
	if meta, ok := m.Metadata[path]; ok {
		return meta, nil
	}
	return nil, ErrAkeylessSecretNotFound
}

func (m *MockAkeylessClient) ListItems(ctx context.Context, token, path string) ([]string, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	paths := make([]string, 0, len(m.Secrets))
	for p := range m.Secrets {
		paths = append(paths, p)
	}
	return paths, nil
}

// Akeyless error types
var (
	ErrAkeylessSecretNotFound = &akeylessError{code: "itemNotFound", message: "secret not found"}
	ErrAkeylessUnauthorized   = &akeylessError{code: "unauthorized", message: "authentication failed"}
	ErrAkeylessPermission     = &akeylessError{code: "permissionDenied", message: "permission denied"}
	ErrAkeylessRateLimited    = &akeylessError{code: "rateLimited", message: "rate limit exceeded"}
)

type akeylessError struct {
	code    string
	message string
}

func (e *akeylessError) Error() string {
	return e.message
}

func (e *akeylessError) Code() string {
	return e.code
}

// IsAkeylessNotFound returns true if the error is a not found error
func IsAkeylessNotFound(err error) bool {
	if ae, ok := err.(*akeylessError); ok {
		return ae.code == "itemNotFound"
	}
	return false
}

// Ensure MockAkeylessClient implements AkeylessClient
var _ AkeylessClient = (*MockAkeylessClient)(nil)
