package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestGetCommand_NonexistentVariableSuggestions(t *testing.T) {
	t.Run("few_variables_shows_list", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "dsops.yaml")

		configData := &config.Definition{
			Version: 0,
			Envs: map[string]config.Environment{
				"test": {
					"VAR_ONE": {Literal: "value1"},
					"VAR_TWO": {Literal: "value2"},
					"VAR_THREE": {Literal: "value3"},
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
		require.Error(t, err)
		// Error should mention available variables when count <= 10
		assert.Contains(t, err.Error(), "variable not found")
	})

	t.Run("many_variables_shows_count", func(t *testing.T) {
		tempDir := t.TempDir()
		configPath := filepath.Join(tempDir, "dsops.yaml")

		// Create environment with more than 10 variables
		env := config.Environment{}
		for i := 0; i < 15; i++ {
			env[fmt.Sprintf("VAR_%02d", i)] = config.Variable{Literal: fmt.Sprintf("value%d", i)}
		}

		configData := &config.Definition{
			Version: 0,
			Envs: map[string]config.Environment{
				"test": env,
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
		require.Error(t, err)
		assert.Contains(t, err.Error(), "variable not found")
	})
}

func TestGetCommand_JSONOutputMetadata(t *testing.T) {
	// Test that JSON output includes all expected metadata fields
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
	output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "DATABASE_URL", "--json"})

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(output), &result))

	// Verify all expected fields are present
	assert.Equal(t, "DATABASE_URL", result["variable"])
	assert.Equal(t, "postgres://localhost/testdb", result["value"])
	assert.Equal(t, "test", result["environment"])
	assert.Equal(t, "literal", result["source"]) // Source should be "literal" for literal values
}

func TestGetCommand_EmptyEnvironment(t *testing.T) {
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

	cmd := NewGetCommand(cfg)
	cmd.SetArgs([]string{"--env", "empty", "--var", "ANY_VAR"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "variable not found")
}

func TestGetCommand_MultipleEnvironments(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs: map[string]config.Environment{
			"dev": {
				"DATABASE_URL": {Literal: "postgres://localhost/dev"},
			},
			"prod": {
				"DATABASE_URL": {Literal: "postgres://prod-server/prod"},
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

	t.Run("get from dev", func(t *testing.T) {
		cmd := NewGetCommand(cfg)
		output := captureGetOutput(t, cmd, []string{"--env", "dev", "--var", "DATABASE_URL"})
		assert.Equal(t, "postgres://localhost/dev", output)
	})

	t.Run("get from prod", func(t *testing.T) {
		cmd := NewGetCommand(cfg)
		output := captureGetOutput(t, cmd, []string{"--env", "prod", "--var", "DATABASE_URL"})
		assert.Equal(t, "postgres://prod-server/prod", output)
	})
}

func TestGetCommand_SpecialCharacterValues(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs: map[string]config.Environment{
			"test": {
				"PASSWORD_WITH_SPECIAL": {Literal: "p@ss=word!#$%^&*()"},
				"MULTILINE":             {Literal: "line1\nline2\nline3"},
				"WITH_QUOTES":           {Literal: `value with "quotes" and 'apostrophes'`},
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

	t.Run("special characters in password", func(t *testing.T) {
		cmd := NewGetCommand(cfg)
		output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "PASSWORD_WITH_SPECIAL"})
		assert.Equal(t, "p@ss=word!#$%^&*()", output)
	})

	t.Run("multiline value", func(t *testing.T) {
		cmd := NewGetCommand(cfg)
		output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "MULTILINE"})
		assert.Equal(t, "line1\nline2\nline3", output)
	})

	t.Run("quotes in value", func(t *testing.T) {
		cmd := NewGetCommand(cfg)
		output := captureGetOutput(t, cmd, []string{"--env", "test", "--var", "WITH_QUOTES"})
		assert.Equal(t, `value with "quotes" and 'apostrophes'`, output)
	})
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
		_ = w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		t.Logf("Command output before error: %s", buf.String())
		require.NoError(t, err)
	}

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	return buf.String()
}
