package fakes

import (
	"context"
	"time"

	"github.com/systmms/dsops/internal/providers/contracts"
)

// FakeAkeylessClient is a test double for contracts.AkeylessClient
type FakeAkeylessClient struct {
	// Token is the token returned by Authenticate
	Token string

	// TokenTTL is the TTL returned by Authenticate
	TokenTTL time.Duration

	// Secrets is a map of path to secret data
	Secrets map[string]*contracts.AkeylessSecret

	// Metadata is a map of path to metadata
	Metadata map[string]*contracts.AkeylessMetadata

	// AuthErr is returned by Authenticate if set
	AuthErr error

	// GetErr is returned by GetSecret if set (overrides Secrets lookup)
	GetErr error

	// DescribeErr is returned by DescribeItem if set
	DescribeErr error

	// ListErr is returned by ListItems if set
	ListErr error

	// AuthCallCount tracks how many times Authenticate was called
	AuthCallCount int

	// GetCallCount tracks how many times GetSecret was called
	GetCallCount int
}

// NewFakeAkeylessClient creates a new fake Akeyless client with defaults
func NewFakeAkeylessClient() *FakeAkeylessClient {
	return &FakeAkeylessClient{
		Token:    "fake-akeyless-token",
		TokenTTL: 30 * time.Second,
		Secrets:  make(map[string]*contracts.AkeylessSecret),
		Metadata: make(map[string]*contracts.AkeylessMetadata),
	}
}

// SetSecret adds a secret to the fake Akeyless
func (f *FakeAkeylessClient) SetSecret(path, value string) {
	if f.Secrets == nil {
		f.Secrets = make(map[string]*contracts.AkeylessSecret)
	}
	now := time.Now()
	f.Secrets[path] = &contracts.AkeylessSecret{
		Path:      path,
		Value:     value,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if f.Metadata == nil {
		f.Metadata = make(map[string]*contracts.AkeylessMetadata)
	}
	f.Metadata[path] = &contracts.AkeylessMetadata{
		Path:         path,
		ItemType:     "static_secret",
		Version:      1,
		CreationDate: now,
		LastModified: now,
	}
}

// Authenticate obtains an access token
func (f *FakeAkeylessClient) Authenticate(ctx context.Context) (string, time.Duration, error) {
	f.AuthCallCount++
	if f.AuthErr != nil {
		return "", 0, f.AuthErr
	}
	return f.Token, f.TokenTTL, nil
}

// GetSecret retrieves a secret by path
func (f *FakeAkeylessClient) GetSecret(ctx context.Context, token, path string, version *int) (*contracts.AkeylessSecret, error) {
	f.GetCallCount++
	if f.GetErr != nil {
		return nil, f.GetErr
	}

	if secret, ok := f.Secrets[path]; ok {
		return secret, nil
	}
	return nil, ErrFakeAkeylessSecretNotFound
}

// DescribeItem gets metadata about a secret
func (f *FakeAkeylessClient) DescribeItem(ctx context.Context, token, path string) (*contracts.AkeylessMetadata, error) {
	if f.DescribeErr != nil {
		return nil, f.DescribeErr
	}

	if meta, ok := f.Metadata[path]; ok {
		return meta, nil
	}
	return nil, ErrFakeAkeylessSecretNotFound
}

// ListItems lists secrets at a path
func (f *FakeAkeylessClient) ListItems(ctx context.Context, token, path string) ([]string, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}

	paths := make([]string, 0, len(f.Secrets))
	for p := range f.Secrets {
		paths = append(paths, p)
	}
	return paths, nil
}

// ErrFakeAkeylessSecretNotFound is returned when a secret doesn't exist
var ErrFakeAkeylessSecretNotFound = &fakeAkeylessError{code: "itemNotFound", message: "secret not found"}

// ErrFakeAkeylessUnauthorized is returned for auth failures
var ErrFakeAkeylessUnauthorized = &fakeAkeylessError{code: "unauthorized", message: "authentication failed"}

type fakeAkeylessError struct {
	code    string
	message string
}

func (e *fakeAkeylessError) Error() string {
	return e.message
}

func (e *fakeAkeylessError) Code() string {
	return e.code
}

// Ensure FakeAkeylessClient implements contracts.AkeylessClient
var _ contracts.AkeylessClient = (*FakeAkeylessClient)(nil)
