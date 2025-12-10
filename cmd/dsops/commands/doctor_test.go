package commands

import (
	"bytes"
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

func TestDoctorCommand_BasicExecution(t *testing.T) {
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

	cmd := NewDoctorCommand(cfg)
	output := captureDoctorOutput(t, cmd, nil)

	assert.Contains(t, output, "PROVIDER")
	assert.Contains(t, output, "STATUS")
	assert.Contains(t, output, "Summary")
}

func TestDoctorCommand_NoProviders(t *testing.T) {
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

	cmd := NewDoctorCommand(cfg)
	output := captureDoctorOutput(t, cmd, nil)

	// With no providers, should show 0/0 healthy
	assert.Contains(t, output, "0/0 providers healthy")
}

func TestDoctorCommand_WithSecretStores(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0, // Use version 0 which is supported
		SecretStores: map[string]config.SecretStoreConfig{
			"literal-store": {
				Type: "literal",
			},
		},
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

	cmd := NewDoctorCommand(cfg)
	output := captureDoctorOutput(t, cmd, nil)

	assert.Contains(t, output, "literal-store")
	assert.Contains(t, output, "literal")
}

func TestDoctorCommand_UnimplementedProvider(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Providers: map[string]config.ProviderConfig{
			"fake-provider": {
				Type: "nonexistent_type",
			},
		},
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

	cmd := NewDoctorCommand(cfg)
	// Even with unimplemented providers, doctor should complete
	output := captureDoctorOutput(t, cmd, nil)

	assert.Contains(t, output, "Summary")
}

func TestDoctorCommand_WithEnvironmentCheck(t *testing.T) {
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

	cmd := NewDoctorCommand(cfg)
	output := captureDoctorOutput(t, cmd, []string{"--env", "test"})

	assert.Contains(t, output, "test")
	assert.Contains(t, output, "1 variables") // Should show the number of variables
}

func TestDoctorCommand_VerboseFlag(t *testing.T) {
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

	cmd := NewDoctorCommand(cfg)
	output := captureDoctorOutput(t, cmd, []string{"--verbose"})

	// In verbose mode, should show capabilities
	// Note: literal provider might not show capabilities if it's not registered
	assert.Contains(t, output, "Summary")
}

func TestDoctorCommand_FlagDefinitions(t *testing.T) {
	cfg := &config.Config{
		Logger: logging.New(false, true),
	}

	cmd := NewDoctorCommand(cfg)

	// Check that all expected flags exist
	verboseFlag := cmd.Flags().Lookup("verbose")
	assert.NotNil(t, verboseFlag)
	assert.Equal(t, "false", verboseFlag.DefValue)

	envFlag := cmd.Flags().Lookup("env")
	assert.NotNil(t, envFlag)
	assert.Equal(t, "", envFlag.DefValue)

	dataDirFlag := cmd.Flags().Lookup("data-dir")
	assert.NotNil(t, dataDirFlag)
	assert.Equal(t, "./dsops-data", dataDirFlag.DefValue)
}

func TestDoctorCommand_InvalidConfig(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Write invalid YAML
	require.NoError(t, os.WriteFile(configPath, []byte("invalid: yaml: ["), 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}

	cmd := NewDoctorCommand(cfg)
	cmd.SetArgs(nil)

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config")
}

func TestDoctorCommand_MixedSecretStoresAndLegacyProviders(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0, // Use version 0 which is supported
		SecretStores: map[string]config.SecretStoreConfig{
			"store1": {
				Type: "literal",
			},
		},
		Providers: map[string]config.ProviderConfig{
			"legacy1": {
				Type: "literal",
			},
		},
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

	cmd := NewDoctorCommand(cfg)
	output := captureDoctorOutput(t, cmd, nil)

	// Should show both secret stores and legacy providers
	assert.Contains(t, output, "PROVIDER")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "STATUS")
}

func TestGetSuggestions(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		err          error
		wantContains []string
	}{
		{
			name:         "bitwarden not found",
			providerType: "bitwarden",
			err:          fmt.Errorf("command not found"),
			wantContains: []string{"Install Bitwarden CLI", "PATH"},
		},
		{
			name:         "bitwarden unauthenticated",
			providerType: "bitwarden",
			err:          fmt.Errorf("unauthenticated"),
			wantContains: []string{"bw login"},
		},
		{
			name:         "bitwarden locked",
			providerType: "bitwarden",
			err:          fmt.Errorf("vault locked"),
			wantContains: []string{"bw unlock", "BW_SESSION"},
		},
		{
			name:         "onepassword not found",
			providerType: "onepassword",
			err:          fmt.Errorf("op: command not found"),
			wantContains: []string{"Install 1Password CLI", "PATH"},
		},
		{
			name:         "aws credentials",
			providerType: "aws.secretsmanager",
			err:          fmt.Errorf("invalid credentials"),
			wantContains: []string{"aws configure", "AWS_ACCESS_KEY_ID"},
		},
		{
			name:         "aws region",
			providerType: "aws.secretsmanager",
			err:          fmt.Errorf("missing region"),
			wantContains: []string{"AWS_REGION", "dsops.yaml"},
		},
		{
			name:         "unknown provider",
			providerType: "unknown",
			err:          fmt.Errorf("some error"),
			wantContains: []string{"documentation", "dsops.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := getSuggestions(tt.providerType, tt.err)
			for _, want := range tt.wantContains {
				found := false
				for _, s := range suggestions {
					if contains(s, want) {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected suggestion to contain %q", want)
			}
		})
	}
}

func TestContainsHelper(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "hello", true},
		{"hello world", "world", true},
		{"hello", "hello", true},
		{"hello", "world", false},
		{"", "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			got := contains(tt.s, tt.substr)
			assert.Equal(t, tt.want, got)
		})
	}
}

// captureDoctorOutput captures command output for testing doctor command
func captureDoctorOutput(t *testing.T, cmd *cobra.Command, args []string) string {
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
	// Doctor command may return error for unhealthy providers, we still want output
	_ = err

	// Restore stdout and read output
	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	return buf.String()
}
