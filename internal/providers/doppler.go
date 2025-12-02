package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/logging"
	pkgexec "github.com/systmms/dsops/pkg/exec"
	"github.com/systmms/dsops/pkg/provider"
)

// DopplerProvider implements the provider.Provider interface for Doppler.
type DopplerProvider struct {
	config   DopplerConfig
	logger   *logging.Logger
	executor pkgexec.CommandExecutor
}

// DopplerConfig represents the configuration for the Doppler provider.
type DopplerConfig struct {
	Token   string `yaml:"token,omitempty"`   // Service token
	Project string `yaml:"project,omitempty"` // Project name
	Config  string `yaml:"config,omitempty"`  // Config/environment name
}

// dopplerSecret represents a secret response from Doppler CLI.
type dopplerSecret struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// dopplerSecretsResponse represents the response from doppler secrets get --json.
type dopplerSecretsResponse map[string]dopplerSecret

// NewDopplerProvider creates a new Doppler provider.
func NewDopplerProvider(config DopplerConfig) *DopplerProvider {
	logger := logging.New(false, false)
	return &DopplerProvider{
		config:   config,
		logger:   logger,
		executor: pkgexec.DefaultExecutor(),
	}
}

// NewDopplerProviderWithExecutor creates a new Doppler provider with a custom executor.
// This is primarily for testing, allowing command execution to be mocked.
func NewDopplerProviderWithExecutor(config DopplerConfig, executor pkgexec.CommandExecutor) *DopplerProvider {
	logger := logging.New(false, false)
	return &DopplerProvider{
		config:   config,
		logger:   logger,
		executor: executor,
	}
}

// Name returns the provider name.
func (p *DopplerProvider) Name() string {
	return "doppler"
}

// Capabilities returns the provider capabilities.
func (p *DopplerProvider) Capabilities() provider.Capabilities {
	return provider.Capabilities{
		SupportsVersioning: false,
		SupportsMetadata:   true,
		SupportsWatching:   false,
		SupportsBinary:     false,
		RequiresAuth:       true,
		AuthMethods:        []string{"service_token"},
	}
}

// Validate checks if the provider is properly configured.
func (p *DopplerProvider) Validate(ctx context.Context) error {
	// Check if doppler CLI is available
	if _, err := exec.LookPath("doppler"); err != nil {
		return dserrors.UserError{
			Message:    "Doppler CLI not found",
			Suggestion: "Install the Doppler CLI using: curl -sLf --connect-timeout 3 --retry 3 https://cli.doppler.com/install.sh | sh",
			Details:    "The Doppler CLI is required to authenticate and fetch secrets from Doppler",
			Err:        err,
		}
	}

	// Test authentication
	_, _, err := p.executeDoppler(ctx, "secrets", "get", "--json")
	if err != nil {
		return dserrors.UserError{
			Message:    "Failed to authenticate with Doppler",
			Suggestion: "Ensure your service token is valid and has access to the specified project/config",
			Details:    fmt.Sprintf("Authentication failed with token ending in ...%s", p.maskToken()),
			Err:        err,
		}
	}

	return nil
}

// Resolve retrieves a secret value from Doppler.
func (p *DopplerProvider) Resolve(ctx context.Context, ref provider.Reference) (provider.SecretValue, error) {
	secretName := ref.Key

	p.logger.Debug("Fetching secret %s from Doppler", logging.Secret(secretName))

	// Get the specific secret
	stdout, stderr, err := p.executeDoppler(ctx, "secrets", "get", secretName, "--json")
	if err != nil {
		stdoutStr := string(stdout)
		stderrStr := string(stderr)
		errStr := err.Error()
		if strings.Contains(stdoutStr, "not found") || strings.Contains(stderrStr, "not found") || strings.Contains(errStr, "not found") {
			return provider.SecretValue{}, dserrors.UserError{
				Message:    fmt.Sprintf("Secret '%s' not found in Doppler", secretName),
				Suggestion: fmt.Sprintf("Verify the secret name exists in project '%s' config '%s'", p.config.Project, p.config.Config),
				Details:    "You can list available secrets with: doppler secrets",
				Err:        err,
			}
		}

		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Failed to retrieve secret from Doppler",
			Suggestion: "Check your network connection and Doppler service status",
			Details:    fmt.Sprintf("Error retrieving secret '%s'", secretName),
			Err:        err,
		}
	}

	var secretResponse dopplerSecret
	if err := json.Unmarshal(stdout, &secretResponse); err != nil {
		return provider.SecretValue{}, dserrors.UserError{
			Message:    "Invalid response format from Doppler",
			Suggestion: "This might be a temporary issue with the Doppler service",
			Details:    fmt.Sprintf("Failed to parse JSON response for secret '%s'", secretName),
			Err:        err,
		}
	}

	p.logger.Debug("Successfully retrieved secret %s from Doppler", logging.Secret(secretName))

	return provider.SecretValue{
		Value:     secretResponse.Value,
		UpdatedAt: time.Now(),
		Metadata:  map[string]string{"name": secretResponse.Name},
	}, nil
}

