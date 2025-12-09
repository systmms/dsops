---
title: "Rotation Notifications"
description: "Configure notifications for secret rotation events via Slack, email, PagerDuty, and webhooks"
lead: "Stay informed about rotation events with multi-channel notifications. Get instant alerts for rotation failures, track progress in Slack, maintain email audit trails, and create PagerDuty incidents for critical issues."
date: 2025-12-09T00:00:00-00:00
lastmod: 2025-12-09T00:00:00-00:00
draft: false
weight: 30
---

## Overview

dsops supports multi-channel notifications for rotation events, enabling teams to:

- **Track rotation progress** in real-time via Slack
- **Maintain audit trails** with email notifications
- **Alert on-call teams** through PagerDuty incidents
- **Integrate with custom systems** using generic webhooks

Notifications are **best-effort** and **non-blocking** - rotation continues even if notification delivery fails, ensuring that notification problems never block secret rotation.

## Notification Events

Notifications can be triggered for the following rotation lifecycle events:

| Event | Description | When Triggered |
|-------|-------------|----------------|
| `started` | Rotation begins | When `dsops rotation rotate` starts |
| `completed` | Rotation succeeds | After verification passes |
| `failed` | Rotation fails | On any error during rotation |
| `rollback` | Rollback occurs | After automatic or manual rollback |

## Notification Providers

### Slack

Send rotation events to Slack channels using incoming webhooks.

**Configuration**:

```yaml
services:
  postgres-prod:
    rotation:
      notifications:
        slack:
          webhook_url:
            from:
              store: vault
              key: notifications/slack/webhook
          channel: "#ops-alerts"
          events: [started, completed, failed, rollback]
          mentions:
            on_failure: ["@oncall", "@sre-team"]
          message_prefix: "[PROD]"  # Optional
```

**Slack Message Format**:

Success:
```
✅ [PROD] Rotation Completed: postgres-prod

Environment: production
Strategy: two-key
Duration: 45s
New version: 2025-12-09-1430
Previous version: marked for deletion (grace period: 24h)

Initiated by: automation (schedule)
```

Failure:
```
❌ [PROD] Rotation Failed: postgres-prod

Environment: production
Error: Connection verification failed
Suggestion: Check database connectivity and credentials
Rollback: Automatic rollback initiated

@oncall @sre-team
```

**Configuration Options**:

- `webhook_url`: Slack incoming webhook URL (store securely)
- `channel`: Target Slack channel (e.g., `#ops-alerts`)
- `events`: List of events to notify about
- `mentions`: Users/groups to mention on specific events
- `message_prefix`: Optional prefix for all messages

**See**: [`examples/notifications/slack.yaml`](/examples/notifications/slack.yaml)

### Email (SMTP)

Send rotation notifications via email for audit trails and stakeholder updates.

**Configuration**:

```yaml
services:
  postgres-prod:
    rotation:
      notifications:
        email:
          smtp:
            host: smtp.example.com
            port: 587
            username:
              from:
                store: vault
                key: smtp/username
            password:
              from:
                store: vault
                key: smtp/password
            tls: true  # Enable TLS
          from: "dsops-rotation@example.com"
          to:
            - "ops-team@example.com"
            - "security@example.com"
          cc: ["manager@example.com"]  # Optional
          events: [completed, failed, rollback]
          batch_mode: immediate  # or hourly, daily
          priority:
            on_failure: high
            on_success: normal
```

**Email Format**:

```
Subject: [dsops] Rotation Completed: postgres-prod (production)

Service: postgres-prod
Environment: production
Status: ✅ Completed
Duration: 45 seconds
Strategy: two-key
Timestamp: 2025-12-09 14:30:00 UTC

New Secret Version: 2025-12-09-1430
Previous Secret: Marked for deletion (grace period: 24 hours)

Audit Trail: Run `dsops rotation history --service postgres-prod` for details
```

**Batching Modes**:

