package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"gopkg.in/yaml.v3"
)

func TestExecCommand_MissingCommand(t *testing.T) {
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

	cmd := NewExecCommand(cfg)
	cmd.SetArgs([]string{"--env", "test"}) // No command after --

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "No command specified")
}

func TestExecCommand_MissingEnvFlag(t *testing.T) {
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

	cmd := NewExecCommand(cfg)
	cmd.SetArgs([]string{"echo", "hello"}) // Missing --env flag

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestExecCommand_NonexistentEnvironment(t *testing.T) {
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

	cmd := NewExecCommand(cfg)
	cmd.SetArgs([]string{"--env", "nonexistent", "echo", "hello"})

	err := cmd.Execute()
	assert.Error(t, err)
}

func TestExecCommand_FlagParsing(t *testing.T) {
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

	cmd := NewExecCommand(cfg)

	// Test that flags are properly defined
	envFlag := cmd.Flags().Lookup("env")
	assert.NotNil(t, envFlag)
	assert.Equal(t, "", envFlag.DefValue)

	printFlag := cmd.Flags().Lookup("print")
	assert.NotNil(t, printFlag)
	assert.Equal(t, "false", printFlag.DefValue)

	allowOverrideFlag := cmd.Flags().Lookup("allow-override")
	assert.NotNil(t, allowOverrideFlag)
	assert.Equal(t, "false", allowOverrideFlag.DefValue)

	workingDirFlag := cmd.Flags().Lookup("working-dir")
	assert.NotNil(t, workingDirFlag)
	assert.Equal(t, "", workingDirFlag.DefValue)

	timeoutFlag := cmd.Flags().Lookup("timeout")
	assert.NotNil(t, timeoutFlag)
	assert.Equal(t, "0", timeoutFlag.DefValue)
}

func TestExecCommand_SimpleExecution(t *testing.T) {
	// Skip on CI or if command doesn't exist
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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
				"MY_VAR": {
					Literal: "test-value",
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

	// Create a simple script that will be executed
	scriptPath := filepath.Join(tempDir, "test.sh")
	scriptContent := `#!/bin/sh
echo "MY_VAR=$MY_VAR"
exit 0
`
	require.NoError(t, os.WriteFile(scriptPath, []byte(scriptContent), 0755))

	cmd := NewExecCommand(cfg)
	cmd.SetArgs([]string{"--env", "test", "sh", scriptPath})

	err := cmd.Execute()
	// The command should execute successfully
	assert.NoError(t, err)
}

func TestExecCommand_ResolutionError(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs: map[string]config.Environment{
			"test": {
				"BAD_VAR": {
					From: &config.Reference{
						Provider: "nonexistent_provider",
						Key:      "some/key",
					},
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

	cmd := NewExecCommand(cfg)
	cmd.SetArgs([]string{"--env", "test", "echo", "hello"})

	err := cmd.Execute()
	// Should fail due to unregistered provider
	assert.Error(t, err)
}

func TestExecCommand_EmptyEnvironment(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

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

	cmd := NewExecCommand(cfg)
	cmd.SetArgs([]string{"--env", "empty", "true"}) // Just run 'true' which always succeeds

	err := cmd.Execute()
	assert.NoError(t, err)
}
