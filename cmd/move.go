package cmd

import (
	"errors"
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	moveOnto   string
	moveSource string
)

var moveCmd = &cobra.Command{
	Use:   "move [target]",
	Short: "Rebase a branch onto a different parent",
	Long: `Move a branch to a different parent by rebasing it onto the target branch.
Automatically restacks all descendants.

If no target is specified, opens an interactive selector to choose the new parent.
If no source is specified, moves the current branch.

Example:
  gs move feat-base                    # Move current branch onto feat-base
  gs move                              # Interactive selection
  gs move -o feat-base                 # Using --onto flag
  gs move -t feat-base                 # Using --target flag (alias for --onto)
  gs move -s feat-2 -o main            # Move feat-2 onto main
  gs mv --source feat-3 feat-1         # Move feat-3 onto feat-1`,
	Aliases: []string{"mv"},
	RunE:    runMove,
}

func init() {
	moveCmd.Flags().StringVarP(&moveOnto, "onto", "o", "", "Branch to move onto")
	moveCmd.Flags().StringVarP(&moveOnto, "target", "t", "", "Branch to move onto (alias for --onto)")
	moveCmd.Flags().StringVarP(&moveSource, "source", "s", "", "Branch to move (defaults to current branch)")
	rootCmd.AddCommand(moveCmd)
}

func runMove(cmd *cobra.Command, args []string) error {
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

	// Determine source branch (branch to move)
	sourceBranch := moveSource
	if sourceBranch == "" {
		sourceBranch = currentBranch
	}

	if err := validateNotTrunkAndTracked(metadata, sourceBranch, cfg.Trunk, "move"); err != nil {
		return err
	}

	// Determine target branch
	targetBranch := moveOnto
	if len(args) > 0 {
		targetBranch = args[0]
	}

	// If no target specified, show interactive selector
	if targetBranch == "" {
		s, err := stack.BuildStack(repo, cfg, metadata)
		if err != nil {
			return fmt.Errorf("failed to build stack: %w", err)
		}

		// Get all branches except source
		options := []string{}
		optionsMap := make(map[string]string)

		// Add trunk
		options = append(options, cfg.Trunk)
		optionsMap[cfg.Trunk] = cfg.Trunk

		// Add all tracked branches except source
		for _, node := range s.Nodes {
			if node.Name != sourceBranch {
				// Get context for display
				parent, ok := metadata.GetParent(node.Name)
				if !ok {
					continue
				}
				context := fmt.Sprintf("%s (parent: %s)", node.Name, parent)
				options = append(options, context)
				optionsMap[context] = node.Name
			}
		}

		if len(options) == 0 {
			return fmt.Errorf("no other branches available to move onto")
		}

		prompt := &survey.Select{
			Message: "Select target branch to move onto:",
			Options: options,
		}

		var selected string
		if err := askOne(prompt, &selected); err != nil {
			if errors.Is(err, terminal.InterruptErr) {
				fmt.Println("Cancelled.")
				return nil
			}
			return fmt.Errorf("selection cancelled: %w", err)
		}

		// Map back to branch name
		if mapped, ok := optionsMap[selected]; ok {
			targetBranch = mapped
		} else {
			targetBranch = selected
		}
	}

	// Validate target branch
	if targetBranch == sourceBranch {
		return fmt.Errorf("cannot move branch onto itself")
	}

	if !repo.BranchExists(targetBranch) {
		return fmt.Errorf("target branch '%s' does not exist", targetBranch)
	}

	// Check if target is a descendant of source branch
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	sourceNode := s.GetNode(sourceBranch)
	if sourceNode == nil {
		return fmt.Errorf("branch '%s' not found in stack", sourceBranch)
	}

	if isDescendant(sourceNode, targetBranch) {
		return fmt.Errorf("cannot move branch onto its descendant '%s'", targetBranch)
	}

	// Get old parent
	oldParent, _ := metadata.GetParent(sourceBranch)

	fmt.Printf("Moving '%s' from '%s' to '%s'...\n", sourceBranch, oldParent, targetBranch)

	// Update metadata with new parent
	if err := metadata.UpdateParent(sourceBranch, targetBranch); err != nil {
		return fmt.Errorf("failed to update metadata: %w", err)
	}

	// Save metadata
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// If source is not current branch, checkout source first
	needsCheckoutBack := false
	if sourceBranch != currentBranch {
		needsCheckoutBack = true
		if err := repo.CheckoutBranch(sourceBranch); err != nil {
			// Restore metadata
			if restoreErr := metadata.UpdateParent(sourceBranch, oldParent); restoreErr != nil {
				return fmt.Errorf("failed to restore metadata after checkout error: %v (original error: %w)", restoreErr, err)
			}
			if saveErr := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); saveErr != nil {
				return fmt.Errorf("failed to save restored metadata after checkout error: %v (original error: %w)", saveErr, err)
			}
			return fmt.Errorf("failed to checkout '%s': %w", sourceBranch, err)
		}
	}

	// Rebase onto new parent
	fmt.Printf("Rebasing onto '%s'...\n", targetBranch)
	if _, err := repo.RunGitCommand("rebase", targetBranch); err != nil {
		// Rebase failed, restore old parent
		if restoreErr := metadata.UpdateParent(sourceBranch, oldParent); restoreErr != nil {
			return fmt.Errorf("rebase failed: %w\nFailed to restore metadata: %v", err, restoreErr)
		}
		if saveErr := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); saveErr != nil {
			return fmt.Errorf("rebase failed: %w\nFailed to save restored metadata: %v", err, saveErr)
		}

		// Try to go back to original branch
		if needsCheckoutBack {
			if checkoutErr := repo.CheckoutBranch(currentBranch); checkoutErr != nil {
				return fmt.Errorf("rebase failed: %w\nAlso failed to return to '%s': %v", err, currentBranch, checkoutErr)
			}
		}

		return fmt.Errorf("rebase failed: %w\nMetadata restored to original state", err)
	}

	fmt.Println("✓ Rebased successfully")

	// Rebuild stack with new structure
	s, err = stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to rebuild stack: %w", err)
	}

	sourceNode = s.GetNode(sourceBranch)
	if sourceNode == nil {
		return fmt.Errorf("branch '%s' not found in stack", sourceBranch)
	}

	// Restack children if any
	if len(sourceNode.Children) > 0 {
		fmt.Println("\nRestacking children...")
		if err := restackChildren(repo, s, sourceNode); err != nil {
			return fmt.Errorf("failed to restack children: %w", err)
		}
		fmt.Println("✓ Children restacked")
	}

	// Go back to original branch if needed
	if needsCheckoutBack {
		if err := repo.CheckoutBranch(currentBranch); err != nil {
			fmt.Printf("Warning: could not return to '%s': %v\n", currentBranch, err)
		}
	}

	// Push updated metadata refs to remote
	pushMetadataRefs(repo, sourceBranch)

	fmt.Printf("\n✓ Moved '%s' onto '%s'\n", sourceBranch, targetBranch)

	return nil
}

// isDescendant checks if targetBranch is a descendant of node
func isDescendant(node *stack.Node, targetBranch string) bool {
	for _, child := range node.Children {
		if child.Name == targetBranch {
			return true
		}
		if isDescendant(child, targetBranch) {
			return true
		}
	}
	return false
}
