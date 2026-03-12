package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanStaleBranchesReparentsChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create: main → parent → child
	repo.createBranch(t, "parent", "main")
	repo.commitFile(t, "parent.txt", "parent", "parent commit")

	repo.createBranch(t, "child", "parent")
	repo.commitFile(t, "child.txt", "child", "child commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Delete parent from git (simulating GitHub merge+delete)
	if err := repo.repo.DeleteBranch("parent", true); err != nil {
		t.Fatalf("failed to delete parent: %v", err)
	}

	// Run cleanStaleBranches
	if err := cleanStaleBranches(repo.repo, repo.metadata, repo.cfg, true); err != nil {
		t.Fatalf("cleanStaleBranches failed: %v", err)
	}

	// child should be reparented to main
	parent, ok := repo.metadata.GetParent("child")
	if !ok {
		t.Fatal("child should still be tracked")
	}
	if parent != "main" {
		t.Fatalf("expected child parent to be main, got %s", parent)
	}

	// parent should be untracked
	if repo.metadata.IsTracked("parent") {
		t.Fatal("parent should be untracked after cleanup")
	}
}

func TestCleanStaleBranchesReparentsChainedOrphans(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create: main → A → B → C
	repo.createBranch(t, "branch-a", "main")
	repo.commitFile(t, "a.txt", "a", "a commit")

	repo.createBranch(t, "branch-b", "branch-a")
	repo.commitFile(t, "b.txt", "b", "b commit")

	repo.createBranch(t, "branch-c", "branch-b")
	repo.commitFile(t, "c.txt", "c", "c commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Delete A and B from git
	if err := repo.repo.DeleteBranch("branch-a", true); err != nil {
		t.Fatalf("failed to delete branch-a: %v", err)
	}
	if err := repo.repo.DeleteBranch("branch-b", true); err != nil {
		t.Fatalf("failed to delete branch-b: %v", err)
	}

	// Run cleanStaleBranches
	if err := cleanStaleBranches(repo.repo, repo.metadata, repo.cfg, true); err != nil {
		t.Fatalf("cleanStaleBranches failed: %v", err)
	}

	// C should be reparented to main (skipping both A and B)
	parent, ok := repo.metadata.GetParent("branch-c")
	if !ok {
		t.Fatal("branch-c should still be tracked")
	}
	if parent != "main" {
		t.Fatalf("expected branch-c parent to be main, got %s", parent)
	}
}

func TestCleanStaleBranchesReparentsMultipleChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create: main → parent → {child1, child2}
	repo.createBranch(t, "parent", "main")
	repo.commitFile(t, "parent.txt", "parent", "parent commit")

	repo.createBranch(t, "child1", "parent")
	repo.commitFile(t, "child1.txt", "child1", "child1 commit")

	if err := repo.repo.CheckoutBranch("parent"); err != nil {
		t.Fatalf("failed to checkout parent: %v", err)
	}
	repo.createBranch(t, "child2", "parent")
	repo.commitFile(t, "child2.txt", "child2", "child2 commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Delete parent from git
	if err := repo.repo.DeleteBranch("parent", true); err != nil {
		t.Fatalf("failed to delete parent: %v", err)
	}

	if err := cleanStaleBranches(repo.repo, repo.metadata, repo.cfg, true); err != nil {
		t.Fatalf("cleanStaleBranches failed: %v", err)
	}

	// Both children should be reparented to main
	for _, child := range []string{"child1", "child2"} {
		parent, ok := repo.metadata.GetParent(child)
		if !ok {
			t.Fatalf("%s should still be tracked", child)
		}
		if parent != "main" {
			t.Fatalf("expected %s parent to be main, got %s", child, parent)
		}
	}
}

func TestDeleteMergedBranchesDetectsSquashMerge(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create feature branch with a commit
	repo.createBranch(t, "feat-squash", "main")
	repo.commitFile(t, "feat.txt", "feature", "feat commit")

	// Squash-merge into main
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("merge", "--squash", "feat-squash"); err != nil {
		t.Fatalf("failed to squash merge: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "squash merge feat"); err != nil {
		t.Fatalf("failed to commit squash: %v", err)
	}

	// deleteMergedBranches should detect the squash merge
	if err := deleteMergedBranches(repo.repo, repo.metadata, "main", true); err != nil {
		t.Fatalf("deleteMergedBranches failed: %v", err)
	}

	// feat-squash should be deleted
	if repo.metadata.IsTracked("feat-squash") {
		t.Fatal("feat-squash should be untracked after squash-merge detection")
	}
	if repo.repo.BranchExists("feat-squash") {
		t.Fatal("feat-squash git branch should be deleted")
	}
}

func TestSyncAbortsDirtyWorkingTreeStaged(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Staged change should block sync
	if err := os.WriteFile(filepath.Join(repo.dir, "README.md"), []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "README.md"); err != nil {
		t.Fatalf("failed to stage file: %v", err)
	}

	err := runSync(nil, nil)
	if err == nil {
		t.Fatal("expected runSync to fail with staged changes")
	}
	if err.Error() != "you have uncommitted changes. Please commit or stash them before syncing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncAbortsDirtyWorkingTreeUnstaged(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Unstaged modification to tracked file should block sync
	if err := os.WriteFile(filepath.Join(repo.dir, "README.md"), []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	err := runSync(nil, nil)
	if err == nil {
		t.Fatal("expected runSync to fail with unstaged changes")
	}
	if err.Error() != "you have uncommitted changes. Please commit or stash them before syncing" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncAllowsUntrackedFiles(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Untracked file should NOT block sync
	if err := os.WriteFile(filepath.Join(repo.dir, "untracked.txt"), []byte("new\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Should succeed (no remote, so just fetches and finishes)
	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync should not fail with untracked files: %v", err)
	}
}

func TestSyncNoDeleteSkipsMergedBranches(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create and merge a branch
	repo.createBranch(t, "feat-merged", "main")
	repo.commitFile(t, "feat.txt", "feature", "feat commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("merge", "feat-merged"); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	// Run sync with delete disabled
	prevForce := syncForce
	prevRestack := syncRestack
	prevDelete := syncDelete
	defer func() {
		syncForce = prevForce
		syncRestack = prevRestack
		syncDelete = prevDelete
	}()

	syncForce = true
	syncRestack = false
	syncDelete = false

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync failed: %v", err)
	}

	// Branch should still exist — delete was disabled
	if !repo.metadata.IsTracked("feat-merged") {
		t.Fatal("feat-merged should still be tracked when --delete=false")
	}
	if !repo.repo.BranchExists("feat-merged") {
		t.Fatal("feat-merged git branch should still exist when --delete=false")
	}
}
