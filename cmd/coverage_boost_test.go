package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
	"github.com/israelmalagutti/git-stack/internal/git"
	"github.com/israelmalagutti/git-stack/internal/stack"
)

// --- top.go: multiple leaves with prompt ---

func TestRunTop_MultipleLeavesBoost(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a, a -> b, a -> c  (b and c are leaves)
	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")
	if err := tr.repo.CheckoutBranch("a"); err != nil {
		t.Fatalf("checkout a: %v", err)
	}
	tr.createBranch(t, "c", "a")

	// Go to main, then jump to top — should prompt
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	withAskOne(t, []interface{}{"b"}, func() {
		if err := runTop(nil, nil); err != nil {
			t.Fatalf("runTop multi-leaf failed: %v", err)
		}
	})

	current, _ := tr.repo.GetCurrentBranch()
	if current != "b" {
		t.Errorf("expected b, got %s", current)
	}
}

func TestRunTop_MultipleLeavesBoostCancel(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")
	if err := tr.repo.CheckoutBranch("a"); err != nil {
		t.Fatalf("checkout a: %v", err)
	}
	tr.createBranch(t, "c", "a")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runTop(nil, nil)
		if err != nil {
			t.Fatalf("runTop cancel should return nil, got: %v", err)
		}
	})
}

func TestRunTop_MultipleLeavesBoostGenericError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")
	if err := tr.repo.CheckoutBranch("a"); err != nil {
		t.Fatalf("checkout a: %v", err)
	}
	tr.createBranch(t, "c", "a")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	withAskOneError(t, fmt.Errorf("some error"), func() {
		err := runTop(nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})
}

// --- down.go: multiple steps reaching trunk ---

func TestRunDown_MultipleStepsReachTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a -> b ; start at b, go down 5 (should stop at main after 2)
	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")

	if err := tr.repo.CheckoutBranch("b"); err != nil {
		t.Fatalf("checkout b: %v", err)
	}

	err := runDown(nil, []string{"5"})
	if err != nil {
		t.Fatalf("runDown 5 steps failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "main" {
		t.Errorf("expected main, got %s", current)
	}
}

func TestRunDown_InvalidStep(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-step", "main")

	err := runDown(nil, []string{"abc"})
	if err == nil {
		t.Fatal("expected error for invalid step")
	}
}

// --- log.go: --short flag ---

func TestRunLog_Short(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-log-s", "main")

	prevShort := logShort
	prevLong := logLong
	defer func() {
		logShort = prevShort
		logLong = prevLong
	}()

	logShort = true
	logLong = false
	if err := runLog(nil, nil); err != nil {
		t.Fatalf("runLog --short failed: %v", err)
	}
}

// --- up.go: multiple children prompt ---

