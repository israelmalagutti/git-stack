package cmd

import (
	"testing"
)

func TestCheckoutBranch_AlreadyOnBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-already", "main")
	// We're on feat-already, try to check out the same branch
	if err := runCheckout(nil, []string{"feat-already"}); err != nil {
		t.Fatalf("expected no error when already on branch: %v", err)
	}
}

func TestCheckoutBranch_UntrackedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create an untracked branch
	if err := tr.repo.CreateBranch("untracked-co"); err != nil {
		t.Fatalf("create branch: %v", err)
	}

	// Checkout untracked branch (node == nil path in checkoutBranch)
	if err := runCheckout(nil, []string{"untracked-co"}); err != nil {
		t.Fatalf("checkout untracked failed: %v", err)
	}
}

func TestCheckoutBranch_WithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-parent", "main")
	tr.createBranch(t, "feat-child", "feat-parent")

	// Go to main, then checkout feat-parent (which has children)
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	if err := runCheckout(nil, []string{"feat-parent"}); err != nil {
		t.Fatalf("checkout parent with children failed: %v", err)
	}
}

func TestCheckoutBranch_Trunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-for-co", "main")

	// Use --trunk flag
	prev := checkoutTrunk
	checkoutTrunk = true
	defer func() { checkoutTrunk = prev }()

	if err := runCheckout(nil, nil); err != nil {
		t.Fatalf("checkout trunk failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "main" {
		t.Errorf("expected main, got %s", current)
	}
}
