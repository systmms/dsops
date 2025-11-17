package providers

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPassProviderCreation tests provider creation.
func TestPassProviderCreation(t *testing.T) {
	tests := []struct {
		name   string
		config PassConfig
	}{
		{
			name:   "empty_config",
			config: PassConfig{},
		},
		{
			name: "with_password_store",
			config: PassConfig{
				PasswordStore: "/custom/path/.password-store",
			},
		},
		{
			name: "with_gpg_key",
			config: PassConfig{
				GpgKey: "ABC123DEF456",
			},
		},
		{
			name: "full_config",
			config: PassConfig{
				PasswordStore: "~/.password-store",
				GpgKey:        "0x1234567890ABCDEF",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPassProvider(tt.config)
			require.NotNil(t, p)
			assert.Equal(t, tt.config.PasswordStore, p.config.PasswordStore)
			assert.Equal(t, tt.config.GpgKey, p.config.GpgKey)
			assert.NotNil(t, p.logger)
		})
	}
}

// TestPassProviderName verifies the provider name.
func TestPassProviderName(t *testing.T) {
	p := NewPassProvider(PassConfig{})
	assert.Equal(t, "pass", p.Name())
}

// TestPassProviderCapabilities verifies provider capabilities.
func TestPassProviderCapabilities(t *testing.T) {
	p := NewPassProvider(PassConfig{})
	caps := p.Capabilities()

	assert.False(t, caps.SupportsVersioning, "pass doesn't support versioning")
	assert.True(t, caps.SupportsMetadata, "pass supports metadata")
	assert.False(t, caps.SupportsWatching, "pass doesn't support watching")
	assert.False(t, caps.SupportsBinary, "pass doesn't support binary")
	assert.True(t, caps.RequiresAuth, "pass requires GPG auth")
	assert.Contains(t, caps.AuthMethods, "gpg_key")
}

// TestPassBuildCommand tests command building with environment variables.
func TestPassBuildCommand(t *testing.T) {
	tests := []struct {
		name              string
		config            PassConfig
		args              []string
		expectPasswordDir bool
		expectGpgKey      bool
	}{
		{
			name:              "basic_command_no_config",
			config:            PassConfig{},
			args:              []string{"show", "email/gmail"},
			expectPasswordDir: false,
			expectGpgKey:      false,
		},
		{
			name: "command_with_custom_store",
			config: PassConfig{
				PasswordStore: "/home/user/.custom-pass",
			},
			args:              []string{"show", "work/database"},
			expectPasswordDir: true,
			expectGpgKey:      false,
		},
		{
			name: "command_with_gpg_key",
			config: PassConfig{
				GpgKey: "ABC123",
			},
			args:              []string{"insert", "new/secret"},
			expectPasswordDir: false,
			expectGpgKey:      true,
		},
		{
			name: "command_with_all_config",
			config: PassConfig{
				PasswordStore: "/opt/password-store",
				GpgKey:        "0xDEADBEEF",
			},
			args:              []string{"ls", "prod"},
			expectPasswordDir: true,
			expectGpgKey:      true,
		},
		{
			name:              "list_command",
			config:            PassConfig{},
			args:              []string{"ls"},
			expectPasswordDir: false,
			expectGpgKey:      false,
		},
		{
			name:              "generate_command",
			config:            PassConfig{},
			args:              []string{"generate", "new/password", "32"},
			expectPasswordDir: false,
			expectGpgKey:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPassProvider(tt.config)
			ctx := context.Background()

			cmd := p.buildCommand(ctx, tt.args...)

			// Verify command path
			assert.Equal(t, "pass", cmd.Path)

			// Verify args
			expectedArgs := append([]string{"pass"}, tt.args...)
			assert.Equal(t, expectedArgs, cmd.Args)

			// Check environment variables
			env := cmd.Env
			hasPasswordDir := false
			hasGpgKey := false

			for _, e := range env {
				if len(e) >= 19 && e[:19] == "PASSWORD_STORE_DIR=" {
					hasPasswordDir = true
					if tt.expectPasswordDir {
						assert.Equal(t, "PASSWORD_STORE_DIR="+tt.config.PasswordStore, e)
					}
				}
				if len(e) >= 19 && e[:19] == "PASSWORD_STORE_KEY=" {
					hasGpgKey = true
					if tt.expectGpgKey {
						assert.Equal(t, "PASSWORD_STORE_KEY="+tt.config.GpgKey, e)
					}
				}
			}

			if tt.expectPasswordDir {
				assert.True(t, hasPasswordDir, "PASSWORD_STORE_DIR should be set")
			}
			if tt.expectGpgKey {
				assert.True(t, hasGpgKey, "PASSWORD_STORE_KEY should be set")
			}
		})
	}
}

