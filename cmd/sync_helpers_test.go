package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestDeleteBranchAndCleanup(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.createBranch(t, "feat-child", "feat-parent")

	if err := deleteBranchAndCleanup(repo.repo, repo.metadata, "feat-parent"); err != nil {
		t.Fatalf("deleteBranchAndCleanup failed: %v", err)
	}

	if repo.repo.BranchExists("feat-parent") {
		t.Fatalf("expected branch to be deleted")
	}

	parent, ok := repo.metadata.GetParent("feat-child")
	if !ok || parent != "main" {
		t.Fatalf("expected child to be reparented to main, got %q", parent)
	}
}

func TestRestackAllBranchesNoop(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-restack", "main")

	trunk := &stack.Node{Name: "main", IsTrunk: true}
	child := &stack.Node{Name: "feat-restack", Parent: trunk}
	trunk.Children = []*stack.Node{child}

	s := &stack.Stack{
		Trunk: trunk,
		Nodes: map[string]*stack.Node{
			"main":         trunk,
			"feat-restack": child,
		},
	}

	succeeded, failed := restackAllBranches(repo.repo, s, repo.metadata)
	if len(succeeded) != 0 || len(failed) != 0 {
		t.Fatalf("expected no restack operations")
	}
}
