---
title: "Literal Provider"
description: "Use plain text values directly in configuration"
lead: "The Literal provider allows you to include non-sensitive values directly in your configuration file. Use this for static values, defaults, or values that don't require secure storage."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 10
---

## Overview

The Literal provider is the simplest provider in dsops - it returns exactly the value you specify in the configuration. This is useful for:

- Non-sensitive configuration values
- Default or fallback values
- Static URLs or endpoints
- Development placeholders
- Testing and debugging

{{< alert icon="⚠️" text="Never use the Literal provider for actual secrets like passwords, API keys, or certificates. These should always be stored in secure secret stores." />}}

## Use Cases

### When to Use

- **Static Configuration**: Non-sensitive values like URLs, ports, or feature flags
- **Development Defaults**: Placeholder values for local development
- **Testing**: Mock values for testing workflows
- **Mixed Configurations**: Combine with secure providers for hybrid configs

### When NOT to Use

- **Passwords**: Use a secure provider like 1Password or AWS Secrets Manager
- **API Keys**: Store in proper secret management systems
- **Certificates**: Use dedicated certificate stores
- **Any Sensitive Data**: If it needs protection, don't use Literal

## Configuration

### Basic Setup

```yaml
version: 1

secretStores:
  literal:
    type: literal

envs:
  development:
    API_URL:
      from:
        store: literal
        value: https://api.dev.example.com
    
    DEBUG_MODE:
      from:
        store: literal
        value: "true"
    
    MAX_RETRIES:
      from:
        store: literal
        value: "3"
```

### Provider Options

The Literal provider has no configuration options - it's always available and requires no setup.

## Usage Examples

### Basic Values

```yaml
envs:
  production:
    # Static configuration
    SERVICE_NAME:
      from:
        store: literal
        value: my-awesome-service
    
    # URLs and endpoints
    METRICS_ENDPOINT:
      from:
        store: literal
        value: https://metrics.internal.com/v1/ingest
    
    # Feature flags
    ENABLE_NEW_FEATURE:
      from:
        store: literal
        value: "false"
```

### Numeric Values

```yaml
envs:
  production:
    # Always quote numbers to ensure string type
    PORT:
      from:
        store: literal
        value: "8080"
    
    TIMEOUT_SECONDS:
      from:
        store: literal
        value: "30"
    
    MAX_CONNECTIONS:
      from:
        store: literal
        value: "100"
```

### Boolean Values

```yaml
envs:
  production:
    # Quote booleans to ensure they're treated as strings
    ENABLE_DEBUG:
      from:
        store: literal
        value: "false"
    
    USE_CACHE:
      from:
        store: literal
        value: "true"
```

### Complex Values

```yaml
envs:
  production:
    # JSON strings
    FEATURE_CONFIG:
      from:
        store: literal
        value: '{"feature1": true, "feature2": false, "limit": 100}'
    
    # Multi-line values
    PUBLIC_KEY:
      from:
        store: literal
        value: |
          -----BEGIN PUBLIC KEY-----
          MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQC7VJTUt...
          -----END PUBLIC KEY-----
```

## Common Patterns

### Mixed Configuration

Combine literal values with secure providers:

```yaml
secretStores:
  literal:
    type: literal
  
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth_method: approle

envs:
  production:
    # Non-sensitive: use literal
    DATABASE_HOST:
      from:
        store: literal
        value: db.production.example.com
    
    DATABASE_PORT:
      from:
        store: literal
        value: "5432"
    
    # Sensitive: use secure provider
    DATABASE_PASSWORD:
      from:
        store: vault
        path: secret/database/prod
        key: password
```

### Environment-Specific Values

```yaml
envs:
  development:
    API_URL:
      from:
        store: literal
        value: http://localhost:3000
    
    LOG_LEVEL:
      from:
        store: literal
        value: debug
  
  production:
    API_URL:
      from:
        store: literal
        value: https://api.example.com
    
    LOG_LEVEL:
      from:
        store: literal
        value: info
```

### Default Values Pattern

Use literal as fallback with transforms:

```yaml
envs:
  production:
    # Try AWS first, fall back to literal default
    REGION:
      from:
        store: aws
        path: /config/region
      default:
        from:
          store: literal
          value: us-east-1
```

## Security Considerations

### What NOT to Store

Never use the Literal provider for:

```yaml
# ❌ BAD - Never do this!
envs:
  production:
    DATABASE_PASSWORD:
      from:
        store: literal
        value: "super-secret-password"  # NEVER DO THIS!
    
    API_KEY:
      from:
        store: literal
        value: "sk_live_abc123..."     # NEVER DO THIS!
    
    PRIVATE_KEY:
      from:
        store: literal
        value: |                        # NEVER DO THIS!
          -----BEGIN RSA PRIVATE KEY-----
          MIIEpAIBAAKCAQEA...
          -----END RSA PRIVATE KEY-----
```

