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

var trackCmd = &cobra.Command{
	Use:   "track [branch]",
	Short: "Start tracking a branch with gs",
	Long: `Start tracking an existing branch with gs by selecting its parent branch.

If no branch is specified, the current branch will be tracked.
You'll be prompted to select which branch is the parent of this branch.

Example:
  gs track              # Track current branch
  gs track feature-1    # Track specific branch`,
	RunE: runTrack,
}

func init() {
	rootCmd.AddCommand(trackCmd)
}

func runTrack(cmd *cobra.Command, args []string) error {
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

	// Determine which branch to track
	var branchToTrack string
	if len(args) > 0 {
		branchToTrack = args[0]
	} else {
		// Use current branch
		currentBranch, err := repo.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}
		branchToTrack = currentBranch
	}

	// Verify the branch exists
	if !repo.BranchExists(branchToTrack) {
		return fmt.Errorf("branch '%s' does not exist", branchToTrack)
	}

	// Load metadata
	metadata, err := config.LoadMetadata(repo.GetMetadataPath())
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}

	// Check if already tracked
	if metadata.IsTracked(branchToTrack) {
		parent, _ := metadata.GetParent(branchToTrack)
		return fmt.Errorf("branch '%s' is already tracked with parent '%s'", branchToTrack, parent)
	}

	// Get list of branches for parent selection
	branches, err := repo.ListBranches()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	// Remove the branch being tracked from the list (can't be its own parent)
	parentOptions := []string{}
	for _, b := range branches {
		if b != branchToTrack {
			parentOptions = append(parentOptions, b)
		}
	}

	if len(parentOptions) == 0 {
		return fmt.Errorf("no other branches available to select as parent")
	}

	// Prompt user to select parent branch
	var parent string
	prompt := &survey.Select{
		Message: fmt.Sprintf("Select parent branch for '%s':", branchToTrack),
		Options: parentOptions,
		Description: func(value string, index int) string {
			if value == cfg.Trunk {
				return "(trunk)"
			}
			if metadata.IsTracked(value) {
				// Show if this branch has a parent
				if p, ok := metadata.GetParent(value); ok {
					return fmt.Sprintf("(parent: %s)", p)
				}
			}
			return ""
		},
	}

	// Set trunk as default if it exists in options
	for i, opt := range parentOptions {
		if opt == cfg.Trunk {
			prompt.Default = i
			break
		}
	}

	err = askOne(prompt, &parent, survey.WithValidator(survey.Required))
	if err != nil {
		// Handle ESC/Ctrl+C gracefully
		if errors.Is(err, terminal.InterruptErr) {
			fmt.Println("Cancelled.")
			return nil
		}
		return fmt.Errorf("failed to get parent selection: %w", err)
	}

	// Track the branch
	parentSHA, _ := repo.GetBranchCommit(parent)
	metadata.TrackBranch(branchToTrack, parent, parentSHA)

	// Save metadata
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	colors.PrintTracked(branchToTrack, parent)

	// Show children if any
	children := metadata.GetChildren(branchToTrack)
	if len(children) > 0 {
		fmt.Printf("\n%s %s is the parent of:\n", colors.Info("ℹ"), colors.BranchCurrent(branchToTrack))
		for _, child := range children {
			fmt.Printf("  - %s\n", colors.BranchChild(child))
		}
	}

	return nil
}
