package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Jump to the top of the stack",
	Long: `Jump directly to the top of the current stack (leaf branch).

If multiple leaf branches exist, prompts for selection.

Example:
  gs top    # Jump to top of stack`,
	Aliases: []string{"t"},
	Args:    cobra.NoArgs,
	RunE:    runTop,
}

func init() {
	rootCmd.AddCommand(topCmd)
}

func runTop(cmd *cobra.Command, args []string) error {
	rs, err := loadRepoState()
	if err != nil {
		return err
	}

	currentBranch, err := rs.Repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	node := rs.Stack.GetNode(currentBranch)
	if node == nil {
		return fmt.Errorf("branch '%s' not found in stack", currentBranch)
	}

	// Find all leaf branches reachable from current node
	leaves := findLeaves(node)

	if len(leaves) == 0 {
		return fmt.Errorf("already at top of stack")
	}

	// If current branch is already a leaf
	if len(node.Children) == 0 {
		fmt.Println("Already at top of stack")
		return nil
	}

	var targetBranch string

	if len(leaves) == 1 {
		targetBranch = leaves[0].Name
	} else {
		// Multiple leaves, prompt for selection
		options := make([]string, len(leaves))
		for i, leaf := range leaves {
			options[i] = leaf.Name
		}

		prompt := &survey.Select{
			Message: "Multiple stack tips. Select branch:",
			Options: options,
		}

		if err := askOne(prompt, &targetBranch); err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				fmt.Println("Cancelled.")
				return nil
			}
			return fmt.Errorf("selection cancelled: %w", err)
		}
	}

	// Checkout target
	if err := rs.Repo.CheckoutBranch(targetBranch); err != nil {
		return fmt.Errorf("failed to checkout '%s': %w", targetBranch, err)
	}

	fmt.Printf("Switched to %s\n", targetBranch)
	return nil
}

// findLeaves returns all leaf nodes (nodes with no children) in the subtree
func findLeaves(node *stack.Node) []*stack.Node {
	if len(node.Children) == 0 {
		return nil // Current node is a leaf, but we want descendants
	}

	var leaves []*stack.Node
	for _, child := range node.Children {
		if len(child.Children) == 0 {
			leaves = append(leaves, child)
		} else {
			leaves = append(leaves, findLeaves(child)...)
		}
	}
	return leaves
}
