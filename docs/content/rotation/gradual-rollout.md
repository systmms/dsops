---
title: "Gradual Rollout"
description: "Minimize risk with canary rotations, percentage-based rollouts, and service group coordination"
lead: "Reduce blast radius by rotating secrets gradually. Test on canary instances first, roll out in percentage-based waves, and coordinate rotation across service groups with dependency management."
date: 2025-12-09T00:00:00-00:00
lastmod: 2025-12-09T00:00:00-00:00
draft: false
weight: 40
---

## Overview

Gradual rollout minimizes rotation risk by:

- **Canary rotation**: Rotate one instance first, validate, then proceed
- **Percentage rollout**: Deploy in waves (5% → 25% → 50% → 100%)
- **Service groups**: Coordinate rotation across related services with dependency ordering

**Why Gradual Rollout?**

Immediate rotation (all instances at once) is **high risk**:
- Issues affect entire service simultaneously
- No opportunity to detect problems early
- Difficult to attribute cause of failures

Gradual rollout provides:
- ✅ **Early detection** - Problems surface on canary/first wave
- ✅ **Limited blast radius** - Only subset affected if rotation fails
- ✅ **Easy rollback** - Roll back small subset vs. entire service
- ✅ **Confidence** - Validate at each step before proceeding

## Canary Rotation

Rotate a single canary instance first, monitor health, then roll out to remaining instances.

### Basic Canary Configuration

```yaml
services:
  web-application:
    rotation:
      strategy: canary

      canary:
        # Define instances
        discovery:
          type: explicit
          instances:
            - id: "web-app-canary-1"
              role: "canary"
            - id: "web-app-prod-1"
              role: "production"
            - id: "web-app-prod-2"
              role: "production"

        # Select canary instance
        instance_selector: "role=canary"

        # Monitor canary for 5 minutes
        health_monitoring_period: 5m

        # Abort if 2 failures detected
        failure_threshold: 2

        # Rollout waves after successful canary
        rollout_waves:
          - percentage: 10  # 10% of remaining instances
            wait: 2m
            health_monitoring: 3m

          - percentage: 50  # 50% of remaining instances
            wait: 5m
            health_monitoring: 5m

          - percentage: 100  # All remaining instances
            health_monitoring: 10m
```

### Canary Workflow

1. **Canary selected** via `instance_selector`
2. **Canary rotated** (new secret applied to canary only)
3. **Health monitoring** runs for `health_monitoring_period`
4. **Decision point**:
   - ✅ **Canary healthy** → Proceed to Wave 1
   - ❌ **Canary unhealthy** → Abort, rollback canary
5. **Wave 1** (10%): Rotate, monitor, proceed if healthy
6. **Wave 2** (50%): Rotate, monitor, proceed if healthy
7. **Wave 3** (100%): Rotate remaining instances
8. **Final validation**: All instances healthy

### Canary Instance Selection

**Explicit (config-defined)**:
```yaml
discovery:
  type: explicit
  instances:
    - id: "instance-1"
      role: "canary"
```

**Kubernetes labels**:
```yaml
discovery:
  type: kubernetes
  namespace: production
  label_selector: "app=web,role=canary"
```

**Cloud provider tags**:
```yaml
discovery:
  type: cloud
  provider: aws
  region: us-east-1
  filters:
    - key: "tag:Role"
      value: "canary"
```

### Canary Configuration Options

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `discovery` | Discovery | Yes | - | Instance discovery method |
| `instance_selector` | string | Yes | - | Selector for canary instance |
| `health_monitoring_period` | duration | No | `5m` | Canary monitoring duration |
| `failure_threshold` | int | No | 2 | Failures before abort |
| `rollout_waves` | []Wave | Yes | - | Post-canary rollout waves |

**See**: [`examples/gradual-rollout/canary.yaml`](/examples/gradual-rollout/canary.yaml)

## Percentage Rollout

Rotate instances in percentage-based waves without explicit canary.

### Basic Percentage Configuration

