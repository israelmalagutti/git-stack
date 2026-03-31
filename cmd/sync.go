package cmd

import (
	"fmt"
	"os"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	syncForce   bool
	syncRestack bool
	syncDelete  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync with remote and restack branches",
	Long: `Sync the repository with the remote and restack all branches.

This command:
1. Fetches from all remotes (git fetch --all --prune)
2. Syncs trunk with remote (fast-forward or reset)
3. Prompts to delete branches merged into trunk
4. Restacks all branches that can be rebased without conflicts

Example:
  gs sync              # Full sync with prompts
  gs sync -f           # Force sync without prompts
  gs sync --no-restack # Sync without restacking branches`,
	RunE: runSync,
}

func init() {
	syncCmd.Flags().BoolVarP(&syncForce, "force", "f", false, "Don't prompt for confirmation")
	syncCmd.Flags().BoolVarP(&syncRestack, "restack", "r", true, "Restack branches after syncing")
	syncCmd.Flags().BoolVarP(&syncDelete, "delete", "d", true, "Check for and delete merged branches")
	rootCmd.AddCommand(syncCmd)
}

func runSync(cmd *cobra.Command, args []string) error {
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

	// Abort if working tree is dirty
	dirty, err := repo.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check working tree: %w", err)
	}
	if dirty {
		return fmt.Errorf("you have uncommitted changes. Please commit or stash them before syncing")
	}

	// Save original branch to return to
	originalBranch, _ := repo.GetCurrentBranch()

	// 1. Fetch from remote
	fmt.Println("Fetching from remote...")
	if err := repo.Fetch(); err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}
	fmt.Println("✓ Fetched from origin")

	// 2. Sync trunk with remote
	fmt.Printf("\nSyncing trunk (%s)...\n", cfg.Trunk)
	if err := syncTrunkWithRemote(repo, cfg.Trunk, syncForce); err != nil {
		return err
	}

	// 3. Clean up stale branches from metadata
	if err := cleanStaleBranches(repo, metadata, cfg, syncForce); err != nil {
		return err
	}

	// 4. Clean up branches whose remote was deleted (merged/deleted upstream)
	if syncDelete {
		if err := cleanRemoteDeletedBranches(repo, metadata, cfg, syncForce); err != nil {
			return err
		}
	}

	// 5. Find and prompt to delete merged branches
	if syncDelete {
		if err := deleteMergedBranches(repo, metadata, cfg.Trunk, syncForce); err != nil {
			return err
		}
	}

	// 6. Auto-restack all branches without conflicts
	if syncRestack {
		// Rebuild stack after potential deletions
		s, err := stack.BuildStack(repo, cfg, metadata)
		if err != nil {
			return fmt.Errorf("failed to build stack: %w", err)
		}

		fmt.Println("\nRestacking branches...")
		succeeded, failed := restackAllBranches(repo, s, metadata)

		// Report results
		if len(succeeded) > 0 || len(failed) > 0 {
			fmt.Println()
		}

		if len(succeeded) > 0 {
			fmt.Printf("✓ %d branch(es) restacked\n", len(succeeded))
		}

		if len(failed) > 0 {
			fmt.Printf("✗ %d branch(es) have conflicts:\n", len(failed))
			for _, branch := range failed {
				fmt.Printf("    %s\n", branch)
			}
			fmt.Println("\nRun 'gs restack' on each branch to resolve conflicts.")
		}

		if len(succeeded) == 0 && len(failed) == 0 {
			fmt.Println("✓ All branches are up to date")
		}
	}

	// Return to original branch if possible
	if originalBranch != "" && repo.BranchExists(originalBranch) {
		currentBranch, _ := repo.GetCurrentBranch()
		if currentBranch != originalBranch {
			_ = repo.CheckoutBranch(originalBranch)
		}
	}

	// Push all metadata refs to remote after sync
	pushMetadataRefs(repo)

	fmt.Println("\nSync complete.")
	return nil
}

