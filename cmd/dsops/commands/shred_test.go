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

func TestShredCommand_NoFilesError(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewShredCommand(cfg)
	cmd.SetArgs([]string{}) // No files

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "No files specified")
}

func TestShredCommand_FileNotFound(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewShredCommand(cfg)
	cmd.SetArgs([]string{"--force", "/nonexistent/file.txt"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot access path")
}

func TestShredCommand_ShredsFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "secret.txt")

	// Create a test file with some content
	content := []byte("secret data that should be shredded")
	require.NoError(t, os.WriteFile(testFile, content, 0644))

	// Verify file exists
	_, err := os.Stat(testFile)
	require.NoError(t, err, "test file should exist before shred")

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewShredCommand(cfg)
	cmd.SetArgs([]string{"--force", testFile})

	err = cmd.Execute()
	require.NoError(t, err)

	// Verify file was deleted
	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err), "file should be deleted after shred")
}

func TestShredCommand_InvalidPasses(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "secret.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("data"), 0644))

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	tests := []struct {
		name   string
		passes string
	}{
		{"zero passes", "0"},
		{"negative passes", "-1"},
		{"too many passes", "11"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Create a fresh file for each test
			tempDir := t.TempDir()
			testFile := filepath.Join(tempDir, "secret.txt")
			require.NoError(t, os.WriteFile(testFile, []byte("data"), 0644))

			cmd := NewShredCommand(cfg)
			cmd.SetArgs([]string{"--force", "--passes", tt.passes, testFile})

			err := cmd.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), "Invalid number of passes")
		})
	}
}

func TestShredCommand_DirectoryRequiresRecursive(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "secrets")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewShredCommand(cfg)
	cmd.SetArgs([]string{"--force", subDir})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "directory")
	assert.Contains(t, err.Error(), "--recursive")
}

func TestShredCommand_RecursiveDirectory(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "secrets")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Create files in directory
	file1 := filepath.Join(subDir, "secret1.txt")
	file2 := filepath.Join(subDir, "secret2.txt")
	require.NoError(t, os.WriteFile(file1, []byte("secret 1"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("secret 2"), 0644))

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewShredCommand(cfg)
	cmd.SetArgs([]string{"--force", "--recursive", subDir})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify files were deleted
	_, err = os.Stat(file1)
	assert.True(t, os.IsNotExist(err), "file1 should be deleted")
	_, err = os.Stat(file2)
	assert.True(t, os.IsNotExist(err), "file2 should be deleted")
}

func TestShredCommand_EmptyFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "empty.txt")

	// Create an empty file
	require.NoError(t, os.WriteFile(testFile, []byte{}, 0644))

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewShredCommand(cfg)
	cmd.SetArgs([]string{"--force", testFile})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify file was deleted
	_, err = os.Stat(testFile)
	assert.True(t, os.IsNotExist(err), "empty file should be deleted")
}

func TestShredCommand_MultipleFiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	file1 := filepath.Join(tempDir, "secret1.txt")
	file2 := filepath.Join(tempDir, "secret2.txt")

	require.NoError(t, os.WriteFile(file1, []byte("secret 1"), 0644))
	require.NoError(t, os.WriteFile(file2, []byte("secret 2"), 0644))

	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewShredCommand(cfg)
	cmd.SetArgs([]string{"--force", file1, file2})

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify both files were deleted
	_, err = os.Stat(file1)
	assert.True(t, os.IsNotExist(err), "file1 should be deleted")
	_, err = os.Stat(file2)
	assert.True(t, os.IsNotExist(err), "file2 should be deleted")
}
