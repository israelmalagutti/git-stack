package git

import (
	"fmt"
	"strings"
)

// GetCurrentBranch returns the name of the current branch
func (r *Repo) GetCurrentBranch() (string, error) {
	output, err := r.RunGitCommand("branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := strings.TrimSpace(output)
	if branch == "" {
		return "", fmt.Errorf("not on any branch (detached HEAD)")
	}

	return branch, nil
}

// ListBranches returns a list of all local branches
func (r *Repo) ListBranches() ([]string, error) {
	output, err := r.RunGitCommand("branch", "--format=%(refname:short)")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	if output == "" {
		return []string{}, nil
	}

	branches := strings.Split(output, "\n")
	return branches, nil
}

// BranchExists checks if a branch exists
func (r *Repo) BranchExists(branch string) bool {
	_, err := r.RunGitCommand("rev-parse", "--verify", branch)
	return err == nil
}

// CreateBranch creates a new branch from the current HEAD
func (r *Repo) CreateBranch(name string) error {
	_, err := r.RunGitCommand("branch", name)
	if err != nil {
		return fmt.Errorf("failed to create branch %s: %w", name, err)
	}
	return nil
}

// CheckoutBranch switches to the specified branch
func (r *Repo) CheckoutBranch(branch string) error {
	_, err := r.RunGitCommand("checkout", branch)
	if err != nil {
		return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
	}
	return nil
}

// DeleteBranch deletes a branch (force delete if merged is false)
func (r *Repo) DeleteBranch(branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}

	_, err := r.RunGitCommand("branch", flag, branch)
	if err != nil {
		return fmt.Errorf("failed to delete branch %s: %w", branch, err)
	}
	return nil
}

// GetBranchCommit returns the commit SHA of a branch
func (r *Repo) GetBranchCommit(branch string) (string, error) {
	output, err := r.RunGitCommand("rev-parse", branch)
	if err != nil {
		return "", fmt.Errorf("failed to get commit for branch %s: %w", branch, err)
	}
	return output, nil
}

// Fetch fetches from all remotes with prune
func (r *Repo) Fetch() error {
	_, err := r.RunGitCommand("fetch", "--all", "--prune")
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}
	return nil
}

// HasRemoteBranch checks if a remote tracking branch exists
func (r *Repo) HasRemoteBranch(branch, remote string) bool {
	remoteBranch := remote + "/" + branch
	_, err := r.RunGitCommand("rev-parse", "--verify", remoteBranch)
	return err == nil
}

// CanFastForward checks if local branch can fast-forward to remote
func (r *Repo) CanFastForward(local, remote string) (bool, error) {
	// Get the merge base
	mergeBase, err := r.RunGitCommand("merge-base", local, remote)
	if err != nil {
		return false, fmt.Errorf("failed to get merge base: %w", err)
	}

	// Get local commit
	localCommit, err := r.RunGitCommand("rev-parse", local)
	if err != nil {
		return false, fmt.Errorf("failed to get local commit: %w", err)
	}

	// Can fast-forward if merge-base equals local commit
	return mergeBase == localCommit, nil
}

// ResetToRemote hard resets a local branch to match remote
func (r *Repo) ResetToRemote(branch, remoteBranch string) error {
	// Save current branch
	currentBranch, err := r.GetCurrentBranch()
	if err != nil {
		return err
	}

	// Checkout target branch
	if currentBranch != branch {
		if err := r.CheckoutBranch(branch); err != nil {
			return err
		}
	}

	// Reset to remote
	_, err = r.RunGitCommand("reset", "--hard", remoteBranch)
	if err != nil {
		return fmt.Errorf("failed to reset to %s: %w", remoteBranch, err)
	}

	// Return to original branch if different
	if currentBranch != branch {
		if err := r.CheckoutBranch(currentBranch); err != nil {
			return fmt.Errorf("failed to return to original branch: %w", err)
		}
	}

	return nil
}

// IsMergedInto checks if branch is merged into target (regular merge or squash merge)
func (r *Repo) IsMergedInto(branch, target string) (bool, error) {
	// Fast check: regular merge (branch is ancestor of target)
	output, err := r.RunGitCommand("branch", "--merged", target, "--format=%(refname:short)")
	if err != nil {
		return false, fmt.Errorf("failed to check merged branches: %w", err)
	}

	mergedBranches := strings.Split(output, "\n")
	for _, merged := range mergedBranches {
		if strings.TrimSpace(merged) == branch {
			return true, nil
		}
	}

	// Squash-merge check: if merging branch into target would produce
	// the same tree as target, the branch's changes are already there
	mergeTreeOutput, err := r.RunGitCommand("merge-tree", "--write-tree", target, branch)
	if err != nil {
		// Merge would have conflicts — not merged
		return false, nil
	}
	mergeTree := strings.TrimSpace(strings.Split(mergeTreeOutput, "\n")[0])

	targetTree, err := r.RunGitCommand("rev-parse", target+"^{tree}")
	if err != nil {
		return false, err
	}

	return mergeTree == strings.TrimSpace(targetTree), nil
}

// HasUncommittedChanges checks if the working tree has staged or unstaged changes to tracked files
func (r *Repo) HasUncommittedChanges() (bool, error) {
	output, err := r.RunGitCommand("status", "--porcelain", "-uno")
	if err != nil {
		return false, fmt.Errorf("failed to check working tree status: %w", err)
	}
	return strings.TrimSpace(output) != "", nil
}

// IsBehind checks if branch is behind its parent (needs rebase)
func (r *Repo) IsBehind(branch, parent string) (bool, error) {
	// Get merge base between branch and parent
	mergeBase, err := r.RunGitCommand("merge-base", branch, parent)
	if err != nil {
		return false, fmt.Errorf("failed to get merge base: %w", err)
	}

	// Get parent's current commit
	parentCommit, err := r.RunGitCommand("rev-parse", parent)
	if err != nil {
		return false, fmt.Errorf("failed to get parent commit: %w", err)
	}

	// If merge base != parent commit, branch is behind
	return mergeBase != parentCommit, nil
}

// Rebase rebases a branch onto another
func (r *Repo) Rebase(branch, onto string) error {
	_, err := r.RunGitCommand("rebase", onto, branch)
	return err
}

// RebaseOnto replays only commits between oldBase and branch onto newBase
func (r *Repo) RebaseOnto(branch, newBase, oldBase string) error {
	_, err := r.RunGitCommand("rebase", "--onto", newBase, oldBase, branch)
	return err
}

// AbortRebase aborts an in-progress rebase
func (r *Repo) AbortRebase() error {
	_, err := r.RunGitCommand("rebase", "--abort")
	return err
}
