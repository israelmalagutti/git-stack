package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func TestRunSync_WithRestackAndDelete(t *testing.T) {
	localDir, _, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	// Create and track a branch with a commit
	if err := repo.CreateBranch("feat-sync"); err != nil {
		t.Fatalf("create branch: %v", err)
	}
	if err := repo.CheckoutBranch("feat-sync"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	sha, _ := repo.GetBranchCommit("main")
	metadata.TrackBranch("feat-sync", "main", sha)
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := os.WriteFile(filepath.Join(localDir, "sync.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "sync commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	prev := syncForce
	prevR := syncRestack
	prevD := syncDelete
	syncForce = true
	syncRestack = true
	syncDelete = true
	defer func() {
		syncForce = prev
		syncRestack = prevR
		syncDelete = prevD
	}()

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync restack+delete failed: %v", err)
	}
}

func TestRunSync_DirtyWorkingTree(t *testing.T) {
	localDir, _, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	// Make dirty working tree (tracked file modified)
	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("dirty"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "README.md"); err != nil {
		t.Fatalf("add: %v", err)
	}

	syncForce = true
	defer func() { syncForce = false }()

	err = runSync(nil, nil)
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
}

func TestRunSync_NoDelete(t *testing.T) {
	localDir, _, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	syncForce = true
	syncRestack = true
	syncDelete = false
	defer func() {
		syncForce = false
		syncRestack = false
		syncDelete = true
	}()

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync no-delete failed: %v", err)
	}
}

func TestRunSync_ForceWithRestackAndBranches(t *testing.T) {
	localDir, _, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}

	// Create a merged branch for delete path
	if err := repo.CreateBranch("feat-merged"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.CheckoutBranch("feat-merged"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "merged.txt"), []byte("merged"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "merged commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	sha, _ := repo.GetBranchCommit("main")
	metadata.TrackBranch("feat-merged", "main", sha)

	// Merge into main
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat-merged"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	syncForce = true
	syncRestack = true
	syncDelete = true
	defer func() {
		syncForce = false
		syncRestack = false
		syncDelete = true
	}()

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync force+restack+delete failed: %v", err)
	}
}

func TestConfirmWithOptionsKeys(t *testing.T) {
	// Test all branches of confirmWithOptions
	tests := []struct {
		input    byte
		expected string
	}{
		{'y', "yes"},
		{'Y', "yes"},
		{'a', "all"},
		{'A', "all"},
		{'q', "quit"},
		{'Q', "quit"},
		{'n', "no"},
		{'x', "no"},
	}

	for _, tt := range tests {
		origStdin := os.Stdin
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatalf("pipe: %v", err)
		}
		if _, err := w.Write([]byte{tt.input}); err != nil {
			t.Fatalf("write: %v", err)
		}
		_ = w.Close()
		os.Stdin = r

		prevFn := readKeyFn
		readKeyFn = func() (byte, error) {
			b := make([]byte, 1)
			_, err := r.Read(b)
			return b[0], err
		}

		got := confirmWithOptions()
		readKeyFn = prevFn
		os.Stdin = origStdin

		if got != tt.expected {
			t.Errorf("input=%c: expected %q, got %q", tt.input, tt.expected, got)
		}
	}
}

func TestConfirmNo(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.Write([]byte("n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()
	os.Stdin = r

	prevFn := readKeyFn
	readKeyFn = func() (byte, error) {
		b := make([]byte, 1)
		_, err := r.Read(b)
		return b[0], err
	}
	defer func() { readKeyFn = prevFn }()

	if confirm() {
		t.Fatal("expected confirm to reject 'n'")
	}
}
