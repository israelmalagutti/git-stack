package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

// --- fold.go uncovered paths ---

func TestRunFold_KeepWithGrandparent(t *testing.T) {
	// Test the --keep path where parent has a parent (grandparent path)
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-gp", "main")
	tr.commitFile(t, "gp.txt", "gp", "gp commit")
	tr.createBranch(t, "feat-fold-gp", "feat-gp")
	tr.commitFile(t, "fold-gp.txt", "data", "fold commit")

	foldKeep = true
	foldForce = true
	defer func() { foldKeep = false; foldForce = false }()

	if err := runFold(nil, nil); err != nil {
		t.Fatalf("runFold keep with grandparent failed: %v", err)
	}
}

func TestRunFold_KeepOnTrunkChild(t *testing.T) {
	// Test --keep path where parent is trunk (no grandparent)
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-fold-trunk", "main")
	tr.commitFile(t, "foldtk.txt", "data", "fold commit")

	foldKeep = true
	foldForce = true
	defer func() { foldKeep = false; foldForce = false }()

	if err := runFold(nil, nil); err != nil {
		t.Fatalf("runFold keep trunk child failed: %v", err)
	}
}

func TestRunFold_ConfirmAccepted(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-fold-ok", "main")
	tr.commitFile(t, "fold-ok.txt", "data", "fold commit")

	foldKeep = false
	foldForce = false
	defer func() { foldKeep = false; foldForce = false }()

	withAskOne(t, []interface{}{true}, func() {
		if err := runFold(nil, nil); err != nil {
			t.Fatalf("runFold confirm failed: %v", err)
		}
	})
}

func TestRunFold_NoChangesCommit(t *testing.T) {
	// Fold branch with no changes (empty branch, no diff from parent)
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-fold-empty", "main")
	// Don't commit anything, so fold has no changes

	foldKeep = false
	foldForce = true
	defer func() { foldKeep = false; foldForce = false }()

	if err := runFold(nil, nil); err != nil {
		t.Fatalf("runFold empty failed: %v", err)
	}
}

// --- continue.go uncovered paths ---

func TestRunContinue_NotInProgress(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No continue state exists — prints message but returns nil
	err := runContinue(nil, nil)
	if err != nil {
		t.Fatalf("runContinue should not error: %v", err)
	}
}

// --- commit.go uncovered paths ---

func TestRunCommit_NoChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-none", "main")

	// No changes prints info but returns nil
	err := runCommit(nil, nil)
	if err != nil {
		t.Fatalf("runCommit no changes should not error: %v", err)
	}
}

func TestRunCommit_StagedChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-staged", "main")

	// Create a file and stage it
	if err := os.WriteFile(filepath.Join(tr.dir, "staged.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "staged.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	// Provide commit message via prompt
	withAskOne(t, []interface{}{"Commit staged changes", "test commit msg"}, func() {
		if err := runCommit(nil, nil); err != nil {
			t.Fatalf("runCommit staged failed: %v", err)
		}
	})
}

// --- modify.go uncovered paths ---

func TestRunModify_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	if err := runModify(nil, nil); err == nil {
		t.Fatal("expected error without config")
	}
}

// --- move.go uncovered paths ---

func TestRunMove_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	if err := runMove(nil, nil); err == nil {
		t.Fatal("expected error without config")
	}
}

// --- split.go additional uncovered paths ---

func TestRunSplit_FileModeSingleFile(t *testing.T) {
	// This calls runSplit (not splitByFileMode directly) to cover the runSplit dispatch path
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-split-rsf", "main")
	tr.commitFile(t, "rsf.txt", "rsf data", "rsf commit")

	prev := splitByFile
	prevName := splitName
	prevCommit := splitByCommit
	prevHunk := splitByHunk
	splitByFile = []string{"rsf.txt"}
	splitName = "feat-rsf-base"
	splitByCommit = false
	splitByHunk = false
	defer func() {
		splitByFile = prev
		splitName = prevName
		splitByCommit = prevCommit
		splitByHunk = prevHunk
	}()

	if err := runSplit(nil, nil); err != nil {
		t.Fatalf("runSplit file single failed: %v", err)
	}
}

func TestSplitByCommitMode_OneCommit(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-split-1c", "main")
	tr.commitFile(t, "only.txt", "only", "only commit")

	// Can't split 1 commit by commit mode
	err := splitByCommitMode(tr.repo, tr.cfg, tr.metadata, "feat-split-1c", "main", "feat-base-1c", 1)
	if err == nil {
		t.Fatal("expected error for single commit split by commit")
	}
}

// --- top.go uncovered paths ---

func TestRunTop_Cancelled(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "tl1", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.createBranch(t, "tl2", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Multiple leaves -> prompt, user cancels
	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runTop(nil, nil); err != nil {
			t.Fatalf("runTop cancelled should not error: %v", err)
		}
	})
}

// --- delete.go additional paths ---

func TestRunDelete_UntrackedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := tr.repo.CreateBranch("untracked-del"); err != nil {
		t.Fatalf("create: %v", err)
	}

	err := runDelete(nil, []string{"untracked-del"})
	if err == nil {
		t.Fatal("expected error for untracked branch")
	}
}

func TestRunDelete_NonExistentBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	err := runDelete(nil, []string{"nonexistent-del"})
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}
}

// --- rename.go additional paths ---

func TestRunRename_InsufficientArgs(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No args uses current branch; check error for trunk rename
	err := runRename(nil, nil)
	if err == nil {
		t.Fatal("expected error renaming trunk")
	}
}

// --- stack_restack.go uncovered paths ---

func TestRunStackRestack_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	if err := runStackRestack(nil, nil); err == nil {
		t.Fatal("expected error without config")
	}
}

// --- completion.go ---

func TestCompleteBranchNames_Filtered(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-comp-a", "main")
	tr.createBranch(t, "feat-comp-b", "main")

	// With a prefix
	names, _ := completeBranchNamesFunc(nil, nil, "feat-comp")
	if len(names) < 2 {
		t.Errorf("expected at least 2 branches matching prefix, got %d", len(names))
	}
}

func TestCompleteBranchNames_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	// Should return nil gracefully when no config
	names, _ := completeBranchNamesFunc(nil, nil, "")
	if names != nil {
		t.Errorf("expected nil names without config, got %v", names)
	}
}
