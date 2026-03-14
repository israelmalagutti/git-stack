package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestRunMoveRebaseFailureRestoresMetadata(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-child", "main")
	repo.commitFile(t, "conflict.txt", "child", "child commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}
	repo.commitFile(t, "conflict.txt", "main", "main commit")

	prevSource := moveSource
	prevOnto := moveOnto
	defer func() {
		moveSource = prevSource
		moveOnto = prevOnto
	}()

	moveSource = "feat-child"
	moveOnto = "main"

	if err := runMove(nil, nil); err == nil {
		t.Fatalf("expected runMove rebase failure")
	}

	metadata, err := config.LoadMetadata(repo.repo.GetMetadataPath())
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	parent, ok := metadata.GetParent("feat-child")
	if !ok || parent != "main" {
		t.Fatalf("expected parent restored to main, got %q", parent)
	}
}
