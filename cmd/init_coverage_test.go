package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestRunInit_WithRemoteAndExistingRefs(t *testing.T) {
	// Test the remote-aware init path where remote already has gs config
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	// Create a bare remote
	bareDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	// Create a "source" repo, set up gs config, push refs
	srcDir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", srcDir},
		{"git", "-C", srcDir, "config", "user.email", "test@test.com"},
		{"git", "-C", srcDir, "config", "user.name", "Test User"},
		{"git", "-C", srcDir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup src: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write readme: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", srcDir, "add", "."},
		{"git", "-C", srcDir, "commit", "-m", "Init"},
		{"git", "-C", srcDir, "branch", "-M", "main"},
		{"git", "-C", srcDir, "remote", "add", "origin", bareDir},
		{"git", "-C", srcDir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("push src: %v", err)
		}
	}

	// Update bare repo HEAD to point to main (git init --bare defaults to master)
	if err := exec.Command("git", "-C", bareDir, "symbolic-ref", "HEAD", "refs/heads/main").Run(); err != nil {
		t.Fatalf("set HEAD: %v", err)
	}

	// Now clone into a fresh dir (no gs files)
	cloneDir := filepath.Join(t.TempDir(), "clone")
	if err := exec.Command("git", "clone", bareDir, cloneDir).Run(); err != nil {
		t.Fatalf("clone: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", cloneDir, "config", "user.email", "test@test.com"},
		{"git", "-C", cloneDir, "config", "user.name", "Test User"},
		{"git", "-C", cloneDir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("config clone: %v", err)
		}
	}

	if err := os.Chdir(cloneDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// runInit should prompt for trunk, then create config
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit with remote failed: %v", err)
		}
	})

	// Verify config was saved
	if !config.IsInitialized(filepath.Join(cloneDir, ".git", ".gs_config")) {
		t.Error("expected gs config to be created")
	}
}

func TestRunInit_MigrationAndAlreadyInit(t *testing.T) {
	// Test the path where migration from gw succeeds and config already exists
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	commonDir := tr.repo.GetCommonDir()

	// Create gw legacy files (migration path)
	for _, f := range []string{".gw_config", ".gw_stack_metadata"} {
		if err := os.WriteFile(filepath.Join(commonDir, f), []byte("legacy"), 0644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	// gs files already exist from setupCmdTestRepo, so migration removes gw files
	// but then "already initialized" is detected (with migrated = true, prints paths)
	initReset = false
	defer func() { initReset = false }()

	err := runInit(nil, nil)
	// When migrated=true and already initialized, it should return nil (not error)
	if err != nil {
		t.Fatalf("runInit after migration failed: %v", err)
	}
}

func TestRunInit_ResetWithRemote(t *testing.T) {
	// Test reset followed by re-init in a repo with a remote
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()

	bareDir := t.TempDir()
	if err := exec.Command("git", "init", "--bare", bareDir).Run(); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
		{"git", "-C", dir, "config", "commit.gpgsign", "false"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "Init"},
		{"git", "-C", dir, "branch", "-M", "main"},
		{"git", "-C", dir, "remote", "add", "origin", bareDir},
		{"git", "-C", dir, "push", "-u", "origin", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("push: %v", err)
		}
	}

	// Create gs config first
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(filepath.Join(dir, ".git", ".gs_config")); err != nil {
		t.Fatalf("save config: %v", err)
	}

	// Reset and re-init
	initReset = true
	defer func() { initReset = false }()

	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit reset+remote failed: %v", err)
		}
	})
}
