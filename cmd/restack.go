package cmd

import (
	"github.com/spf13/cobra"
)

var restackCmd = &cobra.Command{
	Use:     "restack",
	Aliases: []string{"rs"},
	Short:   "Restack current branch and its children",
	Long: `Rebase branches to maintain parent-child relationships.

This is an alias for 'gs stack restack'.

Scope flags (mutually exclusive):
  --only        Restack only the current branch
  --upstack     Restack current branch + descendants
  --downstack   Restack ancestors + current branch
  --branch      Start from a specific branch instead of current

Default (no flags): restack the full stack.

Example:
  gs restack              # Restack full stack
  gs restack --only       # Restack only current branch
  gs restack --upstack    # Restack current + descendants
  gs rs                   # Short alias`,
	RunE: runStackRestack,
}

func init() {
	rootCmd.AddCommand(restackCmd)
	registerRestackFlags(restackCmd)
}
