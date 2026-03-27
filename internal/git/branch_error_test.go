package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupSimpleRepo(t *testing.T) (string, func()) {
	t.Helper()
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.email", "test@test.com").Run(); err != nil {
		t.Fatalf("failed to config email: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run(); err != nil {
		t.Fatalf("failed to config name: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "config", "commit.gpgsign", "false").Run(); err != nil {
		t.Fatalf("failed to config gpgsign: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to write readme: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "add", ".").Run(); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "init").Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "branch", "-M", "main").Run(); err != nil {
		t.Fatalf("failed to set main: %v", err)
	}
	return dir, func() {}
}

func TestBranchErrorPaths(t *testing.T) {
	dir, cleanup := setupSimpleRepo(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	// CreateBranch error on existing branch
	if err := repo.CreateBranch("main"); err == nil {
		t.Fatalf("expected create branch error")
	}

	// DeleteBranch error on missing branch
	if err := repo.DeleteBranch("missing", true); err == nil {
		t.Fatalf("expected delete branch error")
	}

	// GetBranchCommit error on missing branch
	if _, err := repo.GetBranchCommit("missing"); err == nil {
		t.Fatalf("expected get branch commit error")
	}

	// Detached HEAD should error in GetCurrentBranch
	if _, err := repo.RunGitCommand("checkout", "--detach"); err != nil {
		t.Fatalf("failed to detach: %v", err)
	}
	if _, err := repo.GetCurrentBranch(); err == nil {
		t.Fatalf("expected error for detached HEAD")
	}
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	// Fetch should not crash when no remotes are configured
	if err := repo.Fetch(); err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}

	// CanFastForward error on missing refs
	if _, err := repo.CanFastForward("main", "origin/main"); err == nil {
		t.Fatalf("expected fast-forward error")
	}

	// ResetToRemote error on missing remote ref
	if err := repo.ResetToRemote("main", "origin/main"); err == nil {
		t.Fatalf("expected reset error")
	}

	// IsMergedInto error on missing target
	if _, err := repo.IsMergedInto("main", "missing"); err == nil {
		t.Fatalf("expected merged check error")
	}

	// IsBehind error on missing parent
	if _, err := repo.IsBehind("main", "missing"); err == nil {
		t.Fatalf("expected is-behind error")
	}

	// Run in non-git directory to cover command failures
	nonRepo := t.TempDir()
	if err := os.Chdir(nonRepo); err != nil {
		t.Fatalf("failed to chdir non-repo: %v", err)
	}
	if _, err := repo.ListBranches(); err == nil {
		t.Fatalf("expected list branches error outside repo")
	}
}

func TestNewRepoOutsideGit(t *testing.T) {
	dir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	if _, err := NewRepo(); err == nil {
		t.Fatalf("expected error outside git repo")
	}
}

func TestEmptyRepoBranches(t *testing.T) {
	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("failed to init repo: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	branches, err := repo.ListBranches()
	if err != nil {
		t.Fatalf("ListBranches failed: %v", err)
	}
	if len(branches) != 0 {
		t.Fatalf("expected no branches, got %v", branches)
	}

	_, _ = repo.GetCurrentBranch()

	_ = repo.Fetch()
}

func TestFetchOutsideRepo(t *testing.T) {
	dir, cleanup := setupSimpleRepo(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	nonRepo := t.TempDir()
	if err := os.Chdir(nonRepo); err != nil {
		t.Fatalf("failed to chdir non-repo: %v", err)
	}

	if err := repo.Fetch(); err == nil {
		t.Fatalf("expected fetch error outside repo")
	}
}

func TestResetToRemoteMissingBranch(t *testing.T) {
	dir, cleanup := setupSimpleRepo(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	if err := repo.ResetToRemote("missing", "origin/missing"); err == nil {
		t.Fatalf("expected reset error for missing branch")
	}
}
