package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

// --- Various missing config error paths ---

func TestRunBottom_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runBottom(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunDown_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runDown(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunUp_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runUp(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTop_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runTop(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunChildren_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runChildren(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunParent_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runParent(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunInfo_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runInfo(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunFold_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runFold(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunSplit_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runSplit(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunTrack_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runTrack(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunUntrack_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runUntrack(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunCheckout_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runCheckout(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunCreate_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runCreate(nil, []string{"test-branch"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunCommit_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runCommit(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunContinue_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	// runContinue prints message when no rebase in progress, returns nil
	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunLand_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runLand(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunRepair_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runRepair(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunSubmit_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runSubmit(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunSync_MissingConfig(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()
	if err := runSync(nil, nil); err == nil {
		t.Fatal("expected error")
	}
}

// --- More specific uncovered paths ---

func TestRunCreate_WithMessageAndStagedChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create a file and stage it before creating branch
	if err := os.WriteFile(filepath.Join(tr.dir, "staged-create.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "staged-create.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = "create with staged"
	defer func() { createMessage = prev }()

	// Provide "staged" response for promptHasChanges (since we have staged changes)
	withAskOne(t, []interface{}{"Commit staged changes"}, func() {
		if err := runCreate(nil, []string{"feat-create-staged"}); err != nil {
			t.Fatalf("runCreate with staged failed: %v", err)
		}
	})
}

func TestRunCreate_WithMessageAllChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create unstaged change
	if err := os.WriteFile(filepath.Join(tr.dir, "unstaged-create.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "unstaged-create.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = "create with all changes"
	defer func() { createMessage = prev }()

	// Mock to commit all (--all)
	withAskOne(t, []interface{}{"Commit all file changes (--all)"}, func() {
		if err := runCreate(nil, []string{"feat-create-all"}); err != nil {
			t.Fatalf("runCreate with all failed: %v", err)
		}
	})
}

func TestRunCreate_AbortedNoMessage(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create some changes
	if err := os.WriteFile(filepath.Join(tr.dir, "abort-create.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "abort-create.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	// Abort at the prompt
	withAskOne(t, []interface{}{"Abort this operation"}, func() {
		if err := runCreate(nil, []string{"feat-create-abort"}); err != nil {
			t.Fatalf("runCreate abort failed: %v", err)
		}
	})
}

func TestRunCreate_InterruptedPrompt(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := os.WriteFile(filepath.Join(tr.dir, "int-create.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "int-create.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	withAskOneError(t, terminal.InterruptErr, func() {
		if err := runCreate(nil, []string{"feat-create-int"}); err != nil {
			t.Fatalf("runCreate interrupt should not error: %v", err)
		}
	})
}

// --- commit.go additional paths ---

func TestRunCommit_WithMessageFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-msg", "main")

	if err := os.WriteFile(filepath.Join(tr.dir, "commitmsg.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "commitmsg.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := commitMessage
	prevAll := commitAll
	commitMessage = "test commit message"
	commitAll = false
	defer func() { commitMessage = prev; commitAll = prevAll }()

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("runCommit with message failed: %v", err)
	}
}

func TestRunCommit_AllFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-all", "main")

	// Create a tracked file then modify it (unstaged change)
	tr.commitFile(t, "allmod.txt", "original", "original commit")
	if err := os.WriteFile(filepath.Join(tr.dir, "allmod.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prev := commitMessage
	prevAll := commitAll
	commitMessage = "commit all changes"
	commitAll = true
	defer func() { commitMessage = prev; commitAll = prevAll }()

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("runCommit --all failed: %v", err)
	}
}

// --- untrack.go additional paths ---

func TestRunUntrack_NotTrackedBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// "not tracked" prints a message but doesn't error
	err := runUntrack(nil, []string{"nonexistent-untrack"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- delete.go with force and children ---

func TestRunDelete_ForceWithChildren(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "del-parent", "main")
	tr.commitFile(t, "delp.txt", "data", "parent commit")
	tr.createBranch(t, "del-child", "del-parent")
	tr.commitFile(t, "delc.txt", "data", "child commit")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	// Force delete parent with children
	prev := deleteForce
	deleteForce = true
	defer func() { deleteForce = prev }()

	// Mock confirm to accept
	withAskOne(t, []interface{}{true}, func() {
		if err := runDelete(nil, []string{"del-parent"}); err != nil {
			t.Fatalf("runDelete force with children failed: %v", err)
		}
	})
}
