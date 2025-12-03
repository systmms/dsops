package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
)

func TestNewInstallHookCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewInstallHookCommand(cfg)

	assert.Equal(t, "install-hook", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("path"))
	assert.NotNil(t, flags.Lookup("force"))
	assert.NotNil(t, flags.Lookup("uninstall"))
}

func TestGeneratePreCommitHook(t *testing.T) {
	t.Parallel()

	hook := generatePreCommitHook()

	// Verify hook content
	assert.Contains(t, hook, "#!/bin/bash")
	assert.Contains(t, hook, "# dsops pre-commit hook")
	assert.Contains(t, hook, "dsops guard repo")
	assert.Contains(t, hook, "git diff --cached")
}

func TestInstallPreCommitHook_NotGitRepo(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	err := installPreCommitHook(tempDir, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not a git repository")
}

func TestInstallPreCommitHook_Success(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a fake git directory
	gitDir := filepath.Join(tempDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	err := installPreCommitHook(tempDir, false)
	require.NoError(t, err)

	// Verify hook was created
	hookPath := filepath.Join(gitDir, "hooks", "pre-commit")
	_, err = os.Stat(hookPath)
	assert.NoError(t, err)

	// Verify hook content
	content, err := os.ReadFile(hookPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "# dsops pre-commit hook")
}

func TestInstallPreCommitHook_HookExists(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a fake git directory with existing hook
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	hookPath := filepath.Join(hooksDir, "pre-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte("existing hook"), 0755))

	// Should fail without --force
	err := installPreCommitHook(tempDir, false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Pre-commit hook already exists")

	// Should succeed with --force
	err = installPreCommitHook(tempDir, true)
	assert.NoError(t, err)
}

func TestUninstallPreCommitHook_NotGitRepo(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	err := uninstallPreCommitHook(tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not a git repository")
}

func TestUninstallPreCommitHook_NoHook(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a fake git directory
	gitDir := filepath.Join(tempDir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))

	// Should not error if no hook exists
	err := uninstallPreCommitHook(tempDir)
	assert.NoError(t, err)
}

func TestUninstallPreCommitHook_NotDsopsHook(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a fake git directory with non-dsops hook
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	hookPath := filepath.Join(hooksDir, "pre-commit")
	require.NoError(t, os.WriteFile(hookPath, []byte("#!/bin/bash\necho 'other hook'\n"), 0755))

	// Should error because it's not a dsops hook
	err := uninstallPreCommitHook(tempDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not installed by dsops")
}

func TestUninstallPreCommitHook_DsopsHook(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a fake git directory
	gitDir := filepath.Join(tempDir, ".git")
	hooksDir := filepath.Join(gitDir, "hooks")
	require.NoError(t, os.MkdirAll(hooksDir, 0755))

	// Install the hook
	hookPath := filepath.Join(hooksDir, "pre-commit")
	hookContent := generatePreCommitHook()
	require.NoError(t, os.WriteFile(hookPath, []byte(hookContent), 0755))

	// Verify it contains our marker
	assert.True(t, strings.Contains(hookContent, "# dsops pre-commit hook"))

	// Should succeed
	err := uninstallPreCommitHook(tempDir)
	assert.NoError(t, err)

	// Verify hook was removed
	_, err = os.Stat(hookPath)
	assert.True(t, os.IsNotExist(err))
}
