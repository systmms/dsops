#!/usr/bin/env bash
set -euo pipefail

echo "🔄 Migrating dsops specs to standard spec-kit directory structure..."

# Check we're in the right directory
if [[ ! -d "specs" ]]; then
  echo "❌ Error: Must run from repository root with specs/ directory"
  exit 1
fi

# Migrate features: 001-004, 005
echo "📁 Migrating feature specs..."
if [[ -d "specs/features" ]]; then
  for spec in specs/features/*.md; do
    if [[ -f "$spec" ]]; then
      basename=$(basename "$spec" .md)
      echo "  - $basename"
      mkdir -p "specs/$basename"
      git mv "$spec" "specs/$basename/spec.md"
    fi
  done

  # Remove empty features directory
  if [[ -z "$(ls -A specs/features 2>/dev/null)" ]]; then
    rmdir specs/features
    echo "  ✓ Removed empty specs/features/"
  fi
fi

# Migrate providers: 080-089
echo "📁 Migrating provider specs..."
if [[ -d "specs/providers" ]]; then
  for spec in specs/providers/*.md; do
    if [[ -f "$spec" ]]; then
      basename=$(basename "$spec" .md)
      echo "  - $basename"
      mkdir -p "specs/$basename"
      git mv "$spec" "specs/$basename/spec.md"
    fi
  done

  # Remove empty providers directory
  if [[ -z "$(ls -A specs/providers 2>/dev/null)" ]]; then
    rmdir specs/providers
    echo "  ✓ Removed empty specs/providers/"
  fi
fi

# Migrate rotation specs: 050
echo "📁 Migrating rotation specs..."
if [[ -d "specs/rotation" ]]; then
  for spec in specs/rotation/*.md; do
    if [[ -f "$spec" ]]; then
      basename=$(basename "$spec" .md)
      echo "  - $basename"
      mkdir -p "specs/$basename"
      git mv "$spec" "specs/$basename/spec.md"
    fi
  done

  # Remove empty rotation directory
  if [[ -z "$(ls -A specs/rotation 2>/dev/null)" ]]; then
    rmdir specs/rotation
    echo "  ✓ Removed empty specs/rotation/"
  fi
fi

# Summary
echo ""
echo "✅ Migration complete!"
echo ""
echo "📊 New structure:"
find specs -name "spec.md" | sort
echo ""
echo "Total specs migrated: $(find specs -name 'spec.md' | wc -l | tr -d ' ')"
