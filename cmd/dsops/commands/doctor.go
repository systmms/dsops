package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/providers"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/pkg/provider"
)

func NewDoctorCommand(cfg *config.Config) *cobra.Command {
	var (
		verbose bool
		envName string
		dataDir string
	)

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check provider connectivity and configuration",
		Long: `Verify that providers are properly configured and accessible.

This command checks:
- Configuration file validity
- Provider authentication and connectivity  
- Environment variable definitions
- Required tools and dependencies

Use --env to also validate a specific environment configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration
			cfg.Logger.Info("Checking dsops configuration...")
			if err := cfg.Load(); err != nil {
				cfg.Logger.Error("Configuration error: %v", err)
				return fmt.Errorf("failed to load config: %w", err)
			}
			cfg.Logger.Info("✓ Configuration loaded successfully")

			// Create resolver
			resolver := resolve.New(cfg)

			// Register and validate providers
			if err := registerProviders(resolver, cfg, dataDir); err != nil {
				cfg.Logger.Error("Provider registration error: %v", err)
				return fmt.Errorf("failed to register providers: %w", err)
			}

			// Check each provider (secret stores and legacy providers only, not services)
			ctx := context.Background()
			results := make([]ProviderHealth, 0)

			// Check secret stores
			for name, storeConfig := range cfg.Definition.SecretStores {
				health := ProviderHealth{
					Name:   name,
					Type:   storeConfig.Type,
					Status: "checking",
				}

				// Get the registered provider
				provider := getRegisteredProvider(resolver, name)
				if provider == nil {
					health.Status = "error"
					health.Error = "secret store not registered"
					health.Suggestions = []string{
						fmt.Sprintf("Secret store type '%s' may not be implemented", storeConfig.Type),
					}
					results = append(results, health)
					continue
				}

				// Validate provider with timeout
				health.Capabilities = provider.Capabilities()
				if err := resolver.ValidateProvider(ctx, name); err != nil {
					health.Status = "error"
					health.Error = err.Error()
					health.Suggestions = getSuggestions(storeConfig.Type, err)
				} else {
					health.Status = "healthy"
					health.Message = "Provider is ready"
				}

				results = append(results, health)
			}

			// Check legacy providers (for backward compatibility)
			for name, providerConfig := range cfg.Definition.Providers {
				health := ProviderHealth{
					Name:   name,
					Type:   providerConfig.Type,
					Status: "checking",
				}

				// Get the registered provider
				provider := getRegisteredProvider(resolver, name)
				if provider == nil {
					health.Status = "error"
					health.Error = "provider not registered"
					health.Suggestions = []string{
						fmt.Sprintf("Provider type '%s' may not be implemented", providerConfig.Type),
					}
					results = append(results, health)
					continue
				}

				// Validate provider with timeout
				health.Capabilities = provider.Capabilities()
				if err := resolver.ValidateProvider(ctx, name); err != nil {
					health.Status = "error"
					health.Error = err.Error()
					health.Suggestions = getSuggestions(providerConfig.Type, err)
				} else {
					health.Status = "healthy"
					health.Message = "Provider is ready"
				}

				results = append(results, health)
			}

			// Note: Services are not checked here as they are rotation targets, not secret providers

			// Display results
			displayHealthResults(results, verbose)

			// Check specific environment if requested
			if envName != "" {
				cfg.Logger.Info("\nChecking environment: %s", envName)
				if err := checkEnvironment(ctx, resolver, cfg, envName); err != nil {
					return fmt.Errorf("environment check failed: %w", err)
				}
			}

			// Summary
			healthy := 0
			for _, result := range results {
				if result.Status == "healthy" {
					healthy++
				}
			}

			fmt.Printf("\nSummary: %d/%d providers healthy\n", healthy, len(results))
			if healthy < len(results) {
				return fmt.Errorf("some providers are not healthy")
			}

			cfg.Logger.Info("✓ All systems operational!")
			return nil
		},
	}

	cmd.Flags().BoolVar(&verbose, "verbose", false, "Show detailed provider information")
	cmd.Flags().StringVar(&envName, "env", "", "Also check specific environment configuration")
	cmd.Flags().StringVar(&dataDir, "data-dir", "./dsops-data", "Path to dsops-data repository (optional)")

	return cmd
}

// ProviderHealth represents the health status of a provider
type ProviderHealth struct {
	Name         string
	Type         string
	Status       string   // healthy, error, checking
	Error        string
	Message      string
	Capabilities provider.Capabilities
	Suggestions  []string
}

// displayHealthResults shows provider health in a formatted table
func displayHealthResults(results []ProviderHealth, verbose bool) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	_, _ = fmt.Fprintf(w, "PROVIDER\tTYPE\tSTATUS\tMESSAGE\n")
	_, _ = fmt.Fprintf(w, "--------\t----\t------\t-------\n")

	for _, result := range results {
		status := result.Status
		message := result.Message
		if result.Error != "" {
			message = result.Error
		}

		// Add status emoji
		switch result.Status {
		case "healthy":
			status = "✓ " + status
		case "error":
			status = "✗ " + status
		default:
			status = "? " + status
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			result.Name, result.Type, status, message)
	}

	_ = w.Flush()

	// Show detailed info if verbose
	if verbose {
		for _, result := range results {
			if result.Status == "error" && len(result.Suggestions) > 0 {
				fmt.Printf("\n%s (%s) suggestions:\n", result.Name, result.Type)
				for _, suggestion := range result.Suggestions {
					fmt.Printf("  • %s\n", suggestion)
				}
			}

			if verbose && result.Status == "healthy" {
				fmt.Printf("\n%s capabilities:\n", result.Name)
				caps := result.Capabilities
				fmt.Printf("  • Versioning: %t\n", caps.SupportsVersioning)
				fmt.Printf("  • Metadata: %t\n", caps.SupportsMetadata)
				fmt.Printf("  • Auth required: %t\n", caps.RequiresAuth)
				if len(caps.AuthMethods) > 0 {
					fmt.Printf("  • Auth methods: %v\n", caps.AuthMethods)
				}
			}
		}
	}
}

// getSuggestions returns helpful suggestions for provider errors
func getSuggestions(providerType string, err error) []string {
	suggestions := make([]string, 0)

	switch providerType {
	case "bitwarden":
		suggestions = append(suggestions, "Install Bitwarden CLI: npm install -g @bitwarden/cli")
		if contains(err.Error(), "not found") {
			suggestions = append(suggestions, "Ensure 'bw' command is in your PATH")
		}
		if contains(err.Error(), "unauthenticated") {
			suggestions = append(suggestions, "Run: bw login your-email@example.com")
		}
		if contains(err.Error(), "locked") {
			suggestions = append(suggestions, "Run: bw unlock")
			suggestions = append(suggestions, "Export session: export BW_SESSION=\"session-key\"")
		}

	case "onepassword":
		suggestions = append(suggestions, "Install 1Password CLI from: https://developer.1password.com/docs/cli/get-started/")
		if contains(err.Error(), "not found") {
			suggestions = append(suggestions, "Ensure 'op' command is in your PATH")
		}

	case "aws.secretsmanager":
		suggestions = append(suggestions, "Configure AWS credentials via CLI, env vars, or IAM roles")
		if contains(err.Error(), "authentication") || contains(err.Error(), "credentials") {
			suggestions = append(suggestions, "Run: aws configure")
			suggestions = append(suggestions, "Or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
			suggestions = append(suggestions, "Verify with: aws sts get-caller-identity")
		}
		if contains(err.Error(), "region") {
			suggestions = append(suggestions, "Set AWS_REGION or configure region in dsops.yaml")
		}

	case "aws.ssm":
		suggestions = append(suggestions, "Configure AWS credentials via CLI, env vars, or IAM roles")
		suggestions = append(suggestions, "Run: aws configure")
		suggestions = append(suggestions, "Or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")

	case "keychain":
		var keychainErr *providers.KeychainError
		if errors.As(err, &keychainErr) {
			switch keychainErr.Op {
			case "query":
				if errors.Is(keychainErr.Err, providers.ErrKeychainItemNotFound) {
					suggestions = append(suggestions, "Ensure the secret exists in your keychain")
					suggestions = append(suggestions, "On macOS: Add via Keychain Access app")
					suggestions = append(suggestions, fmt.Sprintf("Or use: security add-generic-password -a \"%s\" -s \"%s\" -w", keychainErr.Account, keychainErr.Service))
				} else if errors.Is(keychainErr.Err, providers.ErrKeychainAccessDenied) {
					suggestions = append(suggestions, "Keychain access was denied")
					suggestions = append(suggestions, "On macOS: Check Keychain Access permissions")
				}
			case "validate":
				if errors.Is(keychainErr.Err, providers.ErrKeychainUnsupportedPlatform) {
					suggestions = append(suggestions, "Keychain is only available on macOS and Linux")
				} else if errors.Is(keychainErr.Err, providers.ErrKeychainHeadless) {
					suggestions = append(suggestions, "Keychain access requires a GUI session")
					suggestions = append(suggestions, "Set up a keyring daemon for headless use")
				}
			}
		} else {
			// Fallback to string matching for non-typed errors
			errStr := err.Error()
			if strings.Contains(errStr, "not available") || strings.Contains(errStr, "not supported") {
				suggestions = append(suggestions, "Keychain is only available on macOS and Linux")
				suggestions = append(suggestions, "On macOS: Ensure Keychain Access app is working")
				suggestions = append(suggestions, "On Linux: Install and configure gnome-keyring or KWallet")
			}
			if strings.Contains(errStr, "headless") || strings.Contains(errStr, "GUI") {
				suggestions = append(suggestions, "Keychain access requires a GUI session")
				suggestions = append(suggestions, "Set up a keyring daemon for headless use")
			}
			if strings.Contains(errStr, "not found") {
				suggestions = append(suggestions, "Ensure the secret exists in your keychain")
				suggestions = append(suggestions, "On macOS: Add via Keychain Access app")
				suggestions = append(suggestions, "Or use: security add-generic-password -a <account> -s <service> -w")
			}
		}

	case "infisical":
		suggestions = append(suggestions, "Configure Infisical authentication in dsops.yaml")
		var infisicalErr *providers.InfisicalError
		if errors.As(err, &infisicalErr) {
			switch infisicalErr.Op {
			case "auth":
				suggestions = append(suggestions, "Check your machine_identity or service_token credentials")
				suggestions = append(suggestions, "Verify client_id and client_secret are correct")
			case "fetch":
				switch infisicalErr.StatusCode {
				case 404:
					suggestions = append(suggestions, "Verify the secret name exists in Infisical")
					suggestions = append(suggestions, "Check that you're querying the correct environment")
				case 401:
					suggestions = append(suggestions, "Check your access permissions for this secret")
					suggestions = append(suggestions, "Authentication failed - verify credentials")
				case 403:
					suggestions = append(suggestions, "Check your access permissions for this secret")
				}
			}
		} else {
			// Fallback to string matching for non-typed errors
			errStr := err.Error()
			if strings.Contains(errStr, "unauthorized") || strings.Contains(errStr, "401") {
				suggestions = append(suggestions, "Check your machine_identity or service_token credentials")
				suggestions = append(suggestions, "Verify client_id and client_secret are correct")
			}
			if strings.Contains(errStr, "project") {
				suggestions = append(suggestions, "Verify project_id in configuration")
				suggestions = append(suggestions, "Ensure you have access to the project")
			}
			if strings.Contains(errStr, "environment") {
				suggestions = append(suggestions, "Verify environment slug (dev, staging, prod)")
			}
			if strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout") {
				suggestions = append(suggestions, "Check host URL (default: https://app.infisical.com)")
				suggestions = append(suggestions, "For self-hosted: verify your Infisical instance is running")
			}
		}

	case "akeyless":
		suggestions = append(suggestions, "Configure Akeyless authentication in dsops.yaml")
		var akeylessErr *providers.AkeylessError
		if errors.As(err, &akeylessErr) {
			switch akeylessErr.Op {
			case "auth":
				suggestions = append(suggestions, "Check your access_id and authentication configuration")
				suggestions = append(suggestions, "For API key: ensure access_key is correct")
				suggestions = append(suggestions, "For AWS IAM: verify your AWS credentials")
				suggestions = append(suggestions, "For Azure AD: check your Azure identity configuration")
				suggestions = append(suggestions, "For GCP: verify your GCP service account")
			case "fetch":
				if strings.Contains(akeylessErr.Message, "not found") {
					suggestions = append(suggestions, "Verify the secret path exists in Akeyless")
					suggestions = append(suggestions, fmt.Sprintf("Check path: %s", akeylessErr.Path))
				} else {
					suggestions = append(suggestions, "Check your access permissions for this secret")
				}
			case "describe":
				suggestions = append(suggestions, "Verify the secret path exists in Akeyless")
			}
		} else {
			// Fallback to string matching for non-typed errors
			errStr := err.Error()
			if strings.Contains(errStr, "authentication failed") || strings.Contains(errStr, "unauthorized") {
				suggestions = append(suggestions, "Check your access_id and authentication configuration")
				suggestions = append(suggestions, "For API key: ensure access_key is correct")
				suggestions = append(suggestions, "For AWS IAM: verify your AWS credentials")
				suggestions = append(suggestions, "For Azure AD: check your Azure identity configuration")
				suggestions = append(suggestions, "For GCP: verify your GCP service account")
			}
			if strings.Contains(errStr, "not found") {
				suggestions = append(suggestions, "Verify the secret path exists in Akeyless")
				suggestions = append(suggestions, "Check path format: /folder/secret-name")
			}
			if strings.Contains(errStr, "gateway") || strings.Contains(errStr, "connection") {
				suggestions = append(suggestions, "Check gateway_url configuration")
				suggestions = append(suggestions, "Default is Akeyless cloud: https://api.akeyless.io")
			}
		}

	default:
		suggestions = append(suggestions, "Check provider documentation")
		suggestions = append(suggestions, "Verify provider configuration in dsops.yaml")
	}

	return suggestions
}

// checkEnvironment validates a specific environment configuration
func checkEnvironment(ctx context.Context, resolver *resolve.Resolver, cfg *config.Config, envName string) error {
	// Run plan to check environment
	result, err := resolver.Plan(ctx, envName)
	if err != nil {
		return fmt.Errorf("failed to plan environment: %w", err)
	}

	// Display results
	errorCount := 0
	for _, variable := range result.Variables {
		if variable.Error != nil {
			errorCount++
		}
	}

	fmt.Printf("Environment '%s': %d variables, %d errors\n", envName, len(result.Variables), errorCount)

	if errorCount > 0 {
		fmt.Println("\nVariable errors:")
		for _, variable := range result.Variables {
			if variable.Error != nil {
				fmt.Printf("  ✗ %s: %s\n", variable.Name, variable.Error.Error())
			}
		}
		return fmt.Errorf("environment has %d variable errors", errorCount)
	}

	fmt.Printf("✓ Environment '%s' is ready\n", envName)
	return nil
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || 
		(len(s) > len(substr) && 
			(s[:len(substr)] == substr || 
			 s[len(s)-len(substr):] == substr || 
			 len(s) > len(substr)*2)))
}

// getRegisteredProvider gets a provider from the resolver
func getRegisteredProvider(resolver *resolve.Resolver, name string) provider.Provider {
	provider, exists := resolver.GetProvider(name)
	if !exists {
		return nil
	}
	return provider
}