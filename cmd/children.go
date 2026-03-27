package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var childrenCmd = &cobra.Command{
	Use:   "children [branch]",
	Short: "Show the children branches",
	Long: `Show all children branches of the specified branch.

If no branch is specified, shows children of the current branch.`,
	RunE: runChildren,
}

func init() {
	rootCmd.AddCommand(childrenCmd)
}

func runChildren(cmd *cobra.Command, args []string) error {
	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Load config
	_, err = config.Load(repo.GetConfigPath())
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

	// Get children
	children := metadata.GetChildren(branchName)
	if len(children) == 0 {
		fmt.Printf("Branch '%s' has no children\n", branchName)
		return nil
	}

	fmt.Printf("Children of '%s':\n", branchName)
	for _, child := range children {
		fmt.Printf("  %s\n", child)
	}

	return nil
}
