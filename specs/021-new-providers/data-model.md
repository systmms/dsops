# Data Model: New Secret Store Providers

**Date**: 2026-01-03
**Feature**: 021-new-providers

## Overview

This document defines the data structures, configuration schemas, and state management for the three new providers.

---

## 1. Configuration Structures

### OS Keychain Provider

```go
// KeychainConfig holds configuration for the OS Keychain provider
type KeychainConfig struct {
    // ServicePrefix is prepended to service names in references
    // Example: "com.mycompany" + "/myapp" → service="com.mycompany.myapp"
    ServicePrefix string `mapstructure:"service_prefix"`

    // AccessGroup (macOS only) specifies the keychain access group
    // for shared keychain items between applications
    AccessGroup string `mapstructure:"access_group"`
}
```

**YAML Configuration**:
```yaml
secretStores:
  local:
    type: keychain
    service_prefix: "com.mycompany"
    access_group: "TEAMID.com.mycompany.shared"  # Optional, macOS only
```

### Infisical Provider

```go
// InfisicalConfig holds configuration for the Infisical provider
type InfisicalConfig struct {
    // Host is the Infisical instance URL
    // Defaults to "https://app.infisical.com"
    Host string `mapstructure:"host"`

    // ProjectID is the Infisical project identifier (required)
    ProjectID string `mapstructure:"project_id"`

    // Environment is the environment slug (required)
    // Examples: "dev", "staging", "prod"
    Environment string `mapstructure:"environment"`

    // Auth contains authentication configuration
    Auth InfisicalAuth `mapstructure:"auth"`

    // Timeout for API requests (default: 30s)
    Timeout time.Duration `mapstructure:"timeout"`

    // CACert is path to custom CA certificate for self-hosted instances
    CACert string `mapstructure:"ca_cert"`

    // InsecureSkipVerify disables TLS verification (use with caution)
    InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

// InfisicalAuth defines authentication method for Infisical
type InfisicalAuth struct {
    // Method is the authentication method
    // Values: "machine_identity", "service_token", "api_key"
    Method string `mapstructure:"method"`

    // ClientID for machine identity auth
    ClientID string `mapstructure:"client_id"`

    // ClientSecret for machine identity auth
    ClientSecret string `mapstructure:"client_secret"`

    // ServiceToken for service token auth (legacy)
    ServiceToken string `mapstructure:"service_token"`

    // APIKey for API key auth (development)
    APIKey string `mapstructure:"api_key"`
}
```

**YAML Configuration**:
```yaml
secretStores:
  infisical-prod:
    type: infisical
    host: "https://app.infisical.com"
    project_id: "proj_abc123"
    environment: "production"
    timeout: 30s
    auth:
      method: machine_identity
      client_id: "${INFISICAL_CLIENT_ID}"
      client_secret: "${INFISICAL_CLIENT_SECRET}"
```

### Akeyless Provider

```go
// AkeylessConfig holds configuration for the Akeyless provider
type AkeylessConfig struct {
    // AccessID is the Akeyless access ID (required)
    AccessID string `mapstructure:"access_id"`

    // GatewayURL is the custom gateway URL for enterprise deployments
    // Defaults to "https://api.akeyless.io"
    GatewayURL string `mapstructure:"gateway_url"`

    // Auth contains authentication configuration
    Auth AkeylessAuth `mapstructure:"auth"`

    // Timeout for API requests (default: 30s)
    Timeout time.Duration `mapstructure:"timeout"`
}

// AkeylessAuth defines authentication method for Akeyless
type AkeylessAuth struct {
    // Method is the authentication method
    // Values: "api_key", "aws_iam", "azure_ad", "gcp", "oidc", "saml"
    Method string `mapstructure:"method"`

    // AccessKey for API key auth
    AccessKey string `mapstructure:"access_key"`

    // AzureADObjectID for Azure AD auth
    AzureADObjectID string `mapstructure:"azure_ad_object_id"`

    // GCPAudience for GCP auth
    GCPAudience string `mapstructure:"gcp_audience"`
}
```

**YAML Configuration**:
```yaml
secretStores:
  akeyless-prod:
    type: akeyless
    access_id: "p-abc123"
    gateway_url: "https://api.akeyless.io"
    timeout: 30s
    auth:
      method: api_key
      access_key: "${AKEYLESS_ACCESS_KEY}"
```

---

## 2. Provider State

### Token Cache (Per-Process)