### Safe Usage

```yaml
# ✅ GOOD - Safe literal usage
envs:
  production:
    # Non-sensitive configuration
    SERVICE_VERSION:
      from:
        store: literal
        value: "2.1.0"
    
    # Public endpoints
    PUBLIC_API_URL:
      from:
        store: literal
        value: https://api.example.com
    
    # Feature toggles
    MAINTENANCE_MODE:
      from:
        store: literal
        value: "false"
```

## Best Practices

### 1. Document Intent

Always comment why you're using literal values:

```yaml
envs:
  production:
    # Static value - same across all deployments
    COMPANY_NAME:
      from:
        store: literal
        value: "Acme Corp"
    
    # Public API endpoint - not sensitive
    WEATHER_API_URL:
      from:
        store: literal
        value: https://api.weather.com/v1
```

### 2. Use for Configuration, Not Secrets

```yaml
# Good uses of literal
LOG_FORMAT:
  from:
    store: literal
    value: json

CORS_ALLOWED_ORIGINS:
  from:
    store: literal
    value: "https://app.example.com,https://www.example.com"

# Bad uses - should use secure provider
DATABASE_PASSWORD:  # ❌ Never literal
JWT_SECRET:        # ❌ Never literal
```

### 3. Consider Environment Variables

For truly static values, consider if environment variables or build-time configuration might be more appropriate than dsops.

### 4. Type Safety

Always quote values to ensure consistent string types:

```yaml
# Quote everything to avoid YAML type coercion
PORT:
  from:
    store: literal
    value: "8080"      # Not 8080

ENABLE_FEATURE:
  from:
    store: literal
    value: "true"      # Not true
```

## Testing with Literal

The Literal provider is excellent for testing:

```yaml
# test.yaml
version: 1

secretStores:
  literal:
    type: literal

envs:
  test:
    DATABASE_URL:
      from:
        store: literal
        value: "postgresql://test:test@localhost:5432/test"
    
    API_KEY:
      from:
        store: literal
        value: "test-key-not-real"
    
    ENABLE_MOCKS:
      from:
        store: literal
        value: "true"
```

```bash
# Test with literal values
dsops exec --config test.yaml --env test -- npm test
```

## Migration Guide

### From Environment Variables

Migrating from hardcoded environment variables:

```bash
# Before: .env file
API_URL=https://api.example.com
LOG_LEVEL=info
PORT=8080

# After: dsops.yaml
envs:
  production:
    API_URL:
      from:
        store: literal
        value: https://api.example.com
    
    LOG_LEVEL:
      from:
        store: literal
        value: info
    
    PORT:
      from:
        store: literal
        value: "8080"
```

### From Config Files

Migrating from JSON/YAML configs:

```javascript
// Before: config.json
{
  "apiUrl": "https://api.example.com",
  "timeout": 30,
  "retries": 3
}

// After: dsops.yaml with transforms
envs:
  production:
    CONFIG:
      from:
        store: literal
        value: |
          {
            "apiUrl": "https://api.example.com",
            "timeout": 30,
            "retries": 3
          }
```

## Troubleshooting

### Value Not Appearing

If your literal value isn't showing up:

1. Check YAML syntax - improper indentation is common
2. Ensure you're using the correct environment
3. Quote your values to prevent YAML interpretation

### Special Characters

For values with special characters:

```yaml
# Use quotes for special characters
REGEX_PATTERN:
  from:
    store: literal
    value: "^[a-zA-Z0-9]+$"

# Use literal block for multi-line
CERTIFICATE:
  from:
    store: literal
    value: |
      -----BEGIN CERTIFICATE-----
      MIIDXTCCAkWgAwIBAgIJAKl...
      -----END CERTIFICATE-----
```

### Type Issues

YAML can interpret unquoted values:

```yaml
# Without quotes, YAML might interpret these
VERSION:
  from:
    store: literal
    value: 1.0        # Becomes float 1.0

ENABLED:
  from:
    store: literal  
    value: yes        # Becomes boolean true

# With quotes, they're always strings
VERSION:
  from:
    store: literal
    value: "1.0"      # String "1.0"

ENABLED:
  from:
    store: literal
    value: "yes"      # String "yes"
```

## Related Documentation

- [Configuration Reference](/reference/configuration/)
- [Security Best Practices](/security/)
- [Provider Overview](/providers/)
- [Environment Variables Guide](/guides/environment-variables/)