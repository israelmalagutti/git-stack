package cmd

import (
	"os"
	"testing"
)

func TestBranchHasCommits(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-one", "main")
	repo.commitFile(t, "feat.txt", "data", "feat commit")

	has, err := branchHasCommits(repo.repo, "feat-one", "main")
	if err != nil {
		t.Fatalf("branchHasCommits failed: %v", err)
	}
	if !has {
		t.Fatalf("expected branch to have commits")
	}
}

func TestHasUnstagedChanges(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.commitFile(t, "clean.txt", "data", "clean")
	changed, err := hasUnstagedChanges(repo.repo)
	if err != nil {
		t.Fatalf("hasUnstagedChanges failed: %v", err)
	}
	if changed {
		t.Fatalf("expected no unstaged changes")
	}

	if err := os.WriteFile(repo.dir+"/README.md", []byte("dirty"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	changed, err = hasUnstagedChanges(repo.repo)
	if err != nil {
		t.Fatalf("hasUnstagedChanges failed: %v", err)
	}
	if !changed {
		t.Fatalf("expected unstaged changes")
	}
}

func TestCountCommits(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-count", "main")
	repo.commitFile(t, "count.txt", "data", "count commit")

	count, err := countCommits(repo.repo, "feat-count", "main")
	if err != nil {
		t.Fatalf("countCommits failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 commit, got %d", count)
	}
}

func TestNeedsRebase(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-rebase", "main")
	needs, err := needsRebase(repo.repo, "feat-rebase", "main")
	if err != nil {
		t.Fatalf("needsRebase failed: %v", err)
	}
	if needs {
		t.Fatalf("did not expect rebase before parent changes")
	}

	_ = repo.repo.CheckoutBranch("main")
	repo.commitFile(t, "parent.txt", "parent", "parent commit")

	needs, err = needsRebase(repo.repo, "feat-rebase", "main")
	if err != nil {
		t.Fatalf("needsRebase failed: %v", err)
	}
	if !needs {
		t.Fatalf("expected rebase after parent changes")
	}
}

func TestConfirmHelpers(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	if _, err := w.Write([]byte("y\n")); err != nil {
		t.Fatalf("failed to write to pipe: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	if !confirm() {
		t.Fatalf("expected confirm to accept yes")
	}

	r, w, err = os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	if _, err := w.Write([]byte("all\n")); err != nil {
		t.Fatalf("failed to write to pipe: %v", err)
	}
	_ = w.Close()
	os.Stdin = r
	if got := confirmWithOptions(); got != "all" {
		t.Fatalf("expected all, got %q", got)
	}
}
