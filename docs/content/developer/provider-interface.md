---
title: "Provider Interface Guide"
description: "Complete guide to implementing custom secret store providers for dsops"
lead: "Learn how to build custom providers that integrate new secret storage systems into dsops. This guide covers the Provider interface, implementation patterns, and best practices."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 10
---

## Overview

The Provider interface is the core abstraction that enables dsops to work with various secret storage systems. By implementing this interface, you can add support for any secret store, from cloud services to custom enterprise systems.

## The Provider Interface

The Provider interface is defined in `pkg/provider/provider.go`:

```go
type Provider interface {
    // Name returns the provider's unique identifier
    Name() string

    // Resolve retrieves a secret value from the provider
    Resolve(ctx context.Context, ref Reference) (SecretValue, error)

    // Describe returns metadata about a secret without retrieving its value
    Describe(ctx context.Context, ref Reference) (Metadata, error)

    // Capabilities returns the provider's supported features
    Capabilities() Capabilities

    // Validate checks if the provider is properly configured and authenticated
    Validate(ctx context.Context) error
}
```

### Complete API Documentation

For detailed API documentation with all types and methods, see:
- **[pkg/provider GoDoc](https://pkg.go.dev/github.com/systmms/dsops/pkg/provider)** - Full interface documentation
- **[Provider examples](https://github.com/systmms/dsops/tree/main/pkg/provider/examples_test.go)** - Working code examples

## Implementation Steps

### 1. Create Provider Structure

Start by creating a struct that will implement the Provider interface:

```go
package myprovider

import (
    "context"
    "time"
    
    "github.com/systmms/dsops/pkg/provider"
)

type MyProvider struct {
    // Provider configuration
    config Config
    
    // Client for your secret store
    client *MySecretClient
    
    // Provider name
    name string
}

type Config struct {
    Endpoint string
    APIKey   string
    Timeout  time.Duration
}
```

### 2. Implement Name Method

The Name method returns a unique identifier for your provider:

```go
func (p *MyProvider) Name() string {
    return p.name
}
```

### 3. Implement Resolve Method

The Resolve method is the core functionality - retrieving secrets:

```go
func (p *MyProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    // Validate the reference
    if ref.Key == "" {
        return provider.SecretValue{}, provider.ValidationError{
            Provider: p.name,
            Message:  "secret key is required",
        }
    }

    // Fetch the secret from your storage
    secret, err := p.client.GetSecret(ctx, ref.Key)
    if err != nil {
        if isNotFound(err) {
            return provider.SecretValue{}, provider.NotFoundError{
                Provider: p.name,
                Key:      ref.Key,
            }
        }
        return provider.SecretValue{}, err
    }

    // Handle field extraction if specified
    value := secret.Value
    if ref.Field != "" {
        fieldValue, err := extractField(value, ref.Field)
        if err != nil {
            return provider.SecretValue{}, err
        }
        value = fieldValue
    }

    // Handle version selection if specified
    if ref.Version != "" && ref.Version != secret.Version {
        // Fetch specific version
        versionedSecret, err := p.client.GetSecretVersion(ctx, ref.Key, ref.Version)
        if err != nil {
            return provider.SecretValue{}, err
        }
        secret = versionedSecret
    }

    return provider.SecretValue{
        Value:     value,
        Version:   secret.Version,
        UpdatedAt: secret.LastModified,
        Metadata:  secret.Tags,
    }, nil
}
```

### 4. Implement Describe Method

The Describe method provides metadata without retrieving the secret value:

```go
func (p *MyProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
    // Get secret metadata from your storage
    meta, err := p.client.GetSecretMetadata(ctx, ref.Key)
    if err != nil {
        if isNotFound(err) {
            // Return exists=false instead of error for not found
            return provider.Metadata{Exists: false}, nil
        }
        return provider.Metadata{}, err
    }

    return provider.Metadata{
        Exists:      true,
        Version:     meta.Version,
        UpdatedAt:   meta.LastModified,
        Size:        meta.Size,
        Type:        meta.SecretType,
        Permissions: meta.Permissions,
        Tags:        meta.Tags,
    }, nil
}
```

### 5. Implement Capabilities Method

Expose what your provider can do:

```go
func (p *MyProvider) Capabilities() provider.Capabilities {
    return provider.Capabilities{
        SupportsVersioning: true,
        SupportsMetadata:   true,
        SupportsWatching:   false, // Real-time updates
        SupportsBinary:     true,
        RequiresAuth:       true,
        AuthMethods:        []string{"api_key", "oauth2"},
    }
}
```

### 6. Implement Validate Method

Check configuration and connectivity:

```go
func (p *MyProvider) Validate(ctx context.Context) error {
    // Check configuration
    if p.config.APIKey == "" {
        return provider.AuthError{
            Provider: p.name,
            Message:  "API key is required",
        }
    }

    // Test connectivity
    if err := p.client.Ping(ctx); err != nil {
        return provider.AuthError{
            Provider: p.name,
            Message:  "failed to connect: " + err.Error(),
        }
    }

    // Verify permissions
    if err := p.client.CheckPermissions(ctx); err != nil {
        return provider.AuthError{
            Provider: p.name,
            Message:  "insufficient permissions: " + err.Error(),
        }
    }

    return nil
}
```

## Registration

### 1. Create Factory Function

Create a factory function that constructs your provider from configuration:

```go
// NewProvider creates a new MyProvider instance from configuration
func NewProvider(name string, config map[string]interface{}) (provider.Provider, error) {
    // Parse configuration
    cfg := Config{
        Timeout: 30 * time.Second, // Default
    }
    
    if endpoint, ok := config["endpoint"].(string); ok {
        cfg.Endpoint = endpoint
    } else {
        return nil, fmt.Errorf("endpoint is required")
    }
    
    if apiKey, ok := config["api_key"].(string); ok {
        cfg.APIKey = apiKey
    } else {
        return nil, fmt.Errorf("api_key is required")
    }
    
    if timeout, ok := config["timeout_ms"].(int); ok {
        cfg.Timeout = time.Duration(timeout) * time.Millisecond
    }

    // Create client
    client, err := NewMySecretClient(cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create client: %w", err)
    }

    return &MyProvider{
        config: cfg,
        client: client,
        name:   name,
    }, nil
}
```

### 2. Register in Provider Registry

Add your provider to `internal/providers/registry.go`:

```go
func init() {
    // Register your provider factory
    RegisterProviderFactory("my-provider", myprovider.NewProvider)
}
```

## Advanced Features

### Supporting Rotation

Implement the optional `Rotator` interface to support rotation:

```go
type Rotator interface {
    CreateNewVersion(ctx context.Context, ref Reference, newValue []byte, meta map[string]string) (string, error)
    DeprecateVersion(ctx context.Context, ref Reference, version string) error
    GetRotationMetadata(ctx context.Context, ref Reference) (RotationMetadata, error)
}
```

Example:

```go
func (p *MyProvider) CreateNewVersion(ctx context.Context, ref provider.Reference, newValue []byte, meta map[string]string) (string, error) {
    // Create new version in your storage
    version, err := p.client.CreateSecretVersion(ref.Key, newValue, meta)
    if err != nil {
        return "", fmt.Errorf("failed to create version: %w", err)
    }
    
    return version, nil
}
```

### Field Extraction

Support JSON field extraction for structured secrets:

```go
func extractField(value, field string) (string, error) {
    var data map[string]interface{}
    if err := json.Unmarshal([]byte(value), &data); err != nil {
        return "", fmt.Errorf("failed to parse JSON: %w", err)
    }
    
    fieldValue, ok := data[field]
    if !ok {
        return "", fmt.Errorf("field %s not found", field)
    }
    
    return fmt.Sprintf("%v", fieldValue), nil
}
```

## Testing

### Unit Tests

Write comprehensive unit tests for your provider:

```go
func TestMyProvider_Resolve(t *testing.T) {
    provider := &MyProvider{
        client: &MockClient{
            secrets: map[string]Secret{
                "test-key": {
                    Value:   "test-value",
                    Version: "v1",
                },
            },
        },
        name: "test-provider",
    }

    ctx := context.Background()
    ref := provider.Reference{
        Provider: "test-provider",
        Key:      "test-key",
    }

    value, err := provider.Resolve(ctx, ref)
    assert.NoError(t, err)
    assert.Equal(t, "test-value", value.Value)
    assert.Equal(t, "v1", value.Version)
}
```

### Contract Tests

Use the provider contract test suite:

```go
func TestMyProvider_Contract(t *testing.T) {
    provider := setupTestProvider(t)
    
    // Run the standard contract tests
    provider_test.RunProviderContractTests(t, provider, provider_test.ContractTestConfig{
        ValidRef: provider.Reference{
            Provider: "my-provider",
            Key:      "test-secret",
        },
        InvalidRef: provider.Reference{
            Provider: "my-provider",
            Key:      "nonexistent",
        },
    })
}
```

## Security Considerations

### 1. Never Log Secrets

Always use the logging wrapper for secret values:

```go
import "github.com/systmms/dsops/internal/logging"

// Wrong
log.Printf("Secret value: %s", secret.Value)

// Correct
log.Printf("Secret value: %s", logging.Secret(secret.Value))
```

### 2. Secure Memory Handling

Clear sensitive data from memory when done:

```go
defer func() {
    // Clear sensitive data
    for i := range sensitiveBytes {
        sensitiveBytes[i] = 0
    }
}()
```

### 3. Context Cancellation

Always respect context cancellation:

```go
select {
case <-ctx.Done():
    return provider.SecretValue{}, ctx.Err()
default:
    // Continue operation
}
```

### 4. Input Validation

Validate all inputs to prevent injection attacks:

```go
if strings.ContainsAny(ref.Key, "\n\r\t") {
    return provider.SecretValue{}, provider.ValidationError{
        Provider: p.name,
        Message:  "invalid characters in key",
    }
}
```

## Best Practices

### 1. Thread Safety

Ensure your provider is thread-safe:

```go
type MyProvider struct {
    mu     sync.RWMutex
    client *MySecretClient
    cache  map[string]cachedSecret
}

func (p *MyProvider) getCached(key string) (SecretValue, bool) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    cached, ok := p.cache[key]
    return cached.value, ok
}
```

### 2. Error Handling

Use appropriate error types:

```go
// For missing secrets
return provider.NotFoundError{
    Provider: p.name,
    Key:      ref.Key,
}

// For auth failures
return provider.AuthError{
    Provider: p.name,
    Message:  "authentication failed",
}

// For validation errors
return provider.ValidationError{
    Provider: p.name,
    Message:  "invalid configuration",
}
```

### 3. Performance

Optimize for common operations:

- Cache metadata to speed up Describe()
- Use connection pooling
- Implement efficient batch operations
- Support context timeouts

### 4. Documentation

Document your provider thoroughly:

```go
// MyProvider implements the Provider interface for MySecretStore.
//
// Configuration:
//   endpoint: API endpoint URL (required)
//   api_key: API authentication key (required)
//   timeout_ms: Request timeout in milliseconds (default: 30000)
//
// Authentication:
// The provider uses API key authentication. Set the api_key in configuration
// or use the MY_SECRET_API_KEY environment variable.
//
// Example:
//   providers:
//     my-secrets:
//       type: my-provider
//       endpoint: https://api.mysecrets.com
//       api_key: ${MY_SECRET_API_KEY}
type MyProvider struct {
    // ...
}
```

## Example: Complete Provider

Here's a complete example provider implementation:

```go
package custom

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    
    "github.com/systmms/dsops/internal/logging"
    "github.com/systmms/dsops/pkg/provider"
)

type CustomProvider struct {
    name   string
    client *CustomClient
    mu     sync.RWMutex
    cache  map[string]cacheEntry
    logger *logging.Logger
}

type cacheEntry struct {
    value     provider.SecretValue
    expiresAt time.Time
}

func NewProvider(name string, config map[string]interface{}) (provider.Provider, error) {
    endpoint, _ := config["endpoint"].(string)
    if endpoint == "" {
        return nil, fmt.Errorf("endpoint is required")
    }
    
    apiKey, _ := config["api_key"].(string)
    if apiKey == "" {
        return nil, fmt.Errorf("api_key is required")
    }
    
    client := &CustomClient{
        endpoint: endpoint,
        apiKey:   apiKey,
    }
    
    return &CustomProvider{
        name:   name,
        client: client,
        cache:  make(map[string]cacheEntry),
        logger: logging.NewLogger(),
    }, nil
}

func (p *CustomProvider) Name() string {
    return p.name
}

func (p *CustomProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
    // Check cache first
    if cached, ok := p.getFromCache(ref.Key); ok {
        return cached, nil
    }
    
    // Fetch from API
    secret, err := p.client.GetSecret(ctx, ref.Key)
    if err != nil {
        if err == ErrNotFound {
            return provider.SecretValue{}, provider.NotFoundError{
                Provider: p.name,
                Key:      ref.Key,
            }
        }
        return provider.SecretValue{}, err
    }
    
    value := provider.SecretValue{
        Value:     secret.Value,
        Version:   secret.Version,
        UpdatedAt: secret.UpdatedAt,
        Metadata:  secret.Metadata,
    }
    
    // Cache the result
    p.addToCache(ref.Key, value)
    
    return value, nil
}

func (p *CustomProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
    meta, err := p.client.GetMetadata(ctx, ref.Key)
    if err != nil {
        if err == ErrNotFound {
            return provider.Metadata{Exists: false}, nil
        }
        return provider.Metadata{}, err
    }
    
    return provider.Metadata{
        Exists:    true,
        Version:   meta.Version,
        UpdatedAt: meta.UpdatedAt,
        Size:      meta.Size,
        Type:      meta.Type,
        Tags:      meta.Tags,
    }, nil
}

func (p *CustomProvider) Capabilities() provider.Capabilities {
    return provider.Capabilities{
        SupportsVersioning: true,
        SupportsMetadata:   true,
        SupportsWatching:   false,
        SupportsBinary:     true,
        RequiresAuth:       true,
        AuthMethods:        []string{"api_key"},
    }
}

func (p *CustomProvider) Validate(ctx context.Context) error {
    if err := p.client.Ping(ctx); err != nil {
        return provider.AuthError{
            Provider: p.name,
            Message:  fmt.Sprintf("failed to connect: %v", err),
        }
    }
    return nil
}

func (p *CustomProvider) getFromCache(key string) (provider.SecretValue, bool) {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    entry, ok := p.cache[key]
    if !ok || time.Now().After(entry.expiresAt) {
        return provider.SecretValue{}, false
    }
    
    return entry.value, true
}

func (p *CustomProvider) addToCache(key string, value provider.SecretValue) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    p.cache[key] = cacheEntry{
        value:     value,
        expiresAt: time.Now().Add(5 * time.Minute),
    }
}
```

## Next Steps

1. Study existing providers in `internal/providers/` for real-world examples
2. Read the [Security Guidelines](/developer/security/) for security requirements
3. Check the [Testing Guide](/developer/testing/) for comprehensive testing strategies
4. Submit your provider as a pull request

## Resources

- [Provider Interface GoDoc](https://pkg.go.dev/github.com/systmms/dsops/pkg/provider)
- [Example Providers](https://github.com/systmms/dsops/tree/main/internal/providers)
- [Contract Test Suite](https://github.com/systmms/dsops/blob/main/pkg/provider/contract_test.go)
- [Provider Registry](https://github.com/systmms/dsops/blob/main/internal/providers/registry.go)