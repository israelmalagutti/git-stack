package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

func setupTestRepo(t *testing.T) (*git.Repo, *config.Config, *config.Metadata) {
	t.Helper()

	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test User"},
		{"git", "-C", dir, "config", "commit.gpgsign", "false"},
	}
	for _, args := range cmds {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup %v: %v", args, err)
		}
	}

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	for _, args := range [][]string{
		{"git", "-C", dir, "add", "."},
		{"git", "-C", dir, "commit", "-m", "Initial commit"},
		{"git", "-C", dir, "branch", "-M", "main"},
	} {
		if err := exec.Command(args[0], args[1:]...).Run(); err != nil {
			t.Fatalf("setup %v: %v", args, err)
		}
	}

	repo, err := git.NewRepoAt(dir)
	if err != nil {
		t.Fatalf("NewRepoAt: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}

	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	return repo, cfg, metadata
}

func TestCreateBranch(t *testing.T) {
	repo, _, metadata := setupTestRepo(t)

	result, err := CreateBranch(repo, metadata, "feat-test", "main")
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	if result.Branch != "feat-test" {
		t.Errorf("expected branch 'feat-test', got '%s'", result.Branch)
	}
	if result.Parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", result.Parent)
	}

	// Verify git state
	if !repo.BranchExists("feat-test") {
		t.Error("branch should exist in git")
	}
	current, _ := repo.GetCurrentBranch()
	if current != "feat-test" {
		t.Errorf("should be on feat-test, got %s", current)
	}

	// Verify metadata
	if !metadata.IsTracked("feat-test") {
		t.Error("branch should be tracked in metadata")
	}
	parent, ok := metadata.GetParent("feat-test")
	if !ok || parent != "main" {
		t.Errorf("expected parent 'main', got '%s'", parent)
	}
}

func TestCreateBranch_AlreadyExists(t *testing.T) {
	repo, _, metadata := setupTestRepo(t)

	_, err := CreateBranch(repo, metadata, "main", "main")
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
}

func TestDeleteBranch(t *testing.T) {
	repo, cfg, metadata := setupTestRepo(t)

	// Create a branch to delete
	if _, err := CreateBranch(repo, metadata, "feat-del", "main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	// Switch back to main so we're not on the branch being deleted
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack: %v", err)
	}

	result, err := DeleteBranch(repo, metadata, s, "feat-del")
	if err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if result.Deleted != "feat-del" {
		t.Errorf("expected deleted 'feat-del', got '%s'", result.Deleted)
	}
	if result.NewParent != "main" {
		t.Errorf("expected parent 'main', got '%s'", result.NewParent)
	}
	if result.CheckedOut != "" {
		t.Errorf("expected no checkout, got '%s'", result.CheckedOut)
	}

	// Verify git state
	if repo.BranchExists("feat-del") {
		t.Error("branch should be deleted from git")
	}

	// Verify metadata
	if metadata.IsTracked("feat-del") {
		t.Error("branch should be untracked in metadata")
	}
}

func TestDeleteBranch_CurrentBranch(t *testing.T) {
	repo, cfg, metadata := setupTestRepo(t)

	if _, err := CreateBranch(repo, metadata, "feat-current", "main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	// Stay on feat-current (CreateBranch checks it out)

	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack: %v", err)
	}

	result, err := DeleteBranch(repo, metadata, s, "feat-current")
	if err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if result.CheckedOut != "main" {
		t.Errorf("expected checkout to 'main', got '%s'", result.CheckedOut)
	}

	current, _ := repo.GetCurrentBranch()
	if current != "main" {
		t.Errorf("should be on main after delete, got %s", current)
	}
}

func TestDeleteBranch_WithChildren(t *testing.T) {
	repo, cfg, metadata := setupTestRepo(t)

	if _, err := CreateBranch(repo, metadata, "feat-parent", "main"); err != nil {
		t.Fatalf("CreateBranch parent: %v", err)
	}
	if _, err := CreateBranch(repo, metadata, "feat-child", "feat-parent"); err != nil {
		t.Fatalf("CreateBranch child: %v", err)
	}
	if err := repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack: %v", err)
	}

	result, err := DeleteBranch(repo, metadata, s, "feat-parent")
	if err != nil {
		t.Fatalf("DeleteBranch: %v", err)
	}

	if len(result.ReparentedChildren) != 1 || result.ReparentedChildren[0] != "feat-child" {
		t.Errorf("expected reparented [feat-child], got %v", result.ReparentedChildren)
	}

	// Verify child was reparented to main
	parent, ok := metadata.GetParent("feat-child")
	if !ok || parent != "main" {
		t.Errorf("expected child parent 'main', got '%s'", parent)
	}
}

func TestDeleteBranch_Trunk(t *testing.T) {
	repo, cfg, metadata := setupTestRepo(t)

	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack: %v", err)
	}

	_, err = DeleteBranch(repo, metadata, s, "main")
	if err == nil {
		t.Fatal("expected error deleting trunk")
	}
}

func TestDeleteBranch_NotTracked(t *testing.T) {
	repo, cfg, metadata := setupTestRepo(t)

	// Create an untracked branch
	if err := repo.CreateBranch("untracked"); err != nil {
		t.Fatalf("create branch: %v", err)
	}

	s, err := stack.BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack: %v", err)
	}

	_, err = DeleteBranch(repo, metadata, s, "untracked")
	if err == nil {
		t.Fatal("expected error for untracked branch")
	}
}
