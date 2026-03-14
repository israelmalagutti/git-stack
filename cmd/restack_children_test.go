package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/stack"
)

func TestRestackChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-child", "main")
	repo.createBranch(t, "feat-grandchild", "feat-child")

	s, err := stack.BuildStack(repo.repo, repo.cfg, repo.metadata)
	if err != nil {
		t.Fatalf("BuildStack failed: %v", err)
	}

	parent := s.GetNode("main")
	if parent == nil {
		t.Fatalf("expected trunk node")
	}

	if err := restackChildren(repo.repo, s, parent); err != nil {
		t.Fatalf("restackChildren failed: %v", err)
	}
}