// Describe returns metadata about a secret.
func (p *DopplerProvider) Describe(ctx context.Context, ref provider.Reference) (provider.Metadata, error) {
	secretName := ref.Key

	// Get all secrets to find metadata about the specific one
	stdout, _, err := p.executeDoppler(ctx, "secrets", "get", "--json")
	if err != nil {
		return provider.Metadata{}, dserrors.UserError{
			Message:    "Failed to describe secrets from Doppler",
			Suggestion: "Check your network connection and Doppler service status",
			Details:    "Cannot retrieve secret metadata",
			Err:        err,
		}
	}

	var secrets dopplerSecretsResponse
	if err := json.Unmarshal(stdout, &secrets); err != nil {
		return provider.Metadata{}, dserrors.UserError{
			Message:    "Invalid response format from Doppler",
			Suggestion: "This might be a temporary issue with the Doppler service",
			Details:    "Failed to parse secrets list response",
			Err:        err,
		}
	}

	secret, exists := secrets[secretName]
	if !exists {
		return provider.Metadata{}, dserrors.UserError{
			Message:    fmt.Sprintf("Secret '%s' not found in Doppler", secretName),
			Suggestion: fmt.Sprintf("Verify the secret name exists in project '%s' config '%s'", p.config.Project, p.config.Config),
			Details:    "You can list available secrets with: doppler secrets",
			Err:        fmt.Errorf("secret not found"),
		}
	}

	return provider.Metadata{
		Exists:    true,
		UpdatedAt: time.Now(),
		Size:      len(secret.Value),
		Type:      "secret",
		Tags:      map[string]string{"project": p.config.Project, "config": p.config.Config},
	}, nil
}

// executeDoppler runs a doppler command with proper environment setup.
// This method handles authentication via environment variables.
func (p *DopplerProvider) executeDoppler(ctx context.Context, args ...string) (stdout []byte, stderr []byte, err error) {
	// If custom environment variables are needed, wrap in shell
	if p.config.Token != "" || p.config.Project != "" || p.config.Config != "" {
		// Build the environment prefix
		envPrefix := ""
		if p.config.Token != "" {
			envPrefix += fmt.Sprintf("DOPPLER_TOKEN=%s ", p.config.Token)
		}
		if p.config.Project != "" {
			envPrefix += fmt.Sprintf("DOPPLER_PROJECT=%s ", p.config.Project)
		}
		if p.config.Config != "" {
			envPrefix += fmt.Sprintf("DOPPLER_CONFIG=%s ", p.config.Config)
		}

		// Build the full doppler command
		dopplerCmd := "doppler"
		for _, arg := range args {
			// Quote arguments to handle spaces safely
			dopplerCmd += fmt.Sprintf(" %q", arg)
		}

		// Execute via shell with environment variables
		return p.executor.Execute(ctx, "sh", "-c", envPrefix+dopplerCmd)
	}

	// Direct execution without custom environment
	return p.executor.Execute(ctx, "doppler", args...)
}

// buildCommand creates a doppler CLI command with proper authentication.
// Deprecated: Use executeDoppler instead. This method is kept for backward compatibility
// but will be removed in a future version.
func (p *DopplerProvider) buildCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "doppler", args...)

	// Set environment variables for authentication
	env := os.Environ()

	// Use service token if provided
	if p.config.Token != "" {
		env = append(env, fmt.Sprintf("DOPPLER_TOKEN=%s", p.config.Token))
	}

	// Set project if provided
	if p.config.Project != "" {
		env = append(env, fmt.Sprintf("DOPPLER_PROJECT=%s", p.config.Project))
	}

	// Set config if provided
	if p.config.Config != "" {
		env = append(env, fmt.Sprintf("DOPPLER_CONFIG=%s", p.config.Config))
	}

	cmd.Env = env

	return cmd
}

// maskToken masks the service token for logging.
func (p *DopplerProvider) maskToken() string {
	if len(p.config.Token) < 6 {
		return "***"
	}
	return p.config.Token[len(p.config.Token)-4:]
}

