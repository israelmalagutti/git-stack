package ops

import (
	"fmt"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

// CreateResult holds the outcome of creating a new branch.
type CreateResult struct {
	Branch string
	Parent string
}

// CreateBranch creates a new git branch, checks it out, and tracks it in metadata.
// The new branch is created from the current HEAD position. The parent is recorded
// in metadata along with its current commit SHA for precise restacking.
func CreateBranch(repo *git.Repo, metadata *config.Metadata, name, parent string) (*CreateResult, error) {
	if repo.BranchExists(name) {
		return nil, fmt.Errorf("branch '%s' already exists", name)
	}

	if err := repo.CreateBranch(name); err != nil {
		return nil, fmt.Errorf("failed to create branch: %w", err)
	}
	if err := repo.CheckoutBranch(name); err != nil {
		return nil, fmt.Errorf("failed to checkout branch: %w", err)
	}

	parentSHA, _ := repo.GetBranchCommit(parent)
	metadata.TrackBranch(name, parent, parentSHA)
	if err := metadata.SaveWithRefs(repo, repo.GetMetadataPath()); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}
	config.PushMetadataRefs(repo, name)

	return &CreateResult{
		Branch: name,
		Parent: parent,
	}, nil
}
