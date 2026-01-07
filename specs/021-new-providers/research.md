# Research: New Secret Store Providers

**Date**: 2026-01-03
**Feature**: 021-new-providers
**Status**: Complete

## Summary

Research findings for implementing three new secret store providers: OS Keychain, Infisical, and Akeyless.

---

## 1. OS Keychain Provider

### Library Selection

**Decision**: Use `github.com/keybase/go-keychain`

**Rationale**:
- Already in go.sum (no new dependency)
- Cross-platform: macOS Keychain + Linux Secret Service
- Well-maintained by Keybase team
- Simple API for generic password items

**Alternatives Considered**:
- `github.com/zalando/go-keyring` - Also good, but go-keychain already a dependency
- `github.com/99designs/keyring` - More complex, supports more backends than needed
- Direct CGo bindings - Too complex, maintenance burden

### Usage Patterns

```go
// Query a keychain item
item := keychain.NewItem()
item.SetSecClass(keychain.SecClassGenericPassword)
item.SetService("com.myapp.dsops")
item.SetAccount("api-key")
item.SetMatchLimit(keychain.MatchLimitOne)
item.SetReturnData(true)

results, err := keychain.QueryItem(item)
if err == keychain.ErrorItemNotFound {
    // Handle not found
}
```

### Platform Detection

**Decision**: Use Go build tags + runtime.GOOS fallback

```go
// keychain_darwin.go
//go:build darwin

// keychain_linux.go
//go:build linux

// keychain_unsupported.go
//go:build !darwin && !linux
```

**Rationale**: Build tags eliminate dead code on each platform; runtime check as fallback for edge cases.

### Headless Detection

**Decision**: Check for `$DISPLAY` (Linux) and `$SSH_TTY` (both platforms)

**Rationale**: Standard Unix conventions for detecting GUI availability. Return clear error suggesting alternative providers for CI/headless environments.

---

## 2. Infisical Provider

### API Integration

**Decision**: Use REST API directly with standard `net/http`

**Rationale**:
- No official Go SDK available
- REST API is well-documented and stable
- Avoids third-party SDK maintenance risk
- Easier to mock for testing

**Alternatives Considered**:
- Community Go SDK - None mature enough
- Generate from OpenAPI spec - Overkill for our use case

### Authentication Methods

**Decision**: Support all three methods, prioritize Machine Identity

| Method | Use Case | Config Key |
|--------|----------|------------|
| Machine Identity | Production (recommended) | `client_id`, `client_secret` |
| Service Token | Legacy/simple deployments | `service_token` |
| API Key | Development/testing | `api_key` |

**API Endpoints**:
```
POST /api/v1/auth/universal-auth/login  # Machine Identity
GET  /api/v3/secrets/{secretName}       # Fetch secret
GET  /api/v3/secrets                    # List secrets (doctor)
```

### Self-Hosted Support

**Decision**: Make `host` configurable with cloud default

```yaml
host: "https://app.infisical.com"  # Default
host: "https://infisical.internal.company.com"  # Self-hosted
```

**TLS Configuration**: Support custom CA via `ca_cert` or `insecure_skip_verify` (with warning).

---

## 3. Akeyless Provider

### SDK Selection

**Decision**: Use official `github.com/akeylesslabs/akeyless-go` SDK

**Rationale**:
- Official SDK maintained by Akeyless
- Handles authentication complexity
- Supports all auth methods
- Active development

**Alternatives Considered**:
- REST API directly - Possible but auth handling complex
- No alternative SDKs exist

### Authentication Methods

**Decision**: Support common methods, defer exotic ones

| Method | Priority | Config |
|--------|----------|--------|
| API Key | P1 | `access_id`, `access_key` |
| AWS IAM | P1 | `access_id` + auto-detect |
| Azure AD | P2 | `access_id`, `azure_ad_object_id` |
| GCP | P2 | `access_id`, `gcp_audience` |
| SAML/OIDC | P3 | Defer to future |

**Rationale**: API Key and cloud IAM cover 90%+ of use cases. SAML/OIDC require interactive flows better suited to CLI tools.

### SDK Usage Pattern

```go
import "github.com/akeylesslabs/akeyless-go/v3"

// Create client
client := akeyless.NewAPIClient(&akeyless.Configuration{
    Servers: []akeyless.ServerConfiguration{
        {URL: "https://api.akeyless.io"},
    },
})

// Authenticate
authBody := akeyless.NewAuthWithDefaults()
authBody.SetAccessId(accessId)
authBody.SetAccessKey(accessKey)
authRes, _, err := client.V2Api.Auth(ctx).Body(*authBody).Execute()
token := authRes.GetToken()

// Get secret
getBody := akeyless.NewGetSecretValue()
getBody.SetNames([]string{"/path/to/secret"})
getBody.SetToken(token)
res, _, err := client.V2Api.GetSecretValue(ctx).Body(*getBody).Execute()
```

---

## 4. Shared Patterns

### Timeout Configuration

**Decision**: 30-second default with context timeout

```go
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()
```

**Rationale**: Matches AWS SDK defaults; configurable per-provider if needed.

### Credential Caching

**Decision**: In-memory cache per provider instance

```go
type cachedToken struct {
    token     string
    expiresAt time.Time
}

func (p *Provider) getToken(ctx context.Context) (string, error) {
    if p.cache != nil && p.cache.expiresAt.After(time.Now()) {
        return p.cache.token, nil
    }
    // Fetch new token...
}
```

**Rationale**: Provider instances live for process duration; cache is automatically cleared on process exit.

### Error Handling

**Decision**: Map provider errors to standard dsops error types

```go
switch {
case errors.Is(err, ErrNotFound):
    return provider.NotFoundError{Provider: p.Name(), Key: ref.Key}
case errors.Is(err, ErrAuth):
    return provider.AuthError{Provider: p.Name(), Message: err.Error()}
default:
    return fmt.Errorf("%s: %w", p.Name(), err)
}
```

### Doctor Integration

**Decision**: Validate in two phases

1. **Config Validation**: Check required fields present
2. **Connectivity Test**: Attempt auth, verify permissions

```go
func (p *Provider) Validate(ctx context.Context) error {
    // 1. Config check
    if p.config.AccessID == "" {
        return fmt.Errorf("missing required field: access_id")
    }
    // 2. Auth check
    if _, err := p.authenticate(ctx); err != nil {
        return fmt.Errorf("authentication failed: %w", err)
    }
    return nil
}
```

---

## 5. Testing Strategy

### Unit Tests

- Mock external interfaces (HTTP client, keychain, SDK)
- Table-driven tests for all error paths
- 100% coverage of config validation

### Contract Tests

- All three providers pass existing `provider_contract_test.go`
- Add to `TestAllProviders` list in contract tests

### Integration Tests

| Provider | Test Environment | Skip Condition |
|----------|------------------|----------------|
| Keychain | macOS/Linux CI runners | `!darwin && !linux` |
| Infisical | Docker Infisical instance | `INFISICAL_HOST` not set |
| Akeyless | Akeyless dev account | `AKEYLESS_ACCESS_ID` not set |

---

## References

- [go-keychain documentation](https://github.com/keybase/go-keychain)
- [Infisical API Reference](https://infisical.com/docs/api-reference)
- [Akeyless Go SDK](https://github.com/akeylesslabs/akeyless-go)
- [Secret Service D-Bus Spec](https://specifications.freedesktop.org/secret-service/)
