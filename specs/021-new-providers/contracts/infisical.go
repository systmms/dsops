// Package contracts defines interfaces and types for the new providers.
// This file contains contracts for the Infisical provider.
package contracts

import (
	"context"
	"time"

	"github.com/systmms/dsops/pkg/provider"
)

// InfisicalClient abstracts Infisical API operations for testing
type InfisicalClient interface {
	// Authenticate obtains an access token
	Authenticate(ctx context.Context) (token string, expiresIn time.Duration, err error)

	// GetSecret retrieves a single secret by name
	GetSecret(ctx context.Context, token, secretName string, version *int) (*InfisicalSecret, error)

	// ListSecrets lists all secrets (for doctor validation)
	ListSecrets(ctx context.Context, token string) ([]string, error)
}

// InfisicalSecret represents a secret from Infisical
type InfisicalSecret struct {
	SecretKey     string
	SecretValue   string
	Version       int
	Type          string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	SecretComment string
	Tags          []string
}

// InfisicalConfig holds configuration for the Infisical provider
type InfisicalConfig struct {
	Host               string
	ProjectID          string
	Environment        string
	Auth               InfisicalAuth
	Timeout            time.Duration
	CACert             string
	InsecureSkipVerify bool
}

// InfisicalAuth defines authentication configuration
type InfisicalAuth struct {
	Method       string // machine_identity, service_token, api_key
	ClientID     string
	ClientSecret string
	ServiceToken string
	APIKey       string
}

// NewInfisicalProviderFunc is the factory function signature
type NewInfisicalProviderFunc func(name string, config map[string]interface{}) (provider.Provider, error)

// InfisicalReference represents a parsed Infisical secret reference
type InfisicalReference struct {
	Path    string // e.g., "folder/SECRET_NAME"
	Name    string // e.g., "SECRET_NAME"
	Version *int   // nil for latest
}

// ParseInfisicalReference parses an Infisical reference string
// Format: path/to/SECRET_NAME[@vN]
func ParseInfisicalReference(key string) (*InfisicalReference, error) {
	// Implementation in infisical.go
	return nil, nil
}

// MockInfisicalClient is a test double for InfisicalClient
type MockInfisicalClient struct {
	Token       string
	TokenTTL    time.Duration
	Secrets     map[string]*InfisicalSecret
	AuthErr     error
	GetErr      error
	ListErr     error
}

func (m *MockInfisicalClient) Authenticate(ctx context.Context) (string, time.Duration, error) {
	if m.AuthErr != nil {
		return "", 0, m.AuthErr
	}
	return m.Token, m.TokenTTL, nil
}

func (m *MockInfisicalClient) GetSecret(ctx context.Context, token, secretName string, version *int) (*InfisicalSecret, error) {
	if m.GetErr != nil {
		return nil, m.GetErr
	}
	if secret, ok := m.Secrets[secretName]; ok {
		return secret, nil
	}
	return nil, ErrInfisicalSecretNotFound
}

func (m *MockInfisicalClient) ListSecrets(ctx context.Context, token string) ([]string, error) {
	if m.ListErr != nil {
		return nil, m.ListErr
	}
	names := make([]string, 0, len(m.Secrets))
	for name := range m.Secrets {
		names = append(names, name)
	}
	return names, nil
}

// Infisical error types
var (
	ErrInfisicalSecretNotFound = &infisicalError{code: 404, message: "secret not found"}
	ErrInfisicalUnauthorized   = &infisicalError{code: 401, message: "unauthorized"}
	ErrInfisicalForbidden      = &infisicalError{code: 403, message: "forbidden"}
	ErrInfisicalRateLimited    = &infisicalError{code: 429, message: "rate limited"}
)

type infisicalError struct {
	code    int
	message string
}

func (e *infisicalError) Error() string {
	return e.message
}

func (e *infisicalError) StatusCode() int {
	return e.code
}

// IsNotFound returns true if the error is a not found error
func IsInfisicalNotFound(err error) bool {
	if ie, ok := err.(*infisicalError); ok {
		return ie.code == 404
	}
	return false
}

// Ensure MockInfisicalClient implements InfisicalClient
var _ InfisicalClient = (*MockInfisicalClient)(nil)
