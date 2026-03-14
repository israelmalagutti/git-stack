package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestRunStackRestackTrunkAndUntracked(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Trunk with no children
	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack trunk failed: %v", err)
	}

	// Untracked branch error
	if err := repo.repo.CreateBranch("untracked-restack"); err != nil {
		t.Fatalf("failed to create untracked branch: %v", err)
	}
	if err := repo.repo.CheckoutBranch("untracked-restack"); err != nil {
		t.Fatalf("failed to checkout untracked: %v", err)
	}
	if err := runStackRestack(nil, nil); err == nil {
		t.Fatalf("expected untracked restack error")
	}
}

func TestRunStackRestackBranchWithChild(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-restack", "main")
	repo.commitFile(t, "restack.txt", "data", "restack commit")
	repo.createBranch(t, "feat-restack-child", "feat-restack")

	if err := repo.repo.CheckoutBranch("feat-restack"); err != nil {
		t.Fatalf("failed to checkout branch: %v", err)
	}
	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack branch failed: %v", err)
	}
}

func TestRunStackRestackFromTrunkWithChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-restack-trunk", "main")
	repo.commitFile(t, "restack.txt", "data", "restack commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack trunk children failed: %v", err)
	}
}

func TestRestackChildrenCheckoutError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	trunk := &stack.Node{Name: "main", IsTrunk: true}
	child := &stack.Node{Name: "missing-branch", Parent: trunk}
	trunk.Children = []*stack.Node{child}
	s := &stack.Stack{
		Trunk: trunk,
		Nodes: map[string]*stack.Node{
			"main":           trunk,
			"missing-branch": child,
		},
	}

	if err := restackChildren(repo.repo, s, trunk); err == nil {
		t.Fatalf("expected restackChildren checkout error")
	}
}

func TestNeedsRebaseError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if _, err := needsRebase(repo.repo, "missing", "main"); err == nil {
		t.Fatalf("expected needsRebase error")
	}
}
