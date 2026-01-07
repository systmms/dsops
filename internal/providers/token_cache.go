package providers

import (
	"sync"
	"time"
)

// TokenCache stores authentication tokens in memory for per-process caching.
// This implementation is thread-safe and supports automatic expiration.
// Tokens are never persisted to disk per FR-017.
type TokenCache struct {
	mu        sync.RWMutex
	token     string
	expiresAt time.Time
}

// NewTokenCache creates a new empty token cache
func NewTokenCache() *TokenCache {
	return &TokenCache{}
}

// Get retrieves the cached token if it exists and is not expired.
// Returns the token and true if valid, empty string and false otherwise.
func (c *TokenCache) Get() (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.token == "" {
		return "", false
	}

	if time.Now().After(c.expiresAt) {
		return "", false
	}

	return c.token, true
}

// Set stores a token with the specified TTL.
// A small buffer (5 seconds) is subtracted from TTL to ensure
// tokens are refreshed before actual expiration.
func (c *TokenCache) Set(token string, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.token = token
	// Subtract a small buffer to refresh before actual expiration
	buffer := 5 * time.Second
	if ttl > buffer {
		ttl -= buffer
	}
	c.expiresAt = time.Now().Add(ttl)
}

// Clear removes the cached token
func (c *TokenCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.token = ""
	c.expiresAt = time.Time{}
}

// IsExpired returns true if the token is expired or not set
func (c *TokenCache) IsExpired() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.token == "" {
		return true
	}

	return time.Now().After(c.expiresAt)
}

// ExpiresAt returns the expiration time of the current token.
// Returns zero time if no token is cached.
func (c *TokenCache) ExpiresAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.expiresAt
}

// TTL returns the remaining time until the token expires.
// Returns 0 if the token is expired or not set.
func (c *TokenCache) TTL() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.token == "" {
		return 0
	}

	remaining := time.Until(c.expiresAt)
	if remaining < 0 {
		return 0
	}

	return remaining
}
