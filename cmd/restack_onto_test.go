package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestRestackUsesOntoWhenParentRevisionSet(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create a branch with a commit
	repo.createBranch(t, "feat-onto", "main")
	repo.commitFile(t, "feat.txt", "feat content", "feat commit")

	// Record the parent revision (main's SHA at branch creation time)
	mainSHA, err := repo.repo.GetBranchCommit("main")
	if err != nil {
		t.Fatalf("failed to get main SHA: %v", err)
	}
	if err := repo.metadata.SetParentRevision("feat-onto", mainSHA); err != nil {
		t.Fatalf("failed to set parent revision: %v", err)
	}
	if err := repo.metadata.Save(repo.repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// Add a commit to main (making feat-onto behind)
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "main2.txt", "main2", "main commit 2")

	// restackBranchOnto should use --onto rebase
	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	if err := repo.repo.CheckoutBranch("feat-onto"); err != nil {
		t.Fatalf("failed to checkout feat-onto: %v", err)
	}

	if err := restackBranchOnto(repo.repo, metadata, "feat-onto", "main"); err != nil {
		t.Fatalf("restackBranchOnto failed: %v", err)
	}

	// Verify the branch is now based on main's latest
	mergeBase, err := repo.repo.RunGitCommand("merge-base", "feat-onto", "main")
	if err != nil {
		t.Fatalf("failed to get merge base: %v", err)
	}
	newMainSHA, err := repo.repo.GetBranchCommit("main")
	if err != nil {
		t.Fatalf("failed to get main SHA: %v", err)
	}
	if mergeBase != newMainSHA {
		t.Errorf("expected merge base to equal main tip after restack")
	}
}

func TestRestackFallsBackWithoutParentRevision(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create branch without parentRevision
	repo.createBranch(t, "feat-fallback", "main")
	repo.commitFile(t, "feat.txt", "feat content", "feat commit")

	// Add a commit to main
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "main2.txt", "main2", "main commit 2")

	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	// ParentRevision should be empty
	rev := metadata.GetParentRevision("feat-fallback")
	if rev != "" {
		t.Fatalf("expected empty parentRevision, got %s", rev)
	}

	// Should still work via plain rebase fallback
	if err := restackBranchOnto(repo.repo, metadata, "feat-fallback", "main"); err != nil {
		t.Fatalf("restackBranchOnto fallback failed: %v", err)
	}
}

func TestRestackUpdatesParentRevisionAfterRebase(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-update-rev", "main")
	repo.commitFile(t, "feat.txt", "feat", "feat commit")

	// Add commit to main
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "main2.txt", "main2", "main commit")

	if err := repo.repo.CheckoutBranch("feat-update-rev"); err != nil {
		t.Fatalf("failed to checkout branch: %v", err)
	}

	// Run full restack via runStackRestack
	restackOnly = false
	restackUpstack = false
	restackDownstack = false
	restackBranchFlag = ""
	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack failed: %v", err)
	}

	// Reload metadata and check ParentRevision was updated
	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}

	mainSHA, _ := repo.repo.GetBranchCommit("main")
	rev := metadata.GetParentRevision("feat-update-rev")
	if rev != mainSHA {
		t.Errorf("expected parentRevision '%s', got '%s'", mainSHA, rev)
	}
}

func TestRestackDirtyWorkingTree(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-dirty", "main")

	// Create uncommitted change
	if err := os.WriteFile(filepath.Join(repo.dir, "dirty.txt"), []byte("dirty"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "dirty.txt"); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}

	restackOnly = false
	restackUpstack = false
	restackDownstack = false
	restackBranchFlag = ""
	err := runStackRestack(nil, nil)
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
	if err.Error() != "you have uncommitted changes. Please commit or stash them before restacking" {
		t.Fatalf("unexpected error message: %v", err)
	}
}