```yaml
services:
  microservice-cluster:
    rotation:
      strategy: percentage_rollout

      rollout:
        discovery:
          type: cloud
          provider: aws
          region: us-east-1
          filters:
            - key: "tag:Service"
              value: "microservice-cluster"

        # Progressive rollout waves
        waves:
          - percentage: 5
            health_monitoring: 2m

          - percentage: 25
            health_monitoring: 5m

          - percentage: 50
            health_monitoring: 10m

          - percentage: 100
            health_monitoring: 15m

        # Pause on failure instead of auto-rollback
        pause_on_failure: true

        # Progress persistence for resumption
        persist_progress: true
        progress_file: "/var/lib/dsops/rotation-progress/microservice.json"
```

### Wave Configuration

Each wave specifies:
- **percentage**: Percentage of *total* instances (not remaining)
- **health_monitoring**: Duration to monitor this wave
- **wait** (optional): Delay before starting next wave

**Example**: 100 instances, waves [5, 25, 50, 100]
- Wave 1: 5 instances (5%)
- Wave 2: 25 instances total (20 new + 5 from wave 1)
- Wave 3: 50 instances total (25 new + 25 from previous)
- Wave 4: 100 instances total (50 new + 50 from previous)

### Pause on Failure

When `pause_on_failure: true`:

1. **Wave fails** health checks
2. **Rollout paused** (not aborted)
3. **Operator notified** via Slack/PagerDuty
4. **Manual decision**: Continue or rollback

**Resume after investigation**:
```bash
# Option 1: Continue rollout
dsops rotation resume --service microservice-cluster

# Option 2: Rollback all rotated instances
dsops rotation rollback --service microservice-cluster --force
```

### Progress Persistence

Enable progress persistence to resume after crashes:

```yaml
rollout:
  persist_progress: true
  progress_file: "/var/lib/dsops/rotation-progress/service.json"
```

**Progress file format**:
```json
{
  "service": "microservice-cluster",
  "rotation_id": "rot-2025-12-09-abc123",
  "current_wave": 2,
  "rotated_instances": ["i-001", "i-002", "i-003"],
  "status": "paused",
  "timestamp": "2025-12-09T14:30:00Z"
}
```

### Percentage Rollout Options

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `discovery` | Discovery | Yes | - | Instance discovery |
| `waves` | []Wave | Yes | - | Rollout wave definitions |
| `pause_on_failure` | bool | No | false | Pause vs. abort on failure |
| `manual_approval_required` | bool | No | false | Require approval between waves |
| `persist_progress` | bool | No | false | Enable progress persistence |
| `progress_file` | string | No | - | Progress file path |

**See**: [`examples/gradual-rollout/percentage.yaml`](/examples/gradual-rollout/percentage.yaml)

## Service Group Rotation

Rotate multiple related services together with dependency ordering.

### Basic Service Group Configuration

```yaml
# Define service group
service_groups:
  - name: postgres-cluster
    description: "PostgreSQL primary and replicas"

    services:
      - postgres-primary
      - postgres-replica-1
      - postgres-replica-2

    rotation:
      strategy: sequential  # or "parallel"

      # Dependency order
      dependency_order:
        - postgres-primary  # Rotate primary first
        - [postgres-replica-1, postgres-replica-2]  # Then replicas in parallel

      # Failure policy
      failure_policy: rollback_all  # Rollback entire group on any failure

      # Cross-service verification
      cross_service_verification:
        enabled: true
        checks:
          - type: replication_lag
            max_lag_seconds: 10

services:
  postgres-primary:
    type: postgresql
    host: primary-db.example.com
    rotation:
      strategy: two-key

  postgres-replica-1:
    type: postgresql
    host: replica-1-db.example.com
    rotation:
      strategy: two-key

  postgres-replica-2:
    type: postgresql
    host: replica-2-db.example.com
    rotation:
      strategy: two-key
```

### Group Rotation Workflow

1. **Group rotation triggered**:
   ```bash
   dsops rotation rotate --group postgres-cluster
   ```

2. **Dependency graph built** from `dependency_order`

3. **Sequential execution**:
   - Rotate `postgres-primary`
   - Wait for health checks to pass
   - Rotate `postgres-replica-1` and `postgres-replica-2` in parallel
   - Wait for both replicas' health checks
   - Run cross-service verification (replication lag)

