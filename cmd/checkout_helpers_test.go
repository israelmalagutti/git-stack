package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestCheckoutBranch(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-checkout", "main")

	trunk := &stack.Node{Name: "main", IsTrunk: true}
	child := &stack.Node{Name: "feat-checkout", Parent: trunk}
	trunk.Children = []*stack.Node{child}
	s := &stack.Stack{
		Trunk: trunk,
		Nodes: map[string]*stack.Node{
			"main":          trunk,
			"feat-checkout": child,
		},
	}

	// Already on branch path
	if err := checkoutBranch(repo.repo, s, "feat-checkout"); err != nil {
		t.Fatalf("checkoutBranch failed: %v", err)
	}

	// Switch back to main
	if err := checkoutBranch(repo.repo, s, "main"); err != nil {
		t.Fatalf("checkoutBranch failed: %v", err)
	}
}
