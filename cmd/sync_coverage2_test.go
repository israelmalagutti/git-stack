package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func TestReadKeyDirectCoverage(t *testing.T) {
	// Test the readKey function directly via readKeyFn override
	// The real readKey uses term.MakeRaw which fails in tests (non-terminal),
	// so it falls back to os.Stdin.Read. Test that fallback path.
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatalf("write: %v", err)
	}
	_ = w.Close()
	os.Stdin = r

	// Call the real readKey (not the overridden readKeyFn)
	b, err := readKey()
	if err != nil {
		t.Fatalf("readKey error: %v", err)
	}
	if b != 'x' {
		t.Errorf("expected 'x', got %c", b)
	}
}

func TestReadKeyReadError(t *testing.T) {
	origStdin := os.Stdin
	defer func() { os.Stdin = origStdin }()

	// Create a closed pipe to trigger read error
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	_ = w.Close()
	_ = r.Close()
	os.Stdin = r

	_, err = readKey()
	if err == nil {
		t.Fatal("expected read error")
	}
}

func TestSyncTrunkWithRemoteFastForwardFromNonTrunk(t *testing.T) {
	// Test fast-forward when current branch is NOT trunk
	// This exercises the checkout trunk -> ff -> return to original branch path
	localDir, otherDir, cleanup := setupRepoWithRemote(t)
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

	// Create a feature branch so we're not on trunk
	if err := repo.CreateBranch("feat-ff"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.CheckoutBranch("feat-ff"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Make remote ahead
	if err := os.WriteFile(filepath.Join(otherDir, "ff.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "add", ".").Run(); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "commit", "-m", "ff commit").Run(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "push", "origin", "main").Run(); err != nil {
		t.Fatalf("push: %v", err)
	}

	if err := repo.Fetch(); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	// Sync trunk while on feat-ff branch
	if err := syncTrunkWithRemote(repo, "main", true); err != nil {
		t.Fatalf("syncTrunkWithRemote from non-trunk failed: %v", err)
	}

	// Should still be on feat-ff
	current, _ := repo.GetCurrentBranch()
	if current != "feat-ff" {
		t.Errorf("expected to return to feat-ff, got %s", current)
	}
}

func TestSyncTrunkWithRemoteResetConfirmYes(t *testing.T) {
	// Test the diverged trunk with user confirming reset (non-force mode)
	localDir, otherDir, cleanup := setupRepoWithRemote(t)
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

	// Local diverging commit
	if err := os.WriteFile(filepath.Join(localDir, "local-div.txt"), []byte("local"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "local diverge"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Remote diverging commit
	if err := os.WriteFile(filepath.Join(otherDir, "remote-div.txt"), []byte("remote"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "add", ".").Run(); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "commit", "-m", "remote diverge").Run(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "push", "origin", "main").Run(); err != nil {
		t.Fatalf("push: %v", err)
	}

	if err := repo.Fetch(); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	// User confirms reset
	withReadKey('y', func() {
		if err := syncTrunkWithRemote(repo, "main", false); err != nil {
			t.Fatalf("syncTrunkWithRemote reset confirm failed: %v", err)
		}
	})
}

func TestDeleteMergedBranchesYesAndNo(t *testing.T) {
	// Test the "yes" and "no" individual prompt paths (non-force, non-all)
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Create and merge two branches
	repo.createBranch(t, "feat-yes", "main")
	if err := repo.repo.CheckoutBranch("feat-yes"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	repo.commitFile(t, "y.txt", "data", "y commit")
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("merge", "feat-yes"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// 'y' for first (and only) branch
	withReadKey('y', func() {
		if err := deleteMergedBranches(repo.repo, repo.metadata, "main", false); err != nil {
			t.Fatalf("deleteMergedBranches yes failed: %v", err)
		}
	})

	// Verify deleted
	if repo.repo.BranchExists("feat-yes") {
		t.Fatal("expected feat-yes to be deleted")
	}
}

func TestDeleteMergedBranchesNo(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-no", "main")
	if err := repo.repo.CheckoutBranch("feat-no"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	repo.commitFile(t, "n.txt", "data", "n commit")
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("merge", "feat-no"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// 'n' for first branch
	withReadKey('n', func() {
		if err := deleteMergedBranches(repo.repo, repo.metadata, "main", false); err != nil {
			t.Fatalf("deleteMergedBranches no failed: %v", err)
		}
	})

	// Branch should still exist
	if !repo.repo.BranchExists("feat-no") {
		t.Fatal("expected feat-no to still exist")
	}
}

func TestDeleteMergedBranchesForceDeleteError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Track a branch that's merged but we're currently on it (can't delete current branch)
	repo.createBranch(t, "feat-curr-merged", "main")
	if err := repo.repo.CheckoutBranch("feat-curr-merged"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	repo.commitFile(t, "curr.txt", "data", "curr commit")

	// Merge into main
	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("merge", "feat-curr-merged"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Checkout the merged branch (can't delete current branch)
	if err := repo.repo.CheckoutBranch("feat-curr-merged"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Force delete - should report failure for current branch
	err := deleteMergedBranches(repo.repo, repo.metadata, "main", true)
	// Should not error overall (it prints the failure but continues)
	if err != nil {
		t.Fatalf("deleteMergedBranches force error: %v", err)
	}
}

func TestRunSyncSyncErrors(t *testing.T) {
	// Test sync with various error paths

	// Test with repo that has remote but sync trunk fails
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

	prevForce := syncForce
	prevRestack := syncRestack
	prevDelete := syncDelete
	defer func() {
		syncForce = prevForce
		syncRestack = prevRestack
		syncDelete = prevDelete
	}()

	// noRestack sync
	syncForce = true
	syncRestack = false
	syncDelete = false

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync noRestack failed: %v", err)
	}
}

func TestSyncTrunkWithRemoteGetCommitErrors(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	// Add a remote but with no remote tracking branch set up
	if _, err := repo.repo.RunGitCommand("remote", "add", "origin", "/tmp/nonexistent-bare"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	// No remote branch => "no remote tracking branch" path
	if err := syncTrunkWithRemote(repo.repo, "main", true); err != nil {
		t.Fatalf("syncTrunkWithRemote no remote branch: %v", err)
	}
}

func TestConfirmWithReadKeyError(t *testing.T) {
	prev := readKeyFn
	readKeyFn = func() (byte, error) {
		return 0, fmt.Errorf("read error")
	}
	defer func() { readKeyFn = prev }()

	if confirm() {
		t.Error("expected false on error")
	}
	if got := confirmWithOptions(); got != "no" {
		t.Errorf("expected 'no' on error, got %q", got)
	}
}