- `immediate`: Send email immediately for each event (default)
- `hourly`: Batch events into hourly digest
- `daily`: Batch events into daily digest

**Configuration Options**:

- `smtp`: SMTP server configuration
  - `host`: SMTP server hostname
  - `port`: SMTP server port (587 for STARTTLS, 465 for TLS)
  - `username`: SMTP username (can reference secret store)
  - `password`: SMTP password (should reference secret store)
  - `tls`: Enable TLS encryption
  - `starttls`: Enable STARTTLS (alternative to TLS)
- `from`: Sender email address
- `to`: List of recipient email addresses
- `cc`: Optional CC recipients
- `bcc`: Optional BCC recipients
- `events`: List of events to notify about
- `batch_mode`: Email batching strategy
- `priority`: Email priority levels

**See**: [`examples/notifications/email.yaml`](/examples/notifications/email.yaml)

### PagerDuty

Create PagerDuty incidents for rotation failures and critical events.

**Configuration**:

```yaml
services:
  database-primary:
    rotation:
      notifications:
        pagerduty:
          integration_key:
            from:
              store: aws
              key: notifications/pagerduty/integration-key
          service_id: PXXXXXX
          severity: critical  # critical, error, warning, info
          events: [failed, rollback]  # Only alert on failures
          auto_resolve: true  # Resolve incident on successful retry
          custom_details:
            team: "platform-engineering"
            environment: "production"
            runbook: "https://wiki.example.com/runbooks/rotation-failure"
```

**PagerDuty Incident Details**:

```
Title: dsops rotation failed: postgres-prod (production)
Severity: critical
Source: dsops-rotation

Custom Details:
  service: postgres-prod
  environment: production
  strategy: two-key
  error: Connection verification failed
  timestamp: 2025-12-09T14:30:00Z
  team: platform-engineering
  runbook: https://wiki.example.com/runbooks/rotation-failure
```

**Auto-Resolution**:

When `auto_resolve: true`, dsops automatically resolves PagerDuty incidents when:
- Rotation succeeds after a retry
- Manual rollback completes successfully

**Configuration Options**:

- `integration_key`: PagerDuty Events API integration key (store securely)
- `service_id`: PagerDuty service ID
- `severity`: Incident severity (`critical`, `error`, `warning`, `info`)
- `events`: List of events to create incidents for
- `auto_resolve`: Automatically resolve incidents on success
- `custom_details`: Additional context for incidents

**See**: [`examples/notifications/pagerduty.yaml`](/examples/notifications/pagerduty.yaml)

### Generic Webhooks

Send rotation events to custom HTTP endpoints using templated payloads.

**Configuration**:

```yaml
services:
  api-service:
    rotation:
      notifications:
        webhooks:
          - name: "monitoring-system"
            url: "https://monitoring.example.com/api/events"
            method: POST
            headers:
              Authorization: "Bearer {{.Token}}"
              X-API-Key:
                from:
                  store: vault
                  key: monitoring/api-key
            events: [started, completed, failed, rollback]
            payload_template: |
              {
                "event_type": "{{.EventType}}",
                "service": "{{.ServiceName}}",
                "environment": "{{.Environment}}",
                "status": "{{.Status}}",
                "duration_seconds": {{.Duration}},
                "timestamp": "{{.Timestamp}}",
                "rotation_id": "{{.RotationID}}"
              }
            retry:
              max_attempts: 3
              backoff: exponential
            timeout: 10s
```

**Template Variables**:

Available variables for `payload_template`:

| Variable | Type | Description | Example |
|----------|------|-------------|---------|
| `{{.EventType}}` | string | Event type | `completed` |
| `{{.ServiceName}}` | string | Service name | `postgres-prod` |
| `{{.Environment}}` | string | Environment | `production` |
| `{{.Status}}` | string | Rotation status | `success` |
| `{{.Strategy}}` | string | Rotation strategy | `two-key` |
| `{{.Duration}}` | number | Duration in seconds | `45` |
| `{{.Timestamp}}` | string | ISO 8601 timestamp | `2025-12-09T14:30:00Z` |
| `{{.RotationID}}` | string | Unique rotation ID | `rot-2025-12-09-abc123` |
| `{{.Error}}` | string | Error message (if failed) | `Connection timeout` |

