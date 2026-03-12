package cmd

import (
	"fmt"

	"github.com/israelmalagutti/git-wrapper/internal/colors"
	"github.com/israelmalagutti/git-wrapper/internal/config"
	"github.com/israelmalagutti/git-wrapper/internal/git"
	"github.com/israelmalagutti/git-wrapper/internal/stack"
	"github.com/spf13/cobra"
)

var (
	restackOnly       bool
	restackUpstack    bool
	restackDownstack  bool
	restackBranchFlag string
)

var stackRestackCmd = &cobra.Command{
	Use:     "restack",
	Aliases: []string{"r", "fix", "f"},
	Short:   "Rebase stack to maintain parent-child relationships",
	Long: `Ensure each branch in the current stack is based on its parent, rebasing if necessary.

This command:
- Checks if branches need rebasing onto their parents
- Performs rebases using precise --onto when parent revision is known
- Saves state for gw continue if a conflict occurs

Scope flags (mutually exclusive):
  --only        Restack only the current branch
  --upstack     Restack current branch + descendants (default when on a non-trunk branch)
  --downstack   Restack ancestors + current branch
  --branch      Start from a specific branch instead of current

Default (no flags): restack the full stack (ancestors + current + descendants).

Example:
  gw stack restack              # Restack full stack
  gw stack restack --only       # Restack only current branch
  gw stack restack --upstack    # Restack current + descendants
  gw stack restack --downstack  # Restack ancestors + current
  gw stack restack --branch X   # Restack starting from branch X`,
	RunE: runStackRestack,
}

func init() {
	stackCmd.AddCommand(stackRestackCmd)
	registerRestackFlags(stackRestackCmd)
}

func registerRestackFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&restackOnly, "only", false, "Restack only the target branch")
	cmd.Flags().BoolVar(&restackUpstack, "upstack", false, "Restack target branch + descendants")
	cmd.Flags().BoolVar(&restackDownstack, "downstack", false, "Restack ancestors + target branch")
	cmd.Flags().StringVar(&restackBranchFlag, "branch", "", "Start from a specific branch")
}

func runStackRestack(cmd *cobra.Command, args []string) error {
	// Initialize repository
	repo, err := git.NewRepo()
	if err != nil {
		return fmt.Errorf("failed to initialize repository: %w", err)
	}

	// Dirty tree check
	dirty, err := repo.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check working tree: %w", err)
	}
	if dirty {
		return fmt.Errorf("you have uncommitted changes. Please commit or stash them before restacking")
	}

	// Validate mutually exclusive flags
	exclusiveCount := 0
	if restackOnly {
		exclusiveCount++
	}
	if restackUpstack {
		exclusiveCount++
	}
	if restackDownstack {
		exclusiveCount++
	}
	if exclusiveCount > 1 {
		return fmt.Errorf("--only, --upstack, and --downstack are mutually exclusive")
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

	// Get current branch
	currentBranch, err := repo.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	// Build stack
	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		return fmt.Errorf("failed to build stack: %w", err)
	}

	// Resolve start branch
	startBranch := currentBranch
	if restackBranchFlag != "" {
		startBranch = restackBranchFlag
	}

	// Validate that the branch is in the stack
	if startBranch != cfg.Trunk && s.GetNode(startBranch) == nil {
		return fmt.Errorf("branch '%s' is not tracked by gw", startBranch)
	}

	// Compute branches to restack
	branches, err := computeRestackBranches(s, cfg, startBranch)
	if err != nil {
		return err
	}

	if len(branches) == 0 {
		fmt.Println("No branches to restack.")
		return nil
	}

	// Run the linear restack loop
	return runLinearRestack(repo, metadata, s, branches, currentBranch)
}

