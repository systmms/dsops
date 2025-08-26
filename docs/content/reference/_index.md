---
title: "Reference"
description: "Complete reference documentation for dsops"
lead: "Detailed reference documentation for all dsops features, commands, and configuration options."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 50
---

## Reference Documentation

Find detailed information about every aspect of dsops:

{{< cards >}}
  {{< card title="CLI Commands" href="/reference/cli/" >}}
    Complete reference for all dsops commands and options.
  {{< /card >}}
  {{< card title="Configuration" href="/reference/configuration/" >}}
    Full dsops.yaml configuration reference with all options.
  {{< /card >}}
  {{< card title="Implementation Status" href="/reference/status/" >}}
    Current implementation status and roadmap.
  {{< /card >}}
{{< /cards >}}

## Quick Reference

### Common Commands

```bash
dsops init                    # Initialize configuration
dsops plan --env prod        # Show resolution plan
dsops exec --env prod -- cmd # Execute with secrets
dsops render --env prod      # Render to stdout
dsops doctor                 # Check provider setup
```

### Environment Variables

- `DSOPS_CONFIG` - Alternative config file path
- `DSOPS_ENV` - Default environment
- `DSOPS_NO_COLOR` - Disable colored output
- `DSOPS_DEBUG` - Enable debug logging

### Exit Codes

- `0` - Success
- `1` - General error
- `2` - Configuration error
- `3` - Provider error
- `4` - Resolution error