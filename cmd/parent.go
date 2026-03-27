package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var parentCmd = &cobra.Command{
	Use:   "parent [branch]",
	Short: "Show the parent branch",
	Long: `Show the parent branch of the specified branch.

If no branch is specified, shows the parent of the current branch.`,
	RunE: runParent,
}

func init() {
	rootCmd.AddCommand(parentCmd)
}

func runParent(cmd *cobra.Command, args []string) error {
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

	// Determine which branch
	var branchName string
	if len(args) > 0 {
		branchName = args[0]
	} else {
		currentBranch, err := repo.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branchName = currentBranch
	}

	// Trunk has no parent
	if branchName == cfg.Trunk {
		return fmt.Errorf("trunk branch has no parent")
	}

	// Check if branch is tracked
	if !metadata.IsTracked(branchName) {
		return fmt.Errorf("branch '%s' is not tracked by gs", branchName)
	}

	// Get parent
	parent, ok := metadata.GetParent(branchName)
	if !ok {
		return fmt.Errorf("branch '%s' has no parent", branchName)
	}

	fmt.Println(parent)
	return nil
}
