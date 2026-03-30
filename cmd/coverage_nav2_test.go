package cmd

import (
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestRunUp_SingleChild(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-u", "main")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	if err := runUp(nil, nil); err != nil {
		t.Fatalf("runUp single child failed: %v", err)
	}
	cur, _ := tr.repo.GetCurrentBranch()
	if cur != "feat-u" {
		t.Errorf("expected feat-u, got %s", cur)
	}
}

func TestRunUp_MultipleSteps(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "a-up", "main")
	tr.createBranch(t, "b-up", "a-up")
	tr.createBranch(t, "c-up", "b-up")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Go up 2 steps: main -> a-up -> b-up
	if err := runUp(nil, []string{"2"}); err != nil {
		t.Fatalf("runUp 2 steps failed: %v", err)
	}
	cur, _ := tr.repo.GetCurrentBranch()
	if cur != "b-up" {
		t.Errorf("expected b-up, got %s", cur)
	}
}

func TestRunUp_BeyondLeaf(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-leaf-up", "main")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Go up 5 steps from main with only 1 child -- should hit leaf and stop
	if err := runUp(nil, []string{"5"}); err != nil {
		t.Fatalf("runUp beyond leaf failed: %v", err)
	}
}

func TestRunUp_InvalidSteps(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-inv-up", "main")

	if err := runUp(nil, []string{"abc"}); err == nil {
		t.Fatal("expected invalid step error")
	}
	if err := runUp(nil, []string{"-1"}); err == nil {
		t.Fatal("expected invalid step error for -1")
	}
}

func TestRunUp_AtLeaf(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "leaf-up", "main")
	// On leaf-up which has no children

	err := runUp(nil, nil)
	if err == nil {
		t.Fatal("expected already at top error")
	}
}

func TestRunUp_MultipleChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "child1", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.createBranch(t, "child2", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Multiple children, prompt to select
	withAskOne(t, []interface{}{"child1"}, func() {
		if err := runUp(nil, nil); err != nil {
			t.Fatalf("runUp multi-child failed: %v", err)
		}
	})
}

func TestRunUp_MultipleChildrenCancelled(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "mc1", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.createBranch(t, "mc2", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Cancel the prompt
	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runUp(nil, nil); err != nil {
			t.Fatalf("runUp cancelled should not error: %v", err)
		}
	})
}

func TestRunDown_OneStep(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-dn", "main")

	// From feat-dn, go down 1 to main
	if err := runDown(nil, nil); err != nil {
		t.Fatalf("runDown failed: %v", err)
	}
	cur, _ := tr.repo.GetCurrentBranch()
	if cur != "main" {
		t.Errorf("expected main, got %s", cur)
	}
}

func TestRunTrack_WithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a tracked branch, then another tracked one that is its child
	tr.createBranch(t, "track-parent", "main")
	tr.createBranch(t, "track-child", "track-parent")

	// Create an untracked branch
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if err := tr.repo.CreateBranch("untracked-track"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tr.repo.CheckoutBranch("untracked-track"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Track untracked-track with parent main
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runTrack(nil, []string{"untracked-track"}); err != nil {
			t.Fatalf("runTrack failed: %v", err)
		}
	})
}

func TestRunTrack_AlreadyTracked(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "already-tracked", "main")

	err := runTrack(nil, []string{"already-tracked"})
	if err == nil {
		t.Fatal("expected already tracked error")
	}
}

func TestRunTrack_BranchNotExist(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	err := runTrack(nil, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected branch not exist error")
	}
}

func TestRunTrack_Cancelled(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untrack-cancel"); err != nil {
		t.Fatalf("create: %v", err)
	}

	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runTrack(nil, []string{"untrack-cancel"}); err != nil {
			t.Fatalf("runTrack cancel should not error: %v", err)
		}
	})
}
