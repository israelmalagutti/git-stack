package cmd

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
)

var (
	deleteForce bool
)

var deleteCmd = &cobra.Command{
	Use:   "delete [branch]",
	Short: "Delete a branch from the stack",
	Long: `Delete a branch and its metadata. Children will be restacked onto the parent.

If no branch is specified, deletes the current branch.
Prompts for confirmation unless --force is used.

Example:
  gs delete feat-old       # Delete feat-old branch
  gs delete                # Delete current branch (interactive)
  gs delete -f feat-old    # Delete without confirmation`,
	Aliases: []string{"d", "remove", "rm"},
	RunE:    runDelete,
}

func init() {
	deleteCmd.Flags().BoolVarP(&deleteForce, "force", "f", false, "Delete without confirmation")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	rs, err := loadRepoConfig()
	if err != nil {
		return err
	}

	repo, cfg, metadata := rs.Repo, rs.Config, rs.Metadata

	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Determine which branch to delete
	branchToDelete := ""
	if len(args) > 0 {
		branchToDelete = args[0]
	} else {
		// Interactive selection
		s, err := stack.BuildStack(repo, cfg, metadata)
		if err != nil {
			return fmt.Errorf("failed to build stack: %w", err)
		}

		// Get all tracked branches except trunk
		options := []string{}
		optionsMap := make(map[string]string)

		for _, node := range s.Nodes {
			if node.Name == cfg.Trunk {
				continue
			}

			parent := metadata.Branches[node.Name].Parent
			context := fmt.Sprintf("%s (parent: %s)", node.Name, parent)
			if node.IsCurrent {
				context = fmt.Sprintf("%s (current, parent: %s)", node.Name, parent)
			}
			options = append(options, context)
			optionsMap[context] = node.Name
		}

		if len(options) == 0 {
			return fmt.Errorf("no branches available to delete")
		}

		prompt := &survey.Select{
			Message: "Select branch to delete:",
			Options: options,
		}

		var selected string
		if err := askOne(prompt, &selected); err != nil {
			return fmt.Errorf("selection cancelled: %w", err)
		}

		// Map back to branch name
		if mapped, ok := optionsMap[selected]; ok {
			branchToDelete = mapped
		} else {
			branchToDelete = selected
		}
	}

	// Validate branch
	if branchToDelete == "" {
		return fmt.Errorf("no branch specified")
	}

	if err := validateNotTrunkAndTracked(metadata, branchToDelete, cfg.Trunk, "delete"); err != nil {
		return err
	}

	if !repo.BranchExists(branchToDelete) {
		return fmt.Errorf("branch '%s' does not exist", branchToDelete)
	}

	// Build stack to get parent and children
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	deleteNode := s.GetNode(branchToDelete)
	if deleteNode == nil {
		return fmt.Errorf("branch not found in stack")
	}

	parentBranch, ok := metadata.GetParent(branchToDelete)
	if !ok {
		return fmt.Errorf("branch has no parent")
	}

	// Confirm with user (unless --force)
	if !deleteForce {
		message := fmt.Sprintf("Delete branch '%s'?", branchToDelete)
		if len(deleteNode.Children) > 0 {
			message = fmt.Sprintf("Delete branch '%s' and restack %d child branch(es) onto '%s'?",
				branchToDelete, len(deleteNode.Children), parentBranch)
		}

		confirm := false
		prompt := &survey.Confirm{
			Message: message,
			Default: false,
		}
		if err := askOne(prompt, &confirm); err != nil {
			return fmt.Errorf("confirmation cancelled: %w", err)
		}

		if !confirm {
			fmt.Println("Delete cancelled")
			return nil
		}
	}

	// If deleting current branch, checkout parent first
	needToCheckout := (branchToDelete == currentBranch)
	if needToCheckout {
		fmt.Printf("Checking out '%s'...\n", parentBranch)
		if err := repo.CheckoutBranch(parentBranch); err != nil {
			return fmt.Errorf("failed to checkout parent: %w", err)
		}
	}

	// Update children to point to parent
	if len(deleteNode.Children) > 0 {
		fmt.Printf("\nUpdating %d child branch(es)...\n", len(deleteNode.Children))
		for _, child := range deleteNode.Children {
			if err := metadata.UpdateParent(child.Name, parentBranch); err != nil {
				return fmt.Errorf("failed to update child '%s': %w", child.Name, err)
			}
			fmt.Printf("  ✓ Updated '%s' parent to '%s'\n", child.Name, parentBranch)
		}

		if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
			return fmt.Errorf("failed to save metadata: %w", err)
		}
	}

	// Delete the branch
	fmt.Printf("\nDeleting branch '%s'...\n", branchToDelete)
	if _, err := repo.RunGitCommand("branch", "-D", branchToDelete); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	// Remove from metadata
	metadata.UntrackBranch(branchToDelete)
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	// Delete remote metadata ref and push updated children refs
	deleteRemoteMetadataRef(repo, branchToDelete)
	if len(deleteNode.Children) > 0 {
		childNames := make([]string, len(deleteNode.Children))
		for i, c := range deleteNode.Children {
			childNames[i] = c.Name
		}
		pushMetadataRefs(repo, childNames...)
	}

	fmt.Printf("✓ Deleted branch '%s'\n", branchToDelete)

	// Rebuild stack and restack children
	if len(deleteNode.Children) > 0 {
		s, err := stack.BuildStack(repo, cfg, metadata)
		if err != nil {
			return fmt.Errorf("failed to rebuild stack: %w", err)
		}

		parentNode := s.GetNode(parentBranch)
		if parentNode != nil && len(parentNode.Children) > 0 {
			fmt.Println("\nRestacking children...")
			if err := restackChildren(repo, s, parentNode); err != nil {
				return fmt.Errorf("failed to restack children: %w", err)
			}
			fmt.Println("✓ Children restacked")
		}
	}

	return nil
}
