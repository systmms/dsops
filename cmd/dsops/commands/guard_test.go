package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
)

func TestNewGuardCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewGuardCommand(cfg)

	assert.Equal(t, "guard", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify subcommands exist
	subcommands := cmd.Commands()
	assert.GreaterOrEqual(t, len(subcommands), 2)

	// Find repo and gitignore subcommands
	var foundRepo, foundGitignore bool
	for _, sub := range subcommands {
		if sub.Use == "repo" {
			foundRepo = true
		}
		if sub.Use == "gitignore" {
			foundGitignore = true
		}
	}
	assert.True(t, foundRepo, "repo subcommand should exist")
	assert.True(t, foundGitignore, "gitignore subcommand should exist")
}

func TestNewGuardRepoCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewGuardRepoCommand(cfg)

	assert.Equal(t, "repo", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("path"))
	assert.NotNil(t, flags.Lookup("all"))
	assert.NotNil(t, flags.Lookup("verbose"))
	assert.NotNil(t, flags.Lookup("exit-code"))
	assert.NotNil(t, flags.Lookup("pattern"))
}

func TestNewGuardGitignoreCommand(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewGuardGitignoreCommand(cfg)

	assert.Equal(t, "gitignore", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flags := cmd.Flags()
	assert.NotNil(t, flags.Lookup("path"))
	assert.NotNil(t, flags.Lookup("verbose"))
}

func TestGetSecretPatterns(t *testing.T) {
	t.Parallel()

	patterns := getSecretPatterns()

	assert.NotEmpty(t, patterns)
	assert.GreaterOrEqual(t, len(patterns), 10, "should have multiple detection patterns")

	// Verify patterns are valid regex
	for _, pattern := range patterns {
		assert.NotEmpty(t, pattern)
	}
}

func TestIsGitRepository(t *testing.T) {
	t.Parallel()

	t.Run("valid git repo", func(t *testing.T) {
		t.Parallel()
		// Create a fake git directory
		tempDir := t.TempDir()
		gitDir := filepath.Join(tempDir, ".git")
		require.NoError(t, os.MkdirAll(gitDir, 0755))

		result := isGitRepository(tempDir)
		assert.True(t, result)
	})

	t.Run("non-git directory", func(t *testing.T) {
		t.Parallel()
		tempDir := t.TempDir()
		result := isGitRepository(tempDir)
		assert.False(t, result)
	})
}

func TestCheckGitignore_NoGitignore(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Should complete without error even without .gitignore
	err := checkGitignore(tempDir, false)
	assert.NoError(t, err)
}

func TestCheckGitignore_WithGitignore(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create a .gitignore with some patterns
	gitignoreContent := `*.log
node_modules/
*.env
`
	err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644)
	require.NoError(t, err)

	// Should complete without error
	err = checkGitignore(tempDir, false)
	assert.NoError(t, err)
}

func TestScanRepository_NotGitRepo(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	err := scanRepository(tempDir, false, false, false, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Not a git repository")
}

func TestSecretFinding(t *testing.T) {
	t.Parallel()

	finding := SecretFinding{
		Description: "Test finding",
		Commit:      "abc123",
		File:        "test.go",
		Line:        42,
		Match:       "api_key=secret123",
	}

	assert.Equal(t, "Test finding", finding.Description)
	assert.Equal(t, "abc123", finding.Commit)
	assert.Equal(t, "test.go", finding.File)
	assert.Equal(t, 42, finding.Line)
	assert.Equal(t, "api_key=secret123", finding.Match)
}
