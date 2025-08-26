package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
)

func NewInstallHookCommand(cfg *config.Config) *cobra.Command {
	var (
		path      string
		force     bool
		uninstall bool
	)

	cmd := &cobra.Command{
		Use:   "install-hook",
		Short: "Install pre-commit hook to prevent secret leaks",
		Long: `Install a pre-commit git hook that prevents committing secrets.

The hook will:
- Scan staged files for potential secrets before each commit
- Block commits containing potential secrets
- Provide suggestions for fixing issues

Examples:
  dsops install-hook                    # Install in current repository
  dsops install-hook --path /repo       # Install in specific repository
  dsops install-hook --force            # Overwrite existing hook
  dsops install-hook --uninstall        # Remove the hook

Note: This creates a pre-commit hook that calls 'dsops guard repo' to scan
staged changes. Ensure dsops is available in your PATH for the hook to work.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if uninstall {
				return uninstallPreCommitHook(path)
			}
			return installPreCommitHook(path, force)
		},
	}

	cmd.Flags().StringVar(&path, "path", ".", "Repository path")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Overwrite existing hook")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "Remove the pre-commit hook")

	return cmd
}

func installPreCommitHook(repoPath string, force bool) error {
	// Verify this is a git repository
	if !isGitRepository(repoPath) {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Not a git repository: %s", repoPath),
			Suggestion: "Navigate to a git repository or specify --path to one",
		}
	}

	hooksDir := filepath.Join(repoPath, ".git", "hooks")
	hookPath := filepath.Join(hooksDir, "pre-commit")

	fmt.Printf("🔧 Installing pre-commit hook: %s\n", hookPath)

	// Check if hook already exists
	if _, err := os.Stat(hookPath); err == nil && !force {
		return dserrors.UserError{
			Message:    "Pre-commit hook already exists",
			Suggestion: "Use --force to overwrite or --uninstall to remove",
			Details:    fmt.Sprintf("Existing hook: %s", hookPath),
		}
	}

	// Ensure hooks directory exists
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return dserrors.UserError{
			Message:    "Failed to create hooks directory",
			Details:    err.Error(),
			Suggestion: "Check repository permissions",
		}
	}

	// Create the pre-commit hook script
	hookContent := generatePreCommitHook()

	// Write the hook file
	if err := os.WriteFile(hookPath, []byte(hookContent), 0755); err != nil {
		return dserrors.UserError{
			Message:    "Failed to write pre-commit hook",
			Details:    err.Error(),
			Suggestion: "Check file permissions and disk space",
		}
	}

	fmt.Println("✅ Pre-commit hook installed successfully")
	fmt.Println()
	fmt.Println("📋 Hook behavior:")
	fmt.Println("  • Scans staged files for potential secrets before each commit")
	fmt.Println("  • Blocks commits if secrets are detected")
	fmt.Println("  • Provides guidance on fixing issues")
	fmt.Println()
	fmt.Println("🛠️  To test the hook:")
	fmt.Printf("  git add <files> && git commit -m \"test\"\n")
	fmt.Println()
	fmt.Println("🗑️  To remove the hook:")
	fmt.Println("  dsops install-hook --uninstall")

	return nil
}

func uninstallPreCommitHook(repoPath string) error {
	// Verify this is a git repository
	if !isGitRepository(repoPath) {
		return dserrors.UserError{
			Message:    fmt.Sprintf("Not a git repository: %s", repoPath),
			Suggestion: "Navigate to a git repository or specify --path to one",
		}
	}

	hookPath := filepath.Join(repoPath, ".git", "hooks", "pre-commit")

	fmt.Printf("🗑️  Removing pre-commit hook: %s\n", hookPath)

	// Check if hook exists
	if _, err := os.Stat(hookPath); os.IsNotExist(err) {
		fmt.Println("ℹ️  No pre-commit hook found")
		return nil
	}

	// Read the existing hook to verify it's our hook
	content, err := os.ReadFile(hookPath)
	if err != nil {
		return dserrors.UserError{
			Message:    "Failed to read existing hook",
			Details:    err.Error(),
			Suggestion: "Check file permissions",
		}
	}

	// Check if it contains our marker
	if !strings.Contains(string(content), "# dsops pre-commit hook") {
		return dserrors.UserError{
			Message:    "Existing pre-commit hook was not installed by dsops",
			Suggestion: "Remove manually or use --force to overwrite",
			Details:    "Hook content doesn't contain dsops marker",
		}
	}

	// Remove the hook
	if err := os.Remove(hookPath); err != nil {
		return dserrors.UserError{
			Message:    "Failed to remove pre-commit hook",
			Details:    err.Error(),
			Suggestion: "Check file permissions",
		}
	}

	fmt.Println("✅ Pre-commit hook removed successfully")
	return nil
}

func generatePreCommitHook() string {
	return `#!/bin/bash
# dsops pre-commit hook
# This hook prevents committing potential secrets

set -e

echo "🔍 dsops: Scanning staged files for secrets..."

# Check if dsops is available
if ! command -v dsops &> /dev/null; then
    echo "❌ dsops not found in PATH"
    echo "   Install dsops or add it to your PATH to use this hook"
    exit 1
fi

# Get list of staged files
staged_files=$(git diff --cached --name-only --diff-filter=ACM)

if [ -z "$staged_files" ]; then
    echo "ℹ️  No staged files to scan"
    exit 0
fi

# Create temporary directory for staged content
temp_dir=$(mktemp -d)
trap "rm -rf $temp_dir" EXIT

# Extract staged content to temporary files
echo "$staged_files" | while IFS= read -r file; do
    if [ -f "$file" ]; then
        # Create directory structure in temp
        mkdir -p "$temp_dir/$(dirname "$file")"
        # Get staged version of file
        git show ":$file" > "$temp_dir/$file" 2>/dev/null || true
    fi
done

# Scan the staged content
echo "🔍 Scanning staged changes for secrets..."

# Run dsops guard on the temporary directory with staged content
if ! dsops guard repo --path "$temp_dir" --exit-code 2>/dev/null; then
    echo ""
    echo "❌ Potential secrets detected in staged files!"
    echo ""
    echo "🛡️  Security recommendations:"
    echo "   1. Remove secrets from staged files"
    echo "   2. Use dsops to manage secrets properly"
    echo "   3. Add secret files to .gitignore"
    echo "   4. Consider using git filter-branch if secrets are in history"
    echo ""
    echo "🔧 To bypass this check (NOT RECOMMENDED):"
    echo "   git commit --no-verify -m \"your message\""
    echo ""
    exit 1
fi

echo "✅ No secrets detected in staged files"
exit 0
`
}