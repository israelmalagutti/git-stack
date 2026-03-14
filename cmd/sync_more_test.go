package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestRunSyncNoRemote(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync no remote failed: %v", err)
	}
}

func TestSyncTrunkWithRemoteNoRemoteBranch(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	if err := syncTrunkWithRemote(repo.repo, "main", true); err != nil {
		t.Fatalf("expected syncTrunkWithRemote to succeed with no remote branch, got %v", err)
	}
}

func TestCleanStaleBranchesPromptNo(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.metadata.TrackBranch("stale-branch", "main", "")
	if err := repo.metadata.Save(repo.repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	withReadKey('n', func() {
		if err := cleanStaleBranches(repo.repo, repo.metadata, repo.cfg, false); err != nil {
			t.Fatalf("cleanStaleBranches failed: %v", err)
		}
		if !repo.metadata.IsTracked("stale-branch") {
			t.Fatalf("expected stale branch to remain when user declines")
		}
	})
}

func TestDeleteMergedBranchesPromptQuit(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-merged-q", "main")
	if err := repo.repo.CheckoutBranch("feat-merged-q"); err != nil {
		t.Fatalf("failed to checkout branch: %v", err)
	}
	repo.commitFile(t, "q.txt", "data", "feat commit")
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("merge", "feat-merged-q"); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	withReadKey('q', func() {
		if err := deleteMergedBranches(repo.repo, repo.metadata, "main", false); err != nil {
			t.Fatalf("deleteMergedBranches quit failed: %v", err)
		}
	})
}

func TestRestackAllBranchesSuccess(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-restack", "main")
	repo.commitFile(t, "feat.txt", "feat", "feat commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "main.txt", "main", "main commit")

	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("failed to build stack: %v", err)
	}

	succeeded, failed := restackAllBranches(repo.repo, s, repo.metadata)
	if len(failed) != 0 {
		t.Fatalf("expected no restack failures, got %v", failed)
	}
	if len(succeeded) == 0 {
		t.Fatalf("expected restack success")
	}
}

func TestSyncTrunkWithRemoteForceReset(t *testing.T) {
	localDir, otherDir, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(origDir)
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
	if err := os.WriteFile(filepath.Join(otherDir, "remote.txt"), []byte("remote"), 0644); err != nil {
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
		t.Fatalf("syncTrunkWithRemote force reset failed: %v", err)
	}
}

func TestRunSyncInteractiveFlow(t *testing.T) {
	localDir, otherDir, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer os.Chdir(origDir)
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

	// Stale branch in metadata
	metadata.TrackBranch("stale-branch", "main", "")

	// Tracked branch merged into main
	if err := repo.CreateBranch("feat-merged"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-merged", "main", "")
	if err := repo.CheckoutBranch("feat-merged"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "feat merge"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat-merged"); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}

	// Tracked branch that needs restack
	if err := repo.CreateBranch("feat-restack"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-restack", "main", "")
	if err := repo.CheckoutBranch("feat-restack"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "feat restack"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "main moves"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// Diverge remote to force reset prompt
	if err := os.WriteFile(filepath.Join(otherDir, "remote.txt"), []byte("remote"), 0644); err != nil {
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

	// Stay on feature to exercise return-to-original branch
	if err := repo.CheckoutBranch("feat-restack"); err != nil {
		t.Fatalf("failed to checkout feat-restack: %v", err)
	}

	prevForce := syncForce
	prevRestack := syncRestack
	defer func() {
		syncForce = prevForce
		syncRestack = prevRestack
	}()

	syncForce = false
	syncRestack = true

	// y = reset trunk, y = clean stale, a = delete all merged
	withReadKeySequence([]byte{'y', 'y', 'a'}, func() {
		if err := runSync(nil, nil); err != nil {
			t.Fatalf("runSync interactive failed: %v", err)
		}
	})
}
