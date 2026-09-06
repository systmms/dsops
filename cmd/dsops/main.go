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
	cfg := &config.Config{}
	return newRootCommand(cfg).Execute()
}

// newRootCommand builds the root command and wires the global flags into cfg.
// Configuration discovery happens here (not inside config.Load) so that a
// Config constructed directly in tests never consults the real environment.
func newRootCommand(cfg *config.Config) *cobra.Command {
	// Global flags
	var (
		configFile     string
		userConfigFile string
		noColor        bool
		debug          bool
		nonInteractive bool
	)

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
			cfg.UserConfig = config.ResolveUserConfigPath(
				userConfigFile,
				cmd.Flags().Changed("user-config"),
				config.DefaultUserConfigLookup(),
			)
			if cfg.UserConfig.Path != "" {
				logger.Debug("User config (%s): %s", cfg.UserConfig.Origin, cfg.UserConfig.Path)
			} else {
				logger.Debug("User config disabled")
			}
		},
	}

	rootCmd.PersistentFlags().StringVar(&configFile, "config", config.DefaultProjectConfigPath(os.Getenv),
		"Config file path (env: "+config.ProjectConfigEnvVar+")")
	rootCmd.PersistentFlags().StringVar(&userConfigFile, "user-config", "",
		"Machine-level config declaring secret stores for this user (env: "+config.UserConfigEnvVar+"; default: $XDG_CONFIG_HOME/dsops/config.yaml; '"+config.UserConfigDisabled+"' disables)")
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
		commands.NewSecretsCommand(cfg),    // Secrets subcommand with rotation
		commands.NewRotationCommand(cfg),   // Rotation metadata commands
		commands.NewCompletionCommand(cfg), // Shell completion generation
	)

	return rootCmd
}