// syncTrunkWithRemote syncs the trunk branch with its remote
func syncTrunkWithRemote(repo *git.Repo, trunk string, force bool) error {
	remote := "origin/" + trunk

	// Check if remote branch exists
	if !repo.HasRemoteBranch(trunk, "origin") {
		fmt.Printf("✓ %s has no remote tracking branch\n", trunk)
		return nil
	}

	// Check if local and remote are the same
	localCommit, err := repo.GetBranchCommit(trunk)
	if err != nil {
		return err
	}

	remoteCommit, err := repo.GetBranchCommit(remote)
	if err != nil {
		return err
	}

	if localCommit == remoteCommit {
		fmt.Printf("✓ %s is up to date with %s\n", trunk, remote)
		return nil
	}

	// Check if we can fast-forward
	canFF, err := repo.CanFastForward(trunk, remote)
	if err != nil {
		return err
	}

	// Save current branch
	currentBranch, _ := repo.GetCurrentBranch()

	if canFF {
		// Fast-forward
		if currentBranch != trunk {
			if err := repo.CheckoutBranch(trunk); err != nil {
				return err
			}
		}

		_, err := repo.RunGitCommand("merge", "--ff-only", remote)
		if err != nil {
			return fmt.Errorf("failed to fast-forward: %w", err)
		}

		fmt.Printf("✓ Fast-forwarded %s to %s\n", trunk, remote)

		// Return to original branch
		if currentBranch != trunk && currentBranch != "" {
			_ = repo.CheckoutBranch(currentBranch)
		}
	} else {
		// Can't fast-forward - need to reset
		if !force {
			fmt.Printf("Cannot fast-forward %s (local has diverged).\n", trunk)
			fmt.Printf("Reset %s to %s? [y/N]: ", trunk, remote)
			if !confirm() {
				fmt.Println("Skipped trunk sync.")
				return nil
			}
		}

		if err := repo.ResetToRemote(trunk, remote); err != nil {
			return err
		}
		fmt.Printf("✓ Reset %s to %s\n", trunk, remote)
	}

	return nil
}

// cleanStaleBranches removes branches from metadata that no longer exist in git,
// reparenting their children to the nearest living ancestor first
func cleanStaleBranches(repo *git.Repo, metadata *config.Metadata, cfg *config.Config, force bool) error {
	var staleBranches []string

	for branch := range metadata.Branches {
		if !repo.BranchExists(branch) {
			staleBranches = append(staleBranches, branch)
		}
	}

	if len(staleBranches) == 0 {
		return nil
	}

	fmt.Printf("\nFound %d stale branch(es) in metadata:\n", len(staleBranches))
	for _, branch := range staleBranches {
		fmt.Printf("  - %s (deleted from git)\n", branch)
	}

	if !force {
		fmt.Print("Remove from metadata? [y/N]: ")
		if !confirm() {
			return nil
		}
	}

	// Build stale set for O(1) lookup
	staleSet := make(map[string]bool)
	for _, b := range staleBranches {
		staleSet[b] = true
	}

	// Reparent living children of stale branches to nearest living ancestor
	for branch := range metadata.Branches {
		if staleSet[branch] {
			continue
		}
		parent, ok := metadata.GetParent(branch)
		if !ok || !staleSet[parent] {
			continue
		}

		// Walk up to find nearest non-stale ancestor
		newParent := parent
		visited := make(map[string]bool)
		for staleSet[newParent] {
			visited[newParent] = true
			p, ok := metadata.GetParent(newParent)
			if !ok || p == "" || visited[p] {
				newParent = cfg.Trunk
				break
			}
			newParent = p
		}

		if err := metadata.UpdateParent(branch, newParent); err == nil {
			fmt.Printf("  Reparented %s → %s\n", branch, newParent)
		}
	}

	// Untrack all stale branches and delete their remote refs
	for _, branch := range staleBranches {
		metadata.UntrackBranch(branch)
		deleteRemoteMetadataRef(repo, branch)
	}

	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}

	fmt.Printf("✓ Removed %d stale branch(es) from metadata\n", len(staleBranches))
	return nil
}

