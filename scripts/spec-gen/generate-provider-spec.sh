#!/usr/bin/env bash
#
# Generate a provider spec from template and metadata
# Usage: ./generate-provider-spec.sh <provider-type> <spec-number>

set -euo pipefail

PROVIDER_TYPE="${1:-}"
SPEC_NUMBER="${2:-}"

if [ -z "$PROVIDER_TYPE" ] || [ -z "$SPEC_NUMBER" ]; then
    echo "Usage: $0 <provider-type> <spec-number>" >&2
    echo "Example: $0 bitwarden 080" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEMPLATE_FILE="$SCRIPT_DIR/../spec-templates/provider-template.md"
OUTPUT_DIR="$PROJECT_ROOT/specs/providers"

# Sanitize provider type for filename (remove slashes, use only basename)
PROVIDER_BASENAME=$(basename "$PROVIDER_TYPE")
OUTPUT_FILE="$OUTPUT_DIR/${SPEC_NUMBER}-${PROVIDER_BASENAME}.md"

# Extract metadata
echo "Extracting metadata for $PROVIDER_TYPE..." >&2
METADATA=$("$SCRIPT_DIR/extract-provider-metadata.sh" "$PROVIDER_TYPE")

# Helper function to extract JSON field
get_field() {
    local field=$1
    echo "$METADATA" | jq -r ".$field // \"NOT_FOUND\""
}

# Read template
TEMPLATE_CONTENT=$(cat "$TEMPLATE_FILE")

# Replace template variables (simple version)
OUTPUT_CONTENT="$TEMPLATE_CONTENT"

# Replace basic fields
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{SPEC_NUMBER\}\}/$SPEC_NUMBER}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PROVIDER_NAME\}\}/$(get_field PROVIDER_NAME)}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PROVIDER_TYPE\}\}/$PROVIDER_TYPE}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{IMPL_DATE\}\}/$(get_field IMPL_DATE)}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PROVIDER_DESCRIPTION\}\}/$(get_field PROVIDER_DESCRIPTION)}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{INTEGRATION_METHOD\}\}/$(get_field INTEGRATION_METHOD)}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUTH_METHOD\}\}/$(get_field AUTH_METHOD)}"

# Replace file paths
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{MAIN_FILE\}\}/$(get_field MAIN_FILE)}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{MAIN_FILE_TEST\}\}/$(get_field MAIN_FILE_TEST)}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{REGISTRY_ENTRY\}\}/$(get_field REGISTRY_ENTRY)}"

# Replace capabilities
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_VERSIONING\}\}/$(echo "$METADATA" | jq -r '.CAPABILITIES.versioning')}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_METADATA\}\}/$(echo "$METADATA" | jq -r '.CAPABILITIES.metadata')}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_LIST\}\}/$(echo "$METADATA" | jq -r '.CAPABILITIES.list')}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_ROTATION\}\}/$(echo "$METADATA" | jq -r '.CAPABILITIES.rotation')}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_ENCRYPTION\}\}/$(echo "$METADATA" | jq -r '.CAPABILITIES.encryption')}"

# Test coverage
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{TEST_COVERAGE\}\}/$(get_field TEST_COVERAGE)}"

# Example store name
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{EXAMPLE_STORE_NAME\}\}/$(get_field EXAMPLE_STORE_NAME)}"

# Config fields YAML
CONFIG_FIELDS_YAML=$(get_field CONFIG_FIELDS | tr '|' '\n')
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CONFIG_FIELDS_YAML\}\}/$CONFIG_FIELDS_YAML}"

# Replace remaining placeholder sections with TODOs for manual completion
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAPABILITIES_SUMMARY\}\}/[TODO: List key capabilities]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUTH_CREDENTIAL\}\}/credentials}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUTH_TESTS\}\}/}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{FETCH_TESTS\}\}/}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{SPECIAL_FEATURES\}\}/[Provider-specific features]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{SPECIAL_FEATURES_TESTS\}\}/}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PROVIDER_STRUCT_NAME\}\}/$(get_field PROVIDER_NAME)Provider}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{STRUCT_FIELDS\}\}/[TODO: Add struct fields]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{RESOLVE_IMPLEMENTATION\}\}/[TODO: Describe resolution logic]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{DESCRIBE_IMPLEMENTATION\}\}/[TODO: Describe metadata logic]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{VALIDATE_IMPLEMENTATION\}\}/[TODO: Describe validation logic]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAPABILITIES\}\}/[TODO: List capabilities]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CONFIG_FIELDS_TABLE\}\}/[TODO: Add config fields table]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUTH_DETAILS\}\}/[TODO: Add auth details]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUTH_STEP_1\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUTH_STEP_2\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUTH_STEP_3\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{RESOLUTION_STEP_3\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{RESOLUTION_STEP_4\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PATH_EXAMPLE\}\}/path/to/secret}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PATH_FORMAT_DESCRIPTION\}\}/[TODO: Describe path format]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PATH_EXAMPLES\}\}/[TODO: Add path examples]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_VERSIONING_NOTES\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_METADATA_NOTES\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_LIST_NOTES\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_ROTATION_NOTES\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CAP_ENCRYPTION_NOTES\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{DESIGN_DECISIONS\}\}/[TODO: Add design decisions]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{TRADEOFFS\}\}/[TODO: Add tradeoffs]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{CREDENTIAL_STORAGE\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{NETWORK_SECURITY\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{SECRET_LIFETIME\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{AUDIT_TRAIL\}\}/[TODO]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{SECURITY_NOTES\}\}/}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{TEST_DESCRIPTION\}\}/Unit and integration tests}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{ADDITIONAL_TEST_FILES\}\}/}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PROVIDER_SPECIFIC_TESTS\}\}/}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{INTEGRATION_TEST_NOTES\}\}/[TODO: Add integration test notes]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PROVIDER_SLUG\}\}/$PROVIDER_TYPE}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{EXAMPLE_FILES\}\}/$(get_field EXAMPLE_FILES | jq -r '.[]' | tr '\n' ',' | sed 's/,$//')}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{EXAMPLE_CONFIG_SNIPPET\}\}/[TODO: Add example config]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{LESSONS_GOOD\}\}/[TODO: What went well]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{LESSONS_IMPROVE\}\}/[TODO: What could improve]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{PROVIDER_NOTES\}\}/[TODO: Provider-specific notes]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{FUTURE_ENHANCEMENTS\}\}/[TODO: Future enhancements]}"
OUTPUT_CONTENT="${OUTPUT_CONTENT//\{\{RELATED_SPECS\}\}/}"

# Create output directory
mkdir -p "$OUTPUT_DIR"

# Write output
echo "$OUTPUT_CONTENT" > "$OUTPUT_FILE"

echo "✅ Generated spec: $OUTPUT_FILE" >&2
echo "$OUTPUT_FILE"
