---
title: "Architecture"
description: "Understanding dsops architecture and design"
lead: "Learn about dsops' architecture, security model, and design decisions that make it a secure and extensible secret management tool."
date: 2024-08-26T12:00:00-07:00
lastmod: 2024-08-26T12:00:00-07:00
draft: false
weight: 40
---

## Core Architecture

dsops is built with security and extensibility at its core:

{{< cards >}}
  {{< card title="Terminology" href="/architecture/terminology/" >}}
    Key concepts and definitions used throughout dsops.
  {{< /card >}}
  {{< card title="Security Model" href="/architecture/security/" >}}
    How dsops keeps your secrets safe.
  {{< /card >}}
  {{< card title="Data Integration" href="/architecture/data-integration/" >}}
    The dsops-data repository and service definitions.
  {{< /card >}}
{{< /cards >}}

## Design Principles

1. **Ephemeral by Default**: Secrets never touch disk unless explicitly requested
2. **Provider Agnostic**: Clean abstraction between secret stores and services
3. **Fail Secure**: Any error results in no secret exposure
4. **Audit Everything**: Comprehensive logging with automatic redaction

## Component Overview

```mermaid
graph TD
    A[dsops CLI] --> B[Config Parser]
    B --> C[Provider Registry]
    C --> D[Secret Stores]
    C --> E[Services]
    D --> F[Resolution Engine]
    E --> G[Rotation Engine]
    F --> H[Transform Pipeline]
    H --> I[Output/Execution]
```

## Key Components

### Provider Interface
Unified interface for all secret stores and services, enabling consistent behavior across different backends.

### Resolution Engine
Handles dependency graphs, circular reference detection, and parallel resolution of secrets.

### Transform Pipeline
Composable transformations for secret values (base64, JSON extraction, templating).

### Rotation Engine
Manages the lifecycle of secret rotation with support for multiple strategies and rollback capabilities.