# Tasks: SPEC-009 Rotation Phase 5 Completion

**Input**: Design documents from `/specs/009-phase-5-completion/`
**Prerequisites**: plan.md, spec.md with 13 user stories across 4 categories
**Tests**: Required per Constitution Principle VII (TDD)

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story (US1.1, US1.2, US2.1, etc.)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and notification framework foundation

- [X] T001 Create notification package structure in internal/rotation/notifications/
- [X] T002 [P] Create NotificationProvider interface in internal/rotation/notifications/provider.go
- [X] T003 [P] Create RotationEvent type in internal/rotation/notifications/event.go
- [X] T004 [P] Create NotificationManager with async bounded queue (100 events) in internal/rotation/notifications/manager.go
- [X] T005 [P] Add notification config types to internal/config/notifications.go
- [X] T006 Add Prometheus client dependency to go.mod (github.com/prometheus/client_golang)
- [X] T007 Create rollback package structure in internal/rotation/rollback/
- [X] T008 Create health package structure in internal/rotation/health/
- [X] T009 Create gradual package structure in internal/rotation/gradual/

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [X] T010 Implement NotificationManager.Start() background goroutine in internal/rotation/notifications/manager.go
- [X] T011 Implement NotificationManager.Send() with queue logic in internal/rotation/notifications/manager.go
- [X] T012 [P] Add dsops_notifications_dropped_total Prometheus counter in internal/rotation/notifications/metrics.go
- [X] T013 [P] Create EventType enum (started, completed, failed, rollback) in internal/rotation/notifications/event.go
- [X] T014 Integrate NotificationManager into pkg/rotation/engine.go
- [X] T015 [P] Parse notification config in internal/config/config.go
- [X] T016 Add --on-conflict flag to rotation commands in cmd/dsops/commands/secrets_rotate.go

**Checkpoint**: Foundation ready - user story implementation can begin

---

## Phase 3: User Story 1.1 - Slack Integration (Priority: P0) MVP

**Goal**: Send rotation events to Slack channels via webhook

**Independent Test**: `go test ./internal/rotation/notifications/... -run TestSlack`

### Tests for US1.1

- [X] T017 [P] [US1.1] Unit test SlackProvider in internal/rotation/notifications/slack_test.go
- [X] T018 [P] [US1.1] Integration test Slack webhook (mock server) in tests/integration/notifications_slack_test.go

### Implementation for US1.1

- [X] T019 [P] [US1.1] Create SlackConfig struct in internal/rotation/notifications/slack.go
- [X] T020 [US1.1] Implement SlackProvider.Send() with Block Kit formatting in internal/rotation/notifications/slack.go
- [X] T021 [US1.1] Implement SlackProvider.SupportsEvent() for event filtering in internal/rotation/notifications/slack.go
- [X] T022 [US1.1] Add Slack message templates (started, completed, failed, rollback) in internal/rotation/notifications/slack.go
- [X] T023 [US1.1] Implement mentions on failure (@oncall) in internal/rotation/notifications/slack.go
- [X] T024 [US1.1] Register SlackProvider in NotificationManager in internal/rotation/notifications/manager.go

**Checkpoint**: Slack notifications working end-to-end

---

## Phase 4: User Story 1.2 - Email Notifications (Priority: P1)

**Goal**: Send rotation notifications via SMTP email

**Independent Test**: `go test ./internal/rotation/notifications/... -run TestEmail`

### Tests for US1.2

- [X] T025 [P] [US1.2] Unit test EmailProvider in internal/rotation/notifications/email_test.go
- [ ] T026 [P] [US1.2] Integration test SMTP (mock server) in tests/integration/notifications_email_test.go

### Implementation for US1.2

- [X] T027 [P] [US1.2] Create EmailConfig struct with SMTP settings in internal/rotation/notifications/email.go
- [X] T028 [US1.2] Implement EmailProvider.Send() with STARTTLS/TLS in internal/rotation/notifications/email.go
- [X] T029 [US1.2] Create HTML and plain-text email templates in internal/rotation/notifications/email.go
- [X] T030 [US1.2] Implement batch_mode (immediate, hourly, daily) in internal/rotation/notifications/email.go
- [X] T031 [US1.2] Register EmailProvider in NotificationManager in internal/rotation/notifications/manager.go

