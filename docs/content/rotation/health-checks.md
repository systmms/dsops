---
title: "Health Monitoring"
description: "Monitor service health after rotation with built-in checks and custom scripts"
lead: "Ensure rotation success with continuous health monitoring. Detect issues early with protocol-specific checks, run custom validation scripts, and automatically rollback on health failures."
date: 2025-12-09T00:00:00-00:00
lastmod: 2025-12-09T00:00:00-00:00
draft: false
weight: 35
---

## Overview

Health monitoring runs after successful rotation to detect issues that verification might miss:

- **Protocol-specific checks**: SQL query latency, HTTP error rates, connection pool monitoring
- **Custom validation scripts**: Application-specific health checks
- **Automatic rollback**: Trigger rollback when health thresholds are exceeded
- **Prometheus metrics**: Export health metrics for dashboards and alerts

**Why Health Monitoring?**

Verification (connection tests) proves credentials work **at rotation time**, but health monitoring detects **delayed issues**:
- Connection pool exhaustion
- Cache invalidation failures
- Performance degradation
- Subtle configuration issues

## Health Check Configuration

### Basic Setup

```yaml
services:
  postgres-prod:
    rotation:
      health_checks:
        enabled: true
        monitoring_period: 10m  # Monitor for 10 minutes
        interval: 30s  # Check every 30 seconds
        failure_threshold: 3  # 3 consecutive failures triggers rollback

        checks:
          - type: connection  # Built-in connection test
          - type: query_latency
            threshold_ms: 500  # Alert if queries >500ms
```

### Configuration Options

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | bool | No | false | Enable health monitoring |
| `monitoring_period` | duration | No | `10m` | How long to monitor |
| `interval` | duration | No | `30s` | Check frequency |
| `failure_threshold` | int | No | 3 | Consecutive failures before rollback |
| `checks` | []Check | Yes | - | List of health checks |
| `custom_scripts` | []Script | No | - | Custom validation scripts |

## Built-In Health Checks

### SQL Database Checks

For PostgreSQL, MySQL, and other SQL databases:

#### Connection Test

```yaml
checks:
  - type: connection
    name: "db-connection"
```

Validates:
- Database is reachable
- New credentials work
- Connection can be established

#### Query Latency

```yaml
checks:
  - type: query_latency
    name: "query-performance"
    threshold_ms: 500  # P95 latency threshold
    query: "SELECT COUNT(*) FROM users"  # Optional custom query
```

Validates:
- Query execution time within threshold
- No performance degradation
- Database responding quickly

**Metrics tracked**: P50, P95, P99 latency

#### Connection Pool Monitoring

```yaml
checks:
  - type: connection_pool
    name: "pool-exhaustion"
    max_connections: 100
    warning_threshold_pct: 80  # Warn at >80% usage
```

Validates:
- Connection pool not exhausted
- Healthy connection reuse
- No connection leaks

#### Active Transactions

```yaml
checks:
  - type: active_transactions
    name: "transaction-count"
    max_transactions: 50
```

Validates:
- Transaction count within limits
- No hung transactions
- Normal database activity

#### Replication Lag (for replicas)

```yaml
checks:
  - type: replication_lag
    name: "replica-lag"
    max_lag_seconds: 10
```

Validates:
- Replica is synced with primary
- Replication not falling behind
- Data consistency across instances

**See**: [`examples/health-checks/sql-health.yaml`](/examples/health-checks/sql-health.yaml)

### HTTP API Checks

For Stripe, GitHub, and other HTTP APIs:

#### Response Time

```yaml
checks:
  - type: response_time
    name: "api-latency"
    endpoint: "/v1/ping"  # Optional test endpoint
    p50_threshold_ms: 200  # Median response time
    p95_threshold_ms: 500  # 95th percentile
    p99_threshold_ms: 1000  # 99th percentile
```

Validates:
- API responding within latency budgets
- No performance degradation
- Consistent response times

#### Error Rate

```yaml
checks:
  - type: error_rate
    name: "api-errors"
    max_error_rate_pct: 5  # Alert if >5% errors
    window: 5m  # Calculate rate over 5 minutes
    error_codes: [400, 401, 403, 404, 500, 502, 503, 504]
```

Validates:
- Error rate within acceptable limits
- Credentials working correctly
- No authentication/authorization issues

#### Rate Limit Monitoring

```yaml
checks:
  - type: rate_limit
    name: "rate-limits"
    check_headers: true
    warning_threshold_pct: 20  # Warn if <20% quota remaining
```

Validates:
- API quota not exhausted
- Rate limits respected
- Sufficient remaining quota

