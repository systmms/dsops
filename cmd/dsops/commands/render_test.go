package commands

import (
	"bytes"
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

func TestRenderCommand_DotenvFormat(t *testing.T) {
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

	outputPath := filepath.Join(tempDir, ".env")
	cmd := NewRenderCommand(cfg)
	_ = captureRenderOutput(t, cmd, []string{"--env", "test", "--out", outputPath})

	// Check file was created
	assert.FileExists(t, outputPath)

	// Check file contents
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "DATABASE_URL=postgres://localhost/testdb")
	assert.Contains(t, string(content), "API_KEY=test-api-key-123")

	// Check file permissions (0600 default)
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestRenderCommand_JSONFormat(t *testing.T) {
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

	outputPath := filepath.Join(tempDir, "config.json")
	cmd := NewRenderCommand(cfg)
	_ = captureRenderOutput(t, cmd, []string{"--env", "test", "--out", outputPath, "--format", "json"})

	// Check file was created
	assert.FileExists(t, outputPath)

	// Check file contents
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "DATABASE_URL")
	assert.Contains(t, string(content), "postgres://localhost/testdb")
}

func TestRenderCommand_YAMLFormat(t *testing.T) {
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

	outputPath := filepath.Join(tempDir, "config.yaml")
	cmd := NewRenderCommand(cfg)
	_ = captureRenderOutput(t, cmd, []string{"--env", "test", "--out", outputPath, "--format", "yaml"})

	// Check file was created
	assert.FileExists(t, outputPath)

	// Check file contents
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "DATABASE_URL")
	assert.Contains(t, string(content), "postgres://localhost/testdb")
}

func TestRenderCommand_MissingOutFlag(t *testing.T) {
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

	cmd := NewRenderCommand(cfg)
	cmd.SetArgs([]string{"--env", "test"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestRenderCommand_MissingEnvFlag(t *testing.T) {
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

	outputPath := filepath.Join(tempDir, ".env")
	cmd := NewRenderCommand(cfg)
	cmd.SetArgs([]string{"--out", outputPath})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "required flag")
}

func TestRenderCommand_CustomPermissions(t *testing.T) {
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
				"KEY": {
					Literal: "value",
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

	outputPath := filepath.Join(tempDir, ".env")
	cmd := NewRenderCommand(cfg)
	_ = captureRenderOutput(t, cmd, []string{"--env", "test", "--out", outputPath, "--permissions", "0644"})

	// Check file permissions
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestRenderCommand_InvalidPermissions(t *testing.T) {
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

	outputPath := filepath.Join(tempDir, ".env")
	cmd := NewRenderCommand(cfg)
	cmd.SetArgs([]string{"--env", "test", "--out", outputPath, "--permissions", "invalid"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid permissions")
}

func TestRenderCommand_EmptyEnvironment(t *testing.T) {
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

	outputPath := filepath.Join(tempDir, ".env")
	cmd := NewRenderCommand(cfg)
	_ = captureRenderOutput(t, cmd, []string{"--env", "empty", "--out", outputPath})

	// File should still be created, but small (just header comment)
	assert.FileExists(t, outputPath)
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	// Empty dotenv file might have just a comment header
	assert.LessOrEqual(t, len(content), 200) // Small content expected (just header)
}

func TestRenderCommand_TemplateFormat(t *testing.T) {
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

	// Create a template file
	templatePath := filepath.Join(tempDir, "template.txt")
	templateContent := `# Config File
database_url = "{{ .DATABASE_URL }}"
`
	require.NoError(t, os.WriteFile(templatePath, []byte(templateContent), 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}
	require.NoError(t, cfg.Load())

	outputPath := filepath.Join(tempDir, "output.txt")
	cmd := NewRenderCommand(cfg)
	_ = captureRenderOutput(t, cmd, []string{"--env", "test", "--out", outputPath, "--template", templatePath})

	// Check file was created
	assert.FileExists(t, outputPath)

	// Check file contents
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "postgres://localhost/testdb")
}

// captureRenderOutput captures command output for testing render command
func captureRenderOutput(t *testing.T, cmd *cobra.Command, args []string) string {
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