4. **Success** or **rollback all** on failure

### Dependency Ordering

**Sequential** (one at a time):
```yaml
dependency_order:
  - service-1
  - service-2
  - service-3
```

**Parallel** (simultaneous):
```yaml
dependency_order:
  - [service-1, service-2, service-3]
```

**Mixed** (dependencies + parallelism):
```yaml
dependency_order:
  - primary-db  # Must complete first
  - [replica-1, replica-2, replica-3]  # Then all replicas in parallel
  - cache-warmer  # Finally cache warmup
```

### Failure Policies

| Policy | Behavior | Use Case |
|--------|----------|----------|
| `rollback_all` | Rollback all services in group | Critical, tightly coupled services |
| `rollback_failed_only` | Rollback only failed service | Loosely coupled services |
| `continue` | Continue to next service | Best-effort rotation |

**Example**:
```yaml
failure_policy: rollback_all  # Primary/replica consistency critical
```

### Cross-Service Verification

Validate relationships between services after rotation.

**Replication lag check**:
```yaml
cross_service_verification:
  enabled: true
  checks:
    - type: replication_lag
      max_lag_seconds: 10
```

**Connection test**:
```yaml
cross_service_verification:
  enabled: true
  checks:
    - type: replication_connection
      timeout: 30s
```

**Custom validation**:
```yaml
cross_service_verification:
  enabled: true
  checks:
    - type: custom_script
      script: "/scripts/verify-cluster-health.sh"
```

### Service Group Options

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Group identifier |
| `description` | string | No | - | Group description |
| `services` | []string | Yes | - | Services in group |
| `rotation.strategy` | string | No | `sequential` | `sequential` or `parallel` |
| `rotation.dependency_order` | []mixed | Yes | - | Execution order |
| `rotation.failure_policy` | string | No | `rollback_all` | Failure handling |
| `cross_service_verification` | Verification | No | - | Cross-service checks |

**See**: [`examples/gradual-rollout/service-group.yaml`](/examples/gradual-rollout/service-group.yaml)

## Instance Discovery

### Discovery Types

dsops supports 4 instance discovery methods:

#### 1. Explicit (Config-Defined)

```yaml
discovery:
  type: explicit
  instances:
    - id: "instance-1"
      role: "production"
    - id: "instance-2"
      role: "canary"
```

**Use case**: Static infrastructure, known instance list

#### 2. Kubernetes

```yaml
discovery:
  type: kubernetes
  namespace: production
  label_selector: "app=web,tier=backend"
  kubeconfig: "/home/user/.kube/config"  # Optional
  context: "production-cluster"  # Optional
```

**Use case**: Kubernetes deployments, pod-based services

**Label selector syntax**: Kubernetes label selector format
- `app=web` - Match label
- `app=web,tier=backend` - Match multiple labels
- `app!=cache` - Exclude label

#### 3. Cloud Provider (AWS/GCP/Azure)

```yaml
discovery:
  type: cloud
  provider: aws  # or gcp, azure
  region: us-east-1
  filters:
    - key: "tag:Environment"
      value: "production"
    - key: "tag:Service"
      value: "web-app"
```

**Use case**: Cloud-native infrastructure, auto-scaling groups

**Providers**:
- **AWS**: EC2 instance tags, Auto Scaling groups
- **GCP**: Compute instance labels, Instance groups
- **Azure**: VM tags, Scale sets

#### 4. Endpoint (HTTP API)

```yaml
discovery:
  type: endpoint
  url: "https://api.example.com/admin/instances"
  auth:
    type: bearer
    token:
      from:
        store: vault
        key: admin/api-token
  instance_path: "$.instances[*]"
```

**Expected API response**:
```json
{
  "instances": [
    {"id": "instance-1", "status": "healthy", "role": "canary"},
    {"id": "instance-2", "status": "healthy", "role": "production"},
    {"id": "instance-3", "status": "healthy", "role": "production"}
  ]
}
```

**Use case**: Custom service registries, dynamic infrastructure

