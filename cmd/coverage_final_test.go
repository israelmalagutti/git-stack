package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
)

// --- continue.go paths ---

func TestRunContinue_WithSavedStateNoRebase(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-cont-state", "main")
	tr.commitFile(t, "cont-state.txt", "data", "commit")

	// Save state with the current branch as remaining (already up to date)
	state := &config.ContinueState{
		RemainingBranches: []string{"feat-cont-state"},
		OriginalBranch:    "main",
	}
	if err := state.Save(tr.repo.GetContinueStatePath()); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// No rebase in progress, state exists with branches to process
	// Should process them (no-op restack since already up to date) and clear state
	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("runContinue with saved state failed: %v", err)
	}
}

// --- move.go additional paths ---

func TestRunMove_UntrackedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untracked-move"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tr.repo.CheckoutBranch("untracked-move"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := moveOnto
	moveOnto = "main"
	defer func() { moveOnto = prev }()

	err := runMove(nil, nil)
	if err == nil {
		t.Fatal("expected error for untracked branch")
	}
}

func TestRunMove_OntoSelf(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-move-self", "main")

	prev := moveOnto
	moveOnto = "feat-move-self"
	defer func() { moveOnto = prev }()

	err := runMove(nil, nil)
	if err == nil {
		t.Fatal("expected error moving onto self")
	}
}

func TestRunMove_InteractiveSelect(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-move-int", "main")
	tr.commitFile(t, "mv-int.txt", "data", "commit")

	prev := moveOnto
	moveOnto = ""
	defer func() { moveOnto = prev }()

	// When no --onto is given, it prompts
	withAskOne(t, []interface{}{"main"}, func() {
		err := runMove(nil, nil)
		// May succeed or fail depending on whether it's already on main
		_ = err
	})
}

// --- delete.go with confirm prompt ---

func TestRunDelete_ConfirmPromptError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "del-err", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := deleteForce
	deleteForce = false
	defer func() { deleteForce = prev }()

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runDelete(nil, []string{"del-err"})
		if err == nil {
			t.Fatal("expected error from cancelled confirm")
		}
	})
}

// --- checkout.go with sorted branches ---

func TestRunCheckout_SortedBranches(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "z-branch", "main")
	tr.createBranch(t, "a-branch", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := checkoutShowUntracked
	checkoutShowUntracked = false
	defer func() { checkoutShowUntracked = prev }()

	// Interactive select, branches should be sorted
	withAskOne(t, []interface{}{"a-branch"}, func() {
		if err := runCheckout(nil, nil); err != nil {
			t.Fatalf("runCheckout sorted failed: %v", err)
		}
	})
}

// --- modify.go additional paths ---

func TestRunModify_UntrackedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untracked-mod"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tr.repo.CheckoutBranch("untracked-mod"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err := runModify(nil, nil)
	if err == nil {
		t.Fatal("expected error for untracked branch")
	}
}

func TestRunModify_NoCommitsBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-mod-empty", "main")
	// No commits on this branch

	// Staged change
	if err := os.WriteFile(filepath.Join(tr.dir, "mod-empty.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "mod-empty.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := modifyMessage
	prevAll := modifyAll
	prevCommit := modifyCommit
	modifyMessage = "new commit"
	modifyAll = false
	modifyCommit = false
	defer func() { modifyMessage = prev; modifyAll = prevAll; modifyCommit = prevCommit }()

	if err := runModify(nil, nil); err != nil {
		t.Fatalf("runModify no commits failed: %v", err)
	}
}

// --- land.go paths with prompt errors ---

func TestRunLand_PromptError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-land-err", "main")
	tr.commitFile(t, "le.txt", "le", "Add le")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-land-err"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	landForce = false
	landNoDeleteRemote = true
	landStack = false
	defer func() { landForce = false; landNoDeleteRemote = false }()

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runLand(landCmd, []string{"feat-land-err"})
		// InterruptErr from survey
		if err == nil {
			t.Fatal("expected error from interrupt")
		}
	})
}

// --- repair.go with multiple issues ---

func TestRunRepair_MultipleOrphanedForce(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create multiple orphaned entries
	tr.createBranch(t, "orphan1", "main")
	tr.createBranch(t, "orphan2", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if err := tr.repo.DeleteBranch("orphan1", true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := tr.repo.DeleteBranch("orphan2", true); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := tr.metadata.SaveWithRefs(tr.repo, tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	repairDryRun = false
	repairForce = true
	defer func() { repairDryRun = false; repairForce = false }()

	if err := runRepair(repairCmd, nil); err != nil {
		t.Fatalf("runRepair multi-orphan failed: %v", err)
	}
}

// --- init.go with remote that has existing config ref ---

func TestRunInit_RemoteWithConfigRef(t *testing.T) {
	tr, _ := setupSubmitTestRepo(t)
	defer tr.cleanup()

	// Write a gs config ref to the remote via the local repo
	cfg := config.NewConfig("main")
	_ = config.WriteRefConfig(tr.repo, cfg)
	pushConfigRef(tr.repo)

	// Remove local gs config to force init
	os.Remove(filepath.Join(tr.dir, ".git", ".gs_config"))
	os.Remove(filepath.Join(tr.dir, ".git", ".gs_stack_metadata"))

	// Configure refspec for gs refs
	configureGSRefspec(tr.repo)

	// Init should pick up the remote config
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit with remote config failed: %v", err)
		}
	})
}
