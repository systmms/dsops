package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
)

const exampleConfig = `version: 0

# Named providers. All fields are provider-specific.
providers:
  # Password manager examples
  bitwarden:
    type: bitwarden
    # profile: default  # optional, if you use multiple bw profiles
  
  onepassword:
    type: onepassword
    # account: myteam.1password.com  # optional
  
  # Cloud provider examples (uncomment as needed)
  aws_sm:
    type: aws.secretsmanager
    region: us-east-1
    
  # gcp_sm:
  #   type: gcp.secretmanager
  #   project_id: my-project

# Environment definitions
envs:
  development:
    # Example variables - replace with your actual secrets
    # Provider examples (replace with your actual secrets)
    DATABASE_URL:
      from: { provider: bitwarden, key: "dev-database.uri0" }
      # Or from AWS: { provider: aws_sm, key: "dev/database/url" }
    
    API_KEY:
      from: { provider: aws_sm, key: "dev/api/key" }
      # Or from Bitwarden: { provider: bitwarden, key: "api-keys.password" }
      # Or from 1Password: { provider: onepassword, key: "api-keys.password" }
    
    # Literal values for non-secret config
    DEBUG_MODE:
      literal: "true"
    
    NODE_ENV:
      literal: "development"
  
  # production:
  #   DATABASE_URL:
  #     from: { provider: bitwarden, key: "prod-database.uri0" }
  #   API_KEY:
  #     from: { provider: bitwarden, key: "prod-api.password" }
  #   NODE_ENV:
  #     literal: "production"
`

func NewInitCommand(cfg *config.Config) *cobra.Command {
	var example string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize a new dsops configuration",
		Long:  "Create a dsops.yaml file with example configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Check if dsops.yaml already exists
			if _, err := os.Stat(cfg.Path); err == nil {
				return fmt.Errorf("dsops.yaml already exists. Remove it first if you want to reinitialize")
			}

			// TODO: Support different example stacks
			content := exampleConfig

			// Write the file
			if err := os.WriteFile(cfg.Path, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write config file: %w", err)
			}

			cfg.Logger.Info("Created dsops.yaml with example providers and environments")
			cfg.Logger.Info("Next steps:")
			cfg.Logger.Info("  1. Edit dsops.yaml to configure your providers and secrets")
			cfg.Logger.Info("  2. Run 'dsops doctor' to verify provider connectivity")
			cfg.Logger.Info("  3. Run 'dsops plan --env development' to preview your configuration")
			cfg.Logger.Info("  4. Run 'dsops exec --env development -- <your-command>' to run with secrets")

			return nil
		},
	}

	cmd.Flags().StringVar(&example, "example", "", "Example configuration stack (e.g., 'node', 'go', 'python')")

	return cmd
}