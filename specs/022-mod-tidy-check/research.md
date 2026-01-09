# Research: Go Module Tidy Check

**Feature**: 022-mod-tidy-check
**Date**: 2026-01-09

## Summary

This is a straightforward build tooling feature with minimal research requirements. All technical decisions were clear from the specification.

## Decisions

### 1. Hook Tool Selection

**Decision**: Use Lefthook via npx

**Rationale**:
- Cross-platform support (Windows, macOS, Linux)
- No global installation required (`npx lefthook` works out of the box)
- Simple YAML configuration
- Well-maintained and widely adopted

**Alternatives Considered**:
- **Raw git hooks**: Rejected - not cross-platform, hard to manage
- **Husky**: Considered - but requires npm project setup, heavier
- **pre-commit (Python)**: Rejected - requires Python installation

### 2. Tidy Check Strategy

**Decision**: Backup-compare-restore pattern

**Rationale**:
- Non-destructive - original files preserved if check fails
- Works reliably across all platforms
- Clear error reporting with diff output

**Implementation**:
```bash
cp go.mod go.mod.bak && cp go.sum go.sum.bak
go mod tidy
if ! diff -q go.mod go.mod.bak; then
    # Report error, restore originals
fi
rm -f go.mod.bak go.sum.bak
```

### 3. CI Integration Point

**Decision**: Add step to `unit-tests` job after `go mod download`, before linting

**Rationale**:
- Fails fast before expensive checks (linting, tests)
- Dependencies already downloaded (needed for `go mod tidy`)
- Single point of enforcement (no need to duplicate in other jobs)

## No Further Research Needed

This feature is well-defined with standard tooling. No external dependencies, APIs, or complex integrations requiring additional investigation.
