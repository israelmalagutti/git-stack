package cmd

import (
	"github.com/spf13/cobra"
)

var restackCmd = &cobra.Command{
	Use:     "restack",
	Aliases: []string{"rs"},
	Short:   "Restack current branch and its children",
	Long: `Rebase branches to maintain parent-child relationships.

This is an alias for 'gw stack restack'.

Scope flags (mutually exclusive):
  --only        Restack only the current branch
  --upstack     Restack current branch + descendants
  --downstack   Restack ancestors + current branch
  --branch      Start from a specific branch instead of current

Default (no flags): restack the full stack.

Example:
  gw restack              # Restack full stack
  gw restack --only       # Restack only current branch
  gw restack --upstack    # Restack current + descendants
  gw rs                   # Short alias`,
	RunE: runStackRestack,
}

func init() {
	rootCmd.AddCommand(restackCmd)
	registerRestackFlags(restackCmd)
}
