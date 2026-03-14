package stack

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
)

func setupStackRepo(t *testing.T) (*git.Repo, *config.Config, *config.Metadata, string, func()) {
	t.Helper()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	dir := t.TempDir()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Fatalf("failed to init git repo: %v", err)
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

	readme := filepath.Join(dir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test\n"), 0644); err != nil {
		t.Fatalf("failed to write README: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "add", ".").Run(); err != nil {
		t.Fatalf("failed to add: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "commit", "-m", "Initial").Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
	if err := exec.Command("git", "-C", dir, "branch", "-M", "main").Run(); err != nil {
		t.Fatalf("failed to set main: %v", err)
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

	return repo, cfg, metadata, dir, cleanup
}

func TestBuildStack(t *testing.T) {
	repo, cfg, metadata, _, cleanup := setupStackRepo(t)
	defer cleanup()

	if err := repo.CreateBranch("feat-1"); err != nil {
		t.Fatalf("failed to create branch: %v", err)
	}
	metadata.TrackBranch("feat-1", "main", "")
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	s, err := BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack failed: %v", err)
	}
	if s.Trunk == nil || s.Trunk.Name != "main" {
		t.Fatalf("expected trunk main")
	}
	if s.GetNode("feat-1") == nil {
		t.Fatalf("expected tracked branch node")
	}
}

func TestBuildStackMissingTrunk(t *testing.T) {
	repo, cfg, metadata, _, cleanup := setupStackRepo(t)
	defer cleanup()

	cfg.Trunk = "missing"
	if _, err := BuildStack(repo, cfg, metadata); err == nil {
		t.Fatalf("expected error for missing trunk")
	}
}

func TestBuildStackSkipsMissingBranch(t *testing.T) {
	repo, cfg, metadata, _, cleanup := setupStackRepo(t)
	defer cleanup()

	metadata.TrackBranch("missing-branch", "main", "")
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	s, err := BuildStack(repo, cfg, metadata)
	if err != nil {
		t.Fatalf("BuildStack failed: %v", err)
	}
	if s.GetNode("missing-branch") != nil {
		t.Fatalf("expected missing branch to be skipped")
	}
}

func TestGetTopologicalOrder(t *testing.T) {
	trunk := &Node{Name: "main", IsTrunk: true}
	a := &Node{Name: "feat-a", Parent: trunk}
	b := &Node{Name: "feat-b", Parent: a}
	trunk.Children = []*Node{a}
	a.Children = []*Node{b}

	s := &Stack{
		Trunk: trunk,
		Nodes: map[string]*Node{
			"main":   trunk,
			"feat-a": a,
			"feat-b": b,
		},
	}

	order := s.GetTopologicalOrder()
	if len(order) != 2 {
		t.Fatalf("expected 2 non-trunk nodes, got %d", len(order))
	}
	if order[0].Name != "feat-a" || order[1].Name != "feat-b" {
		t.Fatalf("unexpected topological order: %s, %s", order[0].Name, order[1].Name)
	}
}
