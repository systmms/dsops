package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
)

// NewSecretsStatusCommand creates the secrets status command
func NewSecretsStatusCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show rotation status for secrets (use 'dsops rotation status' instead)",
		Long:  `This command shows rotation status. For detailed status, use 'dsops rotation status'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("For rotation status, please use: dsops rotation status")
			return nil
		},
	}
	return cmd
}