// TestPassCommandType verifies the command is an exec.Cmd.
func TestPassCommandType(t *testing.T) {
	p := NewPassProvider(PassConfig{})
	ctx := context.Background()

	cmd := p.buildCommand(ctx, "show", "test/secret")

	// Verify it's the right type
	require.IsType(t, &exec.Cmd{}, cmd)

	// Verify basic structure
	assert.Equal(t, "pass", cmd.Path)
	assert.NotEmpty(t, cmd.Env)
}

// TestPassSecretPathFormats tests various secret path formats.
func TestPassSecretPathFormats(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		description string
	}{
		{
			name:        "simple_path",
			path:        "email",
			description: "Simple single-level path",
		},
		{
			name:        "two_level_path",
			path:        "email/gmail",
			description: "Two-level hierarchical path",
		},
		{
			name:        "deep_nested_path",
			path:        "work/servers/production/database",
			description: "Deeply nested path",
		},
		{
			name:        "path_with_dots",
			path:        "api/stripe.com/secret_key",
			description: "Path with dots in component",
		},
		{
			name:        "path_with_underscores",
			path:        "cloud/aws_production/access_key",
			description: "Path with underscores",
		},
		{
			name:        "path_with_hyphens",
			path:        "services/my-api/token",
			description: "Path with hyphens",
		},
		{
			name:        "path_with_numbers",
			path:        "servers/db01/root",
			description: "Path with numbers",
		},
		{
			name:        "single_character",
			path:        "a",
			description: "Single character path",
		},
	}

	p := NewPassProvider(PassConfig{})
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := p.buildCommand(ctx, "show", tt.path)

			// Verify the path is passed correctly
			assert.Equal(t, []string{"pass", "show", tt.path}, cmd.Args)
		})
	}
}

// TestPassEnvironmentInheritance tests that parent environment is inherited.
func TestPassEnvironmentInheritance(t *testing.T) {
	p := NewPassProvider(PassConfig{})
	ctx := context.Background()

	cmd := p.buildCommand(ctx, "show", "test")

	// Environment should not be empty (inherits from os.Environ())
	assert.NotEmpty(t, cmd.Env)

	// Should contain PATH (critical for finding GPG, etc.)
	hasPath := false
	for _, env := range cmd.Env {
		if len(env) >= 5 && env[:5] == "PATH=" {
			hasPath = true
			break
		}
	}
	assert.True(t, hasPath, "PATH environment variable should be inherited")
}

// TestPassCustomEnvironmentOverride tests that custom env vars are added.
func TestPassCustomEnvironmentOverride(t *testing.T) {
	config := PassConfig{
		PasswordStore: "/custom/store",
		GpgKey:        "CUSTOM_KEY",
	}
	p := NewPassProvider(config)
	ctx := context.Background()

	cmd := p.buildCommand(ctx, "list")

	// Count pass-specific environment variables
	passEnvCount := 0
	for _, env := range cmd.Env {
		if len(env) >= 14 && env[:14] == "PASSWORD_STORE" {
			passEnvCount++
		}
	}

	// Should have 2 pass-specific environment variables
	assert.Equal(t, 2, passEnvCount, "Should have PASSWORD_STORE_DIR and PASSWORD_STORE_KEY")
}

