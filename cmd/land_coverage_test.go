package cmd

import (
	"testing"
)

func TestRunLand_CurrentBranchOnTrunkErrors(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Stay on main (trunk), no args => try to land current branch (trunk)
	landForce = true
	landStack = false
	defer func() { landForce = false }()

	err := runLand(landCmd, nil)
	if err == nil {
		t.Fatal("expected error landing trunk as current branch")
	}
}

func TestRunLand_ConfirmAccepted(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-accept", "main")
	tr.commitFile(t, "ac.txt", "ac", "Add ac")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-accept"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	landForce = false
	landNoDeleteRemote = true
	landStack = false
	defer func() {
		landForce = false
		landNoDeleteRemote = false
	}()

	// User accepts
	withAskOne(t, []interface{}{true}, func() {
		err := runLand(landCmd, []string{"feat-accept"})
		if err != nil {
			t.Fatalf("runLand accept failed: %v", err)
		}
	})

	if tr.repo.BranchExists("feat-accept") {
		t.Error("expected feat-accept to be deleted")
	}
}

func TestRunLand_StackConfirmAccepted(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-sa", "main")
	tr.commitFile(t, "sa.txt", "sa", "Add sa")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-sa"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	landForce = false
	landNoDeleteRemote = true
	landStack = true
	defer func() {
		landForce = false
		landNoDeleteRemote = false
		landStack = false
	}()

	// User accepts
	withAskOne(t, []interface{}{true}, func() {
		err := runLand(landCmd, nil)
		if err != nil {
			t.Fatalf("runLand --stack accept failed: %v", err)
		}
	})

	if tr.repo.BranchExists("feat-sa") {
		t.Error("expected feat-sa to be deleted")
	}
}

func TestRunLand_WithChainedChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a -> b -> c, land a
	tr.createBranch(t, "feat-chain-a", "main")
	tr.commitFile(t, "ca.txt", "ca", "Add ca")
	tr.createBranch(t, "feat-chain-b", "feat-chain-a")
	tr.commitFile(t, "cb.txt", "cb", "Add cb")
	tr.createBranch(t, "feat-chain-c", "feat-chain-b")
	tr.commitFile(t, "cc.txt", "cc", "Add cc")

	// Merge feat-chain-a into main
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-chain-a"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	landForce = true
	landNoDeleteRemote = true
	landStack = false
	defer func() {
		landForce = false
		landNoDeleteRemote = false
	}()

	err := runLand(landCmd, []string{"feat-chain-a"})
	if err != nil {
		t.Fatalf("runLand chained failed: %v", err)
	}

	if tr.repo.BranchExists("feat-chain-a") {
		t.Error("expected feat-chain-a to be deleted")
	}

	// b and c should still exist
	if !tr.repo.BranchExists("feat-chain-b") {
		t.Error("expected feat-chain-b to still exist")
	}
	if !tr.repo.BranchExists("feat-chain-c") {
		t.Error("expected feat-chain-c to still exist")
	}
}
