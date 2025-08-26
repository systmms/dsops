---
title: "Secret Rotation"
description: "Automated secret rotation for enhanced security"
lead: "Keep your secrets fresh with automated rotation strategies. dsops supports multiple rotation patterns and provides full audit trails."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 30
---

## Why Rotate Secrets?

Regular secret rotation is essential for:

- **Compliance**: Meet PCI-DSS, SOC2, and other standards
- **Security**: Limit exposure window if secrets are compromised
- **Hygiene**: Remove stale credentials and access
- **Audit**: Track who has access and when

## Rotation Capabilities

{{< cards >}}
  {{< card title="Automated Rotation" href="/rotation/strategies/" >}}
    Multiple strategies including immediate, two-key, and gradual rotation.
  {{< /card >}}
  {{< card title="Rotation Commands" href="/rotation/commands/" >}}
    Monitor status, view history, and manage rotation operations.
  {{< /card >}}
  {{< card title="Provider Support" href="/rotation/capabilities/" >}}
    See which providers support rotation and their capabilities.
  {{< /card >}}
{{< /cards >}}

## Quick Example

```bash
# Rotate a secret
dsops secrets rotate --env production --key DATABASE_PASSWORD

# Check rotation status
dsops rotation status

# View rotation history
dsops rotation history postgres-prod
```

## Rotation Strategies

dsops supports multiple rotation strategies:

1. **Immediate Rotation**: Replace secret instantly
2. **Two-Key Rotation**: Overlap period with both secrets valid
3. **Gradual Rotation**: Percentage-based rollout
4. **Canary Rotation**: Test with subset first

## Best Practices

- **Schedule Regular Rotations**: Use cron or CI/CD
- **Monitor Rotation Status**: Set up alerts for failures
- **Test Rotation**: Use dry-run mode first
- **Document Dependencies**: Know what uses each secret