**See**: [`examples/gradual-rollout/kubernetes.yaml`](/examples/gradual-rollout/kubernetes.yaml)

### Discovery Configuration Reference

#### Explicit

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | `explicit` |
| `instances` | []Instance | Yes | Instance list |
| `instances[].id` | string | Yes | Instance identifier |
| `instances[].role` | string | No | Instance role/label |

#### Kubernetes

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | `kubernetes` |
| `namespace` | string | Yes | Kubernetes namespace |
| `label_selector` | string | Yes | Pod label selector |
| `kubeconfig` | string | No | Path to kubeconfig |
| `context` | string | No | Kubernetes context |

#### Cloud

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | `cloud` |
| `provider` | string | Yes | `aws`, `gcp`, or `azure` |
| `region` | string | Yes | Cloud region |
| `filters` | []Filter | Yes | Instance filters (tags/labels) |

#### Endpoint

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `type` | string | Yes | `endpoint` |
| `url` | string | Yes | API endpoint URL |
| `auth` | Auth | No | Authentication config |
| `instance_path` | string | Yes | JSONPath to instance list |

## Notifications for Gradual Rollout

Track wave progress with notifications.

### Wave Completion Notifications

```yaml
rotation:
  notifications:
    slack:
      webhook_url: { from: { store: vault, key: slack/webhook } }
      channel: "#deployments"
      events: [started, completed, failed, rollback]
      notify_on_wave_completion: true  # Notify after each wave
```

**Wave notification example**:
```
🔄 Canary Rotation: web-application

Wave 1 (Canary) Completed
  Instances rotated: 1/10 (10%)
  Health checks: ✅ Passed
  Duration: 5m12s

Next wave: 10% (1 instance) in 2 minutes
```

### Progress Updates

```yaml
notifications:
  webhooks:
    - name: "progress-tracker"
      url: "https://dashboard.example.com/api/rotation-progress"
      events: [started, completed, failed, rollback]
      payload_template: |
        {
          "service": "{{.ServiceName}}",
          "wave": "{{.CurrentWave}}",
          "total_waves": "{{.TotalWaves}}",
          "instances_rotated": "{{.InstancesRotated}}",
          "total_instances": "{{.TotalInstances}}",
          "status": "{{.Status}}"
        }
```

## Best Practices

### Canary Strategy

1. **Single canary instance** - One instance is sufficient
2. **Extended monitoring** - Monitor canary for 5-10 minutes
3. **Conservative threshold** - Allow 2-3 failures before abort
4. **Gradual wave progression** - 10% → 50% → 100%

```yaml
canary:
  health_monitoring_period: 10m  # Extended monitoring
  failure_threshold: 2  # Conservative
  rollout_waves:
    - percentage: 10
      wait: 2m
    - percentage: 50
      wait: 5m
    - percentage: 100
```

### Percentage Rollout

1. **Start small** - First wave should be 5-10%
2. **Increase gradually** - Each wave 2-3x previous
3. **Monitor longer per wave** - Later waves get more monitoring
4. **Pause on failure** - Manual decision for critical services

```yaml
waves:
  - percentage: 5    # Start small
    health_monitoring: 2m
  - percentage: 25   # ~5x increase
    health_monitoring: 5m
  - percentage: 50   # 2x increase
    health_monitoring: 10m
  - percentage: 100  # Remainder
    health_monitoring: 15m  # Longest monitoring
```

### Service Groups

1. **Group tightly coupled services** - Primary/replica, API/cache
2. **Order by dependency** - Rotate dependencies first
3. **Use `rollback_all`** for critical groups
4. **Verify cross-service state** - Replication, consistency

```yaml
dependency_order:
  - primary-db  # Dependency first
  - [replica-1, replica-2]  # Dependents in parallel
failure_policy: rollback_all  # Critical coupling
cross_service_verification:
  enabled: true  # Always verify
```

### Instance Discovery

1. **Prefer cloud/k8s discovery** over explicit - Handles dynamic infrastructure
2. **Use descriptive selectors** - `role=canary`, `environment=production`
3. **Test discovery** before production - Verify correct instances selected
4. **Monitor discovery errors** - Alert on empty instance lists

