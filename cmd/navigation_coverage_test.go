package cmd

import (
	"testing"
)

func TestRunDown_MultipleSteps(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a -> b -> c
	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")
	tr.createBranch(t, "c", "b")

	// From c, go down 2 should reach a
	if err := runDown(nil, []string{"2"}); err != nil {
		t.Fatalf("runDown 2 failed: %v", err)
	}
	current, _ := tr.repo.GetCurrentBranch()
	if current != "a" {
		t.Errorf("expected a, got %s", current)
	}
}

func TestRunDown_BeyondTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-d", "main")

	// Down 5 from depth-1 branch should hit trunk limit
	if err := runDown(nil, []string{"5"}); err != nil {
		t.Fatalf("runDown beyond trunk failed: %v", err)
	}
}

func TestRunDown_InvalidSteps(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-inv", "main")

	// Invalid step count
	if err := runDown(nil, []string{"abc"}); err == nil {
		t.Fatal("expected invalid step error")
	}

	// Zero steps
	if err := runDown(nil, []string{"0"}); err == nil {
		t.Fatal("expected invalid step error for 0")
	}
}

func TestRunDown_AlreadyAtTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// On main (trunk)
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err := runDown(nil, nil)
	if err == nil {
		t.Fatal("expected already at trunk error")
	}
}

func TestRunBottom_AlreadyAtTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Already on main
	err := runBottom(nil, nil)
	if err != nil {
		t.Fatalf("runBottom at trunk failed: %v", err)
	}
}

func TestRunBottom_FromBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-bot", "main")
	// We're on feat-bot
	if err := runBottom(nil, nil); err != nil {
		t.Fatalf("runBottom failed: %v", err)
	}
	current, _ := tr.repo.GetCurrentBranch()
	if current != "main" {
		t.Errorf("expected main, got %s", current)
	}
}

func TestRunTop_MultipleLeaves(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a -> b, main -> a -> c
	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")
	if err := tr.repo.CheckoutBranch("a"); err != nil {
		t.Fatalf("checkout a: %v", err)
	}
	tr.createBranch(t, "c", "a")

	// Go back to main
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	// Multiple leaves, should prompt for selection
	withAskOne(t, []interface{}{"b"}, func() {
		if err := runTop(nil, nil); err != nil {
			t.Fatalf("runTop multiple leaves failed: %v", err)
		}
	})

	current, _ := tr.repo.GetCurrentBranch()
	if current != "b" {
		t.Errorf("expected b, got %s", current)
	}
}
