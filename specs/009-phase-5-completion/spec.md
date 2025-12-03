# SPEC-009: Rotation Phase 5 Completion

**Status**: In Progress
**Feature Branch**: 009-phase-5-completion
**Target Milestone**: v0.2
**Related**:
- VISION_ROTATE.md Section 4 (Advanced Features)
- VISION_ROTATE_IMPLEMENTATION.md Phase 5 (Advanced Features)
- VISION.md Section 8 (Secret Rotation)

## Clarifications

### Session 2025-12-03

- Q: What notification delivery model should be used? → A: Async with bounded queue (background goroutine, drop oldest if full)
- Q: How should concurrent rotation requests be handled? → A: Interactive prompt with rotation details + choice (reject/queue/replace/concurrent); `--on-conflict` flag for automation with `reject` as default
- Q: Should health check failure threshold reset between rotations? → A: Per-rotation scope (counter resets at start of each rotation's health monitoring period)
- Q: What should the notification queue bounds be? → A: 100 events, emit `dsops_notifications_dropped_total` Prometheus counter on drop
- Q: How should gradual rollout discover service instances? → A: Pluggable with 4 providers: explicit (default), kubernetes, cloud, endpoint (configurable per service)

## Summary

Complete the remaining Phase 5 (Advanced Features) rotation capabilities to bring rotation from 38% (6/16 features) to 100% completion. This epic consolidates 13 incomplete features across four critical categories: Notifications (0% complete), Rollback & Recovery (25% complete), Verification & Health (25% complete), and Gradual Rollout (25% complete).

**Current State**: Basic rotation works (two-key, immediate, overlap strategies) with audit trails
**Target State**: Production-ready rotation with notifications, rollback, health monitoring, and gradual deployment strategies
**Impact**: Enables safe, observable, and controllable secret rotation for enterprise production environments

## User Stories

### Epic Context: Phase 5 Features

Phase 5 builds on the complete Phase 1-4 foundation:
- ✅ **Phase 1**: Core rotation engine (100%)
- ✅ **Phase 2**: Data-driven architecture (83% - minor gaps acceptable)
- ✅ **Phase 3**: Service integration via dsops-data (100%)
- ✅ **Phase 4**: Data coverage (100% - 84+ service definitions)
- 🟡 **Phase 5**: Advanced features **(38% - THIS SPEC)**

Phase 5 unlocks production readiness by adding observability (notifications), safety (rollback), reliability (health checks), and risk mitigation (gradual rollout).

---

## Category 1: Notifications (0% → 100%) - **HIGHEST PRIORITY**

**Why Critical**: Users cannot trust rotation in production without knowing when it happens, whether it succeeds, and what to do on failure. Notifications are the foundation for all other observability.

### User Story 1.1: Slack Integration for Rotation Events (P0)

**As a** DevOps engineer, **I want** rotation events sent to Slack channels, **so that** my team is immediately aware of rotation activity and can respond to failures.

**Why this priority**: Slack is the most common team communication platform. Without Slack integration, teams resort to polling logs or missing rotation failures entirely.

**Acceptance Criteria**:
1. **Given** rotation is configured with Slack webhook, **When** rotation starts, **Then** Slack message posted with service name, environment, and start time
2. **Given** rotation completes successfully, **When** audit entry is saved, **Then** Slack message posted with success status and duration
3. **Given** rotation fails, **When** error occurs, **Then** Slack message posted with failure reason and suggested remediation
4. **Given** rollback occurs, **When** rollback completes, **Then** Slack message posted with rollback details
5. **Given** Slack webhook is unreachable, **When** notification fails, **Then** rotation continues (notifications are best-effort, not critical path)

**Configuration Example**:
```yaml
services:
  postgres-prod:
    type: postgresql
    rotation:
      notifications:
        slack:
          webhook_url: store://vault/slack/webhook
          channel: "#ops-alerts"
          events: [started, completed, failed, rollback]
          mentions:
            on_failure: ["@oncall"]
```

**Slack Message Format**:
```
🔄 Rotation Started: postgres-prod
Environment: production
Strategy: two-key
Initiated by: automation (schedule)
Time: 2025-08-26 14:30:00 UTC

✅ Rotation Completed: postgres-prod
Duration: 45s
New secret version: 2025-08-26-1430
Previous secret: marked for deletion (grace period: 24h)

❌ Rotation Failed: postgres-prod
Error: Connection verification failed
Suggestion: Check database connectivity and credentials
Rollback: Automatic rollback initiated
```

**Implementation Notes**:
- Use `net/http` for webhook POST requests
- Message formatting using Slack Block Kit for rich formatting
- Rate limiting (max 1 message per rotation event)
- Configurable event filtering (only notify on failures, etc.)

### User Story 1.2: Email Notifications via SMTP (P1)

**As a** security engineer, **I want** rotation notifications sent via email, **so that** I have a permanent audit trail and can alert stakeholders without Slack.

**Why this priority**: Email provides permanent records for compliance and reaches stakeholders who don't use Slack. Critical for regulated industries (PCI-DSS, SOC2, HIPAA).

**Acceptance Criteria**:
1. **Given** SMTP server configured, **When** rotation completes, **Then** email sent to configured recipients
2. **Given** rotation fails, **When** error occurs, **Then** high-priority email sent with failure details
3. **Given** multiple rotations occur, **When** emails are sent, **Then** emails grouped/batched (configurable: immediate, hourly digest, daily digest)
4. **Given** email delivery fails, **When** SMTP error occurs, **Then** error logged but rotation continues

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      notifications:
        email:
          smtp:
            host: smtp.example.com
            port: 587
            username: store://vault/smtp/username
            password: store://vault/smtp/password
            tls: true
          from: "dsops-rotation@example.com"
          to: ["ops-team@example.com", "security@example.com"]
          events: [completed, failed, rollback]
          batch_mode: immediate  # or hourly, daily
```

**Email Format**:
```
Subject: [dsops] Rotation Completed: postgres-prod (production)

Service: postgres-prod
Environment: production
Status: ✅ Completed
Duration: 45 seconds
Strategy: two-key
Timestamp: 2025-08-26 14:30:00 UTC

New Secret Version: 2025-08-26-1430
Previous Secret: Marked for deletion (grace period: 24 hours)

Audit Trail: Run `dsops rotation history --service postgres-prod` for details
```

**Implementation Notes**:
- Use `net/smtp` for email sending
- Support STARTTLS and TLS
- HTML and plain-text email formats
- Email templates for each event type
- Batching logic for digest modes

### User Story 1.3: PagerDuty Integration for Incidents (P1)

**As an** SRE on-call, **I want** rotation failures to create PagerDuty incidents, **so that** critical rotation failures wake me up and I can respond immediately.

**Why this priority**: PagerDuty is the industry standard for incident management. Rotation failures can cause production outages if not addressed quickly.

**Acceptance Criteria**:
1. **Given** PagerDuty integration enabled, **When** rotation fails, **Then** PagerDuty incident created with severity and details
2. **Given** rotation succeeds after retry, **When** success occurs, **Then** PagerDuty incident auto-resolved
3. **Given** rollback occurs, **When** rollback completes, **Then** PagerDuty incident updated with rollback status
4. **Given** PagerDuty API is unreachable, **When** notification fails, **Then** fallback to local logging

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      notifications:
        pagerduty:
          integration_key: store://vault/pagerduty/integration-key
          service_id: PXXXXXX
          severity: error  # or warning, critical, info
          events: [failed, rollback]  # Only alert on failures
          auto_resolve: true  # Resolve incident on successful retry
```

**PagerDuty Incident Details**:
```json
{
  "routing_key": "INTEGRATION_KEY",
  "event_action": "trigger",
  "payload": {
    "summary": "dsops rotation failed: postgres-prod (production)",
    "severity": "error",
    "source": "dsops-rotation",
    "custom_details": {
      "service": "postgres-prod",
      "environment": "production",
      "strategy": "two-key",
      "error": "Connection verification failed",
      "timestamp": "2025-08-26T14:30:00Z"
    }
  }
}
```

**Implementation Notes**:
- Use PagerDuty Events API v2
- Support trigger, acknowledge, resolve event actions
- Deduplication using rotation ID
- Retry logic with exponential backoff

### User Story 1.4: Generic Webhook Notifications (P2)

**As a** platform engineer, **I want** generic webhook notifications, **so that** I can integrate rotation events with custom systems (monitoring dashboards, audit systems, internal tools).

**Why this priority**: Enables integration with any system that accepts webhooks, providing maximum flexibility for custom workflows.

**Acceptance Criteria**:
1. **Given** webhook URL configured, **When** rotation event occurs, **Then** HTTP POST sent to webhook with JSON payload
2. **Given** webhook requires authentication, **When** request is sent, **Then** auth headers included (Bearer token, Basic auth, custom headers)
3. **Given** webhook response is non-200, **When** failure occurs, **Then** retry with exponential backoff (3 attempts)
4. **Given** webhook has custom payload format, **When** event occurs, **Then** payload formatted using Go templates

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      notifications:
        webhooks:
          - name: "custom-monitoring"
            url: "https://monitoring.example.com/api/rotation-events"
            method: POST
            headers:
              Authorization: "Bearer {{.Token}}"
              X-Service-Name: "{{.ServiceName}}"
            events: [started, completed, failed]
            payload_template: |
              {
                "event_type": "{{.EventType}}",
                "service": "{{.ServiceName}}",
                "status": "{{.Status}}",
                "timestamp": "{{.Timestamp}}",
                "environment": "{{.Environment}}"
              }
            retry:
              max_attempts: 3
              backoff: exponential
            timeout: 10s
```

**Webhook Payload (Default)**:
```json
{
  "event": "rotation_completed",
  "service": "postgres-prod",
  "environment": "production",
  "status": "success",
  "strategy": "two-key",
  "duration_seconds": 45,
  "timestamp": "2025-08-26T14:30:00Z",
  "rotation_id": "rot-2025-08-26-1430-abc123",
  "metadata": {
    "previous_version": "2025-08-20-1015",
    "new_version": "2025-08-26-1430",
    "grace_period_hours": 24
  }
}
```

**Implementation Notes**:
- Support custom HTTP methods (POST, PUT, PATCH)
- Template engine for payload customization (Go `text/template`)
- Header injection for authentication
- Retry logic with configurable backoff
- Timeout configuration per webhook

---

## Category 2: Rollback & Recovery (25% → 100%) - **HIGH PRIORITY**

**Why Critical**: Rotation without rollback is dangerous. When rotation fails or causes issues, teams need instant recovery mechanisms to restore service.

### User Story 2.1: Automatic Rollback on Verification Failure (P0)

**As a** service operator, **I want** automatic rollback when verification fails, **so that** failed rotations don't leave my service in a broken state.

**Why this priority**: Manual rollback is slow and error-prone. Automatic rollback is essential for unattended rotation and reduces MTTR (Mean Time To Recovery).

**Acceptance Criteria**:
1. **Given** rotation completes but verification fails, **When** failure detected, **Then** automatic rollback initiated within 5 seconds
2. **Given** rollback is initiated, **When** previous secret is restored, **Then** service verification succeeds
3. **Given** rollback succeeds, **When** state is saved, **Then** audit trail records rollback reason and outcome
4. **Given** rollback fails, **When** error occurs, **Then** alert sent and manual intervention required (incident created)
5. **Given** automatic rollback is disabled, **When** verification fails, **Then** rotation marked as failed but no rollback occurs

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      rollback:
        automatic: true  # Enable automatic rollback
        on_verification_failure: true
        on_health_check_failure: true
        timeout: 30s  # Max time for rollback operation
        max_retries: 2  # Retry rollback if it fails
        notifications:
          - slack
          - pagerduty
```

**Rollback Workflow**:
```
1. Rotation completes → NewSecretValue created
2. Verification runs → Connection test FAILS
3. Rollback triggered:
   a. Restore OldSecretValue as active
   b. Mark NewSecretValue as invalid
   c. Re-verify with OldSecretValue
   d. If verification succeeds → Rollback complete
   e. If verification fails → CRITICAL INCIDENT
4. Audit log updated with rollback event
5. Notifications sent (Slack, PagerDuty, etc.)
```

**Implementation Notes**:
- Rollback logic in `internal/rotation/rollback.go`
- State machine: `rotating` → `verifying` → `rollback_in_progress` → `rolled_back` or `rollback_failed`
- Integration with verification framework
- Timeout enforcement to prevent hung rollbacks
- Detailed logging for rollback troubleshooting

### User Story 2.2: Manual Rollback Command (P1)

**As an** operations engineer, **I want** to manually rollback a rotation, **so that** I can revert problematic rotations discovered after verification passes.

**Why this priority**: Sometimes issues surface after rotation completes (performance degradation, application errors). Manual rollback provides safety net.

**Acceptance Criteria**:
1. **Given** rotation completed successfully, **When** user runs `dsops rotation rollback --service <name>`, **Then** previous secret restored as active
2. **Given** multiple rotations exist, **When** rollback targets specific version, **Then** specified version restored (`--version` flag)
3. **Given** manual rollback requested, **When** confirmation required, **Then** user prompted unless `--force` flag provided
4. **Given** rollback completes, **When** verification runs, **Then** verification passes before marking rollback successful
5. **Given** rollback fails, **When** error occurs, **Then** detailed error message with remediation steps shown

**CLI Usage**:
```bash
# Rollback to previous version
dsops rotation rollback --service postgres-prod --env production

# Rollback to specific version
dsops rotation rollback --service postgres-prod --version 2025-08-20-1015

# Force rollback without confirmation
dsops rotation rollback --service postgres-prod --force

# Dry-run rollback (preview only)
dsops rotation rollback --service postgres-prod --dry-run

# Rollback with custom reason
dsops rotation rollback --service postgres-prod --reason "Performance degradation detected"
```

**Command Output**:
```
⚠️  Rollback Requested: postgres-prod (production)

Current Version:  2025-08-26-1430
Rollback Target:  2025-08-20-1015
Grace Period:     24 hours remaining

This will:
  1. Restore previous secret version as active
  2. Mark current version as invalid
  3. Run verification with previous secret
  4. Update audit trail with rollback event

Proceed with rollback? [y/N]: y

🔄 Rolling back postgres-prod...
✓ Previous secret restored
✓ Verification passed
✓ Audit log updated
✅ Rollback completed successfully

Duration: 12 seconds
```

**Implementation Notes**:
- New command: `cmd/dsops/commands/rotation_rollback.go`
- Rollback logic reusable with automatic rollback
- Version selection from rotation history
- Interactive confirmation with `--force` override
- Integration with notification system

### User Story 2.3: Rollback Notifications (P2)

**As a** security team member, **I want** notifications when rollbacks occur, **so that** I can investigate why rotation failed and prevent future occurrences.

**Why this priority**: Rollbacks indicate problems (failed rotation, service issues, config errors). Tracking rollbacks helps identify patterns and improve reliability.

**Acceptance Criteria**:
1. **Given** automatic rollback occurs, **When** rollback completes, **Then** notification sent via all configured channels (Slack, email, PagerDuty)
2. **Given** manual rollback occurs, **When** user triggers rollback, **Then** notification includes user identity and reason
3. **Given** rollback fails, **When** error occurs, **Then** high-priority notification sent with incident details
4. **Given** rollback notification is sent, **When** message formatted, **Then** message includes rollback reason, previous version, and next steps

**Notification Content**:
```
🔄 Rollback Completed: postgres-prod

Type: Automatic (verification failure)
Environment: production
Previous Version: 2025-08-20-1015 (restored)
Failed Version: 2025-08-26-1430 (invalidated)
Reason: Connection verification failed
Duration: 12 seconds
Initiated by: dsops-automation

Next Steps:
  • Investigate why verification failed
  • Check database connectivity and credentials
  • Review rotation logs: `dsops rotation history --service postgres-prod`
  • Consider updating rotation strategy or verification timeout
```

**Implementation Notes**:
- Reuse notification framework from Category 1
- Rollback-specific message templates
- Include rollback metadata (reason, user, versions)
- Integration with audit trail for investigation

---

## Category 3: Verification & Health (25% → 100%) - **MEDIUM PRIORITY**

**Why Important**: Connection testing (25% complete) validates credentials work, but health checks monitor service behavior after rotation to detect subtle issues.

### User Story 3.1: Service Health Checks (P1)

**As a** platform engineer, **I want** continuous health monitoring after rotation, **so that** I detect degraded service performance or errors caused by rotation.

**Why this priority**: Verification proves credentials work at rotation time, but health checks detect issues that appear later (connection pool exhaustion, cache invalidation failures, etc.).

**Acceptance Criteria**:
1. **Given** rotation completes successfully, **When** health monitoring period starts, **Then** health checks run at configured interval (e.g., every 30s for 10 minutes)
2. **Given** health check fails, **When** threshold exceeded (e.g., 3 consecutive failures), **Then** automatic rollback triggered
3. **Given** health checks pass, **When** monitoring period ends, **Then** rotation marked as fully successful
4. **Given** health check is protocol-specific, **When** service type is SQL database, **Then** query execution time and connection count monitored

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      health_checks:
        enabled: true
        monitoring_period: 10m  # Monitor for 10 minutes after rotation
        interval: 30s  # Check every 30 seconds
        failure_threshold: 3  # 3 consecutive failures triggers rollback
        checks:
          - type: connection  # Built-in: verify connection works
          - type: query_latency  # Built-in: check query performance
            threshold_ms: 500
          - type: connection_pool  # Built-in: check pool exhaustion
            max_connections: 100
```

**Health Check Types by Protocol**:

**SQL Databases** (PostgreSQL, MySQL):
- Connection count (warn if >80% of max)
- Query latency (P50, P95, P99)
- Connection pool exhaustion
- Active transaction count
- Replication lag (if replica)

**HTTP APIs** (Stripe, GitHub):
- Response time (P50, P95, P99)
- Error rate (4xx, 5xx responses)
- Rate limit headers
- API quota remaining

**NoSQL** (MongoDB, Redis):
- Connection count
- Command latency
- Memory usage (Redis)
- Replication lag

**Implementation Notes**:
- Protocol-specific health check implementations in `internal/rotation/health/`
- Background goroutine for monitoring
- Configurable thresholds per service type
- Integration with rollback system
- Metrics collection for dashboards

### User Story 3.2: Custom Health Scripts (P2)

**As a** developer, **I want** to run custom health check scripts after rotation, **so that** I can validate application-specific behavior that dsops doesn't know about.

**Why this priority**: Generic health checks can't cover all use cases. Custom scripts enable validation of business logic, cache warming, application-specific workflows.

**Acceptance Criteria**:
1. **Given** custom health script configured, **When** rotation completes, **Then** script executed with environment variables (service name, new version, etc.)
2. **Given** script exits with code 0, **When** script completes, **Then** health check passes
3. **Given** script exits with non-zero code, **When** script completes, **Then** health check fails and error message captured
4. **Given** script times out, **When** timeout exceeded, **Then** health check fails and rollback considered

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      health_checks:
        custom_scripts:
          - name: "cache-validation"
            script: "/scripts/check-cache-warm.sh"
            timeout: 60s
            environment:
              DATABASE_URL: "{{.NewSecretValue}}"
              SERVICE_NAME: "{{.ServiceName}}"
            retry:
              max_attempts: 3
              backoff: 5s
```

**Script Execution Environment**:
```bash
#!/bin/bash
# /scripts/check-cache-warm.sh

# Environment variables provided by dsops:
# - DSOPS_SERVICE_NAME: postgres-prod
# - DSOPS_NEW_VERSION: 2025-08-26-1430
# - DSOPS_OLD_VERSION: 2025-08-20-1015
# - DATABASE_URL: <new connection string>

# Custom validation logic
psql "$DATABASE_URL" -c "SELECT COUNT(*) FROM cache_table" > /dev/null
if [ $? -ne 0 ]; then
    echo "Cache table not accessible"
    exit 1
fi

# Check application-specific metric
CACHE_SIZE=$(psql "$DATABASE_URL" -t -c "SELECT COUNT(*) FROM cache_table")
if [ "$CACHE_SIZE" -lt 1000 ]; then
    echo "Cache not warmed (size: $CACHE_SIZE)"
    exit 1
fi

echo "Health check passed: cache warmed with $CACHE_SIZE entries"
exit 0
```

**Implementation Notes**:
- Script execution via `os/exec` package
- Environment variable injection
- Stdout/stderr capture for logging
- Timeout enforcement
- Retry logic with backoff
- Validation of script exit codes

### User Story 3.3: Rotation Metric Collection (P2)

**As an** SRE, **I want** rotation metrics exported to Prometheus, **so that** I can build dashboards, set alerts, and track rotation success rates over time.

**Why this priority**: Metrics enable observability, SLO tracking, and proactive issue detection. Essential for production operations at scale.

**Acceptance Criteria**:
1. **Given** Prometheus metrics enabled, **When** rotation starts, **Then** `dsops_rotation_started_total` counter incremented
2. **Given** rotation completes, **When** success or failure recorded, **Then** `dsops_rotation_completed_total{status="success|failure"}` counter incremented
3. **Given** rotation finishes, **When** duration calculated, **Then** `dsops_rotation_duration_seconds` histogram updated
4. **Given** health check runs, **When** check completes, **Then** `dsops_health_check_status` gauge updated (1=healthy, 0=unhealthy)
5. **Given** Prometheus scrapes metrics endpoint, **When** `/metrics` accessed, **Then** all rotation metrics exposed in Prometheus format

**Metrics Specification**:
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

# Health check status gauge
dsops_health_check_status{service="postgres-prod",check_type="connection"} 1
dsops_health_check_status{service="postgres-prod",check_type="query_latency"} 1

# Rollback counters
dsops_rollback_total{service="postgres-prod",type="automatic"} 2
dsops_rollback_total{service="postgres-prod",type="manual"} 1

# Verification metrics
dsops_verification_duration_seconds_bucket{service="postgres-prod",le="1"} 38
dsops_verification_duration_seconds_bucket{service="postgres-prod",le="5"} 42
```

**Configuration Example**:
```yaml
metrics:
  prometheus:
    enabled: true
    port: 9090
    path: /metrics
    labels:
      environment: "production"
      team: "platform"
```

**Implementation Notes**:
- Use `github.com/prometheus/client_golang` library
- HTTP server on configurable port for `/metrics` endpoint
- Metric registration during initialization
- Label consistency (service, environment, strategy)
- Histogram buckets: 1s, 5s, 10s, 30s, 60s, 120s
- Gauge for current health status
- Integration with existing logging framework

---

## Category 4: Gradual Rollout (25% → 100%) - **MEDIUM PRIORITY**

**Why Important**: Immediate rotation is risky. Gradual rollout minimizes blast radius by testing rotation on subset before full deployment.

### User Story 4.1: Canary Rotation Strategy (P1)

**As a** platform engineer, **I want** to rotate secrets on a single instance first, **so that** I validate rotation works before rolling out to all instances.

**Why this priority**: Canary deployments are industry best practice. Testing on single instance catches issues before they impact entire service.

**Acceptance Criteria**:
1. **Given** canary rotation configured, **When** rotation starts, **Then** only canary instance receives new secret
2. **Given** canary instance rotated, **When** health checks pass for canary, **Then** rotation proceeds to remaining instances
3. **Given** canary health check fails, **When** failure detected, **Then** rotation aborted and canary rolled back
4. **Given** canary succeeds, **When** full rollout starts, **Then** remaining instances rotated in waves (configurable batch size)

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      strategy: canary
      canary:
        instance_selector: "role=canary"  # Label/tag selector
        health_monitoring_period: 5m  # Monitor canary for 5 minutes
        failure_threshold: 2  # 2 failures abort rollout
        rollout_waves:
          - percentage: 10  # After canary, rotate 10% of instances
            wait: 2m
          - percentage: 50  # Then 50%
            wait: 5m
          - percentage: 100  # Finally all remaining
```

**Canary Workflow**:
```
1. Identify canary instance (via selector)
2. Rotate secret on canary only
3. Monitor canary health for configured period
4. If canary healthy:
   a. Rotate next wave (10% of instances)
   b. Monitor wave health
   c. Continue to next wave
5. If canary unhealthy:
   a. Abort rotation
   b. Rollback canary
   c. Alert operators
```

**Implementation Notes**:
- Instance discovery via service registry or labels
- Wave-based rollout with configurable delays
- Per-wave health monitoring
- Abort mechanism on health check failure
- Integration with notification system for wave progress

### User Story 4.2: Percentage Rollout Strategy (P2)

**As an** operations engineer, **I want** to rotate secrets in percentage-based waves, **so that** I gradually increase coverage and minimize risk.

**Why this priority**: More granular than canary, percentage rollout provides fine-grained control over blast radius during rotation.

**Acceptance Criteria**:
1. **Given** percentage rollout configured, **When** rotation starts, **Then** first wave rotates configured percentage of instances
2. **Given** wave completes, **When** health checks pass, **Then** next wave starts after configured delay
3. **Given** wave fails health checks, **When** failure detected, **Then** rollout paused and alert sent (manual decision to continue or abort)
4. **Given** all waves complete, **When** final health check passes, **Then** rotation marked as complete

**Configuration Example**:
```yaml
services:
  postgres-prod:
    rotation:
      strategy: percentage_rollout
      rollout:
        waves:
          - percentage: 5
            health_monitoring: 2m
          - percentage: 25
            health_monitoring: 5m
          - percentage: 50
            health_monitoring: 10m
          - percentage: 100
            health_monitoring: 15m
        pause_on_failure: true  # Manual approval required to continue after failure
```

**Implementation Notes**:
- Calculate instance count per wave (round up to ensure 100% coverage)
- Health monitoring per wave
- Pause/resume mechanism for manual intervention
- Progress tracking and resumption on restart
- Integration with audit trail for wave completion

### User Story 4.3: Service Group Rotation (P2)

**As a** platform architect, **I want** to rotate related services together, **so that** dependent services receive updated secrets simultaneously.

**Why this priority**: Some services share secrets or have dependencies (primary/replica databases, API client/server). Group rotation ensures coordination.

**Acceptance Criteria**:
1. **Given** service group defined, **When** group rotation triggered, **Then** all services in group rotated in dependency order
2. **Given** dependency graph exists, **When** rotation starts, **Then** dependent services wait for dependencies to complete
3. **Given** any service in group fails, **When** failure detected, **Then** entire group rollback triggered
4. **Given** group rotation completes, **When** verification runs, **Then** cross-service validation performed (e.g., primary/replica consistency)

**Configuration Example**:
```yaml
service_groups:
  - name: postgres-cluster
    services:
      - postgres-primary
      - postgres-replica-1
      - postgres-replica-2
    rotation:
      strategy: sequential  # or parallel
      dependency_order:
        - postgres-primary  # Rotate primary first
        - [postgres-replica-1, postgres-replica-2]  # Then replicas in parallel
      failure_policy: rollback_all  # Rollback entire group on any failure
      cross_service_verification:
        enabled: true
        checks:
          - type: replication_lag
            max_lag_seconds: 10
```

**Implementation Notes**:
- Dependency graph construction
- Topological sort for rotation order
- Parallel execution where safe (no dependencies)
- Group-level rollback (all-or-nothing)
- Cross-service verification hooks
- Audit trail for group rotation events

---

## Implementation Plan

### Phase 1: Notification Infrastructure (Weeks 1-3)

**Goal**: Build reusable notification framework, implement all 4 notification types

**Milestones**:
- Week 1: Core notification framework + Slack integration
  - `internal/rotation/notifications/` package
  - `NotificationProvider` interface
  - Slack webhook implementation
  - Event filtering and templating

- Week 2: Email + PagerDuty integration
  - SMTP email provider
  - PagerDuty Events API integration
  - Notification batching/digest logic

- Week 3: Generic webhooks + testing
  - Webhook provider with custom templates
  - Retry logic and error handling
  - Unit + integration tests

**Deliverables**:
- ✅ Slack integration (User Story 1.1)
- ✅ Email notifications (User Story 1.2)
- ✅ PagerDuty integration (User Story 1.3)
- ✅ Generic webhooks (User Story 1.4)
- ✅ Test coverage ≥80% for notification package

### Phase 2: Rollback & Recovery (Weeks 4-5)

**Goal**: Implement automatic and manual rollback with notifications

**Milestones**:
- Week 4: Automatic rollback
  - Rollback state machine
  - Integration with verification framework
  - Timeout and retry logic
  - Rollback-specific notifications

- Week 5: Manual rollback command
  - `dsops rotation rollback` command
  - Version selection and confirmation
  - Rollback notifications
  - Testing and documentation

**Deliverables**:
- ✅ Automatic rollback (User Story 2.1)
- ✅ Manual rollback command (User Story 2.2)
- ✅ Rollback notifications (User Story 2.3)
- ✅ Integration tests for rollback scenarios
- ✅ Documentation updates

### Phase 3: Health Monitoring (Weeks 6-7)

**Goal**: Implement health checks and metric collection

**Milestones**:
- Week 6: Service health checks
  - Protocol-specific health check implementations
  - Background monitoring goroutine
  - Threshold-based failure detection
  - Integration with rollback system

- Week 7: Custom scripts + metrics
  - Custom health script execution
  - Prometheus metric collection
  - `/metrics` endpoint
  - Grafana dashboard templates

**Deliverables**:
- ✅ Service health checks (User Story 3.1)
- ✅ Custom health scripts (User Story 3.2)
- ✅ Prometheus metrics (User Story 3.3)
- ✅ Grafana dashboard examples
- ✅ Health monitoring documentation

### Phase 4: Gradual Rollout (Weeks 8-10)

**Goal**: Implement canary, percentage, and group rotation strategies

**Milestones**:
- Week 8: Canary rotation
  - Instance discovery and selection
  - Canary-specific health monitoring
  - Wave-based rollout engine

- Week 9: Percentage rollout
  - Percentage-based wave calculation
  - Pause/resume mechanism
  - Progress persistence and resumption

- Week 10: Service group rotation
  - Dependency graph construction
  - Group-level rollback logic
  - Cross-service verification
  - Final testing and documentation

**Deliverables**:
- ✅ Canary rotation (User Story 4.1)
- ✅ Percentage rollout (User Story 4.2)
- ✅ Service group rotation (User Story 4.3)
- ✅ End-to-end integration tests
- ✅ Complete rotation documentation update

### Phase 5: Integration & Polish (Week 11)

**Goal**: End-to-end testing, documentation, and release preparation

**Tasks**:
- Integration tests for all features
- Documentation review and updates
- Example configurations for all scenarios
- Release notes preparation
- Update VISION_ROTATE_IMPLEMENTATION.md to 100%

**Deliverables**:
- ✅ All Phase 5 features tested end-to-end
- ✅ Documentation complete
- ✅ VISION_ROTATE_IMPLEMENTATION.md updated to 100%
- ✅ Ready for v0.2 release

**Total Timeline**: 11 weeks (~3 months)

---

## Architecture

### Component Structure

```
internal/rotation/
├── notifications/          # Category 1: Notification providers
│   ├── provider.go        # NotificationProvider interface
│   ├── slack.go           # Slack webhook provider
│   ├── email.go           # SMTP email provider
│   ├── pagerduty.go       # PagerDuty Events API
│   ├── webhook.go         # Generic webhook provider
│   └── manager.go         # Notification routing and filtering
├── rollback/              # Category 2: Rollback logic
│   ├── rollback.go        # Rollback orchestration
│   ├── state_machine.go   # Rollback state transitions
│   └── automatic.go       # Automatic rollback triggers
├── health/                # Category 3: Health monitoring
│   ├── checker.go         # Health check orchestration
│   ├── protocols/         # Protocol-specific checks
│   │   ├── sql.go
│   │   ├── http.go
│   │   └── nosql.go
│   ├── scripts.go         # Custom script execution
│   └── metrics.go         # Prometheus metric collection
└── gradual/               # Category 4: Gradual rollout
    ├── canary.go          # Canary rotation strategy
    ├── percentage.go      # Percentage rollout strategy
    ├── group.go           # Service group rotation
    └── waves.go           # Wave-based rollout engine
```

### Notification Provider Interface

```go
// NotificationProvider sends rotation event notifications
type NotificationProvider interface {
    // Name returns the provider name (slack, email, pagerduty, webhook)
    Name() string

    // Send sends a notification for the given event
    Send(ctx context.Context, event RotationEvent) error

    // SupportsEvent checks if provider handles this event type
    SupportsEvent(eventType EventType) bool

    // Validate checks if provider configuration is valid
    Validate(ctx context.Context) error
}

// RotationEvent represents a rotation lifecycle event
type RotationEvent struct {
    Type        EventType         // started, completed, failed, rollback
    Service     string            // Service name
    Environment string            // Environment name
    Strategy    string            // Rotation strategy used
    Status      RotationStatus    // success, failure, rollback
    Error       error             // Error if failed
    Duration    time.Duration     // Rotation duration
    Metadata    map[string]string // Additional context
    Timestamp   time.Time         // Event timestamp
}
```

### Rollback State Machine

```go
// RollbackState represents rollback lifecycle states
type RollbackState string

const (
    RollbackStateIdle          RollbackState = "idle"
    RollbackStateTriggered     RollbackState = "triggered"
    RollbackStateInProgress    RollbackState = "in_progress"
    RollbackStateVerifying     RollbackState = "verifying"
    RollbackStateCompleted     RollbackState = "completed"
    RollbackStateFailed        RollbackState = "failed"
)

// RollbackManager handles rollback orchestration
type RollbackManager struct {
    notifier NotificationManager
    storage  RotationStorage
    logger   logging.Logger
}

// TriggerRollback initiates rollback to previous version
func (rm *RollbackManager) TriggerRollback(ctx context.Context, opts RollbackOptions) error {
    // 1. Validate current state allows rollback
    // 2. Load previous version from history
    // 3. Execute rollback (restore previous secret)
    // 4. Verify with previous secret
    // 5. Update audit trail
    // 6. Send notifications
    // 7. Return success or error
}
```

### Health Check Framework

```go
// HealthChecker performs post-rotation health monitoring
type HealthChecker interface {
    // Name returns the health check name
    Name() string

    // Check performs a single health check
    Check(ctx context.Context, service ServiceConfig) (HealthResult, error)

    // Protocol returns the protocol this checker supports
    Protocol() ProtocolType
}

// HealthMonitor coordinates health checks after rotation
type HealthMonitor struct {
    checkers        []HealthChecker
    checkInterval   time.Duration
    monitorDuration time.Duration
    failureThreshold int
    rollbackManager *RollbackManager
}

// MonitorAfterRotation starts health monitoring after successful rotation
func (hm *HealthMonitor) MonitorAfterRotation(ctx context.Context, service string) error {
    // 1. Start background goroutine
    // 2. Run health checks at interval
    // 3. Track consecutive failures
    // 4. Trigger rollback if threshold exceeded
    // 5. Stop monitoring after duration
    // 6. Report final health status
}
```

### Gradual Rollout Engine

```go
// RolloutStrategy defines gradual rollout behavior
type RolloutStrategy interface {
    // Name returns strategy name (canary, percentage, group)
    Name() string

    // Plan generates rollout waves
    Plan(ctx context.Context, service ServiceConfig) ([]RolloutWave, error)

    // Execute runs the rollout plan
    Execute(ctx context.Context, plan []RolloutWave) error
}

// RolloutWave represents a single rollout phase
type RolloutWave struct {
    Instances        []string      // Instance IDs to rotate in this wave
    Percentage       int           // Percentage of total instances
    WaitDuration     time.Duration // Wait time before next wave
    HealthMonitoring time.Duration // Health monitoring period
}
```

---

## Security Considerations

**Notification Security**:
- Webhook URLs, SMTP passwords, and API keys stored in secret stores (not plaintext)
- TLS required for all external communication (SMTP, webhooks, PagerDuty)
- Secret values never included in notifications (only metadata: service name, status, timestamps)
- Rate limiting on notifications to prevent DoS via excessive rotation triggers

**Rollback Security**:
- Rollback audit trail records who triggered rollback (user identity or automation)
- Previous secret versions secured with same protection as current secrets
- Rollback verification required before marking rollback successful
- Manual rollback requires confirmation (unless `--force`) to prevent accidental reversion

**Health Check Security**:
- Custom scripts run in sandboxed environment (limited permissions)
- Script timeout enforcement to prevent hung processes
- Script output sanitized before logging (no secret leakage)
- Metrics endpoint does not expose secret values (only counts, durations, status)

**Gradual Rollout Security**:
- Instance selectors validated to prevent targeting wrong instances
- Rollback propagates to all rotated instances in wave (consistent state)
- Wave progress persisted securely to prevent replay attacks

---

## Testing Strategy

**Unit Tests** (≥80% coverage):
- Notification provider implementations (mock HTTP servers)
- Rollback state machine transitions
- Health check logic (mock service responses)
- Rollout wave calculation and ordering

**Integration Tests**:
- End-to-end rotation with notifications (Slack webhook mock)
- Automatic rollback on verification failure (Docker PostgreSQL)
- Health monitoring with failure injection
- Canary rotation with multi-instance setup

**Security Tests**:
- Verify no secrets in notification payloads
- Validate TLS enforcement for external communication
- Test script sandboxing and timeout enforcement
- Verify audit trail integrity

**Performance Tests**:
- Notification latency under load
- Health check overhead during monitoring
- Rollout wave execution time
- Metric collection performance

---

## Documentation Updates

**User Documentation**:
- `docs/content/rotation/notifications.md` - Complete notification guide
- `docs/content/rotation/rollback.md` - Rollback procedures and best practices
- `docs/content/rotation/health-checks.md` - Health monitoring configuration
- `docs/content/rotation/gradual-rollout.md` - Canary and percentage rollout examples

**Developer Documentation**:
- `docs/developer/rotation-architecture.md` - Update with new components
- `docs/developer/adding-notification-providers.md` - Provider development guide
- `docs/developer/adding-health-checks.md` - Health checker development guide

**Configuration Reference**:
- Update `docs/content/rotation/configuration.md` with all new config options
- Add complete examples for each feature

**Runbooks**:
- Create operational runbooks for common scenarios:
  - Responding to rotation failures
  - Manual rollback procedures
  - Debugging health check failures
  - Configuring notifications for production

---

## Success Criteria

**Definition of Done**:
1. ✅ All 13 Phase 5 features implemented (Notifications, Rollback, Health, Gradual Rollout)
2. ✅ VISION_ROTATE_IMPLEMENTATION.md Phase 5 updated to 100% (38% → 100%)
3. ✅ Test coverage ≥80% for all new rotation packages
4. ✅ Integration tests for all user stories
5. ✅ Documentation complete for all features
6. ✅ Example configurations for all scenarios
7. ✅ Security review passed (no secrets in notifications/logs)
8. ✅ Performance benchmarks meet targets (<5s notification latency, <10s rollback)
9. ✅ Grafana dashboard templates published
10. ✅ Ready for production use (v0.2 release criteria met)

**Acceptance Testing**:
- Deploy test rotation with all features enabled (PostgreSQL)
- Trigger rotation failure → Verify automatic rollback + notifications
- Run canary rotation → Verify gradual rollout + health monitoring
- Trigger manual rollback → Verify notifications + audit trail
- Load test: 100 rotations/hour with notifications + metrics

---

## Future Enhancements (v0.3+)

**Beyond Phase 5**:
1. **Policy Enforcement** (Phase 6): Rotation policies with compliance reporting
2. **Approval Workflows** (Phase 6): Require approvals for production rotations
3. **Multi-Region Coordination**: Rotate across regions in sequence
4. **Smart Rollout**: AI-driven rollout pacing based on error rates
5. **Rotation Scheduling**: Cron-based automatic rotation
6. **Blue-Green Service Rotation**: Full service replacement (not just secrets)
7. **Chaos Testing Integration**: Inject failures during rotation for resilience testing
8. **Custom Notification Templates**: User-defined message formats
9. **Notification Aggregation**: Single digest for multiple rotation events
10. **Health Check Plugins**: Dynamic health checker loading

---

## Related Specifications

- **SPEC-003**: Secret Resolution Engine (rotation integrates with resolution)
- **SPEC-005**: Testing Strategy & Plan (rotation testing requirements)
- **SPEC-010-019**: Provider Specifications (secret store integration)
- **ADR-001**: Provider → SecretStore + Service Terminology (rotation context)

## References

- VISION_ROTATE.md: Complete rotation vision
- VISION_ROTATE_IMPLEMENTATION.md: Phase 5 feature tracking
- dsops-data repository: Service definitions and rotation metadata
- docs/TERMINOLOGY.md: Secret stores vs. services distinction
- docs/content/rotation/configuration.md: Rotation configuration reference