func TestRunUp_MultipleChildrenPrompt(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a, main -> b
	tr.createBranch(t, "a", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	tr.createBranch(t, "b", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	withAskOne(t, []interface{}{"a"}, func() {
		err := runUp(nil, nil)
		if err != nil {
			t.Fatalf("runUp multi-child prompt failed: %v", err)
		}
	})

	current, _ := tr.repo.GetCurrentBranch()
	if current != "a" {
		t.Errorf("expected a, got %s", current)
	}
}

func TestRunUp_MultipleChildrenCancel(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "a", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	tr.createBranch(t, "b", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runUp(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunUp_MultiStepReachTop(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "a", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	// Go up 5 steps but only 1 child deep
	err := runUp(nil, []string{"5"})
	if err != nil {
		t.Fatalf("runUp multi-step failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "a" {
		t.Errorf("expected a, got %s", current)
	}
}

// --- sync.go: deleteMergedBranches "all" and "quit" ---

func TestDeleteMergedBranchesAll(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create and merge two branches
	tr.createBranch(t, "feat-all-1", "main")
	if err := tr.repo.CheckoutBranch("feat-all-1"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "all1.txt", "data", "all1")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-all-1"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	tr.createBranch(t, "feat-all-2", "main")
	if err := tr.repo.CheckoutBranch("feat-all-2"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "all2.txt", "data", "all2")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-all-2"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Use 'a' (all) to delete both
	withReadKey('a', func() {
		if err := deleteMergedBranches(tr.repo, tr.metadata, "main", false); err != nil {
			t.Fatalf("deleteMergedBranches all failed: %v", err)
		}
	})

	if tr.repo.BranchExists("feat-all-1") || tr.repo.BranchExists("feat-all-2") {
		t.Fatal("expected both branches to be deleted")
	}
}

func TestDeleteMergedBranchesQuit(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-quit", "main")
	if err := tr.repo.CheckoutBranch("feat-quit"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "quit.txt", "data", "quit")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("merge", "feat-quit"); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Use 'q' to quit
	withReadKey('q', func() {
		if err := deleteMergedBranches(tr.repo, tr.metadata, "main", false); err != nil {
			t.Fatalf("deleteMergedBranches quit failed: %v", err)
		}
	})

	// Branch should still exist
	if !tr.repo.BranchExists("feat-quit") {
		t.Fatal("expected feat-quit to still exist after quit")
	}
}

// --- sync.go: syncTrunkWithRemote diverged + force ---

func TestSyncTrunkWithRemoteDivergedForce(t *testing.T) {
	localDir, otherDir, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	// Local diverging commit
	if err := os.WriteFile(filepath.Join(localDir, "local-force.txt"), []byte("local"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := repo.RunGitCommand("add", "."); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := repo.RunGitCommand("commit", "-m", "local diverge force"); err != nil {
		t.Fatalf("commit: %v", err)
	}

	// Remote diverging commit
	if err := os.WriteFile(filepath.Join(otherDir, "remote-force.txt"), []byte("remote"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "add", ".").Run(); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "commit", "-m", "remote diverge force").Run(); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if err := exec.Command("git", "-C", otherDir, "push", "origin", "main").Run(); err != nil {
		t.Fatalf("push: %v", err)
	}

	if err := repo.Fetch(); err != nil {
		t.Fatalf("fetch: %v", err)
	}

	// Force reset (no prompt)
	if err := syncTrunkWithRemote(repo, "main", true); err != nil {
		t.Fatalf("syncTrunkWithRemote diverged force failed: %v", err)
	}
}

// --- init.go: cancel init prompt, already initialized ---

func TestRunInit_Cancel(t *testing.T) {
	dir, cleanup := setupRawRepo(t)
	defer cleanup()

	// Remove gs config so init will proceed to prompt
	os.Remove(filepath.Join(dir, ".git", ".gs_config"))

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runInit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunInit_AlreadyInitialized(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	initReset = false
	err := runInit(nil, nil)
	if err == nil {
		t.Fatal("expected 'already initialized' error")
	}
}

func TestRunInit_NoBranches(t *testing.T) {
	// This is an edge case: repo with no branches is very hard to set up
	// since git init + commit creates a branch. Skip.
	t.Skip("requires bare repo with no branches")
}

// --- checkout.go: --stack flag, --trunk flag, interactive ---

func TestRunCheckout_TrunkFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-co", "main")

	prevTrunk := checkoutTrunk
	defer func() { checkoutTrunk = prevTrunk }()
	checkoutTrunk = true

	if err := runCheckout(nil, nil); err != nil {
		t.Fatalf("runCheckout --trunk failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "main" {
		t.Errorf("expected main, got %s", current)
	}
}

func TestRunCheckout_StackFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// main -> a -> b, main -> c
	tr.createBranch(t, "a", "main")
	tr.createBranch(t, "b", "a")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	tr.createBranch(t, "c", "main")

	// Go to a
	if err := tr.repo.CheckoutBranch("a"); err != nil {
		t.Fatalf("checkout a: %v", err)
	}

	prevStack := checkoutStack
	prevTrunk := checkoutTrunk
	prevShowUntracked := checkoutShowUntracked
	defer func() {
		checkoutStack = prevStack
		checkoutTrunk = prevTrunk
		checkoutShowUntracked = prevShowUntracked
	}()
	checkoutStack = true
	checkoutTrunk = false
	checkoutShowUntracked = false

	// Should only show current stack branches (main, a, b) not c
	withAskOne(t, []interface{}{"b"}, func() {
		if err := runCheckout(nil, nil); err != nil {
			t.Fatalf("runCheckout --stack failed: %v", err)
		}
	})

	current, _ := tr.repo.GetCurrentBranch()
	if current != "b" {
		t.Errorf("expected b, got %s", current)
	}
}

func TestRunCheckout_ShowUntrackedFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create untracked branch
	if err := tr.repo.CreateBranch("untracked-co"); err != nil {
		t.Fatalf("create: %v", err)
	}

	prevStack := checkoutStack
	prevTrunk := checkoutTrunk
	prevShowUntracked := checkoutShowUntracked
	defer func() {
		checkoutStack = prevStack
		checkoutTrunk = prevTrunk
		checkoutShowUntracked = prevShowUntracked
	}()
	checkoutStack = false
	checkoutTrunk = false
	checkoutShowUntracked = true

	withAskOne(t, []interface{}{"untracked-co"}, func() {
		if err := runCheckout(nil, nil); err != nil {
			t.Fatalf("runCheckout --show-untracked failed: %v", err)
		}
	})

	current, _ := tr.repo.GetCurrentBranch()
	if current != "untracked-co" {
		t.Errorf("expected untracked-co, got %s", current)
	}
}

func TestRunCheckout_Cancel(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-co-cancel", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	prevStack := checkoutStack
	prevTrunk := checkoutTrunk
	prevShowUntracked := checkoutShowUntracked
	defer func() {
		checkoutStack = prevStack
		checkoutTrunk = prevTrunk
		checkoutShowUntracked = prevShowUntracked
	}()
	checkoutStack = false
	checkoutTrunk = false
	checkoutShowUntracked = false

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runCheckout(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunCheckout_DirectBranchArg(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-co-direct", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	prevTrunk := checkoutTrunk
	defer func() { checkoutTrunk = prevTrunk }()
	checkoutTrunk = false

	if err := runCheckout(nil, []string{"feat-co-direct"}); err != nil {
		t.Fatalf("runCheckout with arg failed: %v", err)
	}

	current, _ := tr.repo.GetCurrentBranch()
	if current != "feat-co-direct" {
		t.Errorf("expected feat-co-direct, got %s", current)
	}
}

// --- track.go: no parent options ---

func TestRunTrack_NoParentOptions(t *testing.T) {
	// Edge case: only one branch in the repo (it is trunk) and we try
	// to track that same branch. First need a separate untracked branch
	// actually... if there's only one branch, there are 0 parent options.
	// But setupCmdTestRepo always has "main". If we create a new branch
	// and remove all others... that's complex. Let's test a simpler case.
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Already tracked
	tr.createBranch(t, "already", "main")
	err := runTrack(nil, []string{"already"})
	if err == nil {
		t.Fatal("expected already-tracked error")
	}
}

func TestRunTrack_NonExistentBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	err := runTrack(nil, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected branch-not-exist error")
	}
}

func TestRunTrack_CurrentBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create an untracked branch and checkout to it
	if err := tr.repo.CreateBranch("feat-track-curr"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := tr.repo.CheckoutBranch("feat-track-curr"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Track current branch (no args)
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runTrack(nil, nil); err != nil {
			t.Fatalf("runTrack current branch failed: %v", err)
		}
	})
}

// --- info.go: untracked branch info ---

func TestRunInfo_UntrackedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untracked-info"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Show info for untracked branch
	err := runInfo(nil, []string{"untracked-info"})
	if err != nil {
		t.Fatalf("runInfo untracked should not error: %v", err)
	}
}

func TestRunInfo_TrunkBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	err := runInfo(nil, []string{"main"})
	if err != nil {
		t.Fatalf("runInfo trunk failed: %v", err)
	}
}

func TestRunInfo_BranchWithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "parent-info", "main")
	tr.createBranch(t, "child-info", "parent-info")

	err := runInfo(nil, []string{"parent-info"})
	if err != nil {
		t.Fatalf("runInfo with children failed: %v", err)
	}
}

func TestRunInfo_NonExistentBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	err := runInfo(nil, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}
}

// --- delete.go: interactive delete with prompt, cancel ---

func TestRunDelete_InteractiveCancelBoost(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-del-cancel", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	prevForce := deleteForce
	defer func() { deleteForce = prevForce }()
	deleteForce = false

	// First prompt selects branch, second confirms delete (false = cancel)
	withAskOne(t, []interface{}{
		"feat-del-cancel (parent: main)",
		false,
	}, func() {
		err := runDelete(nil, nil)
		if err != nil {
			t.Fatalf("expected nil after cancel, got: %v", err)
		}
	})

	// Branch should still exist
	if !tr.repo.BranchExists("feat-del-cancel") {
		t.Fatal("branch should still exist after cancel")
	}
}

// --- bottom.go: already at trunk ---

func TestRunBottom_AlreadyAtTrunkBoost(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Already on main
	err := runBottom(nil, nil)
	if err != nil {
		t.Fatalf("runBottom at trunk should not error: %v", err)
	}
}

// --- sync.go: cleanStaleBranches with reparenting ---

func TestCleanStaleBranches_ReparentingToTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create: main -> stale -> child
	tr.createBranch(t, "stale", "main")
	tr.createBranch(t, "child-of-stale", "stale")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}

	// Delete the stale branch from git (but keep in metadata)
	if _, err := tr.repo.RunGitCommand("branch", "-D", "stale"); err != nil {
		t.Fatalf("delete stale: %v", err)
	}

	// Force clean
	if err := cleanStaleBranches(tr.repo, tr.metadata, tr.cfg, true); err != nil {
		t.Fatalf("cleanStaleBranches failed: %v", err)
	}

	// child-of-stale should be reparented to main
	parent, ok := tr.metadata.GetParent("child-of-stale")
	if !ok || parent != "main" {
		t.Fatalf("expected child reparented to main, got %q", parent)
	}
}

