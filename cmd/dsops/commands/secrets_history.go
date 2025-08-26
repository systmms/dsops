package commands

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
)

// NewSecretsHistoryCommand creates the secrets history command
func NewSecretsHistoryCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Show rotation history for secrets (use 'dsops rotation history' instead)",
		Long:  `This command shows rotation history. For detailed history, use 'dsops rotation history'.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("For rotation history, please use: dsops rotation history")
			return nil
		},
	}
	return cmd
}