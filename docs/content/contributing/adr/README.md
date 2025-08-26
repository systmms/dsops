# Architecture Decision Records (ADRs)

This directory contains architectural decision records for dsops.

## Structure

```
docs/adr/
├── README.md                    # This file
├── ADR-001-terminology-and-dsops-data.md   # Terminology and externalizing services  
# (templates moved to docs/templates/)
└── accepted/                   # Implemented ADRs (optional organization)
```

## What are ADRs?

Architecture Decision Records (ADRs) document important architectural decisions made during the development of dsops. They capture:

- **Context**: Why this decision needed to be made
- **Options**: What alternatives were considered
- **Decision**: What was chosen and why
- **Consequences**: What are the implications

## When to Create an ADR

Create an ADR for:

- **Naming conventions** (interfaces, packages, commands)
- **Architecture patterns** (how components interact)
- **Technology choices** (frameworks, libraries, protocols)
- **API design decisions** (interfaces, data formats)
- **Security model decisions** (authentication, authorization, encryption)
- **Breaking changes** (configuration format changes, CLI changes)

## ADR Lifecycle

1. **Draft** - Decision being considered, options being evaluated
2. **Accepted** - Decision made, implementation pending
3. **Implemented** - Decision implemented in code
4. **Rejected** - Decision rejected in favor of alternative
5. **Superseded** - Decision replaced by newer ADR

## Naming Convention

Use the format: `ADR-NNN-title.md` where:
- `NNN` is a zero-padded sequential number (001, 002, etc.)
- `title` is a short kebab-case description

Examples:
- `ADR-001-provider-naming.md`
- `ADR-002-rotation-interface-design.md`
- `ADR-003-configuration-format.md`

## How to Create an ADR

1. Copy the template: `cp ../templates/adr-template.md ADR-NNN-title.md`
2. Fill in all sections with thorough analysis
3. Reference related research documents from `docs/research/`
4. Get feedback from team before marking as "Accepted"
5. Update status as implementation progresses
6. Update `VISION_IMPLEMENTATION.md` when implemented

## Current ADRs

### Active Decisions

- **ADR-001**: Terminology and dsops-data externalization - Terminology for secret stores vs services, and externalizing service data

### Historical Decisions

(None yet)

## Decision Index

### Naming & Terminology
- ADR-001: Terminology and dsops-data externalization

### Architecture & Design
(Future ADRs)

### Security & Privacy
(Future ADRs)

### Configuration & APIs  
(Future ADRs)

## Contributing

When creating ADRs:

1. **Do thorough research**: Link to research documents where applicable
2. **Consider all options**: Don't just justify your preferred choice
3. **Think about consequences**: Both positive and negative implications
4. **Be specific**: Avoid vague statements, provide concrete examples
5. **Update related docs**: Ensure ADR decisions flow into implementation docs

## Links

- [ADR methodology](https://adr.github.io/) - General ADR approach
- [MADR template](https://adr.github.io/madr/) - Template inspiration
- Research documentation: [`docs/research/`](../research/README.md)
- Implementation tracking: [`VISION_IMPLEMENTATION.md`](../../VISION_IMPLEMENTATION.md)