package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

type cmdTestRepo struct {
	repo     *git.Repo
	cfg      *config.Config
	metadata *config.Metadata
	dir      string
	cleanup  func()
}

func setupCmdTestRepo(t *testing.T) *cmdTestRepo {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
	}

	cmds := [][]string{
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
		{"git", "-C", dir, "config", "commit.gpgsign", "false"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("failed to run %v: %v", args, err)
		}
	}

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "add", ".").Run(); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "Initial commit").Run(); err != nil {
		t.Fatalf("failed to git commit: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "branch", "-M", "main").Run(); err != nil {
		t.Fatalf("failed to rename branch: %v", err)
	}

	if err := os.Chdir(dir); err != nil {
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

	cleanup := func() {
		if err := os.Chdir(origDir); err != nil {
			t.Errorf("failed to restore cwd: %v", err)
		}
	}

	return &cmdTestRepo{
		repo:     repo,
		cfg:      cfg,
		metadata: metadata,
		dir:      dir,
		cleanup:  cleanup,
	}
}

func (r *cmdTestRepo) createBranch(t *testing.T, name, parent string) {
	t.Helper()
	if err := r.repo.CheckoutBranch(parent); err != nil {
		t.Fatalf("failed to checkout %s: %v", parent, err)
	}
	if err := r.repo.CreateBranch(name); err != nil {
		t.Fatalf("failed to create %s: %v", name, err)
	}
	if err := r.repo.CheckoutBranch(name); err != nil {
		t.Fatalf("failed to checkout %s: %v", name, err)
	}
	r.metadata.TrackBranch(name, parent, "")
	if err := r.metadata.Save(r.repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}
}

func (r *cmdTestRepo) commitFile(t *testing.T, filename, contents, message string) {
	t.Helper()
	path := filepath.Join(r.dir, filename)
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if _, err := r.repo.RunGitCommand("add", filename); err != nil {
		t.Fatalf("failed to add file: %v", err)
	}
	if _, err := r.repo.RunGitCommand("commit", "-m", message); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}

func writeFailingHook(t *testing.T, repoDir, hookName string) {
	t.Helper()

	hooksDir := filepath.Join(repoDir, ".git", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}

	hookPath := filepath.Join(hooksDir, hookName)
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\nexit 1\n"), 0755); err != nil {
		t.Fatalf("failed to write hook: %v", err)
	}
}
