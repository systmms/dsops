package commands

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"gopkg.in/yaml.v3"
)

func TestGetCommand_BasicUsage(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Providers: map[string]config.ProviderConfig{
			"literal": {
				Type: "literal",
			},
		},
		Envs: map[string]config.Environment{
			"test": {
				"DATABASE_URL": {
					Literal: "postgres://localhost/testdb",
				},
				"API_KEY": {
					Literal: "test-api-key-123",
				},
			},
		},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	t.Run("get single variable", func(t *testing.T) {
		cfg := &config.Config{
			Path:   configPath,
			Logger: logging.New(false, true),
		}
		require.NoError(t, cfg.Load())

		cmd := NewGetCommand(cfg)
		output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "DATABASE_URL"})

		// Raw output should just be the value (no newline in fmt.Print)
		assert.Equal(t, "postgres://localhost/testdb", output)
	})

	t.Run("get different variable", func(t *testing.T) {
		cfg := &config.Config{
			Path:   configPath,
			Logger: logging.New(false, true),
		}
		require.NoError(t, cfg.Load())

		cmd := NewGetCommand(cfg)
		output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "API_KEY"})

		assert.Equal(t, "test-api-key-123", output)
	})
}

func TestGetCommand_JSONOutput(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Providers: map[string]config.ProviderConfig{
			"literal": {
				Type: "literal",
			},
		},
		Envs: map[string]config.Environment{
			"test": {
				"DATABASE_URL": {
					Literal: "postgres://localhost/testdb",
				},
			},
		},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	cmd := NewGetCommand(cfg)
	output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "DATABASE_URL", "--json"})

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &result))

	assert.Equal(t, "DATABASE_URL", result["variable"])
	assert.Equal(t, "postgres://localhost/testdb", result["value"])
	assert.Equal(t, "test", result["environment"])
	assert.Equal(t, "literal", result["source"]) // Since it's a literal
}

func TestGetCommand_MissingFlags(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs: map[string]config.Environment{
			"test": {},
		},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	t.Run("missing env flag", func(t *testing.T) {
		cmd := NewGetCommand(cfg)
		cmd.SetArgs([]string{"--var", "DATABASE_URL"})

		err := cmd.Execute()
		assert.Error(t, err)
	})

	t.Run("missing var flag", func(t *testing.T) {
		cmd := NewGetCommand(cfg)
		cmd.SetArgs([]string{"--env", "test"})

		err := cmd.Execute()
		assert.Error(t, err)
	})
}

func TestGetCommand_NonexistentVariable(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs: map[string]config.Environment{
			"test": {
				"DATABASE_URL": {
					Literal: "postgres://localhost/testdb",
				},
			},
		},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	cmd := NewGetCommand(cfg)
	cmd.SetArgs([]string{"--env", "test", "--var", "NONEXISTENT"})

	err := cmd.Execute()
	assert.Error(t, err)
	// Error should mention the variable not being found
}

func TestGetCommand_NonexistentEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs: map[string]config.Environment{
			"production": {},
		},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	cmd := NewGetCommand(cfg)
	cmd.SetArgs([]string{"--env", "nonexistent", "--var", "DATABASE_URL"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestGetCommand_VariableWithTransform(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Providers: map[string]config.ProviderConfig{
			"literal": {
				Type: "literal",
			},
		},
		Envs: map[string]config.Environment{
			"test": {
				"EXTRACTED": {
					Literal:   `{"database": {"url": "postgres://localhost/db"}}`,
					Transform: "json_extract:.database.url", // Path must start with '.'
				},
			},
		},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	cmd := NewGetCommand(cfg)
	output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "EXTRACTED"})

	assert.Equal(t, "postgres://localhost/db", output)
}

// captureGetOutput captures command output for testing get command
func captureGetOutput(t *testing.T, cmd *cobra.Command, args []string) string {
	t.Helper()

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Set args and execute
	if args != nil {
		cmd.SetArgs(args)
	}

	err := cmd.Execute()
	if err != nil {
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		io.Copy(&buf, r)
		t.Logf("Command output before error: %s", buf.String())
		require.NoError(t, err)
	}

	// Restore stdout and read output
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)

	return buf.String()
}
