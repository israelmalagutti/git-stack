package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func TestRunSyncRestack(t *testing.T) {
	localDir, _, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}
	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// Create a feature branch with a commit and track it
	if err := repo.CreateBranch("feat-sync"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-sync", "main", "")
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}
	if err := repo.CheckoutBranch("feat-sync"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "feat.txt"), []byte("feat"), 0644); err != nil {
		t.Fatalf("failed to write feat file: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "feat.txt"); err != nil {
		t.Fatalf("failed to add feat file: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit feat: %v", err)
	}

	// Move main forward so feat is behind
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localDir, "main.txt"), []byte("main"), 0644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "main.txt"); err != nil {
		t.Fatalf("failed to add main file: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "main commit"); err != nil {
		t.Fatalf("failed to commit main: %v", err)
	}

	prevForce := syncForce
	prevRestack := syncRestack
	defer func() {
		syncForce = prevForce
		syncRestack = prevRestack
	}()

	syncForce = true
	syncRestack = true

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync restack failed: %v", err)
	}
}
