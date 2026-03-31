package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/colors"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/spf13/cobra"
)

var (
	untrackForce bool
)

var untrackCmd = &cobra.Command{
	Use:   "untrack [branch]",
	Short: "Stop tracking a branch with gs",
	Long: `Stop tracking a branch with gs.

If no branch is specified, the current branch will be untracked.
Children branches will be reparented to this branch's parent.

Example:
  gs untrack              # Untrack current branch
  gs untrack feature-1    # Untrack specific branch
  gs untrack -f           # Force untrack without confirmation`,
	RunE: runUntrack,
}

func init() {
	rootCmd.AddCommand(untrackCmd)
	untrackCmd.Flags().BoolVarP(&untrackForce, "force", "f", false, "Skip confirmation prompt")
}

func runUntrack(cmd *cobra.Command, args []string) error {
	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Check if gs is initialized
	cfg, err := config.Load(repo.GetConfigPath())
	if err != nil {
		return err
	}

	// Determine which branch to untrack
	var branchToUntrack string
	if len(args) > 0 {
		branchToUntrack = args[0]
	} else {
		// Use current branch
		currentBranch, err := repo.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branchToUntrack = currentBranch
	}

	// Can't untrack trunk
	if branchToUntrack == cfg.Trunk {
		return fmt.Errorf("cannot untrack trunk branch '%s'", cfg.Trunk)
	}

	// Load metadata
	metadata, err := loadMetadata(repo)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Check if branch is tracked
	if !metadata.IsTracked(branchToUntrack) {
		fmt.Printf("%s is not tracked by gs\n", colors.BranchCurrent(branchToUntrack))
		return nil
	}

	// Get parent and children for reparenting
	parent, _ := metadata.GetParent(branchToUntrack)
	children := metadata.GetChildren(branchToUntrack)

	// Confirm if branch has children
	if len(children) > 0 && !untrackForce {
		fmt.Printf("%s has %d children that will be reparented to %s:\n",
			colors.BranchCurrent(branchToUntrack),
			len(children),
			colors.BranchParent(parent))

		for _, child := range children {
			fmt.Printf("  %s %s\n", colors.Muted("•"), colors.BranchChild(child))
		}
		fmt.Println()

		var confirm bool
		prompt := &survey.Confirm{
			Message: "Continue untracking?",
			Default: false,
		}

		if err := askOne(prompt, &confirm); err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				fmt.Println(colors.Muted("Cancelled."))
				return nil
			}
			return err
		}

		if !confirm {
			fmt.Println(colors.Muted("Cancelled."))
			return nil
		}
	}

	// Reparent children to this branch's parent
	for _, child := range children {
		metadata.TrackBranch(child, parent, "")
		fmt.Printf("%s Reparented %s to %s\n",
			colors.Success("✓"),
			colors.BranchChild(child),
			colors.BranchParent(parent))
	}

	// Untrack the branch
	metadata.UntrackBranch(branchToUntrack)

	// Save metadata
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Delete remote metadata ref and push updated children refs
	deleteRemoteMetadataRef(repo, branchToUntrack)
	if len(children) > 0 {
		pushMetadataRefs(repo, children...)
	}

	fmt.Printf("%s Untracked %s\n", colors.Success("✓"), colors.Muted(branchToUntrack))

	return nil
}