#### API Quota

```yaml
checks:
  - type: api_quota
    name: "quota-remaining"
    warning_threshold_pct: 30  # Warn if <30% quota
```

Validates:
- Sufficient API quota available
- No quota exhaustion
- Normal API usage patterns

**See**: [`examples/health-checks/http-health.yaml`](/examples/health-checks/http-health.yaml)

### NoSQL Database Checks

For MongoDB, Redis, and other NoSQL systems:

#### Connection Test

```yaml
checks:
  - type: connection
    name: "redis-connection"
```

#### Command Latency

```yaml
checks:
  - type: command_latency
    name: "redis-performance"
    threshold_ms: 100
    command: "PING"  # Optional test command
```

#### Memory Usage (Redis)

```yaml
checks:
  - type: memory_usage
    name: "redis-memory"
    max_memory_pct: 80  # Alert if >80% memory used
```

## Custom Health Scripts

Run custom validation scripts for application-specific checks.

### Basic Script Configuration

```yaml
health_checks:
  custom_scripts:
    - name: "cache-validation"
      script: "/scripts/health/check-cache-warm.sh"
      timeout: 60s
      environment:
        DATABASE_URL: "{{.NewConnectionString}}"
        SERVICE_NAME: "{{.ServiceName}}"
      retry:
        max_attempts: 3
        backoff: 5s
```

### Script Environment Variables

dsops automatically injects these variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `DSOPS_SERVICE_NAME` | Service name | `postgres-prod` |
| `DSOPS_NEW_VERSION` | New secret version | `2025-12-09-1430` |
| `DSOPS_OLD_VERSION` | Previous version | `2025-12-01-1015` |
| `DSOPS_ENVIRONMENT` | Environment | `production` |
| `DSOPS_STRATEGY` | Rotation strategy | `two-key` |

Plus any custom variables from `environment:` config.

### Example Health Scripts

#### Cache Warming Validation

```bash
#!/bin/bash
# /scripts/health/check-cache-warm.sh

# Check cache table is accessible
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM cache_table" > /dev/null
if [ $? -ne 0 ]; then
    echo "ERROR: Cache table not accessible"
    exit 1
fi

# Verify cache is warmed
CACHE_SIZE=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM cache_table")
if [ "$CACHE_SIZE" -lt 1000 ]; then
    echo "ERROR: Cache not warmed (size: $CACHE_SIZE, required: 1000)"
    exit 1
fi

echo "SUCCESS: Cache warmed with $CACHE_SIZE entries"
exit 0
```

#### Application Query Validation

```bash
#!/bin/bash
# /scripts/health/validate-app-queries.sh

# Test critical application queries
psql "$DATABASE_URL" -t -c "SELECT id FROM users WHERE id = $TEST_USER_ID" > /dev/null
if [ $? -ne 0 ]; then
    echo "ERROR: User lookup query failed"
    exit 1
fi

psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM orders WHERE created_at > NOW() - INTERVAL '1 day'" > /dev/null
if [ $? -ne 0 ]; then
    echo "ERROR: Orders query failed"
    exit 1
fi

echo "SUCCESS: Application queries validated"
exit 0
```

#### Replication Status Check

```bash
#!/bin/bash
# /scripts/health/check-replication.sh

# Check replication lag
LAG=$(psql "$DATABASE_URL" -t -c "SELECT EXTRACT(EPOCH FROM (NOW() - pg_last_xact_replay_timestamp()))::INT")

if [ "$LAG" -gt "$MAX_LAG_SECONDS" ]; then
    echo "ERROR: Replication lag too high: ${LAG}s (max: ${MAX_LAG_SECONDS}s)"
    exit 1
fi

echo "SUCCESS: Replication lag ${LAG}s (within threshold)"
exit 0
```

### Script Configuration Options

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Script identifier |
| `script` | string | Yes | - | Path to script executable |
| `timeout` | duration | No | `60s` | Script execution timeout |
| `environment` | map | No | - | Environment variables |
| `retry.max_attempts` | int | No | 1 | Retry attempts on failure |
| `retry.backoff` | duration | No | `5s` | Backoff between retries |

### Script Exit Codes

- **0**: Health check passed
- **Non-zero**: Health check failed (triggers failure counter)

**See**: [`examples/health-checks/custom-script.yaml`](/examples/health-checks/custom-script.yaml)

## Automatic Rollback Integration

Health checks can trigger automatic rollback when thresholds are exceeded.

### Configuration