// computeRestackBranches returns an ordered list of branches to restack based on scope flags.
// The order is topological: parents before children.
func computeRestackBranches(s *stack.Stack, cfg *config.Config, startBranch string) ([]string, error) {
	if restackOnly {
		if startBranch == cfg.Trunk {
			return nil, fmt.Errorf("cannot restack trunk with --only")
		}
		return []string{startBranch}, nil
	}

	if restackUpstack {
		if startBranch == cfg.Trunk {
			// Upstack from trunk = all branches
			return topologicalAll(s), nil
		}
		result := []string{startBranch}
		result = append(result, descendantsDFS(s, startBranch)...)
		return result, nil
	}

	if restackDownstack {
		if startBranch == cfg.Trunk {
			return nil, fmt.Errorf("cannot restack downstack from trunk")
		}
		return ancestorsOf(s, cfg, startBranch), nil
	}

	// Default: full stack
	if startBranch == cfg.Trunk {
		return topologicalAll(s), nil
	}

	// Ancestors (skip trunk) + startBranch + descendants
	ancestors := ancestorsOf(s, cfg, startBranch)
	descendants := descendantsDFS(s, startBranch)
	return append(ancestors, descendants...), nil
}

// topologicalAll returns all non-trunk branches in topological order
func topologicalAll(s *stack.Stack) []string {
	nodes := s.GetTopologicalOrder()
	result := make([]string, len(nodes))
	for i, n := range nodes {
		result[i] = n.Name
	}
	return result
}

// ancestorsOf returns ancestors of branch (excluding trunk), in topological order (parent before child)
func ancestorsOf(s *stack.Stack, cfg *config.Config, branch string) []string {
	path := s.FindPath(branch)
	var result []string
	for _, n := range path {
		if n.Name == cfg.Trunk {
			continue
		}
		result = append(result, n.Name)
	}
	return result
}

// descendantsDFS returns all descendants of branch in DFS order (sorted children)
func descendantsDFS(s *stack.Stack, branch string) []string {
	node := s.GetNode(branch)
	if node == nil {
		return nil
	}

	var result []string
	for _, child := range node.SortedChildren() {
		result = append(result, child.Name)
		result = append(result, descendantsDFS(s, child.Name)...)
	}
	return result
}

// runLinearRestack iterates through branches, rebasing each one.
// On conflict, saves state for gw continue.
func runLinearRestack(repo *git.Repo, metadata *config.Metadata, s *stack.Stack, branches []string, originalBranch string) error {
	for i, branch := range branches {
		node := s.GetNode(branch)
		if node == nil || node.Parent == nil {
			continue
		}

		parent := node.Parent.Name

		// Checkout the branch
		if err := repo.CheckoutBranch(branch); err != nil {
			return fmt.Errorf("failed to checkout '%s': %w", branch, err)
		}

		// Check if needs rebase
		needsRebase, err := needsRebase(repo, branch, parent)
		if err != nil {
			return err
		}

		if !needsRebase {
			fmt.Printf("%s %s already up to date on %s\n",
				colors.Success("✓"), colors.BranchCurrent(branch), colors.BranchParent(parent))
			continue
		}

		// Perform rebase (precise --onto or fallback)
		rebaseErr := restackBranchOnto(repo, metadata, branch, parent)
		if rebaseErr != nil {
			// Save remaining branches for gw continue
			remaining := branches[i:]
			state := &config.ContinueState{
				RemainingBranches: remaining,
				OriginalBranch:    originalBranch,
			}
			if saveErr := state.Save(repo.GetContinueStatePath()); saveErr != nil {
				fmt.Printf("%s Could not save continue state: %v\n", colors.Warning("⚠"), saveErr)
			}

			fmt.Println()
			fmt.Printf("%s Conflict restacking %s onto %s\n",
				colors.Warning("⚠"),
				colors.BranchCurrent(branch),
				colors.BranchParent(parent))
			fmt.Println()
			fmt.Println(colors.Muted("To continue:"))
			fmt.Println(colors.Muted("  1. Resolve conflicts"))
			fmt.Println(colors.Muted("  2. git add ."))
			fmt.Println(colors.Muted("  3. gw continue"))
			fmt.Println()
			fmt.Println(colors.Muted("To abort: git rebase --abort"))
			return fmt.Errorf("rebase conflict")
		}

		// Update ParentRevision after successful rebase
		parentSHA, _ := repo.GetBranchCommit(parent)
		if err := metadata.SetParentRevision(branch, parentSHA); err == nil {
			_ = metadata.Save(repo.GetMetadataPath())
		}

		fmt.Printf("%s Restacked %s onto %s\n",
			colors.Success("✓"),
			colors.BranchCurrent(branch),
			colors.BranchParent(parent))
	}

	// Clear any continue state
	_ = config.ClearContinueState(repo.GetContinueStatePath())

	// Return to original branch
	if err := repo.CheckoutBranch(originalBranch); err != nil {
		fmt.Printf("%s Could not return to %s: %v\n",
			colors.Warning("⚠"), colors.BranchCurrent(originalBranch), err)
	}

	fmt.Println()
	fmt.Printf("%s All branches restacked!\n", colors.Success("✓"))
	return nil
}

