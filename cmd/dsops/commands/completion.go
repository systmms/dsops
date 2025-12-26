package commands

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/systmms/dsops/internal/config"
)

// NewCompletionCommand creates the completion command for generating shell completions.
func NewCompletionCommand(cfg *config.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for dsops.

To load completions:

Bash:
  $ source <(dsops completion bash)

  # To load completions for each session, execute once:
  # Linux:
  $ dsops completion bash > /etc/bash_completion.d/dsops
  # macOS:
  $ dsops completion bash > $(brew --prefix)/etc/bash_completion.d/dsops

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc

  # To load completions for each session, execute once:
  $ dsops completion zsh > "${fpath[1]}/_dsops"

  # You will need to start a new shell for this setup to take effect.

Fish:
  $ dsops completion fish | source

  # To load completions for each session, execute once:
  $ dsops completion fish > ~/.config/fish/completions/dsops.fish

PowerShell:
  PS> dsops completion powershell | Out-String | Invoke-Expression

  # To load completions for every new session, run:
  PS> dsops completion powershell > dsops.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			}
			return nil
		},
	}

	return cmd
}
