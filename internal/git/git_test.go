package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "gw-git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	origDir, err := os.Getwd()
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to get current dir: %v", err)
	}

	if err := os.Chdir(tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to change to temp dir: %v", err)
	}

	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after init failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to init git repo: %v", err)
	}

	if err := exec.Command("git", "config", "user.email", "test@test.com").Run(); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after git config failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to set git user.email: %v", err)
	}
	if err := exec.Command("git", "config", "user.name", "Test User").Run(); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after git config failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to set git user.name: %v", err)
	}
	if err := exec.Command("git", "config", "commit.gpgsign", "false").Run(); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after git config failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to disable git gpg signing: %v", err)
	}

	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test"), 0644); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after write failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := exec.Command("git", "add", ".").Run(); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after git add failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to git add: %v", err)
	}
	if err := exec.Command("git", "commit", "-m", "Initial commit").Run(); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after git commit failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to git commit: %v", err)
	}
	if err := exec.Command("git", "branch", "-M", "main").Run(); err != nil {
		if chdirErr := os.Chdir(origDir); chdirErr != nil {
			t.Fatalf("failed to restore dir after git branch failure: %v", chdirErr)
		}
		os.RemoveAll(tmpDir)
		t.Fatalf("failed to rename branch to main: %v", err)
	}

	cleanup := func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore dir: %v", err)
		}
		os.RemoveAll(tmpDir)
	}

	return tmpDir, cleanup
}

func TestIsGitRepo(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	t.Run("returns true in git repo", func(t *testing.T) {
		if !IsGitRepo() {
			t.Error("expected true in git repo")
		}
	})
}

func TestNewRepo(t *testing.T) {
	tmpDir, cleanup := setupTestRepo(t)
	defer cleanup()

	t.Run("creates repo instance", func(t *testing.T) {
		repo, err := NewRepo()
		if err != nil {
			t.Fatalf("NewRepo failed: %v", err)
		}

		if repo.GetWorkDir() != tmpDir {
			t.Errorf("expected workdir '%s', got '%s'", tmpDir, repo.GetWorkDir())
		}
	})
}

func TestGetCurrentBranch(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("returns current branch", func(t *testing.T) {
		branch, err := repo.GetCurrentBranch()
		if err != nil {
			t.Fatalf("GetCurrentBranch failed: %v", err)
		}

		if branch != "main" {
			t.Errorf("expected 'main', got '%s'", branch)
		}
	})
}

func TestListBranches(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("lists all branches", func(t *testing.T) {
		if err := repo.CreateBranch("feat-1"); err != nil {
			t.Fatalf("CreateBranch feat-1 failed: %v", err)
		}
		if err := repo.CreateBranch("feat-2"); err != nil {
			t.Fatalf("CreateBranch feat-2 failed: %v", err)
		}

		branches, err := repo.ListBranches()
		if err != nil {
			t.Fatalf("ListBranches failed: %v", err)
		}

		if len(branches) != 3 {
			t.Errorf("expected 3 branches, got %d: %v", len(branches), branches)
		}
	})
}

func TestBranchExists(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("returns true for existing branch", func(t *testing.T) {
		if !repo.BranchExists("main") {
			t.Error("expected true for 'main'")
		}
	})

	t.Run("returns false for non-existing branch", func(t *testing.T) {
		if repo.BranchExists("nonexistent") {
			t.Error("expected false for 'nonexistent'")
		}
	})
}

func TestCreateBranch(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("creates new branch", func(t *testing.T) {
		err := repo.CreateBranch("new-branch")
		if err != nil {
			t.Fatalf("CreateBranch failed: %v", err)
		}

		if !repo.BranchExists("new-branch") {
			t.Error("branch should exist after creation")
		}
	})
}

func TestCheckoutBranch(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()
	if err := repo.CreateBranch("feat-checkout"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	t.Run("switches to branch", func(t *testing.T) {
		err := repo.CheckoutBranch("feat-checkout")
		if err != nil {
			t.Fatalf("CheckoutBranch failed: %v", err)
		}

		current, _ := repo.GetCurrentBranch()
		if current != "feat-checkout" {
			t.Errorf("expected 'feat-checkout', got '%s'", current)
		}
	})

	t.Run("fails for non-existing branch", func(t *testing.T) {
		err := repo.CheckoutBranch("nonexistent")
		if err == nil {
			t.Error("expected error for nonexistent branch")
		}
	})
}

func TestDeleteBranch(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()
	if err := repo.CreateBranch("to-delete"); err != nil {
		t.Fatalf("CreateBranch failed: %v", err)
	}

	t.Run("deletes branch", func(t *testing.T) {
		err := repo.DeleteBranch("to-delete", true)
		if err != nil {
			t.Fatalf("DeleteBranch failed: %v", err)
		}

		if repo.BranchExists("to-delete") {
			t.Error("branch should not exist after deletion")
		}
	})
}

func TestGetBranchCommit(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("returns commit SHA", func(t *testing.T) {
		sha, err := repo.GetBranchCommit("main")
		if err != nil {
			t.Fatalf("GetBranchCommit failed: %v", err)
		}

		if len(sha) != 40 {
			t.Errorf("expected 40 char SHA, got %d: %s", len(sha), sha)
		}
	})
}

func TestRunGitCommand(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("runs git command successfully", func(t *testing.T) {
		output, err := repo.RunGitCommand("status", "--short")
		if err != nil {
			t.Fatalf("RunGitCommand failed: %v", err)
		}

		if output != "" {
			t.Errorf("expected empty output, got '%s'", output)
		}
	})

	t.Run("returns error for invalid command", func(t *testing.T) {
		_, err := repo.RunGitCommand("invalid-command")
		if err == nil {
			t.Error("expected error for invalid command")
		}
	})
}

func TestGetConfigPath(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("returns config path", func(t *testing.T) {
		path := repo.GetConfigPath()
		if !strings.HasSuffix(path, ".gs_config") {
			t.Errorf("expected path ending with .gs_config, got '%s'", path)
		}
	})
}

func TestGetMetadataPath(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	repo, _ := NewRepo()

	t.Run("returns metadata path", func(t *testing.T) {
		path := repo.GetMetadataPath()
		if !strings.HasSuffix(path, ".gs_stack_metadata") {
			t.Errorf("expected path ending with .gs_stack_metadata, got '%s'", path)
		}
	})
}
