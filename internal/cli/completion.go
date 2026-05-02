package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/lHumaNl/echowarp/internal/i18n"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: i18n.T("cli_completion_short"),
		Long: `Generate shell completion script for the specified shell.

To load completions:

Bash:
  $ source <(echowarp completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ echowarp completion bash > /etc/bash_completion.d/echowarp
  # macOS:
  $ echowarp completion bash > $(brew --prefix)/etc/bash_completion.d/echowarp

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  # To load completions for each session, execute once:
  $ echowarp completion zsh > "${fpath[1]}/_echowarp"

Fish:
  $ echowarp completion fish | source
  # To load completions for each session, execute once:
  $ echowarp completion fish > ~/.config/fish/completions/echowarp.fish

PowerShell:
  PS> echowarp completion powershell | Out-String | Invoke-Expression
  # To load completions for every new session, run:
  PS> echowarp completion powershell > echowarp.ps1
  # and source this file from your PowerShell profile.
`,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		DisableFlagsInUseLine: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}
	return cmd
}
