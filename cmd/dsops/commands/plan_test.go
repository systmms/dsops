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

func TestPlanCommand_BasicUsage(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Create a test config with literal provider
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
		Logger: logging.New(false, true), // quiet mode
	}
	require.NoError(t, cfg.Load())

	t.Run("table output", func(t *testing.T) {
		cmd := NewPlanCommand(cfg)
		output := captureOutputPlan(t, cmd, []string{"--env", "test"})

		assert.Contains(t, output, "DATABASE_URL")
		assert.Contains(t, output, "API_KEY")
		assert.Contains(t, output, "✓ OK")
		assert.Contains(t, output, "Total variables: 2")
	})

	t.Run("json output", func(t *testing.T) {
		// Reload config for fresh state
		cfg := &config.Config{
			Path:   configPath,
			Logger: logging.New(false, true),
		}
		require.NoError(t, cfg.Load())

		cmd := NewPlanCommand(cfg)
		output := captureOutputPlan(t, cmd, []string{"--env", "test", "--json"})

		var result map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(output), &result))

		assert.Contains(t, result, "variables")
		assert.Contains(t, result, "summary")

		summary := result["summary"].(map[string]interface{})
		assert.Equal(t, float64(2), summary["total_variables"])
		assert.Equal(t, float64(0), summary["error_count"])
	})
}

func TestPlanCommand_MissingEnvFlag(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs:    map[string]config.Environment{},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	cmd := NewPlanCommand(cfg)
	cmd.SetArgs([]string{}) // No --env flag

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestPlanCommand_InvalidEnvironment(t *testing.T) {
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

	cmd := NewPlanCommand(cfg)
	cmd.SetArgs([]string{"--env", "nonexistent"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestPlanCommand_VariableWithTransform(t *testing.T) {
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
					Literal:   `{"key": "value"}`,
					Transform: "json_extract:key",
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

	cmd := NewPlanCommand(cfg)
	output := captureOutputPlan(t, cmd, []string{"--env", "test"})

	assert.Contains(t, output, "EXTRACTED")
	assert.Contains(t, output, "json_extract")
}

func TestPlanCommand_OptionalVariable(t *testing.T) {
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
				"OPTIONAL_VAR": {
					Literal:  "optional-value",
					Optional: true,
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

	cmd := NewPlanCommand(cfg)
	output := captureOutputPlan(t, cmd, []string{"--env", "test"})

	assert.Contains(t, output, "OPTIONAL_VAR")
	assert.Contains(t, output, "yes") // Optional column
}

func TestPlanCommand_EmptyEnvironment(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs: map[string]config.Environment{
			"empty": {},
		},
	}

	configBytes, _ := yaml.Marshal(configData)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	cmd := NewPlanCommand(cfg)
	output := captureOutputPlan(t, cmd, []string{"--env", "empty"})

	assert.Contains(t, output, "Total variables: 0")
}

// captureOutputPlan captures command output for testing plan command
func captureOutputPlan(t *testing.T, cmd *cobra.Command, args []string) string {
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
