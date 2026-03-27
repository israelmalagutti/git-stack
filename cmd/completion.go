package cmd

import (
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for gs.

To load completions:

  Bash:
    $ source <(gs completion bash)
    # To load completions for each session, execute once:
    # Linux:
    $ gs completion bash > /etc/bash_completion.d/gs
    # macOS:
    $ gs completion bash > $(brew --prefix)/etc/bash_completion.d/gs

  Zsh:
    $ source <(gs completion zsh)
    # To load completions for each session, execute once:
    $ gs completion zsh > "${fpath[1]}/_gs"

  Fish:
    $ gs completion fish | source
    # To load completions for each session, execute once:
    $ gs completion fish > ~/.config/fish/completions/gs.fish

  PowerShell:
    PS> gs completion powershell | Out-String | Invoke-Expression
    # To load completions for each session, execute once:
    PS> gs completion powershell > gs.ps1
    # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(cmd.OutOrStdout())
		case "zsh":
			return rootCmd.GenZshCompletion(cmd.OutOrStdout())
		case "fish":
			return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
		case "powershell":
			return rootCmd.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)

	// Register branch-name completions for commands that take branch arguments
	for _, cmd := range []*cobra.Command{
		checkoutCmd,
		deleteCmd,
		infoCmd,
		parentCmd,
		childrenCmd,
		trackCmd,
		untrackCmd,
	} {
		cmd.ValidArgsFunction = completeBranchNames
	}

	// move takes a target branch (any tracked branch)
	moveCmd.ValidArgsFunction = completeBranchNames

	// Register flag completions
	_ = moveCmd.RegisterFlagCompletionFunc("onto", completeBranchNamesFunc)
	_ = moveCmd.RegisterFlagCompletionFunc("target", completeBranchNamesFunc)
	_ = moveCmd.RegisterFlagCompletionFunc("source", completeBranchNamesFunc)
}

func completeBranchNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return completeBranchNamesFunc(cmd, args, toComplete)
}

func completeBranchNamesFunc(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	repo, err := git.NewRepo()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	metadata, err := loadMetadata(repo)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var branches []string
	branches = append(branches, cfg.Trunk)
	for branch := range metadata.Branches {
		if branch != cfg.Trunk {
			branches = append(branches, branch)
		}
	}

	return branches, cobra.ShellCompDirectiveNoFileComp
}
