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

var renameCmd = &cobra.Command{
	Use:   "rename [new-name]",
	Short: "Rename the current branch",
	Long: `Rename the current branch and update gs tracking.

If no new name is provided, you'll be prompted to enter one.
This updates both the git branch name and gs metadata.

Example:
  gs rename feat-new-name    # Rename current branch
  gs rename                  # Prompt for new name`,
	RunE: runRename,
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

func runRename(cmd *cobra.Command, args []string) error {
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

	// Get current branch
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Can't rename trunk
	if currentBranch == cfg.Trunk {
		return fmt.Errorf("cannot rename trunk branch '%s'", cfg.Trunk)
	}

	// Determine new name
	var newName string
	if len(args) > 0 {
		newName = args[0]
	} else {
		// Prompt for new name
		prompt := &survey.Input{
			Message: fmt.Sprintf("New name for '%s':", currentBranch),
		}

		if err := askOne(prompt, &newName, survey.WithValidator(survey.Required)); err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				fmt.Println(colors.Muted("Cancelled."))
				return nil
			}
			return err
		}
	}

	// Validate new name
	if newName == "" {
		return fmt.Errorf("branch name cannot be empty")
	}

	if newName == currentBranch {
		fmt.Printf("%s is already named %s\n", colors.BranchCurrent(currentBranch), colors.Muted(newName))
		return nil
	}

	// Check if new name already exists
	if repo.BranchExists(newName) {
		return fmt.Errorf("branch '%s' already exists", newName)
	}

	// Load metadata
	metadata, err := loadMetadata(repo)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Rename git branch
	if _, err := repo.RunGitCommand("branch", "-m", currentBranch, newName); err != nil {
		return fmt.Errorf("failed to rename branch: %w", err)
	}

	// Update metadata if branch is tracked
	if metadata.IsTracked(currentBranch) {
		parent, _ := metadata.GetParent(currentBranch)
		children := metadata.GetChildren(currentBranch)
		existingParentRev := metadata.GetParentRevision(currentBranch)

		// Untrack old name
		metadata.UntrackBranch(currentBranch)

		// Track with new name (preserve existing parentRevision)
		metadata.TrackBranch(newName, parent, existingParentRev)

		// Update children to point to new parent name
		for _, child := range children {
			childParent, _ := metadata.GetParent(child)
			if childParent == currentBranch {
				childParentRev := metadata.GetParentRevision(child)
				metadata.TrackBranch(child, newName, childParentRev)
			}
		}

		// Save metadata
		if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
			// Try to rollback git rename
			_, _ = repo.RunGitCommand("branch", "-m", newName, currentBranch)
			return fmt.Errorf("failed to save metadata: %w", err)
		}
	}

	fmt.Printf("%s Renamed %s to %s\n",
		colors.Success("✓"),
		colors.Muted(currentBranch),
		colors.BranchCurrent(newName))

	return nil
}
