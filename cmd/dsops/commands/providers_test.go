package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
	"gopkg.in/yaml.v3"
)

func TestProvidersCommand_ExecutesSuccessfully(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	// Create a minimal config
	configData := &config.Definition{
		Version: 0,
		Envs:    map[string]config.Environment{"test": {}},
	}
	configBytes, err := yaml.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}

	cmd := NewProvidersCommand(cfg)

	// Test that command executes without error
	err = cmd.Execute()
	require.NoError(t, err)
}

func TestProvidersCommand_VerboseFlag(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")

	configData := &config.Definition{
		Version: 0,
		Envs:    map[string]config.Environment{"test": {}},
	}
	configBytes, err := yaml.Marshal(configData)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(configPath, configBytes, 0644))

	cfg := &config.Config{
		Path:   configPath,
		Logger: logging.New(false, true),
	}

	cmd := NewProvidersCommand(cfg)
	cmd.SetArgs([]string{"--verbose"})

	// Test that verbose flag works without error
	err = cmd.Execute()
	require.NoError(t, err)
}

func TestGetProviderDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		providerType string
		wantContains string
	}{
		{"bitwarden", "Bitwarden"},
		{"aws.secretsmanager", "AWS Secrets Manager"},
		{"onepassword", "1Password"},
		{"vault", "HashiCorp Vault"},
		{"unknown-provider", "No description available"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.providerType, func(t *testing.T) {
			t.Parallel()
			desc := getProviderDescription(tt.providerType)
			assert.Contains(t, desc, tt.wantContains)
		})
	}
}

func TestGetProviderDetails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		providerType   string
		wantMinDetails int
	}{
		{"bitwarden", 3},
		{"aws.secretsmanager", 3},
		{"onepassword", 3},
		{"unknown-provider", 1},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.providerType, func(t *testing.T) {
			t.Parallel()
			details := getProviderDetails(tt.providerType)
			assert.GreaterOrEqual(t, len(details), tt.wantMinDetails)
		})
	}
}

func TestGetProviderDetails_UnknownProvider(t *testing.T) {
	t.Parallel()

	details := getProviderDetails("nonexistent-provider")
	require.Len(t, details, 1)
	assert.Equal(t, "No details available", details[0])
}

// --- SPEC-027: machine-level secret stores ---------------------------------
// These tests capture os.Stdout, so they must not run in parallel.

func TestProvidersCommand_ShowsSources(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("version: 0\nsecretStores:\n  proj-store:\n    type: literal\nenvs:\n  test: {}\n"), 0o644))
	userPath := filepath.Join(tempDir, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte("version: 0\nsecretStores:\n  machine-vault:\n    type: vault\n    address: https://vault.example.com\n"), 0o600))

	cfg := &config.Config{
		Path:       configPath,
		Logger:     logging.New(false, true),
		UserConfig: config.UserConfigSpec{Path: userPath, Origin: config.UserConfigOriginFlag},
	}

	output := captureDoctorOutput(t, NewProvidersCommand(cfg), nil)

	assert.Contains(t, output, "Configuration sources:")
	assert.Contains(t, output, "User config:    "+userPath)
	assert.Contains(t, output, "SOURCE")
	assert.Regexp(t, `machine-vault\s+vault\s+user\s+configured`, output)
	assert.Regexp(t, `proj-store\s+literal\s+project\s+configured`, output)
	// Sorted output: machine-vault before proj-store.
	assert.Less(t, strings.Index(output, "machine-vault"), strings.Index(output, "proj-store"))
}

func TestProvidersCommand_BrokenUserConfig_StillListsBuiltins(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "dsops.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte("version: 0\nenvs:\n  test: {}\n"), 0o644))
	userPath := filepath.Join(tempDir, "user.yaml")
	require.NoError(t, os.WriteFile(userPath, []byte("version: 0\ntemplates: []\n"), 0o600))

	cfg := &config.Config{
		Path:       configPath,
		Logger:     logging.New(false, true),
		UserConfig: config.UserConfigSpec{Path: userPath, Origin: config.UserConfigOriginFlag},
	}

	cmd := NewProvidersCommand(cfg)
	output := captureDoctorOutput(t, cmd, nil)
	assert.Contains(t, output, "Built-in Provider Types")
	assert.NotContains(t, output, "Configured Providers")
	require.NoError(t, cmd.Execute(), "listing built-ins must still succeed")
}
