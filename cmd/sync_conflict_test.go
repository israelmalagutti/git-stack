package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func TestRunSync_WithConflictingRestack(t *testing.T) {
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

	// Create a branch with a commit
	if err := repo.CreateBranch("feat-conflict"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.CheckoutBranch("feat-conflict"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "conflict.txt"), []byte("branch"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "branch commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	sha, _ := repo.GetBranchCommit("main")
	metadata.TrackBranch("feat-conflict", "main", sha)

	// Create conflicting change on main
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "conflict.txt"), []byte("main"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "main commit"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Push both to origin
	if _, err := repo.RunGitCommand("push", "origin", "main"); err != nil {
		t.Fatalf("push main: %v", err)
	}

	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	syncForce = true
	syncRestack = true
	syncDelete = true
	defer func() {
		syncForce = false
		syncRestack = false
		syncDelete = true
	}()

	// This should report a conflicting branch
	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync with conflict failed: %v", err)
	}
}

func TestRunSync_ReturnToOriginalBranch(t *testing.T) {
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

	// Create and track a branch, checkout it
	if err := repo.CreateBranch("feat-return"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.CheckoutBranch("feat-return"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	sha, _ := repo.GetBranchCommit("main")
	metadata.TrackBranch("feat-return", "main", sha)
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
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
		t.Fatalf("runSync return to branch failed: %v", err)
	}

	// Should be back on feat-return
	current, _ := repo.GetCurrentBranch()
	if current != "feat-return" {
		t.Errorf("expected to return to feat-return, got %s", current)
	}
}