```yaml
rotation:
  health_checks:
    enabled: true
    monitoring_period: 10m
    interval: 30s
    failure_threshold: 3  # 3 consecutive failures

  rollback:
    automatic: true
    on_health_check_failure: true  # Rollback on health failures
    timeout: 30s
```

### Rollback Workflow

1. **Health check runs** every `interval` (e.g., 30s)
2. **Failure detected** (check fails or script exits non-zero)
3. **Failure counter increments** (consecutive failures tracked)
4. **Threshold reached** (e.g., 3 consecutive failures)
5. **Automatic rollback triggered** (previous secret restored)
6. **Rollback verification** (health checks run with old secret)
7. **Notifications sent** (Slack, email, PagerDuty)

### Failure Threshold Reset

The failure counter **resets** when:
- Health check passes (consecutive failures back to 0)
- Monitoring period ends
- New rotation starts

## Prometheus Metrics

Export rotation and health metrics for monitoring.

### Enable Metrics

```yaml
rotation:
  metrics:
    enabled: true
    port: 9090
    path: /metrics
    labels:
      environment: "production"
      team: "platform"
```

### Available Metrics

#### Rotation Metrics

```prometheus
# Rotation lifecycle counters
dsops_rotation_started_total{service="postgres-prod",environment="production",strategy="two-key"} 42
dsops_rotation_completed_total{service="postgres-prod",environment="production",status="success"} 40
dsops_rotation_completed_total{service="postgres-prod",environment="production",status="failure"} 2

# Rotation duration histogram
dsops_rotation_duration_seconds_bucket{service="postgres-prod",le="10"} 15
dsops_rotation_duration_seconds_bucket{service="postgres-prod",le="30"} 35
dsops_rotation_duration_seconds_bucket{service="postgres-prod",le="60"} 40
dsops_rotation_duration_seconds_sum{service="postgres-prod"} 1250.5
dsops_rotation_duration_seconds_count{service="postgres-prod"} 42
```

#### Health Check Metrics

```prometheus
# Health check status gauge (1=healthy, 0=unhealthy)
dsops_health_check_status{service="postgres-prod",check_type="connection"} 1
dsops_health_check_status{service="postgres-prod",check_type="query_latency"} 1

# Rollback counters
dsops_rollback_total{service="postgres-prod",type="automatic"} 2
dsops_rollback_total{service="postgres-prod",type="manual"} 1

# Verification duration
dsops_verification_duration_seconds_bucket{service="postgres-prod",le="1"} 38
dsops_verification_duration_seconds_bucket{service="postgres-prod",le="5"} 42
```

### Grafana Dashboard

Query examples for Grafana:

**Rotation Success Rate**:
```promql
rate(dsops_rotation_completed_total{status="success"}[5m])
/ rate(dsops_rotation_started_total[5m])
```

**P95 Rotation Duration**:
```promql
histogram_quantile(0.95, rate(dsops_rotation_duration_seconds_bucket[5m]))
```

**Current Health Status**:
```promql
dsops_health_check_status{service="postgres-prod"}
```

## Combined Health Monitoring

Combine multiple health check types for comprehensive coverage.

```yaml
services:
  api-backend:
    rotation:
      health_checks:
        enabled: true
        monitoring_period: 15m
        interval: 30s
        failure_threshold: 3

        # Built-in checks
        checks:
          - type: connection
          - type: query_latency
            threshold_ms: 500
          - type: connection_pool
            max_connections: 100
          - type: http_endpoint
            url: "https://api.example.com/health"
            expected_status: 200

        # Custom validation
        custom_scripts:
          - name: "end-to-end-test"
            script: "/scripts/health/e2e-api-test.sh"
            timeout: 120s

          - name: "data-consistency"
            script: "/scripts/health/check-consistency.sh"
            timeout: 60s

      # Automatic rollback on health failure
      rollback:
        automatic: true
        on_health_check_failure: true

      # Multi-channel notifications
      notifications:
        slack:
          events: [failed, rollback]
        pagerduty:
          events: [failed, rollback]
```

**See**: [`examples/health-checks/combined.yaml`](/examples/health-checks/combined.yaml)

## Best Practices

### Monitoring Period

- **Databases**: 5-10 minutes (detect connection pool issues)
- **APIs**: 10-15 minutes (detect rate limiting, quota issues)
- **Critical services**: 15+ minutes (extended monitoring)

```yaml
monitoring_period: 10m  # Standard
monitoring_period: 15m  # Extended for critical services
```

### Check Interval

- **Fast checks** (connection): 30s interval
- **Expensive checks** (complex queries): 60s interval

