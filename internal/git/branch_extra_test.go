package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupRemoteRepos(t *testing.T) (string, string, func()) {
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

	return local, remote, func() {}
}

func TestRemoteOperations(t *testing.T) {
	localDir, _, cleanup := setupRemoteRepos(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("failed to chdir: %v", err)
	}

	repo, err := NewRepo()
	if err != nil {
		t.Fatalf("failed to open repo: %v", err)
	}

	if repo.GetGitDir() == "" || repo.GetCommonDir() == "" {
		t.Fatalf("expected git dirs to be populated")
	}

	if err := repo.Fetch(); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if !repo.HasRemoteBranch("main", "origin") {
		t.Fatalf("expected remote branch")
	}

	// Create a new commit on origin from another repo to test fast-forward
	otherDir := filepath.Join(filepath.Dir(localDir), "other")
	if err := os.WriteFile(filepath.Join(otherDir, "other.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write in other: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "add", ".").Run(); err != nil {
		t.Fatalf("failed to add in other: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "commit", "-m", "other commit").Run(); err != nil {
		t.Fatalf("failed to commit in other: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "push", "origin", "main").Run(); err != nil {
		t.Fatalf("failed to push from other: %v", err)
	}

	if err := repo.Fetch(); err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	canFF, err := repo.CanFastForward("main", "origin/main")
	if err != nil || !canFF {
		t.Fatalf("expected fast-forward possible, got %v (%v)", canFF, err)
	}

	// Reset while on a different branch to cover checkout/return paths
	if err := repo.CreateBranch("temp"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.CheckoutBranch("temp"); err != nil {
		t.Fatalf("failed to checkout temp: %v", err)
	}
	if err := repo.ResetToRemote("main", "origin/main"); err != nil {
		t.Fatalf("ResetToRemote failed: %v", err)
	}

	behind, err := repo.IsBehind("main", "origin/main")
	if err != nil {
		t.Fatalf("IsBehind failed: %v", err)
	}
	if behind {
		t.Fatalf("expected main not behind origin/main after reset")
	}

	if err := repo.Rebase("main", "origin/main"); err != nil {
		t.Fatalf("Rebase failed: %v", err)
	}

	_ = repo.AbortRebase()

	// IsBehind should be true when parent moves ahead
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := repo.CreateBranch("feat-behind"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.CheckoutBranch("feat-behind"); err != nil {
		t.Fatalf("failed to checkout feat: %v", err)
	}
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "parent moves"); err != nil {
		t.Fatalf("failed to commit on main: %v", err)
	}
	behind, err = repo.IsBehind("feat-behind", "main")
	if err != nil {
		t.Fatalf("IsBehind failed: %v", err)
	}
	if !behind {
		t.Fatalf("expected feat-behind to be behind main")
	}

	// Create a feature branch and merge to test IsMergedInto
	if err := repo.CreateBranch("feat"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	if err := repo.CheckoutBranch("feat"); err != nil {
		t.Fatalf("failed to checkout: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "--allow-empty", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if _, err := repo.RunGitCommand("merge", "feat"); err != nil {
		t.Fatalf("failed to merge: %v", err)
	}
	merged, err := repo.IsMergedInto("feat", "main")
	if err != nil || !merged {
		t.Fatalf("expected feat merged into main, got %v (%v)", merged, err)
	}
}
