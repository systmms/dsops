package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/resolve"
)

func NewGetCommand(cfg *config.Config) *cobra.Command {
	var (
		envName    string
		varName    string
		jsonOutput bool
	)

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a single secret value",
		Long: `Retrieve and display a single secret value.

This command fetches a single variable's value from the configured provider
and outputs it to stdout. By default, only the raw value is printed, making
it suitable for scripting.

Examples:
  # Get a single value
  dsops get --env production --var DATABASE_URL

  # Get value with metadata in JSON format
  dsops get --env production --var API_KEY --json

  # Use in scripts
  export DB_URL=$(dsops get --env prod --var DATABASE_URL)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate flags
			if envName == "" {
				return dserrors.UserError{
					Message:    "Environment name is required",
					Suggestion: "Use --env <environment-name> to specify an environment",
				}
			}
			if varName == "" {
				return dserrors.UserError{
					Message:    "Variable name is required",
					Suggestion: "Use --var <variable-name> to specify which variable to get",
				}
			}

			// Load configuration
			if err := cfg.Load(); err != nil {
				// Config loader now returns user-friendly errors
				return err
			}

			// Get the environment
			env, err := cfg.GetEnvironment(envName)
			if err != nil {
				// GetEnvironment now returns user-friendly errors
				return err
			}

			// Get the variable
			variable, exists := env[varName]
			if !exists {
				// Build a list of available variables
				var available []string
				for name := range env {
					available = append(available, name)
				}
				
				suggestion := fmt.Sprintf("Check your environment '%s' for available variables", envName)
				if len(available) > 0 && len(available) <= 10 {
					suggestion = fmt.Sprintf("Available variables in '%s': %v", envName, available)
				} else if len(available) > 10 {
					suggestion = fmt.Sprintf("Environment '%s' has %d variables. Use 'dsops plan --env %s' to see them all", envName, len(available), envName)
				}
				
				return dserrors.ConfigError{
					Field:      "variable",
					Value:      varName,
					Message:    fmt.Sprintf("variable not found in environment '%s'", envName),
					Suggestion: suggestion,
				}
			}

			// Create resolver
			resolver := resolve.New(cfg)

			// Register providers
			if err := registerProviders(resolver, cfg, ""); err != nil {
				return fmt.Errorf("failed to register providers: %w", err)
			}

			// Create a minimal environment with just this variable
			singleVarEnv := config.Environment{
				varName: variable,
			}

			// Resolve the single variable using the detailed resolver
			ctx := context.Background()
			resolvedVars, err := resolver.ResolveVariablesConcurrently(ctx, singleVarEnv)
			if err != nil {
				// Resolver now returns user-friendly errors
				return err
			}

			// Check if the variable was resolved successfully
			resolved, exists := resolvedVars[varName]
			if !exists {
				return dserrors.UserError{
					Message:    fmt.Sprintf("Failed to resolve variable '%s'", varName),
					Suggestion: "Check that the provider is configured and the secret exists. Use 'dsops plan' to debug",
				}
			}

			// Check for resolution error
			if resolved.Error != nil {
				return resolved.Error
			}

			value := resolved.Value

			// Output the result
			if jsonOutput {
				// JSON output with metadata
				output := map[string]interface{}{
					"variable": varName,
					"value":    value,
					"environment": envName,
				}

				// Add provider info if available
				if variable.From != nil {
					output["provider"] = variable.From.Provider
					output["key"] = variable.From.Key
				} else if variable.Literal != "" {
					output["source"] = "literal"
				}

				encoder := json.NewEncoder(os.Stdout)
				encoder.SetIndent("", "  ")
				if err := encoder.Encode(output); err != nil {
					return fmt.Errorf("failed to encode JSON: %w", err)
				}
			} else {
				// Raw value output (default)
				fmt.Print(value)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment name (required)")
	cmd.Flags().StringVar(&varName, "var", "", "Variable name to get (required)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format with metadata")

	// Mark required flags
	_ = cmd.MarkFlagRequired("env")
	_ = cmd.MarkFlagRequired("var")

	return cmd
}