// cleanRemoteDeletedBranches detects tracked branches whose remote counterpart
// was deleted (e.g., merged and deleted on GitHub) and offers to clean them up.
// After git fetch --prune, origin/<branch> is gone but the local branch remains.
func cleanRemoteDeletedBranches(repo *git.Repo, metadata *config.Metadata, cfg *config.Config, force bool) error {
	if !repoHasRemote(repo) {
		return nil
	}

	var remoteGone []string
	for branch := range metadata.Branches {
		if branch == cfg.Trunk {
			continue
		}
		if !repo.BranchExists(branch) {
			continue // already handled by cleanStaleBranches
		}
		if !repo.HasRemoteBranch(branch, "origin") {
			remoteGone = append(remoteGone, branch)
		}
	}

	if len(remoteGone) == 0 {
		return nil
	}

	fmt.Printf("\nFound %d branch(es) with no remote (deleted upstream):\n", len(remoteGone))
	for _, branch := range remoteGone {
		fmt.Printf("  - %s\n", branch)
	}

	if force {
		fmt.Println()
		for _, branch := range remoteGone {
			if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
				fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
			} else {
				fmt.Printf("  ✓ Deleted %s\n", branch)
			}
		}
	} else {
		fmt.Println()
		deleteAll := false
		for _, branch := range remoteGone {
			if deleteAll {
				if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
					fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
				} else {
					fmt.Printf("  ✓ Deleted %s\n", branch)
				}
				continue
			}

			fmt.Printf("Delete '%s'? [y/n/a(ll)/q(uit)]: ", branch)
			action := confirmWithOptions()

			switch action {
			case "yes":
				if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
					fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
				} else {
					fmt.Printf("  ✓ Deleted %s\n", branch)
				}
			case "all":
				deleteAll = true
				if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
					fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
				} else {
					fmt.Printf("  ✓ Deleted %s\n", branch)
				}
			case "quit":
				return nil
			}
		}
	}

	return nil
}

// deleteMergedBranches finds branches merged into trunk and prompts to delete them
func deleteMergedBranches(repo *git.Repo, metadata *config.Metadata, trunk string, force bool) error {
	var mergedBranches []string

	for branch := range metadata.Branches {
		if branch == trunk {
			continue
		}

		isMerged, err := repo.IsMergedInto(branch, trunk)
		if err != nil {
			continue
		}

		if isMerged {
			mergedBranches = append(mergedBranches, branch)
		}
	}

	if len(mergedBranches) == 0 {
		return nil
	}

	fmt.Printf("\nFound %d branch(es) merged into %s:\n", len(mergedBranches), trunk)

	for _, branch := range mergedBranches {
		fmt.Printf("  - %s\n", branch)
	}

	if force {
		// Delete all without prompting
		fmt.Println()
		for _, branch := range mergedBranches {
			if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
				fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
			} else {
				fmt.Printf("  ✓ Deleted %s\n", branch)
			}
		}
	} else {
		// Prompt for each branch with all/none options
		fmt.Println()
		deleteAll := false
		for _, branch := range mergedBranches {
			if deleteAll {
				if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
					fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
				} else {
					fmt.Printf("  ✓ Deleted %s\n", branch)
				}
				continue
			}

			fmt.Printf("Delete '%s'? [y/n/a(ll)/q(uit)]: ", branch)
			action := confirmWithOptions()

			switch action {
			case "yes":
				if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
					fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
				} else {
					fmt.Printf("  ✓ Deleted %s\n", branch)
				}
			case "all":
				deleteAll = true
				if err := deleteBranchAndCleanup(repo, metadata, branch); err != nil {
					fmt.Printf("  ✗ Failed to delete %s: %v\n", branch, err)
				} else {
					fmt.Printf("  ✓ Deleted %s\n", branch)
				}
			case "quit":
				return nil
			}
		}
	}

	return nil
}

