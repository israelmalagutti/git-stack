package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	foldKeep  bool
	foldForce bool
)

var foldCmd = &cobra.Command{
	Use:   "fold",
	Short: "Fold current branch into its parent",
	Long: `Fold the current branch's changes into its parent branch.

This command:
- Merges the current branch's commits into the parent branch
- Updates all children to point to the parent branch
- Restacks all descendants
- Deletes the current branch (unless --keep is used)

This is useful when you realize a branch should have been part of its parent.

Example:
  gs fold           # Fold into parent, delete current branch
  gs fold --keep    # Fold into parent, keep current branch name`,
	RunE: runFold,
}

func init() {
	foldCmd.Flags().BoolVarP(&foldKeep, "keep", "k", false, "Keep current branch name instead of deleting it")
	foldCmd.Flags().BoolVarP(&foldForce, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(foldCmd)
}

func runFold(cmd *cobra.Command, args []string) error {
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

	// Don't fold trunk
	if currentBranch == cfg.Trunk {
		return fmt.Errorf("cannot fold trunk branch")
	}

	// Check if current branch is tracked
	if !metadata.IsTracked(currentBranch) {
		return fmt.Errorf("branch '%s' is not tracked by gs", currentBranch)
	}

	// Get parent branch
	parentBranch, ok := metadata.GetParent(currentBranch)
	if !ok || parentBranch == "" {
		return fmt.Errorf("current branch has no parent")
	}

	// Build stack to get children
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	currentNode := s.GetNode(currentBranch)
	if currentNode == nil {
		return fmt.Errorf("current branch not found in stack")
	}

	// Confirm with user (unless --force)
	if !foldForce {
		action := fmt.Sprintf("fold '%s' into '%s'", currentBranch, parentBranch)
		if !foldKeep {
			action += fmt.Sprintf(" and delete '%s'", currentBranch)
		}

		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Are you sure you want to %s?", action),
			Default: false,
		}
		if err := askOne(prompt, &confirm); err != nil {
			return fmt.Errorf("confirmation cancelled: %w", err)
		}

		if !confirm {
			fmt.Println("Fold cancelled")
			return nil
		}
	}

	// Checkout parent branch
	fmt.Printf("Checking out '%s'...\n", parentBranch)
	if err := repo.CheckoutBranch(parentBranch); err != nil {
		return fmt.Errorf("failed to checkout parent: %w", err)
	}

	// Merge current branch into parent (squash merge to keep a clean history)
	fmt.Printf("Merging '%s' into '%s'...\n", currentBranch, parentBranch)
	if _, err := repo.RunGitCommand("merge", "--squash", currentBranch); err != nil {
		return fmt.Errorf("failed to merge: %w", err)
	}

	// Check if there are staged changes to commit
	hasStaged, err := hasStagedChanges(repo)
	if err != nil {
		return fmt.Errorf("failed to check staged changes: %w", err)
	}

	if hasStaged {
		// Commit the squashed changes
		commitMsg := fmt.Sprintf("Fold '%s' into '%s'", currentBranch, parentBranch)
		if _, err := repo.RunGitCommand("commit", "-m", commitMsg); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
		fmt.Println("✓ Merged and committed")
	} else {
		fmt.Println("✓ No changes to commit")
	}

	// Update children to point to parent instead of current branch
	if len(currentNode.Children) > 0 {
		fmt.Printf("\nUpdating %d child branch(es)...\n", len(currentNode.Children))
		for _, child := range currentNode.Children {
			if err := metadata.UpdateParent(child.Name, parentBranch); err != nil {
				return fmt.Errorf("failed to update child '%s': %w", child.Name, err)
			}
			fmt.Printf("  ✓ Updated '%s' parent to '%s'\n", child.Name, parentBranch)
		}

		if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}
	}

	// Remove current branch from metadata (unless --keep)
	if !foldKeep {
		metadata.UntrackBranch(currentBranch)
		if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		// Delete the branch
		fmt.Printf("\nDeleting branch '%s'...\n", currentBranch)
		if _, err := repo.RunGitCommand("branch", "-D", currentBranch); err != nil {
			return fmt.Errorf("failed to delete branch: %w", err)
		}
		fmt.Printf("✓ Deleted branch '%s'\n", currentBranch)
	} else {
		// Keep the branch but update its parent to grandparent
		grandparent, ok := metadata.GetParent(parentBranch)
		if ok {
			if err := metadata.UpdateParent(currentBranch, grandparent); err != nil {
				return fmt.Errorf("failed to update branch parent: %w", err)
			}
		} else {
			// Parent is trunk, so current becomes child of trunk
			if err := metadata.UpdateParent(currentBranch, cfg.Trunk); err != nil {
				return fmt.Errorf("failed to update branch parent: %w", err)
			}
		}

		if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}

		fmt.Printf("✓ Kept branch '%s'\n", currentBranch)
	}

	// Rebuild stack
	s, err = stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to rebuild stack: %w", err)
	}

	// Restack children from parent node
	parentNode := s.GetNode(parentBranch)
	if parentNode != nil && len(parentNode.Children) > 0 {
		fmt.Println("\nRestacking children...")
		if err := restackChildren(repo, s, parentNode); err != nil {
			return fmt.Errorf("failed to restack children: %w", err)
		}
		fmt.Println("✓ Children restacked")
	}

	fmt.Printf("\n✓ Folded '%s' into '%s'\n", currentBranch, parentBranch)

	return nil
}
