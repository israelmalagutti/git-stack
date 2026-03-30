package cmd

import (
	"testing"
)

func TestRunLand_SingleBranch_Merged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a tracked branch with a commit
	tr.createBranch(t, "feat-a", "main")
	tr.commitFile(t, "a.txt", "a", "Add a")

	// Merge feat-a into main so it is considered merged
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-a"); err != nil {
		t.Fatalf("merge feat-a: %v", err)
	}

	// Set flags: force (skip prompt), no-delete-remote (no remote)
	landForce = true
	landNoDeleteRemote = true
	landStack = false
	defer func() {
		landForce = false
		landNoDeleteRemote = false
	}()

	err := runLand(landCmd, []string{"feat-a"})
	if err != nil {
		t.Fatalf("runLand failed: %v", err)
	}

	// Branch should be deleted locally
	if tr.repo.BranchExists("feat-a") {
		t.Error("expected feat-a to be deleted after landing")
	}
}

func TestRunLand_TrunkErrors(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	landForce = true
	landStack = false
	defer func() { landForce = false }()

	err := runLand(landCmd, []string{"main"})
	if err == nil {
		t.Fatal("expected error when landing trunk")
	}
}

func TestRunLand_UnmergedBranchErrors(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-unmerged", "main")
	tr.commitFile(t, "u.txt", "u", "unmerged commit")

	// Go back to main so we can land by name
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	landForce = true
	landNoDeleteRemote = true
	landStack = false
	defer func() {
		landForce = false
		landNoDeleteRemote = false
	}()

	err := runLand(landCmd, []string{"feat-unmerged"})
	if err == nil {
		t.Fatal("expected error for unmerged branch")
	}
}

func TestRunLand_CurrentBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-cur", "main")
	tr.commitFile(t, "c.txt", "c", "Add c")

	// Merge into main
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-cur"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Go back to the feature branch so landing uses current branch
	if err := tr.repo.CheckoutBranch("feat-cur"); err != nil {
		t.Fatalf("checkout feat-cur: %v", err)
	}

	landForce = true
	landNoDeleteRemote = true
	landStack = false
	defer func() {
		landForce = false
		landNoDeleteRemote = false
	}()

	// No args => lands current branch
	err := runLand(landCmd, nil)
	if err != nil {
		t.Fatalf("runLand (current) failed: %v", err)
	}

	if tr.repo.BranchExists("feat-cur") {
		t.Error("expected feat-cur to be deleted")
	}
}

func TestRunLand_ConfirmDeclined(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-decline", "main")
	tr.commitFile(t, "d.txt", "d", "Add d")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-decline"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Don't set landForce; mock askOne to decline
	landForce = false
	landNoDeleteRemote = true
	landStack = false
	defer func() { landNoDeleteRemote = false }()

	withAskOne(t, []interface{}{false}, func() {
		err := runLand(landCmd, []string{"feat-decline"})
		if err != nil {
			t.Fatalf("runLand cancel failed: %v", err)
		}
	})

	// Branch should still exist (user declined)
	if !tr.repo.BranchExists("feat-decline") {
		t.Error("expected feat-decline to still exist after decline")
	}
}

func TestRunLand_StackNoMerged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-s1", "main")
	tr.commitFile(t, "s1.txt", "s1", "Add s1")

	landForce = true
	landNoDeleteRemote = true
	landStack = true
	defer func() {
		landForce = false
		landNoDeleteRemote = false
		landStack = false
	}()

	// No merged branches => should print message but not error
	err := runLand(landCmd, nil)
	if err != nil {
		t.Fatalf("runLand --stack (no merged) failed: %v", err)
	}
}

func TestRunLand_StackMerged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-sm", "main")
	tr.commitFile(t, "sm.txt", "sm", "Add sm")

	// Merge into main
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-sm"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	landForce = true
	landNoDeleteRemote = true
	landStack = true
	defer func() {
		landForce = false
		landNoDeleteRemote = false
		landStack = false
	}()

	err := runLand(landCmd, nil)
	if err != nil {
		t.Fatalf("runLand --stack failed: %v", err)
	}

	if tr.repo.BranchExists("feat-sm") {
		t.Error("expected feat-sm to be deleted")
	}
}

func TestRunLand_StackConfirmDeclined(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-sd", "main")
	tr.commitFile(t, "sd.txt", "sd", "Add sd")

	// Merge into main
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-sd"); err != nil {
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

	// User declines the stack landing
	withAskOne(t, []interface{}{false}, func() {
		err := runLand(landCmd, nil)
		if err != nil {
			t.Fatalf("runLand --stack (declined) failed: %v", err)
		}
	})

	// Branch should still exist
	if !tr.repo.BranchExists("feat-sd") {
		t.Error("expected feat-sd to still exist after decline")
	}
}

func TestRunLand_StackMultipleMerged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create two branches, merge both
	tr.createBranch(t, "feat-m1", "main")
	tr.commitFile(t, "m1.txt", "m1", "Add m1")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-m1"); err != nil {
		t.Fatalf("merge m1: %v", err)
	}

	tr.createBranch(t, "feat-m2", "main")
	tr.commitFile(t, "m2.txt", "m2", "Add m2")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-m2"); err != nil {
		t.Fatalf("merge m2: %v", err)
	}

	landForce = true
	landNoDeleteRemote = true
	landStack = true
	defer func() {
		landForce = false
		landNoDeleteRemote = false
		landStack = false
	}()

	err := runLand(landCmd, nil)
	if err != nil {
		t.Fatalf("runLand --stack (multi) failed: %v", err)
	}

	if tr.repo.BranchExists("feat-m1") {
		t.Error("expected feat-m1 to be deleted")
	}
	if tr.repo.BranchExists("feat-m2") {
		t.Error("expected feat-m2 to be deleted")
	}
}

func TestRunLand_WithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a chain: main -> feat-parent -> feat-child
	tr.createBranch(t, "feat-parent", "main")
	tr.commitFile(t, "p.txt", "p", "Add p")
	tr.createBranch(t, "feat-child", "feat-parent")
	tr.commitFile(t, "ch.txt", "ch", "Add ch")

	// Merge feat-parent into main
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-parent"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	landForce = true
	landNoDeleteRemote = true
	landStack = false
	defer func() {
		landForce = false
		landNoDeleteRemote = false
	}()

	err := runLand(landCmd, []string{"feat-parent"})
	if err != nil {
		t.Fatalf("runLand failed: %v", err)
	}

	if tr.repo.BranchExists("feat-parent") {
		t.Error("expected feat-parent to be deleted")
	}

	// feat-child should still exist
	if !tr.repo.BranchExists("feat-child") {
		t.Error("expected feat-child to still exist")
	}
}
