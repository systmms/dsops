---
title: "Rotation Configuration"
description: "Complete guide to configuring secret rotation in dsops"
lead: "Learn how to configure automated secret rotation with service definitions, policies, and scheduling. Covers all configuration options and real-world examples."
date: 2025-08-26T12:00:00-07:00
lastmod: 2025-08-26T12:00:00-07:00
draft: false
weight: 25
---

## Overview

Secret rotation in dsops is configured through:

1. **Service Definitions** - What services use secrets
2. **Rotation Policies** - How and when to rotate
3. **Secret Linkage** - Connecting secrets to services
4. **Scheduling** - Automated rotation triggers

## Basic Configuration

### Minimal Rotation Setup

```yaml
version: 1

# 1. Define where secrets are stored
secretStores:
  aws:
    type: aws.secretsmanager
    region: us-east-1

# 2. Define services that use secrets
services:
  database:
    type: postgresql
    host: db.example.com
    port: 5432

# 3. Link secrets to services
envs:
  production:
    DB_PASSWORD:
      from:
        store: aws
        key: prod/database/password
      service: database  # This enables rotation!
      rotation:
        ttl: 30d  # Rotate every 30 days
```

### Complete Configuration Example

```yaml
version: 1

secretStores:
  vault:
    type: hashicorp.vault
    url: https://vault.example.com
    auth_method: approle
    role_id: ${VAULT_ROLE_ID}
    secret_id: ${VAULT_SECRET_ID}

services:
  # PostgreSQL with full options
  postgres-prod:
    type: postgresql
    host: prod-db.example.com
    port: 5432
    database: production
    ssl_mode: require
    
    # Admin credentials for rotation
    admin_credentials:
      username: postgres
      password:
        from:
          store: vault
          key: admin/postgres/password
    
    # Rotation configuration
    rotation:
      strategy: two-key
      verification:
        enabled: true
        timeout: 30s
        query: "SELECT 1"
      
      # Hooks for custom logic
      hooks:
        pre_rotation: /scripts/notify-rotation-start.sh
        post_rotation: /scripts/update-monitoring.sh
        on_failure: /scripts/alert-team.sh

  # API service example
  stripe-api:
    type: stripe
    environment: production
    rotation:
      strategy: overlap
      overlap_duration: 48h

# Rotation groups for coordinated rotation
rotation_groups:
  database_cluster:
    members:
      - postgres-prod
      - postgres-replica-1
      - postgres-replica-2
    coordination: sequential
    delay_between: 5m

# Global rotation policies
policies:
  rotation:
    # Default settings for all services
    defaults:
      ttl: 90d
      strategy: immediate
      notifications:
        on_failure:
          - type: slack
            webhook: ${SLACK_WEBHOOK_URL}
    
    # Service-specific overrides
    overrides:
      - services: ["*-prod"]
        ttl: 30d
        strategy: two-key
      
      - services: ["stripe-*", "payment-*"]
        ttl: 60d
        require_approval: true

envs:
  production:
    DATABASE_URL:
      from:
        store: vault
        key: database/prod/connection_string
      service: postgres-prod
      rotation:
        ttl: 30d
        notify_before: 7d
        
    STRIPE_API_KEY:
      from:
        store: vault  
        key: stripe/prod/secret_key
      service: stripe-api
      rotation:
        ttl: 60d
        approval_required: true
        approvers: ["security-team", "payments-team"]
```

## Service Configuration

### Service Types

Services are defined by their type, which comes from dsops-data:

```yaml
services:
  # Database service
  my-database:
    type: postgresql  # References dsops-data definition
    host: localhost
    port: 5432
    
  # API service  
  my-api:
    type: github
    organization: my-org
    
  # Custom service
  custom-service:
    type: custom
    protocol: http-api
    endpoint: https://api.example.com
```

### Using dsops-data Definitions

Reference community-maintained service definitions:

```yaml
services:
  postgres:
    type: postgresql
    # Use pre-defined instance configuration
    instance_ref: dsops-data/providers/postgresql/instances/aws-rds.yaml
    
    # Override specific values
    overrides:
      host: my-specific-host.aws.com
      port: 5433
```

### Service Authentication

Configure how dsops authenticates to rotate secrets:

```yaml
services:
  database:
    type: mysql
    host: db.example.com
    
    # Method 1: Direct credentials
    admin_credentials:
      username: root
      password: ${MYSQL_ROOT_PASSWORD}
    
    # Method 2: Reference from secret store
    admin_credentials:
      username:
        from:
          store: vault
          key: mysql/admin/username
      password:
        from:
          store: vault
          key: mysql/admin/password
    
    # Method 3: IAM/Cloud authentication
    auth:
      method: aws-iam
      role_arn: arn:aws:iam::123456789012:role/RDSAccess
```

## Rotation Policies

### TTL (Time To Live)

Control when secrets should be rotated:

```yaml
rotation:
  ttl: 30d  # Rotate every 30 days
  
  # Advanced TTL options
  ttl:
    min: 7d      # Minimum time between rotations
    max: 90d     # Maximum age allowed
    target: 30d  # Target rotation interval
    
  # Conditional TTL
  ttl:
    default: 90d
    conditions:
      - if: environment == "production"
        then: 30d
      - if: service_type == "database"
        then: 60d
```

