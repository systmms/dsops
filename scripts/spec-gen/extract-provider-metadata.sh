#!/usr/bin/env bash
#
# Extract metadata for provider specs from codebase
# Usage: ./extract-provider-metadata.sh <provider-type>
# Output: JSON metadata for template population

set -euo pipefail

PROVIDER_TYPE="${1:-}"
if [ -z "$PROVIDER_TYPE" ]; then
    echo "Usage: $0 <provider-type>" >&2
    echo "Example: $0 bitwarden" >&2
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PROVIDERS_DIR="$PROJECT_ROOT/internal/providers"

# Handle both flat files (bitwarden.go) and subdirectories (vault/vault.go)
if [[ "$PROVIDER_TYPE" == *"/"* ]]; then
    # Subdirectory case (e.g., vault/vault)
    PROVIDER_FILE="$PROVIDERS_DIR/${PROVIDER_TYPE}.go"
else
    # Flat file case
    PROVIDER_FILE="$PROVIDERS_DIR/${PROVIDER_TYPE}.go"
fi

if [ ! -f "$PROVIDER_FILE" ]; then
    echo "Error: Provider file not found: $PROVIDER_FILE" >&2
    echo "Available providers:" >&2
    ls "$PROVIDERS_DIR"/*.go 2>/dev/null | grep -v "_test.go\|registry.go" | xargs -n1 basename | sed 's/.go$//' | head -10 >&2
    echo "..." >&2
    ls -d "$PROVIDERS_DIR"/*/ 2>/dev/null | xargs -n1 basename >&2
    exit 1
fi

PROVIDER_TEST_FILE="${PROVIDER_FILE%.go}_test.go"

# Helper functions
extract_provider_name() {
    # Extract from struct name or capitalize provider type
    local struct_name=$(grep "type.*Provider struct" "$PROVIDER_FILE" | head -1 | sed 's/type //;s/Provider.*//')
    if [ -n "$struct_name" ]; then
        echo "$struct_name"
    else
        # Fallback: capitalize first letter of provider type (handle paths)
        local base_name=$(basename "$PROVIDER_TYPE")
        echo "$base_name" | sed 's/_/ /g; s/-/ /g; s/\b\(.\)/\u\1/g'
    fi
}

extract_description() {
    # Extract from file header comments
    local desc=$(head -20 "$PROVIDER_FILE" | grep "^//" | grep -i "provides\|integrates\|implements" | head -1 | sed 's|^// ||; s/\.$//')
    if [ -n "$desc" ]; then
        echo "$desc"
    else
        echo "$PROVIDER_TYPE secret management system"
    fi
}

extract_integration_method() {
    # Determine CLI, SDK, or API based on imports
    if grep -q "exec.Command" "$PROVIDER_FILE" 2>/dev/null; then
        echo "CLI wrapper (executes ${PROVIDER_TYPE} command-line tool)"
    elif grep -q "github.com/aws/aws-sdk-go" "$PROVIDER_FILE" 2>/dev/null; then
        echo "AWS SDK for Go"
    elif grep -q "cloud.google.com" "$PROVIDER_FILE" 2>/dev/null; then
        echo "Google Cloud SDK"
    elif grep -q "github.com/Azure" "$PROVIDER_FILE" 2>/dev/null; then
        echo "Azure SDK for Go"
    elif grep -q "github.com/hashicorp/vault" "$PROVIDER_FILE" 2>/dev/null; then
        echo "HashiCorp Vault Go client"
    elif grep -q "http.Client\|http.NewRequest" "$PROVIDER_FILE" 2>/dev/null; then
        echo "HTTP API client"
    else
        echo "Direct integration"
    fi
}

extract_auth_method() {
    # Examine Validate() function for auth method
    local validate_func=$(grep -A20 "func.*Validate.*context.Context" "$PROVIDER_FILE" | head -30)

    if echo "$validate_func" | grep -q "exec.Command"; then
        echo "CLI authentication (requires ${PROVIDER_TYPE} CLI to be logged in)"
    elif echo "$validate_func" | grep -q "AWS_\|aws.Config"; then
        echo "AWS credentials (from environment variables, config file, or IAM role)"
    elif echo "$validate_func" | grep -q "GOOGLE_\|google."; then
        echo "Google Cloud credentials (Application Default Credentials or service account key)"
    elif echo "$validate_func" | grep -q "AZURE_\|azure."; then
        echo "Azure credentials (from environment variables or Azure CLI)"
    elif echo "$validate_func" | grep -q "VAULT_"; then
        echo "HashiCorp Vault token (from VAULT_TOKEN environment variable)"
    else
        echo "Provider-specific authentication method"
    fi
}

