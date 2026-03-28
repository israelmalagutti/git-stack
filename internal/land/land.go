package land

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/provider"
)

// Opts configures a single branch landing.
type Opts struct {
	Branch         string
	NoDeleteRemote bool
}

// PRBaseUpdate records a child PR whose base branch was updated.
type PRBaseUpdate struct {
	Branch   string
	PRNumber int
	NewBase  string
}

// Result describes what happened after landing a branch.
type Result struct {
	Landed             string
	NewParent          string
	ReparentedChildren []string
	UpdatedPRBases     []PRBaseUpdate
	CheckedOut         string // non-empty if we had to switch branches
}

// Branch lands a single merged branch. It verifies the branch is merged,
// reparents children, optionally updates children's PR bases, deletes the
// branch, and untracks it. The caller is responsible for saving metadata,
// pushing refs, and restacking afterward.
func Branch(
	repo *git.Repo,
	metadata *config.Metadata,
	prov provider.Provider, // may be nil for no-PR flow
	trunk string,
	currentBranch string,
	remote string,
	opts Opts,
) (*Result, error) {
	branch := opts.Branch

	// Validate
	if !metadata.IsTracked(branch) {
		return nil, fmt.Errorf("branch '%s' is not tracked by gs", branch)
	}

	parent, ok := metadata.GetParent(branch)
	if !ok || parent == "" {
		return nil, fmt.Errorf("branch '%s' has no parent", branch)
	}

	// Check merge status
	merged, err := isBranchMerged(repo, metadata, prov, branch, trunk)
	if err != nil {
		return nil, err
	}
	if !merged {
		return nil, fmt.Errorf("branch '%s' is not merged into '%s'", branch, trunk)
	}

	result := &Result{
		Landed:    branch,
		NewParent: parent,
	}

	// Reparent children
	children := metadata.GetChildren(branch)
	for _, child := range children {
		if err := metadata.UpdateParent(child, parent); err != nil {
			return nil, fmt.Errorf("failed to reparent '%s': %w", child, err)
		}
		result.ReparentedChildren = append(result.ReparentedChildren, child)
	}

	// Update children's PR base branches on the provider
	if prov != nil {
		for _, child := range children {
			pr := metadata.GetPR(child)
			if pr == nil {
				continue
			}
			if err := prov.UpdatePRBase(pr.Number, parent); err != nil {
				// Warn but don't abort — PR base can be updated manually
				continue
			}
			result.UpdatedPRBases = append(result.UpdatedPRBases, PRBaseUpdate{
				Branch:   child,
				PRNumber: pr.Number,
				NewBase:  parent,
			})
		}
	}

	// If landing the current branch, checkout parent first
	if branch == currentBranch {
		if err := repo.CheckoutBranch(parent); err != nil {
			return nil, fmt.Errorf("failed to checkout parent '%s': %w", parent, err)
		}
		result.CheckedOut = parent
	}

	// Delete local branch
	if repo.BranchExists(branch) {
		if err := repo.DeleteBranch(branch, true); err != nil {
			return nil, fmt.Errorf("failed to delete local branch: %w", err)
		}
	}

	// Delete remote branch (best-effort)
	if !opts.NoDeleteRemote && remote != "" {
		_, _ = repo.RunGitCommand("push", remote, "--delete", branch)
	}

	// Untrack from metadata
	metadata.UntrackBranch(branch)

	return result, nil
}

// FindMergedBranches returns all tracked branches whose PRs are merged
// or whose commits are in trunk.
func FindMergedBranches(
	repo *git.Repo,
	metadata *config.Metadata,
	prov provider.Provider,
	trunk string,
) ([]string, error) {
	var merged []string
	for branch := range metadata.Branches {
		if branch == trunk {
			continue
		}
		ok, err := isBranchMerged(repo, metadata, prov, branch, trunk)
		if err != nil {
			continue
		}
		if ok {
			merged = append(merged, branch)
		}
	}
	return merged, nil
}

// isBranchMerged checks if a branch is merged via PR status or git merge-base.
func isBranchMerged(
	repo *git.Repo,
	metadata *config.Metadata,
	prov provider.Provider,
	branch, trunk string,
) (bool, error) {
	// Check PR status if available
	if prov != nil {
		if pr := metadata.GetPR(branch); pr != nil {
			status, err := prov.GetPRStatus(pr.Number)
			if err == nil && status.State == "merged" {
				return true, nil
			}
		}
	}

	// Fall back to git merge-base check
	merged, err := repo.IsMergedInto(branch, trunk)
	if err != nil {
		return false, fmt.Errorf("failed to check merge status: %w", err)
	}
	return merged, nil
}
