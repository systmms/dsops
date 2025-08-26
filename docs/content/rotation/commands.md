---
title: "Rotation Commands"
description: "CLI commands for managing secret rotation"
lead: "The dsops rotation command family provides visibility into secret rotation operations, history, and compliance status."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 20
---

## Overview

Secret rotation is critical for security compliance. The rotation commands help you:

- Monitor rotation status across all services
- Track rotation history for audit trails
- Identify services requiring rotation
- Analyze rotation metrics and patterns

## Commands

### `dsops rotation status`

Display the current rotation status for services.

```bash
# Show status for all services
dsops rotation status

# Show status for a specific service
dsops rotation status postgres-prod

# Show status in JSON format (for automation)
dsops rotation status --format json

# Show verbose output with additional details
dsops rotation status --verbose
```

#### Output Format

The status command displays:
- **Service Name**: The service identifier
- **Status**: Current rotation state (✅ Active, 🔄 Rotating, ❌ Failed, ⚪ Never Rotated, 🟡 Needs Rotation)
- **Last Rotation**: When the service was last rotated
- **Next Rotation**: Scheduled next rotation time (based on TTL)
- **Result**: Last rotation result

#### Example Output

```
SERVICE         STATUS            LAST ROTATION   NEXT ROTATION   RESULT
-------         ------            -------------   -------------   ------
postgres-prod   ✅ Active         24 hr ago       in 6 days       ✅ Success
stripe-api      🟡 Needs Rotation 31 days ago     overdue         ✅ Success
mongo-dev       ⚪ Never Rotated   Never           Not scheduled   -
redis-cache     ❌ Failed         2 hr ago        in 7 days       ❌ Failed
```

### `dsops rotation history`

Show historical rotation events for auditing and troubleshooting.

```bash
# Show history for all services (last 50 entries)
dsops rotation history

# Show history for a specific service
dsops rotation history postgres-prod

# Limit the number of entries
dsops rotation history --limit 10

# Filter by date range
dsops rotation history --since 2024-01-01 --until 2024-12-31

# Filter by status
dsops rotation history --status failed

# Export as JSON for processing
dsops rotation history --format json > rotation-audit.json
```

#### Output Format

The history command displays:
- **Timestamp**: When the rotation occurred
- **Service**: Service name
- **Type**: Credential type (password, api_key, certificate, etc.)
- **Status**: Result of the rotation
- **Duration**: How long the rotation took
- **Error**: Error message if failed

#### Example Output

```
TIMESTAMP            SERVICE         TYPE      STATUS        DURATION  ERROR
---------            -------         ----      ------        --------  -----
2024-12-01 14:30:45  postgres-prod   password  ✅ Success    2.3s      -
2024-12-01 14:28:12  stripe-api      api_key   ✅ Success    1.1s      -
2024-12-01 14:25:33  redis-cache     password  ❌ Failed     0.5s      Connection refused
2024-12-01 10:15:22  mongo-dev       cert      ✅ Success    5.2s      -
```

## Storage Location

Rotation metadata is stored locally in:
- Linux/macOS: `~/.local/share/dsops/rotation/`
- Windows: `%LOCALAPPDATA%\dsops\rotation\`
- Custom: Set `DSOPS_ROTATION_DIR` environment variable

Files are organized as:
```
rotation/
├── status/
│   ├── postgres-prod.json
│   ├── stripe-api.json
│   └── ...
└── history/
    ├── postgres-prod/
    │   ├── 2024-12-01T14:30:45Z-<id>.json
    │   └── ...
    └── ...
```

## Integration with Rotation Engine

The rotation commands automatically integrate with the dsops rotation engine. When rotations occur:

1. Status is updated immediately
2. History entries are created with full audit trails
3. Next rotation times are calculated based on TTL
4. Metrics are aggregated for compliance reporting

## Use Cases

### Compliance Reporting

Generate rotation compliance reports:

```bash
# Export all rotation history for the last quarter
dsops rotation history \
  --since 2024-10-01 \
  --until 2024-12-31 \
  --format json > q4-rotation-audit.json

# Check services needing rotation
dsops rotation status --format json | \
  jq -r '.[] | select(.status == "needs_rotation") | .service_name'
```

### Monitoring Integration

Monitor rotation status in your observability stack:

```bash
# Prometheus/Grafana integration
dsops rotation status --format json | \
  jq -r '.[] | "\(.service_name)_last_rotation_hours \((now - (.last_rotation | fromdateiso8601)) / 3600)"'
```

### Alerting on Failed Rotations

```bash
# Check for recent failures
failed=$(dsops rotation history --limit 10 --status failed --format json | jq -r '.count')
if [ "$failed" -gt 0 ]; then
  echo "ALERT: $failed rotation failures detected"
fi
```

## Best Practices

1. **Regular Status Checks**: Run `dsops rotation status` daily to identify services needing rotation
2. **History Retention**: The system automatically retains 90 days of history by default
3. **JSON Export**: Use JSON format for integration with monitoring and alerting systems
4. **Audit Trails**: Export rotation history monthly for compliance records

## Troubleshooting

### No Rotation History Found

If you see "No rotation history found", it means:
- No rotations have been performed yet
- The rotation storage directory is not accessible
- The service name doesn't match any configured services

### Storage Permission Issues

Ensure the storage directory has proper permissions:
```bash
chmod -R 700 ~/.local/share/dsops/rotation
```

### Custom Storage Location

To use a custom storage location:
```bash
export DSOPS_ROTATION_DIR=/path/to/rotation/storage
dsops rotation status
```