## Troubleshooting

### Canary always fails

**Check**:
1. Canary instance selector is correct
2. Canary instance is actually healthy (not infrastructure issue)
3. Health check thresholds are reasonable
4. Canary has same configuration as production instances

**Debug**:
```bash
# Verify canary selection
dsops rotation plan --service web-app --dry-run

# Test health checks manually on canary
DATABASE_URL="new-secret" /scripts/health/check.sh
```

### Wave gets stuck (never completes)

**Causes**:
1. Health checks never pass
2. Monitoring period too short
3. Instance discovery failing for wave

**Solutions**:
1. Increase `health_monitoring` period
2. Check health check logs
3. Verify instance discovery returns correct instances

### Rollback fails during gradual rollout

**Issue**: Only some instances rolled back

**Cause**: Wave-based rollback tracks which instances were rotated

**Solution**:
```bash
# Check rollback status
dsops rotation history --service web-app

# Manual rollback all instances
dsops rotation rollback --service web-app --all-instances --force
```

### Service group dependency deadlock

**Issue**: Services wait on each other

**Cause**: Circular dependency in `dependency_order`

**Example (wrong)**:
```yaml
dependency_order:
  - [service-a, service-b]  # Both wait for each other
```

**Solution**: Order dependencies properly
```yaml
dependency_order:
  - service-a  # Rotate first
  - service-b  # Then this (depends on service-a)
```

### Instance discovery returns empty list

**Check**:
1. **Kubernetes**: Namespace and label selector are correct
2. **Cloud**: Region and tag filters match instances
3. **Endpoint**: API endpoint is reachable and returns correct format

**Debug Kubernetes**:
```bash
kubectl get pods -n production -l "app=web,tier=backend"
```

**Debug AWS**:
```bash
aws ec2 describe-instances \
  --region us-east-1 \
  --filters "Name=tag:Service,Values=web-app"
```

## Configuration Reference

### Canary

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `discovery` | Discovery | Yes | - | Instance discovery |
| `instance_selector` | string | Yes | - | Canary selector |
| `health_monitoring_period` | duration | No | `5m` | Canary monitoring |
| `failure_threshold` | int | No | 2 | Failures before abort |
| `rollout_waves` | []Wave | Yes | - | Post-canary waves |

### Percentage Rollout

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `discovery` | Discovery | Yes | - | Instance discovery |
| `waves` | []Wave | Yes | - | Rollout waves |
| `pause_on_failure` | bool | No | false | Pause vs. abort |
| `manual_approval_required` | bool | No | false | Require approval |
| `persist_progress` | bool | No | false | Enable persistence |
| `progress_file` | string | No | - | Progress file path |

### Service Group

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | Group name |
| `services` | []string | Yes | - | Services in group |
| `rotation.strategy` | string | No | `sequential` | Strategy |
| `rotation.dependency_order` | []mixed | Yes | - | Execution order |
| `rotation.failure_policy` | string | No | `rollback_all` | Failure policy |
| `cross_service_verification` | Verification | No | - | Cross-service checks |

### Wave

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `percentage` | int | Yes | - | Percentage of total instances |
| `health_monitoring` | duration | Yes | - | Monitoring duration |
| `wait` | duration | No | `0` | Wait before starting wave |

## Related Documentation

- [Rotation Strategies](/docs/rotation/strategies) - Two-key, immediate, overlap strategies
- [Health Checks](/docs/rotation/health-checks) - Health monitoring per wave
- [Notifications](/docs/rotation/notifications) - Wave progress notifications
- [Rollback & Recovery](/docs/rotation/rollback) - Rollback during gradual rollout

## Examples

Complete working examples:
- [`examples/gradual-rollout/canary.yaml`](/examples/gradual-rollout/canary.yaml)
- [`examples/gradual-rollout/percentage.yaml`](/examples/gradual-rollout/percentage.yaml)
- [`examples/gradual-rollout/service-group.yaml`](/examples/gradual-rollout/service-group.yaml)
- [`examples/gradual-rollout/kubernetes.yaml`](/examples/gradual-rollout/kubernetes.yaml)
