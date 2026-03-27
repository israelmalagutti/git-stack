package cmd

import (
	"fmt"
	"strconv"

	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
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

	// Already at trunk
	if currentBranch == cfg.Trunk {
		return fmt.Errorf("already at trunk")
	}

	// Build stack
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	// Navigate down
	targetBranch := currentBranch
	for i := 0; i < steps; i++ {
		node := s.GetNode(targetBranch)
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

	if err := repo.CheckoutBranch(targetBranch); err != nil {
		return fmt.Errorf("failed to checkout '%s': %w", targetBranch, err)
	}

	colors.PrintNav("down", targetBranch)
	return nil
}