```go
// tokenCache stores authentication tokens in memory
type tokenCache struct {
    mu        sync.RWMutex
    token     string
    expiresAt time.Time
}

func (c *tokenCache) Get() (string, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if c.token == "" || time.Now().After(c.expiresAt) {
        return "", false
    }
    return c.token, true
}

func (c *tokenCache) Set(token string, ttl time.Duration) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.token = token
    c.expiresAt = time.Now().Add(ttl)
}
```

**Lifecycle**:
- Created when provider instance is created
- Lives for duration of dsops process
- Automatically cleared on process exit
- No disk persistence (per FR-017)

---

## 3. Reference Parsing

### Keychain Reference

```
store://keychain/service-name/account-name
         ↓           ↓              ↓
     provider    service        account
```

**Parsing Rules**:
- `service-name`: Combined with `service_prefix` if configured
- `account-name`: Used as-is for keychain lookup
- Version not supported (keychain doesn't support versioning)

### Infisical Reference

```
store://infisical/SECRET_NAME
store://infisical/folder/SECRET_NAME
store://infisical/folder/subfolder/SECRET_NAME@v2
         ↓              ↓                       ↓
     provider         path                  version
```

**Parsing Rules**:
- Path can be nested with `/`
- Version suffix `@vN` optional
- Combined with `project_id` and `environment` from config

### Akeyless Reference

```
store://akeyless/path/to/secret
store://akeyless/path/to/secret@latest
         ↓           ↓            ↓
     provider       path       version
```

**Parsing Rules**:
- Path is hierarchical (like file paths)
- Version suffix optional (default: latest)

---

## 4. Error Types

### Provider-Specific Errors

```go
// KeychainError wraps OS keychain errors
type KeychainError struct {
    Op      string // Operation: "query", "validate"
    Service string
    Account string
    Err     error
}

// InfisicalError wraps Infisical API errors
type InfisicalError struct {
    Op         string // Operation: "auth", "fetch", "list"
    StatusCode int
    Message    string
    Err        error
}

// AkeylessError wraps Akeyless SDK errors
type AkeylessError struct {
    Op      string // Operation: "auth", "fetch", "list"
    Path    string
    Message string
    Err     error
}
```

### Error Mapping

| Provider Error | dsops Error Type | User Message |
|----------------|------------------|--------------|
| Keychain item not found | `NotFoundError` | "Secret not found in keychain: service={service}, account={account}" |
| Keychain access denied | `AuthError` | "Keychain access denied. Grant access in Keychain Access.app or authenticate with Touch ID/password" |
| Infisical 401 | `AuthError` | "Infisical authentication failed. Check credentials and try again" |
| Infisical 404 | `NotFoundError` | "Secret not found in Infisical: {path} in project {project_id}/{environment}" |
| Akeyless auth failure | `AuthError` | "Akeyless authentication failed: {message}" |
| Akeyless secret not found | `NotFoundError` | "Secret not found in Akeyless: {path}" |
| Network timeout | wrapped error | "Request timed out after 30s. Check network connectivity and try again" |

---

## 5. Capabilities

### Provider Capabilities Matrix

| Capability | Keychain | Infisical | Akeyless |
|------------|----------|-----------|----------|
| `SupportsVersioning` | false | true | true |
| `SupportsMetadata` | true | true | true |
| `SupportsWatching` | false | false | false |
| `SupportsBinary` | true | true | true |
| `RequiresAuth` | false* | true | true |
| `AuthMethods` | ["os"] | ["machine_identity", "service_token", "api_key"] | ["api_key", "aws_iam", "azure_ad", "gcp"] |

*Keychain uses OS-level auth (Touch ID, password) on access, not configured credentials.

---

## 6. Validation Rules

### Configuration Validation

**Keychain**:
- No required fields
- `access_group` only valid on macOS (warn on Linux)

**Infisical**:
- `project_id` required
- `environment` required
- One of `client_id`+`client_secret`, `service_token`, or `api_key` required
- `host` defaults to cloud if empty

**Akeyless**:
- `access_id` required
- Authentication credentials required based on `auth.method`
- `gateway_url` defaults to cloud if empty

### Reference Validation

```go
func (p *KeychainProvider) validateRef(ref provider.Reference) error {
    parts := strings.Split(ref.Key, "/")
    if len(parts) < 2 {
        return fmt.Errorf("keychain reference must be service/account, got: %s", ref.Key)
    }
    return nil
}
```