### Scheduling

Configure automated rotation schedules:

```yaml
# Method 1: TTL-based (recommended)
rotation:
  ttl: 30d
  check_interval: 1h  # How often to check if rotation needed

# Method 2: Cron-based
rotation:
  schedule: "0 2 * * SUN"  # Every Sunday at 2 AM
  timezone: "America/New_York"

# Method 3: External trigger
rotation:
  trigger: webhook
  webhook_secret: ${ROTATION_WEBHOOK_SECRET}
```

### Rotation Windows

Define when rotation can occur:

```yaml
rotation:
  ttl: 30d
  windows:
    # Allow rotation only during maintenance windows
    allowed:
      - days: ["Saturday", "Sunday"]
        hours: ["02:00-06:00"]
        timezone: "UTC"
      
    # Blackout periods
    blocked:
      - name: "Holiday Freeze"
        start: "2024-12-15"
        end: "2025-01-05"
      - name: "Black Friday"
        start: "2024-11-25"
        end: "2024-11-30"
```

## Strategy Configuration

### Strategy-Specific Options

Each rotation strategy has unique configuration options:

#### Immediate Strategy
```yaml
rotation:
  strategy: immediate
  verification:
    enabled: true
    timeout: 30s
    retry:
      attempts: 3
      delay: 5s
```

#### Two-Key Strategy
```yaml
rotation:
  strategy: two-key
  slots:
    - name: primary
      label: "key1"
    - name: secondary  
      label: "key2"
  transition:
    propagation_delay: 5m
    cleanup_delay: 1h
  verification:
    both_keys: true  # Verify both keys work
```

#### Overlap Strategy
```yaml
rotation:
  strategy: overlap
  overlap_duration: 24h
  grace_period: 1h  # Extra time before old key removal
  deployment:
    method: blue-green
    health_check:
      endpoint: /health
      interval: 30s
```

#### Gradual Strategy
```yaml
rotation:
  strategy: gradual
  stages:
    - name: canary
      percentage: 10
      duration: 1h
      validation:
        error_threshold: 1%
        rollback_on_failure: true
    
    - name: main
      percentage: 50
      duration: 2h
      
    - name: complete
      percentage: 100
      duration: 30m
```

## Notifications and Monitoring

### Notification Configuration

```yaml
rotation:
  notifications:
    # Pre-rotation notifications
    before:
      - type: email
        recipients: ["ops-team@example.com"]
        lead_time: 7d
        
      - type: slack
        channel: "#rotations"
        lead_time: 24h
        message: |
          📅 Upcoming rotation for {{.Service}}
          Scheduled: {{.ScheduledTime}}
          Strategy: {{.Strategy}}
    
    # Success notifications
    success:
      - type: slack
        channel: "#rotations"
        message: "✅ {{.Service}} rotated successfully"
    
    # Failure notifications
    failure:
      - type: pagerduty
        service_key: ${PAGERDUTY_KEY}
        severity: critical
        
      - type: webhook
        url: https://api.example.com/rotation-failed
        method: POST
        headers:
          Authorization: "Bearer ${WEBHOOK_TOKEN}"
        body: |
          {
            "service": "{{.Service}}",
            "error": "{{.Error}}",
            "time": "{{.Time}}"
          }
```

### Monitoring Integration

```yaml
rotation:
  monitoring:
    # Prometheus metrics
    metrics:
      enabled: true
      port: 9090
      path: /metrics
      
    # OpenTelemetry traces
    tracing:
      enabled: true
      endpoint: otel-collector:4317
      
    # Custom health checks
    health_checks:
      - name: "rotation_lag"
        query: |
          SELECT service, 
                 last_rotation,
                 EXTRACT(EPOCH FROM (NOW() - last_rotation)) as seconds_since
          FROM rotation_status
          WHERE seconds_since > ttl_seconds * 1.1
```

## Advanced Configuration

### Rotation Groups

Coordinate rotation across related services:

```yaml
rotation_groups:
  # Rotate database cluster members together
  database_cluster:
    members:
      - postgres-primary
      - postgres-replica-1
      - postgres-replica-2
    strategy: sequential
    delay_between: 5m
    rollback: all-or-nothing
    
  # Rotate API keys in parallel
  api_services:
    members:
      - payment-api
      - shipping-api
      - inventory-api
    strategy: parallel
    verification:
      required: all  # All must succeed
```

### Conditional Rotation

Rotate based on conditions beyond TTL:

```yaml
rotation:
  conditions:
    # Always check TTL
    - type: ttl
      max_age: 90d
      
    # Security score from external service
    - type: webhook
      url: https://security.example.com/score
      field: "score"
      operator: "<"
      value: 80
      
    # Usage-based rotation
    - type: usage
      metric: api_calls
      threshold: 1000000
      
    # Compliance requirement
    - type: schedule
      cron: "0 0 1 * *"  # First day of month
      description: "Monthly compliance rotation"
```

### Multi-Environment Configuration