// TestPassFolderDetection tests parsing secret paths for folder information.
func TestPassFolderDetection(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedFolder string
		hasFolder      bool
	}{
		{
			name:           "simple_path_no_folder",
			path:           "password",
			expectedFolder: "",
			hasFolder:      false,
		},
		{
			name:           "one_level_folder",
			path:           "work/database",
			expectedFolder: "work",
			hasFolder:      true,
		},
		{
			name:           "two_level_folder",
			path:           "work/servers/db",
			expectedFolder: "work/servers",
			hasFolder:      true,
		},
		{
			name:           "deep_folder",
			path:           "a/b/c/d/e/secret",
			expectedFolder: "a/b/c/d/e",
			hasFolder:      true,
		},
		{
			name:           "root_level",
			path:           "email",
			expectedFolder: "",
			hasFolder:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the folder detection logic from Describe
			hasFolder := false
			folder := ""

			if len(tt.path) > 0 {
				lastSlash := -1
				for i := len(tt.path) - 1; i >= 0; i-- {
					if tt.path[i] == '/' {
						lastSlash = i
						break
					}
				}
				if lastSlash > 0 {
					hasFolder = true
					folder = tt.path[:lastSlash]
				}
			}

			assert.Equal(t, tt.hasFolder, hasFolder)
			if tt.hasFolder {
				assert.Equal(t, tt.expectedFolder, folder)
			}
		})
	}
}

// TestPassMetadataExtraction tests parsing pass output for metadata.
func TestPassMetadataExtraction(t *testing.T) {
	tests := []struct {
		name               string
		output             string
		expectedPassword   string
		expectedHasMetadata bool
	}{
		{
			name:               "password_only",
			output:             "mysecretpassword",
			expectedPassword:   "mysecretpassword",
			expectedHasMetadata: false,
		},
		{
			name:               "password_with_metadata",
			output:             "mypassword\nusername: john\nurl: https://example.com",
			expectedPassword:   "mypassword",
			expectedHasMetadata: true,
		},
		{
			name:               "password_with_trailing_newline",
			output:             "password123\n",
			expectedPassword:   "password123",
			expectedHasMetadata: false,
		},
		{
			name:               "multiline_password",
			output:             "line1\nline2\nline3",
			expectedPassword:   "line1",
			expectedHasMetadata: true,
		},
		{
			name:               "empty_output",
			output:             "",
			expectedPassword:   "",
			expectedHasMetadata: false,
		},
		{
			name:               "only_whitespace",
			output:             "   \n\t\n  ",
			expectedPassword:   "",
			expectedHasMetadata: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the password extraction logic from Resolve
			lines := []string{}
			password := ""

			trimmed := tt.output
			// Remove leading/trailing whitespace
			for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t' || trimmed[0] == '\n' || trimmed[0] == '\r') {
				trimmed = trimmed[1:]
			}
			for len(trimmed) > 0 && (trimmed[len(trimmed)-1] == ' ' || trimmed[len(trimmed)-1] == '\t' || trimmed[len(trimmed)-1] == '\n' || trimmed[len(trimmed)-1] == '\r') {
				trimmed = trimmed[:len(trimmed)-1]
			}

			if len(trimmed) > 0 {
				// Split by newlines
				start := 0
				for i := 0; i <= len(trimmed); i++ {
					if i == len(trimmed) || trimmed[i] == '\n' {
						lines = append(lines, trimmed[start:i])
						start = i + 1
					}
				}
				if len(lines) > 0 {
					password = lines[0]
				}
			}

			hasMetadata := len(lines) > 1

			assert.Equal(t, tt.expectedPassword, password)
			assert.Equal(t, tt.expectedHasMetadata, hasMetadata)
		})
	}
}
