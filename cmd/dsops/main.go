package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/cmd/dsops/commands"
	"github.com/systmms/dsops/internal/config"
	"github.com/systmms/dsops/internal/logging"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Global flags
	var (
		configFile      string
		noColor         bool
		debug           bool
		nonInteractive  bool
	)

	// Create config placeholder
	cfg := &config.Config{}

	rootCmd := &cobra.Command{
		Use:   "dsops",
		Short: "Developer Secret Operations - Manage secrets across providers",
		Long: `dsops pulls secrets from your vault(s) and renders .env files or 
launches commands with ephemeral environment variables.`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, commit, date),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Initialize logger with parsed flags
			logger := logging.New(debug, noColor)
			
			// Update config with parsed values
			cfg.Path = configFile
			cfg.Logger = logger
			cfg.NonInteractive = nonInteractive
		},
	}

	rootCmd.PersistentFlags().StringVar(&configFile, "config", "dsops.yaml", "Config file path")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug logging")
	rootCmd.PersistentFlags().BoolVar(&nonInteractive, "non-interactive", false, "Non-interactive mode")


	// Add commands
	rootCmd.AddCommand(
		commands.NewInitCommand(cfg),
		commands.NewPlanCommand(cfg),
		commands.NewRenderCommand(cfg),
		commands.NewExecCommand(cfg),
		commands.NewGetCommand(cfg),
		commands.NewDoctorCommand(cfg),
		commands.NewProvidersCommand(cfg),
		commands.NewLoginCommand(cfg),
		commands.NewShredCommand(cfg),
		commands.NewGuardCommand(cfg),
		commands.NewInstallHookCommand(cfg),
		commands.NewLeakCommand(cfg),
		commands.NewSecretsCommand(cfg),           // Secrets subcommand with rotation
		commands.NewRotationCommand(cfg),          // Rotation metadata commands
		commands.NewCompletionCommand(cfg),        // Shell completion generation
	)

	return rootCmd.Execute()
}