// restackBranchOnto rebases a branch using --onto when ParentRevision is available, otherwise falls back to plain rebase
func restackBranchOnto(repo *git.Repo, metadata *config.Metadata, branch, parent string) error {
	parentRev := metadata.GetParentRevision(branch)
	if parentRev != "" {
		return repo.RebaseOnto(branch, parent, parentRev)
	}
	return repo.Rebase(branch, parent)
}

// restackBranch rebases a branch onto its parent (used by other commands and tests)
func restackBranch(repo *git.Repo, branch, parent string) error {
	// Check if branch needs rebasing
	nr, err := needsRebase(repo, branch, parent)
	if err != nil {
		return err
	}

	if !nr {
		fmt.Printf("%s does not need to be restacked on %s.\n", branch, parent)
		return nil
	}

	// Perform rebase
	_, err = repo.RunGitCommand("rebase", parent, branch)
	if err != nil {
		fmt.Printf("\nHit conflict restacking %s on %s.\n", branch, parent)
		fmt.Println("\nTo fix and continue:")
		fmt.Println("  (1) resolve the merge conflicts")
		fmt.Println("  (2) stage changes with: git add .")
		fmt.Println("  (3) continue rebase: git rebase --continue")
		fmt.Println("  (4) restack remaining: gw stack restack")
		fmt.Println("\nTo abort: git rebase --abort")
		return fmt.Errorf("rebase conflict")
	}

	fmt.Printf("Restacked %s on %s.\n", branch, parent)
	return nil
}

// restackChildren recursively restacks all children of a node (used by fold, delete, split, modify, move)
func restackChildren(repo *git.Repo, s *stack.Stack, parent *stack.Node) error {
	for _, child := range parent.Children {
		// Checkout child branch
		if err := repo.CheckoutBranch(child.Name); err != nil {
			return fmt.Errorf("failed to checkout '%s': %w", child.Name, err)
		}

		// Restack this child
		if err := restackBranch(repo, child.Name, parent.Name); err != nil {
			return err
		}

		// Recursively restack its children
		if len(child.Children) > 0 {
			if err := restackChildren(repo, s, child); err != nil {
				return err
			}
		}
	}

	return nil
}

// needsRebase checks if a branch needs to be rebased onto its parent
func needsRebase(repo *git.Repo, branch, parent string) (bool, error) {
	// Get merge base between branch and parent
	mergeBase, err := repo.RunGitCommand("merge-base", branch, parent)
	if err != nil {
		return false, fmt.Errorf("failed to get merge base: %w", err)
	}

	// Get parent's current commit
	parentCommit, err := repo.RunGitCommand("rev-parse", parent)
	if err != nil {
		return false, fmt.Errorf("failed to get parent commit: %w", err)
	}

	// If merge base != parent commit, branch needs rebasing
	return mergeBase != parentCommit, nil
}