**Checkpoint**: Email notifications working

---

## Phase 5: User Story 1.3 - PagerDuty Integration (Priority: P1)

**Goal**: Create PagerDuty incidents for rotation failures

**Independent Test**: `go test ./internal/rotation/notifications/... -run TestPagerDuty`

### Tests for US1.3

- [X] T032 [P] [US1.3] Unit test PagerDutyProvider in internal/rotation/notifications/pagerduty_test.go
- [ ] T033 [P] [US1.3] Integration test PagerDuty Events API (mock) in tests/integration/notifications_pagerduty_test.go

### Implementation for US1.3

- [X] T034 [P] [US1.3] Create PagerDutyConfig struct in internal/rotation/notifications/pagerduty.go
- [X] T035 [US1.3] Implement PagerDutyProvider.Send() with Events API v2 in internal/rotation/notifications/pagerduty.go
- [X] T036 [US1.3] Implement trigger/acknowledge/resolve actions in internal/rotation/notifications/pagerduty.go
- [X] T037 [US1.3] Add deduplication using rotation ID in internal/rotation/notifications/pagerduty.go
- [X] T038 [US1.3] Implement auto_resolve on successful retry in internal/rotation/notifications/pagerduty.go
- [X] T039 [US1.3] Register PagerDutyProvider in NotificationManager in internal/rotation/notifications/manager.go

**Checkpoint**: PagerDuty incidents working

---

## Phase 6: User Story 1.4 - Generic Webhooks (Priority: P2)

**Goal**: Send notifications to custom webhook endpoints

**Independent Test**: `go test ./internal/rotation/notifications/... -run TestWebhook`

### Tests for US1.4

- [X] T040 [P] [US1.4] Unit test WebhookProvider in internal/rotation/notifications/webhook_test.go
- [ ] T041 [P] [US1.4] Integration test webhook with retry logic in tests/integration/notifications_webhook_test.go

### Implementation for US1.4

- [X] T042 [P] [US1.4] Create WebhookConfig struct with templates in internal/rotation/notifications/webhook.go
- [X] T043 [US1.4] Implement WebhookProvider.Send() with Go templates in internal/rotation/notifications/webhook.go
- [X] T044 [US1.4] Add configurable headers and auth in internal/rotation/notifications/webhook.go
- [X] T045 [US1.4] Implement retry with exponential backoff (3 attempts) in internal/rotation/notifications/webhook.go
- [X] T046 [US1.4] Register WebhookProvider in NotificationManager in internal/rotation/notifications/manager.go

**Checkpoint**: All notification providers complete

---

## Phase 7: User Story 2.1 - Automatic Rollback (Priority: P0)

**Goal**: Automatically rollback on verification failure

**Independent Test**: `go test ./internal/rotation/rollback/... -run TestAutomatic`

### Tests for US2.1

- [X] T047 [P] [US2.1] Unit test RollbackManager in internal/rotation/rollback/manager_test.go
- [X] T048 [P] [US2.1] Integration test rollback with PostgreSQL in tests/integration/rollback_test.go

### Implementation for US2.1

- [X] T049 [P] [US2.1] Create RollbackConfig struct in internal/rotation/rollback/config.go
- [X] T050 [US2.1] Implement RollbackManager.TriggerRollback() in internal/rotation/rollback/manager.go
- [X] T051 [US2.1] Implement state machine (rotating->verifying->rollback_in_progress->rolled_back) in internal/rotation/rollback/state.go
- [X] T052 [US2.1] Add timeout enforcement (30s default) in internal/rotation/rollback/manager.go
- [X] T053 [US2.1] Integrate with verification framework in pkg/rotation/engine.go
- [X] T054 [US2.1] Add rollback audit trail entries in internal/rotation/rollback/manager.go
- [X] T055 [US2.1] Integrate with notification system for rollback events in internal/rotation/rollback/manager.go

**Checkpoint**: Automatic rollback on verification failure working

---

## Phase 8: User Story 2.2 - Manual Rollback Command (Priority: P1)

**Goal**: CLI command for manual rollback

**Independent Test**: `./bin/dsops rotation rollback --help`

