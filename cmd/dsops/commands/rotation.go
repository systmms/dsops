package commands

import (
	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
)

// NewRotationCommand creates the parent 'rotation' command
func NewRotationCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rotation",
		Short: "Manage secret rotation metadata and history",
		Long: `View and manage secret rotation status and history.

This command family provides visibility into rotation operations:
- Current rotation status for services
- Historical rotation records
- Rotation metrics and compliance

Examples:
  # Show rotation status for all services
  dsops rotation status
  
  # Show rotation history for a specific service
  dsops rotation history postgres-prod
  
  # Show rotation status in JSON format
  dsops rotation status --format json`,
	}

	// Add subcommands
	cmd.AddCommand(
		NewRotationStatusCmd(cfg),
		NewRotationHistoryCmd(cfg),
	)

	return cmd
}