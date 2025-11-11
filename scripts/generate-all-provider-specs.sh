#!/usr/bin/env bash
#
# Generate all provider specs from template
# This script generates retrospective specifications for all 14 implemented providers

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GEN_DIR="$SCRIPT_DIR/spec-gen"

# Provider mapping: spec_number:provider_type:file_name
# Format: spec_number:provider_type:file_or_dir_name
declare -a PROVIDERS=(
    "080:bitwarden:bitwarden"
    "081:onepassword:onepassword"
    "082:literal:literal"
    "083:pass:pass"
    "084:doppler:doppler"
    "085:vault:vault/vault"  # HashiCorp Vault (in vault/ subdirectory)
    "086:aws-secretsmanager:aws_secretsmanager"
    "087:aws-ssm:aws_ssm"
    "088:azure-keyvault:azure_keyvault"
    "089:gcp-secretmanager:gcp_secretmanager"
)

# Note: Some files like aws_unified.go, azure_unified.go, gcp_unified.go
# are helper files, not standalone providers.

echo "🚀 Generating provider specifications..."
echo "=========================================="
echo

GENERATED=0
FAILED=0

for entry in "${PROVIDERS[@]}"; do
    IFS=':' read -r spec_num provider_type file_name <<< "$entry"

    echo "Generating SPEC-$spec_num: $provider_type (from $file_name)..."

    if "$GEN_DIR/generate-provider-spec.sh" "$file_name" "$spec_num" 2>&1 | grep -q "✅"; then
        GENERATED=$((GENERATED + 1))
        echo "  ✅ Success"
    else
        FAILED=$((FAILED + 1))
        echo "  ❌ Failed (check if file exists: internal/providers/${file_name}.go)"
    fi
    echo
done

echo "=========================================="
echo "📊 Generation Summary:"
echo "  Generated: $GENERATED specs"
echo "  Failed: $FAILED specs"
echo "  Total: ${#PROVIDERS[@]} providers"
echo
echo "📁 Output directory: specs/providers/"
echo
echo "⚠️  Note: Generated specs contain [TODO] markers for manual completion."
echo "   Review and refine specs before committing."
echo

if [ $FAILED -eq 0 ]; then
    echo "✅ All provider specs generated successfully!"
    exit 0
else
    echo "⚠️  Some specs failed to generate. Check errors above."
    exit 1
fi
