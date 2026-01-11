package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
	dserrors "github.com/systmms/dsops/internal/errors"
	"github.com/systmms/dsops/internal/execenv"
	"github.com/systmms/dsops/internal/resolve"
	"github.com/systmms/dsops/internal/secure"
)

func NewExecCommand(cfg *config.Config) *cobra.Command {
	var (
		envName       string
		printVars     bool
		allowOverride bool
		workingDir    string
		timeout       int
	)

	cmd := &cobra.Command{
		Use:   "exec --env <name> -- <command> [args...]",
		Short: "Execute command with ephemeral environment variables",
		Long: `Execute a command with environment variables resolved from configured 
secret providers. Secrets are injected into the child process environment 
and never written to disk.

The command must be separated from dsops arguments with '--'.

Examples:
  dsops exec --env development -- npm start
  dsops exec --env production -- docker compose up
  dsops exec --env staging --print -- python app.py`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate arguments
			if len(args) == 0 {
				return dserrors.UserError{
					Message:    "No command specified",
					Suggestion: "Use: dsops exec --env <name> -- <command> [args...]",
				}
			}

			// Validate command
			if err := execenv.ValidateCommand(args); err != nil {
				cfg.Logger.Warn("Command validation: %s", err.Error())
			}

			// Load configuration
			if err := cfg.Load(); err != nil {
				return dserrors.UserError{
					Message:    "Failed to load configuration",
					Details:    err.Error(),
					Suggestion: "Check that dsops.yaml exists and is valid YAML",
					Err:        err,
				}
			}

			// Create resolver
			resolver := resolve.New(cfg)

			// Register providers
			if err := registerProviders(resolver, cfg, ""); err != nil {
				return dserrors.UserError{
					Message:    "Failed to register providers",
					Details:    err.Error(),
					Suggestion: "Check provider configuration in dsops.yaml. Run 'dsops doctor' to diagnose",
					Err:        err,
				}
			}

			// Resolve secrets
			ctx := context.Background()
			resolved, err := resolver.Resolve(ctx, envName)
			if err != nil {
				// The resolver already returns user-friendly errors
				return err
			}

			// Convert to environment map
			environment := make(map[string]string)
			var resolveErrors []string

			for name, variable := range resolved {
				if variable.Error != nil {
					resolveErrors = append(resolveErrors, fmt.Sprintf("%s: %s", name, variable.Error))
					continue
				}
				environment[name] = variable.Value
			}

			// Check for resolution errors
			if len(resolveErrors) > 0 {
				cfg.Logger.Error("Failed to resolve %d variables:", len(resolveErrors))
				for _, err := range resolveErrors {
					cfg.Logger.Error("  %s", err)
				}
				return dserrors.UserError{
					Message:    fmt.Sprintf("Failed to resolve %d variables", len(resolveErrors)),
					Details:    "See errors above for details",
					Suggestion: "Fix the errors and try again. Use 'dsops plan' to debug",
				}
			}

			cfg.Logger.Info("Successfully resolved %d environment variables", len(environment))

			// Wrap secrets in SecureBuffers for secure handling
			// This ensures secrets are encrypted in memory until needed
			secureEnv := make(map[string]*secure.SecureBuffer)
			var wrapErrors []string

			for name, value := range environment {
				buf, err := secure.NewSecureBufferFromString(value)
				if err != nil {
					wrapErrors = append(wrapErrors, fmt.Sprintf("%s: %s", name, err))
					continue
				}
				secureEnv[name] = buf
			}

			// Check for wrapping errors
			if len(wrapErrors) > 0 {
				// Cleanup any buffers created before error
				for _, buf := range secureEnv {
					buf.Destroy()
				}
				cfg.Logger.Error("Failed to secure %d variables:", len(wrapErrors))
				for _, err := range wrapErrors {
					cfg.Logger.Error("  %s", err)
				}
				return dserrors.UserError{
					Message:    fmt.Sprintf("Failed to secure %d variables", len(wrapErrors)),
					Details:    "This may indicate a memory protection issue",
					Suggestion: "Try running with --debug for more information",
				}
			}

			// Create executor
			executor := execenv.New(cfg.Logger)

			// Execute command with both Environment (for display) and SecureEnvironment (for execution)
			options := execenv.ExecOptions{
				Command:           args,
				Environment:       environment,  // Kept for --print display (masked)
				SecureEnvironment: secureEnv,    // Used for secure execution
				AllowOverride:     allowOverride,
				PrintVars:         printVars,
				WorkingDir:        workingDir,
				Timeout:           timeout,
			}

			return executor.Exec(ctx, options)
		},
	}

	cmd.Flags().StringVar(&envName, "env", "", "Environment name to use (required)")
	cmd.Flags().BoolVar(&printVars, "print", false, "Print resolved variables (values masked)")
	cmd.Flags().BoolVar(&allowOverride, "allow-override", false, "Allow existing environment variables to override dsops values")
	cmd.Flags().StringVar(&workingDir, "working-dir", "", "Working directory for the command")
	cmd.Flags().IntVar(&timeout, "timeout", 0, "Command timeout in seconds (0 for no timeout)")

	_ = cmd.MarkFlagRequired("env")

	return cmd
}