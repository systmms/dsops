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

func TestInitCommand_CreatesConfig(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}

	cmd := NewInitCommand(cfg)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err, "config file should exist")

	// Verify content contains expected elements
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "version:")
	assert.Contains(t, string(content), "providers:")
	assert.Contains(t, string(content), "envs:")
}

func TestInitCommand_ExistingConfigError(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Create existing config file
	require.NoError(t, os.WriteFile(configPath, []byte("existing config"), 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}

	cmd := NewInitCommand(cfg)

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestInitCommand_CustomPath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "custom", "dsops.yaml")

	// Create parent directory
	require.NoError(t, os.MkdirAll(filepath.Dir(configPath), 0755))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}

	cmd := NewInitCommand(cfg)

	err := cmd.Execute()
	require.NoError(t, err)

	// Verify file was created at custom path
	_, err = os.Stat(configPath)
	require.NoError(t, err, "config file should exist at custom path")
}
