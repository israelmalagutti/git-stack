package cmd

import (
	"testing"
)

func TestRunTop_SingleLeaf(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a single child: main -> feat-only
	tr.createBranch(t, "feat-only", "main")

	// Go back to main
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	err := runTop(nil, nil)
	if err != nil {
		t.Fatalf("runTop single leaf failed: %v", err)
	}

	current, err := tr.repo.GetCurrentBranch()
	if err != nil {
		t.Fatalf("get current: %v", err)
	}
	if current != "feat-only" {
		t.Errorf("expected to be on feat-only, got %s", current)
	}
}

func TestRunTop_AlreadyAtLeaf(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-leaf", "main")
	// We're on feat-leaf which has no children => "already at top of stack" error
	err := runTop(nil, nil)
	if err == nil || err.Error() != "already at top of stack" {
		t.Fatalf("expected 'already at top of stack' error, got: %v", err)
	}
}

func TestRunTop_DeepChain(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a -> b -> c
	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")
	tr.createBranch(t, "c", "b")

	// Go to main, jump to top
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	err := runTop(nil, nil)
	if err != nil {
		t.Fatalf("runTop deep chain failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "c" {
		t.Errorf("expected c, got %s", current)
	}
}
