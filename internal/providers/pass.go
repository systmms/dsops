package providers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
	"github.com/systmms/dsops/pkg/provider"
)

// PassProvider implements the provider.Provider interface for pass (zx2c4).
type PassProvider struct {
	config PassConfig
	logger *logging.Logger
}

// PassConfig represents the configuration for the pass provider.
type PassConfig struct {
	PasswordStore string `yaml:"password_store,omitempty"` // Custom password store path (optional)
	GpgKey        string `yaml:"gpg_key,omitempty"`        // Specific GPG key to use (optional)
}

// NewPassProvider creates a new pass provider.
func NewPassProvider(config PassConfig) *PassProvider {
	logger := logging.New(false, false)
	return &PassProvider{
		config: config,
		logger: logger,
	}
}

// Name returns the provider name.
func (p *PassProvider) Name() string {
	return "pass"
}

// Capabilities returns the provider capabilities.
func (p *PassProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false, // pass doesn't have built-in versioning (though Git can provide this)
		SupportsMetadata:   true,  // Can provide basic metadata
		SupportsWatching:   false, // No built-in watching
		SupportsBinary:     false, // Text-based storage
		RequiresAuth:       true,  // Requires GPG key setup
		AuthMethods:        []string{"gpg_key"},
	}
}

// Validate checks if the provider is properly configured.
func (p *PassProvider) Validate(ctx context.Context) error {
	// Check if pass CLI is available
	if _, err := exec.LookPath("pass"); err != nil {
		return dserrors.UserError{
			Message:    "pass CLI not found",
			Suggestion: "Install pass: https://www.passwordstore.org/ (brew install pass, apt install pass, etc.)",
			Details:    "The pass command-line tool is required to access the password store",
			Err:        err,
		}
	}

	// Test basic functionality by listing the password store
	cmd := p.buildCommand(ctx, "list")
	if err := cmd.Run(); err != nil {
		return dserrors.UserError{
			Message:    "Failed to access pass password store",
			Suggestion: "Initialize pass with 'pass init <gpg-key-id>' or check that your GPG key is set up correctly",
			Details:    "Cannot list password store entries",
			Err:        err,
		}
	}

	return nil
}

// Resolve retrieves a secret value from pass.
func (p *PassProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	secretPath := ref.Key

	p.logger.Debug("Fetching secret %s from pass", logging.Secret(secretPath))

	// Use 'pass show' to get the password
	cmd := p.buildCommand(ctx, "show", secretPath)
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "not in the password store") || 
		   strings.Contains(string(output), "not in the password store") {
			return provider.SecretValue{}, dserrors.UserError{
				Message:    fmt.Sprintf("Secret '%s' not found in pass", secretPath),
				Suggestion: "Check the secret path with 'pass ls' or 'pass find <keyword>'",
				Details:    "The secret path might not exist or may be in a different location",
				Err:        err,
			}
		}

		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to retrieve secret from pass",
			Suggestion: "Check your GPG key setup and that the password store is accessible",
			Details:    fmt.Sprintf("Error retrieving secret '%s'", secretPath),
			Err:        err,
		}
	}

	// pass stores the password on the first line, with optional additional data on subsequent lines
	secretValue := strings.TrimSpace(string(output))
	
	// Extract just the password (first line) if there are multiple lines
	lines := strings.Split(secretValue, "\n")
	password := lines[0]

	p.logger.Debug("Successfully retrieved secret %s from pass", logging.Secret(secretPath))

	metadata := map[string]string{
		"path": secretPath,
	}

	// If there are additional lines, include them as metadata
	if len(lines) > 1 {
		metadata["additional_data"] = strings.Join(lines[1:], "\n")
	}

	return provider.SecretValue{
		Value:     password,
		UpdatedAt: time.Now(), // pass doesn't track modification times easily
		Metadata:  metadata,
	}, nil
}

// Describe returns metadata about a secret.
func (p *PassProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	secretPath := ref.Key

	// Check if the secret exists by trying to show it
	cmd := p.buildCommand(ctx, "show", secretPath)
	output, err := cmd.Output()
	if err != nil {
		return provider.Metadata{}, dserrors.UserError{
			Message:    fmt.Sprintf("Secret '%s' not found in pass", secretPath),
			Suggestion: "Check the secret path with 'pass ls' or 'pass find <keyword>'",
			Details:    "Cannot describe non-existent secret",
			Err:        err,
		}
	}

	secretContent := strings.TrimSpace(string(output))
	lines := strings.Split(secretContent, "\n")

	tags := map[string]string{
		"path": secretPath,
	}

	// Check if it's in a subdirectory (folder)
	if strings.Contains(secretPath, "/") {
		parts := strings.Split(secretPath, "/")
		if len(parts) > 1 {
			tags["folder"] = strings.Join(parts[:len(parts)-1], "/")
		}
	}

	// Determine type based on content structure
	secretType := "password"
	if len(lines) > 1 {
		secretType = "password_with_metadata"
	}

	return provider.Metadata{
		Exists:    true,
		UpdatedAt: time.Now(), // pass doesn't easily provide modification times
		Size:      len(secretContent),
		Type:      secretType,
		Tags:      tags,
	}, nil
}

// buildCommand creates a pass CLI command with proper environment setup.
func (p *PassProvider) buildCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "pass", args...)

	// Set environment variables if configured
	env := os.Environ()

	// Set custom password store path if provided
	if p.config.PasswordStore != "" {
		env = append(env, fmt.Sprintf("PASSWORD_STORE_DIR=%s", p.config.PasswordStore))
	}

	// Set specific GPG key if provided
	if p.config.GpgKey != "" {
		env = append(env, fmt.Sprintf("PASSWORD_STORE_KEY=%s", p.config.GpgKey))
	}

	cmd.Env = env

	return cmd
}

