package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/logging"
)

// TestConfig_LoadsBitwardenMultiAccountExample verifies that the runnable
// example committed under examples/bitwarden-multi-account.yaml actually
// parses through the same loader the CLI uses. Lightweight compensation for
// the deferred T034 quickstart walkthrough.
func TestConfig_LoadsBitwardenMultiAccountExample(t *testing.T) {
	t.Parallel()

	root, err := findRepoRoot()
	require.NoError(t, err)

	configPath := filepath.Join(root, "examples", "bitwarden-multi-account.yaml")

	cfg := &Config{
		Path:   configPath,
		Logger: logging.New(false, false),
	}
	err = cfg.Load()
	require.NoError(t, err, "examples/bitwarden-multi-account.yaml must parse cleanly")

	require.NotNil(t, cfg.Definition)
	assert.Equal(t, 0, cfg.Definition.Version)
	assert.Contains(t, cfg.Definition.Providers, "bw-personal")
	assert.Contains(t, cfg.Definition.Providers, "bw-work")
	assert.Equal(t, "bitwarden", cfg.Definition.Providers["bw-personal"].Type)
	assert.Equal(t, "bitwarden", cfg.Definition.Providers["bw-work"].Type)
}

// findRepoRoot returns the dsops repo root by climbing from the current
// working directory until it finds a go.mod file.
func findRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd, nil
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return "", os.ErrNotExist
		}
		wd = parent
	}
}
