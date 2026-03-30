package ops

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

// DeleteResult holds the outcome of a branch deletion.
type DeleteResult struct {
	Deleted            string
	ReparentedChildren []string
	NewParent          string
	CheckedOut         string // non-empty if we had to checkout parent
}

// DeleteBranch removes a tracked branch from the stack and git.
// It validates the branch, reparents children to the deleted branch's parent,
// deletes the git branch, and cleans up metadata. If the deleted branch is
// the current branch, it checks out the parent first.
//
// This does NOT restack children — callers should do that themselves since
// the CLI and MCP callers handle it differently (interactive vs. silent).
func DeleteBranch(repo *git.Repo, metadata *config.Metadata, s *stack.Stack, branchName string) (*DeleteResult, error) {
	// Validate
	if branchName == s.TrunkName {
		return nil, fmt.Errorf("cannot delete trunk branch")
	}
	if !repo.BranchExists(branchName) {
		return nil, fmt.Errorf("branch '%s' does not exist", branchName)
	}
	if !metadata.IsTracked(branchName) {
		return nil, fmt.Errorf("branch '%s' is not tracked by gs", branchName)
	}

	node := s.GetNode(branchName)
	if node == nil {
		return nil, fmt.Errorf("branch not found in stack")
	}

	parentBranch, ok := metadata.GetParent(branchName)
	if !ok {
		return nil, fmt.Errorf("branch has no parent")
	}

	result := &DeleteResult{
		Deleted:   branchName,
		NewParent: parentBranch,
	}

	// If deleting current branch, checkout parent first
	if branchName == s.Current {
		if err := repo.CheckoutBranch(parentBranch); err != nil {
			return nil, fmt.Errorf("failed to checkout parent: %w", err)
		}
		result.CheckedOut = parentBranch
	}

	// Reparent children
	result.ReparentedChildren = make([]string, 0, len(node.Children))
	for _, child := range node.Children {
		if err := metadata.UpdateParent(child.Name, parentBranch); err != nil {
			return nil, fmt.Errorf("failed to reparent '%s': %w", child.Name, err)
		}
		result.ReparentedChildren = append(result.ReparentedChildren, child.Name)
	}

	// Delete the git branch
	if _, err := repo.RunGitCommand("branch", "-D", branchName); err != nil {
		return nil, fmt.Errorf("failed to delete branch: %w", err)
	}

	// Remove from metadata and persist
	metadata.UntrackBranch(branchName)
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	// Clean up remote refs
	config.DeleteRemoteMetadataRef(repo, branchName)
	if len(result.ReparentedChildren) > 0 {
		config.PushMetadataRefs(repo, result.ReparentedChildren...)
	}

	return result, nil
}
