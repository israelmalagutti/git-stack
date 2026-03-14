package cmd

import (
	"github.com/spf13/cobra"
)

var stackCmd = &cobra.Command{
	Use:   "stack <command>",
	Short: "Manage stacks of branches",
	Long: `Manage stacks of branches.

A stack is a series of branches that depend on each other, with each branch
building on top of its parent. Stack commands help you maintain and submit
these branches.

Available commands:
  restack    Rebase stack to maintain parent-child relationships

Example:
  gs stack restack    # Rebase current stack
  gs stack r          # Short alias for restack`,
}

func init() {
	rootCmd.AddCommand(stackCmd)
}
