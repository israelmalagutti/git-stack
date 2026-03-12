package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsMergedIntoDetectsSquashMerge(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Create feature branch with commits
	if _, err := repo.RunGitCommand("checkout", "-b", "feat"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "feat.txt"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Squash-merge into main
	if _, err := repo.RunGitCommand("checkout", "main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "--squash", "feat"); err != nil {
		t.Fatalf("failed to squash merge: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "squash merge feat"); err != nil {
		t.Fatalf("failed to commit squash: %v", err)
	}

	// Regular merge check should miss it
	output, err := repo.RunGitCommand("branch", "--merged", "main", "--format=%(refname:short)")
	if err != nil {
		t.Fatalf("failed to list merged: %v", err)
	}
	for _, b := range splitLines(output) {
		if b == "feat" {
			t.Fatal("git branch --merged should NOT detect squash merge")
		}
	}

	// IsMergedInto should detect it
	merged, err := repo.IsMergedInto("feat", "main")
	if err != nil {
		t.Fatalf("IsMergedInto failed: %v", err)
	}
	if !merged {
		t.Fatal("IsMergedInto should detect squash-merged branch")
	}
}

func TestIsMergedIntoRegularMerge(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Create and regular-merge a branch
	if _, err := repo.RunGitCommand("checkout", "-b", "feat"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "feat.txt"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if _, err := repo.RunGitCommand("checkout", "main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat"); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	merged, err := repo.IsMergedInto("feat", "main")
	if err != nil {
		t.Fatalf("IsMergedInto failed: %v", err)
	}
	if !merged {
		t.Fatal("IsMergedInto should detect regular merge")
	}
}

func TestIsMergedIntoNotMerged(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Create branch with changes NOT merged to main
	if _, err := repo.RunGitCommand("checkout", "-b", "feat"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "feat.txt"), []byte("feature\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "feat.txt"); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	merged, err := repo.IsMergedInto("feat", "main")
	if err != nil {
		t.Fatalf("IsMergedInto failed: %v", err)
	}
	if merged {
		t.Fatal("IsMergedInto should NOT detect unmerged branch")
	}
}

func TestHasUncommittedChanges(t *testing.T) {
	dir, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Clean state
	dirty, err := repo.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}
	if dirty {
		t.Fatal("expected clean working tree")
	}

	// Staged change
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("dirty\n"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "README.md"); err != nil {
		t.Fatalf("failed to stage: %v", err)
	}

	dirty, err = repo.HasUncommittedChanges()
	if err != nil {
		t.Fatalf("HasUncommittedChanges failed: %v", err)
	}
	if !dirty {
		t.Fatal("expected dirty working tree with staged changes")
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}
