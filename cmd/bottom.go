package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var bottomCmd = &cobra.Command{
	Use:   "bottom",
	Short: "Jump to trunk branch",
	Long: `Jump directly to the trunk branch (bottom of the stack).

Example:
  gs bottom    # Checkout trunk`,
	Aliases: []string{"b"},
	Args:    cobra.NoArgs,
	RunE:    runBottom,
}

func init() {
	rootCmd.AddCommand(bottomCmd)
}

func runBottom(cmd *cobra.Command, args []string) error {
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

	// Get current branch
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Already at trunk
	if currentBranch == cfg.Trunk {
		fmt.Println("Already at trunk")
		return nil
	}

	// Checkout trunk
	if err := repo.CheckoutBranch(cfg.Trunk); err != nil {
		return fmt.Errorf("failed to checkout trunk: %w", err)
	}

	fmt.Printf("Switched to %s\n", cfg.Trunk)
	return nil
}
