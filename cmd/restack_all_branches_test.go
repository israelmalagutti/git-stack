package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestRestackAllBranchesConflict(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-conflict", "main")
	if err := repo.repo.CheckoutBranch("feat-conflict"); err != nil {
		t.Fatalf("failed to checkout feat: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.dir, "conflict.txt"), []byte("feat"), 0644); err != nil {
		t.Fatalf("failed to write feat file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add feat file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "feat commit"); err != nil {
		t.Fatalf("failed to commit feat: %v", err)
	}

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.dir, "conflict.txt"), []byte("main"), 0644); err != nil {
		t.Fatalf("failed to write main file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("add", "conflict.txt"); err != nil {
		t.Fatalf("failed to add main file: %v", err)
	}
	if _, err := repo.repo.RunGitCommand("commit", "-m", "main commit"); err != nil {
		t.Fatalf("failed to commit main: %v", err)
	}

	trunk := &stack.Node{Name: "main", IsTrunk: true}
	child := &stack.Node{Name: "feat-conflict", Parent: trunk}
	trunk.Children = []*stack.Node{child}
	s := &stack.Stack{
		Trunk: trunk,
		Nodes: map[string]*stack.Node{
			"main":          trunk,
			"feat-conflict": child,
		},
	}

	_, failed := restackAllBranches(repo.repo, s, repo.metadata)
	if len(failed) == 0 {
		t.Fatalf("expected conflict in restackAllBranches")
	}
	_, _ = repo.repo.RunGitCommand("rebase", "--abort")
}
