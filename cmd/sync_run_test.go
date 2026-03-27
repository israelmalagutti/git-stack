package cmd

import (
	"os"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func TestRunSync(t *testing.T) {
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

	prevForce := syncForce
	prevRestack := syncRestack
	defer func() {
		syncForce = prevForce
		syncRestack = prevRestack
	}()

	syncForce = true
	syncRestack = false

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync failed: %v", err)
	}
}