### Tests for US2.2

- [X] T056 [P] [US2.2] Unit test rollback command in cmd/dsops/commands/rotation_rollback_test.go
- [X] T057 [P] [US2.2] Integration test manual rollback flow in tests/integration/rollback/manual_test.go

### Implementation for US2.2

- [X] T058 [US2.2] Create rotation_rollback.go command in cmd/dsops/commands/rotation_rollback.go
- [X] T059 [US2.2] Implement --service, --env, --version flags in cmd/dsops/commands/rotation_rollback.go
- [X] T060 [US2.2] Implement --force flag to skip confirmation in cmd/dsops/commands/rotation_rollback.go
- [X] T061 [US2.2] Implement --reason flag for audit trail in cmd/dsops/commands/rotation_rollback.go
- [X] T062 [US2.2] Implement --dry-run flag in cmd/dsops/commands/rotation_rollback.go
- [X] T063 [US2.2] Add interactive confirmation prompt in cmd/dsops/commands/rotation_rollback.go
- [X] T064 [US2.2] Register rollback subcommand under rotation in cmd/dsops/commands/rotation.go

**Checkpoint**: Manual rollback command working

---

## Phase 9: User Story 2.3 - Rollback Notifications (Priority: P2)

**Goal**: Send notifications when rollbacks occur

**Independent Test**: Rollback triggers Slack/Email notifications

### Implementation for US2.3

- [X] T065 [US2.3] Create rollback-specific message templates in internal/rotation/notifications/templates.go
- [X] T066 [US2.3] Include rollback metadata (reason, versions, user) in notifications in internal/rotation/rollback/manager.go
- [X] T067 [US2.3] Add next steps recommendations to rollback notifications in internal/rotation/notifications/templates.go

**Checkpoint**: Rollback notifications working

---

## Phase 10: User Story 3.1 - Service Health Checks (Priority: P1)

**Goal**: Background health monitoring after rotation

**Independent Test**: `go test ./internal/rotation/health/... -run TestMonitor`

### Tests for US3.1

- [ ] T068 [P] [US3.1] Unit test HealthMonitor in internal/rotation/health/monitor_test.go
- [ ] T069 [P] [US3.1] Unit test SQL health checker in internal/rotation/health/sql_test.go
- [ ] T070 [P] [US3.1] Integration test health monitoring in tests/integration/health_test.go

### Implementation for US3.1

- [ ] T071 [P] [US3.1] Create HealthChecker interface in internal/rotation/health/checker.go
- [ ] T072 [P] [US3.1] Create HealthResult struct in internal/rotation/health/checker.go
- [ ] T073 [US3.1] Implement HealthMonitor with background goroutine in internal/rotation/health/monitor.go
- [ ] T074 [US3.1] Implement configurable interval (default 30s) and period (default 10m) in internal/rotation/health/monitor.go
- [ ] T075 [US3.1] Implement failure_threshold (default 3) triggering rollback in internal/rotation/health/monitor.go
- [ ] T076 [US3.1] Implement SQLHealthChecker (ping, query_latency, connection_pool) in internal/rotation/health/sql.go
- [ ] T077 [US3.1] Implement HTTPHealthChecker (response_time, error_rate) in internal/rotation/health/http.go
- [ ] T078 [US3.1] Integrate HealthMonitor with RollbackManager in internal/rotation/health/monitor.go

**Checkpoint**: Health monitoring with rollback integration working

---

## Phase 11: User Story 3.2 - Custom Health Scripts (Priority: P2)

**Goal**: Execute custom health check scripts

**Independent Test**: `go test ./internal/rotation/health/... -run TestScript`

### Tests for US3.2

- [ ] T079 [P] [US3.2] Unit test ScriptHealthChecker in internal/rotation/health/script_test.go

### Implementation for US3.2

- [ ] T080 [US3.2] Implement ScriptHealthChecker with os/exec in internal/rotation/health/script.go
- [ ] T081 [US3.2] Inject DSOPS_* environment variables in internal/rotation/health/script.go
- [ ] T082 [US3.2] Implement timeout enforcement in internal/rotation/health/script.go
- [ ] T083 [US3.2] Capture stdout/stderr for logging in internal/rotation/health/script.go
- [ ] T084 [US3.2] Implement retry with backoff in internal/rotation/health/script.go

