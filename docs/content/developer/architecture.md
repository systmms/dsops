---
title: "Architecture Overview"
description: "Comprehensive guide to dsops architecture, design principles, and component interactions"
lead: "Understand the dsops architecture, from high-level design principles to detailed component interactions. This guide provides the technical foundation for developing and extending dsops."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 30
---

## Overview

dsops follows a modular, layered architecture that separates concerns and enables extensibility. The system is designed around clear boundaries between secret storage, secret consumption, and the orchestration layer that connects them.

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Command Line Interface                          │
│                        (cmd/dsops/commands/)                            │
├─────────────────────────────────────────────────────────────────────────┤
│                         Configuration Layer                             │
│                          (internal/config/)                             │
├─────────────────────────────────────────────────────────────────────────┤
│                         Resolution Engine                               │
│                          (internal/resolve/)                            │
├─────────────────────┬─────────────────────────┬────────────────────────┤
│   Secret Stores     │    Rotation Engine      │   Service Definitions  │
│ (pkg/secretstore/)  │    (pkg/rotation/)      │   (pkg/service/)       │
├─────────────────────┼─────────────────────────┼────────────────────────┤
│   Store Providers   │   Rotation Strategies   │   Protocol Adapters    │
│(internal/providers/)│  (pkg/rotation/*)       │   (pkg/protocol/)      │
├─────────────────────┴─────────────────────────┴────────────────────────┤
│                    Security & Logging Layer                             │
│              (internal/logging/, internal/policy/)                      │
└─────────────────────────────────────────────────────────────────────────┘
```

## Core Design Principles

### 1. Separation of Concerns

dsops strictly separates:

- **Secret Stores**: Where secrets are stored (AWS, Vault, 1Password)
- **Services**: What uses secrets (PostgreSQL, Stripe, GitHub)
- **Resolution**: How secrets are fetched and transformed
- **Rotation**: How secrets are updated in services

This separation enables:
- Independent evolution of components
- Clear testing boundaries
- Flexible deployment patterns
- Easy addition of new providers/services

### 2. Interface-Driven Design

All major components are defined by interfaces:

```go
// Secret storage abstraction
type SecretStore interface {
    Name() string
    Resolve(ctx context.Context, ref SecretRef) (SecretValue, error)
    Describe(ctx context.Context, ref SecretRef) (SecretMetadata, error)
    Capabilities() SecretStoreCapabilities
    Validate(ctx context.Context) error
}

// Rotation abstraction
type SecretValueRotator interface {
    Name() string
    SupportsSecret(ctx context.Context, secret SecretInfo) bool
    Rotate(ctx context.Context, request RotationRequest) (*RotationResult, error)
    Verify(ctx context.Context, request VerificationRequest) error
    Rollback(ctx context.Context, request RollbackRequest) error
    GetStatus(ctx context.Context, secret SecretInfo) (*RotationStatusInfo, error)
}
```

### 3. Security First

Security is built into every layer:

- **Memory-Only by Default**: Secrets never touch disk unless explicitly requested
- **Automatic Redaction**: All logging automatically redacts sensitive values
- **Context Propagation**: Security context flows through all operations
- **Principle of Least Privilege**: Components only access what they need

### 4. Extensibility

The architecture supports multiple extension points:

- **Provider Plugins**: Add new secret stores
- **Rotation Strategies**: Implement service-specific rotation
- **Protocol Adapters**: Support new communication protocols
- **Transform Pipeline**: Add custom value transformations

## Component Deep Dive

### Command Line Interface

The CLI layer provides user interaction through Cobra commands:

```
cmd/dsops/
├── main.go              # Entry point
└── commands/
    ├── exec.go          # Execute with secrets
    ├── render.go        # Render templates
    ├── plan.go          # Show resolution plan
    ├── doctor.go        # Health checks
    ├── secrets.go       # Secret management
    └── rotation.go      # Rotation commands
```

Each command:
- Parses arguments and flags
- Loads configuration
- Delegates to business logic
- Handles errors and output formatting

### Configuration System

The configuration system handles:

```go
type Definition struct {
    Version      int                          // Config format version
    SecretStores map[string]SecretStoreConfig // Where secrets are stored
    Services     map[string]ServiceConfig     // What uses secrets
    Envs         map[string]Environment       // Variable definitions
}
```

Features:
- YAML-based configuration
- Environment variable substitution
- Validation and error reporting
- Legacy format compatibility

### Resolution Engine

The resolution engine is the heart of secret fetching:

```go
type Resolver struct {
    providers map[string]provider.Provider
    logger    *logging.Logger
    policy    *policy.Enforcer
}

func (r *Resolver) Resolve(ctx context.Context, env Environment) (ResolvedSecrets, error) {
    // 1. Build dependency graph
    // 2. Validate references
    // 3. Fetch secrets in parallel
    // 4. Apply transforms
    // 5. Enforce policies
    // 6. Return results
}
```

Key features:
- Parallel resolution for performance
- Dependency tracking
- Transform pipeline
- Error aggregation
- Policy enforcement

### Secret Store Abstraction

The modern secret store abstraction uses URI-based references:

```
store://store-name/path/to/secret#field?version=v&option=value
```

Components:
- **URI Parser**: Converts URIs to structured references
- **Store Registry**: Maps store names to implementations
- **Capability Negotiation**: Adapts to store features
- **Error Standardization**: Consistent error handling

Example implementation:

```go
func (s *AWSSecretsManager) Resolve(ctx context.Context, ref SecretRef) (SecretValue, error) {
    input := &secretsmanager.GetSecretValueInput{
        SecretId: aws.String(ref.Path),
    }
    
    if ref.Version != "" {
        input.VersionId = aws.String(ref.Version)
    }
    
    output, err := s.client.GetSecretValue(ctx, input)
    if err != nil {
        if isNotFound(err) {
            return SecretValue{}, NotFoundError{Store: s.Name(), Path: ref.Path}
        }
        return SecretValue{}, err
    }
    
    value := *output.SecretString
    if ref.Field != "" {
        value = extractJSONField(value, ref.Field)
    }
    
    return SecretValue{
        Value:     value,
        Version:   *output.VersionId,
        UpdatedAt: *output.CreatedDate,
    }, nil
}
```

### Rotation Architecture

The rotation system orchestrates secret lifecycle management:

```
┌─────────────────────────────────────────────────────────────┐
│                    Rotation Engine                          │
├─────────────────────────────────────────────────────────────┤
│              Strategy Registration & Routing                │
├─────────────────────┬─────────────────────┬─────────────────┤
│   Immediate         │    Two-Key          │    Overlap      │
│   Strategy          │    Strategy         │    Strategy     │
├─────────────────────┴─────────────────────┴─────────────────┤
│                  Protocol Adapters                          │
│         (SQL, HTTP API, NoSQL, Certificate)                 │
├─────────────────────────────────────────────────────────────┤
│                 Service Definitions                         │
│                   (dsops-data)                              │
└─────────────────────────────────────────────────────────────┘
```

Rotation flow:
1. Request validation
2. Strategy selection
3. New value generation
4. Service update
5. Verification
6. Storage update
7. Cleanup/rollback

### Security Layer

The security layer provides:

```go
// Automatic secret redaction
type Secret string

func (s Secret) String() string {
    return "[REDACTED]"
}

func (s Secret) GoString() string {
    return "[REDACTED]"
}

// Policy enforcement
type PolicyEnforcer struct {
    rules []Rule
}

func (e *PolicyEnforcer) Enforce(secret SecretInfo) error {
    for _, rule := range e.rules {
        if err := rule.Check(secret); err != nil {
            return PolicyViolation{Rule: rule, Error: err}
        }
    }
    return nil
}
```

### Transform Pipeline

Transforms modify secret values during resolution:

```go
type Transform interface {
    Name() string
    Apply(ctx context.Context, value string, config map[string]interface{}) (string, error)
}

// Built-in transforms
- json_extract: Extract field from JSON
- base64_decode: Decode base64
- template: Apply Go template
- regex_replace: Pattern replacement
```

## Data Flow

### Secret Resolution Flow

```
User Request → CLI Command → Load Config → Build Environment
                                              ↓
                                        Resolution Engine
                                              ↓
                              ┌───────────────┼───────────────┐
                              ↓               ↓               ↓
                        Provider 1      Provider 2      Provider N
                              ↓               ↓               ↓
                              └───────────────┼───────────────┘
                                              ↓
                                    Transform Pipeline
                                              ↓
                                    Policy Enforcement
                                              ↓
                                    Output Generation
                                              ↓
                                Execute/Render/Display
```

### Rotation Flow

```
Rotation Request → Validate Secret → Select Strategy
                                           ↓
                                    Generate New Value
                                           ↓
                              ┌────────────┼────────────┐
                              ↓                         ↓
                        Update Service          Store in Provider
                              ↓                         ↓
                        Verify Working          Update Reference
                              ↓                         ↓
                              └────────────┼────────────┘
                                           ↓
                                    Cleanup Old Value
                                           ↓
                                    Audit & Notify
```

## Extension Points

### Adding a Provider

1. Implement the `SecretStore` interface
2. Create a factory function
3. Register in the provider registry
4. Add configuration support

```go
// 1. Implement interface
type MyProvider struct {}

func (p *MyProvider) Resolve(ctx context.Context, ref SecretRef) (SecretValue, error) {
    // Implementation
}

// 2. Factory function
func NewMyProvider(config map[string]interface{}) (SecretStore, error) {
    // Parse config and create instance
}

// 3. Register
func init() {
    RegisterProvider("my-provider", NewMyProvider)
}
```

### Adding a Rotation Strategy

1. Implement `SecretValueRotator`
2. Register with rotation engine
3. Add service definitions

```go
// 1. Implement interface
type MyRotator struct {}

func (r *MyRotator) Rotate(ctx context.Context, req RotationRequest) (*RotationResult, error) {
    // Implementation
}

// 2. Register
engine.RegisterStrategy(&MyRotator{})
```

### Adding a Transform

1. Implement `Transform` interface
2. Register in transform registry

```go
type UppercaseTransform struct {}

func (t *UppercaseTransform) Apply(ctx context.Context, value string, config map[string]interface{}) (string, error) {
    return strings.ToUpper(value), nil
}
```

## Performance Considerations

### Parallel Resolution

Secrets are resolved in parallel where possible:

```go
type parallelResolver struct {
    workers int
}

func (r *parallelResolver) resolveAll(ctx context.Context, refs []SecretRef) ([]SecretValue, error) {
    results := make([]SecretValue, len(refs))
    errors := make([]error, len(refs))
    
    var wg sync.WaitGroup
    sem := make(chan struct{}, r.workers)
    
    for i, ref := range refs {
        wg.Add(1)
        go func(idx int, ref SecretRef) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            
            results[idx], errors[idx] = r.resolve(ctx, ref)
        }(i, ref)
    }
    
    wg.Wait()
    return results, aggregateErrors(errors)
}
```

### Caching Strategy

Providers can implement caching:

```go
type cachedProvider struct {
    provider Provider
    cache    *lru.Cache
    ttl      time.Duration
}

func (c *cachedProvider) Resolve(ctx context.Context, ref Reference) (SecretValue, error) {
    key := ref.String()
    
    if cached, ok := c.cache.Get(key); ok {
        if !cached.(cacheEntry).expired() {
            return cached.(cacheEntry).value, nil
        }
    }
    
    value, err := c.provider.Resolve(ctx, ref)
    if err != nil {
        return SecretValue{}, err
    }
    
    c.cache.Add(key, cacheEntry{value: value, expires: time.Now().Add(c.ttl)})
    return value, nil
}
```

## Error Handling

### Error Types

```go
// User errors - clear action required
type ConfigError struct {
    Field      string
    Value      interface{}
    Message    string
    Suggestion string
}

// System errors - may be transient
type ProviderError struct {
    Provider string
    Operation string
    Err error
}

// Security errors - policy violations
type PolicyError struct {
    Rule string
    Secret string
    Reason string
}
```

### Error Aggregation

Multiple errors are collected and reported:

```go
type ErrorCollector struct {
    errors []error
}

func (c *ErrorCollector) Add(err error) {
    if err != nil {
        c.errors = append(c.errors, err)
    }
}

func (c *ErrorCollector) Error() error {
    if len(c.errors) == 0 {
        return nil
    }
    return MultiError{Errors: c.errors}
}
```

## Testing Architecture

### Unit Testing

Each component has focused unit tests:

```go
func TestResolver_Resolve(t *testing.T) {
    resolver := &Resolver{
        providers: map[string]provider.Provider{
            "test": &MockProvider{
                secrets: map[string]string{
                    "key1": "value1",
                },
            },
        },
    }
    
    env := Environment{
        "VAR1": Variable{
            From: &Reference{Provider: "test", Key: "key1"},
        },
    }
    
    resolved, err := resolver.Resolve(context.Background(), env)
    assert.NoError(t, err)
    assert.Equal(t, "value1", resolved["VAR1"])
}
```

### Integration Testing

Integration tests verify component interactions:

```go
func TestEndToEnd_SecretResolution(t *testing.T) {
    // Setup real providers
    // Load test configuration
    // Execute resolution
    // Verify results
}
```

### Contract Testing

Provider contract tests ensure compliance:

```go
func TestProvider_Contract(t *testing.T, provider Provider) {
    // Test all interface methods
    // Verify error handling
    // Check capability accuracy
}
```

## Future Architecture

### Plugin System

Future support for external providers:

```go
type PluginProvider struct {
    cmd *exec.Cmd
    rpc *rpc.Client
}

func (p *PluginProvider) Resolve(ctx context.Context, ref Reference) (SecretValue, error) {
    var result SecretValue
    err := p.rpc.Call("Provider.Resolve", ref, &result)
    return result, err
}
```

### Event System

Planned event-driven architecture:

```go
type EventBus interface {
    Publish(event Event) error
    Subscribe(eventType string, handler EventHandler) error
}

type RotationEvent struct {
    Type string
    Secret SecretInfo
    Result RotationResult
    Timestamp time.Time
}
```

## Resources

- [API Documentation](https://pkg.go.dev/github.com/systmms/dsops)
- [Design Documents](/contributing/adr/)
- [Security Architecture](/developer/security/)
- [Testing Guide](/developer/testing/)