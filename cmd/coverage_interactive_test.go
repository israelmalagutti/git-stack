package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
)

// --- delete.go interactive path ---

func TestRunDelete_InteractiveSelect(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "del-interactive", "main")
	tr.commitFile(t, "del.txt", "data", "del commit")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := deleteForce
	deleteForce = false
	defer func() { deleteForce = prev }()

	// First prompt: select branch, second prompt: confirm
	withAskOne(t, []interface{}{"del-interactive (parent: main)", true}, func() {
		if err := runDelete(nil, nil); err != nil {
			t.Fatalf("runDelete interactive failed: %v", err)
		}
	})
}

func TestRunDelete_InteractiveCancel(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "del-cancel", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := deleteForce
	deleteForce = false
	defer func() { deleteForce = prev }()

	// Select branch then decline confirm
	withAskOne(t, []interface{}{"del-cancel (parent: main)", false}, func() {
		if err := runDelete(nil, nil); err != nil {
			t.Fatalf("runDelete cancel failed: %v", err)
		}
	})
}

func TestRunDelete_CurrentBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "del-current", "main")
	tr.commitFile(t, "delcur.txt", "data", "del commit")
	// Stay on del-current

	prev := deleteForce
	deleteForce = true
	defer func() { deleteForce = prev }()

	if err := runDelete(nil, []string{"del-current"}); err != nil {
		t.Fatalf("runDelete current failed: %v", err)
	}

	// Should be on main now
	cur, _ := tr.repo.GetCurrentBranch()
	if cur != "main" {
		t.Errorf("expected main, got %s", cur)
	}
}

func TestRunDelete_TrunkError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	err := runDelete(nil, []string{"main"})
	if err == nil {
		t.Fatal("expected error deleting trunk")
	}
}

func TestRunDelete_InteractiveNoBranches(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No tracked branches besides trunk -> "no branches available"
	err := runDelete(nil, nil)
	if err == nil {
		t.Fatal("expected error when no branches to delete")
	}
}

// --- checkout.go interactive with description ---

func TestRunCheckout_InteractiveWithDescription(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-desc", "main")
	tr.createBranch(t, "feat-desc-child", "feat-desc")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := checkoutShowUntracked
	checkoutShowUntracked = false
	defer func() { checkoutShowUntracked = prev }()

	// Interactive with feat-desc (has parent info in description)
	withAskOne(t, []interface{}{"feat-desc"}, func() {
		if err := runCheckout(nil, nil); err != nil {
			t.Fatalf("runCheckout interactive desc failed: %v", err)
		}
	})
}

// --- move.go additional paths ---

func TestRunMove_OntoFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-move-a", "main")
	tr.commitFile(t, "mvA.txt", "a", "a commit")
	tr.createBranch(t, "feat-move-b", "main")
	tr.commitFile(t, "mvB.txt", "b", "b commit")

	// Move feat-move-a onto feat-move-b
	if err := tr.repo.CheckoutBranch("feat-move-a"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := moveOnto
	moveOnto = "feat-move-b"
	defer func() { moveOnto = prev }()

	if err := runMove(nil, nil); err != nil {
		t.Fatalf("runMove --onto failed: %v", err)
	}
}

func TestRunMove_TrunkError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// On main, try to move trunk
	prev := moveOnto
	moveOnto = "main"
	defer func() { moveOnto = prev }()

	err := runMove(nil, nil)
	if err == nil {
		t.Fatal("expected error moving trunk")
	}
}

// --- modify.go additional paths ---

func TestRunModify_OnTrunkAmend(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// On main (trunk), modify amends the last commit
	if err := os.WriteFile(filepath.Join(tr.dir, "modify.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "modify.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := modifyMessage
	prevAll := modifyAll
	modifyMessage = ""
	modifyAll = false
	defer func() { modifyMessage = prev; modifyAll = prevAll }()

	// Trunk modify should succeed (amend)
	if err := runModify(nil, nil); err != nil {
		t.Fatalf("runModify on trunk failed: %v", err)
	}
}

func TestRunModify_WithChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-modify", "main")
	tr.commitFile(t, "first.txt", "data", "first commit")

	// Modify a tracked file
	if err := os.WriteFile(filepath.Join(tr.dir, "first.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "first.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := modifyMessage
	prevAll := modifyAll
	modifyMessage = "modified commit"
	modifyAll = false
	defer func() { modifyMessage = prev; modifyAll = prevAll }()

	if err := runModify(nil, nil); err != nil {
		t.Fatalf("runModify failed: %v", err)
	}
}

// --- rename.go additional paths ---

func TestRunRename_WithArg(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-old-name", "main")
	// On feat-old-name

	if err := runRename(nil, []string{"feat-new-name"}); err != nil {
		t.Fatalf("runRename failed: %v", err)
	}

	if tr.repo.BranchExists("feat-old-name") {
		t.Error("expected old branch to be gone")
	}
	if !tr.repo.BranchExists("feat-new-name") {
		t.Error("expected new branch to exist")
	}
}

func TestRunRename_WithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-rn-parent", "main")
	tr.createBranch(t, "feat-rn-child", "feat-rn-parent")

	if err := tr.repo.CheckoutBranch("feat-rn-parent"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	if err := runRename(nil, []string{"feat-rn-renamed"}); err != nil {
		t.Fatalf("runRename with children failed: %v", err)
	}
}

func TestRunRename_Prompted(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-rn-prompt", "main")

	withAskOne(t, []interface{}{"feat-rn-prompted"}, func() {
		if err := runRename(nil, nil); err != nil {
			t.Fatalf("runRename prompted failed: %v", err)
		}
	})
}

func TestRunRename_SameName(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-same", "main")

	err := runRename(nil, []string{"feat-same"})
	// Same name prints message, no error
	if err != nil {
		t.Fatalf("runRename same name should not error: %v", err)
	}
}

// --- stack_restack.go additional paths ---

func TestRunStackRestack_NoBranchesToRestack(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No branches need restack
	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack noop failed: %v", err)
	}
}

func TestRunStackRestack_WithBranches(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-rs-a", "main")
	tr.commitFile(t, "rs-a.txt", "a", "a commit")

	// Add commit to main so restack is needed
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "main-rs.txt", "main", "main commit")

	// Go back to feature branch
	if err := tr.repo.CheckoutBranch("feat-rs-a"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack failed: %v", err)
	}
}

// --- info.go with PR metadata ---

func TestRunInfo_WithPR(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-info-pr", "main")
	tr.commitFile(t, "info-pr.txt", "data", "pr commit")

	// Set PR metadata
	if err := tr.metadata.SetPR("feat-info-pr", &config.PRInfo{Number: 42, Provider: "github"}); err != nil {
		t.Fatalf("set PR: %v", err)
	}
	if err := tr.metadata.Save(tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	if err := runInfo(nil, nil); err != nil {
		t.Fatalf("runInfo with PR failed: %v", err)
	}
}

// --- top.go path where current branch has 0 children but is in stack ---

func TestRunTop_Cancelled2(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "tl-a", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.createBranch(t, "tl-b", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runTop(nil, nil)
		if err != nil {
			t.Fatalf("runTop cancel should not error: %v", err)
		}
	})
}