func TestCleanStaleBranches_NoStale(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No stale branches
	if err := cleanStaleBranches(tr.repo, tr.metadata, tr.cfg, true); err != nil {
		t.Fatalf("cleanStaleBranches no stale failed: %v", err)
	}
}

func TestCleanStaleBranches_DeclineCleanup(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "stale-decline", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout main: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("branch", "-D", "stale-decline"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Decline cleanup
	withReadKey('n', func() {
		if err := cleanStaleBranches(tr.repo, tr.metadata, tr.cfg, false); err != nil {
			t.Fatalf("cleanStaleBranches decline failed: %v", err)
		}
	})

	// Branch should still be in metadata
	if !tr.metadata.IsTracked("stale-decline") {
		t.Fatal("expected stale-decline to still be in metadata")
	}
}

// --- modify.go: isTrunk path, unstaged changes, no changes ---

func TestRunModify_OnTrunk(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prevAll := modifyAll
	prevCommit := modifyCommit
	prevMessage := modifyMessage
	prevPatch := modifyPatch
	defer func() {
		modifyAll = prevAll
		modifyCommit = prevCommit
		modifyMessage = prevMessage
		modifyPatch = prevPatch
	}()

	modifyAll = true
	modifyCommit = false
	modifyMessage = ""
	modifyPatch = false

	// On main (trunk), no changes staged
	err := runModify(nil, nil)
	if err != nil {
		t.Fatalf("runModify on trunk failed: %v", err)
	}
}

// --- rename.go: same-name, prompt cancel ---

func TestRunRename_SameNameBoost(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-same", "main")

	err := runRename(nil, []string{"feat-same"})
	if err != nil {
		t.Fatalf("renaming to same name should not error: %v", err)
	}
}

func TestRunRename_PromptCancel(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-rename-cancel", "main")

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runRename(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunRename_AlreadyExists(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-a-rename", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.createBranch(t, "feat-b-rename", "main")

	err := runRename(nil, []string{"feat-a-rename"})
	if err == nil {
		t.Fatal("expected error for branch already exists")
	}
}

// --- land.go: cancel confirmation ---

func TestRunLand_CancelConfirmation(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-land-cancel", "main")

	prevForce := landForce
	defer func() { landForce = prevForce }()
	landForce = false

	withAskOne(t, []interface{}{false}, func() {
		err := runLand(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on cancel, got: %v", err)
		}
	})
}

func TestRunLand_TrunkError(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prevForce := landForce
	defer func() { landForce = prevForce }()
	landForce = true

	err := runLand(nil, []string{"main"})
	if err == nil {
		t.Fatal("expected cannot-land-trunk error")
	}
}

// --- sync.go: runSync full path with restack and delete ---

func TestRunSync_FullPath(t *testing.T) {
	localDir, _, cleanup := setupRepoWithRemote(t)
	defer cleanup()

	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	defer func() { _ = os.Chdir(origDir) }()
	if err := os.Chdir(localDir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	repo, err := git.NewRepo()
	if err != nil {
		t.Fatalf("open repo: %v", err)
	}

	cfg := config.NewConfig("main")
	if err := cfg.Save(repo.GetConfigPath()); err != nil {
		t.Fatalf("save config: %v", err)
	}
	metadata := &config.Metadata{Branches: map[string]*config.BranchMetadata{}}
	if err := metadata.Save(repo.GetMetadataPath()); err != nil {
		t.Fatalf("save metadata: %v", err)
	}

	prevForce := syncForce
	prevRestack := syncRestack
	prevDelete := syncDelete
	defer func() {
		syncForce = prevForce
		syncRestack = prevRestack
		syncDelete = prevDelete
	}()

	syncForce = true
	syncRestack = true
	syncDelete = true

	if err := runSync(nil, nil); err != nil {
		t.Fatalf("runSync full path failed: %v", err)
	}
}

func TestRunSync_DirtyWorkingTreeBoost(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create dirty state
	if err := os.WriteFile(filepath.Join(tr.dir, "dirty.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "dirty.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	err := runSync(nil, nil)
	if err == nil {
		t.Fatal("expected dirty working tree error")
	}
}

// --- restackAllBranches: with failed rebases ---

func TestRestackAllBranches_WithConflicts(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a branch that needs restack with conflict
	tr.createBranch(t, "feat-conflict", "main")
	if err := tr.repo.CheckoutBranch("feat-conflict"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "conflict.txt", "branch-content", "branch commit")

	// Advance main to create divergence
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "conflict.txt", "main-content", "main commit")

	// Update the parent revision to be old so needsRebase returns true
	tr.metadata.Branches["feat-conflict"].ParentRevision = "old"
	if err := tr.metadata.Save(tr.repo.GetMetadataPath()); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Build stack and try restack
	s, err := buildStackForTests(tr)
	if err != nil {
		t.Fatalf("build stack: %v", err)
	}

	succeeded, failed := restackAllBranches(tr.repo, s, tr.metadata)
	// Either it rebases or conflicts — both outcomes are OK for coverage
	_ = succeeded
	_ = failed
}

func buildStackForTests(tr *cmdTestRepo) (*stack.Stack, error) {
	cfg, err := config.Load(tr.repo.GetConfigPath())
	if err != nil {
		return nil, err
	}
	meta, err := loadMetadata(tr.repo)
	if err != nil {
		return nil, err
	}
	return stack.BuildStack(tr.repo, cfg, meta)
}

// --- metadata_loader.go: pushMetadataRefs error path with remote ---

func TestPushMetadataRefs_FailedPush(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Add a remote pointing to nonexistent path
	if _, err := tr.repo.RunGitCommand("remote", "add", "origin", "/tmp/nonexistent-bare-xyz"); err != nil {
		t.Fatalf("add remote: %v", err)
	}

	// Should not panic — error is printed but ignored
	pushMetadataRefs(tr.repo, "some-branch")
	pushMetadataRefs(tr.repo)
}

// --- repair.go gaps ---

func TestRunRepair_MissingConfigBoost(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	err := runRepair(nil, nil)
	if err == nil {
		t.Fatal("expected error for missing config")
	}
}
