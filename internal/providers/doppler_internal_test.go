package providers

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDopplerMaskToken tests token masking for secure logging.
func TestDopplerMaskToken(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected string
	}{
		{
			name:     "normal_token",
			token:    "dp.st.abcdefghijklmnop",
			expected: "mnop",
		},
		{
			name:     "long_token",
			token:    "dp.st.1234567890abcdefghijklmnopqrstuvwxyz",
			expected: "wxyz",
		},
		{
			name:     "short_token",
			token:    "abc",
			expected: "***",
		},
		{
			name:     "empty_token",
			token:    "",
			expected: "***",
		},
		{
			name:     "exactly_six_chars",
			token:    "123456",
			expected: "3456",
		},
		{
			name:     "five_chars",
			token:    "12345",
			expected: "***",
		},
		{
			name:     "seven_chars",
			token:    "1234567",
			expected: "4567",
		},
		{
			name:     "with_special_chars",
			token:    "dp.st.key!@#$%^&*()",
			expected: "&*()",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &DopplerProvider{
				config: DopplerConfig{
					Token: tt.token,
				},
			}
			result := p.maskToken()
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestDopplerBuildCommand tests command building with environment injection.
func TestDopplerBuildCommand(t *testing.T) {
	tests := []struct {
		name            string
		config          DopplerConfig
		args            []string
		expectToken     bool
		expectProject   bool
		expectConfig    bool
		expectedCommand string
	}{
		{
			name: "command_with_all_config",
			config: DopplerConfig{
				Token:   "dp.st.token123",
				Project: "my-project",
				Config:  "production",
			},
			args:            []string{"secrets", "get", "--json"},
			expectToken:     true,
			expectProject:   true,
			expectConfig:    true,
			expectedCommand: "doppler",
		},
		{
			name: "command_with_token_only",
			config: DopplerConfig{
				Token: "dp.st.token123",
			},
			args:          []string{"secrets", "list"},
			expectToken:   true,
			expectProject: false,
			expectConfig:  false,
		},
		{
			name: "command_with_project_only",
			config: DopplerConfig{
				Project: "my-project",
			},
			args:          []string{"secrets", "get", "DB_PASSWORD"},
			expectToken:   false,
			expectProject: true,
			expectConfig:  false,
		},
		{
			name:          "command_with_no_config",
			config:        DopplerConfig{},
			args:          []string{"whoami"},
			expectToken:   false,
			expectProject: false,
			expectConfig:  false,
		},
		{
			name: "command_with_config_only",
			config: DopplerConfig{
				Config: "staging",
			},
			args:          []string{"run", "--"},
			expectToken:   false,
			expectProject: false,
			expectConfig:  true,
		},
		{
			name: "complex_args",
			config: DopplerConfig{
				Token:   "token",
				Project: "project",
				Config:  "config",
			},
			args:          []string{"secrets", "get", "KEY", "--plain", "--no-check-version"},
			expectToken:   true,
			expectProject: true,
			expectConfig:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewDopplerProvider(tt.config)
			ctx := context.Background()

			cmd := p.buildCommand(ctx, tt.args...)

			// Verify command path
			assert.Equal(t, "doppler", cmd.Path)

			// Verify args (first arg is command name)
			expectedArgs := append([]string{"doppler"}, tt.args...)
			assert.Equal(t, expectedArgs, cmd.Args)

			// Verify environment variables
			env := cmd.Env
			hasToken := false
			hasProject := false
			hasConfig := false

			for _, e := range env {
				if len(e) >= 13 && e[:13] == "DOPPLER_TOKEN" {
					hasToken = true
					if tt.expectToken {
						assert.Equal(t, "DOPPLER_TOKEN="+tt.config.Token, e)
					}
				}
				if len(e) >= 15 && e[:15] == "DOPPLER_PROJECT" {
					hasProject = true
					if tt.expectProject {
						assert.Equal(t, "DOPPLER_PROJECT="+tt.config.Project, e)
					}
				}
				if len(e) >= 14 && e[:14] == "DOPPLER_CONFIG" {
					hasConfig = true
					if tt.expectConfig {
						assert.Equal(t, "DOPPLER_CONFIG="+tt.config.Config, e)
					}
				}
			}

			if tt.expectToken {
				assert.True(t, hasToken, "DOPPLER_TOKEN should be set")
			}
			if tt.expectProject {
				assert.True(t, hasProject, "DOPPLER_PROJECT should be set")
			}
			if tt.expectConfig {
				assert.True(t, hasConfig, "DOPPLER_CONFIG should be set")
			}
		})
	}
}

// TestDopplerProviderCreation tests provider creation.
func TestDopplerProviderCreation(t *testing.T) {
	tests := []struct {
		name   string
		config DopplerConfig
	}{
		{
			name: "full_config",
			config: DopplerConfig{
				Token:   "dp.st.token",
				Project: "my-project",
				Config:  "production",
			},
		},
		{
			name:   "empty_config",
			config: DopplerConfig{},
		},
		{
			name: "partial_config_token_only",
			config: DopplerConfig{
				Token: "dp.st.partial",
			},
		},
		{
			name: "partial_config_project_only",
			config: DopplerConfig{
				Project: "test-project",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewDopplerProvider(tt.config)
			require.NotNil(t, p)
			assert.Equal(t, tt.config.Token, p.config.Token)
			assert.Equal(t, tt.config.Project, p.config.Project)
			assert.Equal(t, tt.config.Config, p.config.Config)
			assert.NotNil(t, p.logger)
		})
	}
}

// TestDopplerProviderName verifies the provider name.
func TestDopplerProviderName(t *testing.T) {
	p := NewDopplerProvider(DopplerConfig{})
	assert.Equal(t, "doppler", p.Name())
}

// TestDopplerProviderCapabilities verifies provider capabilities.
func TestDopplerProviderCapabilities(t *testing.T) {
	p := NewDopplerProvider(DopplerConfig{})
	caps := p.Capabilities()

	assert.False(t, caps.SupportsVersioning, "Doppler doesn't support versioning")
	assert.True(t, caps.SupportsMetadata, "Doppler supports metadata")
	assert.False(t, caps.SupportsWatching, "Doppler doesn't support watching")
	assert.False(t, caps.SupportsBinary, "Doppler doesn't support binary")
	assert.True(t, caps.RequiresAuth, "Doppler requires auth")
	assert.Contains(t, caps.AuthMethods, "service_token")
}

// TestDopplerCommandExecution tests that commands are properly built for execution.
func TestDopplerCommandExecution(t *testing.T) {
	// Test that the command builder creates a proper exec.Cmd structure
	config := DopplerConfig{
		Token:   "dp.st.test",
		Project: "test-proj",
		Config:  "dev",
	}
	p := NewDopplerProvider(config)
	ctx := context.Background()

	// Test secrets get command
	cmd := p.buildCommand(ctx, "secrets", "get", "MY_SECRET", "--json")

	// Verify it's an exec.Cmd
	require.IsType(t, &exec.Cmd{}, cmd)

	// Verify command structure
	assert.Equal(t, "doppler", cmd.Path)
	assert.Equal(t, []string{"doppler", "secrets", "get", "MY_SECRET", "--json"}, cmd.Args)

	// Verify environment is populated
	assert.NotEmpty(t, cmd.Env)
}

// TestDopplerEnvironmentInjection tests that environment variables are properly injected.
func TestDopplerEnvironmentInjection(t *testing.T) {
	config := DopplerConfig{
		Token:   "dp.st.test-token",
		Project: "project-name",
		Config:  "config-name",
	}
	p := NewDopplerProvider(config)
	ctx := context.Background()

	cmd := p.buildCommand(ctx, "secrets", "get", "--json")

	// Count Doppler-specific env vars
	dopplerEnvCount := 0
	for _, env := range cmd.Env {
		if len(env) >= 7 && env[:7] == "DOPPLER" {
			dopplerEnvCount++
		}
	}

	// Should have 3 Doppler environment variables
	assert.Equal(t, 3, dopplerEnvCount, "Should have DOPPLER_TOKEN, DOPPLER_PROJECT, DOPPLER_CONFIG")
}

// TestDopplerTokenSecurity tests that token handling is secure.
func TestDopplerTokenSecurity(t *testing.T) {
	// Verify that masked tokens don't reveal the full token
	sensitiveToken := "dp.st.very-secret-production-token-12345"
	p := &DopplerProvider{
		config: DopplerConfig{
			Token: sensitiveToken,
		},
	}

	masked := p.maskToken()

	// Masked version should only show last 4 chars
	assert.Equal(t, "2345", masked)

	// Should not contain the full token
	assert.NotContains(t, masked, "very-secret")
	assert.NotContains(t, masked, "production")

	// Should be significantly shorter than original
	assert.Less(t, len(masked), len(sensitiveToken))
}
