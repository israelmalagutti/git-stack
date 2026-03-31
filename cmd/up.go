package cmd

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var upCmd = &cobra.Command{
	Use:   "up [steps]",
	Short: "Move up the stack toward leaves",
	Long: `Move up the stack by checking out child branches.

Default is 1 step. Specify a number to move multiple steps.
If multiple children exist, prompts for selection.

Example:
  gs up      # Move to child branch
  gs up 2    # Move 2 levels toward leaves`,
	Aliases: []string{"u"},
	Args:    cobra.MaximumNArgs(1),
	RunE:    runUp,
}

func init() {
	rootCmd.AddCommand(upCmd)
}

func runUp(cmd *cobra.Command, args []string) error {
	// Parse steps
	steps := 1
	if len(args) > 0 {
		n, err := strconv.Atoi(args[0])
		if err != nil || n < 1 {
			return fmt.Errorf("invalid step count: %s", args[0])
		}
		steps = n
	}

	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Load config
	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return err
	}

	// Load metadata
	metadata, err := loadMetadata(repo)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Get current branch
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Build stack
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	// Navigate up
	targetBranch := currentBranch
	for i := 0; i < steps; i++ {
		node := s.GetNode(targetBranch)
		if node == nil {
			return fmt.Errorf("branch '%s' not found in stack", targetBranch)
		}

		if len(node.Children) == 0 {
			if i == 0 {
				return fmt.Errorf("already at top of stack")
			}
			fmt.Printf("%s Reached top after %d step(s)\n", colors.Info("→"), i)
			break
		}

		// If multiple children, prompt for selection
		if len(node.Children) == 1 {
			targetBranch = node.Children[0].Name
		} else {
			// Build options
			options := make([]string, len(node.Children))
			for j, child := range node.Children {
				options[j] = child.Name
			}

			prompt := &survey.Select{
				Message: fmt.Sprintf("Multiple children of '%s'. Select branch:", targetBranch),
				Options: options,
			}

			var selected string
			if err := askOne(prompt, &selected); err != nil {
				if errors.Is(err, terminal.InterruptErr) {
					fmt.Println("Cancelled.")
					return nil
				}
				return fmt.Errorf("selection cancelled: %w", err)
			}

			targetBranch = selected
		}
	}

	// Checkout target
	if targetBranch == currentBranch {
		return nil
	}

	if err := repo.CheckoutBranch(targetBranch); err != nil {
		return fmt.Errorf("failed to checkout '%s': %w", targetBranch, err)
	}

	colors.PrintNav("up", targetBranch)
	return nil
}