```yaml
interval: 30s  # Standard connection tests
interval: 60s  # Complex validation queries
```

### Failure Threshold

- **Conservative**: Higher threshold (3-5 failures)
- **Aggressive**: Lower threshold (2-3 failures)

```yaml
failure_threshold: 3  # Standard - allows transient issues
failure_threshold: 2  # Aggressive - faster rollback
```

### Custom Scripts

1. **Keep scripts fast** - Timeout quickly to avoid blocking
2. **Test scripts independently** - Run manually before deploying
3. **Use exit codes correctly** - 0 = success, non-zero = failure
4. **Log clearly** - Output helps debugging failures
5. **Avoid secrets in logs** - Don't echo credentials

## Troubleshooting

### Health checks always failing

**Check**:
1. Health check thresholds are reasonable
2. Credentials are correct for new secret
3. Service is actually healthy (not rotation issue)
4. Network connectivity from dsops to service

**Debug**:
```bash
# Run health check script manually
DATABASE_URL="new-connection-string" /scripts/health/check-script.sh

# Check service independently
psql "$DATABASE_URL" -c "SELECT 1"
```

### False positives (healthy service marked unhealthy)

**Solutions**:
1. Increase `failure_threshold` (allow more transient failures)
2. Increase `interval` (check less frequently)
3. Adjust check thresholds (relax latency/error rate limits)
4. Add retry logic to custom scripts

### Health checks not running

**Check**:
1. `health_checks.enabled: true` is set
2. `monitoring_period` is not zero
3. `checks` list is not empty
4. Rotation completed successfully (checks only run after success)

### Rollback not triggering on health failure

**Check**:
1. `rollback.automatic: true` is set
2. `rollback.on_health_check_failure: true` is set
3. Failure threshold was actually reached
4. Check rollback logs: `dsops rotation history --service <name>`

### Script timeouts

**Solutions**:
1. Increase `timeout` value
2. Optimize script (make it faster)
3. Reduce validation scope
4. Run expensive checks less frequently

```yaml
custom_scripts:
  - name: "slow-validation"
    script: "/scripts/slow-check.sh"
    timeout: 120s  # Increased from default 60s
```

## Configuration Reference

### Health Checks

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `enabled` | bool | No | false | Enable health monitoring |
| `monitoring_period` | duration | No | `10m` | Monitoring duration |
| `interval` | duration | No | `30s` | Check frequency |
| `failure_threshold` | int | No | 3 | Consecutive failures before rollback |
| `checks` | []Check | No | [] | Built-in health checks |
| `custom_scripts` | []Script | No | [] | Custom scripts |

### SQL Checks

| Type | Fields | Description |
|------|--------|-------------|
| `connection` | - | Basic connection test |
| `query_latency` | `threshold_ms`, `query` | Query performance |
| `connection_pool` | `max_connections`, `warning_threshold_pct` | Pool monitoring |
| `active_transactions` | `max_transactions` | Transaction count |
| `replication_lag` | `max_lag_seconds` | Replica sync status |

### HTTP API Checks

| Type | Fields | Description |
|------|--------|-------------|
| `response_time` | `p50_threshold_ms`, `p95_threshold_ms`, `p99_threshold_ms` | Response latency |
| `error_rate` | `max_error_rate_pct`, `window`, `error_codes` | Error rate monitoring |
| `rate_limit` | `warning_threshold_pct` | Rate limit status |
| `api_quota` | `warning_threshold_pct` | API quota remaining |

### Custom Scripts

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Script identifier |
| `script` | string | Yes | - | Script path |
| `timeout` | duration | No | `60s` | Execution timeout |
| `environment` | map | No | - | Environment variables |
| `retry.max_attempts` | int | No | 1 | Retry attempts |
| `retry.backoff` | duration | No | `5s` | Retry backoff |

## Related Documentation

- [Rotation Configuration](/docs/rotation/configuration) - General rotation setup
- [Notifications](/docs/rotation/notifications) - Health failure alerts
- [Rollback & Recovery](/docs/rotation/rollback) - Automatic rollback setup
- [Gradual Rollout](/docs/rotation/gradual-rollout) - Health checks per wave

## Examples

Complete working examples:
- [`examples/health-checks/sql-health.yaml`](/examples/health-checks/sql-health.yaml)
- [`examples/health-checks/http-health.yaml`](/examples/health-checks/http-health.yaml)
- [`examples/health-checks/custom-script.yaml`](/examples/health-checks/custom-script.yaml)
- [`examples/health-checks/combined.yaml`](/examples/health-checks/combined.yaml)
