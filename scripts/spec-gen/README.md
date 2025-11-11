# Spec Generation System

Automated system for generating retrospective specifications from the dsops codebase using type-specific templates and metadata extraction.

## Overview

This system extracts metadata from code and populates markdown templates to generate specification documents. It automates the repetitive parts of spec creation while leaving strategic sections for human refinement.

##Status

**✅ Completed**:
- Provider spec template (`scripts/spec-templates/provider-template.md`)
- Provider metadata extraction (`extract-provider-metadata.sh`)
- Provider spec generation (`generate-provider-spec.sh`)
- Orchestrator for all providers (`../generate-all-provider-specs.sh`)
- Successfully tested on Bitwarden provider (SPEC-080)

**⏳ In Progress**:
- Documentation (this file)

**🔜 Pending**:
- Command spec template and generator
- Security spec template and generator
- Rotation spec template and generator
- Service integration spec template and generator
- Validation script
- INDEX.md and STATUS.md generators

## Architecture

```
scripts/
├── spec-templates/          # Markdown templates for each spec type
│   ├── provider-template.md      ✅ Complete
│   ├── command-template.md       ⏳ TODO
│   ├── security-template.md      ⏳ TODO
│   ├── rotation-template.md      ⏳ TODO
│   └── service-template.md       ⏳ TODO
│
├── spec-gen/                # Generation and extraction scripts
│   ├── extract-provider-metadata.sh   ✅ Complete
│   ├── generate-provider-spec.sh      ✅ Complete
│   ├── extract-command-metadata.sh    ⏳ TODO
│   ├── generate-command-spec.sh       ⏳ TODO
│   └── README.md (this file)
│
├── generate-all-provider-specs.sh ✅ Complete
└── generate-all-specs.sh          ⏳ TODO (master orchestrator)
```

## Usage

### Generate Single Provider Spec

```bash
# Generate spec for a specific provider
cd /Users/jonshaffer/work/dsops-nov
./scripts/spec-gen/generate-provider-spec.sh <provider-type> <spec-number>

# Example: Generate Bitwarden spec (SPEC-080)
./scripts/spec-gen/generate-provider-spec.sh bitwarden 080
```

**Output**: `specs/providers/080-bitwarden.md`

### Generate All Provider Specs

```bash
# Generate specs for all 14 providers
./scripts/generate-all-provider-specs.sh
```

**Output**: 10+ provider specs in `specs/providers/`

### Extract Metadata Only

```bash
# Extract metadata as JSON (useful for debugging)
./scripts/spec-gen/extract-provider-metadata.sh bitwarden | jq '.'
```

## Provider Spec Template

### Template Variables

The provider template (`scripts/spec-templates/provider-template.md`) supports these variables:

| Variable | Source | Example |
|----------|--------|---------|
| `{{SPEC_NUMBER}}` | Command line arg | `080` |
| `{{PROVIDER_NAME}}` | Struct name in code | `Bitwarden` |
| `{{PROVIDER_TYPE}}` | File name | `bitwarden` |
| `{{IMPL_DATE}}` | Git log (first commit) | `2025-08-26` |
| `{{PROVIDER_DESCRIPTION}}` | File header comment | `BitwardenProvider implements...` |
| `{{INTEGRATION_METHOD}}` | Import analysis | `CLI wrapper` or `AWS SDK` |
| `{{AUTH_METHOD}}` | Validate() function | `CLI authentication (requires...)` |
| `{{MAIN_FILE}}` | File path | `internal/providers/bitwarden.go` |
| `{{TEST_COVERAGE}}` | `go test -cover` | `0.0` (N/A if no tests) |
| `{{CAP_VERSIONING}}` | Capabilities() func | `✅` or `❌` |
| `{{CAP_METADATA}}` | Capabilities() func | `✅` or `❌` |
| `{{CAP_LIST}}` | Capabilities() func | `✅` or `❌` |
| `{{CAP_ROTATION}}` | Capabilities() func | `✅` or `❌` |
| `{{CAP_ENCRYPTION}}` | Capabilities() func | `✅` or `❌` |
| `{{CONFIG_FIELDS_YAML}}` | Config struct tags | YAML field list |
| `{{EXAMPLE_STORE_NAME}}` | Generated | `bitwarden-dev` |

### Sections with [TODO] Markers

Generated specs include `[TODO]` markers for sections requiring human input:

- **Capabilities Summary**: High-level feature list
- **Test Details**: Specific test names and assertions
- **Special Features**: Provider-specific capabilities
- **Implementation Details**: Code structure and logic
- **Design Decisions**: Why specific approaches were chosen
- **Security Notes**: Security-specific considerations
- **Lessons Learned**: What went well/could improve
- **Future Enhancements**: v0.2+ improvements

**Recommendation**: After generation, search for `[TODO]` and fill in these sections with actual implementation details.

## Metadata Extraction

### How It Works

`extract-provider-metadata.sh` analyzes provider code to extract:

1. **Provider Name**: From struct name (`type BiwardenProvider struct`)
2. **Description**: From file header comments
3. **Integration Method**: From import statements (CLI, SDK, API)
4. **Auth Method**: From `Validate()` function implementation
5. **Implementation Date**: From `git log` (first commit)
6. **File Paths**: Provider file, test file, examples
7. **Capabilities**: From `Capabilities()` return value
8. **Config Fields**: From config struct `yaml` tags
9. **Test Coverage**: From `go test -cover` output

