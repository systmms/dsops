package contracts

import (
	"context"
	"time"
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

// InfisicalReference represents a parsed Infisical secret reference
type InfisicalReference struct {
	Path    string // e.g., "folder/SECRET_NAME"
	Name    string // e.g., "SECRET_NAME"
	Version *int   // nil for latest
}
