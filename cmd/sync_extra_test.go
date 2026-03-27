package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func setupRepoWithRemote(t *testing.T) (string, string, func()) {
	t.Helper()
	base := t.TempDir()
	remote := filepath.Join(base, "remote.git")
	local := filepath.Join(base, "local")
	other := filepath.Join(base, "other")

	if err := exec.Command("git", "init", "--bare", remote).Run(); err != nil {
		t.Fatalf("failed to init bare remote: %v", err)
	}
	if err := exec.Command("git", "init", local).Run(); err != nil {
		t.Fatalf("failed to init local: %v", err)
	}
	if err := exec.Command("git", "init", other).Run(); err != nil {
		t.Fatalf("failed to init other: %v", err)
	}

	cmds := [][]string{
		{"git", "-C", local, "config", "user.email", "test@test.com"},
		{"git", "-C", local, "config", "user.name", "Test User"},
		{"git", "-C", local, "config", "commit.gpgsign", "false"},
		{"git", "-C", other, "config", "user.email", "test@test.com"},
		{"git", "-C", other, "config", "user.name", "Test User"},
		{"git", "-C", other, "config", "commit.gpgsign", "false"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	readme := filepath.Join(local, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	if err := exec.Command("git", "-C", local, "add", ".").Run(); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := exec.Command("git", "-C", local, "commit", "-m", "initial").Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := exec.Command("git", "-C", local, "branch", "-M", "main").Run(); err != nil {
		t.Fatalf("failed to set main: %v", err)
	}
	if err := exec.Command("git", "-C", local, "remote", "add", "origin", remote).Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}
	if err := exec.Command("git", "-C", local, "push", "-u", "origin", "main").Run(); err != nil {
		t.Fatalf("failed to push: %v", err)
	}

	if err := exec.Command("git", "-C", other, "remote", "add", "origin", remote).Run(); err != nil {
		t.Fatalf("failed to add remote to other: %v", err)
	}
	if err := exec.Command("git", "-C", other, "fetch", "origin").Run(); err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}
	if err := exec.Command("git", "-C", other, "checkout", "-b", "main", "origin/main").Run(); err != nil {
		t.Fatalf("failed to checkout other: %v", err)
	}

	return local, other, func() {}
}

func TestSyncHelpers(t *testing.T) {
	localDir, otherDir, cleanup := setupRepoWithRemote(t)
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

	if err := syncTrunkWithRemote(repo, "main", true); err != nil {
		t.Fatalf("syncTrunkWithRemote failed: %v", err)
	}

	// Make remote ahead
	if err := os.WriteFile(filepath.Join(otherDir, "remote.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write remote file: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "add", ".").Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "commit", "-m", "remote commit").Run(); err != nil {
		t.Fatalf("failed to commit remote: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "push", "origin", "main").Run(); err != nil {
		t.Fatalf("failed to push remote: %v", err)
	}

	if err := repo.Fetch(); err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if err := syncTrunkWithRemote(repo, "main", true); err != nil {
		t.Fatalf("syncTrunkWithRemote fast-forward failed: %v", err)
	}

	// Add stale branch to metadata
	metadata.TrackBranch("stale-branch", "main", "")
	if err := cleanStaleBranches(repo, metadata, cfg, true); err != nil {
		t.Fatalf("cleanStaleBranches failed: %v", err)
	}

	// Create and merge a branch, then deleteMergedBranches should remove it
	if err := repo.CreateBranch("feat-merged"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-merged", "main", "")
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}
	if err := repo.CheckoutBranch("feat-merged"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "feat"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat-merged"); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}
	if err := deleteMergedBranches(repo, metadata, "main", true); err != nil {
		t.Fatalf("deleteMergedBranches failed: %v", err)
	}
}

func TestSyncTrunkWithRemoteConfirmSkip(t *testing.T) {
	localDir, otherDir, cleanup := setupRepoWithRemote(t)
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

	// Local commit
	if err := os.WriteFile(filepath.Join(localDir, "local.txt"), []byte("local"), 0644); err != nil {
		t.Fatalf("failed to write local file: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "local.txt"); err != nil {
		t.Fatalf("failed to add local file: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "local commit"); err != nil {
		t.Fatalf("failed to commit local: %v", err)
	}

	// Remote commit to diverge
	if err := os.WriteFile(filepath.Join(otherDir, "remote2.txt"), []byte("remote"), 0644); err != nil {
		t.Fatalf("failed to write remote file: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "add", ".").Run(); err != nil {
		t.Fatalf("failed to add remote: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "commit", "-m", "remote commit 2").Run(); err != nil {
		t.Fatalf("failed to commit remote: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "push", "origin", "main").Run(); err != nil {
		t.Fatalf("failed to push remote: %v", err)
	}

	if err := repo.Fetch(); err != nil {
		t.Fatalf("fetch failed: %v", err)
	}

	// Decline reset
	withReadKey('n', func() {
		if err := syncTrunkWithRemote(repo, "main", false); err != nil {
			t.Fatalf("syncTrunkWithRemote failed: %v", err)
		}
	})
}
