package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestHasUncommittedChangesDirect(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("clean working tree", func(t *testing.T) {
		has, err := repo.HasUncommittedChanges()
		if err != nil {
			t.Fatalf("HasUncommittedChanges failed: %v", err)
		}
		if has {
			t.Error("expected no uncommitted changes in clean repo")
		}
	})

	t.Run("staged changes detected", func(t *testing.T) {
		if err := os.WriteFile("staged.txt", []byte("staged"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if err := exec.Command("git", "add", "staged.txt").Run(); err != nil {
			t.Fatalf("git add failed: %v", err)
		}

		has, err := repo.HasUncommittedChanges()
		if err != nil {
			t.Fatalf("HasUncommittedChanges failed: %v", err)
		}
		if !has {
			t.Error("expected uncommitted changes with staged file")
		}

		// Clean up
		if err := exec.Command("git", "reset", "HEAD", "staged.txt").Run(); err != nil {
			t.Fatalf("git reset failed: %v", err)
		}
		_ = os.Remove("staged.txt")
	})

	t.Run("modified tracked file detected", func(t *testing.T) {
		if err := os.WriteFile("README.md", []byte("modified"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		has, err := repo.HasUncommittedChanges()
		if err != nil {
			t.Fatalf("HasUncommittedChanges failed: %v", err)
		}
		if !has {
			t.Error("expected uncommitted changes with modified tracked file")
		}

		// Clean up
		if err := exec.Command("git", "checkout", "--", "README.md").Run(); err != nil {
			t.Fatalf("git checkout failed: %v", err)
		}
	})
}

func TestIsBehindDirect(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Create feat at same point as main
	if err := repo.CreateBranch("feat-isbehind"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	t.Run("not behind when at same commit", func(t *testing.T) {
		behind, err := repo.IsBehind("feat-isbehind", "main")
		if err != nil {
			t.Fatalf("IsBehind failed: %v", err)
		}
		if behind {
			t.Error("expected feat not behind main when at same commit")
		}
	})

	t.Run("behind when parent advances", func(t *testing.T) {
		if err := exec.Command("git", "checkout", "main").Run(); err != nil {
			t.Fatalf("checkout failed: %v", err)
		}
		if err := os.WriteFile("advance.txt", []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if err := exec.Command("git", "add", "advance.txt").Run(); err != nil {
			t.Fatalf("git add failed: %v", err)
		}
		if err := exec.Command("git", "commit", "-m", "advance main").Run(); err != nil {
			t.Fatalf("git commit failed: %v", err)
		}

		behind, err := repo.IsBehind("feat-isbehind", "main")
		if err != nil {
			t.Fatalf("IsBehind failed: %v", err)
		}
		if !behind {
			t.Error("expected feat behind main after main advanced")
		}
	})
}

func TestIsMergedIntoDirect(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("not merged initially", func(t *testing.T) {
		if err := exec.Command("git", "checkout", "-b", "feat-merge", "main").Run(); err != nil {
			t.Fatalf("checkout -b failed: %v", err)
		}
		if err := os.WriteFile("merge.txt", []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}
		if err := exec.Command("git", "add", "merge.txt").Run(); err != nil {
			t.Fatalf("git add failed: %v", err)
		}
		if err := exec.Command("git", "commit", "-m", "feat-merge commit").Run(); err != nil {
			t.Fatalf("git commit failed: %v", err)
		}

		merged, err := repo.IsMergedInto("feat-merge", "main")
		if err != nil {
			t.Fatalf("IsMergedInto failed: %v", err)
		}
		if merged {
			t.Error("expected not merged before merge")
		}
	})

	t.Run("merged after squash merge", func(t *testing.T) {
		if err := exec.Command("git", "checkout", "main").Run(); err != nil {
			t.Fatalf("checkout failed: %v", err)
		}
		if err := exec.Command("git", "merge", "--squash", "feat-merge").Run(); err != nil {
			t.Fatalf("merge --squash failed: %v", err)
		}
		if err := exec.Command("git", "commit", "-m", "squash merge feat-merge").Run(); err != nil {
			t.Fatalf("git commit failed: %v", err)
		}

		merged, err := repo.IsMergedInto("feat-merge", "main")
		if err != nil {
			t.Fatalf("IsMergedInto failed: %v", err)
		}
		if !merged {
			t.Error("expected merged after squash merge")
		}
	})
}

func TestRunGitCommandWithStdin(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("successful stdin command", func(t *testing.T) {
		output, err := repo.RunGitCommandWithStdin("test content\n", "hash-object", "--stdin")
		if err != nil {
			t.Fatalf("RunGitCommandWithStdin failed: %v", err)
		}
		if len(output) != 40 {
			t.Errorf("expected 40-char SHA, got %d chars: %s", len(output), output)
		}
	})

	t.Run("failed stdin command", func(t *testing.T) {
		_, err := repo.RunGitCommandWithStdin("data", "invalid-command")
		if err == nil {
			t.Error("expected error for invalid command")
		}
	})
}

func TestNewRepoNotGitDir(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(origDir) }()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}

	_, err := NewRepo()
	if err == nil {
		t.Error("expected error for non-git directory")
	}
}

func TestGetCurrentBranchDetachedHead(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Get HEAD SHA and detach
	sha, err := repo.GetBranchCommit("main")
	if err != nil {
		t.Fatalf("GetBranchCommit failed: %v", err)
	}
	if err := exec.Command("git", "checkout", "--detach", sha).Run(); err != nil {
		t.Fatalf("detach HEAD failed: %v", err)
	}

	_, err = repo.GetCurrentBranch()
	if err == nil {
		t.Error("expected error for detached HEAD")
	}
}

func TestCanFastForwardDirect(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Create branch at same point -- can fast-forward
	if err := repo.CreateBranch("ff-test"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	// Add commit to main
	if err := os.WriteFile("ff.txt", []byte("ff"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "ff.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "advance main").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	canFF, err := repo.CanFastForward("ff-test", "main")
	if err != nil {
		t.Fatalf("CanFastForward failed: %v", err)
	}
	if !canFF {
		t.Error("expected ff-test can fast-forward to main")
	}

	// Now add a commit to ff-test, making it diverged
	if err := repo.CheckoutBranch("ff-test"); err != nil {
		t.Fatalf("CheckoutBranch failed: %v", err)
	}
	if err := os.WriteFile("diverge.txt", []byte("diverge"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "diverge.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "diverge").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	canFF, err = repo.CanFastForward("ff-test", "main")
	if err != nil {
		t.Fatalf("CanFastForward failed: %v", err)
	}
	if canFF {
		t.Error("expected ff-test cannot fast-forward to main after diverging")
	}
}

func TestRefOperations(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("WriteRef and ReadRef round-trip", func(t *testing.T) {
		data := []byte(`{"key":"value"}`)
		if err := repo.WriteRef("test/data", data); err != nil {
			t.Fatalf("WriteRef failed: %v", err)
		}

		got, err := repo.ReadRef("test/data")
		if err != nil {
			t.Fatalf("ReadRef failed: %v", err)
		}
		if string(got) != string(data) {
			t.Errorf("expected %q, got %q", string(data), string(got))
		}
	})

	t.Run("RefExists", func(t *testing.T) {
		if !repo.RefExists("test/data") {
			t.Error("expected ref to exist")
		}
		if repo.RefExists("nonexistent/ref") {
			t.Error("expected ref to not exist")
		}
	})

	t.Run("ListRefs", func(t *testing.T) {
		// Write another ref
		if err := repo.WriteRef("test/data2", []byte("more")); err != nil {
			t.Fatalf("WriteRef failed: %v", err)
		}

		refs, err := repo.ListRefs("test/")
		if err != nil {
			t.Fatalf("ListRefs failed: %v", err)
		}
		if len(refs) < 2 {
			t.Errorf("expected at least 2 refs, got %d: %v", len(refs), refs)
		}
	})

	t.Run("DeleteRef", func(t *testing.T) {
		if err := repo.DeleteRef("test/data2"); err != nil {
			t.Fatalf("DeleteRef failed: %v", err)
		}
		if repo.RefExists("test/data2") {
			t.Error("expected ref to be deleted")
		}
	})

	t.Run("ReadRef nonexistent fails", func(t *testing.T) {
		_, err := repo.ReadRef("nonexistent/ref")
		if err == nil {
			t.Error("expected error reading nonexistent ref")
		}
	})

	t.Run("ListRefs empty prefix", func(t *testing.T) {
		refs, err := repo.ListRefs("empty-prefix/")
		if err != nil {
			t.Fatalf("ListRefs failed: %v", err)
		}
		if len(refs) != 0 {
			t.Errorf("expected 0 refs, got %d", len(refs))
		}
	})

	t.Run("HasRemote", func(t *testing.T) {
		if repo.HasRemote("origin") {
			t.Error("expected no remote in test repo")
		}
	})
}

func TestHasRemoteWithRemote(t *testing.T) {
	base := t.TempDir()
	bareDir := filepath.Join(base, "remote.git")
	localDir := filepath.Join(base, "local")

	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}
	if err := exec.Command("git", "init", localDir).Run(); err != nil {
		t.Fatalf("failed to init local: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", localDir, "config", "user.email", "test@test.com"},
		{"git", "-C", localDir, "config", "user.name", "Test User"},
		{"git", "-C", localDir, "config", "commit.gpgsign", "false"},
		{"git", "-C", localDir, "remote", "add", "origin", bareDir},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}
	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", localDir, "add", "."},
		{"git", "-C", localDir, "commit", "-m", "initial"},
		{"git", "-C", localDir, "branch", "-M", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	if !repo.HasRemote("origin") {
		t.Error("expected HasRemote to return true for origin")
	}
	if repo.HasRemote("nonexistent") {
		t.Error("expected HasRemote to return false for nonexistent")
	}

	// Test HasRefspec - no gs refspec configured initially
	has, err := repo.HasRefspec("origin", "+refs/gs/*:refs/gs/*")
	if err != nil {
		t.Fatalf("HasRefspec failed: %v", err)
	}
	if has {
		t.Error("expected no gs refspec initially")
	}

	// ConfigureRefspec
	if err := repo.ConfigureRefspec("origin", "+refs/gs/*:refs/gs/*"); err != nil {
		t.Fatalf("ConfigureRefspec failed: %v", err)
	}
	has, err = repo.HasRefspec("origin", "+refs/gs/*:refs/gs/*")
	if err != nil {
		t.Fatalf("HasRefspec failed: %v", err)
	}
	if !has {
		t.Error("expected gs refspec after ConfigureRefspec")
	}

	// ConfigureRefspec again should be idempotent
	if err := repo.ConfigureRefspec("origin", "+refs/gs/*:refs/gs/*"); err != nil {
		t.Fatalf("ConfigureRefspec (idempotent) failed: %v", err)
	}
}

func TestResetToRemoteSameBranch(t *testing.T) {
	base := t.TempDir()
	bareDir := filepath.Join(base, "remote.git")
	localDir := filepath.Join(base, "local")

	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}
	if err := exec.Command("git", "init", localDir).Run(); err != nil {
		t.Fatalf("failed to init local: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", localDir, "config", "user.email", "test@test.com"},
		{"git", "-C", localDir, "config", "user.name", "Test User"},
		{"git", "-C", localDir, "config", "commit.gpgsign", "false"},
		{"git", "-C", localDir, "remote", "add", "origin", bareDir},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}
	if err := os.WriteFile(filepath.Join(localDir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", localDir, "add", "."},
		{"git", "-C", localDir, "commit", "-m", "initial"},
		{"git", "-C", localDir, "branch", "-M", "main"},
		{"git", "-C", localDir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("Chdir failed: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Reset main to origin/main while on main (currentBranch == branch)
	if err := repo.ResetToRemote("main", "origin/main"); err != nil {
		t.Fatalf("ResetToRemote failed: %v", err)
	}

	current, err := repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if current != "main" {
		t.Errorf("expected to still be on main, got %s", current)
	}
}

func TestRebaseOnto(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Record old main SHA (commit A)
	oldMainSHA, err := repo.GetBranchCommit("main")
	if err != nil {
		t.Fatalf("GetBranchCommit(main) failed: %v", err)
	}

	// Create branch feat from main with commit B
	if err := repo.CreateBranch("feat"); err != nil {
		t.Fatalf("CreateBranch(feat) failed: %v", err)
	}
	if err := repo.CheckoutBranch("feat"); err != nil {
		t.Fatalf("CheckoutBranch(feat) failed: %v", err)
	}
	if err := os.WriteFile("feat.txt", []byte("feat content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "feat.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "commit B on feat").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Go back to main, add commit C
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("CheckoutBranch(main) failed: %v", err)
	}
	if err := os.WriteFile("main2.txt", []byte("main content 2"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "main2.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "commit C on main").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create newbase from main (pointing at commit C)
	if err := repo.CreateBranch("newbase"); err != nil {
		t.Fatalf("CreateBranch(newbase) failed: %v", err)
	}
	newbaseSHA, err := repo.GetBranchCommit("newbase")
	if err != nil {
		t.Fatalf("GetBranchCommit(newbase) failed: %v", err)
	}

	// RebaseOnto: replay feat commits (since oldMainSHA) onto newbase
	if err := repo.RebaseOnto("feat", "newbase", oldMainSHA); err != nil {
		t.Fatalf("RebaseOnto failed: %v", err)
	}

	// Verify feat is now based on newbase
	mergeBase, err := repo.RunGitCommand("merge-base", "feat", "newbase")
	if err != nil {
		t.Fatalf("merge-base failed: %v", err)
	}
	if mergeBase != newbaseSHA {
		t.Errorf("expected feat to be based on newbase (%s), merge-base is %s", newbaseSHA, mergeBase)
	}

	// Verify feat.txt still exists on feat
	if err := repo.CheckoutBranch("feat"); err != nil {
		t.Fatalf("CheckoutBranch(feat) failed: %v", err)
	}
	if _, err := os.Stat("feat.txt"); os.IsNotExist(err) {
		t.Error("feat.txt should exist on feat after rebase")
	}
	// Verify main2.txt also present (from newbase)
	if _, err := os.Stat("main2.txt"); os.IsNotExist(err) {
		t.Error("main2.txt should exist on feat after rebase onto newbase")
	}
}

func TestGetRemoteURL(t *testing.T) {
	base := t.TempDir()
	bareDir := filepath.Join(base, "remote.git")
	localDir := filepath.Join(base, "local")

	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("failed to init bare repo: %v", err)
	}
	if err := exec.Command("git", "init", localDir).Run(); err != nil {
		t.Fatalf("failed to init local repo: %v", err)
	}

	for _, args := range [][]string{
		{"git", "-C", localDir, "config", "user.email", "test@test.com"},
		{"git", "-C", localDir, "config", "user.name", "Test User"},
		{"git", "-C", localDir, "config", "commit.gpgsign", "false"},
		{"git", "-C", localDir, "remote", "add", "origin", bareDir},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	// Need an initial commit for NewRepo to work
	readme := filepath.Join(localDir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", localDir, "add", "."},
		{"git", "-C", localDir, "commit", "-m", "initial"},
		{"git", "-C", localDir, "branch", "-M", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	origDir, _ := os.Getwd()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	t.Run("returns URL for existing remote", func(t *testing.T) {
		url, err := repo.GetRemoteURL("origin")
		if err != nil {
			t.Fatalf("GetRemoteURL(origin) failed: %v", err)
		}
		if url != bareDir {
			t.Errorf("expected %q, got %q", bareDir, url)
		}
	})

	t.Run("returns error for nonexistent remote", func(t *testing.T) {
		_, err := repo.GetRemoteURL("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent remote")
		}
	})
}

func TestGetContinueStatePath(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	path := repo.GetContinueStatePath()
	if !strings.Contains(path, ".gs_continue_state") {
		t.Errorf("expected path containing .gs_continue_state, got %q", path)
	}
}

func TestRebase(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Create branch feat from main
	if err := repo.CreateBranch("feat"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}
	if err := repo.CheckoutBranch("feat"); err != nil {
		t.Fatalf("CheckoutBranch failed: %v", err)
	}
	if err := os.WriteFile("feat.txt", []byte("feat"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "feat.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "feat commit").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Add a commit to main
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("CheckoutBranch(main) failed: %v", err)
	}
	if err := os.WriteFile("main2.txt", []byte("main2"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "main2.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "main commit 2").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	mainSHA, err := repo.GetBranchCommit("main")
	if err != nil {
		t.Fatalf("GetBranchCommit failed: %v", err)
	}

	// Rebase feat onto main
	if err := repo.Rebase("feat", "main"); err != nil {
		t.Fatalf("Rebase failed: %v", err)
	}

	// Verify merge-base of feat and main is now main's tip
	mergeBase, err := repo.RunGitCommand("merge-base", "feat", "main")
	if err != nil {
		t.Fatalf("merge-base failed: %v", err)
	}
	if mergeBase != mainSHA {
		t.Errorf("expected merge-base %s, got %s", mainSHA, mergeBase)
	}

	// Verify feat.txt still exists
	if err := repo.CheckoutBranch("feat"); err != nil {
		t.Fatalf("CheckoutBranch(feat) failed: %v", err)
	}
	if _, err := os.Stat("feat.txt"); os.IsNotExist(err) {
		t.Error("feat.txt should still exist after rebase")
	}
}

func TestAbortRebase(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Create a conflict scenario
	// main: write "main content" to conflict.txt
	if err := os.WriteFile("conflict.txt", []byte("main content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "conflict.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "main: add conflict.txt").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create feat branch from initial commit (before conflict.txt)
	// First get initial commit SHA
	initialSHA, err := repo.RunGitCommand("rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		t.Fatalf("rev-list failed: %v", err)
	}

	if err := exec.Command("git", "checkout", "-b", "feat-conflict", initialSHA).Run(); err != nil {
		t.Fatalf("checkout -b failed: %v", err)
	}

	// Write conflicting content to the same file on feat
	if err := os.WriteFile("conflict.txt", []byte("feat content"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	if err := exec.Command("git", "add", "conflict.txt").Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "feat: add conflict.txt").Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	featSHA, err := repo.GetBranchCommit("feat-conflict")
	if err != nil {
		t.Fatalf("GetBranchCommit failed: %v", err)
	}

	// Attempt rebase — should fail due to conflict
	rebaseErr := repo.Rebase("feat-conflict", "main")
	if rebaseErr == nil {
		t.Fatal("expected rebase to fail due to conflict")
	}

	// Abort the rebase
	if err := repo.AbortRebase(); err != nil {
		t.Fatalf("AbortRebase failed: %v", err)
	}

	// Verify feat-conflict is back to its original SHA
	currentSHA, err := repo.GetBranchCommit("feat-conflict")
	if err != nil {
		t.Fatalf("GetBranchCommit after abort failed: %v", err)
	}
	if currentSHA != featSHA {
		t.Errorf("expected feat-conflict to be at %s after abort, got %s", featSHA, currentSHA)
	}

	// Verify we can still check out the branch normally
	if err := repo.CheckoutBranch("feat-conflict"); err != nil {
		t.Fatalf("CheckoutBranch after abort failed: %v", err)
	}
	current, err := repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("GetCurrentBranch failed: %v", err)
	}
	if current != "feat-conflict" {
		t.Errorf("expected current branch feat-conflict, got %s", current)
	}
}

func TestResetToRemoteDetachedHead(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Detach HEAD so GetCurrentBranch fails inside ResetToRemote
	sha, err := repo.GetBranchCommit("main")
	if err != nil {
		t.Fatalf("GetBranchCommit failed: %v", err)
	}
	if err := exec.Command("git", "checkout", "--detach", sha).Run(); err != nil {
		t.Fatalf("detach HEAD failed: %v", err)
	}

	err = repo.ResetToRemote("main", "origin/main")
	if err == nil {
		t.Error("expected error when HEAD is detached")
	}
}

func TestHasUncommittedChangesError(t *testing.T) {
	badRepo := &Repo{workDir: "/nonexistent-dir-for-test"}
	_, err := badRepo.HasUncommittedChanges()
	if err == nil {
		t.Error("expected error for HasUncommittedChanges with invalid workDir")
	}
}

func TestIsBehindError(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Test error when second rev-parse fails (parent doesn't exist)
	// Create a branch so merge-base succeeds but parent rev-parse would fail
	// Actually, merge-base itself will fail if parent doesn't exist, which is already tested.
	// Instead, test the specific case where merge-base succeeds but rev-parse of parent fails.
	// This is hard to trigger, so let's just verify the error message content.
	_, err = repo.IsBehind("main", "nonexistent-branch")
	if err == nil {
		t.Error("expected error for IsBehind with nonexistent parent")
	}
}

func TestCanFastForwardError(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Test when local branch doesn't exist
	_, err = repo.CanFastForward("nonexistent-branch", "main")
	if err == nil {
		t.Error("expected error for CanFastForward with nonexistent branch")
	}
}

func TestWriteRefError(t *testing.T) {
	badRepo := &Repo{workDir: "/nonexistent-dir-for-test"}
	err := badRepo.WriteRef("test/error", []byte("data"))
	if err == nil {
		t.Error("expected error for WriteRef with invalid workDir")
	}
}

func TestDeleteRefError(t *testing.T) {
	badRepo := &Repo{workDir: "/nonexistent-dir-for-test"}
	err := badRepo.DeleteRef("test/error")
	if err == nil {
		t.Error("expected error for DeleteRef with invalid workDir")
	}
}

func TestListRefsError(t *testing.T) {
	badRepo := &Repo{workDir: "/nonexistent-dir-for-test"}
	_, err := badRepo.ListRefs("test/")
	if err == nil {
		t.Error("expected error for ListRefs with invalid workDir")
	}
}

func TestPushRefsError(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// No remote configured, so push should fail
	err = repo.PushRefs("nonexistent-remote", "refs/gs/test:refs/gs/test")
	if err == nil {
		t.Error("expected error for PushRefs to nonexistent remote")
	}
}

func TestFetchRefsError(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// No remote configured, so fetch should fail
	err = repo.FetchRefs("nonexistent-remote", "+refs/gs/*:refs/gs/*")
	if err == nil {
		t.Error("expected error for FetchRefs from nonexistent remote")
	}
}

func TestDeleteRemoteRefError(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// No remote configured, so delete should fail
	err = repo.DeleteRemoteRef("nonexistent-remote", "test/ref")
	if err == nil {
		t.Error("expected error for DeleteRemoteRef on nonexistent remote")
	}
}

func TestConfigureRefspecError(t *testing.T) {
	badRepo := &Repo{workDir: "/nonexistent-dir-for-test"}
	err := badRepo.ConfigureRefspec("origin", "+refs/gs/*:refs/gs/*")
	if err == nil {
		t.Error("expected error for ConfigureRefspec with invalid workDir")
	}
}

func TestIsMergedIntoErrorPath(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	// Test with nonexistent target - git branch --merged nonexistent should fail
	_, err = repo.IsMergedInto("main", "nonexistent-target")
	if err == nil {
		t.Error("expected error for IsMergedInto with nonexistent target")
	}
}

func TestRebaseOntoError(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	err = repo.RebaseOnto("nonexistent", "main", "HEAD~1")
	if err == nil {
		t.Error("expected error for RebaseOnto with nonexistent branch")
	}
}

func TestRebaseError(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("NewRepo failed: %v", err)
	}

	err = repo.Rebase("nonexistent-branch", "main")
	if err == nil {
		t.Error("expected error for Rebase with nonexistent branch")
	}
}