**Checkpoint**: Custom health scripts working

---

## Phase 12: User Story 3.3 - Prometheus Metrics (Priority: P2)

**Goal**: Export rotation metrics to Prometheus

**Independent Test**: `curl localhost:9090/metrics | grep dsops_`

### Tests for US3.3

- [ ] T085 [P] [US3.3] Unit test metrics registration in internal/rotation/health/metrics_test.go

### Implementation for US3.3

- [ ] T086 [US3.3] Register dsops_rotation_started_total counter in internal/rotation/health/metrics.go
- [ ] T087 [US3.3] Register dsops_rotation_completed_total counter (with status label) in internal/rotation/health/metrics.go
- [ ] T088 [US3.3] Register dsops_rotation_duration_seconds histogram in internal/rotation/health/metrics.go
- [ ] T089 [US3.3] Register dsops_health_check_status gauge in internal/rotation/health/metrics.go
- [ ] T090 [US3.3] Register dsops_rollback_total counter in internal/rotation/health/metrics.go
- [ ] T091 [US3.3] Create HTTP server for /metrics endpoint in internal/rotation/health/server.go
- [ ] T092 [US3.3] Add metrics config parsing in internal/config/config.go
- [ ] T093 [US3.3] Integrate metrics collection in rotation engine in pkg/rotation/engine.go

**Checkpoint**: Prometheus metrics exposed

---

## Phase 13: User Story 4.1 - Canary Rotation (Priority: P1)

**Goal**: Rotate canary instance first, then proceed to full rollout

**Independent Test**: `go test ./internal/rotation/gradual/... -run TestCanary`

### Tests for US4.1

- [ ] T094 [P] [US4.1] Unit test CanaryStrategy in internal/rotation/gradual/canary_test.go

### Implementation for US4.1

- [ ] T095 [P] [US4.1] Create RolloutStrategy interface in internal/rotation/gradual/strategy.go
- [ ] T096 [P] [US4.1] Create DiscoveryProvider interface in internal/rotation/gradual/discovery/provider.go
- [ ] T097 [US4.1] Implement ExplicitDiscovery (config-based) in internal/rotation/gradual/discovery/explicit.go
- [ ] T098 [US4.1] Implement CanaryStrategy.Plan() in internal/rotation/gradual/canary.go
- [ ] T099 [US4.1] Implement CanaryStrategy.Execute() with wave logic in internal/rotation/gradual/canary.go
- [ ] T100 [US4.1] Integrate canary health monitoring in internal/rotation/gradual/canary.go
- [ ] T101 [US4.1] Integrate canary abort/rollback on failure in internal/rotation/gradual/canary.go

**Checkpoint**: Canary rotation working

---

## Phase 14: User Story 4.2 - Percentage Rollout (Priority: P2)

**Goal**: Rotate in percentage-based waves

**Independent Test**: `go test ./internal/rotation/gradual/... -run TestPercentage`

### Tests for US4.2

- [ ] T102 [P] [US4.2] Unit test PercentageStrategy in internal/rotation/gradual/percentage_test.go

### Implementation for US4.2

- [ ] T103 [US4.2] Implement PercentageStrategy.Plan() with wave calculation in internal/rotation/gradual/percentage.go
- [ ] T104 [US4.2] Implement PercentageStrategy.Execute() with health monitoring per wave in internal/rotation/gradual/percentage.go
- [ ] T105 [US4.2] Implement pause_on_failure with manual approval in internal/rotation/gradual/percentage.go
- [ ] T106 [US4.2] Implement progress persistence for resume in internal/rotation/gradual/percentage.go

**Checkpoint**: Percentage rollout working

---

## Phase 15: User Story 4.3 - Service Group Rotation (Priority: P2)

**Goal**: Rotate related services together with dependency ordering

**Independent Test**: `go test ./internal/rotation/gradual/... -run TestGroup`

### Tests for US4.3

- [ ] T107 [P] [US4.3] Unit test GroupStrategy in internal/rotation/gradual/group_test.go

### Implementation for US4.3