Handle different rotation policies per environment:

```yaml
# Base configuration
rotation_defaults: &defaults
  strategy: immediate
  ttl: 90d
  verification:
    enabled: true

envs:
  development:
    DATABASE_PASSWORD:
      service: postgres-dev
      rotation:
        <<: *defaults
        ttl: 180d  # Longer TTL for dev
        
  staging:
    DATABASE_PASSWORD:
      service: postgres-staging
      rotation:
        <<: *defaults
        ttl: 60d
        strategy: overlap
        
  production:
    DATABASE_PASSWORD:
      service: postgres-prod
      rotation:
        <<: *defaults
        ttl: 30d
        strategy: two-key
        approval_required: true
```

### Custom Rotation Logic

Implement custom rotation behavior:

```yaml
services:
  custom-service:
    type: custom
    rotation:
      strategy: script
      script:
        # Inline script
        inline: |
          #!/bin/bash
          echo "Rotating secret for $SERVICE_NAME"
          # Custom logic here
          
        # Or external script
        path: /opt/rotation/custom.py
        interpreter: python3
        
        # Script configuration
        timeout: 300s
        environment:
          - SERVICE_URL=${SERVICE_URL}
          - API_TOKEN=${API_TOKEN}
        
        # Input/output format
        input:
          format: json
          schema:
            type: object
            required: ["old_secret", "service"]
            
        output:
          format: json
          schema:
            type: object
            required: ["success", "new_secret"]
```

## Validation and Testing

### Configuration Validation

```yaml
# Validate rotation configuration
validation:
  pre_rotation:
    - name: "Check service health"
      type: http
      endpoint: "{{.Service.HealthEndpoint}}"
      expected_status: 200
      
    - name: "Verify permissions"
      type: script
      command: "/scripts/check-rotation-perms.sh {{.Service.Name}}"
      
  post_rotation:
    - name: "Test new credentials"
      type: connection_test
      timeout: 30s
      
    - name: "Verify application health"
      type: http
      endpoint: "{{.Service.AppEndpoint}}"
      method: GET
      expected_status: 200
```

### Dry Run Testing

Test rotation without making changes:

```bash
# Test rotation configuration
dsops secrets rotate --env production --dry-run

# Validate specific service
dsops secrets rotate --service postgres-prod --dry-run --verbose

# Test with specific strategy
dsops secrets rotate --env production --key DB_PASSWORD \
  --strategy two-key --dry-run
```

## Security Configuration

### Approval Workflows

Require approval for sensitive rotations:

```yaml
rotation:
  approval:
    required: true
    approvers:
      - team: security
        members: ["alice", "bob"]
        required: 1
        
      - team: platform
        members: ["charlie", "dave"]
        required: 1
        
    timeout: 24h
    method: webhook  # or email, slack
    
    # Auto-approve conditions
    auto_approve:
      - condition: environment != "production"
      - condition: service_type == "development"
```

### Audit Configuration

```yaml
rotation:
  audit:
    # Audit log destination
    destinations:
      - type: file
        path: /var/log/dsops/rotation-audit.log
        format: json
        
      - type: syslog
        host: syslog.example.com
        port: 514
        protocol: tcp
        
      - type: webhook
        url: https://audit.example.com/rotations
        
    # What to audit
    events:
      - rotation_started
      - rotation_completed
      - rotation_failed
      - approval_requested
      - approval_granted
      - rollback_initiated
    
    # Include sensitive data (encrypted)
    include_secrets: false
    
    # Retention
    retention:
      days: 365
      compress: true
```

## Migration and Compatibility

### Migrating from Manual Rotation

```yaml
# Step 1: Import existing rotation schedule
migration:
  import_schedule:
    source: manual_schedule.csv
    format: csv
    mapping:
      service: "Service Name"
      last_rotation: "Last Rotated"
      ttl: "Rotation Interval"

# Step 2: Gradual automation
services:
  legacy-service:
    rotation:
      mode: semi_automated
      require_confirmation: true
      notification:
        - email: ops-team@example.com
        - message: "Please confirm rotation for {{.Service}}"
```

### Backward Compatibility

```yaml
# Support both old and new formats
compatibility:
  # Legacy format support
  enable_v0_format: true
  
  # Migration warnings
  warnings:
    deprecated_fields: true
    
  # Auto-upgrade configuration
  auto_upgrade:
    enabled: true
    backup: true
    backup_path: .dsops/backups/
```

## Best Practices

1. **Start Simple**: Begin with immediate rotation in dev, add complexity as needed
2. **Test Thoroughly**: Always dry-run in staging before production
3. **Monitor Everything**: Set up alerts for failed rotations
4. **Document Decisions**: Include comments explaining strategy choices
5. **Plan for Failure**: Configure rollback and notification strategies
6. **Regular Reviews**: Audit rotation policies quarterly

## Related Documentation

- [Rotation Architecture](/rotation/architecture/) - Technical details
- [Rotation Strategies](/rotation/strategies/) - Strategy deep dive
- [Rotation Commands](/rotation/commands/) - CLI reference
- [dsops.yaml Reference](/reference/configuration/) - Full config options