extract_main_file() {
    # Return path relative to project root
    echo "${PROVIDER_FILE#$PROJECT_ROOT/}"
}

extract_test_files() {
    # Find test file(s) for this provider
    if [ -f "$PROVIDER_TEST_FILE" ]; then
        echo "["
        echo "  \"${PROVIDER_TEST_FILE#$PROJECT_ROOT/}\""
        echo "]"
    else
        echo "[]"
    fi
}

extract_example_files() {
    # Find example config files
    find "$PROJECT_ROOT/examples" -name "*${PROVIDER_TYPE}*.yaml" | sed "s|$PROJECT_ROOT/||" | jq -R . | jq -s .
}

extract_impl_date() {
    # Get first commit date for provider file
    git -C "$PROJECT_ROOT" log --format="%ai" --diff-filter=A -- "$PROVIDER_FILE" 2>/dev/null | head -1 | cut -d' ' -f1 || echo "2025-08-26"
}

extract_capabilities() {
    # Parse Capabilities() function
    local caps_func=$(grep -A30 "func.*Capabilities.*provider.Capabilities" "$PROVIDER_FILE" 2>/dev/null || echo "")

    local versioning="❌"
    local metadata="✅"
    local list="❌"
    local rotation="❌"
    local encryption="✅"

    if echo "$caps_func" | grep -q "Versioning.*true"; then
        versioning="✅"
    fi
    if echo "$caps_func" | grep -q "Metadata.*false"; then
        metadata="❌"
    fi
    if echo "$caps_func" | grep -q "List.*true"; then
        list="✅"
    fi
    if echo "$caps_func" | grep -q "Rotation.*true"; then
        rotation="✅"
    fi

    cat <<EOF
"versioning": "$versioning",
"metadata": "$metadata",
"list": "$list",
"rotation": "$rotation",
"encryption": "$encryption"
EOF
}

extract_config_fields() {
    # Extract config struct fields from provider file
    local config_struct=$(grep -A50 "type.*Config struct" "$PROVIDER_FILE" 2>/dev/null | grep "yaml:" || echo "")

    if [ -z "$config_struct" ]; then
        echo "    # Provider-specific configuration fields"
        return
    fi

    echo "$config_struct" | while IFS= read -r line; do
        local field=$(echo "$line" | sed 's/.*yaml:"\([^"]*\)".*/\1/')
        local comment=$(echo "$line" | sed 's/.*\/\/ \(.*\)/\1/')
        if [ -n "$field" ] && [ "$field" != "$line" ]; then
            if [ -n "$comment" ] && [ "$comment" != "$line" ]; then
                printf "    %s: value  # %s\n" "$field" "$comment"
            else
                printf "    %s: value\n" "$field"
            fi
        fi
    done
}

extract_test_coverage() {
    # Get test coverage for provider (skip if tests don't exist)
    if [ -f "$PROVIDER_TEST_FILE" ]; then
        local coverage=$(cd "$PROJECT_ROOT" && go test -cover "./internal/providers" -run "Test.*${PROVIDER_TYPE}" 2>/dev/null | grep -o "[0-9.]*%" | head -1 | sed 's/%//' || echo "N/A")
        echo "$coverage"
    else
        echo "N/A"
    fi
}

# Generate JSON metadata
cat <<EOF
{
  "SPEC_NUMBER": "XXX",
  "PROVIDER_NAME": "$(extract_provider_name)",
  "PROVIDER_TYPE": "$PROVIDER_TYPE",
  "PROVIDER_DESCRIPTION": "$(extract_description)",
  "INTEGRATION_METHOD": "$(extract_integration_method)",
  "AUTH_METHOD": "$(extract_auth_method)",
  "IMPL_DATE": "$(extract_impl_date)",
  "MAIN_FILE": "$(extract_main_file)",
  "MAIN_FILE_TEST": "$(extract_main_file | sed 's/\.go$/_test.go/')",
  "TEST_FILES": $(extract_test_files),
  "EXAMPLE_FILES": $(extract_example_files),
  "TEST_COVERAGE": "$(extract_test_coverage)",
  "CAPABILITIES": {
    $(extract_capabilities)
  },
  "CONFIG_FIELDS": "$(extract_config_fields | tr '\n' '|' | sed 's/|$//' | sed 's/"/\\"/g')",
  "EXAMPLE_STORE_NAME": "${PROVIDER_TYPE}-dev",
  "REGISTRY_ENTRY": "internal/providers/registry.go"
}
EOF
