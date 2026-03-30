package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/israelmalagutti/git-stack/internal/config"
)

// Test interrupt at specific prompt positions in create.go

func TestRunCreate_InterruptAtCommitMessage_All(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := os.WriteFile(filepath.Join(tr.dir, "int-cm.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "int-cm.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	// Select "all" then interrupt at commit message
	withAskOneSequence(t, []interface{}{
		"Commit all file changes (--all)",
		terminal.InterruptErr,
	}, func() {
		err := runCreate(nil, []string{"feat-int-all-msg"})
		if err != nil {
			t.Fatalf("expected nil error on interrupt: %v", err)
		}
	})
}

func TestRunCreate_InterruptAtCommitMessage_Staged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	if err := os.WriteFile(filepath.Join(tr.dir, "int-st.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "int-st.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := createMessage
	createMessage = ""
	defer func() { createMessage = prev }()

	// Select "staged" then interrupt at commit message
	withAskOneSequence(t, []interface{}{
		"Commit staged changes",
		terminal.InterruptErr,
	}, func() {
		err := runCreate(nil, []string{"feat-int-staged-msg"})
		if err != nil {
			t.Fatalf("expected nil error on interrupt: %v", err)
		}
	})
}

// Test interrupt in commit.go prompt chains

func TestRunCommit_InterruptAtMessage_All(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-int-cm-all", "main")
	if err := os.WriteFile(filepath.Join(tr.dir, "int-all.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "int-all.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := commitMessage
	commitMessage = ""
	defer func() { commitMessage = prev }()

	// Select "all" then interrupt at message prompt
	withAskOneSequence(t, []interface{}{
		"Stage all changes and commit (--all)",
		terminal.InterruptErr,
	}, func() {
		err := runCommit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on interrupt: %v", err)
		}
	})
}

func TestRunCommit_InterruptAtMessage_Staged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-int-cm-stg", "main")
	if err := os.WriteFile(filepath.Join(tr.dir, "int-stg.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "int-stg.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := commitMessage
	commitMessage = ""
	defer func() { commitMessage = prev }()

	withAskOneSequence(t, []interface{}{
		"Commit staged changes",
		terminal.InterruptErr,
	}, func() {
		err := runCommit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on interrupt: %v", err)
		}
	})
}

func TestRunCommit_InterruptAtAction(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-int-cm-act", "main")
	if err := os.WriteFile(filepath.Join(tr.dir, "int-act.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "int-act.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := commitMessage
	commitMessage = ""
	defer func() { commitMessage = prev }()

	withAskOneError(t, terminal.InterruptErr, func() {
		err := runCommit(nil, nil)
		if err != nil {
			t.Fatalf("expected nil on interrupt: %v", err)
		}
	})
}

// More init.go paths

func TestRunInit_BranchSelected_Exists(t *testing.T) {
	_, cleanup := setupRawRepo(t)
	defer cleanup()

	// Select 'main' as trunk
	withAskOne(t, []interface{}{"main"}, func() {
		if err := runInit(nil, nil); err != nil {
			t.Fatalf("runInit failed: %v", err)
		}
	})

	// Now it's already initialized
	if err := runInit(nil, nil); err == nil {
		t.Fatal("expected already initialized error")
	}
}

// Test more specific sync.go paths

func TestSyncTrunkWithRemote_NoRemoteBranch(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	// No remote -> syncTrunkWithRemote should handle gracefully
	err := syncTrunkWithRemote(tr.repo, "main", true)
	if err != nil {
		t.Fatalf("syncTrunkWithRemote no remote failed: %v", err)
	}
}

// Test more log paths

func TestRunLog_WithTrackedBranches(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-log-a", "main")
	tr.commitFile(t, "la.txt", "a", "a commit")
	tr.createBranch(t, "feat-log-b", "feat-log-a")
	tr.commitFile(t, "lb.txt", "b", "b commit")

	prevS := logShort
	prevL := logLong
	logShort = false
	logLong = true
	defer func() { logShort = prevS; logLong = prevL }()

	if err := runLog(nil, nil); err != nil {
		t.Fatalf("runLog long with branches failed: %v", err)
	}
}

// Test continue with saved state

func TestRunContinue_WithSavedState(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-cont", "main")
	tr.commitFile(t, "cont.txt", "data", "cont commit")

	// Create a continue state file with empty remaining branches
	state := &config.ContinueState{
		RemainingBranches: []string{},
		OriginalBranch:    "main",
	}
	if err := state.Save(tr.repo.GetContinueStatePath()); err != nil {
		t.Fatalf("save state: %v", err)
	}

	// No rebase in progress, but state exists with empty branches -> should clear state
	if err := runContinue(nil, nil); err != nil {
		t.Fatalf("runContinue with state failed: %v", err)
	}
}

// Test stack_restack with flags

func TestRunStackRestack_OnlyFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-only-rs", "main")
	tr.commitFile(t, "only-rs.txt", "data", "commit")

	// Go to main and create commit
	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "main-only.txt", "main", "main commit")

	if err := tr.repo.CheckoutBranch("feat-only-rs"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := restackOnly
	restackOnly = true
	defer func() { restackOnly = prev }()

	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack --only failed: %v", err)
	}
}

func TestRunStackRestack_UpstackFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-up-rs", "main")
	tr.commitFile(t, "up-rs.txt", "data", "commit")
	tr.createBranch(t, "feat-up-child", "feat-up-rs")
	tr.commitFile(t, "up-child.txt", "data", "child commit")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "main-up.txt", "main", "main commit")

	if err := tr.repo.CheckoutBranch("feat-up-rs"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := restackUpstack
	restackUpstack = true
	defer func() { restackUpstack = prev }()

	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack --upstack failed: %v", err)
	}
}

func TestRunStackRestack_DownstackFlag(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-down-rs", "main")
	tr.commitFile(t, "down-rs.txt", "data", "commit")
	tr.createBranch(t, "feat-down-child", "feat-down-rs")
	tr.commitFile(t, "down-child.txt", "data", "child commit")

	if err := tr.repo.CheckoutBranch("main"); err != nil {
		t.Fatalf("checkout: %v", err)
	}
	tr.commitFile(t, "main-down.txt", "main", "main commit")

	if err := tr.repo.CheckoutBranch("feat-down-child"); err != nil {
		t.Fatalf("checkout: %v", err)
	}

	prev := restackDownstack
	restackDownstack = true
	defer func() { restackDownstack = prev }()

	if err := runStackRestack(nil, nil); err != nil {
		t.Fatalf("runStackRestack --downstack failed: %v", err)
	}
}