**Retry Logic**:

Webhooks support automatic retry with exponential backoff:
- `max_attempts`: Number of retry attempts (default: 3)
- `backoff`: Backoff strategy (`exponential` or `linear`)

**Configuration Options**:

- `name`: Webhook identifier
- `url`: Target webhook URL
- `method`: HTTP method (`POST`, `PUT`, `PATCH`)
- `headers`: HTTP headers (can reference secret stores)
- `events`: List of events to send
- `payload_template`: Go template for JSON payload
- `retry`: Retry configuration
- `timeout`: Request timeout

**See**: [`examples/notifications/webhook.yaml`](/examples/notifications/webhook.yaml)

## Multi-Channel Notifications

Combine multiple notification channels for comprehensive coverage:

```yaml
services:
  critical-database:
    rotation:
      notifications:
        # Real-time team visibility
        slack:
          webhook_url: { from: { store: vault, key: slack/webhook } }
          channel: "#database-ops"
          events: [started, completed, failed, rollback]

        # Audit trail for compliance
        email:
          smtp: { host: smtp.example.com, port: 587, tls: true }
          from: "db-rotation@example.com"
          to: ["dba-team@example.com", "security@example.com"]
          events: [completed, failed, rollback]

        # On-call alerts for failures
        pagerduty:
          integration_key: { from: { store: vault, key: pagerduty/key } }
          service_id: PXXXXXX
          severity: critical
          events: [failed, rollback]  # Only page for problems

        # Custom monitoring integration
        webhooks:
          - name: "datadog"
            url: "https://api.datadoghq.com/api/v1/events"
            events: [started, completed, failed, rollback]
```

**See**: [`examples/notifications/multi-channel.yaml`](/examples/notifications/multi-channel.yaml)

## Best Practices

### Security

1. **Store webhook URLs and API keys securely**:
   ```yaml
   webhook_url:
     from:
       store: vault
       key: notifications/slack/webhook
   ```

2. **Never hardcode credentials** in configuration files

3. **Use TLS/STARTTLS** for email notifications

4. **Validate webhook endpoints** before production use

### Event Selection

1. **Slack**: All events for team visibility
   ```yaml
   events: [started, completed, failed, rollback]
   ```

2. **Email**: Only important events to reduce noise
   ```yaml
   events: [completed, failed, rollback]  # Skip "started"
   ```

3. **PagerDuty**: Only failures to avoid alert fatigue
   ```yaml
   events: [failed, rollback]
   ```

4. **Webhooks**: All events for monitoring systems
   ```yaml
   events: [started, completed, failed, rollback]
   ```

### Failure Handling

Notifications are **best-effort** and **non-blocking**:

- ✅ Rotation continues if notification fails
- ✅ Errors logged but don't abort rotation
- ✅ Retry logic for transient failures (webhooks)
- ⚠️ No notification delivery guarantees

**Example**: Slack webhook down → Rotation completes successfully, Slack notification skipped

## Troubleshooting

### Slack notifications not appearing

**Check**:
1. Webhook URL is correct and accessible
2. Channel name includes `#` prefix
3. dsops has network access to Slack API
4. Webhook is not rate-limited

**Test**:
```bash
# Test webhook manually
curl -X POST "https://hooks.slack.com/services/YOUR/WEBHOOK/URL" \
  -H "Content-Type: application/json" \
  -d '{"text": "Test from dsops"}'
```

### Email delivery failures

**Check**:
1. SMTP credentials are correct
2. SMTP server hostname/port are correct
3. TLS/STARTTLS configuration matches server requirements
4. Firewall allows outbound SMTP connections

**Test**:
```bash
# Test SMTP connection
openssl s_client -connect smtp.example.com:587 -starttls smtp
```

