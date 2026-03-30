package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
)

func TestRunCreate_NoChangesNoMessage(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	// No changes, no message -> creates branch with no commit
	if err := runCreate(nil, []string{"feat-create-empty"}); err != nil {
		t.Fatalf("runCreate empty failed: %v", err)
	}
}

func TestRunCreate_WithMessageNoChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	prev := createMessage
	createMessage = "empty commit"
	defer func() { createMessage = prev }()

	// Has message but no changes -> "no changes to commit"
	if err := runCreate(nil, []string{"feat-create-msg-nc"}); err != nil {
		t.Fatalf("runCreate message no changes failed: %v", err)
	}
}

func TestRunCreate_WithMessageAndAllChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create some unstaged changes
	tr.commitFile(t, "exist.txt", "original", "original")
	if err := os.WriteFile(filepath.Join(tr.dir, "exist.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prev := createMessage
	createMessage = "commit with message"
	defer func() { createMessage = prev }()

	// Has message and unstaged changes -> prompt for action
	withAskOne(t, []interface{}{"Commit all file changes (--all)"}, func() {
		if err := runCreate(nil, []string{"feat-create-msg-all"}); err != nil {
			t.Fatalf("runCreate message all failed: %v", err)
		}
	})
}

func TestRunCreate_NoMessageWithStagedAndUnstaged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Create both staged and unstaged changes
	if err := os.WriteFile(filepath.Join(tr.dir, "staged.txt"), []byte("staged"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "staged.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tr.dir, "unstaged.txt"), []byte("unstaged"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	// Prompt: "Commit staged changes" then "commit message"
	withAskOne(t, []interface{}{"Commit staged changes", "staged msg"}, func() {
		if err := runCreate(nil, []string{"feat-create-no-msg-staged"}); err != nil {
			t.Fatalf("runCreate no message staged failed: %v", err)
		}
	})
}

func TestRunCreate_NoMessageAllAction(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := os.WriteFile(filepath.Join(tr.dir, "newfile.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "newfile.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	// "Commit all" then message
	withAskOne(t, []interface{}{"Commit all file changes (--all)", "all msg"}, func() {
		if err := runCreate(nil, []string{"feat-create-no-msg-all"}); err != nil {
			t.Fatalf("runCreate no message all failed: %v", err)
		}
	})
}

func TestRunCreate_NoMessageAbortAction(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := os.WriteFile(filepath.Join(tr.dir, "abort.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "abort.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	withAskOne(t, []interface{}{"Abort this operation"}, func() {
		if err := runCreate(nil, []string{"feat-create-no-msg-abort"}); err != nil {
			t.Fatalf("runCreate no message abort failed: %v", err)
		}
	})
}

func TestRunCreate_MessageWithAbortPrompt(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Staged changes
	if err := os.WriteFile(filepath.Join(tr.dir, "msg-abort.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "msg-abort.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = "msg with abort"
	defer func() { createMessage = prev }()

	// Message provided, changes exist -> prompt for action, abort
	withAskOne(t, []interface{}{"Abort this operation"}, func() {
		if err := runCreate(nil, []string{"feat-create-msg-abort"}); err != nil {
			t.Fatalf("runCreate message abort failed: %v", err)
		}
	})
}

func TestRunCreate_BranchAlreadyExists(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "existing-branch", "main")
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	err := runCreate(nil, []string{"existing-branch"})
	if err == nil {
		t.Fatal("expected error for existing branch")
	}
}

func TestRunCreate_InterruptedNoMessage(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := os.WriteFile(filepath.Join(tr.dir, "int.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "int.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	// Interrupt at action prompt
	withAskOneError(t, terminal.InterruptErr, func() {
		err := runCreate(nil, []string{"feat-create-int-nm"})
		if err != nil {
			t.Fatalf("runCreate interrupt should not error: %v", err)
		}
	})
}

func TestRunCreate_MessageNoCommitAction(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// Staged changes
	if err := os.WriteFile(filepath.Join(tr.dir, "nc.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "nc.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = "msg"
	defer func() { createMessage = prev }()

	// "no-commit" action
	withAskOne(t, []interface{}{"Create branch without committing"}, func() {
		if err := runCreate(nil, []string{"feat-create-nc-act"}); err != nil {
			t.Fatalf("runCreate no-commit action failed: %v", err)
		}
	})
}
