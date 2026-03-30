package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestRestackChildrenRecursive(t *testing.T) {
	// Test restackChildren with nested children that all need rebasing
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create: main -> a -> b -> c
	repo.createBranch(t, "feat-a", "main")
	repo.commitFile(t, "a.txt", "a", "a commit")
	repo.createBranch(t, "feat-b", "feat-a")
	repo.commitFile(t, "b.txt", "b", "b commit")
	repo.createBranch(t, "feat-c", "feat-b")
	repo.commitFile(t, "c.txt", "c", "c commit")

	// Move main forward
	if err := repo.repo.CheckoutBranch("feat-a"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	repo.commitFile(t, "a2.txt", "a2", "a commit 2")

	// Build stack
	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	node := s.GetNode("feat-a")
	if node == nil {
		t.Fatal("feat-a not found in stack")
	}

	// Restack children of feat-a
	if err := restackChildren(repo.repo, s, node); err != nil {
		t.Fatalf("restackChildren recursive failed: %v", err)
	}
}

func TestRestackChildrenConflict(t *testing.T) {
	// Test restackChildren when a child has a conflict
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.commitFile(t, "shared.txt", "parent", "parent commit")
	repo.createBranch(t, "feat-child-conflict", "feat-parent")
	repo.commitFile(t, "shared.txt", "child", "child commit")

	// Modify parent to conflict
	if err := repo.repo.CheckoutBranch("feat-parent"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	repo.commitFile(t, "shared.txt", "parent-modified", "parent modify")

	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	node := s.GetNode("feat-parent")
	if node == nil {
		t.Fatal("feat-parent not found")
	}

	err = restackChildren(repo.repo, s, node)
	if err == nil {
		t.Fatal("expected conflict error from restackChildren")
	}
}

func TestDescendantsDFSNilNode(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	// Non-existent branch should return nil
	result := descendantsDFS(s, "nonexistent")
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestRunStackRestackDirtyWorkingTree(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-dirty", "main")

	// Make working tree dirty
	repo.commitFile(t, "dirty.txt", "initial", "initial")

	// Write without committing
	if err := os.WriteFile(filepath.Join(repo.dir, "dirty.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prevOnly := restackOnly
	defer func() { restackOnly = prevOnly }()

	err := runStackRestack(nil, nil)
	if err == nil {
		t.Fatal("expected dirty working tree error")
	}
}

func TestRunStackRestackMutuallyExclusive(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-excl", "main")

	prevOnly := restackOnly
	prevUp := restackUpstack
	prevDown := restackDownstack
	defer func() {
		restackOnly = prevOnly
		restackUpstack = prevUp
		restackDownstack = prevDown
	}()

	restackOnly = true
	restackUpstack = true

	err := runStackRestack(nil, nil)
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
}

func TestRunStackRestackBranchFlag(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-target", "main")
	repo.commitFile(t, "target.txt", "data", "target commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prevBranch := restackBranchFlag
	prevOnly := restackOnly
	prevUp := restackUpstack
	prevDown := restackDownstack
	defer func() {
		restackBranchFlag = prevBranch
		restackOnly = prevOnly
		restackUpstack = prevUp
		restackDownstack = prevDown
	}()

	restackBranchFlag = "feat-target"
	restackOnly = true
	restackUpstack = false
	restackDownstack = false

	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack with --branch failed: %v", err)
	}
}

func TestRunStackRestackBranchNotTracked(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	prevBranch := restackBranchFlag
	defer func() { restackBranchFlag = prevBranch }()

	restackBranchFlag = "nonexistent-branch"

	err := runStackRestack(nil, nil)
	if err == nil {
		t.Fatal("expected not tracked error")
	}
}

func TestComputeRestackBranchesDownstackFromTrunk(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	prevDown := restackDownstack
	defer func() { restackDownstack = prevDown }()
	restackDownstack = true

	_, computeErr := computeRestackBranches(s, repo.cfg, "main")
	if computeErr == nil {
		t.Fatal("expected error for downstack from trunk")
	}
}

func TestComputeRestackBranchesOnlyFromTrunk(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	prevOnly := restackOnly
	defer func() { restackOnly = prevOnly }()
	restackOnly = true

	_, computeErr := computeRestackBranches(s, repo.cfg, "main")
	if computeErr == nil {
		t.Fatal("expected error for only from trunk")
	}
}

func TestRunLinearRestackConflict(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-conflict", "main")
	repo.commitFile(t, "shared.txt", "branch", "branch commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	repo.commitFile(t, "shared.txt", "main-conflict", "main commit")

	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	err = runLinearRestack(repo.repo, repo.metadata, s, []string{"feat-conflict"}, "main")
	if err == nil {
		t.Fatal("expected rebase conflict error")
	}
}

func TestNeedsRebaseParentError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-nr", "main")

	_, err := needsRebase(repo.repo, "feat-nr", "nonexistent-parent")
	if err == nil {
		t.Fatal("expected error for nonexistent parent")
	}
}

