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
