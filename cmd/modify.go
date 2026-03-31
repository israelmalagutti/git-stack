package cmd

import (
	"fmt"
	"strings"

	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	modifyCommit  bool
	modifyAll     bool
	modifyPatch   bool
	modifyMessage string
)

var modifyCmd = &cobra.Command{
	Use:   "modify",
	Short: "Amend current branch and restack children",
	Long: `Modify the current branch by amending its commit or creating a new commit.
Automatically restacks descendants.

If you have unstaged changes, you can stage them with --all or --patch.

Example:
  gs modify              # Amend current commit
  gs modify -c           # Create new commit
  gs modify -a           # Stage all changes and amend
  gs modify -m "msg"     # Amend with message
  gs modify -c -m "msg"  # Create new commit with message`,
	Aliases: []string{"m"},
	RunE:    runModify,
}

func init() {
	modifyCmd.Flags().BoolVarP(&modifyCommit, "commit", "c", false, "Create a new commit instead of amending")
	modifyCmd.Flags().BoolVarP(&modifyAll, "all", "a", false, "Stage all changes before committing")
	modifyCmd.Flags().BoolVarP(&modifyPatch, "patch", "p", false, "Interactively stage changes")
	modifyCmd.Flags().StringVarP(&modifyMessage, "message", "m", "", "Commit message")
	rootCmd.AddCommand(modifyCmd)
}

func runModify(cmd *cobra.Command, args []string) error {
	rs, err := loadRepoConfig()
	if err != nil {
		return err
	}

	repo, cfg, metadata := rs.Repo, rs.Config, rs.Metadata

	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Check if current branch is tracked (trunk is allowed)
	isTrunk := currentBranch == cfg.Trunk
	if !isTrunk && !metadata.IsTracked(currentBranch) {
		return fmt.Errorf("branch '%s' is not tracked by gs", currentBranch)
	}

	// Check if branch has commits (skip for trunk)
	if !isTrunk {
		hasCommits, err := branchHasCommits(repo, currentBranch, cfg.Trunk)
		if err != nil {
			return fmt.Errorf("failed to check commits: %w", err)
		}

		// If no commits, force new commit
		if !hasCommits {
			modifyCommit = true
			fmt.Println("Branch has no commits, creating new commit...")
		}
	}

	// Stage changes if requested
	if modifyPatch {
		fmt.Println("Interactively staging changes...")
		if _, err := repo.RunGitCommand("add", "--patch"); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
	} else if modifyAll {
		fmt.Println("Staging all changes...")
		if _, err := repo.RunGitCommand("add", "--all"); err != nil {
			return fmt.Errorf("failed to stage changes: %w", err)
		}
	} else {
		// Check for unstaged changes
		hasUnstaged, err := hasUnstagedChanges(repo)
		if err != nil {
			return fmt.Errorf("failed to check for unstaged changes: %w", err)
		}

		if hasUnstaged {
			fmt.Println("⚠ You have unstaged changes.")
			fmt.Println("Use -a to stage all, or -p to stage interactively.")
			return fmt.Errorf("unstaged changes present")
		}
	}

	// Check for staged changes
	hasStaged, err := hasStagedChanges(repo)
	if err != nil {
		return fmt.Errorf("failed to check for staged changes: %w", err)
	}

	if !hasStaged && !modifyCommit {
		fmt.Println("✓ No changes to commit")
		return nil
	}

	// Build commit command
	commitArgs := []string{}
	if modifyCommit {
		commitArgs = append(commitArgs, "commit")
	} else {
		commitArgs = append(commitArgs, "commit", "--amend", "--no-edit")
	}

	// Add message if provided
	if modifyMessage != "" {
		commitArgs = append(commitArgs, "-m", modifyMessage)
	}

	// Commit changes
	action := "Amending"
	if modifyCommit {
		action = "Creating"
	}
	fmt.Printf("%s commit...\n", action)

	if _, err := repo.RunGitCommand(commitArgs...); err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	if modifyCommit {
		fmt.Println("✓ Created commit")
	} else {
		fmt.Println("✓ Amended commit")
	}

	// Build stack to check for children
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	node := s.GetNode(currentBranch)
	if node == nil {
		return fmt.Errorf("branch '%s' not found in stack", currentBranch)
	}

	// Restack children if any
	if len(node.Children) > 0 {
		fmt.Println("\nRestacking children...")
		if err := restackChildren(repo, s, node); err != nil {
			return fmt.Errorf("failed to restack children: %w", err)
		}
		fmt.Println("✓ Children restacked")
	}

	// Push updated metadata refs to remote
	pushMetadataRefs(repo)

	return nil
}

// branchHasCommits checks if a branch has any commits beyond trunk
func branchHasCommits(repo *git.Repo, branch, trunk string) (bool, error) {
	output, err := repo.RunGitCommand("rev-list", "--count", fmt.Sprintf("%s..%s", trunk, branch))
	if err != nil {
		return false, err
	}

	count := strings.TrimSpace(output)
	return count != "0", nil
}

// hasUnstagedChanges checks if there are unstaged changes
func hasUnstagedChanges(repo *git.Repo) (bool, error) {
	output, err := repo.RunGitCommand("diff", "--quiet")
	if err != nil {
		// Exit code 1 means there are differences
		return true, nil
	}
	return output != "", nil
}
