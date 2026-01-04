package fakes

import (
	"context"
	"time"

	"github.com/systmms/dsops/internal/providers/contracts"
)

// FakeInfisicalClient is a test double for contracts.InfisicalClient
type FakeInfisicalClient struct {
	// Token is the token returned by Authenticate
	Token string

	// TokenTTL is the TTL returned by Authenticate
	TokenTTL time.Duration

	// Secrets is a map of secret name to secret data
	Secrets map[string]*contracts.InfisicalSecret

	// AuthErr is returned by Authenticate if set
	AuthErr error

	// GetErr is returned by GetSecret if set (overrides Secrets lookup)
	GetErr error

	// ListErr is returned by ListSecrets if set
	ListErr error

	// AuthCallCount tracks how many times Authenticate was called
	AuthCallCount int

	// GetCallCount tracks how many times GetSecret was called
	GetCallCount int
}

// NewFakeInfisicalClient creates a new fake Infisical client with defaults
func NewFakeInfisicalClient() *FakeInfisicalClient {
	return &FakeInfisicalClient{
		Token:    "fake-token",
		TokenTTL: 30 * time.Second,
		Secrets:  make(map[string]*contracts.InfisicalSecret),
	}
}

// SetSecret adds a secret to the fake Infisical
func (f *FakeInfisicalClient) SetSecret(name, value string) {
	if f.Secrets == nil {
		f.Secrets = make(map[string]*contracts.InfisicalSecret)
	}
	f.Secrets[name] = &contracts.InfisicalSecret{
		SecretKey:   name,
		SecretValue: value,
		Version:     1,
		UpdatedAt:   time.Now(),
	}
}

// Authenticate obtains an access token
func (f *FakeInfisicalClient) Authenticate(ctx context.Context) (string, time.Duration, error) {
	f.AuthCallCount++
	if f.AuthErr != nil {
		return "", 0, f.AuthErr
	}
	return f.Token, f.TokenTTL, nil
}

// GetSecret retrieves a single secret by name
func (f *FakeInfisicalClient) GetSecret(ctx context.Context, token, secretName string, version *int) (*contracts.InfisicalSecret, error) {
	f.GetCallCount++
	if f.GetErr != nil {
		return nil, f.GetErr
	}

	if secret, ok := f.Secrets[secretName]; ok {
		return secret, nil
	}
	return nil, ErrFakeInfisicalSecretNotFound
}

// ListSecrets lists all secrets
func (f *FakeInfisicalClient) ListSecrets(ctx context.Context, token string) ([]string, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}

	names := make([]string, 0, len(f.Secrets))
	for name := range f.Secrets {
		names = append(names, name)
	}
	return names, nil
}

// ErrFakeInfisicalSecretNotFound is returned when a secret doesn't exist
var ErrFakeInfisicalSecretNotFound = &fakeInfisicalError{code: 404, message: "secret not found"}

// ErrFakeInfisicalUnauthorized is returned for auth failures
var ErrFakeInfisicalUnauthorized = &fakeInfisicalError{code: 401, message: "unauthorized"}

type fakeInfisicalError struct {
	code    int
	message string
}

func (e *fakeInfisicalError) Error() string {
	return e.message
}

func (e *fakeInfisicalError) StatusCode() int {
	return e.code
}

// Ensure FakeInfisicalClient implements contracts.InfisicalClient
var _ contracts.InfisicalClient = (*FakeInfisicalClient)(nil)
