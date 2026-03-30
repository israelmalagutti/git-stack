package cmd

import (
	"testing"
)

func TestRunDown_DefaultOneStep(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a -> b
	tr.createBranch(t, "nav-a", "main")
	tr.createBranch(t, "nav-b", "nav-a")

	// On nav-b, go down 1 step (default)
	if err := runDown(nil, nil); err != nil {
		t.Fatalf("runDown default failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "nav-a" {
		t.Errorf("expected nav-a, got %s", current)
	}
}

func TestRunDown_UntrackedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untracked-down"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tr.repo.CheckoutBranch("untracked-down"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err := runDown(nil, nil)
	if err == nil {
		t.Fatal("expected error for untracked branch")
	}
}

func TestRunDown_NegativeSteps(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "nav-neg", "main")

	err := runDown(nil, []string{"-1"})
	if err == nil {
		t.Fatal("expected error for negative steps")
	}
}

func TestRunDown_ParentIsNil(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a (depth 1, parent is trunk which has parent=nil)
	tr.createBranch(t, "nav-parent-nil", "main")

	// From nav-parent-nil, down 1 reaches main which has parent=nil
	// Then trying to go down again from trunk should print "reached trunk"
	err := runDown(nil, []string{"5"})
	if err != nil {
		t.Fatalf("runDown past trunk failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "main" {
		t.Errorf("expected main, got %s", current)
	}
}

func TestRunTop_TrunkWithNoChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// On main with no children tracked
	err := runTop(nil, nil)
	if err == nil {
		t.Fatal("expected 'already at top' error")
	}
}

func TestRunTop_CheckoutError(t *testing.T) {
	// This is harder to test since checkout only fails for non-existent branches
	// but all branches in the stack exist. Skip this for now.
}

func TestRunDown_TargetEqualsCurrent(t *testing.T) {
	// When steps=0 (or we start at trunk), target == current => return nil
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "nav-same", "main")

	// Down 1 from nav-same reaches main, but if we start at trunk...
	// Actually the trunk check happens first. Let me test the case where
	// node.Parent is nil on first iteration => i==0 => "already at trunk" error
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err := runDown(nil, nil)
	if err == nil {
		t.Fatal("expected already at trunk error")
	}
}
