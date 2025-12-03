# Implementation Plan: Rotation Phase 5 Completion

**Branch**: `009-phase-5-completion` | **Date**: 2025-12-03 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/009-phase-5-completion/spec.md`

## Summary

Complete Phase 5 rotation features: Notifications (Slack/Email/PagerDuty/Webhooks), Rollback & Recovery (automatic + manual), Health Monitoring (protocol-specific + custom scripts + Prometheus), and Gradual Rollout (canary/percentage/service groups). Bring rotation from 38% to 100% completion.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: net/http, net/smtp, github.com/prometheus/client_golang
**Storage**: File-based (existing `internal/rotation/storage/`)
**Testing**: go test with testify, Docker Compose for integration
**Target Platform**: macOS, Linux, Windows (cross-platform Go)
**Project Type**: CLI tool
**Performance Goals**: <5s notification latency, <10s rollback
**Constraints**: Best-effort notifications (non-blocking), bounded queue (100 events)
**Scale/Scope**: 13 user stories, 4 categories, 125 tasks

## Constitution Check

*All principles pass - no violations.*

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Ephemeral-First | PASS | Notifications don't persist secrets; health checks use in-memory state |
| II. Security by Default | PASS | Use `logging.Secret()` for webhook URLs/API keys; no secrets in notifications |
| III. Provider-Agnostic | PASS | NotificationProvider interface abstracts Slack/Email/PagerDuty/Webhook |
| IV. Data-Driven | PASS | Notification configs in YAML; health check types from dsops-data |
| V. Developer Experience | PASS | Rich CLI output; `--dry-run` for all commands |
| VI. Cross-Platform | PASS | Pure Go implementation |
| VII. TDD | PASS | Tests written before implementation per constitution |
| VIII. Explicit Over Implicit | PASS | Rollback requires `--force` or confirmation; notifications opt-in |
| IX. Deterministic | PASS | Same config produces same notification routing |

## Project Structure

### Documentation (this feature)

```text
specs/009-phase-5-completion/
├── spec.md              # Feature specification (13 user stories)
├── plan.md              # This file
├── tasks.md             # Task breakdown (125 tasks)
└── research.md          # Research findings (deferred - inline in plan)
```

### Source Code (repository root)

```text
internal/rotation/
├── notifications/           # Category 1: Notification providers
│   ├── provider.go         # NotificationProvider interface + Manager
│   ├── slack.go            # Slack webhook provider
│   ├── email.go            # SMTP email provider
│   ├── pagerduty.go        # PagerDuty Events API
│   ├── webhook.go          # Generic webhook provider
│   ├── metrics.go          # Prometheus notification metrics
│   └── templates.go        # Message templates
├── rollback/               # Category 2: Rollback logic
│   ├── manager.go          # RollbackManager orchestration
│   ├── config.go           # Rollback configuration
│   └── state.go            # State machine
├── health/                 # Category 3: Health monitoring
│   ├── monitor.go          # HealthMonitor background goroutine
│   ├── checker.go          # HealthChecker interface
│   ├── sql.go              # SQL health checks
│   ├── http.go             # HTTP API health checks
│   ├── script.go           # Custom script execution
│   ├── metrics.go          # Prometheus metrics
│   └── server.go           # /metrics HTTP server
└── gradual/                # Category 4: Gradual rollout
    ├── strategy.go         # RolloutStrategy interface
    ├── canary.go           # Canary rotation
    ├── percentage.go       # Percentage rollout
    ├── group.go            # Service group rotation
    └── discovery/          # Instance discovery
        ├── provider.go     # DiscoveryProvider interface
        ├── explicit.go     # Config-based listing
        ├── kubernetes.go   # K8s label selector
        ├── cloud.go        # AWS/GCP/Azure tags
        └── endpoint.go     # HTTP endpoint discovery

cmd/dsops/commands/
├── rotation_rollback.go    # Manual rollback command

tests/integration/
├── notifications_slack_test.go
├── notifications_email_test.go
├── notifications_pagerduty_test.go
├── notifications_webhook_test.go
├── rollback_test.go
├── rollback_manual_test.go
└── health_test.go
```

**Structure Decision**: Single CLI project with new packages under `internal/rotation/` following existing patterns. No architectural changes needed.

## Key Interfaces

### NotificationProvider

```go
type NotificationProvider interface {
    Name() string
    Send(ctx context.Context, event RotationEvent) error
    SupportsEvent(eventType EventType) bool
    Validate(ctx context.Context) error
}

type RotationEvent struct {
    Type        EventType              // started, completed, failed, rollback
    Service     string
    Environment string
    Strategy    string
    Status      RotationStatus
    Error       error
    Duration    time.Duration
    Metadata    map[string]string
    Timestamp   time.Time
}
```

### HealthChecker

```go
type HealthChecker interface {
    Name() string
    Check(ctx context.Context, service ServiceConfig) (HealthResult, error)
    Protocol() ProtocolType
}
```

### DiscoveryProvider

```go
type DiscoveryProvider interface {
    Name() string
    Discover(ctx context.Context, config DiscoveryConfig) ([]Instance, error)
    Validate(config DiscoveryConfig) error
}
```

## Existing Infrastructure to Leverage

| Purpose | File | Pattern |
|---------|------|---------|
| Notification System | `internal/incident/notifications.go` | Slack + GitHub implementations |
| HTTP Client | `pkg/rotation/webhook.go` | 30s timeout, context support |
| Logging/Redaction | `internal/logging/logger.go` | `logging.Secret()` wrapper |
| State Machine | `pkg/rotation/storage.go` | `RotationStatus` enum |
| CLI Patterns | `cmd/dsops/commands/shred.go` | Confirmation, --force flag |
| Protocol Adapters | `pkg/protocol/sql.go`, `http_api.go` | SQL, HTTP adapters |
| Verification | `pkg/rotation/interface.go` | `VerificationTest` types |

## Implementation Phases

### Phase 1: Notification Infrastructure (Weeks 1-3)
- Core notification framework + async queue
- Slack integration (P0)
- Email + PagerDuty (P1)
- Generic webhooks (P2)

### Phase 2: Rollback & Recovery (Weeks 4-5)
- Automatic rollback on verification failure (P0)
- Manual rollback command (P1)
- Rollback notifications (P2)

### Phase 3: Health Monitoring (Weeks 6-7)
- Service health checks (P1)
- Custom health scripts (P2)
- Prometheus metrics (P2)

### Phase 4: Gradual Rollout (Weeks 8-10)
- Instance discovery framework
- Canary rotation (P1)
- Percentage rollout (P2)
- Service group rotation (P2)

### Phase 5: Integration & Polish (Week 11)
- End-to-end tests
- Documentation
- Examples

## MVP Scope

**Phases 1-2 (Setup → Slack → Auto Rollback)**: 34 tasks

Delivers:
- Slack notifications for rotation events
- Automatic rollback on verification failure
- Core notification framework for future providers

## Complexity Tracking

No constitution violations - no complexity justification needed.