// deleteBranchAndCleanup deletes a branch and updates metadata
func deleteBranchAndCleanup(repo *git.Repo, metadata *config.Metadata, branch string) error {
	// Update children to point to deleted branch's parent
	parent, _ := metadata.GetParent(branch)
	children := metadata.GetChildren(branch)

	for _, child := range children {
		if parent != "" {
			if err := metadata.UpdateParent(child, parent); err != nil {
				return fmt.Errorf("failed to update parent for '%s': %w", child, err)
			}
		}
	}

	// Delete git branch
	if err := repo.DeleteBranch(branch, true); err != nil {
		return err
	}

	// Remove from metadata
	metadata.UntrackBranch(branch)

	// Delete remote metadata ref
	deleteRemoteMetadataRef(repo, branch)

	// Save metadata
	return metadata.SaveWithRefs(repo, repo.GetMetadataPath())
}

// restackAllBranches restacks all branches in topological order, skipping those with conflicts
func restackAllBranches(repo *git.Repo, s *stack.Stack, metadata *config.Metadata) (succeeded, failed []string) {
	branches := s.GetTopologicalOrder()

	for _, node := range branches {
		if node.Parent == nil {
			continue
		}

		// Check if needs rebase
		needsRebase, err := repo.IsBehind(node.Name, node.Parent.Name)
		if err != nil {
			continue
		}

		if !needsRebase {
			continue
		}

		fmt.Printf("  Rebasing %s onto %s...", node.Name, node.Parent.Name)

		// Try rebase (precise --onto or fallback)
		err = restackBranchOnto(repo, metadata, node.Name, node.Parent.Name)
		if err != nil {
			// Abort and record failure
			_ = repo.AbortRebase()
			failed = append(failed, node.Name)
			fmt.Println(" ✗ conflict")
		} else {
			// Update ParentRevision after successful rebase
			parentSHA, _ := repo.GetBranchCommit(node.Parent.Name)
			if parentSHA != "" {
				_ = metadata.SetParentRevision(node.Name, parentSHA)
			}
			succeeded = append(succeeded, node.Name)
			fmt.Println(" ✓")
		}
	}

	// Save metadata if any rebases succeeded
	if len(succeeded) > 0 {
		_ = metadata.SaveWithRefs(repo, repo.GetMetadataPath())
	}

	return succeeded, failed
}

// readKeyFn reads a single keypress from stdin. Swappable for testing.
var readKeyFn = readKey

func readKey() (byte, error) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		// Fallback for non-terminal (e.g. pipes in tests)
		b := make([]byte, 1)
		if _, err := os.Stdin.Read(b); err != nil {
			return 0, err
		}
		return b[0], nil
	}
	defer func() { _ = term.Restore(fd, oldState) }()

	b := make([]byte, 1)
	if _, err := os.Stdin.Read(b); err != nil {
		return 0, err
	}
	return b[0], nil
}

// confirm reads a single y/n keypress from stdin (no Enter required)
func confirm() bool {
	b, err := readKeyFn()
	fmt.Println() // move past the prompt line
	if err != nil {
		return false
	}
	return b == 'y' || b == 'Y'
}

// confirmWithOptions reads a single y/n/a/q keypress from stdin (no Enter required)
// Returns: "yes", "no", "all", or "quit"
func confirmWithOptions() string {
	b, err := readKeyFn()
	fmt.Println() // move past the prompt line
	if err != nil {
		return "no"
	}
	switch b {
	case 'y', 'Y':
		return "yes"
	case 'a', 'A':
		return "all"
	case 'q', 'Q':
		return "quit"
	default:
		return "no"
	}
}