- [ ] T108 [US4.3] Implement GroupStrategy with dependency graph in internal/rotation/gradual/group.go
- [ ] T109 [US4.3] Implement topological sort for rotation order in internal/rotation/gradual/group.go
- [ ] T110 [US4.3] Implement parallel execution where safe in internal/rotation/gradual/group.go
- [ ] T111 [US4.3] Implement group-level rollback (all-or-nothing) in internal/rotation/gradual/group.go
- [ ] T112 [US4.3] Add cross-service verification hooks in internal/rotation/gradual/group.go

**Checkpoint**: Service group rotation working

---

## Phase 16: Additional Discovery Providers

**Goal**: Support Kubernetes, cloud, and endpoint discovery

- [ ] T113 [P] Implement KubernetesDiscovery in internal/rotation/gradual/discovery/kubernetes.go
- [ ] T114 [P] Implement CloudDiscovery (AWS/GCP/Azure tags) in internal/rotation/gradual/discovery/cloud.go
- [ ] T115 [P] Implement EndpointDiscovery in internal/rotation/gradual/discovery/endpoint.go

---

## Phase 17: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, examples, final validation

- [ ] T116 [P] Update docs/content/reference/status.md with Phase 5 completion
- [ ] T117 [P] Update specs/009-phase-5-completion/spec.md status to "Implemented"
- [ ] T118 [P] Create notification examples in examples/notifications/
- [ ] T119 [P] Create health check examples in examples/health-checks/
- [ ] T120 [P] Create gradual rollout examples in examples/gradual-rollout/
- [ ] T121 Create user documentation in docs/content/rotation/notifications.md
- [ ] T122 Create user documentation in docs/content/rotation/health-checks.md
- [ ] T123 Create user documentation in docs/content/rotation/gradual-rollout.md
- [ ] T124 Run full test suite with coverage validation (>=80%)
- [ ] T125 Security review: verify no secrets in notifications/logs

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup - BLOCKS all user stories
- **User Stories (Phases 3-15)**: All depend on Foundational phase
- **Discovery Providers (Phase 16)**: Depends on Phase 13 (Canary)
- **Polish (Phase 17)**: Depends on all user stories complete

### User Story Dependencies

| Story | Depends On | Can Parallel With |
|-------|------------|-------------------|
| US1.1 (Slack) | Foundational | - |
| US1.2 (Email) | Foundational | US1.1 |
| US1.3 (PagerDuty) | Foundational | US1.1, US1.2 |
| US1.4 (Webhook) | Foundational | US1.1, US1.2, US1.3 |
| US2.1 (Auto Rollback) | Foundational, US1.1 (notifications) | US1.2, US1.3, US1.4 |
| US2.2 (Manual Rollback) | US2.1 | US1.x |
| US2.3 (Rollback Notifs) | US1.1, US2.1 | - |
| US3.1 (Health Checks) | US2.1 (rollback integration) | US1.x |
| US3.2 (Custom Scripts) | US3.1 | US1.x, US2.x |
| US3.3 (Metrics) | Foundational | All others |
| US4.1 (Canary) | US3.1 (health integration) | US3.2, US3.3 |
| US4.2 (Percentage) | US4.1 | - |
| US4.3 (Group) | US4.1 | US4.2 |

---

## Implementation Strategy

### MVP First (Slack + Auto Rollback)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: US1.1 (Slack)
4. Complete Phase 7: US2.1 (Auto Rollback)
5. **STOP and VALIDATE**: Test Slack notifications + auto rollback
6. Deploy/demo MVP

### Incremental Delivery

1. MVP: Slack + Auto Rollback (Phases 1-3, 7)
2. +Email, PagerDuty (Phases 4-5)
3. +Manual Rollback (Phase 8)
4. +Health Monitoring (Phase 10)
5. +Prometheus Metrics (Phase 12)
6. +Canary Rollout (Phase 13)
7. +Remaining features

---

## Summary

| Metric | Value |
|--------|-------|
| **Total Tasks** | 125 |
| **User Stories** | 13 |
| **Phases** | 17 |
| **Parallel Opportunities** | 48 tasks marked [P] |
| **MVP Scope** | US1.1 + US2.1 (Phases 1-3, 7) |
