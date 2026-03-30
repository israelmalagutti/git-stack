package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunCommit_PromptedAll(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-pa", "main")

	// Create unstaged change (no staged)
	if err := os.WriteFile(filepath.Join(tr.dir, "pa.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "pa.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := commitMessage
	prevAll := commitAll
	commitMessage = ""
	commitAll = false
	defer func() { commitMessage = prev; commitAll = prevAll }()

	// Select "Stage all changes and commit" then provide message
	withAskOne(t, []interface{}{"Stage all changes and commit (--all)", "all commit msg"}, func() {
		if err := runCommit(nil, nil); err != nil {
			t.Fatalf("runCommit prompted all failed: %v", err)
		}
	})
}

func TestRunCommit_PromptedAbort(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-ab", "main")

	if err := os.WriteFile(filepath.Join(tr.dir, "ab.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "ab.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}

	prev := commitMessage
	prevAll := commitAll
	commitMessage = ""
	commitAll = false
	defer func() { commitMessage = prev; commitAll = prevAll }()

	withAskOne(t, []interface{}{"Abort"}, func() {
		if err := runCommit(nil, nil); err != nil {
			t.Fatalf("runCommit abort failed: %v", err)
		}
	})
}

func TestRunCommit_WithMessageUnstagedOnly(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-us", "main")

	// Create tracked file, modify it (unstaged change, no staged)
	tr.commitFile(t, "us.txt", "original", "original")
	if err := os.WriteFile(filepath.Join(tr.dir, "us.txt"), []byte("modified"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prev := commitMessage
	prevAll := commitAll
	commitMessage = "unstaged auto-stage commit"
	commitAll = false
	defer func() { commitMessage = prev; commitAll = prevAll }()

	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("runCommit unstaged only with message failed: %v", err)
	}
}

func TestRunCommit_WithMessageNoChanges(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-nc", "main")

	prev := commitMessage
	prevAll := commitAll
	commitMessage = "no changes"
	commitAll = false
	defer func() { commitMessage = prev; commitAll = prevAll }()

	// No changes - should print info, no error
	if err := runCommit(nil, nil); err != nil {
		t.Fatalf("runCommit no changes with message failed: %v", err)
	}
}

func TestRunCommit_PromptedStaged(t *testing.T) {
	tr := setupCmdTestRepo(t)
	defer tr.cleanup()

	tr.createBranch(t, "feat-commit-ps", "main")

	// Stage a file, also have unstaged changes
	if err := os.WriteFile(filepath.Join(tr.dir, "staged-ps.txt"), []byte("staged"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := tr.repo.RunGitCommand("add", "staged-ps.txt"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tr.dir, "unstaged-ps.txt"), []byte("unstaged"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	prev := commitMessage
	prevAll := commitAll
	commitMessage = ""
	commitAll = false
	defer func() { commitMessage = prev; commitAll = prevAll }()

	// "Commit staged changes" then message
	withAskOne(t, []interface{}{"Commit staged changes", "staged commit msg"}, func() {
		if err := runCommit(nil, nil); err != nil {
			t.Fatalf("runCommit prompted staged failed: %v", err)
		}
	})
}
