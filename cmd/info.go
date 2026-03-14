package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info [branch]",
	Short: "Display information about a branch",
	Long: `Display detailed information about a branch in the stack.

If no branch is specified, shows information for the current branch.

Information includes:
  - Parent branch
  - Children branches
  - Stack path from trunk
  - Commit SHA
  - Stack depth`,
	RunE: runInfo,
}

func init() {
	rootCmd.AddCommand(infoCmd)
}

func runInfo(cmd *cobra.Command, args []string) error {
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
	metadata, err := config.LoadMetadata(repo.GetMetadataPath())
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Determine which branch to show info for
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

	// Verify branch exists
	if !repo.BranchExists(branchName) {
		return fmt.Errorf("branch '%s' does not exist", branchName)
	}

	// Build stack
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	// Get node
	node := s.GetNode(branchName)
	if node == nil {
		fmt.Printf("Branch: %s\n", branchName)
		fmt.Println("Status: Not tracked by gs")
		fmt.Printf("\nRun 'gs track %s' to start tracking this branch\n", branchName)
		return nil
	}

	// Display info
	fmt.Printf("Branch: %s\n", branchName)

	if node.IsTrunk {
		fmt.Println("Type: Trunk branch")
	} else {
		fmt.Println("Type: Tracked branch")
	}

	// Commit info
	if node.CommitSHA != "" {
		fmt.Printf("Commit: %s\n", node.CommitSHA[:7])

		// Get commit message
		msg, err := repo.RunGitCommand("log", "-1", "--format=%s", branchName)
		if err == nil && msg != "" {
			fmt.Printf("Message: %s\n", msg)
		}
	}

	// Parent
	if node.Parent != nil {
		fmt.Printf("Parent: %s\n", node.Parent.Name)
	} else {
		fmt.Println("Parent: None (root)")
	}

	// Children
	children := s.GetChildren(branchName)
	if len(children) > 0 {
		fmt.Printf("Children: %d\n", len(children))
		for _, child := range children {
			fmt.Printf("  - %s\n", child.Name)
		}
	} else {
		fmt.Println("Children: None")
	}

	// Stack depth
	depth := s.GetStackDepth(branchName)
	if depth >= 0 {
		fmt.Printf("Stack depth: %d\n", depth)
	}

	// Path from trunk
	path := s.RenderPath(branchName)
	if path != "" {
		fmt.Printf("Path: %s\n", path)
	}

	return nil
}
