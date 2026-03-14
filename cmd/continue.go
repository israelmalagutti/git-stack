package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var continueCmd = &cobra.Command{
	Use:   "continue",
	Short: "Continue after resolving conflicts",
	Long: `Continue a rebase operation after resolving conflicts.

This command:
1. Continues the in-progress rebase (git rebase --continue)
2. Updates parent revision tracking
3. Resumes restacking remaining branches from saved state

Use this after resolving merge conflicts during a restack operation.

Example:
  # After resolving conflicts:
  git add .
  gs continue`,
	RunE: runContinue,
}

func init() {
	rootCmd.AddCommand(continueCmd)
}

func runContinue(cmd *cobra.Command, args []string) error {
	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Check if a rebase is in progress
	rebaseActive := isRebaseInProgress(repo)

	if !rebaseActive {
		// Check if we have saved state without an active rebase
		state, _ := config.LoadContinueState(repo.GetContinueStatePath())
		if state == nil {
			fmt.Println(colors.Muted("No rebase in progress and no saved restack state."))
			return nil
		}
		// State exists but no rebase — maybe user already completed rebase manually
		// Fall through to resume remaining branches
	}

	// Continue the rebase if one is in progress
	if rebaseActive {
		fmt.Println(colors.Muted("Continuing rebase..."))
		if _, err := repo.RunGitCommand("rebase", "--continue"); err != nil {
			return fmt.Errorf("rebase --continue failed: resolve conflicts and try again")
		}
	}

	// Get current branch (the one we just finished rebasing)
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if rebaseActive {
		fmt.Printf("%s Rebased %s\n", colors.Success("✓"), colors.BranchCurrent(currentBranch))
	}

	// Load config and metadata
	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return err
	}

	metadata, err := config.LoadMetadata(repo.GetMetadataPath())
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Update ParentRevision for just-rebased branch
	if parent, ok := metadata.GetParent(currentBranch); ok {
		parentSHA, _ := repo.GetBranchCommit(parent)
		if parentSHA != "" {
			if err := metadata.SetParentRevision(currentBranch, parentSHA); err == nil {
				_ = metadata.Save(repo.GetMetadataPath())
			}
		}
	}

	// Load continue state
	state, err := config.LoadContinueState(repo.GetContinueStatePath())
	if err != nil {
		return fmt.Errorf("failed to load continue state: %w", err)
	}

	if state != nil && len(state.RemainingBranches) > 0 {
		// Remove the completed branch from remaining
		remaining := state.RemainingBranches
		if len(remaining) > 0 && remaining[0] == currentBranch {
			remaining = remaining[1:]
		}

		if len(remaining) > 0 {
			// Build stack for remaining branches
			s, err := stack.BuildStack(repo, cfg, metadata)
			if err != nil {
				return fmt.Errorf("failed to build stack: %w", err)
			}

			fmt.Println()
			fmt.Println(colors.Muted("Restacking remaining branches..."))

			// Continue the linear restack with remaining branches
			return runLinearRestack(repo, metadata, s, remaining, state.OriginalBranch)
		}

		// All done — clear state and return to original branch
		_ = config.ClearContinueState(repo.GetContinueStatePath())

		if state.OriginalBranch != "" && state.OriginalBranch != currentBranch {
			if err := repo.CheckoutBranch(state.OriginalBranch); err != nil {
				fmt.Printf("%s Could not return to %s: %v\n",
					colors.Warning("⚠"),
					colors.BranchCurrent(state.OriginalBranch),
					err)
			}
		}

		fmt.Println()
		fmt.Printf("%s All branches restacked!\n", colors.Success("✓"))
		return nil
	}

	// No saved state — fall back to restacking children of current branch
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	node := s.GetNode(currentBranch)
	if node != nil && len(node.Children) > 0 {
		fmt.Println()
		fmt.Println(colors.Muted("Restacking children..."))

		descendants := descendantsDFS(s, currentBranch)
		if len(descendants) > 0 {
			return runLinearRestack(repo, metadata, s, descendants, currentBranch)
		}
	}

	fmt.Println()
	fmt.Printf("%s All done!\n", colors.Success("✓"))

	return nil
}

// continueRestackChildren rebases children onto parent after a continue (used by tests)
func continueRestackChildren(repo *git.Repo, s *stack.Stack, parent *stack.Node) error {
	for _, child := range parent.Children {
		if err := repo.CheckoutBranch(child.Name); err != nil {
			return fmt.Errorf("failed to checkout '%s': %w", child.Name, err)
		}

		needsRebase, err := childNeedsRebase(repo, child.Name, parent.Name)
		if err != nil {
			return err
		}

		if !needsRebase {
			fmt.Printf("%s %s already up to date\n",
				colors.Success("✓"),
				colors.BranchCurrent(child.Name))
		} else {
			if _, err := repo.RunGitCommand("rebase", parent.Name, child.Name); err != nil {
				fmt.Println()
				fmt.Printf("%s Conflict restacking %s onto %s\n",
					colors.Warning("⚠"),
					colors.BranchCurrent(child.Name),
					colors.BranchParent(parent.Name))
				fmt.Println()
				fmt.Println(colors.Muted("To continue:"))
				fmt.Println(colors.Muted("  1. Resolve conflicts"))
				fmt.Println(colors.Muted("  2. git add ."))
				fmt.Println(colors.Muted("  3. gs continue"))
				fmt.Println()
				fmt.Println(colors.Muted("To abort: git rebase --abort"))
				return fmt.Errorf("rebase conflict")
			}

			fmt.Printf("%s Restacked %s onto %s\n",
				colors.Success("✓"),
				colors.BranchCurrent(child.Name),
				colors.BranchParent(parent.Name))
		}

		if len(child.Children) > 0 {
			if err := continueRestackChildren(repo, s, child); err != nil {
				return err
			}
		}
	}

	return nil
}

// childNeedsRebase checks if a child branch needs rebasing onto parent
func childNeedsRebase(repo *git.Repo, child, parent string) (bool, error) {
	mergeBase, err := repo.RunGitCommand("merge-base", child, parent)
	if err != nil {
		return false, fmt.Errorf("failed to get merge base: %w", err)
	}

	parentCommit, err := repo.RunGitCommand("rev-parse", parent)
	if err != nil {
		return false, fmt.Errorf("failed to get parent commit: %w", err)
	}

	return mergeBase != parentCommit, nil
}

// isRebaseInProgress checks if a rebase is currently in progress
func isRebaseInProgress(repo *git.Repo) bool {
	gitDir := repo.GetGitDir()

	// Check for rebase-merge directory (interactive rebase)
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		return true
	}

	// Check for rebase-apply directory (regular rebase)
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		return true
	}

	return false
}
