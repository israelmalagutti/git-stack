package cmd

import (
	"fmt"
	"strconv"

	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down [steps]",
	Short: "Move down the stack toward trunk",
	Long: `Move down the stack by checking out parent branches.

Default is 1 step. Specify a number to move multiple steps.

Example:
  gs down      # Move to parent branch
  gs down 2    # Move 2 levels toward trunk`,
	Aliases: []string{"dn"},
	Args:    cobra.MaximumNArgs(1),
	RunE:    runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) error {
	// Parse steps
	steps := 1
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return fmt.Errorf("invalid step count: %s", args[0])
		}
		steps = n
	}

	rs, err := loadRepoState()
	if err != nil {
		return err
	}

	currentBranch, err := rs.Repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == rs.Config.Trunk {
		return fmt.Errorf("already at trunk")
	}

	// Navigate down
	targetBranch := currentBranch
	for i := 0; i < steps; i++ {
		node := rs.Stack.GetNode(targetBranch)
		if node == nil {
			return fmt.Errorf("branch '%s' not found in stack", targetBranch)
		}

		if node.Parent == nil {
			if i == 0 {
				return fmt.Errorf("already at trunk")
			}
			fmt.Printf("%s Reached trunk after %d step(s)\n", colors.Info("→"), i)
			break
		}

		targetBranch = node.Parent.Name
	}

	// Checkout target
	if targetBranch == currentBranch {
		return nil
	}

	if err := rs.Repo.CheckoutBranch(targetBranch); err != nil {
		return fmt.Errorf("failed to checkout '%s': %w", targetBranch, err)
	}

	colors.PrintNav("down", targetBranch)
	return nil
}
