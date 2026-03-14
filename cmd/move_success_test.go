package cmd

import (
	"testing"

	"github.com/israelmalagutti/git-stack/internal/config"
)

func TestRunMoveSuccessWithChildren(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-parent", "main")
	repo.commitFile(t, "parent.txt", "parent", "parent commit")
	repo.createBranch(t, "feat-child", "feat-parent")
	repo.commitFile(t, "child.txt", "child", "child commit")
	repo.createBranch(t, "feat-grandchild", "feat-child")
	repo.commitFile(t, "grand.txt", "grand", "grand commit")

	if err := repo.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("failed to checkout main: %v", err)
	}

	prevSource := moveSource
	prevOnto := moveOnto
	defer func() {
		moveSource = prevSource
		moveOnto = prevOnto
	}()

	moveSource = "feat-child"
	moveOnto = "main"
	if err := runMove(nil, nil); err != nil {
		t.Fatalf("runMove success failed: %v", err)
	}

	current, err := repo.repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("failed to get current branch: %v", err)
	}
	if current != "main" {
		t.Fatalf("expected to return to main, got %s", current)
	}
}

func TestRunMoveBuildStackError(t *testing.T) {
	repo := setupCmdTestRepo(t)
	defer repo.cleanup()

	repo.createBranch(t, "feat-move", "main")

	cfg, err := config.Load(repo.repo.GetConfigPath())
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	cfg.Trunk = "missing"
	if err := cfg.Save(repo.repo.GetConfigPath()); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	prevSource := moveSource
	prevOnto := moveOnto
	defer func() {
		moveSource = prevSource
		moveOnto = prevOnto
	}()

	moveSource = "feat-move"
	moveOnto = "main"
	if err := runMove(nil, nil); err == nil {
		t.Fatalf("expected runMove build stack error")
	}
}
