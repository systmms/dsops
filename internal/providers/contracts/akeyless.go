package contracts

import (
	"context"
	"time"
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
	Tags      []string
}

// AkeylessMetadata represents secret metadata
type AkeylessMetadata struct {
	Path             string
	ItemType         string
	Version          int
	CreationDate     time.Time
	LastModified     time.Time
	Tags             []string
	RotationInterval string
	LastRotationDate *time.Time
}

// AkeylessReference represents a parsed Akeyless secret reference
type AkeylessReference struct {
	Path    string // e.g., "/prod/database/password"
	Version *int   // nil for latest
}
