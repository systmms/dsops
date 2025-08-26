package commands

import (
	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
)

// NewSecretsCommand creates the parent 'secrets' command
func NewSecretsCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets",
		Short: "Manage secret values (rotate, validate, etc.)",
		Long: `Manage the lifecycle of secret values like passwords, API keys, and certificates.

This command family focuses on rotating actual secret VALUES, not encryption keys.
For file encryption key rotation, use SOPS instead.

Examples:
  dsops secrets rotate --env production --key DATABASE_PASSWORD --strategy postgres
  dsops secrets status --env production
  dsops secrets history --env production --key API_KEY`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewSecretsRotateCommand(cfg),
		NewSecretsHistoryCommand(cfg),
		NewSecretsStatusCommand(cfg),
	)

	return cmd
}