### Data Flow

```
Provider Code → extract-provider-metadata.sh → JSON metadata
                                                      ↓
Template File → generate-provider-spec.sh → Populated Spec
```

## Extending the System

### Adding New Spec Types

To add support for a new spec type (e.g., commands):

1. **Create Template** (`scripts/spec-templates/command-template.md`):
   ```markdown
   # SPEC-{{SPEC_NUMBER}}: {{COMMAND_NAME}} Command

   {{COMMAND_DESCRIPTION}}

   [... template structure ...]
   ```

2. **Create Metadata Extractor** (`scripts/spec-gen/extract-command-metadata.sh`):
   ```bash
   #!/usr/bin/env bash
   COMMAND_NAME="${1:-}"
   COMMAND_FILE="cmd/dsops/commands/${COMMAND_NAME}.go"

   # Extract metadata from command file
   # Output JSON
   ```

3. **Create Generator** (`scripts/spec-gen/generate-command-spec.sh`):
   ```bash
   #!/usr/bin/env bash
   # Similar to generate-provider-spec.sh but for commands
   ```

4. **Add to Orchestrator**:
   ```bash
   # In generate-all-specs.sh
   ./scripts/spec-gen/generate-command-spec.sh "plan" "006"
   ```

### Template Best Practices

- **Use descriptive variable names**: `{{PROVIDER_NAME}}` not `{{NAME}}`
- **Include [TODO] for strategic sections**: Areas requiring human insight
- **Preserve markdown structure**: Ensure valid markdown after substitution
- **Use consistent formatting**: Follow existing spec format (SPEC-001 through SPEC-004)
- **Add usage examples**: Show how template variables are populated

## Current Provider List

Based on `internal/providers/` file structure:

| Spec | Provider Type | File | Status |
|------|---------------|------|--------|
| 080 | bitwarden | bitwarden.go | ✅ Generated |
| 081 | onepassword | onepassword.go | ⏳ Pending |
| 082 | literal | literal.go | ⏳ Pending |
| 083 | pass | pass.go | ⏳ Pending |
| 084 | doppler | doppler.go | ⏳ Pending |
| 085 | vault | vault/ (subdir) | ⏳ Pending |
| 086 | aws-secretsmanager | aws_secretsmanager.go | ⏳ Pending |
| 087 | aws-ssm | aws_ssm.go | ⏳ Pending |
| 088 | azure-keyvault | azure_keyvault.go | ⏳ Pending |
| 089 | gcp-secretmanager | gcp_secretmanager.go | ⏳ Pending |

**Note**: Files like `aws_unified.go`, `azure_unified.go`, `gcp_unified.go` are helper files, not standalone providers.

## Next Steps

### Immediate (To Complete Spec Generation)

1. **Run provider spec generation**:
   ```bash
   ./scripts/generate-all-provider-specs.sh
   ```

2. **Review generated specs**: Search for `[TODO]` and fill in manual sections

3. **Commit provider specs**: Once reviewed and refined

### Short Term (Complete Template System)

4. **Create command spec template** and generator (SPEC-005 through SPEC-010)
5. **Create security spec template** (SPEC-020 through SPEC-029)
6. **Create rotation spec template** (SPEC-050 through SPEC-069)
7. **Create service integration template** (SPEC-070 through SPEC-079)

### Medium Term (Documentation & Automation)

8. **Generate specs/INDEX.md**: Catalog of all specs with status
9. **Generate specs/STATUS.md**: Dashboard showing completion
10. **Create validation script**: Check spec format and completeness
11. **Update CLAUDE.md**: Document spec generation workflow
12. **GitHub integration**: Issue templates, workflows, project board

## Troubleshooting

### Script Fails with "Provider not found"

**Problem**: `extract-provider-metadata.sh` can't find provider file

**Solution**: Check provider type matches filename:
```bash
ls internal/providers/*.go | grep -v _test | grep -v aws_ | grep -v azure_ | grep -v gcp_
```

### Generated Spec Has "NOT_FOUND"

**Problem**: Metadata extraction returned empty value

**Solution**: Check JSON output from extractor:
```bash
./scripts/spec-gen/extract-provider-metadata.sh bitwarden | jq '.'
```

### Template Variable Not Replaced

**Problem**: `{{VARIABLE}}` still appears in generated spec

**Solution**: Add variable replacement in `generate-provider-spec.sh`:
```bash
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{VARIABLE\}\}/$value}"
```

## References

- **Existing Specs**: `specs/features/001-cli-framework.md` through `004-transform-pipeline.md`
- **VISION.md**: Product vision and feature list
- **Provider Code**: `internal/providers/*.go`
- **Provider Tests**: `internal/providers/*_test.go`
- **Examples**: `examples/*.yaml`

## Contributing

When adding new providers or updating templates:

1. Update provider list in `generate-all-provider-specs.sh`
2. Test generation on new provider before committing
3. Document any new template variables in this README
4. Update CLAUDE.md with new generation patterns

---

**Last Updated**: 2025-11-11
**Maintainer**: See CLAUDE.md for AI assistant guidance