### PagerDuty incidents not created

**Check**:
1. Integration key is correct (Events API v2)
2. Service ID matches PagerDuty service
3. dsops has network access to PagerDuty API
4. Events are configured correctly (`events: [failed, rollback]`)

**Test**:
```bash
# Test PagerDuty Events API
curl -X POST "https://events.pagerduty.com/v2/enqueue" \
  -H "Content-Type: application/json" \
  -d '{
    "routing_key": "YOUR_INTEGRATION_KEY",
    "event_action": "trigger",
    "payload": {
      "summary": "Test incident from dsops",
      "severity": "error",
      "source": "dsops-test"
    }
  }'
```

### Webhook timeouts

**Check**:
1. Webhook endpoint is reachable
2. Endpoint responds within timeout (default 10s)
3. Network latency is acceptable

**Increase timeout**:
```yaml
webhooks:
  - name: "slow-endpoint"
    url: "https://slow-api.example.com/events"
    timeout: 30s  # Increase from default 10s
```

## Configuration Reference

### Slack

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `webhook_url` | string/ref | Yes | - | Slack incoming webhook URL |
| `channel` | string | No | - | Target channel (e.g., `#ops`) |
| `events` | []string | Yes | - | Events to notify about |
| `mentions` | map | No | - | User/group mentions by event |
| `message_prefix` | string | No | - | Prefix for all messages |

### Email

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `smtp.host` | string | Yes | - | SMTP server hostname |
| `smtp.port` | int | Yes | - | SMTP server port |
| `smtp.username` | string/ref | No | - | SMTP username |
| `smtp.password` | string/ref | No | - | SMTP password |
| `smtp.tls` | bool | No | false | Enable TLS |
| `smtp.starttls` | bool | No | false | Enable STARTTLS |
| `from` | string | Yes | - | Sender email address |
| `to` | []string | Yes | - | Recipient email addresses |
| `cc` | []string | No | - | CC recipients |
| `bcc` | []string | No | - | BCC recipients |
| `events` | []string | Yes | - | Events to notify about |
| `batch_mode` | string | No | `immediate` | Batching mode |
| `priority` | map | No | - | Priority by event type |

### PagerDuty

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `integration_key` | string/ref | Yes | - | PagerDuty Events API key |
| `service_id` | string | Yes | - | PagerDuty service ID |
| `severity` | string | No | `error` | Incident severity |
| `events` | []string | Yes | - | Events to create incidents for |
| `auto_resolve` | bool | No | false | Auto-resolve on success |
| `custom_details` | map | No | - | Additional incident context |

### Webhooks

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Webhook identifier |
| `url` | string | Yes | - | Target webhook URL |
| `method` | string | No | `POST` | HTTP method |
| `headers` | map | No | - | HTTP headers |
| `events` | []string | Yes | - | Events to send |
| `payload_template` | string | Yes | - | Go template for payload |
| `retry.max_attempts` | int | No | 3 | Max retry attempts |
| `retry.backoff` | string | No | `exponential` | Backoff strategy |
| `timeout` | duration | No | `10s` | Request timeout |

## Related Documentation

- [Rotation Configuration](/docs/rotation/configuration) - General rotation setup
- [Rollback & Recovery](/docs/rotation/rollback) - Rollback notifications
- [Health Checks](/docs/rotation/health-checks) - Health monitoring alerts
- [Gradual Rollout](/docs/rotation/gradual-rollout) - Wave progress notifications

## Examples

Complete working examples:
- [`examples/notifications/slack.yaml`](/examples/notifications/slack.yaml)
- [`examples/notifications/email.yaml`](/examples/notifications/email.yaml)
- [`examples/notifications/pagerduty.yaml`](/examples/notifications/pagerduty.yaml)
- [`examples/notifications/webhook.yaml`](/examples/notifications/webhook.yaml)
- [`examples/notifications/multi-channel.yaml`](/examples/notifications/multi-channel.yaml)
